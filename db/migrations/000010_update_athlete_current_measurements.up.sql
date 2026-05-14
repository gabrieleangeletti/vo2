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
    WHERE metric_type IN ('lt1', 'lt2', 'vo2max', 'weight', 'max_hr', 'resting_hr')
      AND deleted_at IS NULL
    ORDER BY athlete_id, metric_type, measured_at DESC
)
SELECT
    a.athlete_id,

    lt1.value AS lt1_value,
    lt1.measured_at AS lt1_measured_at,
    lt1.iana_timezone AS lt1_iana_timezone,
    lt1.source AS lt1_source,
    lt1.notes AS lt1_notes,

    lt2.value AS lt2_value,
    lt2.measured_at AS lt2_measured_at,
    lt2.iana_timezone AS lt2_iana_timezone,
    lt2.source AS lt2_source,
    lt2.notes AS lt2_notes,

    vo2max.value AS vo2max_value,
    vo2max.measured_at AS vo2max_measured_at,
    vo2max.iana_timezone AS vo2max_iana_timezone,
    vo2max.source AS vo2max_source,
    vo2max.notes AS vo2max_notes,

    weight.value AS weight_value,
    weight.measured_at AS weight_measured_at,
    weight.iana_timezone AS weight_iana_timezone,
    weight.source AS weight_source,
    weight.notes AS weight_notes,

    max_hr.value AS max_hr_value,
    max_hr.measured_at AS max_hr_measured_at,
    max_hr.iana_timezone AS max_hr_iana_timezone,
    max_hr.source AS max_hr_source,
    max_hr.notes AS max_hr_notes,

    resting_hr.value AS resting_hr_value,
    resting_hr.measured_at AS resting_hr_measured_at,
    resting_hr.iana_timezone AS resting_hr_iana_timezone,
    resting_hr.source AS resting_hr_source,
    resting_hr.notes AS resting_hr_notes

FROM (SELECT DISTINCT athlete_id FROM latest) a
LEFT JOIN latest lt1        ON lt1.athlete_id = a.athlete_id AND lt1.metric_type = 'lt1'
LEFT JOIN latest lt2        ON lt2.athlete_id = a.athlete_id AND lt2.metric_type = 'lt2'
LEFT JOIN latest vo2max     ON vo2max.athlete_id = a.athlete_id AND vo2max.metric_type = 'vo2max'
LEFT JOIN latest weight     ON weight.athlete_id = a.athlete_id AND weight.metric_type = 'weight'
LEFT JOIN latest max_hr     ON max_hr.athlete_id = a.athlete_id AND max_hr.metric_type = 'max_hr'
LEFT JOIN latest resting_hr ON resting_hr.athlete_id = a.athlete_id AND resting_hr.metric_type = 'resting_hr';
