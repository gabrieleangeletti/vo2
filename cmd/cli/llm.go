package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"

	"github.com/google/uuid"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"

	"github.com/gabrieleangeletti/stride"
	"github.com/gabrieleangeletti/vo2/activity"
)

func llmSummarizeCmd(cfg config) *cobra.Command {
	return &cobra.Command{
		Use:   "llm-summarize",
		Short: "Generate LLM summaries for activities",
		Long:  `Generate LLM summaries for activities`,
		Run: func(cmd *cobra.Command, args []string) {
			providerID, err := strconv.Atoi(args[0])
			if err != nil {
				log.Fatal(err)
			}

			athleteID := uuid.MustParse(args[1])

			ctx := cmd.Context()

			activities, err := cfg.store.ListAthleteActivitiesEndurance(ctx, providerID, athleteID)
			if err != nil {
				log.Fatal(err)
			}

			if len(activities) == 0 {
				log.Fatal("No activities to process")
			}

			bar := progressbar.Default(int64(len(activities)))

			for _, act := range activities {
				err := bar.Add(1)
				if err != nil {
					log.Fatal(err)
				}

				strideAct, ts, err := cfg.store.GetActivityGPXFromMemory(ctx, act)
				if err != nil {
					if errors.Is(err, activity.ErrNoGPXFile) {
						continue
					}
					log.Fatal(err)
				}

				config := stride.LLMSummaryConfig{
					ElevationHysteresisM: 3.0,
					MinSegmentDistM:      100.0,
					Athlete: stride.AthleteBaseline{
						MaxHR:     196,
						RestingHR: 46,
						AeTHR:     149,
						AnTHR:     173,
					},
				}

				summary, err := stride.SummarizeForLLM(strideAct, ts, config)
				if err != nil {
					log.Fatal(err)
				}

				output := map[string]any{
					"activityId":    act.ID,
					"activityName":  act.Name,
					"startTime":     act.StartTime,
					"sport":         act.Sport,
					"llmRunSummary": summary,
				}

				out, err := json.Marshal(output)
				if err != nil {
					log.Fatal(err)
				}

				fmt.Println(string(out))
				break
			}

			err = bar.Finish()
			if err != nil {
				log.Fatal(err)
			}
		},
	}
}
