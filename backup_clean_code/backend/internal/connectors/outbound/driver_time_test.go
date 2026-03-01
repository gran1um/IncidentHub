package outbound

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTimeDriverSend(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if payload["message"] != "hello time" {
			t.Fatalf("expected message hello time, got %v", payload["message"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"reply":           "sent",
			"conversation_id": "time-conv-1",
			"cursor":          "time-1",
		})
	}))
	defer server.Close()

	driver := NewTimeDriver(server.Client())
	resp, err := driver.Send(context.Background(), map[string]any{"send_url": server.URL}, SendRequest{
		ConversationID: "recipient-1",
		Message:        "hello time",
	})
	if err != nil {
		t.Fatalf("send returned error: %v", err)
	}
	if resp.Reply != "sent" {
		t.Fatalf("expected reply sent, got %q", resp.Reply)
	}
	if resp.ConversationID != "time-conv-1" {
		t.Fatalf("expected conversation id time-conv-1, got %q", resp.ConversationID)
	}
}

func TestTimeDriverPoll(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"messages": []map[string]any{
				{
					"id":      "time-msg-1",
					"author":  "time-bot",
					"content": "ping",
				},
			},
			"next_cursor": "time-2",
		})
	}))
	defer server.Close()

	driver := NewTimeDriver(server.Client())
	resp, err := driver.Poll(context.Background(), map[string]any{"poll_url": server.URL}, PollRequest{})
	if err != nil {
		t.Fatalf("poll returned error: %v", err)
	}
	if len(resp.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(resp.Messages))
	}
	if resp.Messages[0].Content != "ping" {
		t.Fatalf("expected message content ping, got %q", resp.Messages[0].Content)
	}
}
