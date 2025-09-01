package internal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/gabrieleangeletti/stride/strava"
)

type Handler struct {
	db  *sqlx.DB
	mux *http.ServeMux
}

func NewHandler(db *sqlx.DB) *Handler {
	h := &Handler{
		db:  db,
		mux: http.NewServeMux(),
	}

	h.mux.HandleFunc("GET /providers/strava/auth/callback", stravaAuthHandler(h.db))
	h.mux.HandleFunc("GET /providers/strava/webhook", stravaRegisterWebhookHandler(h.db))
	h.mux.HandleFunc("POST /providers/strava/webhook", stravaWebhookHandler(h.db))

	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) ProcessHistoricalDataTask(ctx context.Context, task HistoricalDataTask) error {
	provider, err := GetProviderByID(h.db, task.ProviderID)
	if err != nil {
		return fmt.Errorf("failed to get provider: %w", err)
	}

	user, err := GetUserByID(h.db, task.UserID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	// TODO: make provider agnostic
	driver := NewStravaDriver()

	credentials, err := ensureValidCredentials(ctx, h.db, driver, provider, user)
	if err != nil {
		return fmt.Errorf("failed to get valid credentials: %w", err)
	}

	activities, err := GetStravaActivitySummaries(credentials, task.StartTime, task.EndTime)
	if err != nil {
		return fmt.Errorf("failed to get Strava activities: %w", err)
	}

	if len(activities) == 0 {
		slog.Info("No activities found for time range", "userId", task.UserID, "startTime", task.StartTime, "endTime", task.EndTime)
		return nil
	}

	slog.Info("Processing historical activities", "userId", task.UserID, "activityCount", len(activities), "startTime", task.StartTime, "endTime", task.EndTime)

	for _, activity := range activities {
		data, err := json.Marshal(activity)
		if err != nil {
			return fmt.Errorf("failed to marshal activity: %w", err)
		}

		activityData := ProviderActivityRawData{
			ProviderID:         provider.ID,
			UserID:             user.ID,
			ProviderActivityID: strconv.Itoa(activity.ID),
			StartTime:          activity.StartDate,
			ElapsedTime:        activity.ElapsedTime,
			Data:               data,
		}

		err = activityData.Save(h.db)
		if err != nil {
			return fmt.Errorf("failed to save activity: %w", err)
		}
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

		provider, err := GetProviderBySlug(db, "strava")
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

		user, err := CreateUser(tx, provider.ID, strconv.Itoa(tokenResponse.Athlete.ID))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		credentials := ProviderOAuth2Credentials{
			ProviderID:   provider.ID,
			UserID:       user.ID,
			AccessToken:  tokenResponse.AccessToken,
			RefreshToken: tokenResponse.RefreshToken,
			ExpiresAt:    time.Unix(int64(tokenResponse.ExpiresAt), 0),
		}

		err = credentials.Save(tx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			slog.Error(err.Error())
			http.Error(w, ErrGeneric.Error(), http.StatusInternalServerError)
			return
		}

		if err := queueHistoricalDataTasks(r.Context(), user.ID, provider.ID, HistoricalDataTaskTypeActivity); err != nil {
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

		isValid, err := verifyWebhook(db, verifyToken)
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

func stravaWebhookHandler(db *sqlx.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		slog.Info("Received Strava Webhook")

		var event strava.WebhookEvent
		err := json.NewDecoder(r.Body).Decode(&event)
		if err != nil {
			slog.Error(err.Error())
			http.Error(w, ErrGeneric.Error(), http.StatusBadRequest)
			return
		}

		provider, err := GetProviderBySlug(db, "strava")
		if err != nil {
			slog.Error(err.Error())
			http.Error(w, ErrGeneric.Error(), http.StatusBadRequest)
			return
		}

		user, err := GetUser(db, provider.ID, strconv.Itoa(event.OwnerID))
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				slog.Error(fmt.Sprintf("%s: (provider: %d, userExternalID: %d)", ErrUserNotFound, provider.ID, event.OwnerID))
				http.Error(w, ErrGeneric.Error(), http.StatusBadRequest)
				return
			}

			slog.Error(err.Error())
			http.Error(w, ErrGeneric.Error(), http.StatusBadRequest)
			return
		}

		driver := NewStravaDriver()

		credentials, err := ensureValidCredentials(r.Context(), db, driver, provider, user)
		if err != nil {
			slog.Error(err.Error())
			http.Error(w, ErrGeneric.Error(), http.StatusBadRequest)
			return
		}

		client := driver.NewClient(credentials.AccessToken)

		if event.ObjectType == strava.WebhookActivity {
			if event.AspectType == strava.WebhookCreate {
				stravaActivity, err := client.GetActivitySummary(event.ObjectID, false)
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

				activity := ProviderActivityRawData{
					ProviderID:         provider.ID,
					UserID:             user.ID,
					ProviderActivityID: strconv.Itoa(event.ObjectID),
					StartTime:          stravaActivity.StartDate,
					ElapsedTime:        stravaActivity.ElapsedTime,
					Data:               data,
				}

				err = activity.Save(db)
				if err != nil {
					slog.Error(err.Error())
					http.Error(w, ErrGeneric.Error(), http.StatusInternalServerError)
					return
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
