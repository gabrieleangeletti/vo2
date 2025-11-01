package activity

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/gabrieleangeletti/stride"
	"github.com/gabrieleangeletti/stride/strava"
	"github.com/gabrieleangeletti/vo2/database"
	"github.com/gabrieleangeletti/vo2/internal/generated/models"
	"github.com/gabrieleangeletti/vo2/provider"
)

var (
	ErrNoGPXFile = errors.New("activity has no GPX file")
)

type ProviderActivityRawData struct {
	ID                  uuid.UUID       `json:"id" db:"id"`
	ProviderID          int             `json:"providerId" db:"provider_id"`
	AthleteID           uuid.UUID       `json:"athleteId" db:"athlete_id"`
	ProviderActivityID  string          `json:"providerActivityId" db:"provider_activity_id"`
	StartTime           time.Time       `json:"startTime" db:"start_time"`
	ElapsedTime         int             `json:"elapsedTime" db:"elapsed_time"`
	IanaTimezone        sql.NullString  `json:"ianaTimezone" db:"iana_timezone"`
	UTCOffset           sql.NullInt32   `json:"utcOffset" db:"utc_offset"`
	Data                json.RawMessage `json:"data" db:"data"`
	DetailedActivityURI sql.NullString  `json:"detailedActivityUri" db:"detailed_activity_uri"`
	CreatedAt           time.Time       `json:"createdAt" db:"created_at"`
	UpdatedAt           sql.NullTime    `json:"updatedAt" db:"updated_at"`
	DeletedAt           sql.NullTime    `json:"deletedAt" db:"deleted_at"`
}

func (a *ProviderActivityRawData) ToEnduranceActivity(prov stride.Provider) (*EnduranceActivity, error) {
	switch prov {
	case "strava":
		var act strava.ActivityDetailed
		err := json.Unmarshal(a.Data, &act)
		if err != nil {
			return nil, err
		}

		isEndurance, err := act.IsEnduranceActivity()
		if err != nil {
			return nil, err
		}

		if !isEndurance {
			return nil, stride.ErrActivityIsNotEndurance
		}

		sport, err := act.Sport()
		if err != nil {
			return nil, err
		}

		var utcOffset *int32
		if a.UTCOffset.Valid {
			utcOffset = &a.UTCOffset.Int32
		}

		var elevGain *int32
		if act.TotalElevationGain > 0 {
			gain := int32(act.TotalElevationGain)
			elevGain = &gain
		}

		enduranceActivity := &EnduranceActivity{
			ProviderID:            a.ProviderID,
			AthleteID:             a.AthleteID,
			ProviderRawActivityID: a.ID,
			Name:                  act.Name,
			Description:           act.Description,
			Sport:                 sport,
			StartTime:             a.StartTime,
			EndTime:               a.StartTime.Add(time.Duration(a.ElapsedTime) * time.Second),
			IanaTimezone:          a.IanaTimezone.String,
			UTCOffset:             utcOffset,
			ElapsedTime:           act.ElapsedTime,
			MovingTime:            act.MovingTime,
			Distance:              int(act.Distance),
			AvgSpeed:              act.AverageSpeed,
			ElevGain:              elevGain,
		}

		summaryPolyline := act.SummaryPolyline()
		if summaryPolyline != "" {
			enduranceActivity.SummaryPolyline = summaryPolyline

			wkt, err := stride.PolylineToWKT(summaryPolyline)
			if err != nil {
				return nil, err
			}

			enduranceActivity.SummaryRoute = wkt
		}

		return enduranceActivity, nil
	default:
		return nil, fmt.Errorf("%w: %d", provider.ErrUnsupportedProvider, a.ProviderID)
	}
}

func (a *ProviderActivityRawData) Save(ctx context.Context, db *sqlx.DB) error {
	_, err := db.ExecContext(ctx, `
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
	`,
		a.ProviderID, a.AthleteID, a.ProviderActivityID, a.StartTime, a.ElapsedTime, a.IanaTimezone, a.UTCOffset, a.Data, a.DetailedActivityURI,
	)
	if err != nil {
		return err
	}

	return nil
}

