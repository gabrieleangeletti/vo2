CREATE TYPE gender AS ENUM ('male', 'female', 'other');

CREATE TABLE vo2.athletes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL,
    age SMALLINT NOT NULL,
    height_cm SMALLINT NOT NULL,
    country CHAR(2) NOT NULL,
    gender gender NOT NULL,
    first_name VARCHAR(255) NOT NULL,
    last_name VARCHAR(255) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP,

    UNIQUE(email),

    FOREIGN KEY (user_id) REFERENCES vo2.users(id)
);

CREATE TRIGGER set_athletes_updated_time BEFORE
UPDATE
    ON vo2.athletes FOR EACH ROW EXECUTE PROCEDURE vo2.set_updated_at_timestamp();

CREATE INDEX idx_athletes_user_id ON vo2.athletes(user_id);

CREATE TYPE vo2.athlete_measurement_type AS ENUM (
    'lt1',
    'lt2',
    'vo2max',
    'weight'
);

CREATE TYPE vo2.athlete_measurement_source AS ENUM (
    'lab_test',
    'field_test',
    'manual'
);

CREATE TABLE vo2.athlete_measurement_history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    athlete_id UUID NOT NULL,
    measured_at TIMESTAMPTZ NOT NULL,
    iana_timezone VARCHAR(255) NOT NULL,
    metric_type vo2.athlete_measurement_type NOT NULL,
    value NUMERIC(10, 2) NOT NULL,
    source vo2.athlete_measurement_source NOT NULL,
    notes TEXT,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP,

    UNIQUE(athlete_id, measured_at, iana_timezone, metric_type),

    FOREIGN KEY (athlete_id) REFERENCES vo2.athletes(id)
);

CREATE TRIGGER set_athlete_measurement_history_updated_time BEFORE
UPDATE
    ON vo2.athlete_measurement_history FOR EACH ROW EXECUTE PROCEDURE vo2.set_updated_at_timestamp();

CREATE INDEX idx_athlete_measurement_history_lookup ON vo2.athlete_measurement_history(athlete_id, metric_type, measured_at DESC);

CREATE OR REPLACE VIEW vo2.athlete_current_measurements AS
WITH latest AS (
    SELECT DISTINCT ON (athlete_id, metric_type)
        athlete_id,
        metric_type,
        value,
        measured_at,
        iana_timezone,
        source,
        notes
    FROM vo2.athlete_measurement_history
    WHERE metric_type IN ('lt1', 'lt2', 'vo2max', 'weight')
      AND deleted_at IS NULL
    ORDER BY athlete_id, metric_type, measured_at DESC
)
SELECT
    a.athlete_id,

    lt1.value AS lt1_value,
    lt1.measured_at AS lt1_measured_at,
    lt1.iana_timezone AS lt1_timezone,
    lt1.source AS lt1_source,
    lt1.notes AS lt1_notes,

    lt2.value AS lt2_value,
    lt2.measured_at AS lt2_measured_at,
    lt2.iana_timezone AS lt2_timezone,
    lt2.source AS lt2_source,
    lt2.notes AS lt2_notes,

    vo2max.value AS vo2max_value,
    vo2max.measured_at AS vo2max_measured_at,
    vo2max.iana_timezone AS vo2max_timezone,
    vo2max.source AS vo2max_source,
    vo2max.notes AS vo2max_notes,

    weight.value AS weight_value,
    weight.measured_at AS weight_measured_at,
    weight.iana_timezone AS weight_timezone,
    weight.source AS weight_source,
    weight.notes AS weight_notes

FROM (SELECT DISTINCT athlete_id FROM latest) a
LEFT JOIN latest lt1     ON lt1.athlete_id = a.athlete_id AND lt1.metric_type = 'lt1'
LEFT JOIN latest lt2     ON lt2.athlete_id = a.athlete_id AND lt2.metric_type = 'lt2'
LEFT JOIN latest vo2max  ON vo2max.athlete_id = a.athlete_id AND vo2max.metric_type = 'vo2max'
LEFT JOIN latest weight  ON weight.athlete_id = a.athlete_id AND weight.metric_type = 'weight';
