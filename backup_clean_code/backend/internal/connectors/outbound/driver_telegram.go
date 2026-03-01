package outbound

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type TelegramDriver struct {
	client *http.Client
}

func NewTelegramDriver(client *http.Client) *TelegramDriver {
	return &TelegramDriver{client: client}
}

func (d *TelegramDriver) Kind() string {
	return "telegram"
}

func (d *TelegramDriver) Send(ctx context.Context, cfg map[string]any, req SendRequest) (SendResponse, error) {
	token := strings.TrimSpace(firstString(cfg, "botToken", "token", "accessToken"))
	chatID := resolveTelegramChatID(cfg, req.Metadata, req.ConversationID)
	if token == "" || chatID == "" {
		return SendResponse{}, fmt.Errorf("telegram connector requires botToken and chatId")
	}

	baseURL := strings.TrimSpace(firstString(cfg, "apiBaseURL", "api_base_url"))
	if baseURL == "" {
		return SendResponse{}, fmt.Errorf("telegram connector requires apiBaseURL")
	}
	sendURL := fmt.Sprintf("%s/bot%s/sendMessage", strings.TrimRight(baseURL, "/"), token)

	payload := map[string]any{
		"chat_id": chatID,
		"text":    req.Message,
	}
	rawPayload, _ := json.Marshal(payload)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, sendURL, bytes.NewReader(rawPayload))
	if err != nil {
		return SendResponse{}, fmt.Errorf("build telegram send request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(httpReq)
	if err != nil {
		return SendResponse{}, fmt.Errorf("execute telegram send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	rawBody, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return SendResponse{}, fmt.Errorf("read telegram send response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		description := ""
		var parsedError map[string]any
		if json.Unmarshal(rawBody, &parsedError) == nil {
			description = strings.TrimSpace(firstString(parsedError, "description"))
		}
		if description != "" {
			if strings.Contains(strings.ToLower(description), "chat not found") && !isTelegramNumericID(chatID) {
				return SendResponse{}, fmt.Errorf("telegram recipient %q is not linked to this bot yet: ask user to send /start to the bot and sync once", chatID)
			}
			return SendResponse{}, fmt.Errorf("telegram send failed with status %s: %s", resp.Status, description)
		}
		return SendResponse{}, fmt.Errorf("telegram send failed with status %s", resp.Status)
	}

	reply := strings.TrimSpace(firstString(cfg, "deliveryReply", "delivery_reply"))
	if reply == "" {
		reply = fmt.Sprintf("Message delivered to Telegram chat %s", chatID)
	}

	var parsed map[string]any
	if json.Unmarshal(rawBody, &parsed) == nil {
		replyFromAPI := firstString(parsed, "description")
		if replyFromAPI != "" {
			reply = reply + ". " + replyFromAPI
		}
	}

	return SendResponse{
		Reply:          reply,
		ConversationID: defaultString(req.ConversationID, chatID),
		Metadata: map[string]any{
			"provider": "telegram",
			"chat_id":  chatID,
		},
	}, nil
}

func (d *TelegramDriver) Poll(ctx context.Context, cfg map[string]any, req PollRequest) (PollResponse, error) {
	token := strings.TrimSpace(firstString(cfg, "botToken", "token", "accessToken"))
	chatID := strings.TrimSpace(defaultString(req.ConversationID, firstString(cfg, "chatId", "chat_id")))
	if token == "" || chatID == "" {
		return PollResponse{}, fmt.Errorf("telegram connector requires botToken and chatId")
	}

	baseURL := strings.TrimSpace(firstString(cfg, "apiBaseURL", "api_base_url"))
	if baseURL == "" {
		return PollResponse{}, fmt.Errorf("telegram connector requires apiBaseURL")
	}

	offset := strings.TrimSpace(req.Cursor)
	pollURL := fmt.Sprintf("%s/bot%s/getUpdates?timeout=1", strings.TrimRight(baseURL, "/"), token)
	if offset != "" {
		if parsedOffset, err := strconv.Atoi(offset); err == nil && parsedOffset > 0 {
			pollURL = fmt.Sprintf("%s&offset=%d", pollURL, parsedOffset+1)
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, pollURL, http.NoBody)
	if err != nil {
		return PollResponse{}, fmt.Errorf("build telegram poll request: %w", err)
	}
	resp, err := d.client.Do(httpReq)
	if err != nil {
		return PollResponse{}, fmt.Errorf("execute telegram poll request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return PollResponse{}, fmt.Errorf("telegram poll failed with status %s", resp.Status)
	}

	rawBody, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return PollResponse{}, fmt.Errorf("read telegram poll response: %w", err)
	}
	var payload struct {
		Result []struct {
			UpdateID int `json:"update_id"`
			Message  struct {
				MessageID int   `json:"message_id"`
				Date      int64 `json:"date"`
				From      struct {
					ID        int64  `json:"id"`
					Username  string `json:"username"`
					FirstName string `json:"first_name"`
				} `json:"from"`
				Chat struct {
					ID int64 `json:"id"`
				} `json:"chat"`
				Text string `json:"text"`
			} `json:"message"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return PollResponse{}, fmt.Errorf("decode telegram poll response: %w", err)
	}

	maxUpdate := 0
	messages := make([]PollMessage, 0, len(payload.Result))
	for _, update := range payload.Result {
		if update.UpdateID > maxUpdate {
			maxUpdate = update.UpdateID
		}
		if fmt.Sprint(update.Message.Chat.ID) != chatID {
			continue
		}
		if strings.TrimSpace(update.Message.Text) == "" {
			continue
		}
		author := strings.TrimSpace(update.Message.From.Username)
		if author == "" {
			author = strings.TrimSpace(update.Message.From.FirstName)
		}
		if author == "" {
			author = "telegram-user"
		}
		timestamp := time.Unix(update.Message.Date, 0).UTC().Format(time.RFC3339)
		chatIDString := fmt.Sprint(update.Message.Chat.ID)
		externalUserID := ""
		if update.Message.From.ID != 0 {
			externalUserID = fmt.Sprint(update.Message.From.ID)
		}
		messages = append(messages, PollMessage{
			ExternalID: fmt.Sprintf("telegram-%d", update.Message.MessageID),
			Author:     author,
			Content:    update.Message.Text,
			Timestamp:  timestamp,
			Metadata: map[string]any{
				"provider":         "telegram",
				"chat_id":          chatIDString,
				"external_user_id": externalUserID,
				"username":         strings.TrimSpace(update.Message.From.Username),
				"display_name":     author,
			},
		})
	}

	nextCursor := req.Cursor
	if maxUpdate > 0 {
		nextCursor = strconv.Itoa(maxUpdate)
	}

	return PollResponse{
		Messages:       messages,
		ConversationID: defaultString(req.ConversationID, chatID),
		NextCursor:     nextCursor,
		Metadata: map[string]any{
			"provider": "telegram",
			"chat_id":  chatID,
		},
	}, nil
}

func isTelegramNumericID(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false
	}
	trimmed = strings.TrimPrefix(trimmed, "-")
	if trimmed == "" {
		return false
	}
	for _, ch := range trimmed {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func resolveTelegramChatID(cfg map[string]any, metadata map[string]any, conversationID string) string {
	chatID := strings.TrimSpace(firstString(metadata,
		"chat_id",
		"chatId",
		"recipient_chat_id",
		"recipientChatId",
		"recipient",
		"target",
		"external_user_id",
		"externalUserId",
	))
	if chatID == "" {
		participant := metadata["participant"]
		if participantMap, ok := participant.(map[string]any); ok {
			chatID = strings.TrimSpace(firstString(participantMap,
				"chat_id",
				"chatId",
				"recipient_chat_id",
				"recipientChatId",
				"recipient",
				"target",
				"external_user_id",
				"externalUserId",
			))
		}
	}
	if chatID == "" {
		chatID = strings.TrimSpace(conversationID)
	}
	if chatID == "" {
		chatID = strings.TrimSpace(firstString(cfg, "chatId", "chat_id"))
	}
	return chatID
}
