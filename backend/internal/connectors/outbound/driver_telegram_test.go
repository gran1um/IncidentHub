package outbound

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTelegramDriverSendUsesMetadataChatID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if payload["chat_id"] != "123456" {
			t.Fatalf("expected chat_id=123456, got %v", payload["chat_id"])
		}
		_, _ = w.Write([]byte(`{"ok":true,"description":"sent"}`))
	}))
	defer server.Close()

	driver := NewTelegramDriver(server.Client())
	resp, err := driver.Send(context.Background(), map[string]any{
		"botToken":   "token",
		"apiBaseURL": server.URL,
	}, SendRequest{
		Message:  "hello",
		Metadata: map[string]any{"chat_id": "123456"},
	})
	if err != nil {
		t.Fatalf("send returned error: %v", err)
	}
	if resp.ConversationID != "123456" {
		t.Fatalf("expected conversation id 123456, got %q", resp.ConversationID)
	}
}

func TestTelegramDriverSendUsesParticipantChatID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if payload["chat_id"] != "participant-chat" {
			t.Fatalf("expected participant chat id, got %v", payload["chat_id"])
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	driver := NewTelegramDriver(server.Client())
	_, err := driver.Send(context.Background(), map[string]any{
		"botToken":   "token",
		"apiBaseURL": server.URL,
	}, SendRequest{
		Message: "hello",
		Metadata: map[string]any{
			"participant": map[string]any{
				"chat_id": "participant-chat",
			},
		},
	})
	if err != nil {
		t.Fatalf("send returned error: %v", err)
	}
}

func TestTelegramDriverPollUsesConversationID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		_, _ = w.Write([]byte(`{
			"ok": true,
			"result": [
				{
					"update_id": 10,
					"message": {
						"message_id": 20,
						"date": 1700000000,
						"text": "pong",
						"from": {"username": "user-a"},
						"chat": {"id": 777}
					}
				}
			]
		}`))
	}))
	defer server.Close()

	driver := NewTelegramDriver(server.Client())
	resp, err := driver.Poll(context.Background(), map[string]any{
		"botToken":   "token",
		"apiBaseURL": server.URL,
	}, PollRequest{
		ConversationID: "777",
	})
	if err != nil {
		t.Fatalf("poll returned error: %v", err)
	}
	if len(resp.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(resp.Messages))
	}
	if resp.Messages[0].Content != "pong" {
		t.Fatalf("expected pong, got %q", resp.Messages[0].Content)
	}
	if got := strings.TrimSpace(firstString(resp.Messages[0].Metadata, "chat_id")); got != "777" {
		t.Fatalf("expected metadata chat_id=777, got %q", got)
	}
	if got := strings.TrimSpace(firstString(resp.Messages[0].Metadata, "username")); got != "user-a" {
		t.Fatalf("expected metadata username=user-a, got %q", got)
	}
}

func TestTelegramDriverSendRequiresAPIBaseURL(t *testing.T) {
	driver := NewTelegramDriver(http.DefaultClient)
	_, err := driver.Send(context.Background(), map[string]any{
		"botToken": "token",
		"chatId":   "123",
	}, SendRequest{
		Message: "hello",
	})
	if err == nil {
		t.Fatal("expected error for missing apiBaseURL")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "apibaseurl") {
		t.Fatalf("expected apiBaseURL validation error, got %v", err)
	}
}

func TestTelegramDriverSendReturnsRecipientLinkHint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"ok":false,"description":"Bad Request: chat not found"}`))
	}))
	defer server.Close()

	driver := NewTelegramDriver(server.Client())
	_, err := driver.Send(context.Background(), map[string]any{
		"botToken":   "token",
		"apiBaseURL": server.URL,
	}, SendRequest{
		Message: "hello",
		Metadata: map[string]any{
			"participant": map[string]any{
				"target": "@unknown-user",
			},
		},
	})
	if err == nil {
		t.Fatal("expected chat-link hint error for unknown username")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "not linked to this bot yet") {
		t.Fatalf("expected recipient link hint, got %v", err)
	}
}
