package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"

	"github.com/gabrieleangeletti/stride"
	"github.com/gabrieleangeletti/stride/strava"
	"github.com/gabrieleangeletti/vo2/activity"
	"github.com/gabrieleangeletti/vo2/internal"
	"github.com/gabrieleangeletti/vo2/provider"
)

func newActivityCmd(cfg config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "activity",
		Short: "Activity cli",
		Long:  `Activity cli`,
	}

	cmd.AddCommand(normalizeActivityCmd(cfg))

	return cmd
}

func normalizeActivityCmd(cfg config) *cobra.Command {
	return &cobra.Command{
		Use:   "normalize",
		Short: "Normalize activity data",
		Long:  `Normalize activity data`,
		Run: func(cmd *cobra.Command, args []string) {
			providerID, err := strconv.Atoi(args[0])
			if err != nil {
				panic(err)
			}

			userID := uuid.MustParse(args[1])

			ctx := cmd.Context()

			rawActivities, err := activity.GetProviderActivityRawData(cfg.DB, providerID, userID)
			if err != nil {
				panic(err)
			}

			providerMap, err := provider.GetMap(cfg.DB)
			if err != nil {
				panic(err)
			}

			activityRepo := activity.NewEnduranceOutdoorActivityRepo(cfg.DB)

			bar := progressbar.Default(int64(len(rawActivities)))

			for _, raw := range rawActivities {
				err := bar.Add(1)
				if err != nil {
					panic(err)
				}

				act, err := raw.ToEnduranceOutdoorActivity(providerMap)
				if err != nil {
					if errors.Is(err, stride.ErrActivityIsNotOutdoorEndurance) ||
						errors.Is(err, stride.ErrUnsupportedSportType) {
						continue
					}

					panic(err)
				}

				actID, err := activityRepo.Upsert(ctx, act)
				if err != nil {
					panic(err)
				}

				act.ID = actID

				if raw.DetailedActivityURI.Valid {
					data, err := internal.DownloadObject(ctx, raw.DetailedActivityURI.String)
					if err != nil {
						panic(err)
					}

					var streams strava.ActivityStream
					err = json.Unmarshal(data, &streams)
					if err != nil {
						panic(err)
					}

					var stravaActivity strava.ActivityDetailed
					err = json.Unmarshal(raw.Data, &stravaActivity)
					if err != nil {
						panic(err)
					}

					strideActivity, err := stravaActivity.ToActivity()
					if err != nil {
						panic(err)
					}

					ts, err := streams.ToTimeseries(raw.StartTime)
					if err != nil {
						panic(err)
					}

					sport, err := stravaActivity.Sport()
					if err != nil {
						panic(err)
					}

					gpxData, err := stride.CreateGPXFileInMemory(strideActivity, ts, sport)
					if err != nil {
						panic(err)
					}

					objectKey := fmt.Sprintf("activity_details/%s/gpx/%s.gpx", "strava", act.ID)

					res, err := internal.UploadObject(ctx, objectKey, gpxData, nil)
					if err != nil {
						panic(fmt.Errorf("Error uploading activity streams: %w", err))
					}

					act.GpxFileURI = sql.NullString{
						String: res.Location,
						Valid:  true,
					}

					avgHR, err := stride.CalculateAverageHeartRate(ts, stride.AvgHeartRateAnalysisConfig{
						Method:       stride.HeartRateMethodTimeWeighted,
						ExcludeZeros: true,
						MinValidRate: 40,
						MaxValidRate: 220,
						MaxHeartRate: 193,
					})
					if err != nil {
						if !errors.Is(err, stride.ErrNoValidData) {
							fmt.Printf("Error: %v\n", err)
							return
						}
					}

					if avgHR > 0 {
						act.AvgHR = sql.NullInt16{Valid: true, Int16: int16(math.Round(avgHR))}
					}

					thirtySec := 30 * time.Second

					maxHR, err := stride.CalculateMaxHeartRate(ts, stride.MaxHeartRateAnalysisConfig{
						Method:         stride.MaxHeartRateMethodRollingWindow,
						WindowDuration: &thirtySec,
					})
					if err != nil {
						if !errors.Is(err, stride.ErrNoValidData) {
							fmt.Printf("Error: %v\n", err)
							return
						}
					}

					if maxHR > 0 {
						act.MaxHR = sql.NullInt16{Valid: true, Int16: int16(maxHR)}
					}

					_, err = activityRepo.Upsert(ctx, act)
					if err != nil {
						panic(err)
					}
				}

				tags := act.ExtractActivityTags()
				if len(tags) > 0 {
					err = activityRepo.UpsertTagsAndLinkActivity(ctx, act, tags)
					if err != nil {
						panic(err)
					}
				}
			}

			err = bar.Finish()
			if err != nil {
				panic(err)
			}
		},
	}
}
