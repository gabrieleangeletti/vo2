package store

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/gabrieleangeletti/vo2"
	"github.com/gabrieleangeletti/vo2/activity"
	"github.com/gabrieleangeletti/vo2/internal/generated/models"
)

// dbStore provides the implementation of the Store interface.
// It uses a pgx connection pool and sqlc-generated queries.
type dbStore struct {
	db *sqlx.DB
	q  *models.Queries
}

func NewReader(db *sqlx.DB) vo2.Reader {
	return &dbStore{
		db: db,
		q:  models.New(db),
	}
}

// NewStore creates a new dbStore instance.
func NewStore(db *sqlx.DB) vo2.Store {
	return &dbStore{
		db: db,
		q:  models.New(db),
	}
}

// UpsertActivityEndurance inserts or updates an endurance activity.
func (s *dbStore) UpsertActivityEndurance(ctx context.Context, arg *activity.EnduranceActivity) (*activity.EnduranceActivity, error) {
	res, err := s.q.UpsertActivityEndurance(ctx, arg.ToUpsertParams())
	if err != nil {
		return nil, err
	}

	return activity.NewEnduranceActivity(res), nil
}

// GetActivityEndurance retrieves an endurance activity by its ID.
func (s *dbStore) GetActivityEndurance(ctx context.Context, id uuid.UUID) (*activity.EnduranceActivity, error) {
	res, err := s.q.GetActivityEndurance(ctx, id)
	if err != nil {
		return nil, err
	}

	return activity.NewEnduranceActivity(res), nil
}

// ListActivitiesEnduranceByTag retrieves a list of activities by tag.
func (s *dbStore) ListActivitiesEnduranceByTag(ctx context.Context, providerID int, userID uuid.UUID, tag string) ([]*activity.EnduranceActivity, error) {
	res, err := s.q.ListActivitiesEnduranceByTag(ctx, models.ListActivitiesEnduranceByTagParams{
		ProviderID: int32(providerID),
		UserID:     userID,
		Tag:        tag,
	})
	if err != nil {
		return nil, err
	}

	activities := make([]*activity.EnduranceActivity, len(res))
	for i, r := range res {
		activities[i] = activity.NewEnduranceActivity(r)
	}

	return activities, nil
}

// GetActivityTags retrieves all tags for a given activity.
func (s *dbStore) GetActivityTags(ctx context.Context, activityID uuid.UUID) ([]*activity.ActivityTag, error) {
	res, err := s.q.GetActivityTags(ctx, activityID)
	if err != nil {
		return nil, err
	}

	tags := make([]*activity.ActivityTag, len(res))
	for i, r := range res {
		tags[i] = activity.NewActivityTag(r)
	}

	return tags, nil
}

func (s *dbStore) UpsertTagsAndLinkActivity(ctx context.Context, a *activity.EnduranceActivity, tags []*activity.ActivityTag) error {
	if len(tags) == 0 {
		return nil
	}

	var names []string
	var descriptions []string

	for _, tag := range tags {
		names = append(names, tag.Name)
		descriptions = append(descriptions, tag.Description)
	}

	query := `
	WITH upserted_tags AS (
		INSERT INTO vo2.activity_tags (name, description)
		SELECT unnest($1::text[]), unnest($2::text[])
		ON CONFLICT (name)
		DO UPDATE SET description = COALESCE(EXCLUDED.description, activity_tags.description)
		RETURNING id, name
	)
	INSERT INTO vo2.activities_endurance_tags (activity_id, tag_id)
	SELECT $3, ut.id
	FROM upserted_tags ut
	ON CONFLICT DO NOTHING`

	_, err := s.db.ExecContext(ctx, query, names, descriptions, a.ID)
	if err != nil {
		return err
	}

	return nil
}

func (s *dbStore) SaveProviderActivityRawData(ctx context.Context, arg *activity.ProviderActivityRawData) (uuid.UUID, error) {
	query := `
	INSERT INTO vo2.provider_activity_raw_data
		(provider_id, user_id, provider_activity_id, start_time, elapsed_time, iana_timezone, utc_offset, data, detailed_activity_uri)
	VALUES
		($1, $2, $3, $4, $5, $6, $7, $8, $9)
	ON CONFLICT
		(provider_id, user_id, provider_activity_id)
	DO UPDATE SET
		start_time = $4,
		elapsed_time = $5,
		iana_timezone = $6,
		utc_offset = $7,
		data = $8,
		detailed_activity_uri = $9
	RETURNING id
	`

	var id uuid.UUID
	err := s.db.QueryRowContext(ctx, query,
		arg.ProviderID,
		arg.UserID,
		arg.ProviderActivityID,
		arg.StartTime,
		arg.ElapsedTime,
		arg.IanaTimezone,
		arg.UTCOffset,
		arg.Data,
		arg.DetailedActivityURI,
	).Scan(&id)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

// GetAthleteVolume retrieves volume data for an athlete by provider, frequency, sport, and time range.
func (s *dbStore) GetAthleteVolume(ctx context.Context, params vo2.GetAthleteVolumeParams) ([]*vo2.AthleteVolumeData, error) {
	queryParams := models.GetAthleteVolumeParams{
		Frequency:    params.Frequency,
		UserID:       params.UserID,
		ProviderSlug: params.ProviderSlug,
		Sport:        params.Sport,
		StartDate:    params.StartDate,
	}

	res, err := s.q.GetAthleteVolume(ctx, queryParams)
	if err != nil {
		return nil, err
	}

	volumeData := make([]*vo2.AthleteVolumeData, len(res))
	for i, r := range res {
		volumeData[i] = &vo2.AthleteVolumeData{
			Period:                   r.Period,
			ActivityCount:            r.ActivityCount,
			TotalDistanceMeters:      r.TotalDistanceMeters,
			TotalElapsedTimeSeconds:  r.TotalElapsedTimeSeconds,
			TotalMovingTimeSeconds:   r.TotalMovingTimeSeconds,
			TotalElevationGainMeters: r.TotalElevationGainMeters,
		}
	}

	return volumeData, nil
}
