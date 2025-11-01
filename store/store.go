package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/gabrieleangeletti/stride"
	"github.com/gabrieleangeletti/vo2"
	"github.com/gabrieleangeletti/vo2/activity"
	"github.com/gabrieleangeletti/vo2/internal/generated/models"
)

// Reader defines the interface for read-only database operations.
// It's implemented by Store.
type Reader interface {
	GetActivityEndurance(ctx context.Context, id uuid.UUID) (*activity.EnduranceActivity, error)
	ListAthleteActivitiesEndurance(ctx context.Context, providerID int, athleteID uuid.UUID) ([]*activity.EnduranceActivity, error)
	ListAthleteActivitiesEnduranceByIDs(ctx context.Context, ids []uuid.UUID) ([]*activity.EnduranceActivity, error)
	ListActivitiesEnduranceByTag(ctx context.Context, providerID int, athleteID uuid.UUID, tag string) ([]*activity.EnduranceActivity, error)
	GetActivityTags(ctx context.Context, activityID uuid.UUID) ([]*activity.ActivityTag, error)
	GetAthlete(ctx context.Context, athleteID uuid.UUID) (*vo2.Athlete, error)
	GetUserAthletes(ctx context.Context, userID uuid.UUID) ([]*vo2.Athlete, error)
	GetAthleteCurrentMeasurements(ctx context.Context, athleteID uuid.UUID) (*vo2.AthleteCurrentMeasurements, error)
	GetAthleteVolume(ctx context.Context, params vo2.GetAthleteVolumeParams) (map[stride.Sport][]*vo2.AthleteVolumeData, error)
}

// Store defines the interface for read and write database operations.
// It's implemented by Store.
type Store interface {
	Reader
	UpsertAthlete(ctx context.Context, arg *vo2.Athlete) (*vo2.Athlete, error)
	UpsertActivityEndurance(ctx context.Context, arg *activity.EnduranceActivity) (*activity.EnduranceActivity, error)
	UploadActivityGPX(ctx context.Context, act *activity.EnduranceActivity, strideActivity *stride.Activity, timeseries *stride.ActivityTimeseries) (string, error)
	StoreActivityEndurance(ctx context.Context, provider stride.Provider, activityRaw *activity.ProviderActivityRawData, rawAct stride.ActivityConvertible, ts stride.ActivityTimeseriesConvertible) (*activity.EnduranceActivity, error)
	UpsertActivityThresholdAnalysis(ctx context.Context, arg *activity.ThresholdAnalysis) (*activity.ThresholdAnalysis, error)
	UpsertTagsAndLinkActivity(ctx context.Context, a *activity.EnduranceActivity, tags []*activity.ActivityTag) error
	SaveProviderActivityRawData(ctx context.Context, arg *activity.ProviderActivityRawData) (uuid.UUID, error)

	// Temporary until we migrate away from using the object store outside of this package.
	GetObjectStore() ObjectStore
}

// store provides the implementation of the Store interface.
// It uses a pgx connection pool and sqlc-generated queries.
type store struct {
	db  *sqlx.DB
	obj ObjectStore
	q   *models.Queries
}

func NewReader(db *sqlx.DB) (Reader, error) {
	obj, err := newS3ObjectStore()
	if err != nil {
		return nil, err
	}

	return &store{
		db:  db,
		obj: obj,
		q:   models.New(db),
	}, nil
}

// NewStore creates a new store instance.
func NewStore(db *sqlx.DB) (Store, error) {
	obj, err := newS3ObjectStore()
	if err != nil {
		return nil, err
	}

	return &store{
		db:  db,
		obj: obj,
		q:   models.New(db),
	}, nil
}

func (s *store) GetObjectStore() ObjectStore {
	return s.obj
}

func (s *store) UpsertAthlete(ctx context.Context, arg *vo2.Athlete) (*vo2.Athlete, error) {
	res, err := s.q.UpsertAthlete(ctx, arg.ToUpsertParams())
	if err != nil {
		return nil, err
	}

	return newAthlete(res), nil
}

func (s *store) GetAthlete(ctx context.Context, athleteID uuid.UUID) (*vo2.Athlete, error) {
	res, err := s.q.GetAthleteByID(ctx, athleteID)
	if err != nil {
		return nil, err
	}

	return newAthlete(res), nil
}

