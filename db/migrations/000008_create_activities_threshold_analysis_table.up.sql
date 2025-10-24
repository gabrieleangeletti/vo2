CREATE TABLE vo2.activities_threshold_analysis (
    id SERIAL PRIMARY KEY,
    activity_endurance_id UUID NOT NULL,
    time_at_lt1_threshold INT NOT NULL,
    time_at_lt2_threshold INT NOT NULL,
    raw_analysis JSONB NOT NULL,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP,

    UNIQUE (activity_endurance_id),

    FOREIGN KEY (activity_endurance_id) REFERENCES vo2.activities_endurance(id)
);

CREATE TRIGGER set_activities_threshold_analysis_updated_time BEFORE
UPDATE
    ON vo2.activities_threshold_analysis FOR EACH ROW EXECUTE PROCEDURE vo2.set_updated_at_timestamp();
