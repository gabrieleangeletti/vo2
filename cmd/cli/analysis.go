package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/olekukonko/tablewriter"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"

	"github.com/gabrieleangeletti/stride"
)

func newAnalysisCmd(cfg config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analysis",
		Short: "Analysis cli",
		Long:  `Analysis cli`,
	}

	cmd.AddCommand(evalThresholdAnalysisCmd(cfg))

	return cmd
}

type thresholdAnalysis struct {
	ID       string    `json:"id"`
	Date     time.Time `json:"date"`
	Sport    string    `json:"sport"`
	Kind     string    `json:"kind"`
	Expected int       `json:"expected"`
}

func evalThresholdAnalysisCmd(cfg config) *cobra.Command {
	return &cobra.Command{
		Use:   "eval-threshold-analysis",
		Short: "Evaluate activity thresholds",
		Long:  `Evaluate activity thresholds`,
		Run: func(cmd *cobra.Command, args []string) {
			athleteID := uuid.MustParse(args[0])

			ctx := cmd.Context()

			datasetRaw, err := os.ReadFile("data/datasets/threshold_analysis.json")
			if err != nil {
				log.Fatal(err)
			}

			var dataset []thresholdAnalysis
			err = json.Unmarshal(datasetRaw, &dataset)
			if err != nil {
				log.Fatal(err)
			}

			datasetMap := make(map[uuid.UUID]thresholdAnalysis)
			for _, row := range dataset {
				datasetMap[uuid.MustParse(row.ID)] = row
			}

			measurements, err := cfg.store.GetAthleteCurrentMeasurements(ctx, athleteID)
			if err != nil {
				log.Fatal(err)
			}

			ids := make([]uuid.UUID, len(dataset))
			for _, row := range dataset {
				ids = append(ids, uuid.MustParse(row.ID))
			}

			activities, err := cfg.store.ListAthleteActivitiesEnduranceByIDs(ctx, ids)
			if err != nil {
				log.Fatal("Error getting activities: ", err)
			}

			tableData := make([][]string, 0, len(activities))

			yTrue := make([]float64, len(activities))
			yPred := make([]float64, len(activities))

			bar := progressbar.Default(int64(len(activities)))

			for _, act := range activities {
				err := bar.Add(1)
				if err != nil {
					log.Fatal(err)
				}

				if act.GpxFileURI == "" {
					slog.Warn("Activity has no GPX file", "id", act.ID)
					continue
				}

				data, err := cfg.store.GetObjectStore().DownloadObject(ctx, act.GpxFileURI)
				if err != nil {
					log.Fatal(err)
				}

				_, ts, err := stride.ParseGPXFileFromMemory(data)
				if err != nil {
					if errors.Is(err, stride.ErrNoTrackPoints) {
						slog.Warn("Activity has no track points", "id", act.ID)
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

				datasetEntry := datasetMap[act.ID]

				switch datasetEntry.Kind {
				case "lt1":
					tableData = append(tableData, []string{
						act.ID.String(),
						datasetEntry.Sport,
						datasetEntry.Kind,
						strconv.Itoa(result.TimeAtLT1Seconds),
						strconv.Itoa(datasetEntry.Expected),
					})

					yTrue = append(yTrue, float64(datasetEntry.Expected))
					yPred = append(yPred, float64(result.TimeAtLT1Seconds))
				case "lt2":
					tableData = append(tableData, []string{
						act.ID.String(),
						datasetEntry.Sport,
						datasetEntry.Kind,
						strconv.Itoa(result.TimeAtLT2Seconds),
						strconv.Itoa(datasetEntry.Expected),
					})

					yTrue = append(yTrue, float64(datasetEntry.Expected))
					yPred = append(yPred, float64(result.TimeAtLT2Seconds))
				default:
					log.Fatal("Unknown dataset element kind")
				}
			}

			err = bar.Finish()
			if err != nil {
				log.Fatal(err)
			}

			table := tablewriter.NewWriter(os.Stdout)
			table.Header([]string{"ID", "Sport", "Kind", "Time @ LT (Actual)", "Time @ LT (Expected)"})
			table.Bulk(tableData)
			table.Render()

			rmse, err := RMSE(yTrue, yPred)
			if err != nil {
				log.Fatal(err)
			}

			mae, err := MAE(yTrue, yPred)
			if err != nil {
				log.Fatal(err)
			}

			fmt.Printf("RMSE: %.3f\n", rmse)
			fmt.Printf("MAE:  %.3f\n", mae)
		},
	}
}

// RMSE computes the Root Mean Squared Error between predicted and actual values.
// Returns an error if the slices have different lengths or are empty.
func RMSE(yTrue, yPred []float64) (float64, error) {
	if len(yTrue) == 0 || len(yPred) == 0 {
		return 0, errors.New("input slices must not be empty")
	}
	if len(yTrue) != len(yPred) {
		return 0, errors.New("input slices must have the same length")
	}

	var sumSq float64
	for i := range yTrue {
		diff := yTrue[i] - yPred[i]
		sumSq += diff * diff
	}

	rmse := math.Sqrt(sumSq / float64(len(yTrue)))
	return rmse, nil
}

// MAE computes the Mean Absolute Error between predicted and actual values.
// Returns an error if the slices have different lengths or are empty.
func MAE(yTrue, yPred []float64) (float64, error) {
	if len(yTrue) == 0 || len(yPred) == 0 {
		return 0, errors.New("input slices must not be empty")
	}
	if len(yTrue) != len(yPred) {
		return 0, errors.New("input slices must have the same length")
	}

	var sumAbs float64
	for i := range yTrue {
		diff := math.Abs(yTrue[i] - yPred[i])
		sumAbs += diff
	}

	mae := sumAbs / float64(len(yTrue))
	return mae, nil
}
