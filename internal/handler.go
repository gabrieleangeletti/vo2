package internal

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

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

		provider, err := GetProvider(db, "strava")
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

		tx.Commit()

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
		if challenge == "" {
			http.Error(w, "invalid hub.challenge", http.StatusBadRequest)
			return
		}

		isValid, err := verifyWebhook(db, verifyToken)
		if err != nil {
			http.Error(w, "error while validating token: "+err.Error(), http.StatusInternalServerError)
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

		provider, err := GetProvider(db, "strava")
		if err != nil {
			slog.Error(err.Error())
			http.Error(w, ErrGeneric.Error(), http.StatusInternalServerError)
			return
		}

		user, err := GetUser(db, provider.ID, strconv.Itoa(event.OwnerID))
		if err != nil {
			if err == sql.ErrNoRows {
				slog.Error(fmt.Sprintf("User %d not found", event.OwnerID))
				http.Error(w, ErrGeneric.Error(), http.StatusNotFound)
				return
			}

			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		credentials, err := GetProviderOAuth2Credentials(db, provider.ID, user.ID)
		if err != nil {
			slog.Error(err.Error())
			http.Error(w, ErrGeneric.Error(), http.StatusInternalServerError)
			return
		}

		if err := refreshOAuth2CredentialsIfExpired(db, provider, credentials); err != nil {
			slog.Error(err.Error())
			http.Error(w, ErrGeneric.Error(), http.StatusInternalServerError)
			return
		}

		client := strava.NewClient(credentials.AccessToken)

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
