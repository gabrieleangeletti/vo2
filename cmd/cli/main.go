package main

import (
	"log"
	"log/slog"
	"os"

	"github.com/jmoiron/sqlx"
	"github.com/spf13/cobra"

	"github.com/gabrieleangeletti/vo2/database"
	"github.com/gabrieleangeletti/vo2/internal"
)

type config struct {
	DB *sqlx.DB
}

func newRootCmd(cfg config) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "vo2",
		Short: "vo2 CLI",
		Long:  "vo2 CLI",
	}

	rootCmd.AddCommand(newProviderCmd(cfg))
	rootCmd.AddCommand(newActivityCmd(cfg))
	rootCmd.AddCommand(newAnalysisCmd(cfg))

	return rootCmd
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	db, err := database.NewDB(internal.DefaultDBConfig())
	if err != nil {
		panic(err)
	}

	cfg := config{
		DB: db,
	}

	rootCmd := newRootCmd(cfg)
	err = rootCmd.Execute()
	if err != nil {
		log.Fatal(err)
	}
}
