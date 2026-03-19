package inbound

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func (w *Worker) fetchS3Records(ctx context.Context, cfg inboundConnectorConfig) ([]map[string]any, error) {
	s3Cfg := cfg.S3
	endpoint, secure, err := normalizeS3Endpoint(s3Cfg.Endpoint, s3Cfg.UseSSL)
	if err != nil {
		return nil, fmt.Errorf("inbound connector %q: invalid S3 endpoint: %w", cfg.DisplayName, err)
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(s3Cfg.Auth.AccessKeyID, s3Cfg.Auth.SecretAccessKey, s3Cfg.Auth.SessionToken),
		Secure:       secure,
		Region:       s3Cfg.Region,
		BucketLookup: s3LookupType(s3Cfg.PathStyle),
	})
	if err != nil {
		return nil, fmt.Errorf("create S3 client for inbound connector %q: %w", cfg.DisplayName, err)
	}

	reqCtx, cancel := withOptionalTimeout(ctx, firstPositiveDuration(s3Cfg.Timeout, cfg.Timeout, 20*time.Second))
	defer cancel()

	maxObjects := s3Cfg.MaxObjects
	if maxObjects <= 0 {
		maxObjects = 100
	}
	listOpts := minio.ListObjectsOptions{
		Prefix:    s3Cfg.Prefix,
		Recursive: true,
	}

	records := make([]map[string]any, 0)
	processed := 0
	for objectInfo := range client.ListObjects(reqCtx, s3Cfg.Bucket, listOpts) {
		if objectInfo.Err != nil {
			return nil, fmt.Errorf("list S3 objects for inbound connector %q: %w", cfg.DisplayName, objectInfo.Err)
		}
		if strings.TrimSpace(s3Cfg.ObjectPattern) != "" && !matchesObjectPattern(s3Cfg.ObjectPattern, objectInfo.Key) {
			continue
		}

		payloadRecords, readErr := readS3ObjectRecords(reqCtx, client, s3Cfg, cfg.ArrayPath, objectInfo.Key)
		if readErr != nil {
			return nil, fmt.Errorf("read S3 object %q for inbound connector %q: %w", objectInfo.Key, cfg.DisplayName, readErr)
		}
		records = append(records, payloadRecords...)
		processed++
		if processed >= maxObjects {
			break
		}
	}

	return records, nil
}

func readS3ObjectRecords(ctx context.Context, client *minio.Client, cfg inboundS3SourceConfig, arrayPath, objectKey string) ([]map[string]any, error) {
	obj, err := client.GetObject(ctx, cfg.Bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("get object: %w", err)
	}
	defer func() { _ = obj.Close() }()

	limit := cfg.ObjectSizeLimitBytes
	if limit <= 0 {
		limit = 5 * 1024 * 1024
	}
	body, err := io.ReadAll(io.LimitReader(obj, limit))
	if err != nil {
		return nil, fmt.Errorf("read object body: %w", err)
	}
	if len(body) == 0 {
		return []map[string]any{}, nil
	}

	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		record := map[string]any{
			"id":          objectKey,
			"title":       objectKey,
			"description": string(body),
			"source":      "s3",
			"s3_key":      objectKey,
		}
		return []map[string]any{record}, nil
	}

	records := extractRecords(payload, arrayPath)
	for _, record := range records {
		if _, exists := record["s3_key"]; !exists {
			record["s3_key"] = objectKey
		}
		if _, exists := record["id"]; !exists {
			record["id"] = objectKey
		}
	}
	return records, nil
}

func normalizeS3Endpoint(raw string, fallbackSecure bool) (endpoint string, secure bool, err error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false, fmt.Errorf("endpoint is required")
	}
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		parsed, err := url.Parse(trimmed)
		if err != nil {
			return "", false, err
		}
		if parsed.Host == "" {
			return "", false, fmt.Errorf("host is empty")
		}
		return parsed.Host, parsed.Scheme == "https", nil
	}
	return trimmed, fallbackSecure, nil
}

func s3LookupType(pathStyle bool) minio.BucketLookupType {
	if pathStyle {
		return minio.BucketLookupPath
	}
	return minio.BucketLookupAuto
}

func matchesObjectPattern(pattern, key string) bool {
	trimmed := strings.TrimSpace(pattern)
	if trimmed == "" {
		return true
	}
	if ok, err := path.Match(trimmed, key); err == nil {
		return ok
	}
	if strings.HasPrefix(trimmed, "*.") {
		suffix := strings.TrimPrefix(trimmed, "*")
		return strings.HasSuffix(strings.ToLower(key), strings.ToLower(suffix))
	}
	return strings.Contains(strings.ToLower(key), strings.ToLower(trimmed))
}
