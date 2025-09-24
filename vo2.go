package vo2

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/gabrieleangeletti/vo2/activity"
)

type AthleteVolumeData struct {
	Period                   string `json:"period"`
	ActivityCount            int32  `json:"activityCount"`
	TotalDistanceMeters      int32  `json:"totalDistanceMeters"`
	TotalElapsedTimeSeconds  int64  `json:"totalElapsedTimeSeconds"`
	TotalMovingTimeSeconds   int64  `json:"totalMovingTimeSeconds"`
	TotalElevationGainMeters int32  `json:"totalElevationGainMeters"`
}

type GetAthleteVolumeParams struct {
	Frequency    string    `json:"frequency"`
	UserID       uuid.UUID `json:"userId"`
	ProviderSlug string    `json:"providerSlug"`
	Sport        string    `json:"sport"`
	StartDate    time.Time `json:"startDate"`
}

// Reader defines the interface for read-only database operations.
// It's implemented by DBStore.
type Reader interface {
	GetActivityEnduranceOutdoor(ctx context.Context, id uuid.UUID) (*activity.EnduranceOutdoorActivity, error)
	ListActivitiesEnduranceOutdoorByTag(ctx context.Context, providerID int, userID uuid.UUID, tag string) ([]*activity.EnduranceOutdoorActivity, error)
	GetActivityTags(ctx context.Context, activityID uuid.UUID) ([]*activity.ActivityTag, error)
	GetAthleteVolume(ctx context.Context, params GetAthleteVolumeParams) ([]*AthleteVolumeData, error)
}

// Store defines the interface for read and write database operations.
// It's implemented by DBStore.
type Store interface {
	Reader
	UpsertActivityEnduranceOutdoor(ctx context.Context, arg *activity.EnduranceOutdoorActivity) (*activity.EnduranceOutdoorActivity, error)
	UpsertTagsAndLinkActivity(ctx context.Context, a *activity.EnduranceOutdoorActivity, tags []*activity.ActivityTag) error
	SaveProviderActivityRawData(ctx context.Context, arg *activity.ProviderActivityRawData) (uuid.UUID, error)
}
