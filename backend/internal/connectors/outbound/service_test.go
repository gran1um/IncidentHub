package outbound

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/models"
)

func TestServiceSendRequiresChannel(t *testing.T) {
	svc := NewService(config.OutboundConnectorsConfig{})

	_, err := svc.Send(context.Background(), models.CatalogItem{
		Data: map[string]any{
			"config": map[string]any{"url": "https://example.local"},
		},
	}, SendRequest{Message: "hello"})
	if err == nil {
		t.Fatal("expected error for missing connector channel")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "channel") {
		t.Fatalf("expected channel validation error, got %v", err)
	}
}

func TestServicePollWebhookPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		payload := map[string]any{
			"messages": []map[string]any{
				{"id": "m-1", "author": "bot", "content": "pong", "timestamp": "2024-01-01T00:00:00Z"},
			},
			"next_cursor": "2",
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	svc := NewService(config.OutboundConnectorsConfig{})
	resp, err := svc.Poll(context.Background(), models.CatalogItem{
		Data: map[string]any{
			"channel": "webhook",
			"config": map[string]any{
				"poll_url": server.URL,
			},
		},
	}, PollRequest{})
	if err != nil {
		t.Fatalf("poll error: %v", err)
	}
	if len(resp.Messages) != 1 {
		t.Fatalf("expected one message, got %d", len(resp.Messages))
	}
	if resp.NextCursor != "2" {
		t.Fatalf("unexpected next cursor: %q", resp.NextCursor)
	}
}

func TestServiceUnsupportedDriver(t *testing.T) {
	svc := NewService(config.OutboundConnectorsConfig{})
	_, err := svc.Send(context.Background(), models.CatalogItem{
		Data: map[string]any{
			"channel": "unsupported",
			"config":  map[string]any{},
		},
	}, SendRequest{Message: "hello"})
	if err == nil {
		t.Fatal("expected unsupported driver error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "not supported") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServiceRedisDriverRegistered(t *testing.T) {
	svc := NewService(config.OutboundConnectorsConfig{})
	_, err := svc.Send(context.Background(), models.CatalogItem{
		Data: map[string]any{
			"channel": "redis",
			"config": map[string]any{
				"addr": "127.0.0.1:6379",
			},
		},
	}, SendRequest{Message: "hello"})
	if err == nil {
		t.Fatal("expected redis key validation error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "key") {
		t.Fatalf("expected redis driver error, got %v", err)
	}
}
