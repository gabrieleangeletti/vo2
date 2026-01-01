-- name: UpsertActivityEndurance :one
INSERT INTO vo2.activities_endurance
	(provider_id, athlete_id, provider_raw_activity_id, name, description, sport, start_time, end_time, iana_timezone, utc_offset, elapsed_time, moving_time, distance, elev_gain, elev_loss, avg_speed, avg_hr, max_hr, summary_polyline, summary_route, gpx_file_uri, fit_file_uri)
VALUES
	(
    	@provider_id,
    	@athlete_id,
    	@provider_raw_activity_id,
    	@name,
    	@description,
    	@sport,
    	@start_time,
    	@end_time,
    	@iana_timezone,
    	@utc_offset,
    	@elapsed_time,
    	@moving_time,
    	@distance,
    	@elev_gain,
    	@elev_loss,
    	@avg_speed,
    	@avg_hr,
    	@max_hr,
    	@summary_polyline,
    	NULLIF(@summary_route, ''),
    	@gpx_file_uri,
    	@fit_file_uri
)
ON CONFLICT
	(provider_id, athlete_id, provider_raw_activity_id)
DO UPDATE SET
	name = @name,
	description = @description,
	sport = @sport,
	start_time = @start_time,
	end_time = @end_time,
	iana_timezone = @iana_timezone,
	utc_offset = @utc_offset,
	elapsed_time = @elapsed_time,
	moving_time = @moving_time,
	distance = @distance,
	elev_gain = @elev_gain,
	elev_loss = @elev_loss,
	avg_speed = @avg_speed,
	avg_hr = @avg_hr,
	max_hr = @max_hr,
	summary_polyline = @summary_polyline,
	summary_route = NULLIF(@summary_route, ''),
	gpx_file_uri = @gpx_file_uri,
	fit_file_uri = @fit_file_uri
RETURNING *;

-- name: UpsertActivityThresholdAnalysis :one
INSERT INTO vo2.activities_threshold_analysis (
	activity_endurance_id,
	time_at_lt1_threshold,
	time_at_lt2_threshold,
	raw_analysis
)
VALUES (
	@activity_endurance_id,
	@time_at_lt1_threshold,
	@time_at_lt2_threshold,
	@raw_analysis
)
ON CONFLICT
	(activity_endurance_id)
DO UPDATE SET
	time_at_lt1_threshold = @time_at_lt1_threshold,
	time_at_lt2_threshold = @time_at_lt2_threshold,
	raw_analysis = @raw_analysis
RETURNING *;

-- name: GetProviderActivityRaw :one
SELECT
    *
FROM vo2.provider_activity_raw_data
WHERE
    id = $1;

-- name: GetAthleteCurrentMeasurements :one
SELECT
    *
FROM
    vo2.athlete_current_measurements
WHERE
    athlete_id = @athlete_id;

-- name: GetActivityEndurance :one
SELECT
	a.*
FROM vo2.activities_endurance a
WHERE
    a.id = $1;

-- name: ListAthleteActivitiesEndurance :many
SELECT
	*
FROM vo2.activities_endurance
WHERE
	provider_id = sqlc.arg(provider_id) AND
	athlete_id = sqlc.arg(athlete_id)
ORDER BY
    start_time DESC;

-- name: ListActivitiesEnduranceById :many
SELECT
	*
FROM vo2.activities_endurance
WHERE
    id = ANY(@ids::uuid[]);

-- name: ListActivitiesEnduranceByTag :many
SELECT
	a.*
FROM vo2.activities_endurance a
JOIN vo2.activities_endurance_tags at ON at.activity_id = a.id
JOIN vo2.activity_tags t ON at.tag_id = t.id
WHERE
	a.provider_id = sqlc.arg(provider_id) AND
	a.athlete_id = sqlc.arg(athlete_id) AND
	lower(t.name) = lower(sqlc.arg(tag))
ORDER BY
    a.start_time DESC;

-- name: GetActivityTags :many
SELECT
    t.*
FROM
vo2.activities_endurance_tags at
JOIN vo2.activity_tags t ON at.tag_id = t.id
WHERE
    at.activity_id = $1;

