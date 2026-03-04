package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOllamaClientChatParsesMessageContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"model":"llama3.2:3b","message":{"role":"assistant","content":"ok"}}`))
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL, 2*time.Second)
	out, err := client.Chat(context.Background(), "llama3.2:3b", []ChatMessage{
		{Role: "user", Content: "hello"},
	})
	if err != nil {
		t.Fatalf("chat failed: %v", err)
	}
	if out != "ok" {
		t.Fatalf("unexpected content: %q", out)
	}
}

func TestOllamaClientChatUsesLegacyResponseField(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"response":"legacy answer"}`))
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL, 2*time.Second)
	out, err := client.Chat(context.Background(), "llama3.2:3b", []ChatMessage{
		{Role: "user", Content: "hello"},
	})
	if err != nil {
		t.Fatalf("chat failed: %v", err)
	}
	if out != "legacy answer" {
		t.Fatalf("unexpected content: %q", out)
	}
}

func TestOllamaClientChatReturnsErrorOnHTTPFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL, 2*time.Second)
	_, err := client.Chat(context.Background(), "llama3.2:3b", []ChatMessage{
		{Role: "user", Content: "hello"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestOllamaClientPing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL, 2*time.Second)
	if err := client.Ping(context.Background()); err != nil {
		t.Fatalf("ping failed: %v", err)
	}
}
