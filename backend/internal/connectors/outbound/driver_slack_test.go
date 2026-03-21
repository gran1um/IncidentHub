package outbound

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSlackDriverSend(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat.postMessage" {
			t.Fatalf("expected chat.postMessage, got %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("unexpected auth header: %s", got)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if payload["channel"] != "C123" {
			t.Fatalf("expected channel C123, got %#v", payload["channel"])
		}
		_, _ = w.Write([]byte(`{"ok":true,"channel":"C123","ts":"1710000000.100","message":{"text":"hello","thread_ts":"1710000000.100"}}`))
	}))
	defer server.Close()

	driver := NewSlackDriver(server.Client())
	resp, err := driver.Send(context.Background(), map[string]any{
		"botToken":   "test-token",
		"apiBaseURL": server.URL,
	}, SendRequest{
		Message:  "hello",
		Metadata: map[string]any{"channel_id": "C123"},
	})
	if err != nil {
		t.Fatalf("send returned error: %v", err)
	}
	if resp.ConversationID != "C123|1710000000.100" {
		t.Fatalf("unexpected conversation id: %q", resp.ConversationID)
	}
}

func TestSlackDriverPoll(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/conversations.replies" {
			t.Fatalf("expected conversations.replies, got %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("channel"); got != "C123" {
			t.Fatalf("expected channel query C123, got %q", got)
		}
		if got := r.URL.Query().Get("ts"); got != "1710000000.100" {
			t.Fatalf("expected thread ts query, got %q", got)
		}
		_, _ = w.Write([]byte(`{"ok":true,"messages":[{"ts":"1710000001.000","thread_ts":"1710000000.100","text":"reply","user":"U123"}]}`))
	}))
	defer server.Close()

	driver := NewSlackDriver(server.Client())
	resp, err := driver.Poll(context.Background(), map[string]any{
		"botToken":   "test-token",
		"apiBaseURL": server.URL,
	}, PollRequest{ConversationID: "C123|1710000000.100"})
	if err != nil {
		t.Fatalf("poll returned error: %v", err)
	}
	if len(resp.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(resp.Messages))
	}
	if resp.Messages[0].Content != "reply" {
		t.Fatalf("unexpected content: %q", resp.Messages[0].Content)
	}
	if resp.Messages[0].Timestamp != "2024-03-09T16:00:01Z" {
		t.Fatalf("unexpected normalized timestamp: %q", resp.Messages[0].Timestamp)
	}
}
