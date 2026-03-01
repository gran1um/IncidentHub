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

type WebhookDriver struct {
	client *http.Client
}

func NewWebhookDriver(client *http.Client) *WebhookDriver {
	return &WebhookDriver{client: client}
}

func (d *WebhookDriver) Kind() string {
	return "webhook"
}

func (d *WebhookDriver) Send(ctx context.Context, cfg map[string]any, req SendRequest) (SendResponse, error) {
	endpoint := strings.TrimSpace(firstString(cfg, "url", "endpoint", "send_url", "sendUrl", "baseUrl"))
	if endpoint == "" {
		return SendResponse{}, fmt.Errorf("webhook connector url is required")
	}

	payload := map[string]any{
		"thread_id":       req.ThreadID,
		"conversation_id": req.ConversationID,
		"author":          req.Author,
		"message":         req.Message,
		"metadata":        req.Metadata,
	}
	body, _ := json.Marshal(payload)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return SendResponse{}, fmt.Errorf("build webhook send request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	for key, value := range parseHeaders(cfg) {
		httpReq.Header.Set(key, value)
	}

	resp, err := d.client.Do(httpReq)
	if err != nil {
		return SendResponse{}, fmt.Errorf("send webhook request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return SendResponse{}, fmt.Errorf("read webhook response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return SendResponse{}, fmt.Errorf("webhook send failed with status %s", resp.Status)
	}

	if len(raw) == 0 {
		return SendResponse{}, nil
	}

	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err == nil {
		return SendResponse{
			Reply:          firstString(parsed, "reply", "message", "text"),
			ConversationID: firstString(parsed, "conversation_id", "conversationId"),
			Cursor:         firstString(parsed, "cursor", "next_cursor", "nextCursor"),
			Metadata:       parsed,
		}, nil
	}

	return SendResponse{Reply: strings.TrimSpace(string(raw))}, nil
}

func (d *WebhookDriver) Poll(ctx context.Context, cfg map[string]any, req PollRequest) (PollResponse, error) {
	pollURL := strings.TrimSpace(firstString(cfg, "poll_url", "pollUrl"))
	if pollURL == "" {
		return PollResponse{}, nil
	}
	parsed, err := url.Parse(pollURL)
	if err != nil {
		return PollResponse{}, fmt.Errorf("parse webhook poll url: %w", err)
	}
	query := parsed.Query()
	if req.ConversationID != "" {
		query.Set("conversation_id", req.ConversationID)
	}
	if req.Cursor != "" {
		query.Set("cursor", req.Cursor)
	}
	if req.ThreadID != "" {
		query.Set("thread_id", req.ThreadID)
	}
	if subject := strings.TrimSpace(firstString(req.Metadata, "subject", "email_subject", "topic")); subject != "" {
		query.Set("subject", subject)
	}
	parsed.RawQuery = query.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), http.NoBody)
	if err != nil {
		return PollResponse{}, fmt.Errorf("build webhook poll request: %w", err)
	}
	for key, value := range parseHeaders(cfg) {
		httpReq.Header.Set(key, value)
	}

	resp, err := d.client.Do(httpReq)
	if err != nil {
		return PollResponse{}, fmt.Errorf("execute webhook poll request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return PollResponse{}, fmt.Errorf("webhook poll failed with status %s", resp.Status)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return PollResponse{}, fmt.Errorf("read webhook poll response: %w", err)
	}
	if len(raw) == 0 {
		return PollResponse{}, nil
	}

	var parsedPayload map[string]any
	if err := json.Unmarshal(raw, &parsedPayload); err != nil {
		return PollResponse{}, fmt.Errorf("decode webhook poll response: %w", err)
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
			Author:     defaultString(firstString(item, "author", "from"), "external"),
			Content:    defaultString(firstString(item, "content", "message", "text"), ""),
			Timestamp:  firstString(item, "timestamp", "created_at", "createdAt"),
		})
	}

	return PollResponse{
		Messages:       messages,
		ConversationID: defaultString(firstString(parsedPayload, "conversation_id", "conversationId"), req.ConversationID),
		NextCursor:     defaultString(firstString(parsedPayload, "next_cursor", "nextCursor", "cursor"), req.Cursor),
		Metadata:       parsedPayload,
	}, nil
}

func parseHeaders(cfg map[string]any) map[string]string {
	out := map[string]string{}
	value, ok := cfg["headers"]
	if !ok {
		value = cfg["defaultHeaders"]
	}
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			if key == "" {
				continue
			}
			out[key] = fmt.Sprint(item)
		}
	case string:
		parsed := map[string]string{}
		if json.Unmarshal([]byte(typed), &parsed) == nil {
			for key, item := range parsed {
				out[key] = item
			}
		}
	}
	return out
}

func defaultString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
