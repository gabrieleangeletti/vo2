CREATE TABLE vo2.provider_activity_raw_data (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    provider_id INT NOT NULL,
    user_id UUID NOT NULL,
    provider_activity_id VARCHAR(255) NOT NULL,
    start_time TIMESTAMP NOT NULL,
    elapsed_time INT NOT NULL,
    "data" JSONB NOT NULL,
    detailed_activity_uri TEXT,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP,

    UNIQUE (provider_id, user_id, provider_activity_id),

    FOREIGN KEY (provider_id) REFERENCES vo2.providers (id),
    FOREIGN KEY (user_id) REFERENCES vo2.users (id)
);

CREATE TRIGGER set_provider_activity_raw_data_updated_time BEFORE
UPDATE
    ON vo2.provider_activity_raw_data FOR EACH ROW EXECUTE PROCEDURE vo2.set_updated_at_timestamp();
