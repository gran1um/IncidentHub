package outbound

import (
	"context"
	"fmt"
	"time"
)

type MockDriver struct{}

func NewMockDriver() *MockDriver {
	return &MockDriver{}
}

func (d *MockDriver) Kind() string {
	return "mock"
}

func (d *MockDriver) Send(_ context.Context, _ map[string]any, req SendRequest) (SendResponse, error) {
	return SendResponse{
		Reply:          fmt.Sprintf("Mock reply: received '%s'", req.Message),
		ConversationID: req.ConversationID,
		Metadata: map[string]any{
			"provider": "mock",
		},
	}, nil
}

func (d *MockDriver) Poll(_ context.Context, _ map[string]any, _ PollRequest) (PollResponse, error) {
	return PollResponse{
		Messages: []PollMessage{
			{
				ExternalID: fmt.Sprintf("mock-%d", time.Now().Unix()),
				Author:     "mock-bot",
				Content:    "Mock sync event",
				Timestamp:  time.Now().UTC().Format(time.RFC3339),
			},
		},
		Metadata: map[string]any{
			"provider": "mock",
		},
	}, nil
}
