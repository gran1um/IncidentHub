package nodes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"incidenthub/backend/internal/workflow"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type s3ObjectNode struct{}

func newS3ObjectNode() workflow.NodeExecutor {
	return s3ObjectNode{}
}

func (n s3ObjectNode) Type() string {
	return "s3_object"
}

func (n s3ObjectNode) Execute(ctx context.Context, req workflow.NodeExecuteRequest) (workflow.NodeExecuteResult, error) {
	action := normalizeLabel(toString(req.Node.Config["action"]))
	if action == "" {
		action = "put"
	}

	endpointRaw := workflow.RenderTemplate(toString(req.Node.Config["endpoint"]), req.Scope)
	endpoint, secure, err := normalizeS3NodeEndpoint(endpointRaw, workflow.BoolFromAny(req.Node.Config["useSSL"], false))
	if err != nil {
		return workflow.NodeExecuteResult{}, err
	}
	bucket := strings.TrimSpace(workflow.RenderTemplate(toString(req.Node.Config["bucket"]), req.Scope))
	if bucket == "" {
		return workflow.NodeExecuteResult{}, fmt.Errorf("s3 bucket is required")
	}
	key := strings.TrimSpace(workflow.RenderTemplate(toString(req.Node.Config["key"]), req.Scope))
	if key == "" {
		return workflow.NodeExecuteResult{}, fmt.Errorf("s3 object key is required")
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds: credentials.NewStaticV4(
			strings.TrimSpace(workflow.RenderTemplate(toString(req.Node.Config["accessKeyId"]), req.Scope)),
			strings.TrimSpace(workflow.RenderTemplate(toString(req.Node.Config["secretAccessKey"]), req.Scope)),
			strings.TrimSpace(workflow.RenderTemplate(toString(req.Node.Config["sessionToken"]), req.Scope)),
		),
		Secure:       secure,
		Region:       s3NodeDefaultString(strings.TrimSpace(workflow.RenderTemplate(toString(req.Node.Config["region"]), req.Scope)), "us-east-1"),
		BucketLookup: s3NodeLookupType(workflow.BoolFromAny(req.Node.Config["pathStyle"], false)),
	})
	if err != nil {
		return workflow.NodeExecuteResult{}, fmt.Errorf("create s3 client: %w", err)
	}

	output := workflow.CopyMap(req.Payload)
	output["s3_action"] = action
	output["s3_bucket"] = bucket
	output["s3_key"] = key

	switch action {
	case "put":
		body := workflow.RenderTemplate(toString(req.Node.Config["body"]), req.Scope)
		if body == "" {
			rawPayload, _ := json.Marshal(req.Payload)
			body = string(rawPayload)
		}
		contentType := strings.TrimSpace(toString(req.Node.Config["contentType"]))
		if contentType == "" {
			contentType = "application/json"
		}
		size := int64(len(body))
		if _, err := client.PutObject(ctx, bucket, key, bytes.NewReader([]byte(body)), size, minio.PutObjectOptions{
			ContentType: contentType,
		}); err != nil {
			return workflow.NodeExecuteResult{}, fmt.Errorf("s3 put object failed: %w", err)
		}
		output["s3_written_bytes"] = size
	case "presign_get":
		ttlSeconds := toInt(req.Node.Config["ttlSeconds"], 900)
		if ttlSeconds <= 0 {
			ttlSeconds = 900
		}
		if ttlSeconds > 86400 {
			ttlSeconds = 86400
		}
		link, err := client.PresignedGetObject(ctx, bucket, key, time.Duration(ttlSeconds)*time.Second, nil)
		if err != nil {
			return workflow.NodeExecuteResult{}, fmt.Errorf("s3 presign failed: %w", err)
		}
		output["s3_url"] = link.String()
		output["s3_ttl_seconds"] = ttlSeconds
	case "get":
		maxBytes := toInt(req.Node.Config["maxBytes"], 1024*1024)
		if maxBytes <= 0 {
			maxBytes = 1024 * 1024
		}
		if maxBytes > 10*1024*1024 {
			maxBytes = 10 * 1024 * 1024
		}
		object, err := client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
		if err != nil {
			return workflow.NodeExecuteResult{}, fmt.Errorf("s3 get object failed: %w", err)
		}
		defer func() { _ = object.Close() }()

		raw, err := io.ReadAll(io.LimitReader(object, int64(maxBytes)))
		if err != nil {
			return workflow.NodeExecuteResult{}, fmt.Errorf("read s3 object failed: %w", err)
		}
		output["s3_content"] = string(raw)
		output["s3_content_bytes"] = len(raw)
		output["s3_content_truncated"] = len(raw) >= maxBytes
	default:
		return workflow.NodeExecuteResult{}, fmt.Errorf("unsupported s3 action %q", action)
	}

	return workflow.NodeExecuteResult{
		Output: output,
	}, nil
}

func normalizeS3NodeEndpoint(raw string, fallbackSecure bool) (endpoint string, endpointSecure bool, err error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", false, fmt.Errorf("s3 endpoint is required")
	}
	if strings.Contains(value, "://") {
		parsed, err := url.Parse(value)
		if err != nil {
			return "", false, fmt.Errorf("parse s3 endpoint: %w", err)
		}
		host := strings.TrimSpace(parsed.Host)
		if host == "" {
			return "", false, fmt.Errorf("s3 endpoint host cannot be empty")
		}
		secure := parsed.Scheme == "https"
		return host, secure, nil
	}
	return value, fallbackSecure, nil
}

func s3NodeLookupType(pathStyle bool) minio.BucketLookupType {
	if pathStyle {
		return minio.BucketLookupPath
	}
	return minio.BucketLookupDNS
}

func s3NodeDefaultString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