func GetProviderActivityRawData(ctx context.Context, db *sqlx.DB, providerID int, athleteID uuid.UUID) ([]*ProviderActivityRawData, error) {
	var data []*ProviderActivityRawData

	err := db.SelectContext(ctx, &data, `
	SELECT * FROM vo2.provider_activity_raw_data
	WHERE provider_id = $1 AND athlete_id = $2
	`, providerID, athleteID)
	if err != nil {
		return nil, err
	}

	return data, nil
}

type EnduranceActivity struct {
	ID                    uuid.UUID    `json:"id"`
	ProviderID            int          `json:"providerId"`
	AthleteID             uuid.UUID    `json:"athleteId"`
	ProviderRawActivityID uuid.UUID    `json:"providerRawActivityId"`
	Name                  string       `json:"name"`
	Description           string       `json:"description,omitzero"`
	Sport                 stride.Sport `json:"sport"`
	StartTime             time.Time    `json:"startTime"`
	EndTime               time.Time    `json:"endTime"`
	IanaTimezone          string       `json:"ianaTimezone,omitzero"`
	UTCOffset             *int32       `json:"utcOffset,omitempty"`
	ElapsedTime           int          `json:"elapsedTime"`
	MovingTime            int          `json:"movingTime"`
	Distance              int          `json:"distance"`
	ElevGain              *int32       `json:"elevGain"`
	ElevLoss              *int32       `json:"elevLoss"`
	AvgSpeed              float64      `json:"avgSpeed"`
	AvgHR                 int16        `json:"avgHR,omitzero"`
	MaxHR                 int16        `json:"maxHR,omitzero"`
	SummaryPolyline       string       `json:"summaryPolyline,omitzero"`
	SummaryRoute          string       `json:"summaryRoute,omitzero"`
	GpxFileURI            string       `json:"gpxFileURI,omitzero"`
	FitFileURI            string       `json:"fitFileURI,omitzero"`
	CreatedAt             time.Time    `json:"createdAt"`
	UpdatedAt             time.Time    `json:"updatedAt,omitzero"`
	DeletedAt             time.Time    `json:"deletedAt,omitzero"`

	Provider *provider.Data `json:"provider"`
	Tags     []*ActivityTag `json:"tags"`
}

func NewEnduranceActivity(a models.Vo2ActivitiesEndurance) *EnduranceActivity {
	var utcOffset *int32
	if a.UtcOffset.Valid {
		utcOffset = &a.UtcOffset.Int32
	}

	var elevationGain *int32
	if a.ElevGain.Valid {
		elevationGain = &a.ElevGain.Int32
	}

	var elevationLoss *int32
	if a.ElevLoss.Valid {
		elevationLoss = &a.ElevLoss.Int32
	}

	var summaryRoute string
	if a.SummaryRoute != nil {
		summaryRoute = a.SummaryRoute.(string)
	}

	return &EnduranceActivity{
		ID:                    a.ID,
		ProviderID:            int(a.ProviderID),
		AthleteID:             a.AthleteID,
		ProviderRawActivityID: a.ProviderRawActivityID,
		Name:                  a.Name,
		Description:           a.Description.String,
		Sport:                 stride.Sport(a.Sport),
		StartTime:             a.StartTime,
		EndTime:               a.EndTime,
		IanaTimezone:          a.IanaTimezone.String,
		UTCOffset:             utcOffset,
		ElapsedTime:           int(a.ElapsedTime),
		MovingTime:            int(a.MovingTime),
		Distance:              int(a.Distance),
		ElevGain:              elevationGain,
		ElevLoss:              elevationLoss,
		AvgSpeed:              a.AvgSpeed,
		AvgHR:                 int16(a.AvgHr.Int32),
		MaxHR:                 int16(a.MaxHr.Int32),
		SummaryPolyline:       a.SummaryPolyline.String,
		SummaryRoute:          summaryRoute,
		GpxFileURI:            a.GpxFileUri.String,
		FitFileURI:            a.FitFileUri.String,
	}
}

