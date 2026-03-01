package cache

import (
	"context"
	"errors"
	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/metrics"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/redis/go-redis/v9"
)

func TestClientNilBranches(t *testing.T) {
	var nilClient *Client
	if err := nilClient.Close(); err != nil {
		t.Fatalf("nil close should not fail: %v", err)
	}
	if err := nilClient.Set(context.Background(), "a", "b", time.Second); err != nil {
		t.Fatalf("nil set should not fail: %v", err)
	}
	if err := nilClient.Del(context.Background(), "a"); err != nil {
		t.Fatalf("nil del should not fail: %v", err)
	}

	if err := nilClient.Ping(context.Background()); !errors.Is(err, redis.Nil) {
		t.Fatalf("nil ping should return redis.Nil, got %v", err)
	}
	if _, err := nilClient.Get(context.Background(), "a"); !errors.Is(err, redis.Nil) {
		t.Fatalf("nil get should return redis.Nil, got %v", err)
	}
	if _, err := nilClient.Incr(context.Background(), "a"); !errors.Is(err, redis.Nil) {
		t.Fatalf("nil incr should return redis.Nil, got %v", err)
	}
	if _, err := nilClient.IncrWithWindow(context.Background(), "a", 0); !errors.Is(err, redis.Nil) {
		t.Fatalf("nil incrWithWindow should return redis.Nil, got %v", err)
	}
}

func TestClientZeroValueBranches(t *testing.T) {
	zero := &Client{}
	if err := zero.Close(); err != nil {
		t.Fatalf("zero close should not fail: %v", err)
	}
	if err := zero.Set(context.Background(), "a", "b", time.Second); err != nil {
		t.Fatalf("zero set should not fail: %v", err)
	}
	if err := zero.Del(context.Background(), "a"); err != nil {
		t.Fatalf("zero del should not fail: %v", err)
	}

	if err := zero.Ping(context.Background()); !errors.Is(err, redis.Nil) {
		t.Fatalf("zero ping should return redis.Nil, got %v", err)
	}
	if _, err := zero.Get(context.Background(), "a"); !errors.Is(err, redis.Nil) {
		t.Fatalf("zero get should return redis.Nil, got %v", err)
	}
	if _, err := zero.Incr(context.Background(), "a"); !errors.Is(err, redis.Nil) {
		t.Fatalf("zero incr should return redis.Nil, got %v", err)
	}
	if _, err := zero.IncrWithWindow(context.Background(), "a", time.Second); !errors.Is(err, redis.Nil) {
		t.Fatalf("zero incrWithWindow should return redis.Nil, got %v", err)
	}
}

func TestGetMissDoesNotCountAsRedisErrorMetric(t *testing.T) {
	redisServer, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer redisServer.Close()

	collector := metrics.New("incidenthub_test")
	metrics.SetGlobal(collector)
	defer metrics.SetGlobal(nil)

	client, err := New(context.Background(), config.RedisConfig{Addr: redisServer.Addr()})
	if err != nil {
		t.Fatalf("create redis client: %v", err)
	}
	defer func() { _ = client.Close() }()

	if _, err := client.Get(context.Background(), "missing-key"); !errors.Is(err, redis.Nil) {
		t.Fatalf("expected redis.Nil on missing key, got %v", err)
	}

	if got := testutil.ToFloat64(collector.ModuleOperations.WithLabelValues("cache", "get", "miss")); got != 1 {
		t.Fatalf("expected cache/get miss metric = 1, got %v", got)
	}
	if got := testutil.ToFloat64(collector.ModuleOperations.WithLabelValues("cache", "get", "error")); got != 0 {
		t.Fatalf("expected cache/get error metric = 0 for cache miss, got %v", got)
	}
	snapshot := collector.ModuleOperationSnapshot("cache", 60*time.Second)
	if snapshot.Errors != 0 {
		t.Fatalf("expected cache snapshot errors to stay 0 on miss, got %d", snapshot.Errors)
	}
}
