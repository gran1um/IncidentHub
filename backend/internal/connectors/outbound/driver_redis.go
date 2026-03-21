package outbound

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisDriver struct {
}

func NewRedisDriver() *RedisDriver {
	return &RedisDriver{}
}

func (d *RedisDriver) Kind() string {
	return "redis"
}

func (d *RedisDriver) Send(ctx context.Context, cfg map[string]any, req SendRequest) (SendResponse, error) {
	options, err := resolveRedisDriverOptions(cfg)
	if err != nil {
		return SendResponse{}, err
	}
	sendKey := strings.TrimSpace(firstString(cfg, "send_key", "sendKey", "key", "queue", "queue_key", "queueKey", "list", "list_key", "listKey"))
	if sendKey == "" {
		return SendResponse{}, fmt.Errorf("redis connector send key is required")
	}

	timeout := redisDriverConfigDuration(cfg, 10*time.Second, "timeout", "timeout_seconds", "send_timeout", "sendTimeout")
	sendCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client := redis.NewClient(options)
	defer func() { _ = client.Close() }()

	if pingErr := client.Ping(sendCtx).Err(); pingErr != nil {
		return SendResponse{}, fmt.Errorf("connect to redis: %w", pingErr)
	}

	payload, err := redisDriverSendPayload(cfg, req)
	if err != nil {
		return SendResponse{}, err
	}
	if err := client.RPush(sendCtx, sendKey, payload).Err(); err != nil {
		return SendResponse{}, fmt.Errorf("write redis message: %w", err)
	}

	reply := strings.TrimSpace(firstString(cfg, "deliveryReply", "delivery_reply"))
	if reply == "" {
		reply = fmt.Sprintf("Message queued to Redis list %s", sendKey)
	}

	return SendResponse{
		Reply:          reply,
		ConversationID: defaultString(req.ConversationID, sendKey),
		Metadata: map[string]any{
			"provider": "redis",
			"key":      sendKey,
			"db":       options.DB,
		},
	}, nil
}

func (d *RedisDriver) Poll(ctx context.Context, cfg map[string]any, req PollRequest) (PollResponse, error) {
	options, err := resolveRedisDriverOptions(cfg)
	if err != nil {
		return PollResponse{}, err
	}
	pollKey := strings.TrimSpace(firstString(cfg, "poll_key", "pollKey", "key", "queue", "queue_key", "queueKey", "list", "list_key", "listKey"))
	if pollKey == "" {
		return PollResponse{}, fmt.Errorf("redis connector poll key is required")
	}

	maxMessages := redisDriverConfigInt(cfg, 20, "poll_max_messages", "pollMaxMessages", "max_messages", "maxMessages", "limit")
	if maxMessages <= 0 {
		maxMessages = 20
	}
	popFrom := strings.ToLower(defaultString(firstString(cfg, "pop_from", "popFrom", "side"), "left"))
	if popFrom != "left" && popFrom != "right" {
		return PollResponse{}, fmt.Errorf("redis connector pop_from must be left or right")
	}

	timeout := redisDriverConfigDuration(cfg, 10*time.Second, "timeout", "timeout_seconds", "poll_timeout", "pollTimeout")
	pollCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client := redis.NewClient(options)
	defer func() { _ = client.Close() }()

	if pingErr := client.Ping(pollCtx).Err(); pingErr != nil {
		return PollResponse{}, fmt.Errorf("connect to redis: %w", pingErr)
	}

	values, err := redisDriverPopMessages(pollCtx, client, pollKey, popFrom, maxMessages)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return PollResponse{
				Messages:       []PollMessage{},
				ConversationID: defaultString(req.ConversationID, pollKey),
				NextCursor:     req.Cursor,
				Metadata: map[string]any{
					"provider": "redis",
					"key":      pollKey,
					"db":       options.DB,
				},
			}, nil
		}
		return PollResponse{}, fmt.Errorf("read redis messages: %w", err)
	}

	messages := make([]PollMessage, 0, len(values))
	for idx, value := range values {
		messages = append(messages, redisDriverPollMessage(pollKey, value, idx))
	}

	return PollResponse{
		Messages:       messages,
		ConversationID: defaultString(req.ConversationID, pollKey),
		NextCursor:     req.Cursor,
		Metadata: map[string]any{
			"provider": "redis",
			"key":      pollKey,
			"db":       options.DB,
		},
	}, nil
}

