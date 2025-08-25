package main

import (
	"log"
	"log/slog"
	"os"

	"github.com/jmoiron/sqlx"
	"github.com/spf13/cobra"

	"github.com/gabrieleangeletti/vo2/internal"
)

type Config struct {
	DB *sqlx.DB
}

func NewRootCmd(cfg Config) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "vo2",
		Short: "vo2 CLI",
		Long:  "vo2 CLI",
	}

	rootCmd.AddCommand(NewProviderCmd(cfg))

	return rootCmd
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	db, err := internal.NewDB(internal.DefaultDBConfig())
	if err != nil {
		panic(err)
	}

	cfg := Config{
		DB: db,
	}

	rootCmd := NewRootCmd(cfg)
	err = rootCmd.Execute()
	if err != nil {
		log.Fatal(err)
	}
}
