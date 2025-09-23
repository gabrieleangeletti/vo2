package internal

import (
	"context"
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

func CreateWebhookVerification(ctx context.Context, db *sqlx.DB) (*WebhookVerification, error) {
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

	_, err = db.ExecContext(ctx, `
	INSERT INTO vo2.webhook_verifications (token, created_at, expires_at)
	VALUES ($1, $2, $3)
	`, verification.Token, verification.CreatedAt, verification.ExpiresAt)
	if err != nil {
		return nil, err
	}

	return verification, nil
}

func DeleteWebhookVerification(ctx context.Context, db *sqlx.DB, v *WebhookVerification) error {
	_, err := db.ExecContext(ctx, "DELETE FROM vo2.webhook_verifications WHERE token = $1", v.Token)
	if err != nil {
		return err
	}

	return nil
}

func verifyWebhook(ctx context.Context, db *sqlx.DB, token string) (bool, error) {
	var verification WebhookVerification

	err := db.GetContext(ctx, &verification, "SELECT * FROM vo2.webhook_verifications WHERE token = $1", token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}

		return false, err
	}

	if time.Now().After(verification.ExpiresAt) {
		return false, ErrWebhookVerificationExpired
	}

	_, err = db.ExecContext(ctx, "DELETE FROM vo2.webhook_verifications WHERE token = $1", verification.Token)
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
