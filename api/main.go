package main

import (
	"log"
	"net/http"

	"github.com/gabrieleangeletti/vo2/core"
)

func main() {
	db, err := core.NewDB(core.CfgDB{
		Host:     core.GetSecret("POSTGRES_HOST", true),
		Port:     core.GetSecret("POSTGRES_PORT", true),
		User:     core.GetSecret("POSTGRES_USER", true),
		Password: core.GetSecret("POSTGRES_PASSWORD", true),
		DB:       core.GetSecret("POSTGRES_DB", true),
	})
	if err != nil {
		panic(err)
	}
	defer db.Close()

	h := NewHandler(db)

	if err := stravaAuthHandler(h); err != nil {
		log.Fatal(err)
	}

	server := &http.Server{
		Addr:    ":8080",
		Handler: h,
	}

	err = server.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}
}
