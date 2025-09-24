package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"slices"
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
	"github.com/gabrieleangeletti/vo2/store"
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
				log.Fatal(err)
			}

			userID := uuid.MustParse(args[1])

			ctx := cmd.Context()

			rawActivities, err := activity.GetProviderActivityRawData(ctx, cfg.DB, providerID, userID)
			if err != nil {
				log.Fatal(err)
			}

			providerMap, err := provider.GetMap(ctx, cfg.DB)
			if err != nil {
				log.Fatal(err)
			}

			store := store.NewStore(cfg.DB)

			bar := progressbar.Default(int64(len(rawActivities)))

			slices.SortFunc(rawActivities, func(a *activity.ProviderActivityRawData, b *activity.ProviderActivityRawData) int {
				if a.StartTime.Before(b.StartTime) {
					return 1
				}

				if a.StartTime.After(b.StartTime) {
					return -1
				}

				return 0
			})

			for _, raw := range rawActivities {
				err := bar.Add(1)
				if err != nil {
					log.Fatal(err)
				}

				act, err := raw.ToEnduranceOutdoorActivity(providerMap)
				if err != nil {
					if errors.Is(err, stride.ErrActivityIsNotOutdoorEndurance) ||
						errors.Is(err, stride.ErrUnsupportedSportType) {
						continue
					}

					log.Fatal(err)
				}

				act, err = store.UpsertActivityEnduranceOutdoor(ctx, act)
				if err != nil {
					log.Fatal(err)
				}

				if raw.DetailedActivityURI.Valid {
					data, err := internal.DownloadObject(ctx, raw.DetailedActivityURI.String)
					if err != nil {
						log.Fatal(err)
					}

					var streams strava.ActivityStream
					err = json.Unmarshal(data, &streams)
					if err != nil {
						log.Fatal(err)
					}

					var stravaActivity strava.ActivityDetailed
					err = json.Unmarshal(raw.Data, &stravaActivity)
					if err != nil {
						log.Fatal(err)
					}

					strideActivity, err := stravaActivity.ToActivity()
					if err != nil {
						log.Fatal(err)
					}

					ts, err := streams.ToTimeseries(raw.StartTime)
					if err != nil {
						log.Fatal(err)
					}

					sport, err := stravaActivity.Sport()
					if err != nil {
						log.Fatal(err)
					}

					gpxData, err := stride.CreateGPXFileInMemory(strideActivity, ts, sport)
					if err != nil {
						log.Fatal(err)
					}

					objectKey := fmt.Sprintf("activity_details/%s/gpx/%s.gpx", "strava", act.ID)

					res, err := internal.UploadObject(ctx, objectKey, gpxData, nil)
					if err != nil {
						log.Fatal(fmt.Errorf("Error uploading activity streams: %w", err))
					}

					act.GpxFileURI = res.Location

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
						avgValue := int16(math.Round(avgHR))
						act.AvgHR = avgValue
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
						maxValue := int16(maxHR)
						act.MaxHR = maxValue
					}

					_, err = store.UpsertActivityEnduranceOutdoor(ctx, act)
					if err != nil {
						log.Fatal(err)
					}
				}

				tags := act.ExtractActivityTags()
				if len(tags) > 0 {
					err = store.UpsertTagsAndLinkActivity(ctx, act, tags)
					if err != nil {
						log.Fatal(err)
					}
				}
			}

			err = bar.Finish()
			if err != nil {
				log.Fatal(err)
			}
		},
	}
}
