package security

import (
	"context"
	"testing"
	"time"

	"incidenthub/backend/internal/cache"
	"incidenthub/backend/internal/config"

	miniredis "github.com/alicebob/miniredis/v2"
)

func TestNewRedisRateLimiterDefaults(t *testing.T) {
	limiter := NewRedisRateLimiter(nil, 0, 0)
	if limiter.limit != 10 {
		t.Fatalf("expected default limit 10, got %d", limiter.limit)
	}
	if limiter.window != 5*time.Minute {
		t.Fatalf("expected default window 5m, got %s", limiter.window)
	}
	if limiter.prefix != "rl:login:" {
		t.Fatalf("expected default prefix rl:login:, got %s", limiter.prefix)
	}
}

func TestRedisRateLimiterAllowNilBranches(t *testing.T) {
	var nilLimiter *RedisRateLimiter
	if !nilLimiter.Allow("user@example.com") {
		t.Fatal("nil limiter should allow")
	}

	limiter := NewRedisRateLimiter(nil, 2, time.Minute)
	if !limiter.Allow("") {
		t.Fatal("empty key should allow")
	}
	if !limiter.Allow("    ") {
		t.Fatal("blank key should allow")
	}
	if !limiter.Allow("user@example.com") {
		t.Fatal("nil redis client should fail-open and allow")
	}
}

func TestRedisRateLimiterAllowWithRedisCounter(t *testing.T) {
	redisServer, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer redisServer.Close()

	cacheClient, err := cache.New(context.Background(), config.RedisConfig{
		Addr: redisServer.Addr(),
	})
	if err != nil {
		t.Fatalf("init cache client: %v", err)
	}
	defer func() { _ = cacheClient.Close() }()

	limiter := NewRedisRateLimiter(cacheClient, 2, time.Minute)

	if !limiter.Allow("User@Example.com") {
		t.Fatal("first request should be allowed")
	}
	if !limiter.Allow(" user@example.com ") {
		t.Fatal("second request should be allowed and use normalized key")
	}
	if limiter.Allow("USER@EXAMPLE.COM") {
		t.Fatal("third request should be blocked by limit")
	}
	if !limiter.Allow("other-user@example.com") {
		t.Fatal("different key should have independent counter")
	}
}
