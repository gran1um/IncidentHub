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

type OutlookDriver struct {
	client *http.Client
}

func NewOutlookDriver(client *http.Client) *OutlookDriver {
	return &OutlookDriver{client: client}
}

func (d *OutlookDriver) Kind() string {
	return "outlook"
}

func (d *OutlookDriver) Send(ctx context.Context, cfg map[string]any, req SendRequest) (SendResponse, error) {
	accessToken, err := d.fetchAccessToken(ctx, cfg)
	if err != nil {
		return SendResponse{}, err
	}
	mailbox := strings.TrimSpace(firstString(cfg, "mailbox", "userId", "user_id", "sender", "from"))
	if mailbox == "" {
		return SendResponse{}, fmt.Errorf("outlook connector requires mailbox")
	}
	apiBaseURL := strings.TrimSpace(firstString(cfg, "apiBaseURL", "api_base_url", "baseUrl"))
	if apiBaseURL == "" {
		apiBaseURL = "https://graph.microsoft.com/v1.0"
	}

	toRecipients := outlookRecipients(normalizeEmailRecipients(req.Metadata["to"], req.Metadata["recipient"], req.Metadata["target"]))
	ccRecipients := outlookRecipients(normalizeEmailRecipients(req.Metadata["cc"]))
	subject := strings.TrimSpace(firstString(req.Metadata, "subject", "email_subject", "topic"))
	messagePayload := map[string]any{
		"subject": subject,
		"body": map[string]any{
			"contentType": "Text",
			"content":     req.Message,
		},
		"toRecipients": toRecipients,
	}
	if len(ccRecipients) > 0 {
		messagePayload["ccRecipients"] = ccRecipients
	}
	payload := map[string]any{
		"message":         messagePayload,
		"saveToSentItems": true,
	}
	rawPayload, _ := json.Marshal(payload)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(apiBaseURL, "/")+"/users/"+url.PathEscape(mailbox)+"/sendMail", bytes.NewReader(rawPayload))
	if err != nil {
		return SendResponse{}, fmt.Errorf("build outlook send request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := d.client.Do(httpReq)
	if err != nil {
		return SendResponse{}, fmt.Errorf("execute outlook send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
		return SendResponse{}, fmt.Errorf("outlook send failed with status %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}

	metadata := map[string]any{
		"provider": "outlook",
		"mailbox":  mailbox,
		"subject":  subject,
		"to":       normalizeEmailRecipients(req.Metadata["to"], req.Metadata["recipient"], req.Metadata["target"]),
		"cc":       normalizeEmailRecipients(req.Metadata["cc"]),
	}
	conversationID := strings.TrimSpace(req.ConversationID)
	if conversationID == "" {
		conversationID = subject
	}
	return SendResponse{
		Reply:          defaultString(firstString(req.Metadata, "send_reply"), fmt.Sprintf("Outlook email queued to %d recipient(s)", len(toRecipients))),
		ConversationID: conversationID,
		Metadata:       metadata,
	}, nil
}

func (d *OutlookDriver) Poll(ctx context.Context, cfg map[string]any, req PollRequest) (PollResponse, error) {
	accessToken, err := d.fetchAccessToken(ctx, cfg)
	if err != nil {
		return PollResponse{}, err
	}
	mailbox := strings.TrimSpace(firstString(cfg, "mailbox", "userId", "user_id", "sender", "from"))
	if mailbox == "" {
		return PollResponse{}, fmt.Errorf("outlook connector requires mailbox")
	}
	apiBaseURL := strings.TrimSpace(firstString(cfg, "apiBaseURL", "api_base_url", "baseUrl"))
	if apiBaseURL == "" {
		apiBaseURL = "https://graph.microsoft.com/v1.0"
	}
	subject := strings.TrimSpace(firstString(req.Metadata, "subject", "email_subject", "topic"))
	conversationID := strings.TrimSpace(req.ConversationID)

	messagesURL := strings.TrimRight(apiBaseURL, "/") + "/users/" + url.PathEscape(mailbox) + "/messages?$top=50"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, messagesURL, http.NoBody)
	if err != nil {
		return PollResponse{}, fmt.Errorf("build outlook poll request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := d.client.Do(httpReq)
	if err != nil {
		return PollResponse{}, fmt.Errorf("execute outlook poll request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
		return PollResponse{}, fmt.Errorf("outlook poll failed with status %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return PollResponse{}, fmt.Errorf("read outlook poll response: %w", err)
	}
	parsed := map[string]any{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return PollResponse{}, fmt.Errorf("decode outlook poll response: %w", err)
	}
	rawItems, _ := parsed["value"].([]any)
	messages := make([]PollMessage, 0, len(rawItems))
	nextCursor := req.Cursor
	for _, rawItem := range rawItems {
		item, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		itemSubject := strings.TrimSpace(firstString(item, "subject"))
		itemConversationID := strings.TrimSpace(firstString(item, "conversationId", "conversation_id"))
		if subject != "" && itemSubject != subject {
			continue
		}
		if conversationID != "" && conversationID != subject && itemConversationID != "" && itemConversationID != conversationID {
			continue
		}
		timestamp := strings.TrimSpace(firstString(item, "receivedDateTime", "createdDateTime", "sentDateTime"))
		if req.Cursor != "" && timestamp != "" && timestamp <= req.Cursor {
			continue
		}
		fromPayload, _ := item["from"].(map[string]any)
		emailAddress, _ := fromPayload["emailAddress"].(map[string]any)
		author := defaultString(firstString(emailAddress, "name", "address"), "outlook-user")
		content := strings.TrimSpace(firstString(item, "bodyPreview"))
		if content == "" {
			bodyPayload, _ := item["body"].(map[string]any)
			content = strings.TrimSpace(firstString(bodyPayload, "content"))
		}
		metadata := map[string]any{
			"provider":         "outlook",
			"subject":          itemSubject,
			"conversation_id":  itemConversationID,
			"internet_message": strings.TrimSpace(firstString(item, "internetMessageId")),
			"from_email":       strings.TrimSpace(firstString(emailAddress, "address")),
		}
		messages = append(messages, PollMessage{
			ExternalID: strings.TrimSpace(firstString(item, "id")),
			Author:     author,
			Content:    content,
			Timestamp:  timestamp,
			Metadata:   metadata,
		})
		if timestamp != "" && (nextCursor == "" || timestamp > nextCursor) {
			nextCursor = timestamp
		}
		if conversationID == "" && itemConversationID != "" {
			conversationID = itemConversationID
		}
	}
	metadata := map[string]any{
		"provider": "outlook",
		"mailbox":  mailbox,
	}
	if subject != "" {
		metadata["subject"] = subject
	}
	if conversationID != "" {
		metadata["conversation_id"] = conversationID
	}
	for key, value := range parsed {
		metadata[key] = value
	}
	return PollResponse{
		Messages:       messages,
		ConversationID: conversationID,
		NextCursor:     nextCursor,
		Metadata:       metadata,
	}, nil
}

func (d *OutlookDriver) fetchAccessToken(ctx context.Context, cfg map[string]any) (string, error) {
	tenantID := strings.TrimSpace(firstString(cfg, "tenantId", "tenant_id"))
	clientID := strings.TrimSpace(firstString(cfg, "clientId", "client_id"))
	clientSecret := strings.TrimSpace(firstString(cfg, "clientSecret", "client_secret", "secret"))
	if tenantID == "" || clientID == "" || clientSecret == "" {
		return "", fmt.Errorf("outlook connector requires tenantId, clientId, and clientSecret")
	}
	authBaseURL := strings.TrimSpace(firstString(cfg, "authBaseURL", "auth_base_url"))
	if authBaseURL == "" {
		authBaseURL = "https://login.microsoftonline.com"
	}
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("scope", firstNonEmptyString(strings.TrimSpace(firstString(cfg, "scope")), "https://graph.microsoft.com/.default"))

	tokenURL := strings.TrimRight(authBaseURL, "/") + "/" + url.PathEscape(tenantID) + "/oauth2/v2.0/token"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("build outlook token request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := d.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("execute outlook token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return "", fmt.Errorf("read outlook token response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("outlook token request failed with status %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	payload := map[string]any{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", fmt.Errorf("decode outlook token response: %w", err)
	}
	accessToken := strings.TrimSpace(firstString(payload, "access_token"))
	if accessToken == "" {
		return "", fmt.Errorf("outlook token response missing access_token")
	}
	return accessToken, nil
}

func outlookRecipients(addresses []string) []map[string]any {
	out := make([]map[string]any, 0, len(addresses))
	for _, address := range addresses {
		trimmed := strings.TrimSpace(address)
		if trimmed == "" {
			continue
		}
		out = append(out, map[string]any{
			"emailAddress": map[string]any{
				"address": trimmed,
			},
		})
	}
	return out
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
