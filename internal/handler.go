package internal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/httplog/v3"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/gabrieleangeletti/stride"
	"github.com/gabrieleangeletti/stride/strava"
	"github.com/gabrieleangeletti/vo2"
	"github.com/gabrieleangeletti/vo2/activity"
	"github.com/gabrieleangeletti/vo2/database"
	"github.com/gabrieleangeletti/vo2/provider"
	"github.com/gabrieleangeletti/vo2/store"
	"github.com/gabrieleangeletti/vo2/util"
)

type Environment string

const (
	Development Environment = "development"
	Production  Environment = "production"
)

type Handler struct {
	db          *sqlx.DB
	handler     http.Handler
	store       store.Store
	middlewares []func(http.Handler) http.Handler
}

func NewHandler(db *sqlx.DB, options ...func(*Handler)) *Handler {
	s, err := store.NewStore(db)
	if err != nil {
		log.Fatal(err)
	}

	h := &Handler{
		db:    db,
		store: s,
	}

	for _, opt := range options {
		opt(h)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /providers/strava/auth/callback", stravaAuthHandler(h.db, h.store))
	mux.HandleFunc("GET /providers/strava/webhook", stravaRegisterWebhookHandler(h.db))
	mux.HandleFunc("POST /providers/strava/webhook", stravaWebhookHandler(h.db, h.store))

	mux.HandleFunc("GET /athletes/{athleteID}/metrics/volume", athleteVolumeHandler(h.db, h.store))

	h.handler = h.chain(mux)

	return h
}

func WithHttpLogging(env Environment) func(h *Handler) {
	debugFn := func(r *http.Request) bool {
		return env == Development
	}

	logFormat := httplog.SchemaECS.Concise(false)

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		ReplaceAttr: logFormat.ReplaceAttr,
	})).With(
		slog.String("app", "vo2"),
		slog.String("version", "v0.0.1"),
		slog.String("env", string(env)),
	)

	middleware := httplog.RequestLogger(logger, &httplog.Options{
		Level:              slog.LevelInfo,
		Schema:             httplog.SchemaECS,
		RecoverPanics:      true,
		LogRequestHeaders:  []string{"Origin"},
		LogResponseHeaders: []string{},
		LogRequestBody:     debugFn,
		LogResponseBody:    debugFn,
	})

	return func(h *Handler) {
		h.middlewares = append(h.middlewares, middleware)
	}
}

func (h *Handler) chain(base http.Handler) http.Handler {
	if len(h.middlewares) == 0 {
		return base
	}

	handler := base
	for i := len(h.middlewares) - 1; i >= 0; i-- {
		handler = h.middlewares[i](handler)
	}

	return handler
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.handler.ServeHTTP(w, r)
}

