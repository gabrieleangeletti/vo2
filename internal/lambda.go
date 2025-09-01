package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/aws/aws-lambda-go/events"
	"github.com/jmoiron/sqlx"
)

type LambdaHandler struct {
	db       *sqlx.DB
	handler  *Handler
	initOnce sync.Once
	initErr  error
}

func NewLambdaHandler() *LambdaHandler {
	return &LambdaHandler{}
}

func (l *LambdaHandler) init() {
	l.initOnce.Do(func() {
		db, err := NewDB(DefaultDBConfig())
		if err != nil {
			l.initErr = fmt.Errorf("failed to initialize database: %w", err)
			return
		}
		l.db = db
		l.handler = NewHandler(db)
	})
}

func (l *LambdaHandler) HandleRequest(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	l.init()
	if l.initErr != nil {
		slog.Error("failed to initialize lambda handler: " + l.initErr.Error())
		return events.APIGatewayV2HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       fmt.Sprintf("Internal Server Error: %v", l.initErr),
		}, nil
	}

	httpRequest, err := l.convertToHTTPRequest(request)
	if err != nil {
		return events.APIGatewayV2HTTPResponse{
			StatusCode: 500,
			Body:       fmt.Sprintf("Failed to convert request: %v", err),
		}, nil
	}

	recorder := &ResponseRecorder{
		statusCode: 200,
		headers:    make(http.Header),
		body:       &bytes.Buffer{},
	}

	l.handler.ServeHTTP(recorder, httpRequest)

	response := events.APIGatewayV2HTTPResponse{
		StatusCode: recorder.statusCode,
		Body:       recorder.body.String(),
		Headers:    make(map[string]string),
	}

	for key, values := range recorder.headers {
		if len(values) > 0 {
			response.Headers[key] = values[0]
		}
	}

	return response, nil
}

func (l *LambdaHandler) HandleSQSRequest(ctx context.Context, sqsEvent events.SQSEvent) error {
	l.init()
	if l.initErr != nil {
		slog.Error("failed to initialize lambda handler: " + l.initErr.Error())
		return l.initErr
	}

	for _, record := range sqsEvent.Records {
		slog.Info("Processing SQS message", "messageId", record.MessageId)

		var task HistoricalDataTask
		if err := json.Unmarshal([]byte(record.Body), &task); err != nil {
			slog.Error("Failed to unmarshal SQS message", "error", err, "messageId", record.MessageId)
			continue
		}

		if err := l.handler.ProcessHistoricalDataTask(ctx, task); err != nil {
			slog.Error("Failed to process historical data task", "error", err, "messageId", record.MessageId, "userId", task.UserID)
			return err
		}

		slog.Info("Successfully processed SQS message", "messageId", record.MessageId, "userId", task.UserID)
	}

	return nil
}

func (l *LambdaHandler) convertToHTTPRequest(request events.APIGatewayV2HTTPRequest) (*http.Request, error) {
	u, err := url.Parse(request.RawPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse path: %w", err)
	}

	if request.RawQueryString != "" {
		u.RawQuery = request.RawQueryString
	}

	var body bytes.Buffer
	if request.Body != "" {
		if request.IsBase64Encoded {
			return nil, fmt.Errorf("base64 encoded bodies not supported")
		}
		body.WriteString(request.Body)
	}

	req, err := http.NewRequest(strings.ToUpper(request.RequestContext.HTTP.Method), u.String(), &body)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	for key, value := range request.Headers {
		req.Header.Set(key, value)
	}

	return req, nil
}

type ResponseRecorder struct {
	statusCode int
	headers    http.Header
	body       *bytes.Buffer
}

func (r *ResponseRecorder) Header() http.Header {
	return r.headers
}

func (r *ResponseRecorder) Write(data []byte) (int, error) {
	return r.body.Write(data)
}

func (r *ResponseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
}