// GetUserAthletes retrieves the athletes associated with a user.
func (s *store) GetUserAthletes(ctx context.Context, userID uuid.UUID) ([]*vo2.Athlete, error) {
	res, err := s.q.GetUserAthletes(ctx, userID)
	if err != nil {
		return nil, err
	}

	athletes := make([]*vo2.Athlete, len(res))

	for i, r := range res {
		athletes[i] = newAthlete(r)
	}

	return athletes, nil
}

// GetAthleteCurrentMeasurements retrieves the current measurements for an athlete.
func (s *store) GetAthleteCurrentMeasurements(ctx context.Context, athleteID uuid.UUID) (*vo2.AthleteCurrentMeasurements, error) {
	res, err := s.q.GetAthleteCurrentMeasurements(ctx, athleteID)
	if err != nil {
		return nil, err
	}

	return newAthleteCurrentMeasurements(res), nil
}

func (s *store) UpsertActivityThresholdAnalysis(ctx context.Context, arg *activity.ThresholdAnalysis) (*activity.ThresholdAnalysis, error) {
	res, err := s.q.UpsertActivityThresholdAnalysis(ctx, arg.ToUpsertParams())
	if err != nil {
		return nil, err
	}

	return activity.NewActivityThresholdAnalysis(res), nil
}

// UpsertActivityEndurance inserts or updates an endurance activity.
func (s *store) UpsertActivityEndurance(ctx context.Context, arg *activity.EnduranceActivity) (*activity.EnduranceActivity, error) {
	res, err := s.q.UpsertActivityEndurance(ctx, arg.ToUpsertParams())
	if err != nil {
		return nil, err
	}

	return activity.NewEnduranceActivity(res), nil
}

// UploadActivityGPX generates a GPX file for an endurance activity, uploads it to the object storage, and returns its URL.
func (s *store) UploadActivityGPX(ctx context.Context, act *activity.EnduranceActivity, strideActivity *stride.Activity, timeseries *stride.ActivityTimeseries) (string, error) {
	gpxData, err := stride.CreateGPXFileInMemory(strideActivity, timeseries)
	if err != nil {
		return "", err
	}

	objectKey := fmt.Sprintf("activity_details/%s/gpx/%s.gpx", strideActivity.Provider, act.ID)

	res, err := s.obj.UploadObject(ctx, objectKey, gpxData, nil)
	if err != nil {
		return "", err
	}

	return res.Location, nil
}

// StoreActivityEndurance is a higher-level function that does the e2e storing of an endurance activity.
//
// * Converts the raw provider activity into the standardized format.
// * Generates and uploads the activity's GPX file.
// * Calculates the activity's HR metrics.
// * Upserts the activity.
func (s *store) StoreActivityEndurance(ctx context.Context, provider stride.Provider, activityRaw *activity.ProviderActivityRawData, rawAct stride.ActivityConvertible, ts stride.ActivityTimeseriesConvertible) (*activity.EnduranceActivity, error) {
	act, err := activityRaw.ToEnduranceActivity(provider)
	if err != nil {
		if !(errors.Is(err, stride.ErrActivityIsNotEndurance) || errors.Is(err, stride.ErrUnsupportedSportType)) {
			return nil, err
		}
	}

	strideActivity, err := rawAct.ToActivity()
	if err != nil {
		return nil, err
	}

	timeseries, err := ts.ToTimeseries(strideActivity.StartTime)
	if err != nil {
		return nil, err
	}

	if len(timeseries.Data) > 0 {
		gpxFileURI, err := s.UploadActivityGPX(ctx, act, strideActivity, timeseries)
		if err != nil {
			return nil, err
		}
		act.GpxFileURI = gpxFileURI

		hrMetrics, err := timeseries.HRMetrics()
		if err != nil {
			return nil, err
		}
		act.AvgHR = hrMetrics.AvgHR
		act.MaxHR = hrMetrics.MaxHR
	}

	act, err = s.UpsertActivityEndurance(ctx, act)
	if err != nil {
		return nil, err
	}

	return act, nil
}