func (h *Handler) ProcessHistoricalDataTask(ctx context.Context, task HistoricalDataTask) error {
	prov, err := provider.GetByID(h.db, task.ProviderID)
	if err != nil {
		return fmt.Errorf("failed to get provider: %w", err)
	}

	// TODO: make provider agnostic
	driver := NewStravaDriver()

	credentials, err := ensureValidCredentials(ctx, h.db, driver, prov, task.AthleteID)
	if err != nil {
		return fmt.Errorf("failed to get valid credentials: %w", err)
	}

	client := driver.NewClient(credentials.AccessToken)

	activities, err := GetStravaActivitySummaries(client, task.StartTime, task.EndTime)
	if err != nil {
		return fmt.Errorf("failed to get Strava activities: %w", err)
	}

	if len(activities) == 0 {
		slog.Info("No activities found for time range", "athleteId", task.AthleteID, "startTime", task.StartTime, "endTime", task.EndTime)
		return nil
	}

	existingActivities, err := activity.GetProviderActivityRawData(ctx, h.db, prov.ID, task.AthleteID)
	if err != nil {
		return fmt.Errorf("failed to get existing activities: %w", err)
	}

	existingActivitiesMap := make(map[int64]*activity.ProviderActivityRawData)
	for _, a := range existingActivities {
		id, err := strconv.ParseInt(a.ProviderActivityID, 10, 64)
		if err != nil {
			return fmt.Errorf("failed to parse activity ID: %w", err)
		}

		existingActivitiesMap[id] = a
	}

	slog.Info("Processing historical activities", "athleteId", task.AthleteID, "activityCount", len(activities), "startTime", task.StartTime, "endTime", task.EndTime)

	for _, act := range activities {
		// Note: this is the most basic way to handle rate limits. We basically go until we hit the limit.
		// Then we manually retry and ignore the activities that were already processed.
		// TODO: implement proper solution.
		if existing, ok := existingActivitiesMap[act.ID]; ok {
			if !existing.DetailedActivityURI.Valid {
				streams, err := client.GetActivityStreams(act.ID)
				if err != nil {
					return fmt.Errorf("failed to get activity streams: %w", err)
				}

				err = h.store.UploadRawActivityDetails(ctx, stride.Provider(prov.Slug), existing, streams)
				if err != nil {
					return fmt.Errorf("failed to upload raw activity details: %w", err)
				}
			}

			continue
		}

		detailedActivity, err := client.GetActivity(act.ID, false)
		if err != nil {
			return fmt.Errorf("failed to get detailed activity: %w", err)
		}

		data, err := json.Marshal(detailedActivity)
		if err != nil {
			return fmt.Errorf("failed to marshal activity: %w", err)
		}

		activityRaw := activity.ProviderActivityRawData{
			ProviderID:         prov.ID,
			AthleteID:          task.AthleteID,
			ProviderActivityID: strconv.FormatInt(detailedActivity.ID, 10),
			StartTime:          act.StartDate,
			ElapsedTime:        act.ElapsedTime,
			IanaTimezone:       database.ToNullString(detailedActivity.IanaTimezone()),
			Data:               data,
		}

		activityRawID, err := h.store.SaveProviderActivityRawData(ctx, &activityRaw)
		if err != nil {
			return fmt.Errorf("failed to save activity: %w", err)
		}

		activityRaw.ID = activityRawID

		streams, err := client.GetActivityStreams(detailedActivity.ID)
		if err != nil {
			return fmt.Errorf("failed to get activity streams: %w", err)
		}

		err = h.store.UploadRawActivityDetails(ctx, stride.Provider(prov.Slug), &activityRaw, streams)
		if err != nil {
			return fmt.Errorf("failed to upload raw activity details: %w", err)
		}

		return nil
	}

	slog.Info("Successfully processed historical activities", "athleteId", task.AthleteID, "processedCount", len(activities))

	return nil
}

func stravaAuthHandler(db *sqlx.DB, dbStore store.Store) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()

		errorCode := r.URL.Query().Get("error")
		if errorCode != "" {
			http.Error(w, errorCode, http.StatusBadRequest)
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "No code provided", http.StatusBadRequest)
			return
		}

		auth := strava.NewAuth(
			util.GetSecret("STRAVA_CLIENT_ID", true),
			util.GetSecret("STRAVA_CLIENT_SECRET", true),
		)

		tokenResponse, err := auth.ExchangeCodeForAccessToken(code)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		prov, err := provider.GetBySlug(db, "strava")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		tx, err := db.Beginx()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		user, err := CreateUser(tx, prov.ID, strconv.Itoa(tokenResponse.Athlete.ID))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// TODO: hardcoded for now.
		athlete, err := dbStore.UpsertAthlete(ctx, &vo2.Athlete{
			UserID:      user.ID,
			Age:         33,
			HeightCm:    180,
			Country:     "it",
			Gender:      vo2.GenderMale,
			FirstName:   "Gabriele",
			LastName:    "Angeletti",
			DisplayName: "Gabriele Angeletti",
			Email:       "angeletti.gabriele@gmail.com",
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		credentials := ProviderOAuth2Credentials{
			ProviderID:   prov.ID,
			UserID:       user.ID,
			AccessToken:  tokenResponse.AccessToken,
			RefreshToken: tokenResponse.RefreshToken,
			ExpiresAt:    time.Unix(int64(tokenResponse.ExpiresAt), 0),
		}

		err = credentials.SaveTx(r.Context(), tx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			slog.Error(err.Error())
			http.Error(w, ErrGeneric.Error(), http.StatusInternalServerError)
			return
		}

		if err := queueHistoricalDataTasks(r.Context(), athlete.ID, prov.ID, HistoricalDataTaskTypeActivity); err != nil {
			slog.Error("Failed to queue historical data tasks", "error", err, "athleteId", athlete.ID)
			// Queue historical data tasks synchronously to ensure completion in case we are serverless (e.g. Lambda).
			// Don't fail the entire auth flow if queuing fails - user is still authenticated.
			// TODO: we should rearchitect this to run async even in serverless environments.
		}

		resp := map[string]bool{"success": true}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		json.NewEncoder(w).Encode(resp)
	}
}

