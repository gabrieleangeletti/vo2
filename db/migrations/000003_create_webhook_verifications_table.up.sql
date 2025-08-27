CREATE TABLE vo2.webhook_verifications (
    id SERIAL PRIMARY KEY,
    token VARCHAR(255) NOT NULL,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL,

    UNIQUE ("token")
);