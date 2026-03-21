package outbound

import (
	"context"
	"strings"
	"testing"

	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/models"
)

func TestS3Driver_ParseEndpoint(t *testing.T) {
	host, secure, err := s3DriverParseEndpoint("https://minio.local:9000")
	if err != nil {
		t.Fatalf("parse endpoint failed: %v", err)
	}
	if host != "minio.local:9000" {
		t.Fatalf("unexpected host: %q", host)
	}
	if !secure {
		t.Fatalf("expected secure endpoint for https")
	}

	if _, _, err := s3DriverParseEndpoint(""); err == nil {
		t.Fatalf("expected error for empty endpoint")
	}
}

func TestS3Driver_SendValidationErrors(t *testing.T) {
	d := NewS3Driver()

	// Missing endpoint.
	_, err := d.Send(context.Background(), map[string]any{
		"bucket": "bucket",
	}, SendRequest{})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "endpoint") {
		t.Fatalf("expected endpoint validation error, got %v", err)
	}

	// Missing bucket.
	_, err = d.Send(context.Background(), map[string]any{
		"endpoint": "http://localhost:9000",
	}, SendRequest{})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "bucket") {
		t.Fatalf("expected bucket validation error, got %v", err)
	}

	// Missing credentials.
	_, err = d.Send(context.Background(), map[string]any{
		"endpoint": "http://localhost:9000",
		"bucket":   "bucket",
	}, SendRequest{})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "access key") {
		t.Fatalf("expected credentials validation error, got %v", err)
	}
}

// Sanity check that the driver is wired through Service.resolveDriver.
func TestS3Driver_ServiceResolution(t *testing.T) {
	svc := NewService(config.OutboundConnectorsConfig{})
	connector := models.CatalogItem{
		Data: map[string]any{
			"type": "S3",
			"config": map[string]any{
				"endpoint":        "http://localhost:9000",
				"bucket":          "bucket",
				"accessKeyId":     "key",
				"secretAccessKey": "secret",
			},
		},
	}

	_, _, err := svc.resolveDriver(connector)
	if err != nil {
		t.Fatalf("expected s3 connector to resolve, got %v", err)
	}
}
