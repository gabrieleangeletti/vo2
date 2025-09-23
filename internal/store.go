package internal

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/gabrieleangeletti/vo2/activity"
	"github.com/gabrieleangeletti/vo2/internal/generated/models"
)

// Store defines the interface for database operations.
// It's implemented by DBStore.
type Store interface {
	UpsertActivityEnduranceOutdoor(ctx context.Context, arg *activity.EnduranceOutdoorActivity) (*activity.EnduranceOutdoorActivity, error)
	GetActivityEnduranceOutdoor(ctx context.Context, id uuid.UUID) (*activity.EnduranceOutdoorActivity, error)
	ListActivitiesEnduranceOutdoorByTag(ctx context.Context, providerID int, userID uuid.UUID, tag string) ([]*activity.EnduranceOutdoorActivity, error)
	GetActivityTags(ctx context.Context, activityID uuid.UUID) ([]*activity.ActivityTag, error)
	UpsertTagsAndLinkActivity(ctx context.Context, a *activity.EnduranceOutdoorActivity, tags []*activity.ActivityTag) error
	SaveProviderActivityRawData(ctx context.Context, arg *activity.ProviderActivityRawData) error
}

// DBStore provides the implementation of the Store interface.
// It uses a pgx connection pool and sqlc-generated queries.
type DBStore struct {
	db *sqlx.DB
	q  *models.Queries
}

// NewStore creates a new DBStore instance.
func NewStore(db *sqlx.DB) Store {
	return &DBStore{
		db: db,
		q:  models.New(db),
	}
}

// UpsertActivityEnduranceOutdoor inserts or updates an endurance activity.
func (s *DBStore) UpsertActivityEnduranceOutdoor(ctx context.Context, arg *activity.EnduranceOutdoorActivity) (*activity.EnduranceOutdoorActivity, error) {
	res, err := s.q.UpsertActivityEnduranceOutdoor(ctx, arg.ToUpsertParams())
	if err != nil {
		return nil, err
	}

	return activity.NewEnduranceOutdoorActivity(res), nil
}

// GetActivityEnduranceOutdoor retrieves an endurance activity by its ID.
func (s *DBStore) GetActivityEnduranceOutdoor(ctx context.Context, id uuid.UUID) (*activity.EnduranceOutdoorActivity, error) {
	res, err := s.q.GetActivityEnduranceOutdoor(ctx, id)
	if err != nil {
		return nil, err
	}

	return activity.NewEnduranceOutdoorActivity(res), nil
}

// ListActivitiesEnduranceOutdoorByTag retrieves a list of activities by tag.
func (s *DBStore) ListActivitiesEnduranceOutdoorByTag(ctx context.Context, providerID int, userID uuid.UUID, tag string) ([]*activity.EnduranceOutdoorActivity, error) {
	res, err := s.q.ListActivitiesEnduranceOutdoorByTag(ctx, models.ListActivitiesEnduranceOutdoorByTagParams{
		ProviderID: int32(providerID),
		UserID:     userID,
		Tag:        tag,
	})
	if err != nil {
		return nil, err
	}

	activities := make([]*activity.EnduranceOutdoorActivity, len(res))
	for i, r := range res {
		activities[i] = activity.NewEnduranceOutdoorActivity(r)
	}

	return activities, nil
}

// GetActivityTags retrieves all tags for a given activity.
func (s *DBStore) GetActivityTags(ctx context.Context, activityID uuid.UUID) ([]*activity.ActivityTag, error) {
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

func (s *DBStore) UpsertTagsAndLinkActivity(ctx context.Context, a *activity.EnduranceOutdoorActivity, tags []*activity.ActivityTag) error {
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
	INSERT INTO vo2.activities_endurance_outdoor_tags (activity_id, tag_id)
	SELECT $3, ut.id
	FROM upserted_tags ut
	ON CONFLICT DO NOTHING`

	_, err := s.db.ExecContext(ctx, query, names, descriptions, a.ID)
	if err != nil {
		return err
	}

	return nil
}

func (s *DBStore) SaveProviderActivityRawData(ctx context.Context, arg *activity.ProviderActivityRawData) error {
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
	`
	_, err := s.db.ExecContext(ctx, query,
		arg.ProviderID,
		arg.UserID,
		arg.ProviderActivityID,
		arg.StartTime,
		arg.ElapsedTime,
		arg.IanaTimezone,
		arg.UTCOffset,
		arg.Data,
		arg.DetailedActivityURI,
	)
	return err
}
