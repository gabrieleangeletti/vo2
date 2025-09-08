package internal

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"net/url"
	"path/filepath"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

var (
	s3Client   *s3.Client
	s3Once     sync.Once
	bucketName string
)

type UploadOptions struct {
	ContentType          string
	ACL                  types.ObjectCannedACL
	Metadata             map[string]string
	ServerSideEncryption *types.ServerSideEncryption
}

type UploadResult struct {
	Location string
	ETag     string
	Key      string
}

func getS3Client() *s3.Client {
	s3Once.Do(func() {
		cfg, err := config.LoadDefaultConfig(context.Background())
		if err != nil {
			panic(fmt.Sprintf("failed to load AWS config: %v", err))
		}

		s3Client = s3.NewFromConfig(cfg)
		bucketName = GetSecret("AWS_S3_BUCKET_NAME", true)
	})

	return s3Client
}

func UploadObject(ctx context.Context, key string, data []byte, opts *UploadOptions) (*UploadResult, error) {
	if opts == nil {
		opts = &UploadOptions{}
	}

	if opts.ContentType == "" {
		opts.ContentType = mime.TypeByExtension(filepath.Ext(key))
		if opts.ContentType == "" {
			opts.ContentType = "application/octet-stream"
		}
	}

	reader := bytes.NewReader(data)

	return uploadReader(ctx, reader, key, opts)
}

func uploadReader(ctx context.Context, reader io.Reader, key string, opts *UploadOptions) (*UploadResult, error) {
	client := getS3Client()

	putObjectInput := &s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(key),
		Body:        reader,
		ContentType: aws.String(opts.ContentType),
	}

	if opts.ACL != "" {
		putObjectInput.ACL = opts.ACL
	}

	if len(opts.Metadata) > 0 {
		putObjectInput.Metadata = opts.Metadata
	}

	if opts.ServerSideEncryption != nil {
		putObjectInput.ServerSideEncryption = *opts.ServerSideEncryption
	}

	result, err := client.PutObject(ctx, putObjectInput)
	if err != nil {
		return nil, fmt.Errorf("failed to upload to S3: %w", err)
	}

	location := fmt.Sprintf("https://%s.s3.amazonaws.com/%s", bucketName, key)

	return &UploadResult{
		Location: location,
		ETag:     aws.ToString(result.ETag),
		Key:      key,
	}, nil
}

func DownloadObject(ctx context.Context, keyOrURL string) ([]byte, error) {
	client := getS3Client()

	var inputBucket, inputKey string

	if strings.HasPrefix(keyOrURL, "https://") {
		u, err := url.Parse(keyOrURL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse S3 URL: %w", err)
		}

		// Assuming virtual-hosted-style URL: https://<bucket>.s3.amazonaws.com/<key>
		if !strings.HasSuffix(u.Host, ".s3.amazonaws.com") {
			return nil, fmt.Errorf("URL is not a recognized S3 URL format: %s", keyOrURL)
		}

		hostParts := strings.Split(u.Host, ".")
		inputBucket = hostParts[0]
		inputKey = strings.TrimPrefix(u.Path, "/")
	} else {
		inputBucket = bucketName
		inputKey = keyOrURL
	}

	getObjectInput := &s3.GetObjectInput{
		Bucket: aws.String(inputBucket),
		Key:    aws.String(inputKey),
	}

	result, err := client.GetObject(ctx, getObjectInput)
	if err != nil {
		return nil, fmt.Errorf("failed to download from S3: %w", err)
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return data, nil
}

func ListObjects(ctx context.Context, prefix string) ([]string, error) {
	client := getS3Client()

	keys := []string{}
	paginator := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects from S3: %w", err)
		}

		for _, content := range page.Contents {
			keys = append(keys, aws.ToString(content.Key))
		}
	}

	return keys, nil
}
