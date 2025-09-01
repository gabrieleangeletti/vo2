package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	"github.com/gabrieleangeletti/vo2/internal"
)

func main() {
	lambdaHandler := internal.NewLambdaHandler()
	lambda.Start(func(ctx context.Context, event json.RawMessage) (any, error) {
		return handleEvent(ctx, lambdaHandler, event)
	})
}

func handleEvent(ctx context.Context, handler *internal.LambdaHandler, event json.RawMessage) (any, error) {
	var sqsEvent events.SQSEvent
	if err := json.Unmarshal(event, &sqsEvent); err == nil && len(sqsEvent.Records) > 0 {
		if sqsEvent.Records[0].EventSource == "aws:sqs" {
			return nil, handler.HandleSQSRequest(ctx, sqsEvent)
		}
	}

	var httpRequest events.APIGatewayV2HTTPRequest
	if err := json.Unmarshal(event, &httpRequest); err == nil && httpRequest.RequestContext.HTTP.Method != "" {
		return handler.HandleRequest(ctx, httpRequest)
	}

	return nil, fmt.Errorf("unsupported event type")
}
