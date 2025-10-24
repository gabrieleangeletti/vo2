CREATE TABLE vo2.activities_endurance (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    provider_id INT NOT NULL,
    athlete_id UUID NOT NULL,
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

    UNIQUE (provider_id, athlete_id, provider_raw_activity_id),

    FOREIGN KEY (provider_id) REFERENCES vo2.providers (id),
    FOREIGN KEY (athlete_id) REFERENCES vo2.athletes (id),
    FOREIGN KEY (provider_raw_activity_id) REFERENCES vo2.provider_activity_raw_data (id),

    CHECK (iana_timezone IS NOT NULL OR utc_offset IS NOT NULL)
);

CREATE TRIGGER set_activities_endurance_updated_time BEFORE
UPDATE
    ON vo2.activities_endurance FOR EACH ROW EXECUTE PROCEDURE vo2.set_updated_at_timestamp();

COMMENT ON TABLE vo2.activities_endurance IS 'Endurance activities';

COMMENT ON COLUMN vo2.activities_endurance.name IS 'Name of the activity, as given by the original provider.';
COMMENT ON COLUMN vo2.activities_endurance.description IS 'Description of the activity, as given by the original provider.';
COMMENT ON COLUMN vo2.activities_endurance.sport IS 'Sport type of the activity. Endurance activities are: running, cycling, gravel cycling, hiking, trail running, elliptical, swimming, stair-stepper, and inline-skating.';
COMMENT ON COLUMN vo2.activities_endurance.start_time IS 'Start time of the activity. Measured in UTC.';
COMMENT ON COLUMN vo2.activities_endurance.end_time IS 'End time of the activity. Measured in UTC.';
COMMENT ON COLUMN vo2.activities_endurance.iana_timezone IS 'IANA timezone of the activity.';
COMMENT ON COLUMN vo2.activities_endurance.utc_offset IS 'UTC offset of the activity. Measured in seconds.';
COMMENT ON COLUMN vo2.activities_endurance.elapsed_time IS 'The overall elapsed time of the activity. Measured in seconds.';
COMMENT ON COLUMN vo2.activities_endurance.moving_time IS 'The overall moving time of the activity. Measured in seconds.';
COMMENT ON COLUMN vo2.activities_endurance.distance IS 'Distance of the activity. Measured in meters.';
COMMENT ON COLUMN vo2.activities_endurance.elev_gain IS 'Elevation gain of the activity. Measured in meters.';
COMMENT ON COLUMN vo2.activities_endurance.elev_loss IS 'Elevation loss of the activity. Measured in meters.';
COMMENT ON COLUMN vo2.activities_endurance.avg_speed IS 'Average speed of the activity. Measured in meters per second.';
COMMENT ON COLUMN vo2.activities_endurance.avg_hr IS 'Average heart rate during the activity. Measured in beats per minute.';
COMMENT ON COLUMN vo2.activities_endurance.max_hr IS 'Maximum heart rate during the activity. Measured in beats per minute.';
COMMENT ON COLUMN vo2.activities_endurance.summary_polyline IS 'Summary polyline of the activity. Encoded in Google Polyline format.';
COMMENT ON COLUMN vo2.activities_endurance.summary_route IS 'Summary postgis geometry route of the activity.';
COMMENT ON COLUMN vo2.activities_endurance.gpx_file_uri IS 'URI of the GPX file of the activity.';
COMMENT ON COLUMN vo2.activities_endurance.fit_file_uri IS 'URI of the FIT file of the activity.';
