package storage

import (
	"context"
	"fmt"
	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/tracing"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type S3Storage struct {
	client        *minio.Client
	presignClient *minio.Client
	bucket        string
}

func NewS3Storage(ctx context.Context, cfg config.S3Config) (store ArtifactStorage, err error) {
	ctx, span, startedAt := tracing.StartModuleOperation(ctx, "storage", "new_s3_storage")
	defer func() {
		tracing.FinishModuleOperation(span, startedAt, "storage", "new_s3_storage", err)
	}()

	if !cfg.Enabled {
		return NoopStorage{}, nil
	}

	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("S3_ENDPOINT is required when S3 is enabled")
	}
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, fmt.Errorf("S3_BUCKET is required when S3 is enabled")
	}
	if strings.TrimSpace(cfg.AccessKey) == "" || strings.TrimSpace(cfg.SecretKey) == "" {
		return nil, fmt.Errorf("S3_ACCESS_KEY and S3_SECRET_KEY are required when S3 is enabled")
	}

	endpoint, secure, err := parseS3Endpoint(endpoint, cfg.UseSSL)
	if err != nil {
		return nil, err
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: secure,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("init s3 client: %w", err)
	}

	if cfg.AutoCreateBucket {
		exists, err := client.BucketExists(ctx, cfg.Bucket)
		if err != nil {
			return nil, fmt.Errorf("check s3 bucket: %w", err)
		}
		if !exists {
			if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{Region: cfg.Region}); err != nil {
				return nil, fmt.Errorf("create s3 bucket: %w", err)
			}
		}
	}

	presignClient := client
	publicEndpoint := strings.TrimSpace(cfg.PublicEndpoint)
	if publicEndpoint != "" {
		publicHost, publicSecure, parseErr := parseS3Endpoint(publicEndpoint, secure)
		if parseErr != nil {
			return nil, fmt.Errorf("parse s3 public endpoint: %w", parseErr)
		}
		publicClient, newErr := minio.New(publicHost, &minio.Options{
			Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
			Secure: publicSecure,
			Region: cfg.Region,
		})
		if newErr != nil {
			return nil, fmt.Errorf("init s3 public client: %w", newErr)
		}
		presignClient = publicClient
	}

	return &S3Storage{client: client, presignClient: presignClient, bucket: cfg.Bucket}, nil
}

func (s *S3Storage) Upload(ctx context.Context, key string, body io.Reader, size int64, contentType string) (err error) {
	ctx, span, startedAt := tracing.StartModuleOperation(ctx, "storage", "upload")
	defer func() {
		tracing.FinishModuleOperation(span, startedAt, "storage", "upload", err)
	}()

	if s == nil || s.client == nil {
		return ErrStorageDisabled
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	_, err = s.client.PutObject(ctx, s.bucket, key, body, size, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return fmt.Errorf("upload artifact to s3: %w", err)
	}
	return nil
}

func (s *S3Storage) PresignGet(ctx context.Context, key string, ttl time.Duration) (presignedURL string, err error) {
	ctx, span, startedAt := tracing.StartModuleOperation(ctx, "storage", "presign_get")
	defer func() {
		tracing.FinishModuleOperation(span, startedAt, "storage", "presign_get", err)
	}()

	if s == nil || s.client == nil {
		return "", ErrStorageDisabled
	}
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}

	presignClient := s.presignClient
	if presignClient == nil {
		presignClient = s.client
	}
	u, err := presignClient.PresignedGetObject(ctx, s.bucket, key, ttl, nil)
	if err != nil {
		return "", fmt.Errorf("presign s3 object: %w", err)
	}
	return u.String(), nil
}

func parseS3Endpoint(raw string, fallbackSecure bool) (host string, secure bool, err error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false, fmt.Errorf("S3 endpoint is required")
	}

	secure = fallbackSecure
	host = trimmed
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		u, parseErr := url.Parse(trimmed)
		if parseErr != nil {
			return "", false, fmt.Errorf("parse s3 endpoint: %w", parseErr)
		}
		host = strings.TrimSpace(u.Host)
		secure = u.Scheme == "https"
	}
	if host == "" {
		return "", false, fmt.Errorf("S3 endpoint host cannot be empty")
	}
	return host, secure, nil
}

func (s *S3Storage) Health(ctx context.Context) (err error) {
	ctx, span, startedAt := tracing.StartModuleOperation(ctx, "storage", "health")
	defer func() {
		tracing.FinishModuleOperation(span, startedAt, "storage", "health", err)
	}()

	if s == nil || s.client == nil {
		return ErrStorageDisabled
	}
	if _, err = s.client.BucketExists(ctx, s.bucket); err != nil {
		return fmt.Errorf("s3 bucket check: %w", err)
	}
	return nil
}
