package inbound

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"incidenthub/backend/internal/config"
)

func TestApplyHTTPAuthVariants(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.local", http.NoBody)
	applyHTTPAuth(req, inboundAuthConfig{
		Type:     "api_key",
		APIKey:   "secret",
		Headers:  map[string]string{"X-Trace": "trace-1"},
		Username: "ignored",
	})
	if req.Header.Get("X-API-Key") != "secret" {
		t.Fatalf("expected API key header, got %q", req.Header.Get("X-API-Key"))
	}
	if req.Header.Get("X-Trace") != "trace-1" {
		t.Fatalf("expected custom header, got %q", req.Header.Get("X-Trace"))
	}

	req = httptest.NewRequest(http.MethodGet, "http://example.local", http.NoBody)
	applyHTTPAuth(req, inboundAuthConfig{Type: "bearer", Token: "token-1"})
	if req.Header.Get("Authorization") != "Bearer token-1" {
		t.Fatalf("expected bearer token, got %q", req.Header.Get("Authorization"))
	}

	req = httptest.NewRequest(http.MethodGet, "http://example.local", http.NoBody)
	applyHTTPAuth(req, inboundAuthConfig{Type: "basic", Username: "u", Password: "p"})
	username, password, ok := req.BasicAuth()
	if !ok || username != "u" || password != "p" {
		t.Fatalf("unexpected basic auth credentials: %s/%s", username, password)
	}
}

func TestDurationHelpers(t *testing.T) {
	if got := firstPositiveDuration(0, -1, 3*time.Second, 2*time.Second); got != 3*time.Second {
		t.Fatalf("unexpected first positive duration: %s", got)
	}

	ctx := context.Background()
	if same, cancel := withOptionalTimeout(ctx, 0); same != ctx {
		cancel()
		t.Fatal("expected same context for zero timeout")
	}
	withTimeout, cancel := withOptionalTimeout(ctx, 10*time.Millisecond)
	defer cancel()
	if withTimeout == ctx {
		t.Fatal("expected derived timeout context")
	}
}

func TestFetchHTTPRecordsSuccessAndStatusError(t *testing.T) {
	successServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{{"id": "a-1", "title": "Alert"}},
		})
	}))
	defer successServer.Close()

	worker := &Worker{httpClient: successServer.Client(), cfg: configForHTTPTests()}
	records, err := worker.fetchHTTPRecords(context.Background(), inboundConnectorConfig{
		DisplayName: "HTTP",
		ArrayPath:   "items",
		HTTP: inboundHTTPSourceConfig{
			URL:    successServer.URL,
			Method: http.MethodGet,
		},
	})
	if err != nil {
		t.Fatalf("unexpected fetch error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	defer errorServer.Close()

	_, err = worker.fetchHTTPRecords(context.Background(), inboundConnectorConfig{
		DisplayName: "HTTP",
		HTTP: inboundHTTPSourceConfig{
			URL:    errorServer.URL,
			Method: http.MethodGet,
		},
	})
	if err == nil {
		t.Fatal("expected non-success HTTP status error")
	}
}

func configForHTTPTests() config.InboundConnectorsConfig {
	return config.InboundConnectorsConfig{HTTPTimeout: 2 * time.Second, BatchLimit: 100}
}
