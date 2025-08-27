package internal

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type ProviderActivityRawData struct {
	ID                  uuid.UUID       `json:"id" db:"id"`
	ProviderID          int             `json:"providerId" db:"provider_id"`
	UserID              uuid.UUID       `json:"userId" db:"user_id"`
	ProviderActivityID  string          `json:"providerActivityId" db:"provider_activity_id"`
	StartTime           time.Time       `json:"startTime" db:"start_time"`
	ElapsedTime         int             `json:"elapsedTime" db:"elapsed_time"`
	Data                json.RawMessage `json:"data" db:"data"`
	DetailedActivityURI sql.NullString  `json:"detailedActivityUri" db:"detailed_activity_uri"`
	CreatedAt           time.Time       `json:"createdAt" db:"created_at"`
	UpdatedAt           sql.NullTime    `json:"updatedAt" db:"updated_at"`
	DeletedAt           sql.NullTime    `json:"deletedAt" db:"deleted_at"`
}

func (w *ProviderActivityRawData) Save(db *sqlx.DB) error {
	_, err := db.NamedExec(`
	INSERT INTO vo2.provider_activity_raw_data
		(provider_id, user_id, provider_activity_id, start_time, elapsed_time, data, detailed_activity_uri)
	VALUES
		(:provider_id, :user_id, :provider_activity_id, :start_time, :elapsed_time, :data, :detailed_activity_uri)
	ON CONFLICT
		(provider_id, user_id, provider_activity_id)
	DO UPDATE SET
		start_time = :start_time,
		elapsed_time = :elapsed_time,
		data = :data,
		detailed_activity_uri = :detailed_activity_uri
	`, w)
	if err != nil {
		return err
	}

	return nil
}
