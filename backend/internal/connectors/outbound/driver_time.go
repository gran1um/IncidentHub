package outbound

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type TimeDriver struct {
	client *http.Client
}

func NewTimeDriver(client *http.Client) *TimeDriver {
	return &TimeDriver{client: client}
}

func (d *TimeDriver) Kind() string {
	return "time"
}

func (d *TimeDriver) Send(ctx context.Context, cfg map[string]any, req SendRequest) (SendResponse, error) {
	endpoint := strings.TrimSpace(firstString(cfg, "send_url", "sendUrl", "url", "endpoint"))
	if endpoint == "" {
		return SendResponse{}, fmt.Errorf("time connector send_url is required")
	}

	recipient := strings.TrimSpace(req.ConversationID)
	if recipient == "" {
		recipient = strings.TrimSpace(firstString(req.Metadata, "recipient", "target", "chat_id"))
	}

	payload := map[string]any{
		"thread_id":       req.ThreadID,
		"conversation_id": req.ConversationID,
		"recipient":       recipient,
		"author":          req.Author,
		"message":         req.Message,
		"metadata":        req.Metadata,
	}
	body, _ := json.Marshal(payload)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return SendResponse{}, fmt.Errorf("build time send request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	for key, value := range parseHeaders(cfg) {
		httpReq.Header.Set(key, value)
	}

	resp, err := d.client.Do(httpReq)
	if err != nil {
		return SendResponse{}, fmt.Errorf("send time request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return SendResponse{}, fmt.Errorf("read time response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return SendResponse{}, fmt.Errorf("time send failed with status %s", resp.Status)
	}

	if len(raw) == 0 {
		return SendResponse{
			Reply:          "Time message sent",
			ConversationID: req.ConversationID,
			Metadata: map[string]any{
				"provider":  "time",
				"recipient": recipient,
			},
		}, nil
	}

	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err == nil {
		reply := strings.TrimSpace(firstString(parsed, "reply", "message", "text"))
		if reply == "" {
			reply = "Time message sent"
		}
		metadata := map[string]any{"provider": "time"}
		for key, value := range parsed {
			metadata[key] = value
		}
		if recipient != "" {
			metadata["recipient"] = recipient
		}
		return SendResponse{
			Reply:          reply,
			ConversationID: defaultString(firstString(parsed, "conversation_id", "conversationId"), req.ConversationID),
			Cursor:         firstString(parsed, "cursor", "next_cursor", "nextCursor"),
			Metadata:       metadata,
		}, nil
	}

	return SendResponse{
		Reply:          strings.TrimSpace(string(raw)),
		ConversationID: req.ConversationID,
		Metadata: map[string]any{
			"provider":  "time",
			"recipient": recipient,
		},
	}, nil
}

func (d *TimeDriver) Poll(ctx context.Context, cfg map[string]any, req PollRequest) (PollResponse, error) {
	pollURL := strings.TrimSpace(firstString(cfg, "poll_url", "pollUrl"))
	if pollURL == "" {
		return PollResponse{}, nil
	}
	parsedURL, err := url.Parse(pollURL)
	if err != nil {
		return PollResponse{}, fmt.Errorf("parse time poll url: %w", err)
	}
	query := parsedURL.Query()
	if req.ConversationID != "" {
		query.Set("conversation_id", req.ConversationID)
	}
	if req.Cursor != "" {
		query.Set("cursor", req.Cursor)
	}
	if req.ThreadID != "" {
		query.Set("thread_id", req.ThreadID)
	}
	parsedURL.RawQuery = query.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), http.NoBody)
	if err != nil {
		return PollResponse{}, fmt.Errorf("build time poll request: %w", err)
	}
	for key, value := range parseHeaders(cfg) {
		httpReq.Header.Set(key, value)
	}

	resp, err := d.client.Do(httpReq)
	if err != nil {
		return PollResponse{}, fmt.Errorf("execute time poll request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return PollResponse{}, fmt.Errorf("time poll failed with status %s", resp.Status)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return PollResponse{}, fmt.Errorf("read time poll response: %w", err)
	}
	if len(raw) == 0 {
		return PollResponse{}, nil
	}

	var parsedPayload map[string]any
	if err := json.Unmarshal(raw, &parsedPayload); err != nil {
		return PollResponse{}, fmt.Errorf("decode time poll response: %w", err)
	}

	messages := make([]PollMessage, 0)
	rawMessages, _ := parsedPayload["messages"].([]any)
	for _, rawMessage := range rawMessages {
		item, ok := rawMessage.(map[string]any)
		if !ok {
			continue
		}
		messages = append(messages, PollMessage{
			ExternalID: firstString(item, "id", "external_id", "externalId"),
			Author:     defaultString(firstString(item, "author", "from", "sender"), "time-user"),
			Content:    defaultString(firstString(item, "content", "message", "text"), ""),
			Timestamp:  firstString(item, "timestamp", "created_at", "createdAt"),
			Metadata: map[string]any{
				"provider": "time",
			},
		})
	}

	metadata := map[string]any{"provider": "time"}
	for key, value := range parsedPayload {
		metadata[key] = value
	}

	return PollResponse{
		Messages:       messages,
		ConversationID: defaultString(firstString(parsedPayload, "conversation_id", "conversationId"), req.ConversationID),
		NextCursor:     defaultString(firstString(parsedPayload, "next_cursor", "nextCursor", "cursor"), req.Cursor),
		Metadata:       metadata,
	}, nil
}
