package main

import (
	"log"

	"github.com/aws/aws-lambda-go/lambda"

	"github.com/gabrieleangeletti/vo2/internal"
)

func main() {
	db, err := internal.NewDB(internal.DefaultDBConfig())
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	defer db.Close()

	lambdaHandler := internal.NewLambdaHandler(db)
	lambda.Start(lambdaHandler.HandleRequest)
}
