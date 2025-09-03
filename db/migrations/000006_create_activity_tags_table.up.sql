CREATE TABLE vo2.activity_tags (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP,

    UNIQUE (name)
);

CREATE TRIGGER set_activity_tags_updated_time BEFORE
UPDATE
    ON vo2.activity_tags FOR EACH ROW EXECUTE PROCEDURE vo2.set_updated_at_timestamp();

CREATE TABLE vo2.activities_endurance_outdoor_tags (
    activity_id UUID NOT NULL,
    tag_id INT NOT NULL,

    PRIMARY KEY (activity_id, tag_id),

    FOREIGN KEY (activity_id) REFERENCES vo2.activities_endurance_outdoor (id) ON DELETE CASCADE,
    FOREIGN KEY (tag_id) REFERENCES vo2.activity_tags (id) ON DELETE CASCADE
);