func resolveRedisDriverOptions(cfg map[string]any) (*redis.Options, error) {
	rawURL := strings.TrimSpace(firstString(cfg, "url", "redis_url", "redisUrl", "dsn"))
	var options *redis.Options
	if rawURL != "" {
		parsed, err := redis.ParseURL(rawURL)
		if err != nil {
			return nil, fmt.Errorf("parse redis url: %w", err)
		}
		options = parsed
	} else {
		options = &redis.Options{}
	}

	addr := strings.TrimSpace(firstString(cfg, "addr", "address"))
	if addr == "" {
		host := strings.TrimSpace(firstString(cfg, "host"))
		if host != "" {
			addr = fmt.Sprintf("%s:%d", host, redisDriverConfigInt(cfg, 6379, "port"))
		}
	}
	if addr != "" {
		options.Addr = addr
	}
	if strings.TrimSpace(options.Addr) == "" {
		return nil, fmt.Errorf("redis connector addr or url is required")
	}

	if username := strings.TrimSpace(firstString(cfg, "username", "user")); username != "" {
		options.Username = username
	}
	if password := strings.TrimSpace(firstString(cfg, "password", "pass")); password != "" {
		options.Password = password
	}
	options.DB = redisDriverConfigInt(cfg, options.DB, "db", "database")
	if options.DB < 0 {
		options.DB = 0
	}

	dialFallback := options.DialTimeout
	if dialFallback <= 0 {
		dialFallback = 5 * time.Second
	}
	readFallback := options.ReadTimeout
	if readFallback <= 0 {
		readFallback = 5 * time.Second
	}
	writeFallback := options.WriteTimeout
	if writeFallback <= 0 {
		writeFallback = 5 * time.Second
	}
	options.DialTimeout = redisDriverConfigDuration(cfg, dialFallback, "dial_timeout", "dialTimeout")
	options.ReadTimeout = redisDriverConfigDuration(cfg, readFallback, "read_timeout", "readTimeout")
	options.WriteTimeout = redisDriverConfigDuration(cfg, writeFallback, "write_timeout", "writeTimeout")

	useTLS := redisDriverConfigBool(cfg, false, "use_tls", "useTls", "tls", "ssl")
	if useTLS {
		skipVerify := redisDriverConfigBool(
			cfg,
			false,
			"skip_tls_verify",
			"skipTlsVerify",
			"insecure_skip_verify",
			"insecureSkipVerify",
		)

		options.TLSConfig = &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: skipVerify, //nolint:gosec // Configurable for internal/self-signed Redis endpoints.
		}
	}

	return options, nil
}

func redisDriverSendPayload(cfg map[string]any, req SendRequest) (string, error) {
	if redisDriverConfigBool(cfg, false, "raw_payload", "rawPayload") {
		return req.Message, nil
	}
	payload := map[string]any{
		"thread_id":       req.ThreadID,
		"conversation_id": req.ConversationID,
		"author":          req.Author,
		"message":         req.Message,
		"metadata":        req.Metadata,
		"timestamp":       time.Now().UTC().Format(time.RFC3339Nano),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode redis payload: %w", err)
	}
	return string(raw), nil
}

