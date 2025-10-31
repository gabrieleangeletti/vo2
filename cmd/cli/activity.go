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

				act, err := raw.ToEnduranceActivity(providerMap)
				if err != nil {
					if errors.Is(err, stride.ErrActivityIsNotEndurance) ||
						errors.Is(err, stride.ErrUnsupportedSportType) {
						continue
					}

					log.Fatal(err)
				}

				act, err = cfg.dbStore.UpsertActivityEndurance(ctx, act)
				if err != nil {
					log.Fatal(err)
				}

				if raw.DetailedActivityURI.Valid {
					data, err := cfg.objectStore.DownloadObject(ctx, raw.DetailedActivityURI.String)
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

					gpxData, err := stride.CreateGPXFileInMemory(strideActivity, ts)
					if err != nil {
						log.Fatal(err)
					}

					objectKey := fmt.Sprintf("activity_details/%s/gpx/%s.gpx", "strava", act.ID)

					res, err := cfg.objectStore.UploadObject(ctx, objectKey, gpxData, nil)
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

					_, err = cfg.dbStore.UpsertActivityEndurance(ctx, act)
					if err != nil {
						log.Fatal(err)
					}
				}

				tags := act.ExtractActivityTags()
				if len(tags) > 0 {
					err = cfg.dbStore.UpsertTagsAndLinkActivity(ctx, act, tags)
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

			measurements, err := cfg.dbStore.GetAthleteCurrentMeasurements(ctx, athleteID)
			if err != nil {
				log.Fatal(err)
			}

			activities, err := cfg.dbStore.ListAthleteActivitiesEndurance(ctx, providerID, athleteID)
			if err != nil {
				log.Fatal(err)
			}

			bar := progressbar.Default(int64(len(activities)))

			for _, act := range activities {
				err := bar.Add(1)
				if err != nil {
					log.Fatal(err)
				}

				if act.GpxFileURI == "" {
					continue
				}

				data, err := cfg.objectStore.DownloadObject(ctx, act.GpxFileURI)
				if err != nil {
					log.Fatal(err)
				}

				_, ts, err := stride.ParseGPXFileFromMemory(data)
				if err != nil {
					if errors.Is(err, stride.ErrNoTrackPoints) {
						_, err = cfg.dbStore.UpsertActivityThresholdAnalysis(ctx, &activity.ThresholdAnalysis{
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

				_, err = cfg.dbStore.UpsertActivityThresholdAnalysis(ctx, &activity.ThresholdAnalysis{
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
