package storage

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

type Config struct {
	Endpoint       string
	PublicEndpoint string
	AccessKey      string
	SecretKey      string
	Bucket         string
	UseSSL         bool
}

type Client interface {
	UploadResume(ctx context.Context, key string, contentType string, body io.Reader) error
	GetPresignedDownloadURL(ctx context.Context, key string, expiry time.Duration) (string, error)
	DeleteResume(ctx context.Context, key string) error
}

type S3Client struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	bucket        string
}

var _ Client = (*S3Client)(nil)

func GenerateResumeKey(userID string, originalFilename string) string {
	base := filepath.Base(originalFilename)
	sanitized := strings.ReplaceAll(base, " ", "_")
	return fmt.Sprintf("resumes/%s/%s-%s", userID, uuid.NewString(), sanitized)
}

func newAWSConfig(ctx context.Context, endpoint string, cfg Config) (aws.Config, error) {
	if endpoint != "" && !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		scheme := "http"
		if cfg.UseSSL {
			scheme = "https"
		}
		endpoint = fmt.Sprintf("%s://%s", scheme, endpoint)
	}
	// MinIO requires a custom path-style endpoint; the AWS SDK v2 global endpoint
	// resolver APIs are deprecated but remain the supported way to target MinIO
	// until BaseEndpoint / service options fully replace this path.
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) { //nolint:staticcheck // SA1019: MinIO custom endpoint
		return aws.Endpoint{ //nolint:staticcheck // SA1019: MinIO custom endpoint
			URL:               endpoint,
			HostnameImmutable: true,
			SigningRegion:     "us-east-1",
		}, nil
	})
	return awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithEndpointResolverWithOptions(customResolver), //nolint:staticcheck // SA1019: MinIO custom endpoint
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")),
		awsconfig.WithRegion("us-east-1"),
	)
}

func NewS3Client(ctx context.Context, cfg Config) (*S3Client, error) {
	internalCfg, err := newAWSConfig(ctx, cfg.Endpoint, cfg)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	internalClient := s3.NewFromConfig(internalCfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	presignEndpoint := cfg.PublicEndpoint
	if strings.TrimSpace(presignEndpoint) == "" {
		presignEndpoint = cfg.Endpoint
	}
	presignAWS, err := newAWSConfig(ctx, presignEndpoint, cfg)
	if err != nil {
		return nil, fmt.Errorf("load public aws config: %w", err)
	}
	publicClient := s3.NewFromConfig(presignAWS, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	return &S3Client{
		client:        internalClient,
		presignClient: s3.NewPresignClient(publicClient),
		bucket:        cfg.Bucket,
	}, nil
}

func (s *S3Client) UploadResume(ctx context.Context, key string, contentType string, body io.Reader) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("put s3 object: %w", err)
	}
	return nil
}

func (s *S3Client) GetPresignedDownloadURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	req, err := s.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("presign get object: %w", err)
	}
	return req.URL, nil
}

func (s *S3Client) DeleteResume(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete s3 object %q: %w", key, err)
	}
	return nil
}

// CheckBucket verifies if the configured bucket exists without mutating or creating it.
func (s *S3Client) CheckBucket(ctx context.Context) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("s3 client is not initialized")
	}
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(s.bucket),
	})
	return err
}

// EnsureBucket verifies if the configured bucket exists, and creates it if it does not.
func (s *S3Client) EnsureBucket(ctx context.Context) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("s3 client is not initialized")
	}
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(s.bucket),
	})
	if err == nil {
		return nil
	}

	_, err = s.client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(s.bucket),
	})
	if err != nil {
		return fmt.Errorf("auto-create s3 bucket %q: %w", s.bucket, err)
	}
	return nil
}

