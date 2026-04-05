package storage

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"incidenthub/backend/internal/config"
)

func TestNoopStorage(t *testing.T) {
	s := NoopStorage{}

	if err := s.Upload(context.Background(), "x", strings.NewReader("x"), 1, "text/plain"); !errors.Is(err, ErrStorageDisabled) {
		t.Fatalf("noop upload expected ErrStorageDisabled, got %v", err)
	}
	if _, err := s.PresignGet(context.Background(), "x", time.Minute); !errors.Is(err, ErrStorageDisabled) {
		t.Fatalf("noop presign expected ErrStorageDisabled, got %v", err)
	}
	if err := s.Health(context.Background()); !errors.Is(err, ErrStorageDisabled) {
		t.Fatalf("noop health expected ErrStorageDisabled, got %v", err)
	}
}

func TestS3StorageNilReceiverBranches(t *testing.T) {
	var s *S3Storage
	if err := s.Upload(context.Background(), "x", strings.NewReader("x"), 1, "text/plain"); !errors.Is(err, ErrStorageDisabled) {
		t.Fatalf("nil s3 upload expected ErrStorageDisabled, got %v", err)
	}
	if _, err := s.PresignGet(context.Background(), "x", 0); !errors.Is(err, ErrStorageDisabled) {
		t.Fatalf("nil s3 presign expected ErrStorageDisabled, got %v", err)
	}
	if err := s.Health(context.Background()); !errors.Is(err, ErrStorageDisabled) {
		t.Fatalf("nil s3 health expected ErrStorageDisabled, got %v", err)
	}
}

func TestNewS3StorageDisabledReturnsNoop(t *testing.T) {
	st, err := NewS3Storage(context.Background(), config.S3Config{Enabled: false})
	if err != nil {
		t.Fatalf("disabled s3 should not fail: %v", err)
	}
	if err := st.Health(context.Background()); !errors.Is(err, ErrStorageDisabled) {
		t.Fatalf("disabled storage health expected ErrStorageDisabled, got %v", err)
	}
}

func TestNewS3StorageValidationErrors(t *testing.T) {
	base := config.S3Config{
		Enabled:   true,
		Endpoint:  "http://localhost:9000",
		Bucket:    "bucket",
		AccessKey: "key",
		SecretKey: "secret",
	}

	{
		_, err := NewS3Storage(context.Background(), config.S3Config{Enabled: true})
		if err == nil || !strings.Contains(err.Error(), "S3_ENDPOINT is required") {
			t.Fatalf("expected missing endpoint error, got %v", err)
		}
	}
	{
		cfg := base
		cfg.Bucket = ""
		_, err := NewS3Storage(context.Background(), cfg)
		if err == nil || !strings.Contains(err.Error(), "S3_BUCKET is required") {
			t.Fatalf("expected missing bucket error, got %v", err)
		}
	}
	{
		cfg := base
		cfg.AccessKey = ""
		_, err := NewS3Storage(context.Background(), cfg)
		if err == nil || !strings.Contains(err.Error(), "S3_ACCESS_KEY and S3_SECRET_KEY are required") {
			t.Fatalf("expected missing credentials error, got %v", err)
		}
	}
	{
		cfg := base
		cfg.Endpoint = "://bad-endpoint"
		_, err := NewS3Storage(context.Background(), cfg)
		if err == nil {
			t.Fatalf("expected endpoint validation error")
		}
	}
}
