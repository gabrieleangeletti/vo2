package internal

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

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

	if err := stravaAuthHandler(h); err != nil {
		log.Fatal(err)
	}

	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func stravaAuthHandler(h *Handler) error {
	h.mux.HandleFunc("/providers/auth/strava/callback", func(w http.ResponseWriter, r *http.Request) {
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

		credentialsRaw, err := json.Marshal(tokenResponse)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		provider, err := GetProvider(h.db, "strava")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		credentials := ProviderCredentials{
			ProviderID:     provider.ID,
			UserExternalID: strconv.Itoa(tokenResponse.Athlete.ID),
			Credentials:    credentialsRaw,
		}

		err = credentials.Save(h.db)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		resp := map[string]bool{"success": true}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		json.NewEncoder(w).Encode(resp)
	})

	return nil
}