// ExtractActivityTags extracts hashtags from the activity description.
func (a *EnduranceActivity) ExtractActivityTags() []*ActivityTag {
	if a.Description == "" {
		return nil
	}

	var tags []*ActivityTag

	re := regexp.MustCompile(`#[\p{L}\d_-]+`)
	hashTags := re.FindAllString(a.Description, -1)

	for _, hashTag := range hashTags {
		tags = append(tags, &ActivityTag{Name: hashTag[1:]})
	}

	return tags
}

// ToUpsertParams converts the domain model to sqlc UpsertActivityEndurance parameters
func (a *EnduranceActivity) ToUpsertParams() models.UpsertActivityEnduranceParams {
	return models.UpsertActivityEnduranceParams{
		ProviderID:            int32(a.ProviderID),
		AthleteID:             a.AthleteID,
		ProviderRawActivityID: a.ProviderRawActivityID,
		Name:                  a.Name,
		Description:           sql.NullString{String: a.Description, Valid: true},
		Sport:                 string(a.Sport),
		StartTime:             a.StartTime,
		EndTime:               a.EndTime,
		IanaTimezone:          sql.NullString{String: a.IanaTimezone, Valid: true},
		UtcOffset:             database.ToNullInt32FromPtr(a.UTCOffset),
		ElapsedTime:           int32(a.ElapsedTime),
		MovingTime:            int32(a.MovingTime),
		Distance:              int32(a.Distance),
		ElevGain:              database.ToNullInt32FromPtr(a.ElevGain),
		ElevLoss:              database.ToNullInt32FromPtr(a.ElevLoss),
		AvgSpeed:              a.AvgSpeed,
		AvgHr:                 sql.NullInt32{Int32: int32(a.AvgHR), Valid: true},
		MaxHr:                 sql.NullInt32{Int32: int32(a.MaxHR), Valid: true},
		SummaryPolyline:       sql.NullString{String: a.SummaryPolyline, Valid: true},
		SummaryRoute:          a.SummaryRoute,
		GpxFileUri:            sql.NullString{String: a.GpxFileURI, Valid: true},
		FitFileUri:            sql.NullString{String: a.FitFileURI, Valid: true},
	}
}

type ThresholdAnalysis struct {
	ID                  int             `json:"id"`
	ActivityEnduranceID uuid.UUID       `json:"activity_endurance_id"`
	TimeAtLt1Threshold  int32           `json:"time_at_lt1_threshold"`
	TimeAtLt2Threshold  int32           `json:"time_at_lt2_threshold"`
	RawAnalysis         json.RawMessage `json:"raw_analysis"`
	CreatedAt           time.Time       `json:"createdAt"`
	UpdatedAt           time.Time       `json:"updatedAt,omitzero"`
	DeletedAt           time.Time       `json:"deletedAt,omitzero"`
}

func (a *ThresholdAnalysis) ToUpsertParams() models.UpsertActivityThresholdAnalysisParams {
	return models.UpsertActivityThresholdAnalysisParams{
		ActivityEnduranceID: a.ActivityEnduranceID,
		TimeAtLt1Threshold:  a.TimeAtLt1Threshold,
		TimeAtLt2Threshold:  a.TimeAtLt2Threshold,
		RawAnalysis:         a.RawAnalysis,
	}
}

func NewActivityThresholdAnalysis(a models.Vo2ActivitiesThresholdAnalysis) *ThresholdAnalysis {
	return &ThresholdAnalysis{
		ID:                  int(a.ID),
		ActivityEnduranceID: a.ActivityEnduranceID,
		TimeAtLt1Threshold:  a.TimeAtLt1Threshold,
		TimeAtLt2Threshold:  a.TimeAtLt2Threshold,
		RawAnalysis:         a.RawAnalysis,
	}
}
