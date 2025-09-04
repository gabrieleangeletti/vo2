package internal

import (
	"errors"
	"log/slog"
	"time"

	"github.com/schollz/progressbar/v3"

	"github.com/gabrieleangeletti/stride"
	"github.com/gabrieleangeletti/stride/strava"
)

func GetStravaActivitySummaries(client *strava.Client, startTime, endTime time.Time) ([]strava.ActivitySummary, error) {
	page := 1

	var activities []strava.ActivitySummary

	bar := progressbar.Default(-1)

	for {
		err := bar.Add(1)
		if err != nil {
			slog.Error("Error adding to progress bar", "error", err)
			break
		}

		pageActivities, err := client.GetActivitySummaries(startTime, endTime, page)
		if err != nil {
			if errors.Is(err, stride.ErrRateLimitExceeded) {
				slog.Error("Rate limit exceeded", "error", err)
				break
			}

			slog.Error("Error getting activities", "error", err)
			break
		}

		activities = append(activities, pageActivities...)

		if len(pageActivities) < 200 {
			break
		}

		page++
	}

	if len(activities) == 0 {
		slog.Info("No activities found")
		return nil, nil
	}

	err := bar.Finish()
	if err != nil {
		slog.Error("Error finishing progress bar", "error", err)
	}

	return activities, nil
}
