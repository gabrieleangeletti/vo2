package internal

import (
	"context"
	"encoding/json"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/google/uuid"

	"github.com/gabrieleangeletti/vo2/provider"
	"github.com/gabrieleangeletti/vo2/util"
)

type SQSTaskType string

const (
	TaskTypeHistoricalData      SQSTaskType = "historical_data"
	TaskTypePostProcessActivity SQSTaskType = "post_process_activity"
)

type SQSTaskMessage struct {
	Type SQSTaskType     `json:"type"`
	Data json.RawMessage `json:"data"`
}

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

type PostProcessActivityTask struct {
	AthleteID     uuid.UUID         `json:"athleteId"`
	Provider      provider.Provider `json:"provider"`
	RawActivityID uuid.UUID         `json:"rawActivityId"`
}

type SQSClient struct {
	client                 *sqs.Client
	historicalQueueURL     string
	postProcessingQueueURL string
}

func NewSQSClient() (*SQSClient, error) {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, err
	}

	client := sqs.NewFromConfig(cfg)

	historicalQueueURL := util.GetSecret("HISTORICAL_DATA_QUEUE_URL", true)
	postProcessingQueueURL := util.GetSecret("POST_PROCESSING_QUEUE_URL", true)

	return &SQSClient{
		client:                 client,
		historicalQueueURL:     historicalQueueURL,
		postProcessingQueueURL: postProcessingQueueURL,
	}, nil
}

func (s *SQSClient) SendHistoricalDataTask(ctx context.Context, task HistoricalDataTask) error {
	taskJSON, err := json.Marshal(task)
	if err != nil {
		return err
	}

	message := SQSTaskMessage{
		Type: TaskTypeHistoricalData,
		Data: taskJSON,
	}

	messageJSON, err := json.Marshal(message)
	if err != nil {
		return err
	}

	_, err = s.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:       aws.String(s.historicalQueueURL),
		MessageBody:    aws.String(string(messageJSON)),
		MessageGroupId: aws.String(task.AthleteID.String()),
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *SQSClient) SendPostProcessActivityTask(ctx context.Context, task PostProcessActivityTask) error {
	taskJSON, err := json.Marshal(task)
	if err != nil {
		return err
	}

	message := SQSTaskMessage{
		Type: TaskTypePostProcessActivity,
		Data: taskJSON,
	}

	messageJSON, err := json.Marshal(message)
	if err != nil {
		return err
	}

	_, err = s.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(s.postProcessingQueueURL),
		MessageBody: aws.String(string(messageJSON)),
	})
	return err
}
