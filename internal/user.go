package internal

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

var (
	ErrUserNotFound = errors.New("user not found")
)

type User struct {
	ID             uuid.UUID    `json:"id" db:"id"`
	ProviderID     int          `json:"provider_id" db:"provider_id"`
	UserExternalID string       `json:"user_external_id" db:"user_external_id"`
	CreatedAt      time.Time    `json:"createdAt" db:"created_at"`
	UpdatedAt      sql.NullTime `json:"updatedAt" db:"updated_at"`
	DeletedAt      sql.NullTime `json:"deletedAt" db:"deleted_at"`
}

func GetUser(db *sqlx.DB, providerID int, userExternalID string) (*User, error) {
	var u User

	err := db.Get(&u, "SELECT * FROM vo2.users WHERE provider_id = $1 AND user_external_id = $2", providerID, userExternalID)
	if err != nil {
		return nil, err
	}

	return &u, nil
}

func GetUserByID(ctx context.Context, db *sqlx.DB, userID uuid.UUID) (*User, error) {
	var u User

	err := db.GetContext(ctx, &u, "SELECT * FROM vo2.users WHERE id = $1", userID)
	if err != nil {
		return nil, err
	}

	return &u, nil
}

func GetAthleteUser(ctx context.Context, db *sqlx.DB, athleteID uuid.UUID) (*User, error) {
	var u User

	err := db.GetContext(ctx, &u, "SELECT * FROM vo2.users WHERE id = (select user_id from vo2.athletes where id = $1)", athleteID)
	if err != nil {
		return nil, err
	}

	return &u, nil
}

func CreateUser(tx *sqlx.Tx, providerID int, userExternalID string) (*User, error) {
	user := &User{
		ProviderID:     providerID,
		UserExternalID: userExternalID,
	}

	_, err := tx.Exec(`
	INSERT INTO vo2.users (provider_id, user_external_id)
	VALUES ($1, $2)
	ON CONFLICT
		(provider_id, user_external_id)
	DO NOTHING
	`, user.ProviderID, user.UserExternalID)
	if err != nil {
		return nil, err
	}

	err = tx.Get(user, "SELECT * FROM vo2.users WHERE provider_id = $1 AND user_external_id = $2", providerID, userExternalID)
	if err != nil {
		return nil, err
	}

	return user, nil
}
