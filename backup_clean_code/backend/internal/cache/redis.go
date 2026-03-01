package cache

import (
	"context"
	"errors"
	"fmt"
	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/tracing"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	rdb *redis.Client
}

//nolint:gochecknoglobals // Shared Redis Lua script (immutable) reused across client instances.
var incrWindowScript = redis.NewScript(`
local current = redis.call("INCR", KEYS[1])
if current == 1 then
  redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
return current
`)

func New(ctx context.Context, cfg config.RedisConfig) (*Client, error) {
	ctx, span, startedAt := tracing.StartModuleOperation(ctx, "cache", "new")
	var err error
	defer func() {
		tracing.FinishModuleOperation(span, startedAt, "cache", "new", err)
	}()

	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Username:     cfg.Username,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	})

	if err = rdb.Ping(ctx).Err(); err != nil {
		err = fmt.Errorf("ping redis: %w", err)
		return nil, err
	}

	return &Client{rdb: rdb}, nil
}

func (c *Client) Close() error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Close()
}

func (c *Client) Ping(ctx context.Context) error {
	ctx, span, startedAt := tracing.StartModuleOperation(ctx, "cache", "ping")
	var err error
	defer func() {
		tracing.FinishModuleOperation(span, startedAt, "cache", "ping", err)
	}()

	if c == nil || c.rdb == nil {
		err = redis.Nil
		return err
	}
	err = c.rdb.Ping(ctx).Err()
	return err
}

func (c *Client) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	ctx, span, startedAt := tracing.StartModuleOperation(ctx, "cache", "set")
	var err error
	defer func() {
		tracing.FinishModuleOperation(span, startedAt, "cache", "set", err)
	}()

	if c == nil || c.rdb == nil {
		return nil
	}
	err = c.rdb.Set(ctx, key, value, ttl).Err()
	return err
}

func (c *Client) Get(ctx context.Context, key string) (string, error) {
	ctx, span, startedAt := tracing.StartModuleOperation(ctx, "cache", "get")
	var err error
	defer func() {
		tracing.FinishModuleOperationWithStatus(span, startedAt, "cache", "get", cacheGetMetricStatus(err), err)
	}()

	if c == nil || c.rdb == nil {
		err = redis.Nil
		return "", err
	}
	var value string
	value, err = c.rdb.Get(ctx, key).Result()
	return value, err
}

func cacheGetMetricStatus(err error) string {
	if err == nil {
		return "ok"
	}
	if errors.Is(err, redis.Nil) {
		return "miss"
	}
	return "error"
}

func (c *Client) Del(ctx context.Context, key string) error {
	ctx, span, startedAt := tracing.StartModuleOperation(ctx, "cache", "del")
	var err error
	defer func() {
		tracing.FinishModuleOperation(span, startedAt, "cache", "del", err)
	}()

	if c == nil || c.rdb == nil {
		return nil
	}
	err = c.rdb.Del(ctx, key).Err()
	return err
}

func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	ctx, span, startedAt := tracing.StartModuleOperation(ctx, "cache", "incr")
	var err error
	defer func() {
		tracing.FinishModuleOperation(span, startedAt, "cache", "incr", err)
	}()

	if c == nil || c.rdb == nil {
		err = redis.Nil
		return 0, err
	}
	var value int64
	value, err = c.rdb.Incr(ctx, key).Result()
	return value, err
}

func (c *Client) IncrWithWindow(ctx context.Context, key string, window time.Duration) (int64, error) {
	ctx, span, startedAt := tracing.StartModuleOperation(ctx, "cache", "incr_with_window")
	var err error
	defer func() {
		tracing.FinishModuleOperation(span, startedAt, "cache", "incr_with_window", err)
	}()

	if c == nil || c.rdb == nil {
		err = redis.Nil
		return 0, err
	}
	if window <= 0 {
		window = time.Minute
	}

	result, err := incrWindowScript.Run(ctx, c.rdb, []string{key}, window.Milliseconds()).Result()
	if err != nil {
		return 0, err
	}

	switch v := result.(type) {
	case int64:
		return v, nil
	case string:
		parsed, parseErr := strconv.ParseInt(v, 10, 64)
		if parseErr != nil {
			err = fmt.Errorf("parse increment result: %w", parseErr)
			return 0, err
		}
		return parsed, nil
	default:
		err = fmt.Errorf("unexpected increment result type %T", result)
		return 0, err
	}
}
