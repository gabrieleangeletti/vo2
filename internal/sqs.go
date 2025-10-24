package internal

import (
	"context"
	"encoding/json"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/google/uuid"
)

type HistoricalDataTaskType string

const (
	HistoricalDataTaskTypeActivity HistoricalDataTaskType = "activity"
)

type HistoricalDataTask struct {
	AthleteID  uuid.UUID              `json:"athleteId"`
	ProviderID int                    `json:"providerId"`
	Type       HistoricalDataTaskType `json:"type"`
	StartTime  time.Time              `json:"startTime"`
	EndTime    time.Time              `json:"endTime"`
}

type SQSClient struct {
	client   *sqs.Client
	queueURL string
}

func NewSQSClient() (*SQSClient, error) {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, err
	}

	client := sqs.NewFromConfig(cfg)
	queueURL := GetSecret("HISTORICAL_DATA_QUEUE_URL", true)

	return &SQSClient{
		client:   client,
		queueURL: queueURL,
	}, nil
}

func (s *SQSClient) SendHistoricalDataTask(ctx context.Context, task HistoricalDataTask) error {
	taskJSON, err := json.Marshal(task)
	if err != nil {
		return err
	}

	_, err = s.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:       aws.String(s.queueURL),
		MessageBody:    aws.String(string(taskJSON)),
		MessageGroupId: aws.String(task.AthleteID.String()),
	})
	if err != nil {
		return err
	}

	return nil
}
