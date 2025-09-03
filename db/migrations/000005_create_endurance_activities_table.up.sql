CREATE TABLE vo2.activities_endurance_outdoor (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    provider_id INT NOT NULL,
    user_id UUID NOT NULL,
    provider_raw_activity_id UUID NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    sport TEXT NOT NULL,
    start_time TIMESTAMPTZ NOT NULL,
    end_time   TIMESTAMPTZ NOT NULL,
    iana_timezone TEXT,
    utc_offset INT,
    elapsed_time INT NOT NULL,
    moving_time INT NOT NULL,
    distance INT NOT NULL,
    elev_gain INT,
    elev_loss INT,
    avg_speed FLOAT NOT NULL,
    avg_hr INT,
    max_hr INT,
    summary_polyline TEXT,
    summary_route postgis.geometry(LineString, 4326),

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP,

    UNIQUE (provider_id, user_id, provider_raw_activity_id),

    FOREIGN KEY (provider_id) REFERENCES vo2.providers (id),
    FOREIGN KEY (user_id) REFERENCES vo2.users (id),
    FOREIGN KEY (provider_raw_activity_id) REFERENCES vo2.provider_activity_raw_data (id),

    CHECK (iana_timezone IS NOT NULL OR utc_offset IS NOT NULL)
);

CREATE TRIGGER set_activities_endurance_outdoor_updated_time BEFORE
UPDATE
    ON vo2.activities_endurance_outdoor FOR EACH ROW EXECUTE PROCEDURE vo2.set_updated_at_timestamp();
