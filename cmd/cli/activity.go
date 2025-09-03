package main

import (
	"context"
	"errors"
	"strconv"

	"github.com/google/uuid"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"

	"github.com/gabrieleangeletti/stride"
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

			ctx := context.Background()

			rawActivities, err := activity.GetProviderActivityRawData(cfg.DB, providerID, userID)
			if err != nil {
				panic(err)
			}

			providerMap, err := provider.GetMap(cfg.DB)
			if err != nil {
				panic(err)
			}

			activityRepo := activity.NewEnduranceOutdoorActivityRepo(cfg.DB)
			tagRepo := activity.NewActivityTagRepo(cfg.DB)

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

				tags := act.ActivityTags()
				if len(tags) > 0 {
					tags, err = tagRepo.Upsert(ctx, tags)
					if err != nil {
						panic(err)
					}

					tagRepo.TagEnduranceOutdoorActivity(ctx, act, tags)
				}
			}

			err = bar.Finish()
			if err != nil {
				panic(err)
			}
		},
	}
}
