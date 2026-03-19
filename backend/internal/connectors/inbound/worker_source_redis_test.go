package inbound

import (
	"context"
	"testing"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestFetchRedisRecords(t *testing.T) {
	server, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer server.Close()

	seed := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer func() { _ = seed.Close() }()

	key := "incidenthub.inbound.alerts"
	if pushErr := seed.RPush(context.Background(), key,
		`{"id":"ext-1","title":"Redis payload","description":"from queue"}`,
		`plain-payload`,
	).Err(); pushErr != nil {
		t.Fatalf("seed redis list: %v", pushErr)
	}

	worker := &Worker{}
	records, err := worker.fetchRedisRecords(context.Background(), inboundConnectorConfig{
		DisplayName: "Redis",
		Redis: inboundRedisSourceConfig{
			Addr:        server.Addr(),
			Key:         key,
			MaxMessages: 10,
			PopFrom:     "left",
		},
	})
	if err != nil {
		t.Fatalf("fetch redis records: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if got := extractString(records[0], "id"); got != "ext-1" {
		t.Fatalf("expected first redis id ext-1, got %q", got)
	}
	if got := extractString(records[1], "value"); got != "plain-payload" {
		t.Fatalf("expected second redis value plain-payload, got %q", got)
	}

	if llen := seed.LLen(context.Background(), key).Val(); llen != 0 {
		t.Fatalf("expected Redis queue to be consumed, got LLEN=%d", llen)
	}
}

func TestFetchRedisRecordsPopRight(t *testing.T) {
	server, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer server.Close()

	seed := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer func() { _ = seed.Close() }()

	key := "incidenthub.inbound.reverse"
	if pushErr := seed.RPush(context.Background(), key, "first", "second").Err(); pushErr != nil {
		t.Fatalf("seed redis list: %v", pushErr)
	}

	worker := &Worker{}
	records, err := worker.fetchRedisRecords(context.Background(), inboundConnectorConfig{
		DisplayName: "Redis",
		Redis: inboundRedisSourceConfig{
			Addr:        server.Addr(),
			Key:         key,
			MaxMessages: 1,
			PopFrom:     "right",
		},
	})
	if err != nil {
		t.Fatalf("fetch redis records: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if got := extractString(records[0], "value"); got != "second" {
		t.Fatalf("expected right-pop payload second, got %q", got)
	}
}

func TestInboundRedisPopMessagesNilClient(t *testing.T) {
	if _, err := inboundRedisPopMessages(context.Background(), nil, "k", "left", 1); err == nil {
		t.Fatal("expected error for nil redis client")
	}
}
