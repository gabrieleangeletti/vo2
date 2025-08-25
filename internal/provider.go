package internal

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/jmoiron/sqlx"
)

type ConnectionType string

const (
	OAuth2ConnectionType ConnectionType = "oauth2"
)

type Provider struct {
	ID             int            `json:"id" db:"id"`
	Name           string         `json:"name" db:"name"`
	Slug           string         `json:"slug" db:"slug"`
	ConnectionType ConnectionType `json:"connection_type" db:"connection_type"`
	Description    string         `json:"description" db:"description"`
	CreatedAt      time.Time      `json:"createdAt" db:"created_at"`
	UpdatedAt      sql.NullTime   `json:"updatedAt" db:"updated_at"`
	DeletedAt      sql.NullTime   `json:"deletedAt" db:"deleted_at"`
}

func GetProvider(db *sqlx.DB, slug string) (*Provider, error) {
	var provider Provider

	err := db.Get(&provider, "SELECT * FROM vo2.providers WHERE slug = $1", slug)
	if err != nil {
		return nil, err
	}

	return &provider, nil
}

type ProviderCredentials struct {
	ID             int             `json:"id" db:"id"`
	ProviderID     int             `json:"providerId" db:"provider_id"`
	UserExternalID string          `json:"userExternalId" db:"user_external_id"`
	Credentials    json.RawMessage `json:"credentials" db:"credentials"`
	CreatedAt      time.Time       `json:"createdAt" db:"created_at"`
	UpdatedAt      sql.NullTime    `json:"updatedAt" db:"updated_at"`
	DeletedAt      sql.NullTime    `json:"deletedAt" db:"deleted_at"`
}

func (c *ProviderCredentials) Save(db *sqlx.DB) error {
	var err error

	_, err = db.NamedExec(`
	INSERT INTO vo2.provider_credentials (provider_id, user_external_id, credentials)
	VALUES (:provider_id, :user_external_id, :credentials)
	ON CONFLICT (provider_id, user_external_id) DO UPDATE SET credentials = :credentials
	`, c)
	if err != nil {
		return err
	}

	return nil
}