-- name: UpsertAthlete :one
INSERT INTO vo2.athletes
    (user_id, age, height_cm, country, gender, first_name, last_name, display_name, email)
VALUES (
	@user_id,
	@age,
	@height_cm,
	@country,
	@gender,
	@first_name,
	@last_name,
	@display_name,
	@email)
ON CONFLICT (user_id) DO UPDATE SET
	age = @age,
	height_cm = @height_cm,
	country = @country,
	gender = @gender,
	first_name = @first_name,
	last_name = @last_name,
	display_name = @display_name,
	email = @email
RETURNING *;

-- name: GetAthleteByID :one
SELECT
    *
FROM
    vo2.athletes
WHERE
    id = $1;

-- name: GetUserAthletes :many
SELECT
    *
FROM
    vo2.athletes
WHERE
    user_id = @user_id;

-- name: GetAthleteRunningYTDVolume :one
SELECT
    COALESCE(SUM(a.distance), 0)::int AS total_distance_meters,
    COALESCE(SUM(a.moving_time), 0)::bigint AS total_moving_time_seconds,
    COALESCE(SUM(a.elev_gain), 0)::int AS total_elevation_gain_meters,
    COUNT(*)::int AS activity_count
FROM vo2.activities_endurance a
JOIN vo2.providers p ON a.provider_id = p.id
WHERE
    a.athlete_id = @athlete_id
    AND p.slug = @provider_slug
    AND a.start_time >= date_trunc('year', NOW())
    AND lower(a.sport) IN ('running', 'trail-running');

-- name: GetAthleteVolume :many
WITH selected_sports AS (
    SELECT DISTINCT lower(ss.sport) AS sport
    FROM unnest(sqlc.arg(sports)::text[]) AS ss(sport)
),
all_periods AS (
    SELECT generate_series(
        date_trunc(@frequency::text, @start_date::timestamptz),
        date_trunc(@frequency::text, NOW()),
        CASE
            WHEN @frequency::text = 'day' THEN '1 day'
            WHEN @frequency::text = 'week' THEN '1 week'
            ELSE '1 month'
        END::interval
    ) as period_ts
),
period_sports AS (
    SELECT ap.period_ts, ss.sport
    FROM all_periods ap
    CROSS JOIN selected_sports ss
),
period_data AS (
    SELECT
        CASE
            WHEN @frequency::text = 'day' THEN date_trunc('day', a.start_time)
            WHEN @frequency::text = 'week' THEN date_trunc('week', a.start_time)
            ELSE date_trunc('month', a.start_time)
        END as period_ts,
        lower(a.sport) AS sport,
        COUNT(*)::int AS activity_count,
        COALESCE(SUM(a.distance), 0)::int AS total_distance_meters,
        COALESCE(SUM(a.elapsed_time), 0)::bigint AS total_elapsed_time_seconds,
        COALESCE(SUM(a.moving_time), 0)::bigint AS total_moving_time_seconds,
        COALESCE(SUM(a.elev_gain), 0)::int AS total_elevation_gain_meters
    FROM vo2.activities_endurance a
    JOIN vo2.providers p ON a.provider_id = p.id
    JOIN selected_sports ss ON lower(a.sport) = ss.sport
    WHERE
        a.athlete_id = @athlete_id
        AND p.slug = @provider_slug
        AND a.start_time >= @start_date::timestamptz
    GROUP BY period_ts, lower(a.sport)
)
SELECT
    period_sports.period_ts::date::text as period,
    period_sports.sport,
    COALESCE(period_data.activity_count, 0)::int as activity_count,
    COALESCE(period_data.total_distance_meters, 0)::int as total_distance_meters,
    COALESCE(period_data.total_elapsed_time_seconds, 0)::bigint as total_elapsed_time_seconds,
    COALESCE(period_data.total_moving_time_seconds, 0)::bigint as total_moving_time_seconds,
    COALESCE(period_data.total_elevation_gain_meters, 0)::int as total_elevation_gain_meters
FROM period_sports
LEFT JOIN period_data
    ON period_sports.period_ts = period_data.period_ts
    AND period_sports.sport = period_data.sport
ORDER BY period_sports.sport, period_sports.period_ts;
