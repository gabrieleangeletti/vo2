package main

import (
	"encoding/json"
	"errors"
	"log"
	"slices"
	"strconv"

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
	cmd.AddCommand(analyzeActivityThresholdsCmd(cfg))

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

			athleteID := uuid.MustParse(args[1])

			ctx := cmd.Context()

			rawActivities, err := activity.GetProviderActivityRawData(ctx, cfg.DB, providerID, athleteID)
			if err != nil {
				log.Fatal(err)
			}

			providerMap, err := provider.GetMap(ctx, cfg.DB)
			if err != nil {
				log.Fatal(err)
			}

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

				prov, ok := providerMap[raw.ProviderID]
				if !ok {
					log.Fatalf("provider %d not found", raw.ProviderID)
				}

				var stravaActivity strava.ActivityDetailed
				err = json.Unmarshal(raw.Data, &stravaActivity)
				if err != nil {
					log.Fatal(err)
				}

				var streams *strava.ActivityStream

				if !raw.DetailedActivityURI.Valid {
					driver := internal.NewStravaDriver()

					credentials, err := internal.EnsureValidCredentials(ctx, cfg.DB, driver, &prov, athleteID)
					if err != nil {
						log.Fatal(err)
					}

					client := driver.NewClient(credentials.AccessToken)

					streams, err = client.GetActivityStreams(stravaActivity.ID)
					if err != nil {
						log.Fatal(err)
					}

					err = cfg.store.UploadRawActivityDetails(ctx, stride.ProviderStrava, raw, streams)
					if err != nil {
						log.Fatal(err)
					}
				} else {
					err = cfg.store.GetActivityRawTimeseries(ctx, raw, streams)
					if err != nil {
						log.Fatal(err)
					}
				}

				act, err := cfg.store.StoreActivityEndurance(ctx, stride.Provider(prov.Slug), raw, stravaActivity, streams)
				if err != nil {
					log.Fatal(err)
				}

				tags := act.ExtractActivityTags()
				if len(tags) > 0 {
					err = cfg.store.UpsertTagsAndLinkActivity(ctx, act, tags)
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

func analyzeActivityThresholdsCmd(cfg config) *cobra.Command {
	return &cobra.Command{
		Use:   "analyze-thresholds",
		Short: "Analyze activity thresholds",
		Long:  `Analyze activity thresholds`,
		Run: func(cmd *cobra.Command, args []string) {
			providerID, err := strconv.Atoi(args[0])
			if err != nil {
				log.Fatal(err)
			}

			athleteID := uuid.MustParse(args[1])

			ctx := cmd.Context()

			measurements, err := cfg.store.GetAthleteCurrentMeasurements(ctx, athleteID)
			if err != nil {
				log.Fatal(err)
			}

			activities, err := cfg.store.ListAthleteActivitiesEndurance(ctx, providerID, athleteID)
			if err != nil {
				log.Fatal(err)
			}

			bar := progressbar.Default(int64(len(activities)))

			for _, act := range activities {
				err := bar.Add(1)
				if err != nil {
					log.Fatal(err)
				}

				ts, err := cfg.store.GetActivityTimeseries(ctx, act)
				if err != nil {
					if errors.Is(err, activity.ErrNoGPXFile) || errors.Is(err, stride.ErrNoTrackPoints) {
						_, err = cfg.store.UpsertActivityThresholdAnalysis(ctx, &activity.ThresholdAnalysis{
							ActivityEnduranceID: act.ID,
							TimeAtLt1Threshold:  0,
							TimeAtLt2Threshold:  0,
							RawAnalysis:         []byte("{}"),
						})
						continue
					}
					log.Fatal(err)
				}

				result, err := stride.AnalyzeHeartRateThresholds(ts, stride.HRThresholdAnalysisConfig{
					LT1:                        uint8(measurements.Lt1Value),
					LT2:                        uint8(measurements.Lt2Value),
					BucketSizeSeconds:          40,
					MinValidPointsPerBucket:    5,
					ThresholdTolerancePercent:  0.05,
					LT1OverlapTolerancePercent: 0.05,
					MinConsecutiveBuckets:      6,
					ConsecutivePeriodThreshold: 0.80,
				})
				if err != nil {
					log.Fatal(err)
				}

				rawAnalysis, err := json.Marshal(result)
				if err != nil {
					log.Fatal(err)
				}

				_, err = cfg.store.UpsertActivityThresholdAnalysis(ctx, &activity.ThresholdAnalysis{
					ActivityEnduranceID: act.ID,
					TimeAtLt1Threshold:  int32(result.TimeAtLT1Seconds),
					TimeAtLt2Threshold:  int32(result.TimeAtLT2Seconds),
					RawAnalysis:         rawAnalysis,
				})
				if err != nil {
					log.Fatal(err)
				}
			}

			err = bar.Finish()
			if err != nil {
				log.Fatal(err)
			}
		},
	}
}
