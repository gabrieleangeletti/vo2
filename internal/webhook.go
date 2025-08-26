package internal

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
)

var (
	ErrWebhookVerificationExpired = errors.New("webhook verification expired")
)

type WebhookVerification struct {
	ID        int       `json:"id" db:"id"`
	Token     string    `json:"token" db:"token"`
	CreatedAt time.Time `json:"createdAt" db:"created_at"`
	ExpiresAt time.Time `json:"expiresAt" db:"expires_at"`
}

func CreateWebhookVerification(db *sqlx.DB) (*WebhookVerification, error) {
	token, err := generateVerificationToken()
	if err != nil {
		return nil, err
	}

	now := time.Now()

	verification := &WebhookVerification{
		Token:     token,
		CreatedAt: now,
		ExpiresAt: now.Add(5 * time.Minute),
	}

	_, err = db.NamedExec(`
	INSERT INTO vo2.webhook_verifications (token, created_at, expires_at)
	VALUES (:token, :created_at, :expires_at)
	`, verification)
	if err != nil {
		return nil, err
	}

	return verification, nil
}

func verifyWebhook(db *sqlx.DB, token string) (bool, error) {
	var verification WebhookVerification

	err := db.Get(&verification, "SELECT * FROM vo2.webhook_verifications WHERE token = $1", token)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}

		return false, err
	}

	if verification.ExpiresAt.After(time.Now()) {
		return false, ErrWebhookVerificationExpired
	}

	_, err = db.NamedExec("DELETE FROM vo2.webhook_verifications WHERE token = $1", token)
	if err != nil {
		return false, err
	}

	return true, nil
}

func generateVerificationToken() (string, error) {
	bytes := make([]byte, 32) // 256 bits
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return hex.EncodeToString(bytes), nil
}
