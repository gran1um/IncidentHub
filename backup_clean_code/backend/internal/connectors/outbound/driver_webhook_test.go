package outbound

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebhookDriverSend(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if payload["message"] != "hello" {
			t.Fatalf("expected message hello, got %v", payload["message"])
		}
		_, _ = w.Write([]byte(`{"reply":"ok","conversation_id":"conv-1","cursor":"42"}`))
	}))
	defer server.Close()

	driver := NewWebhookDriver(server.Client())
	resp, err := driver.Send(context.Background(), map[string]any{"url": server.URL}, SendRequest{Message: "hello"})
	if err != nil {
		t.Fatalf("send returned error: %v", err)
	}
	if resp.Reply != "ok" {
		t.Fatalf("expected reply ok, got %q", resp.Reply)
	}
	if resp.ConversationID != "conv-1" {
		t.Fatalf("expected conversation id conv-1, got %q", resp.ConversationID)
	}
}

func TestWebhookDriverPoll(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		_, _ = w.Write([]byte(`{"messages":[{"id":"m1","author":"bot","content":"pong"}],"next_cursor":"2"}`))
	}))
	defer server.Close()

	driver := NewWebhookDriver(server.Client())
	resp, err := driver.Poll(context.Background(), map[string]any{"poll_url": server.URL}, PollRequest{})
	if err != nil {
		t.Fatalf("poll returned error: %v", err)
	}
	if len(resp.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(resp.Messages))
	}
	if resp.Messages[0].Content != "pong" {
		t.Fatalf("expected message content pong, got %q", resp.Messages[0].Content)
	}
}
