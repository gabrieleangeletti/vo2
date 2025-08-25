-- Create utility functions in the vo2 schema.

CREATE OR REPLACE FUNCTION vo2.set_updated_at_timestamp() RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = current_timestamp;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Providers

CREATE TYPE vo2.provider_connection_type AS ENUM (
    'oauth2'
);

CREATE TABLE vo2.providers (
    id SERIAL PRIMARY KEY,
    "name" VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL,
    connection_type vo2.provider_connection_type NOT NULL,
    "description" TEXT,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP,

    UNIQUE ("slug")
);

CREATE TRIGGER set_providers_updated_time BEFORE
UPDATE
    ON vo2.providers FOR EACH ROW EXECUTE PROCEDURE vo2.set_updated_at_timestamp();

INSERT INTO vo2.providers ("name", slug, connection_type, "description") VALUES
('Strava', 'strava', 'oauth2', 'Strava | Running, Cycling & Hiking App - Train, Track & Share') ON CONFLICT (slug)
DO UPDATE SET name = EXCLUDED.name, slug = EXCLUDED.slug, connection_type = EXCLUDED.connection_type, description = EXCLUDED.description;

CREATE TABLE vo2.provider_credentials (
    id SERIAL PRIMARY KEY,
    provider_id INT NOT NULL,
    user_external_id VARCHAR(255) NOT NULL,
    "credentials" JSONB NOT NULL,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP,

    UNIQUE (provider_id, user_external_id),

    FOREIGN KEY (provider_id) REFERENCES vo2.providers (id)
);

CREATE TRIGGER set_provider_credentials_updated_time BEFORE
UPDATE
    ON vo2.provider_credentials FOR EACH ROW EXECUTE PROCEDURE vo2.set_updated_at_timestamp();
