package inbound

import (
	"context"
	"testing"

	"github.com/minio/minio-go/v7"
)

func TestNormalizeS3Endpoint(t *testing.T) {
	if _, _, err := normalizeS3Endpoint("", true); err == nil {
		t.Fatal("expected endpoint required error")
	}

	host, secure, err := normalizeS3Endpoint("https://minio.local:9000", false)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if host != "minio.local:9000" || !secure {
		t.Fatalf("unexpected normalized endpoint: host=%q secure=%t", host, secure)
	}

	host, secure, err = normalizeS3Endpoint("minio.local:9000", true)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if host != "minio.local:9000" || !secure {
		t.Fatalf("unexpected normalized endpoint for raw host: host=%q secure=%t", host, secure)
	}
}

func TestS3PatternAndLookupHelpers(t *testing.T) {
	if !matchesObjectPattern("feed/*.json", "feed/alert.json") {
		t.Fatal("expected glob pattern to match")
	}
	if matchesObjectPattern("feed/*.json", "feed/alert.txt") {
		t.Fatal("expected glob pattern mismatch")
	}
	if !matchesObjectPattern("feed/*indicator*", "feed/new-indicator-event") {
		t.Fatal("expected wildcard pattern to match")
	}

	if got := s3LookupType(true); got != minio.BucketLookupPath {
		t.Fatalf("expected path lookup mode, got %v", got)
	}
	if got := s3LookupType(false); got == minio.BucketLookupPath {
		t.Fatalf("expected non-path lookup mode, got %v", got)
	}
}

func TestFetchS3RecordsInvalidEndpoint(t *testing.T) {
	worker := &Worker{}
	_, err := worker.fetchS3Records(context.Background(), inboundConnectorConfig{
		DisplayName: "S3",
		S3: inboundS3SourceConfig{
			Endpoint: "http://",
			Bucket:   "alerts",
			Region:   "us-east-1",
		},
	})
	if err == nil {
		t.Fatal("expected invalid endpoint error")
	}
}