func stravaRegisterWebhookHandler(db *sqlx.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		mode := r.URL.Query().Get("hub.mode")
		if mode != "subscribe" {
			http.Error(w, "invalid hub.mode", http.StatusBadRequest)
			return
		}

		challenge := r.URL.Query().Get("hub.challenge")
		if challenge == "" {
			http.Error(w, "invalid hub.challenge", http.StatusBadRequest)
			return
		}

		verifyToken := r.URL.Query().Get("hub.verify_token")
		if verifyToken == "" {
			http.Error(w, "invalid hub.verify_token", http.StatusBadRequest)
			return
		}

		isValid, err := verifyWebhook(r.Context(), db, verifyToken)
		if err != nil {
			slog.Error("error while verifying strava webhook token: " + err.Error())
			http.Error(w, "error while verifying token", http.StatusInternalServerError)
			return
		}

		if !isValid {
			http.Error(w, "invalid verification token", http.StatusBadRequest)
			return
		}

		resp := map[string]string{"hub.challenge": challenge}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		json.NewEncoder(w).Encode(resp)
	}
}

func stravaWebhookHandler(db *sqlx.DB, dbStore store.Store) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		slog.Info("Received Strava Webhook")

		ctx := context.Background()

		var event strava.WebhookEvent
		err := json.NewDecoder(r.Body).Decode(&event)
		if err != nil {
			slog.Error(err.Error())
			http.Error(w, ErrGeneric.Error(), http.StatusBadRequest)
			return
		}

		prov, err := provider.GetBySlug(db, "strava")
		if err != nil {
			slog.Error(err.Error())
			http.Error(w, ErrGeneric.Error(), http.StatusBadRequest)
			return
		}

		user, err := GetUser(db, prov.ID, strconv.Itoa(event.OwnerID))
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				slog.Error(fmt.Sprintf("%s: (provider: %d, userExternalID: %d)", ErrUserNotFound, prov.ID, event.OwnerID))
				http.Error(w, ErrGeneric.Error(), http.StatusBadRequest)
				return
			}

			slog.Error(err.Error())
			http.Error(w, ErrGeneric.Error(), http.StatusBadRequest)
			return
		}

		athletes, err := dbStore.GetUserAthletes(ctx, user.ID)
		if err != nil {
			slog.Error(err.Error())
			http.Error(w, ErrGeneric.Error(), http.StatusInternalServerError)
			return
		}

		// TODO: for now, we assume that one user has only one athlete.
		athlete := athletes[0]

		driver := NewStravaDriver()

		credentials, err := ensureValidCredentials(r.Context(), db, driver, prov, athlete.ID)
		if err != nil {
			slog.Error(err.Error())
			http.Error(w, ErrGeneric.Error(), http.StatusBadRequest)
			return
		}

		client := driver.NewClient(credentials.AccessToken)

		auth := driver.NewAuth()

		// TODO: we should store subscriptions in the database to avoid unnecessary API calls.
		subs, err := auth.GetWebhookSubscriptions()
		if err != nil {
			slog.Error(err.Error())
			http.Error(w, ErrGeneric.Error(), http.StatusBadRequest)
			return
		}

		isValidSubscription := slices.ContainsFunc(subs, func(s strava.WebhookSubscription) bool {
			return s.ID == event.SubscriptionID
		})

		if !isValidSubscription {
			slog.Error("invalid subscription")
			http.Error(w, "invalid event payload", http.StatusBadRequest)
			return
		}

		if event.ObjectType == strava.WebhookActivity {
			if event.AspectType == strava.WebhookCreate || event.AspectType == strava.WebhookUpdate {
				stravaActivity, err := client.GetActivity(event.ObjectID, false)
				if err != nil {
					slog.Error(err.Error())
					http.Error(w, ErrGeneric.Error(), http.StatusInternalServerError)
					return
				}

				streams, err := client.GetActivityStreams(stravaActivity.ID)
				if err != nil {
					slog.Error(err.Error())
					http.Error(w, ErrGeneric.Error(), http.StatusInternalServerError)
					return
				}

				data, err := json.Marshal(stravaActivity)
				if err != nil {
					slog.Error(err.Error())
					http.Error(w, ErrGeneric.Error(), http.StatusInternalServerError)
					return
				}

				activityRaw := activity.ProviderActivityRawData{
					ProviderID:         prov.ID,
					AthleteID:          athlete.ID,
					ProviderActivityID: strconv.FormatInt(event.ObjectID, 10),
					StartTime:          stravaActivity.StartDate,
					ElapsedTime:        stravaActivity.ElapsedTime,
					IanaTimezone:       database.ToNullString(stravaActivity.IanaTimezone()),
					Data:               data,
				}

				activityRawID, err := dbStore.SaveProviderActivityRawData(ctx, &activityRaw)
				if err != nil {
					slog.Error(err.Error())
					http.Error(w, ErrGeneric.Error(), http.StatusInternalServerError)
					return
				}

				activityRaw.ID = activityRawID

				err = dbStore.UploadRawActivityDetails(ctx, stride.ProviderStrava, &activityRaw, streams)
				if err != nil {
					slog.Error(err.Error())
					http.Error(w, ErrGeneric.Error(), http.StatusInternalServerError)
					return
				}

				act, err := dbStore.StoreActivityEndurance(ctx, stride.ProviderStrava, &activityRaw, stravaActivity, streams)
				if err != nil {
					slog.Error(err.Error())
					http.Error(w, ErrGeneric.Error(), http.StatusInternalServerError)
					return
				}

				tags := act.ExtractActivityTags()

				if len(tags) > 0 {
					err = dbStore.UpsertTagsAndLinkActivity(ctx, act, tags)
					if err != nil {
						slog.Error(err.Error())
						http.Error(w, ErrGeneric.Error(), http.StatusInternalServerError)
						return
					}
				}
			}
		}
	}
}

