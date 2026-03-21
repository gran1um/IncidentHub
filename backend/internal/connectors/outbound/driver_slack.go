package outbound

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type SlackDriver struct {
	client *http.Client
}

func NewSlackDriver(client *http.Client) *SlackDriver {
	return &SlackDriver{client: client}
}

func (d *SlackDriver) Kind() string {
	return "slack"
}

func (d *SlackDriver) Send(ctx context.Context, cfg map[string]any, req SendRequest) (SendResponse, error) {
	token := strings.TrimSpace(firstString(cfg, "botToken", "token", "accessToken"))
	channelID := slackResolveChannelID(cfg, req.Metadata, req.ConversationID)
	if token == "" || channelID == "" {
		return SendResponse{}, fmt.Errorf("slack connector requires botToken and channel_id")
	}

	baseURL := strings.TrimSpace(firstString(cfg, "apiBaseURL", "api_base_url", "baseUrl"))
	if baseURL == "" {
		baseURL = "https://slack.com/api"
	}
	threadTS := slackResolveThreadTS(req.Metadata)
	payload := map[string]any{
		"channel": channelID,
		"text":    req.Message,
	}
	if threadTS != "" {
		payload["thread_ts"] = threadTS
	}
	body, _ := json.Marshal(payload)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/chat.postMessage", bytes.NewReader(body))
	if err != nil {
		return SendResponse{}, fmt.Errorf("build slack send request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)

	resp, err := d.client.Do(httpReq)
	if err != nil {
		return SendResponse{}, fmt.Errorf("execute slack send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return SendResponse{}, fmt.Errorf("read slack send response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return SendResponse{}, fmt.Errorf("slack send failed with status %s", resp.Status)
	}

	parsed := map[string]any{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return SendResponse{}, fmt.Errorf("decode slack send response: %w", err)
	}
	if ok, exists := parsed["ok"].(bool); exists && !ok {
		return SendResponse{}, fmt.Errorf("slack send failed: %s", strings.TrimSpace(firstString(parsed, "error")))
	}

	messagePayload, _ := parsed["message"].(map[string]any)
	responseThreadTS := strings.TrimSpace(firstString(messagePayload, "thread_ts"))
	if responseThreadTS == "" {
		responseThreadTS = strings.TrimSpace(firstString(parsed, "ts"))
	}
	conversationID := channelID
	if responseThreadTS != "" {
		conversationID = slackConversationID(channelID, responseThreadTS)
	}
	metadata := map[string]any{
		"provider":   "slack",
		"channel_id": channelID,
		"thread_ts":  responseThreadTS,
		"ts":         strings.TrimSpace(firstString(parsed, "ts")),
	}
	if responseThreadTS == "" {
		delete(metadata, "thread_ts")
	}
	for key, value := range parsed {
		metadata[key] = value
	}
	return SendResponse{
		Reply:          defaultString(firstString(messagePayload, "text"), "Slack message sent"),
		ConversationID: conversationID,
		Cursor:         strings.TrimSpace(firstString(parsed, "ts")),
		Metadata:       metadata,
	}, nil
}

func (d *SlackDriver) Poll(ctx context.Context, cfg map[string]any, req PollRequest) (PollResponse, error) {
	token := strings.TrimSpace(firstString(cfg, "botToken", "token", "accessToken"))
	if token == "" {
		return PollResponse{}, fmt.Errorf("slack connector requires botToken")
	}
	baseURL := strings.TrimSpace(firstString(cfg, "apiBaseURL", "api_base_url", "baseUrl"))
	if baseURL == "" {
		baseURL = "https://slack.com/api"
	}
	channelID, threadTS := slackSplitConversationID(req.ConversationID)
	if channelID == "" {
		channelID = slackResolveChannelID(cfg, req.Metadata, req.ConversationID)
	}
	if threadTS == "" {
		threadTS = slackResolveThreadTS(req.Metadata)
	}
	if channelID == "" {
		return PollResponse{}, fmt.Errorf("slack poll requires channel_id")
	}

	endpoint := strings.TrimRight(baseURL, "/") + "/conversations.history"
	params := url.Values{}
	params.Set("channel", channelID)
	params.Set("limit", "100")
	if req.Cursor != "" {
		params.Set("oldest", req.Cursor)
	}
	if threadTS != "" {
		endpoint = strings.TrimRight(baseURL, "/") + "/conversations.replies"
		params.Set("ts", threadTS)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+params.Encode(), http.NoBody)
	if err != nil {
		return PollResponse{}, fmt.Errorf("build slack poll request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)

	resp, err := d.client.Do(httpReq)
	if err != nil {
		return PollResponse{}, fmt.Errorf("execute slack poll request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return PollResponse{}, fmt.Errorf("slack poll failed with status %s", resp.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return PollResponse{}, fmt.Errorf("read slack poll response: %w", err)
	}
	parsed := map[string]any{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return PollResponse{}, fmt.Errorf("decode slack poll response: %w", err)
	}
	if ok, exists := parsed["ok"].(bool); exists && !ok {
		return PollResponse{}, fmt.Errorf("slack poll failed: %s", strings.TrimSpace(firstString(parsed, "error")))
	}

	rawMessages, _ := parsed["messages"].([]any)
	messages := make([]PollMessage, 0, len(rawMessages))
	nextCursor := req.Cursor
	for _, rawMessage := range rawMessages {
		item, ok := rawMessage.(map[string]any)
		if !ok {
			continue
		}
		ts := strings.TrimSpace(firstString(item, "ts"))
		if req.Cursor != "" && ts != "" && slackTimestampLTE(ts, req.Cursor) {
			continue
		}
		author := strings.TrimSpace(firstString(item, "username", "user", "bot_id"))
		if author == "" {
			author = "slack-user"
		}
		metadata := map[string]any{
			"provider":   "slack",
			"channel_id": channelID,
			"thread_ts":  firstString(item, "thread_ts"),
			"ts":         ts,
		}
		if threadTS == "" {
			delete(metadata, "thread_ts")
		}
		messages = append(messages, PollMessage{
			ExternalID: ts,
			Author:     author,
			Content:    defaultString(firstString(item, "text"), ""),
			Timestamp:  slackTimestampToRFC3339(ts),
			Metadata:   metadata,
		})
		if ts != "" && (nextCursor == "" || !slackTimestampLTE(ts, nextCursor)) {
			nextCursor = ts
		}
	}
	metadata := map[string]any{
		"provider":   "slack",
		"channel_id": channelID,
	}
	if threadTS != "" {
		metadata["thread_ts"] = threadTS
	}
	for key, value := range parsed {
		metadata[key] = value
	}
	return PollResponse{
		Messages:       messages,
		ConversationID: slackConversationID(channelID, threadTS),
		NextCursor:     nextCursor,
		Metadata:       metadata,
	}, nil
}

func slackTimestampToRFC3339(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	parts := strings.SplitN(trimmed, ".", 2)
	seconds, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return trimmed
	}
	nanos := int64(0)
	if len(parts) == 2 {
		fraction := parts[1]
		if len(fraction) > 9 {
			fraction = fraction[:9]
		}
		for len(fraction) < 9 {
			fraction += "0"
		}
		parsedNanos, nanosErr := strconv.ParseInt(fraction, 10, 64)
		if nanosErr == nil {
			nanos = parsedNanos
		}
	}
	return time.Unix(seconds, nanos).UTC().Format(time.RFC3339Nano)
}

func slackResolveChannelID(cfg map[string]any, metadata map[string]any, conversationID string) string {
	channelID, _ := slackSplitConversationID(conversationID)
	if channelID != "" {
		return channelID
	}
	channelID = strings.TrimSpace(firstString(metadata,
		"channel_id",
		"channelId",
		"recipient_channel_id",
		"recipientChannelId",
		"chat_id",
		"chatId",
		"recipient",
		"target",
	))
	if channelID == "" {
		participant, _ := metadata["participant"].(map[string]any)
		channelID = strings.TrimSpace(firstString(participant,
			"channel_id",
			"channelId",
			"chat_id",
			"chatId",
			"recipient",
			"target",
		))
	}
	if channelID == "" {
		channelID = strings.TrimSpace(firstString(cfg, "channelId", "channel_id"))
	}
	return channelID
}

func slackResolveThreadTS(metadata map[string]any) string {
	threadTS := strings.TrimSpace(firstString(metadata, "thread_ts", "threadTs"))
	if threadTS == "" {
		participant, _ := metadata["participant"].(map[string]any)
		threadTS = strings.TrimSpace(firstString(participant, "thread_ts", "threadTs"))
	}
	return threadTS
}

func slackConversationID(channelID, threadTS string) string {
	channelID = strings.TrimSpace(channelID)
	threadTS = strings.TrimSpace(threadTS)
	if channelID == "" {
		return ""
	}
	if threadTS == "" {
		return channelID
	}
	return channelID + "|" + threadTS
}

func slackSplitConversationID(conversationID string) (channelID string, threadTS string) {
	trimmed := strings.TrimSpace(conversationID)
	if trimmed == "" {
		return "", ""
	}
	parts := strings.SplitN(trimmed, "|", 2)
	if len(parts) == 1 {
		return strings.TrimSpace(parts[0]), ""
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
}

func slackTimestampLTE(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || right == "" {
		return false
	}
	return left <= right
}
