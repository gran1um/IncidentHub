package outbound

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3Driver implements outbound connector support for S3-compatible object storage
// (S3, MinIO, etc.). It supports simple get/put operations.
type S3Driver struct{}

func NewS3Driver() *S3Driver {
	return &S3Driver{}
}

func (d *S3Driver) Kind() string {
	return "object_storage"
}

func (d *S3Driver) Send(ctx context.Context, cfg map[string]any, req SendRequest) (SendResponse, error) {
	endpoint := strings.TrimSpace(firstString(cfg, "endpoint", "endpoint_url", "endpointUrl", "url"))
	bucket := strings.TrimSpace(firstString(cfg, "bucket"))
	region := strings.TrimSpace(firstString(cfg, "region"))
	accessKey := strings.TrimSpace(firstString(cfg, "access_key", "accessKey", "access_key_id", "accessKeyId"))
	secretKey := strings.TrimSpace(firstString(cfg, "secret_key", "secretKey", "secret_access_key", "secretAccessKey"))

	if endpoint == "" {
		return SendResponse{}, fmt.Errorf("s3 connector endpoint is required")
	}
	if bucket == "" {
		return SendResponse{}, fmt.Errorf("s3 connector bucket is required")
	}
	if accessKey == "" || secretKey == "" {
		return SendResponse{}, fmt.Errorf("s3 connector access key and secret key are required")
	}

	host, secure, err := s3DriverParseEndpoint(endpoint)
	if err != nil {
		return SendResponse{}, err
	}

	client, err := minio.New(host, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: secure,
		Region: region,
	})
	if err != nil {
		return SendResponse{}, fmt.Errorf("s3 connector init client: %w", err)
	}

	mode := strings.ToLower(strings.TrimSpace(firstString(req.Metadata, "mode")))
	if mode == "" {
		mode = strings.ToLower(strings.TrimSpace(firstString(cfg, "mode")))
	}
	if mode == "" {
		mode = "get"
	}

	key := strings.TrimSpace(firstString(req.Metadata, "key", "objectKey", "path"))
	if key == "" {
		return SendResponse{}, fmt.Errorf("s3 connector key is required")
	}

	switch mode {
	case "get", "read", "download":
		obj, err := client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
		if err != nil {
			return SendResponse{}, fmt.Errorf("s3 connector get object: %w", err)
		}
		defer func() { _ = obj.Close() }()

		stat, err := obj.Stat()
		if err != nil {
			return SendResponse{}, fmt.Errorf("s3 connector stat object: %w", err)
		}

		// Limit body to 2 MiB for safety.
		body, err := io.ReadAll(io.LimitReader(obj, 2*1024*1024))
		if err != nil {
			return SendResponse{}, fmt.Errorf("s3 connector read object: %w", err)
		}

		metadata := map[string]any{
			"bucket":       bucket,
			"key":          key,
			"size":         stat.Size,
			"content_type": stat.ContentType,
			"etag":         stat.ETag,
		}

		return SendResponse{
			Reply:          string(body),
			ConversationID: req.ConversationID,
			Metadata:       metadata,
		}, nil

	case "put", "write", "upload":
		data := req.Message
		if data == "" {
			data = strings.TrimSpace(fmt.Sprint(req.Metadata["body"]))
		}
		contentType := strings.TrimSpace(firstString(req.Metadata, "content_type", "contentType", "mime_type"))
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		reader := strings.NewReader(data)
		size := int64(len(data))

		_, err := client.PutObject(ctx, bucket, key, reader, size, minio.PutObjectOptions{ContentType: contentType})
		if err != nil {
			return SendResponse{}, fmt.Errorf("s3 connector put object: %w", err)
		}

		metadata := map[string]any{
			"bucket":       bucket,
			"key":          key,
			"size":         size,
			"content_type": contentType,
		}
		return SendResponse{
			ConversationID: req.ConversationID,
			Metadata:       metadata,
		}, nil

	default:
		return SendResponse{}, fmt.Errorf("s3 connector unsupported mode %q (expected get/put)", mode)
	}
}

func (d *S3Driver) Poll(context.Context, map[string]any, PollRequest) (PollResponse, error) {
	// S3 connectors are request/response only for v1.5.
	return PollResponse{}, nil
}

func s3DriverParseEndpoint(raw string) (endpointHost string, isSecure bool, err error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false, fmt.Errorf("s3 connector endpoint is required")
	}

	secure := false
	host := trimmed
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		u, err := url.Parse(trimmed)
		if err != nil {
			return "", false, fmt.Errorf("s3 connector parse endpoint: %w", err)
		}
		host = strings.TrimSpace(u.Host)
		secure = u.Scheme == "https"
	}
	if host == "" {
		return "", false, fmt.Errorf("s3 connector endpoint host cannot be empty")
	}
	return host, secure, nil
}
