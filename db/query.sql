-- name: UpsertActivityEnduranceOutdoor :one
INSERT INTO vo2.activities_endurance_outdoor
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

-- name: GetActivityEnduranceOutdoor :one
SELECT
	a.*
FROM vo2.activities_endurance_outdoor a
WHERE a.id = $1;

-- name: ListActivitiesEnduranceOutdoorByTag :many
SELECT
	a.*
FROM vo2.activities_endurance_outdoor a
JOIN vo2.activities_endurance_outdoor_tags at ON at.activity_id = a.id
JOIN vo2.activity_tags t ON at.tag_id = t.id
WHERE
	a.provider_id = sqlc.arg(provider_id) AND
	a.user_id = sqlc.arg(user_id) AND
	lower(t.name) = lower(sqlc.arg(tag));

-- name: GetActivityTags :many
SELECT
    t.*
FROM
vo2.activities_endurance_outdoor_tags at
JOIN vo2.activity_tags t ON at.tag_id = t.id
WHERE at.activity_id = $1;
