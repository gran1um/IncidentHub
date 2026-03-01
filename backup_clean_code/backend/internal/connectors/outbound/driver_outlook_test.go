package outbound

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOutlookDriverSend(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tenant-1/oauth2/v2.0/token":
			_, _ = w.Write([]byte(`{"access_token":"token-123"}`))
		case "/v1.0/users/security@example.com/sendMail":
			if got := r.Header.Get("Authorization"); got != "Bearer token-123" {
				t.Fatalf("unexpected auth header: %s", got)
			}
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			message, _ := payload["message"].(map[string]any)
			if message["subject"] != "Incident update" {
				t.Fatalf("unexpected subject: %#v", message["subject"])
			}
			w.WriteHeader(http.StatusAccepted)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	driver := NewOutlookDriver(server.Client())
	resp, err := driver.Send(context.Background(), map[string]any{
		"tenantId":     "tenant-1",
		"clientId":     "client-1",
		"clientSecret": "secret-1",
		"mailbox":      "security@example.com",
		"authBaseURL":  server.URL,
		"apiBaseURL":   server.URL + "/v1.0",
	}, SendRequest{
		Message: "hello",
		Metadata: map[string]any{
			"to":      "user@example.com",
			"subject": "Incident update",
		},
	})
	if err != nil {
		t.Fatalf("send returned error: %v", err)
	}
	if resp.ConversationID != "Incident update" {
		t.Fatalf("unexpected conversation id: %q", resp.ConversationID)
	}
}

func TestOutlookDriverPoll(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tenant-1/oauth2/v2.0/token":
			_, _ = w.Write([]byte(`{"access_token":"token-123"}`))
		case "/v1.0/users/security@example.com/messages":
			payload := `{
				"value": [
					{
						"id": "msg-1",
						"subject": "Incident update",
						"conversationId": "conv-1",
						"receivedDateTime": "2026-03-08T10:00:00Z",
						"bodyPreview": "reply",
						"from": {
							"emailAddress": {
								"name": "Analyst",
								"address": "analyst@example.com"
							}
						}
					}
				]
			}`
			_, _ = w.Write([]byte(payload))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	driver := NewOutlookDriver(server.Client())
	resp, err := driver.Poll(context.Background(), map[string]any{
		"tenantId":     "tenant-1",
		"clientId":     "client-1",
		"clientSecret": "secret-1",
		"mailbox":      "security@example.com",
		"authBaseURL":  server.URL,
		"apiBaseURL":   server.URL + "/v1.0",
	}, PollRequest{Metadata: map[string]any{"subject": "Incident update"}})
	if err != nil {
		t.Fatalf("poll returned error: %v", err)
	}
	if len(resp.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(resp.Messages))
	}
	if resp.Messages[0].Author != "Analyst" {
		t.Fatalf("unexpected author: %q", resp.Messages[0].Author)
	}
}