func redisDriverPopMessages(ctx context.Context, client *redis.Client, key, popFrom string, maxMessages int) ([]string, error) {
	if client == nil {
		return nil, fmt.Errorf("redis client is required")
	}
	if maxMessages <= 0 {
		maxMessages = 20
	}
	if strings.EqualFold(popFrom, "right") {
		return client.RPopCount(ctx, key, maxMessages).Result()
	}
	return client.LPopCount(ctx, key, maxMessages).Result()
}

func redisDriverPollMessage(key, value string, index int) PollMessage {
	content := strings.TrimSpace(value)
	message := PollMessage{
		ExternalID: redisDriverGeneratedExternalID(key, index),
		Author:     "external",
		Content:    content,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Metadata: map[string]any{
			"provider": "redis",
			"key":      key,
		},
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return message
	}

	if externalID := firstString(payload, "external_id", "externalId", "id"); strings.TrimSpace(externalID) != "" {
		message.ExternalID = externalID
	}
	message.Author = defaultString(firstString(payload, "author", "from", "user"), message.Author)
	message.Content = defaultString(firstString(payload, "content", "message", "text"), message.Content)
	if timestamp := strings.TrimSpace(firstString(payload, "timestamp", "created_at", "createdAt")); timestamp != "" {
		message.Timestamp = timestamp
	}

	if metadata, ok := payload["metadata"].(map[string]any); ok {
		merged := map[string]any{
			"provider": "redis",
			"key":      key,
		}
		for itemKey, itemValue := range metadata {
			merged[itemKey] = itemValue
		}
		message.Metadata = merged
	}

	return message
}

func redisDriverGeneratedExternalID(key string, index int) string {
	return fmt.Sprintf("%s:%d:%d", key, time.Now().UTC().UnixNano(), index)
}

func redisDriverConfigBool(cfg map[string]any, fallback bool, keys ...string) bool {
	for _, key := range keys {
		value, exists := cfg[key]
		if !exists {
			continue
		}
		switch typed := value.(type) {
		case bool:
			return typed
		case string:
			if parsed, err := strconv.ParseBool(strings.TrimSpace(typed)); err == nil {
				return parsed
			}
		case float64:
			return typed != 0
		case float32:
			return typed != 0
		case int:
			return typed != 0
		case int32:
			return typed != 0
		case int64:
			return typed != 0
		}
	}
	return fallback
}

func redisDriverConfigInt(cfg map[string]any, fallback int, keys ...string) int {
	for _, key := range keys {
		value, exists := cfg[key]
		if !exists {
			continue
		}
		switch typed := value.(type) {
		case int:
			return typed
		case int32:
			return int(typed)
		case int64:
			return int(typed)
		case float64:
			return int(typed)
		case float32:
			return int(typed)
		case string:
			if parsed, err := strconv.Atoi(strings.TrimSpace(typed)); err == nil {
				return parsed
			}
		}
	}
	return fallback
}

func redisDriverConfigDuration(cfg map[string]any, fallback time.Duration, keys ...string) time.Duration {
	for _, key := range keys {
		value, exists := cfg[key]
		if !exists {
			continue
		}
		switch typed := value.(type) {
		case time.Duration:
			if typed > 0 {
				return typed
			}
		case int:
			if typed > 0 {
				return time.Duration(typed) * time.Second
			}
		case int32:
			if typed > 0 {
				return time.Duration(typed) * time.Second
			}
		case int64:
			if typed > 0 {
				return time.Duration(typed) * time.Second
			}
		case float64:
			if typed > 0 {
				return time.Duration(typed) * time.Second
			}
		case float32:
			if typed > 0 {
				return time.Duration(typed) * time.Second
			}
		case string:
			trimmed := strings.TrimSpace(typed)
			if trimmed == "" {
				continue
			}
			if parsed, err := time.ParseDuration(trimmed); err == nil && parsed > 0 {
				return parsed
			}
			if parsed, err := strconv.Atoi(trimmed); err == nil && parsed > 0 {
				return time.Duration(parsed) * time.Second
			}
		}
	}
	return fallback
}
