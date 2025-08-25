package main

import (
	"log"
	"net/http"

	"github.com/gabrieleangeletti/vo2/internal"
)

func main() {
	db, err := internal.NewDB(internal.CfgDB{
		Host:     internal.GetSecret("POSTGRES_HOST", true),
		Port:     internal.GetSecret("POSTGRES_PORT", true),
		User:     internal.GetSecret("POSTGRES_USER", true),
		Password: internal.GetSecret("POSTGRES_PASSWORD", true),
		DB:       internal.GetSecret("POSTGRES_DB", true),
	})
	if err != nil {
		panic(err)
	}
	defer db.Close()

	h := internal.NewHandler(db)

	server := &http.Server{
		Addr:    ":8080",
		Handler: h,
	}

	err = server.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}
}
