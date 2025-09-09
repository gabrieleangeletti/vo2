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
    gpx_file_uri TEXT,
    fit_file_uri TEXT,

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

COMMENT ON TABLE vo2.activities_endurance_outdoor IS 'Endurance activities performed outdoors';

COMMENT ON COLUMN vo2.activities_endurance_outdoor.name IS 'Name of the activity, as given by the original provider.';
COMMENT ON COLUMN vo2.activities_endurance_outdoor.description IS 'Description of the activity, as given by the original provider.';
COMMENT ON COLUMN vo2.activities_endurance_outdoor.sport IS 'Sport type of the activity. Endurance outdoor activities are: running, cycling, gravel cycling, hiking, and trail running.';
COMMENT ON COLUMN vo2.activities_endurance_outdoor.start_time IS 'Start time of the activity. Measured in UTC.';
COMMENT ON COLUMN vo2.activities_endurance_outdoor.end_time IS 'End time of the activity. Measured in UTC.';
COMMENT ON COLUMN vo2.activities_endurance_outdoor.iana_timezone IS 'IANA timezone of the activity.';
COMMENT ON COLUMN vo2.activities_endurance_outdoor.utc_offset IS 'UTC offset of the activity. Measured in seconds.';
COMMENT ON COLUMN vo2.activities_endurance_outdoor.elapsed_time IS 'The overall elapsed time of the activity. Measured in seconds.';
COMMENT ON COLUMN vo2.activities_endurance_outdoor.moving_time IS 'The overall moving time of the activity. Measured in seconds.';
COMMENT ON COLUMN vo2.activities_endurance_outdoor.distance IS 'Distance of the activity. Measured in meters.';
COMMENT ON COLUMN vo2.activities_endurance_outdoor.elev_gain IS 'Elevation gain of the activity. Measured in meters.';
COMMENT ON COLUMN vo2.activities_endurance_outdoor.elev_loss IS 'Elevation loss of the activity. Measured in meters.';
COMMENT ON COLUMN vo2.activities_endurance_outdoor.avg_speed IS 'Average speed of the activity. Measured in meters per second.';
COMMENT ON COLUMN vo2.activities_endurance_outdoor.avg_hr IS 'Average heart rate during the activity. Measured in beats per minute.';
COMMENT ON COLUMN vo2.activities_endurance_outdoor.max_hr IS 'Maximum heart rate during the activity. Measured in beats per minute.';
COMMENT ON COLUMN vo2.activities_endurance_outdoor.summary_polyline IS 'Summary polyline of the activity. Encoded in Google Polyline format.';
COMMENT ON COLUMN vo2.activities_endurance_outdoor.summary_route IS 'Summary postgis geometry route of the activity.';
COMMENT ON COLUMN vo2.activities_endurance_outdoor.gpx_file_uri IS 'URI of the GPX file of the activity.';
COMMENT ON COLUMN vo2.activities_endurance_outdoor.fit_file_uri IS 'URI of the FIT file of the activity.';
