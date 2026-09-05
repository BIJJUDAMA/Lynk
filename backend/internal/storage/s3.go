package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
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
}

type S3Client struct {
	client         *s3.Client
	presignClient  *s3.PresignClient
	bucket         string
	publicEndpoint string
}

var _ Client = (*S3Client)(nil)

func GenerateResumeKey(userID string, originalFilename string) string {
	base := filepath.Base(originalFilename)
	sanitized := strings.ReplaceAll(base, " ", "_")
	return fmt.Sprintf("resumes/%s/%s-%s", userID, uuid.NewString(), sanitized)
}

func NewS3Client(ctx context.Context, cfg Config) (*S3Client, error) {
	endpoint := cfg.Endpoint
	if endpoint != "" && !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		scheme := "http"
		if cfg.UseSSL {
			scheme = "https"
		}
		endpoint = fmt.Sprintf("%s://%s", scheme, endpoint)
	}

	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL:               endpoint,
			HostnameImmutable: true,
			SigningRegion:     "us-east-1",
		}, nil
	})

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithEndpointResolverWithOptions(customResolver),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")),
		awsconfig.WithRegion("us-east-1"),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	presignClient := s3.NewPresignClient(client)

	return &S3Client{
		client:         client,
		presignClient:  presignClient,
		bucket:         cfg.Bucket,
		publicEndpoint: cfg.PublicEndpoint,
	}, nil
}

func (s *S3Client) rewritePresignedURL(rawURL string) string {
	if s.publicEndpoint == "" {
		return rawURL
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	pub, err := url.Parse(s.publicEndpoint)
	if err != nil || pub.Host == "" || pub.Scheme == "" {
		return rawURL
	}
	parsed.Scheme = pub.Scheme
	parsed.Host = pub.Host
	return parsed.String()
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
	return s.rewritePresignedURL(req.URL), nil
}
