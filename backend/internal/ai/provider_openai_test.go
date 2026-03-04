package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOpenAIClientChatParsesChoiceMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected authorization header %q", got)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"openai ok"}}]}`))
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "test-key", 2*time.Second)
	out, err := client.Chat(context.Background(), "gpt-4o-mini", []ChatMessage{
		{Role: "user", Content: "hello"},
	})
	if err != nil {
		t.Fatalf("chat failed: %v", err)
	}
	if out != "openai ok" {
		t.Fatalf("unexpected content: %q", out)
	}
}

func TestOpenAIClientChatReturnsErrorOnHTTPFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "test-key", 2*time.Second)
	if _, err := client.Chat(context.Background(), "gpt-4o-mini", []ChatMessage{{Role: "user", Content: "hello"}}); err == nil {
		t.Fatal("expected error")
	}
}

func TestOpenAIClientChatSupportsChatCompletionsEndpointWithoutAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("expected no authorization header, got %q", got)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL+"/v1/chat/completions", "", 2*time.Second)
	out, err := client.Chat(context.Background(), "Qwen/Qwen3-14B-AWQ", []ChatMessage{
		{Role: "user", Content: "hello"},
	})
	if err != nil {
		t.Fatalf("chat failed: %v", err)
	}
	if out != "ok" {
		t.Fatalf("unexpected content: %q", out)
	}
}

func TestOpenAIClientPing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected authorization header %q", got)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "test-key", 2*time.Second)
	if err := client.Ping(context.Background()); err != nil {
		t.Fatalf("ping failed: %v", err)
	}
}

func TestOpenAIClientPingSupportsChatCompletionsEndpointWithoutAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("expected no authorization header, got %q", got)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[{"id":"Qwen/Qwen3-14B-AWQ"}]}`))
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL+"/v1/chat/completions", "", 2*time.Second)
	if err := client.Ping(context.Background()); err != nil {
		t.Fatalf("ping failed: %v", err)
	}
}
