package main

import (
	"log"
	"net/http"

	"github.com/gabrieleangeletti/vo2/database"
	"github.com/gabrieleangeletti/vo2/internal"
)

func main() {
	db, err := database.NewDB(internal.DefaultDBConfig())
	if err != nil {
		panic(err)
	}
	defer db.Close()

	h := internal.NewHandler(db, internal.WithHttpLogging(internal.Development))

	server := &http.Server{
		Addr:    ":8080",
		Handler: h,
	}

	err = server.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}
}
