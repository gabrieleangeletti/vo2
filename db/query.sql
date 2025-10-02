-- name: UpsertActivityEndurance :one
INSERT INTO vo2.activities_endurance
	(provider_id, user_id, provider_raw_activity_id, name, description, sport, start_time, end_time, iana_timezone, utc_offset, elapsed_time, moving_time, distance, elev_gain, elev_loss, avg_speed, avg_hr, max_hr, summary_polyline, summary_route, gpx_file_uri, fit_file_uri)
VALUES
	(
    	@provider_id,
    	@user_id,
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
	(provider_id, user_id, provider_raw_activity_id)
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

-- name: GetActivityEndurance :one
SELECT
	a.*
FROM vo2.activities_endurance a
WHERE a.id = $1;

-- name: ListActivitiesEnduranceByTag :many
SELECT
	a.*
FROM vo2.activities_endurance a
JOIN vo2.activities_endurance_tags at ON at.activity_id = a.id
JOIN vo2.activity_tags t ON at.tag_id = t.id
WHERE
	a.provider_id = sqlc.arg(provider_id) AND
	a.user_id = sqlc.arg(user_id) AND
	lower(t.name) = lower(sqlc.arg(tag));

-- name: GetActivityTags :many
SELECT
    t.*
FROM
vo2.activities_endurance_tags at
JOIN vo2.activity_tags t ON at.tag_id = t.id
WHERE at.activity_id = $1;

-- name: GetAthleteVolume :many
WITH all_periods AS (
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
period_data AS (
    SELECT
        CASE
            WHEN @frequency::text = 'day' THEN date_trunc('day', start_time)
            WHEN @frequency::text = 'week' THEN date_trunc('week', start_time)
            ELSE date_trunc('month', start_time)
        END as period_ts,
        distance,
        elapsed_time,
        moving_time,
        elev_gain
    FROM vo2.activities_endurance a
    JOIN vo2.providers p ON a.provider_id = p.id
    WHERE
        a.user_id = @user_id
        AND p.slug = @provider_slug
        AND lower(a.sport) = lower(@sport)
        AND a.start_time >= @start_date::timestamptz
)
SELECT
    all_periods.period_ts::date::text as period,
    COUNT(period_data.period_ts)::int as activity_count,
    COALESCE(SUM(period_data.distance), 0)::int as total_distance_meters,
    COALESCE(SUM(period_data.elapsed_time), 0)::bigint as total_elapsed_time_seconds,
    COALESCE(SUM(period_data.moving_time), 0)::bigint as total_moving_time_seconds,
    COALESCE(SUM(period_data.elev_gain), 0)::int as total_elevation_gain_meters
FROM all_periods
LEFT JOIN period_data ON all_periods.period_ts = period_data.period_ts
GROUP BY all_periods.period_ts
ORDER BY all_periods.period_ts;
