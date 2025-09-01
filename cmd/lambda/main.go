package main

import (
	"github.com/aws/aws-lambda-go/lambda"

	"github.com/gabrieleangeletti/vo2/internal"
)

func main() {
	lambdaHandler := internal.NewLambdaHandler()
	lambda.Start(lambdaHandler.HandleRequest)
}
