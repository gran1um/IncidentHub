package outbound

import (
	"context"
	"strings"
	"testing"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisDriverSendRequiresKey(t *testing.T) {
	driver := NewRedisDriver()
	_, err := driver.Send(context.Background(), map[string]any{
		"addr": "127.0.0.1:6379",
	}, SendRequest{Message: "hello"})
	if err == nil {
		t.Fatal("expected redis send key validation error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "key") {
		t.Fatalf("expected key validation error, got: %v", err)
	}
}

func TestRedisDriverSendAndPoll(t *testing.T) {
	server, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer server.Close()

	seed := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer func() { _ = seed.Close() }()

	driver := NewRedisDriver()
	cfg := map[string]any{
		"addr":     server.Addr(),
		"send_key": "incidenthub.outbound.redis",
		"poll_key": "incidenthub.inbound.redis",
	}

	sendResp, err := driver.Send(context.Background(), cfg, SendRequest{
		ThreadID:       "thread-1",
		ConversationID: "conv-1",
		Author:         "soc",
		Message:        "hello from incidenthub",
	})
	if err != nil {
		t.Fatalf("send via redis driver: %v", err)
	}
	if sendResp.ConversationID != "conv-1" {
		t.Fatalf("unexpected send conversation id: %q", sendResp.ConversationID)
	}

	written, err := seed.LLen(context.Background(), "incidenthub.outbound.redis").Result()
	if err != nil {
		t.Fatalf("read outbound redis queue length: %v", err)
	}
	if written != 1 {
		t.Fatalf("expected one queued outbound payload, got %d", written)
	}

	if pushErr := seed.RPush(context.Background(), "incidenthub.inbound.redis",
		`{"id":"msg-1","author":"external-user","message":"pong","timestamp":"2026-02-19T10:00:00Z"}`,
	).Err(); pushErr != nil {
		t.Fatalf("seed inbound redis payload: %v", pushErr)
	}

	pollResp, err := driver.Poll(context.Background(), cfg, PollRequest{
		ConversationID: "conv-1",
	})
	if err != nil {
		t.Fatalf("poll via redis driver: %v", err)
	}
	if len(pollResp.Messages) != 1 {
		t.Fatalf("expected one polled message, got %d", len(pollResp.Messages))
	}
	message := pollResp.Messages[0]
	if message.ExternalID != "msg-1" {
		t.Fatalf("unexpected external id: %q", message.ExternalID)
	}
	if message.Author != "external-user" {
		t.Fatalf("unexpected author: %q", message.Author)
	}
	if message.Content != "pong" {
		t.Fatalf("unexpected content: %q", message.Content)
	}

	remaining, err := seed.LLen(context.Background(), "incidenthub.inbound.redis").Result()
	if err != nil {
		t.Fatalf("read inbound redis queue length: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("expected inbound queue to be consumed, got %d", remaining)
	}
}

func TestRedisDriverPollRequiresKey(t *testing.T) {
	driver := NewRedisDriver()
	_, err := driver.Poll(context.Background(), map[string]any{
		"addr": "127.0.0.1:6379",
	}, PollRequest{})
	if err == nil {
		t.Fatal("expected redis poll key validation error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "key") {
		t.Fatalf("expected key validation error, got: %v", err)
	}
}
