package vo2

import (
	"context"

	"github.com/google/uuid"

	"github.com/gabrieleangeletti/vo2/activity"
)

// Reader defines the interface for read-only database operations.
// It's implemented by DBStore.
type Reader interface {
	GetActivityEnduranceOutdoor(ctx context.Context, id uuid.UUID) (*activity.EnduranceOutdoorActivity, error)
	ListActivitiesEnduranceOutdoorByTag(ctx context.Context, providerID int, userID uuid.UUID, tag string) ([]*activity.EnduranceOutdoorActivity, error)
	GetActivityTags(ctx context.Context, activityID uuid.UUID) ([]*activity.ActivityTag, error)
}

// Store defines the interface for read and write database operations.
// It's implemented by DBStore.
type Store interface {
	Reader
	UpsertActivityEnduranceOutdoor(ctx context.Context, arg *activity.EnduranceOutdoorActivity) (*activity.EnduranceOutdoorActivity, error)
	UpsertTagsAndLinkActivity(ctx context.Context, a *activity.EnduranceOutdoorActivity, tags []*activity.ActivityTag) error
	SaveProviderActivityRawData(ctx context.Context, arg *activity.ProviderActivityRawData) error
}
