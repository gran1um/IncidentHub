package outbound

import (
	"context"
	"testing"
)

func TestMockDriverSendAndPoll(t *testing.T) {
	driver := NewMockDriver()

	sendResp, err := driver.Send(context.Background(), map[string]any{}, SendRequest{Message: "ping", ConversationID: "conv-1"})
	if err != nil {
		t.Fatalf("send error: %v", err)
	}
	if sendResp.ConversationID != "conv-1" {
		t.Fatalf("unexpected conversation id: %q", sendResp.ConversationID)
	}
	if sendResp.Reply == "" {
		t.Fatal("expected non-empty reply")
	}

	pollResp, err := driver.Poll(context.Background(), map[string]any{}, PollRequest{})
	if err != nil {
		t.Fatalf("poll error: %v", err)
	}
	if len(pollResp.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(pollResp.Messages))
	}
	if pollResp.Messages[0].Content == "" {
		t.Fatal("expected non-empty poll message content")
	}
}
