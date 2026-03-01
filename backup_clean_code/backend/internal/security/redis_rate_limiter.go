package security

import (
	"context"
	"incidenthub/backend/internal/cache"
	"strings"
	"time"
)

type RedisRateLimiter struct {
	cache  *cache.Client
	limit  int
	window time.Duration
	prefix string
}

func NewRedisRateLimiter(cacheClient *cache.Client, limit int, window time.Duration) *RedisRateLimiter {
	if limit <= 0 {
		limit = 10
	}
	if window <= 0 {
		window = 5 * time.Minute
	}
	return &RedisRateLimiter{
		cache:  cacheClient,
		limit:  limit,
		window: window,
		prefix: "rl:login:",
	}
}

func (r *RedisRateLimiter) Allow(key string) bool {
	if r == nil {
		return true
	}
	normalized := strings.TrimSpace(strings.ToLower(key))
	if normalized == "" {
		return true
	}

	counter, err := r.cache.IncrWithWindow(context.Background(), r.prefix+normalized, r.window)
	if err != nil {
		// Fail-open: avoid full auth outage when redis is temporarily unavailable.
		return true
	}
	return counter <= int64(r.limit)
}
