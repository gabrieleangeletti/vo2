package internal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/gabrieleangeletti/stride"
	"github.com/gabrieleangeletti/stride/strava"
	"github.com/gabrieleangeletti/vo2"
	"github.com/gabrieleangeletti/vo2/activity"
	"github.com/gabrieleangeletti/vo2/database"
	"github.com/gabrieleangeletti/vo2/provider"
	"github.com/gabrieleangeletti/vo2/store"
)

type Handler struct {
	db    *sqlx.DB
	mux   *http.ServeMux
	store vo2.Store
}

func NewHandler(db *sqlx.DB) *Handler {
	h := &Handler{
		db:    db,
		mux:   http.NewServeMux(),
		store: store.NewStore(db),
	}

	h.mux.HandleFunc("GET /providers/strava/auth/callback", stravaAuthHandler(h.db))
	h.mux.HandleFunc("GET /providers/strava/webhook", stravaRegisterWebhookHandler(h.db))
	h.mux.HandleFunc("POST /providers/strava/webhook", stravaWebhookHandler(h.db, h.store))

	h.mux.HandleFunc("GET /athletes/{athleteID}/metrics/volume", athleteVolumeHandler(h.db, h.store))

	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) ProcessHistoricalDataTask(ctx context.Context, task HistoricalDataTask) error {
	prov, err := provider.GetByID(h.db, task.ProviderID)
	if err != nil {
		return fmt.Errorf("failed to get provider: %w", err)
	}

	// TODO: make provider agnostic
	driver := NewStravaDriver()

	credentials, err := ensureValidCredentials(ctx, h.db, driver, prov, task.UserID)
	if err != nil {
		return fmt.Errorf("failed to get valid credentials: %w", err)
	}

	client := driver.NewClient(credentials.AccessToken)

	activities, err := GetStravaActivitySummaries(client, task.StartTime, task.EndTime)
	if err != nil {
		return fmt.Errorf("failed to get Strava activities: %w", err)
	}

	if len(activities) == 0 {
		slog.Info("No activities found for time range", "userId", task.UserID, "startTime", task.StartTime, "endTime", task.EndTime)
		return nil
	}

	existingActivities, err := activity.GetProviderActivityRawData(ctx, h.db, prov.ID, task.UserID)
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

	slog.Info("Processing historical activities", "userId", task.UserID, "activityCount", len(activities), "startTime", task.StartTime, "endTime", task.EndTime)

	for _, act := range activities {
		// Note: this is the most basic way to handle rate limits. We basically go until we hit the limit.
		// Then we manually retry and ignore the activities that were already processed.
		// TODO: implement proper solution.
		if existing, ok := existingActivitiesMap[act.ID]; ok {
			if !existing.DetailedActivityURI.Valid {
				var detailedActivity strava.ActivityDetailed
				err := json.Unmarshal(existing.Data, &detailedActivity)
				if err != nil {
					return fmt.Errorf("failed to unmarshal activity: %w", err)
				}

				streams, err := client.GetActivityStreams(detailedActivity.ID)
				if err != nil {
					return fmt.Errorf("failed to get activity streams: %w", err)
				}

				err = UploadRawActivityDetails(ctx, h.db, prov.Slug, existing, streams)
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
			UserID:             task.UserID,
			ProviderActivityID: strconv.FormatInt(detailedActivity.ID, 10),
			StartTime:          act.StartDate,
			ElapsedTime:        act.ElapsedTime,
			IanaTimezone:       database.ToNullString(detailedActivity.IanaTimezone()),
			Data:               data,
		}

		err = h.store.SaveProviderActivityRawData(ctx, &activityRaw)
		if err != nil {
			return fmt.Errorf("failed to save activity: %w", err)
		}

		streams, err := client.GetActivityStreams(detailedActivity.ID)
		if err != nil {
			return fmt.Errorf("failed to get activity streams: %w", err)
		}

		err = UploadRawActivityDetails(ctx, h.db, prov.Slug, &activityRaw, streams)
		if err != nil {
			return fmt.Errorf("failed to upload raw activity details: %w", err)
		}

		return nil
	}

	slog.Info("Successfully processed historical activities", "userId", task.UserID, "processedCount", len(activities))

	return nil
}

func stravaAuthHandler(db *sqlx.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
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
			GetSecret("STRAVA_CLIENT_ID", true),
			GetSecret("STRAVA_CLIENT_SECRET", true),
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

		if err := queueHistoricalDataTasks(r.Context(), user.ID, prov.ID, HistoricalDataTaskTypeActivity); err != nil {
			slog.Error("Failed to queue historical data tasks", "error", err, "userId", user.ID)
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

func stravaWebhookHandler(db *sqlx.DB, store vo2.Store) func(http.ResponseWriter, *http.Request) {
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

		providerMap, err := provider.GetMap(ctx, db)
		if err != nil {
			slog.Error(err.Error())
			http.Error(w, ErrGeneric.Error(), http.StatusInternalServerError)
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

		driver := NewStravaDriver()

		credentials, err := ensureValidCredentials(r.Context(), db, driver, prov, user.ID)
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
					UserID:             user.ID,
					ProviderActivityID: strconv.FormatInt(event.ObjectID, 10),
					StartTime:          stravaActivity.StartDate,
					ElapsedTime:        stravaActivity.ElapsedTime,
					IanaTimezone:       database.ToNullString(stravaActivity.IanaTimezone()),
					Data:               data,
				}

				err = store.SaveProviderActivityRawData(ctx, &activityRaw)
				if err != nil {
					slog.Error(err.Error())
					http.Error(w, ErrGeneric.Error(), http.StatusInternalServerError)
					return
				}

				err = UploadRawActivityDetails(ctx, db, prov.Slug, &activityRaw, streams)
				if err != nil {
					slog.Error(err.Error())
					http.Error(w, ErrGeneric.Error(), http.StatusInternalServerError)
					return
				}

				act, err := activityRaw.ToEnduranceOutdoorActivity(providerMap)
				if err != nil {
					if !(errors.Is(err, stride.ErrActivityIsNotOutdoorEndurance) || errors.Is(err, stride.ErrUnsupportedSportType)) {
						slog.Error(err.Error())
						http.Error(w, ErrGeneric.Error(), http.StatusInternalServerError)
						return
					}
				}

				upsertedAct, err := store.UpsertActivityEnduranceOutdoor(ctx, act)
				if err != nil {
					slog.Error(err.Error())
					http.Error(w, ErrGeneric.Error(), http.StatusInternalServerError)
					return
				}

				act.ID = upsertedAct.ID

				tags := act.ExtractActivityTags()

				if len(tags) > 0 {
					err = store.UpsertTagsAndLinkActivity(ctx, act, tags)
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

func queueHistoricalDataTasks(ctx context.Context, userID uuid.UUID, providerID int, taskType HistoricalDataTaskType) error {
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
			UserID:     userID,
			ProviderID: providerID,
			Type:       taskType,
			StartTime:  monthStart,
			EndTime:    monthEnd,
		}

		if err := sqsClient.SendHistoricalDataTask(ctx, task); err != nil {
			return fmt.Errorf("failed to send historical data task: %w", err)
		}

		slog.Info("Queued historical data task", "userId", userID, "startTime", monthStart, "endTime", monthEnd)
	}

	return nil
}

func athleteVolumeHandler(db *sqlx.DB, store vo2.Store) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		athleteIDStr := r.PathValue("athleteID")
		athleteID, err := uuid.Parse(athleteIDStr)
		if err != nil {
			http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
			return
		}

		user, err := GetUserByID(ctx, db, athleteID)
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
		sport := r.URL.Query().Get("sport")

		if provider == "" || frequency == "" || startDate == "" || sport == "" {
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

		volumeParams := vo2.GetAthleteVolumeParams{
			Frequency:    frequency,
			UserID:       user.ID,
			ProviderSlug: provider,
			Sport:        sport,
			StartDate:    startDateTime,
		}

		volumeData, err := store.GetAthleteVolume(ctx, volumeParams)
		if err != nil {
			slog.Error("Failed to get athlete volume", "error", err, "params", volumeParams)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		response := map[string]any{
			"userId":    user.ID,
			"provider":  provider,
			"frequency": frequency,
			"sport":     sport,
			"startDate": startDateTime.Format("2006-01-02"),
			"data":      volumeData,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}
}
