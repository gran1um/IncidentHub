package nodes

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"incidenthub/backend/internal/workflow"

	"github.com/redis/go-redis/v9"
)

type redisNode struct{}

func newRedisNode() workflow.NodeExecutor {
	return redisNode{}
}

func (n redisNode) Type() string {
	return "redis"
}

func (n redisNode) Execute(ctx context.Context, req workflow.NodeExecuteRequest) (workflow.NodeExecuteResult, error) {
	options, err := resolveRedisNodeOptions(req.Node.Config, req.Scope)
	if err != nil {
		return workflow.NodeExecuteResult{}, err
	}
	client := redis.NewClient(options)
	defer func() { _ = client.Close() }()

	timeoutSeconds := toInt(req.Node.Config["timeoutSeconds"], 10)
	if timeoutSeconds <= 0 {
		timeoutSeconds = 10
	}
	if timeoutSeconds > 120 {
		timeoutSeconds = 120
	}
	opCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	if err := client.Ping(opCtx).Err(); err != nil {
		return workflow.NodeExecuteResult{}, fmt.Errorf("connect to redis: %w", err)
	}

	operation := normalizeLabel(toString(req.Node.Config["operation"]))
	if operation == "" {
		operation = "set"
	}
	key := strings.TrimSpace(workflow.RenderTemplate(toString(req.Node.Config["key"]), req.Scope))
	if key == "" {
		return workflow.NodeExecuteResult{}, fmt.Errorf("redis key is required")
	}

	output := workflow.CopyMap(req.Payload)
	output["redis_operation"] = operation
	output["redis_key"] = key
	output["redis_db"] = options.DB

	switch operation {
	case "set":
		value := workflow.RenderTemplate(toString(req.Node.Config["value"]), req.Scope)
		if value == "" {
			rawPayload, _ := json.Marshal(req.Payload)
			value = string(rawPayload)
		}
		ttlSeconds := toInt(req.Node.Config["ttlSeconds"], 0)
		ttl := time.Duration(ttlSeconds) * time.Second
		if ttlSeconds < 0 {
			ttl = 0
		}
		if err := client.Set(opCtx, key, value, ttl).Err(); err != nil {
			return workflow.NodeExecuteResult{}, fmt.Errorf("redis SET failed: %w", err)
		}
		output["redis_value"] = value
	case "get":
		value, err := client.Get(opCtx, key).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				output["redis_exists"] = false
				output["redis_value"] = ""
				return workflow.NodeExecuteResult{Output: output, NextLabel: "miss"}, nil
			}
			return workflow.NodeExecuteResult{}, fmt.Errorf("redis GET failed: %w", err)
		}
		output["redis_exists"] = true
		output["redis_value"] = value
		return workflow.NodeExecuteResult{Output: output, NextLabel: "hit"}, nil
	case "del":
		deleted, err := client.Del(opCtx, key).Result()
		if err != nil {
			return workflow.NodeExecuteResult{}, fmt.Errorf("redis DEL failed: %w", err)
		}
		output["redis_deleted"] = deleted
	case "incr":
		nextValue, err := client.Incr(opCtx, key).Result()
		if err != nil {
			return workflow.NodeExecuteResult{}, fmt.Errorf("redis INCR failed: %w", err)
		}
		output["redis_value"] = nextValue
	case "lpush":
		value := workflow.RenderTemplate(toString(req.Node.Config["value"]), req.Scope)
		if value == "" {
			return workflow.NodeExecuteResult{}, fmt.Errorf("value is required for redis LPUSH")
		}
		nextLen, err := client.LPush(opCtx, key, value).Result()
		if err != nil {
			return workflow.NodeExecuteResult{}, fmt.Errorf("redis LPUSH failed: %w", err)
		}
		output["redis_list_length"] = nextLen
	case "rpush":
		value := workflow.RenderTemplate(toString(req.Node.Config["value"]), req.Scope)
		if value == "" {
			return workflow.NodeExecuteResult{}, fmt.Errorf("value is required for redis RPUSH")
		}
		nextLen, err := client.RPush(opCtx, key, value).Result()
		if err != nil {
			return workflow.NodeExecuteResult{}, fmt.Errorf("redis RPUSH failed: %w", err)
		}
		output["redis_list_length"] = nextLen
	case "lpop":
		value, err := client.LPop(opCtx, key).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				output["redis_exists"] = false
				output["redis_value"] = ""
				return workflow.NodeExecuteResult{Output: output, NextLabel: "miss"}, nil
			}
			return workflow.NodeExecuteResult{}, fmt.Errorf("redis LPOP failed: %w", err)
		}
		output["redis_exists"] = true
		output["redis_value"] = value
		return workflow.NodeExecuteResult{Output: output, NextLabel: "hit"}, nil
	case "rpop":
		value, err := client.RPop(opCtx, key).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				output["redis_exists"] = false
				output["redis_value"] = ""
				return workflow.NodeExecuteResult{Output: output, NextLabel: "miss"}, nil
			}
			return workflow.NodeExecuteResult{}, fmt.Errorf("redis RPOP failed: %w", err)
		}
		output["redis_exists"] = true
		output["redis_value"] = value
		return workflow.NodeExecuteResult{Output: output, NextLabel: "hit"}, nil
	default:
		return workflow.NodeExecuteResult{}, fmt.Errorf("unsupported redis operation %q", operation)
	}

	return workflow.NodeExecuteResult{
		Output: output,
	}, nil
}

func resolveRedisNodeOptions(config map[string]any, scope map[string]any) (*redis.Options, error) {
	rawURL := strings.TrimSpace(workflow.RenderTemplate(toString(config["url"]), scope))
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

	addr := strings.TrimSpace(workflow.RenderTemplate(toString(config["addr"]), scope))
	if addr == "" {
		host := strings.TrimSpace(workflow.RenderTemplate(toString(config["host"]), scope))
		port := toInt(config["port"], 6379)
		if host != "" {
			addr = fmt.Sprintf("%s:%d", host, port)
		}
	}
	if addr == "" {
		return nil, fmt.Errorf("redis addr or url is required")
	}
	options.Addr = addr
	options.DB = toInt(config["db"], options.DB)
	if options.DB < 0 {
		options.DB = 0
	}

	username := strings.TrimSpace(workflow.RenderTemplate(toString(config["username"]), scope))
	password := strings.TrimSpace(workflow.RenderTemplate(toString(config["password"]), scope))
	if username != "" {
		options.Username = username
	}
	if password != "" {
		options.Password = password
	}

	if workflow.BoolFromAny(config["useTLS"], false) {
		options.TLSConfig = &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: workflow.BoolFromAny(config["skipTLSVerify"], false), //nolint:gosec // Optional for internal lab environments.
		}
	}
	return options, nil
}
