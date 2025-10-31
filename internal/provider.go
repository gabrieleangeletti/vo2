package internal

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/gabrieleangeletti/stride/strava"
	"github.com/gabrieleangeletti/vo2/provider"
	"github.com/gabrieleangeletti/vo2/util"
)

const (
	refreshTokenBuffer = 5 * time.Minute
)

type OAuth2Token struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

type ProviderDriver[C any, A any] interface {
	NewClient(accessToken string) C
	NewAuth() A
	RefreshToken(ctx context.Context, refreshToken string) (*OAuth2Token, error)
}

type StravaDriver struct {
	clientID     string
	clientSecret string
}

func NewStravaDriver() *StravaDriver {
	return &StravaDriver{
		clientID:     util.GetSecret("STRAVA_CLIENT_ID", true),
		clientSecret: util.GetSecret("STRAVA_CLIENT_SECRET", true),
	}
}

func (d *StravaDriver) NewClient(accessToken string) *strava.Client {
	return strava.NewClient(accessToken)
}

func (d *StravaDriver) NewAuth() *strava.Auth {
	return strava.NewAuth(d.clientID, d.clientSecret)
}

func (d *StravaDriver) RefreshToken(ctx context.Context, refreshToken string) (*OAuth2Token, error) {
	auth := strava.NewAuth(d.clientID, d.clientSecret)

	tokenResponse, err := auth.RefreshToken(refreshToken)
	if err != nil {
		return nil, err
	}

	token := OAuth2Token{
		AccessToken:  tokenResponse.AccessToken,
		RefreshToken: tokenResponse.RefreshToken,
		ExpiresAt:    time.Unix(int64(tokenResponse.ExpiresAt), 0),
	}

	return &token, nil
}

func ensureValidCredentials[C any, A any](ctx context.Context, db *sqlx.DB, driver ProviderDriver[C, A], prov *provider.Provider, athleteID uuid.UUID) (*ProviderOAuth2Credentials, error) {
	user, err := GetAthleteUser(ctx, db, athleteID)
	if err != nil {
		return nil, err
	}

	credentials, err := GetProviderOAuth2Credentials(db, prov.ID, user.ID)
	if err != nil {
		return nil, err
	}

	if credentials.Expired(refreshTokenBuffer) {
		tx, err := db.Beginx()
		if err != nil {
			return nil, err
		}
		defer tx.Rollback()

		err = tx.Get(credentials, "SELECT * FROM vo2.provider_oauth2_credentials WHERE provider_id = $1 AND user_id = $2 FOR UPDATE", prov.ID, user.ID)
		if err != nil {
			return nil, err
		}

		refreshed, err := refreshIfExpired(ctx, driver, credentials)
		if err != nil {
			return nil, err
		}

		if refreshed {
			if err := credentials.SaveTx(ctx, tx); err != nil {
				return nil, err
			}
		}

		if err := tx.Commit(); err != nil {
			return nil, err
		}
	}

	return credentials, nil
}

type ProviderOAuth2Credentials struct {
	ID           int          `json:"id" db:"id"`
	ProviderID   int          `json:"providerId" db:"provider_id"`
	UserID       uuid.UUID    `json:"userId" db:"user_id"`
	AccessToken  string       `json:"accessToken" db:"access_token"`
	RefreshToken string       `json:"refresh_Token" db:"refresh_token"`
	ExpiresAt    time.Time    `json:"expiresAt" db:"expires_at"`
	CreatedAt    time.Time    `json:"createdAt" db:"created_at"`
	UpdatedAt    sql.NullTime `json:"updatedAt" db:"updated_at"`
	DeletedAt    sql.NullTime `json:"deletedAt" db:"deleted_at"`
}

func (c *ProviderOAuth2Credentials) Expired(buffer time.Duration) bool {
	return time.Now().Add(-buffer).After(c.ExpiresAt)
}

func (c *ProviderOAuth2Credentials) Save(ctx context.Context, db *sqlx.DB) error {
	_, err := db.ExecContext(ctx, `
	INSERT INTO vo2.provider_oauth2_credentials
		(provider_id, user_id, access_token, refresh_token, expires_at)
	VALUES
		($1, $2, $3, $4, $5)
	ON CONFLICT
		(provider_id, user_id)
	DO UPDATE SET
		access_token = $3, refresh_token = $4, expires_at = $5
	`, c.ProviderID, c.UserID, c.AccessToken, c.RefreshToken, c.ExpiresAt)
	if err != nil {
		return err
	}

	return nil
}

func (c *ProviderOAuth2Credentials) SaveTx(ctx context.Context, tx *sqlx.Tx) error {
	_, err := tx.ExecContext(ctx, `
	INSERT INTO vo2.provider_oauth2_credentials
		(provider_id, user_id, access_token, refresh_token, expires_at)
	VALUES
		($1, $2, $3, $4, $5)
	ON CONFLICT
		(provider_id, user_id)
	DO UPDATE SET
		access_token = $3, refresh_token = $4, expires_at = $5
	`, c.ProviderID, c.UserID, c.AccessToken, c.RefreshToken, c.ExpiresAt)
	if err != nil {
		return err
	}

	return nil
}

func GetProviderOAuth2Credentials(db *sqlx.DB, providerID int, userID uuid.UUID) (*ProviderOAuth2Credentials, error) {
	var credentials ProviderOAuth2Credentials

	err := db.Get(&credentials, "SELECT * FROM vo2.provider_oauth2_credentials WHERE provider_id = $1 AND user_id = $2", providerID, userID)
	if err != nil {
		return nil, err
	}

	return &credentials, nil
}

func refreshIfExpired[C any, A any](ctx context.Context, provider ProviderDriver[C, A], credentials *ProviderOAuth2Credentials) (bool, error) {
	if credentials.Expired(refreshTokenBuffer) {
		newToken, err := provider.RefreshToken(ctx, credentials.RefreshToken)
		if err != nil {
			return false, err
		}

		credentials.AccessToken = newToken.AccessToken
		credentials.RefreshToken = newToken.RefreshToken
		credentials.ExpiresAt = newToken.ExpiresAt

		return true, nil
	}

	return false, nil
}
