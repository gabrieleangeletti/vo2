package provider

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
)

var (
	ErrProviderNotFound    = errors.New("provider not found")
	ErrUnsupportedProvider = errors.New("unsupported provider")
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

type Data struct {
	ID   int    `db:"id"`
	Name string `db:"name"`
	Slug string `db:"slug"`
}

func GetByID(db *sqlx.DB, id int) (*Provider, error) {
	var provider Provider

	err := db.Get(&provider, "SELECT * FROM vo2.providers WHERE id = $1", id)
	if err != nil {
		return nil, err
	}

	return &provider, nil
}

func GetBySlug(db *sqlx.DB, slug string) (*Provider, error) {
	var provider Provider

	err := db.Get(&provider, "SELECT * FROM vo2.providers WHERE slug = $1", slug)
	if err != nil {
		return nil, err
	}

	return &provider, nil
}

func GetMap(ctx context.Context, db *sqlx.DB) (map[int]Provider, error) {
	var providers []Provider
	err := db.SelectContext(ctx, &providers, "SELECT * FROM vo2.providers")
	if err != nil {
		return nil, err
	}

	providerMap := make(map[int]Provider)
	for _, provider := range providers {
		providerMap[provider.ID] = provider
	}

	return providerMap, nil
}
