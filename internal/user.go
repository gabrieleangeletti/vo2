package internal

import (
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/gabrieleangeletti/vo2/database"
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

func GetUser(db database.IDB, providerID int, userExternalID string) (*User, error) {
	var u User

	err := db.Get(&u, "SELECT * FROM vo2.users WHERE provider_id = $1 AND user_external_id = $2", providerID, userExternalID)
	if err != nil {
		return nil, err
	}

	return &u, nil
}

func GetUserByID(db database.IDB, userID uuid.UUID) (*User, error) {
	var u User

	err := db.Get(&u, "SELECT * FROM vo2.users WHERE id = $1", userID)
	if err != nil {
		return nil, err
	}

	return &u, nil
}

func CreateUser(db database.IDB, providerID int, userExternalID string) (*User, error) {
	user := &User{
		ProviderID:     providerID,
		UserExternalID: userExternalID,
	}

	_, err := db.NamedExec(`
	INSERT INTO vo2.users (provider_id, user_external_id)
	VALUES (:provider_id, :user_external_id)
	ON CONFLICT
		(provider_id, user_external_id)
	DO NOTHING
		
	`, user)
	if err != nil {
		return nil, err
	}

	user, err = GetUser(db, providerID, userExternalID)
	if err != nil {
		return nil, err
	}

	return user, nil
}
