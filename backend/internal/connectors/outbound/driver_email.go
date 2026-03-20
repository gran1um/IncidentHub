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

type EmailDriver struct {
	client *http.Client
}

func NewEmailDriver(client *http.Client) *EmailDriver {
	return &EmailDriver{client: client}
}

func (d *EmailDriver) Kind() string {
	return "email"
}

func (d *EmailDriver) Send(ctx context.Context, cfg map[string]any, req SendRequest) (SendResponse, error) {
	endpoint := strings.TrimSpace(firstString(cfg, "send_url", "sendUrl", "url", "endpoint"))
	if endpoint == "" {
		return SendResponse{}, fmt.Errorf("email connector send_url is required")
	}

	to := normalizeEmailRecipients(
		req.Metadata["to"],
		req.Metadata["recipient"],
		req.Metadata["target"],
		req.Metadata["email"],
		firstString(cfg, "to", "recipient", "email"),
		extractNestedField(req.Metadata, "participant", "email"),
		extractNestedField(req.Metadata, "participant", "target"),
	)
	if len(to) == 0 {
		return SendResponse{}, fmt.Errorf("email connector requires at least one recipient")
	}
	cc := normalizeEmailRecipients(req.Metadata["cc"], req.Metadata["manager_emails"], extractNestedField(req.Metadata, "participant", "manager_emails"))
	bcc := normalizeEmailRecipients(req.Metadata["bcc"])
	subject := strings.TrimSpace(firstString(req.Metadata, "subject", "email_subject", "topic", "thread_subject"))
	if subject == "" {
		subject = strings.TrimSpace(firstString(cfg, "subject", "default_subject"))
	}

	payload := map[string]any{
		"thread_id":       req.ThreadID,
		"conversation_id": req.ConversationID,
		"author":          req.Author,
		"from":            firstString(cfg, "from", "from_email", "sender"),
		"to":              to,
		"cc":              cc,
		"bcc":             bcc,
		"subject":         subject,
		"message":         req.Message,
		"metadata":        req.Metadata,
	}

	body, _ := json.Marshal(payload)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return SendResponse{}, fmt.Errorf("build email send request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	for key, value := range parseHeaders(cfg) {
		httpReq.Header.Set(key, value)
	}

	resp, err := d.client.Do(httpReq)
	if err != nil {
		return SendResponse{}, fmt.Errorf("send email request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return SendResponse{}, fmt.Errorf("read email response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return SendResponse{}, fmt.Errorf("email send failed with status %s", resp.Status)
	}

	if len(raw) == 0 {
		return SendResponse{
			Reply:          fmt.Sprintf("Email queued to %d recipient(s)", len(to)),
			ConversationID: req.ConversationID,
			Metadata: map[string]any{
				"provider": "email",
				"to":       to,
				"cc":       cc,
				"subject":  subject,
			},
		}, nil
	}

	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err == nil {
		reply := strings.TrimSpace(firstString(parsed, "reply", "message", "text"))
		if reply == "" {
			reply = fmt.Sprintf("Email queued to %d recipient(s)", len(to))
		}
		metadata := parsed
		metadata["provider"] = "email"
		metadata["to"] = to
		metadata["cc"] = cc
		if subject != "" {
			metadata["subject"] = subject
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
			"provider": "email",
			"to":       to,
			"cc":       cc,
			"subject":  subject,
		},
	}, nil
}

func (d *EmailDriver) Poll(ctx context.Context, cfg map[string]any, req PollRequest) (PollResponse, error) {
	pollURL := strings.TrimSpace(firstString(cfg, "poll_url", "pollUrl"))
	if pollURL == "" {
		return PollResponse{}, nil
	}
	parsedURL, err := url.Parse(pollURL)
	if err != nil {
		return PollResponse{}, fmt.Errorf("parse email poll url: %w", err)
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
	subject := strings.TrimSpace(firstString(req.Metadata, "subject", "email_subject", "topic"))
	if subject != "" {
		query.Set("subject", subject)
	}
	parsedURL.RawQuery = query.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), http.NoBody)
	if err != nil {
		return PollResponse{}, fmt.Errorf("build email poll request: %w", err)
	}
	for key, value := range parseHeaders(cfg) {
		httpReq.Header.Set(key, value)
	}

	resp, err := d.client.Do(httpReq)
	if err != nil {
		return PollResponse{}, fmt.Errorf("execute email poll request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return PollResponse{}, fmt.Errorf("email poll failed with status %s", resp.Status)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return PollResponse{}, fmt.Errorf("read email poll response: %w", err)
	}
	if len(raw) == 0 {
		return PollResponse{}, nil
	}

	var parsedPayload map[string]any
	if err := json.Unmarshal(raw, &parsedPayload); err != nil {
		return PollResponse{}, fmt.Errorf("decode email poll response: %w", err)
	}

	messages := make([]PollMessage, 0)
	rawMessages, _ := parsedPayload["messages"].([]any)
	for _, rawMessage := range rawMessages {
		item, ok := rawMessage.(map[string]any)
		if !ok {
			continue
		}
		messageMetadata := map[string]any{}
		if nested, ok := item["metadata"].(map[string]any); ok {
			for key, value := range nested {
				messageMetadata[key] = value
			}
		}
		messageMetadata["provider"] = "email"
		messageSubject := strings.TrimSpace(firstString(item, "subject", "topic"))
		if messageSubject != "" {
			messageMetadata["subject"] = messageSubject
		}

		messages = append(messages, PollMessage{
			ExternalID: firstString(item, "id", "external_id", "externalId"),
			Author:     defaultString(firstString(item, "author", "from", "sender"), "email-user"),
			Content:    defaultString(firstString(item, "content", "message", "text", "body"), ""),
			Timestamp:  firstString(item, "timestamp", "created_at", "createdAt"),
			Metadata:   messageMetadata,
		})
	}

	metadata := map[string]any{
		"provider": "email",
	}
	for key, value := range parsedPayload {
		metadata[key] = value
	}
	if subject != "" {
		metadata["subject"] = subject
	}

	return PollResponse{
		Messages:       messages,
		ConversationID: defaultString(firstString(parsedPayload, "conversation_id", "conversationId"), req.ConversationID),
		NextCursor:     defaultString(firstString(parsedPayload, "next_cursor", "nextCursor", "cursor"), req.Cursor),
		Metadata:       metadata,
	}, nil
}

func normalizeEmailRecipients(values ...any) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	appendOne := func(raw string) {
		candidate := strings.TrimSpace(raw)
		if candidate == "" {
			return
		}
		candidate = strings.Trim(candidate, ",;")
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			return
		}
		lower := strings.ToLower(candidate)
		if _, exists := seen[lower]; exists {
			return
		}
		seen[lower] = struct{}{}
		out = append(out, candidate)
	}
	for _, value := range values {
		switch typed := value.(type) {
		case nil:
			continue
		case string:
			for _, token := range strings.FieldsFunc(typed, func(r rune) bool {
				return r == ',' || r == ';'
			}) {
				appendOne(token)
			}
		case []string:
			for _, token := range typed {
				appendOne(token)
			}
		case []any:
			for _, token := range typed {
				appendOne(fmt.Sprint(token))
			}
		default:
			appendOne(fmt.Sprint(typed))
		}
	}
	return out
}

func extractNestedField(payload map[string]any, key string, field string) any {
	if payload == nil {
		return nil
	}
	raw, ok := payload[key]
	if !ok {
		return nil
	}
	nested, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	return nested[field]
}
