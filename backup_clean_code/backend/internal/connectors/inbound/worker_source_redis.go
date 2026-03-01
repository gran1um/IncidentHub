package inbound

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

func (w *Worker) fetchRedisRecords(ctx context.Context, cfg inboundConnectorConfig) ([]map[string]any, error) {
	redisCfg := cfg.Redis
	if strings.TrimSpace(redisCfg.Addr) == "" {
		return nil, fmt.Errorf("inbound connector %q requires Redis addr", cfg.DisplayName)
	}
	if strings.TrimSpace(redisCfg.Key) == "" {
		return nil, fmt.Errorf("inbound connector %q requires Redis key", cfg.DisplayName)
	}

	client := redis.NewClient(inboundRedisClientOptions(redisCfg))
	defer func() { _ = client.Close() }()

	reqCtx, cancel := withOptionalTimeout(ctx, firstPositiveDuration(redisCfg.Timeout, cfg.Timeout, 10*time.Second))
	defer cancel()

	if err := client.Ping(reqCtx).Err(); err != nil {
		return nil, fmt.Errorf("ping Redis source for inbound connector %q: %w", cfg.DisplayName, err)
	}

	messages, err := inboundRedisPopMessages(reqCtx, client, redisCfg.Key, redisCfg.PopFrom, redisCfg.MaxMessages)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return []map[string]any{}, nil
		}
		return nil, fmt.Errorf("read Redis messages for inbound connector %q: %w", cfg.DisplayName, err)
	}

	records := make([]map[string]any, 0, len(messages))
	for idx, payload := range messages {
		records = append(records, redisRecordFromPayload(redisCfg.Key, payload, idx))
	}
	return records, nil
}

func inboundRedisClientOptions(cfg inboundRedisSourceConfig) *redis.Options {
	options := &redis.Options{
		Addr:         strings.TrimSpace(cfg.Addr),
		Username:     strings.TrimSpace(cfg.Auth.Username),
		Password:     strings.TrimSpace(cfg.Auth.Password),
		DB:           cfg.DB,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}
	if cfg.UseTLS {
		options.TLSConfig = &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: cfg.SkipTLSVerify, //nolint:gosec // Explicitly configurable for self-signed internal Redis endpoints.
		}
	}
	return options
}

func inboundRedisPopMessages(ctx context.Context, client *redis.Client, key, popFrom string, maxMessages int) ([]string, error) {
	if client == nil {
		return nil, fmt.Errorf("redis client is required")
	}
	if maxMessages <= 0 {
		maxMessages = 100
	}
	if strings.EqualFold(strings.TrimSpace(popFrom), "right") {
		return client.RPopCount(ctx, key, maxMessages).Result()
	}
	return client.LPopCount(ctx, key, maxMessages).Result()
}

func redisRecordFromPayload(key, payload string, index int) map[string]any {
	text := strings.TrimSpace(payload)
	defaultRecord := map[string]any{
		"id":          inboundRedisGeneratedID(key, text, index),
		"title":       fmt.Sprintf("Redis message %s", key),
		"source":      "redis",
		"redis_key":   key,
		"raw_payload": text,
	}

	if text == "" {
		defaultRecord["value"] = ""
		return defaultRecord
	}

	var parsed any
	if err := json.Unmarshal([]byte(text), &parsed); err == nil {
		if record, ok := parsed.(map[string]any); ok {
			if extractString(record, "id") == "" {
				record["id"] = inboundRedisGeneratedID(key, text, index)
			}
			if extractString(record, "title") == "" {
				record["title"] = fmt.Sprintf("Redis message %s", key)
			}
			if extractString(record, "source") == "" {
				record["source"] = "redis"
			}
			record["redis_key"] = key
			record["raw_payload"] = text
			return record
		}
		defaultRecord["payload"] = parsed
		return defaultRecord
	}

	defaultRecord["value"] = text
	return defaultRecord
}

func inboundRedisGeneratedID(key, payload string, index int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%d|%s|%d", key, index, payload, time.Now().UTC().UnixNano())))
	return fmt.Sprintf("redis:%x", sum[:12])
}