func queueHistoricalDataTasks(ctx context.Context, athleteID uuid.UUID, providerID int, taskType HistoricalDataTaskType) error {
	sqsClient, err := NewSQSClient()
	if err != nil {
		return fmt.Errorf("failed to create SQS client: %w", err)
	}

	now := time.Now()

	for i := range fourYearsInMonths {
		targetMonth := now.AddDate(0, -i, 0)

		monthStart := time.Date(targetMonth.Year(), targetMonth.Month(), 1, 0, 0, 0, 0, targetMonth.Location())
		monthEnd := monthStart.AddDate(0, 1, 0)

		if i == 0 && monthEnd.After(now) {
			monthEnd = now
		}

		task := HistoricalDataTask{
			AthleteID:  athleteID,
			ProviderID: providerID,
			Type:       taskType,
			StartTime:  monthStart,
			EndTime:    monthEnd,
		}

		if err := sqsClient.SendHistoricalDataTask(ctx, task); err != nil {
			return fmt.Errorf("failed to send historical data task: %w", err)
		}

		slog.Info("Queued historical data task", "athleteId", athleteID, "startTime", monthStart, "endTime", monthEnd)
	}

	return nil
}

func athleteVolumeHandler(db *sqlx.DB, dbStore store.Store) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		apiKey := util.GetSecret("VO2_API_KEY", true)
		if r.Header.Get("x-vo2-api-key") != apiKey {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := r.Context()

		athleteIDStr := r.PathValue("athleteID")
		athleteID, err := uuid.Parse(athleteIDStr)
		if err != nil {
			http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
			return
		}

		athlete, err := dbStore.GetAthlete(ctx, athleteID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				http.Error(w, "User not found", http.StatusNotFound)
				return
			}

			slog.Error("Failed to get user", "error", err, "athleteID", athleteID)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		provider := r.URL.Query().Get("provider")
		frequency := r.URL.Query().Get("frequency")
		startDate := r.URL.Query().Get("startDate")
		sportParams := r.URL.Query()["sport"]

		if provider == "" || frequency == "" || startDate == "" || len(sportParams) == 0 {
			http.Error(w, "Missing required query parameters: provider, frequency, startDate, sport", http.StatusBadRequest)
			return
		}

		if frequency != "day" && frequency != "week" && frequency != "month" {
			http.Error(w, "Invalid frequency, must be one of: day, week, month", http.StatusBadRequest)
			return
		}

		startDateTime, err := time.Parse("2006-01-02", startDate)
		if err != nil {
			http.Error(w, "Invalid startDate format, expected YYYY-MM-DD", http.StatusBadRequest)
			return
		}

		sportsSeen := make(map[stride.Sport]struct{})
		sports := make([]stride.Sport, 0)
		sportStrings := make([]string, 0)

		for _, raw := range sportParams {
			for part := range strings.SplitSeq(raw, ",") {
				s := strings.TrimSpace(part)
				if s == "" {
					continue
				}

				s = strings.ToLower(s)
				sport, err := stride.ParseSport(s)
				if err != nil {
					http.Error(w, "Invalid sport", http.StatusBadRequest)
					return
				}

				if !stride.IsEnduranceActivity(sport) {
					http.Error(w, "Invalid sport, must be an endurance activity", http.StatusBadRequest)
					return
				}

				if _, exists := sportsSeen[sport]; exists {
					continue
				}

				sportsSeen[sport] = struct{}{}
				sports = append(sports, sport)
				sportStrings = append(sportStrings, string(sport))
			}
		}

		if len(sports) == 0 {
			http.Error(w, "At least one sport must be provided", http.StatusBadRequest)
			return
		}

		volumeParams := vo2.GetAthleteVolumeParams{
			Frequency:    frequency,
			AthleteID:    athlete.ID,
			ProviderSlug: provider,
			Sports:       sports,
			StartDate:    startDateTime,
		}

		volumeData, err := dbStore.GetAthleteVolume(ctx, volumeParams)
		if err != nil {
			slog.Error("Failed to get athlete volume", "error", err, "params", volumeParams)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		dataBySport := make(map[string][]*vo2.AthleteVolumeData, len(sports))
		for _, sport := range sports {
			series, ok := volumeData[sport]
			if !ok || series == nil {
				dataBySport[string(sport)] = []*vo2.AthleteVolumeData{}
				continue
			}

			dataBySport[string(sport)] = series
		}

		response := map[string]any{
			"athleteId": athlete.ID,
			"provider":  provider,
			"frequency": frequency,
			"sports":    sportStrings,
			"startDate": startDateTime.Format("2006-01-02"),
			"data":      dataBySport,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}
}
