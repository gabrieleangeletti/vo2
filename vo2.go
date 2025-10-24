package vo2

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/gabrieleangeletti/stride"
	"github.com/gabrieleangeletti/vo2/activity"
	"github.com/gabrieleangeletti/vo2/internal/generated/models"
)

type AthleteMeasurementSource string

const (
	AthleteMeasurementSourceLabTest   AthleteMeasurementSource = "lab_test"
	AthleteMeasurementSourceFieldTest AthleteMeasurementSource = "field_test"
	AthleteMeasurementSourceManual    AthleteMeasurementSource = "manual"
)

type Gender string

const (
	GenderMale   Gender = "male"
	GenderFemale Gender = "female"
	GenderOther  Gender = "other"
)

type Athlete struct {
	ID          uuid.UUID `json:"id"`
	UserID      uuid.UUID `json:"userID"`
	Age         int16     `json:"age"`
	HeightCm    int16     `json:"heightCm"`
	Country     string    `json:"country"`
	Gender      Gender    `json:"gender"`
	FirstName   string    `json:"firstName"`
	LastName    string    `json:"lastName"`
	DisplayName string    `json:"displayName"`
	Email       string    `json:"email"`
}

func (a *Athlete) ToUpsertParams() models.UpsertAthleteParams {
	return models.UpsertAthleteParams{
		UserID:      a.UserID,
		Age:         a.Age,
		HeightCm:    a.HeightCm,
		Country:     a.Country,
		Gender:      models.Gender(a.Gender),
		FirstName:   a.FirstName,
		LastName:    a.LastName,
		DisplayName: a.DisplayName,
		Email:       a.Email,
	}
}

type AthleteCurrentMeasurements struct {
	AthleteID        uuid.UUID                `json:"athleteID" db:"athlete_id"`
	Lt1Value         float64                  `json:"lt1Value,omitzero" db:"lt1_value"`
	Lt1MeasuredAt    time.Time                `json:"lt1MeasuredAt,omitzero" db:"lt1_measured_at"`
	Lt1Timezone      string                   `json:"lt1Timezone,omitzero" db:"lt1_timezone"`
	Lt1Source        AthleteMeasurementSource `json:"lt1Source,omitzero" db:"lt1_source"`
	Lt1Notes         string                   `json:"lt1Notes,omitzero" db:"lt1_notes"`
	Lt2Value         float64                  `json:"lt2Value,omitzero" db:"lt2_value"`
	Lt2MeasuredAt    time.Time                `json:"lt2MeasuredAt,omitzero" db:"lt2_measured_at"`
	Lt2Timezone      string                   `json:"lt2Timezone,omitzero" db:"lt2_timezone"`
	Lt2Source        AthleteMeasurementSource `json:"lt2Source,omitzero" db:"lt2_source"`
	Lt2Notes         string                   `json:"lt2Notes,omitzero" db:"lt2_notes"`
	Vo2maxValue      float64                  `json:"vo2maxValue,omitzero" db:"vo2max_value"`
	Vo2maxMeasuredAt time.Time                `json:"vo2maxMeasuredAt,omitzero" db:"vo2max_measured_at"`
	Vo2maxTimezone   string                   `json:"vo2maxTimezone,omitzero" db:"vo2max_timezone"`
	Vo2maxSource     AthleteMeasurementSource `json:"vo2maxSource,omitzero" db:"vo2max_source"`
	Vo2maxNotes      string                   `json:"vo2maxNotes,omitzero" db:"vo2max_notes"`
	WeightValue      float64                  `json:"weightValue,omitzero" db:"weight_value"`
	WeightMeasuredAt time.Time                `json:"weightMeasuredAt,omitzero" db:"weight_measured_at"`
	WeightTimezone   string                   `json:"weightTimezone,omitzero" db:"weight_timezone"`
	WeightSource     AthleteMeasurementSource `json:"weightSource,omitzero" db:"weight_source"`
	WeightNotes      string                   `json:"weightNotes,omitzero" db:"weight_notes"`
}

type AthleteVolumeData struct {
	Period                   string `json:"period"`
	ActivityCount            int32  `json:"activityCount"`
	TotalDistanceMeters      int32  `json:"totalDistanceMeters"`
	TotalElapsedTimeSeconds  int64  `json:"totalElapsedTimeSeconds"`
	TotalMovingTimeSeconds   int64  `json:"totalMovingTimeSeconds"`
	TotalElevationGainMeters int32  `json:"totalElevationGainMeters"`
}

type GetAthleteVolumeParams struct {
	Frequency    string         `json:"frequency"`
	AthleteID    uuid.UUID      `json:"athleteId"`
	ProviderSlug string         `json:"providerSlug"`
	Sports       []stride.Sport `json:"sports"`
	StartDate    time.Time      `json:"startDate"`
}

// Reader defines the interface for read-only database operations.
// It's implemented by DBStore.
type Reader interface {
	GetActivityEndurance(ctx context.Context, id uuid.UUID) (*activity.EnduranceActivity, error)
	ListAthleteActivitiesEndurance(ctx context.Context, providerID int, athleteID uuid.UUID) ([]*activity.EnduranceActivity, error)
	ListActivitiesEnduranceByTag(ctx context.Context, providerID int, athleteID uuid.UUID, tag string) ([]*activity.EnduranceActivity, error)
	GetActivityTags(ctx context.Context, activityID uuid.UUID) ([]*activity.ActivityTag, error)
	GetAthlete(ctx context.Context, athleteID uuid.UUID) (*Athlete, error)
	GetUserAthletes(ctx context.Context, userID uuid.UUID) ([]*Athlete, error)
	GetAthleteCurrentMeasurements(ctx context.Context, athleteID uuid.UUID) (*AthleteCurrentMeasurements, error)
	GetAthleteVolume(ctx context.Context, params GetAthleteVolumeParams) (map[stride.Sport][]*AthleteVolumeData, error)
}

// Store defines the interface for read and write database operations.
// It's implemented by DBStore.
type Store interface {
	Reader
	UpsertAthlete(ctx context.Context, arg *Athlete) (*Athlete, error)
	UpsertActivityEndurance(ctx context.Context, arg *activity.EnduranceActivity) (*activity.EnduranceActivity, error)
	UpsertActivityThresholdAnalysis(ctx context.Context, arg *activity.ThresholdAnalysis) (*activity.ThresholdAnalysis, error)
	UpsertTagsAndLinkActivity(ctx context.Context, a *activity.EnduranceActivity, tags []*activity.ActivityTag) error
	SaveProviderActivityRawData(ctx context.Context, arg *activity.ProviderActivityRawData) (uuid.UUID, error)
}
