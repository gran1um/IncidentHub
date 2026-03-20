package outbound

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEmailDriverSend(t *testing.T) {
	var capturedTo []any
	var capturedCC []any
	var capturedSubject string
	var capturedMessage string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		capturedTo, _ = payload["to"].([]any)
		capturedCC, _ = payload["cc"].([]any)
		capturedSubject = strings.TrimSpace(firstString(payload, "subject"))
		capturedMessage = strings.TrimSpace(firstString(payload, "message"))
		_, _ = w.Write([]byte(`{"reply":"email accepted","conversation_id":"mail-conv-1","cursor":"mail-cursor-1"}`))
	}))
	defer server.Close()

	driver := NewEmailDriver(server.Client())
	resp, err := driver.Send(context.Background(), map[string]any{"send_url": server.URL}, SendRequest{
		Message: "email payload",
		Metadata: map[string]any{
			"to":      "user@example.com",
			"cc":      []string{"manager@example.com"},
			"subject": "Incident update",
		},
	})
	if err != nil {
		t.Fatalf("send returned error: %v", err)
	}
	if resp.Reply != "email accepted" {
		t.Fatalf("expected reply email accepted, got %q", resp.Reply)
	}
	if resp.ConversationID != "mail-conv-1" {
		t.Fatalf("expected conversation id mail-conv-1, got %q", resp.ConversationID)
	}
	if len(capturedTo) != 1 || strings.TrimSpace(firstString(map[string]any{"value": capturedTo[0]}, "value")) != "user@example.com" {
		t.Fatalf("unexpected to recipients: %#v", capturedTo)
	}
	if len(capturedCC) != 1 || strings.TrimSpace(firstString(map[string]any{"value": capturedCC[0]}, "value")) != "manager@example.com" {
		t.Fatalf("unexpected cc recipients: %#v", capturedCC)
	}
	if capturedSubject != "Incident update" {
		t.Fatalf("unexpected subject: %q", capturedSubject)
	}
	if capturedMessage != "email payload" {
		t.Fatalf("unexpected message: %q", capturedMessage)
	}
}

func TestEmailDriverPollWithSubject(t *testing.T) {
	capturedSubject := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		capturedSubject = strings.TrimSpace(r.URL.Query().Get("subject"))
		_, _ = w.Write([]byte(`{
			"messages":[
				{"id":"mail-1","author":"user@example.com","subject":"Incident update","content":"Thanks, got it","timestamp":"2026-02-27T10:00:00Z"}
			],
			"next_cursor":"mail-cursor-2"
		}`))
	}))
	defer server.Close()

	driver := NewEmailDriver(server.Client())
	resp, err := driver.Poll(context.Background(), map[string]any{"poll_url": server.URL}, PollRequest{
		ConversationID: "mail-conv-1",
		Metadata: map[string]any{
			"subject": "Incident update",
		},
	})
	if err != nil {
		t.Fatalf("poll returned error: %v", err)
	}
	if capturedSubject != "Incident update" {
		t.Fatalf("expected subject query Incident update, got %q", capturedSubject)
	}
	if len(resp.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(resp.Messages))
	}
	if resp.Messages[0].Content != "Thanks, got it" {
		t.Fatalf("unexpected message content: %q", resp.Messages[0].Content)
	}
	if got := strings.TrimSpace(firstString(resp.Messages[0].Metadata, "subject")); got != "Incident update" {
		t.Fatalf("expected message subject Incident update, got %q", got)
	}
}