// GetActivityEndurance retrieves an endurance activity by its ID.
func (s *store) GetActivityEndurance(ctx context.Context, id uuid.UUID) (*activity.EnduranceActivity, error) {
	res, err := s.q.GetActivityEndurance(ctx, id)
	if err != nil {
		return nil, err
	}

	return activity.NewEnduranceActivity(res), nil
}

func (s *store) ListAthleteActivitiesEndurance(ctx context.Context, providerID int, athleteID uuid.UUID) ([]*activity.EnduranceActivity, error) {
	res, err := s.q.ListAthleteActivitiesEndurance(ctx, models.ListAthleteActivitiesEnduranceParams{
		ProviderID: int32(providerID),
		AthleteID:  athleteID,
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

func (s *store) ListAthleteActivitiesEnduranceByIDs(ctx context.Context, ids []uuid.UUID) ([]*activity.EnduranceActivity, error) {
	res, err := s.q.ListActivitiesEnduranceById(ctx, ids)
	if err != nil {
		return nil, err
	}

	activities := make([]*activity.EnduranceActivity, len(res))
	for i, r := range res {
		activities[i] = activity.NewEnduranceActivity(r)
	}

	return activities, nil
}

// ListActivitiesEnduranceByTag retrieves a list of activities by tag.
func (s *store) ListActivitiesEnduranceByTag(ctx context.Context, providerID int, athleteID uuid.UUID, tag string) ([]*activity.EnduranceActivity, error) {
	res, err := s.q.ListActivitiesEnduranceByTag(ctx, models.ListActivitiesEnduranceByTagParams{
		ProviderID: int32(providerID),
		AthleteID:  athleteID,
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
func (s *store) GetActivityTags(ctx context.Context, activityID uuid.UUID) ([]*activity.ActivityTag, error) {
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

func (s *store) UpsertTagsAndLinkActivity(ctx context.Context, a *activity.EnduranceActivity, tags []*activity.ActivityTag) error {
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

func (s *store) SaveProviderActivityRawData(ctx context.Context, arg *activity.ProviderActivityRawData) (uuid.UUID, error) {
	query := `
	INSERT INTO vo2.provider_activity_raw_data
		(provider_id, athlete_id, provider_activity_id, start_time, elapsed_time, iana_timezone, utc_offset, data, detailed_activity_uri)
	VALUES
		($1, $2, $3, $4, $5, $6, $7, $8, $9)
	ON CONFLICT
		(provider_id, athlete_id, provider_activity_id)
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
		arg.AthleteID,
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

// GetAthleteVolume retrieves volume data for an athlete by provider, frequency, sports, and time range.
func (s *store) GetAthleteVolume(ctx context.Context, params vo2.GetAthleteVolumeParams) (map[stride.Sport][]*vo2.AthleteVolumeData, error) {
	sports := make([]string, len(params.Sports))
	for i, sport := range params.Sports {
		sports[i] = strings.ToLower(string(sport))
	}

	queryParams := models.GetAthleteVolumeParams{
		Sports:       sports,
		Frequency:    params.Frequency,
		StartDate:    params.StartDate,
		AthleteID:    params.AthleteID,
		ProviderSlug: params.ProviderSlug,
	}

	res, err := s.q.GetAthleteVolume(ctx, queryParams)
	if err != nil {
		return nil, err
	}

	volumeData := make(map[stride.Sport][]*vo2.AthleteVolumeData, len(params.Sports))
	for _, sport := range params.Sports {
		volumeData[sport] = make([]*vo2.AthleteVolumeData, 0)
	}

	for _, r := range res {
		sport := stride.Sport(r.Sport)
		entry := &vo2.AthleteVolumeData{
			Period:                   r.Period,
			ActivityCount:            r.ActivityCount,
			TotalDistanceMeters:      r.TotalDistanceMeters,
			TotalElapsedTimeSeconds:  r.TotalElapsedTimeSeconds,
			TotalMovingTimeSeconds:   r.TotalMovingTimeSeconds,
			TotalElevationGainMeters: r.TotalElevationGainMeters,
		}

		volumeData[sport] = append(volumeData[sport], entry)
	}

	return volumeData, nil
}
