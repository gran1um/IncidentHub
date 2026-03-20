package outbound

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

func TestKafkaSendPayloadRawAndStructured(t *testing.T) {
	rawPayload, err := kafkaSendPayload(map[string]any{"raw_payload": true}, SendRequest{Message: "plain"})
	if err != nil {
		t.Fatalf("raw payload error: %v", err)
	}
	if string(rawPayload) != "plain" {
		t.Fatalf("expected plain raw payload, got %q", string(rawPayload))
	}

	structuredPayload, err := kafkaSendPayload(map[string]any{}, SendRequest{
		ThreadID:       "thread-1",
		ConversationID: "conv-1",
		Author:         "analyst",
		Message:        "hello",
		Metadata:       map[string]any{"scope": "test"},
	})
	if err != nil {
		t.Fatalf("structured payload error: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(structuredPayload, &decoded); err != nil {
		t.Fatalf("decode structured payload: %v", err)
	}
	if decoded["thread_id"] != "thread-1" {
		t.Fatalf("unexpected thread_id: %v", decoded["thread_id"])
	}
	if decoded["message"] != "hello" {
		t.Fatalf("unexpected message: %v", decoded["message"])
	}
}

func TestKafkaHeadersAndBrokersParsing(t *testing.T) {
	headers := kafkaHeadersFromConfig(map[string]any{
		"headers": map[string]any{"X-Test": "1"},
	})
	if len(headers) != 1 || headers[0].Key != "X-Test" {
		t.Fatalf("unexpected headers from map: %#v", headers)
	}

	headers = kafkaHeadersFromConfig(map[string]any{
		"headers": `{"X-Trace":"trace-1"}`,
	})
	if len(headers) != 1 || headers[0].Key != "X-Trace" {
		t.Fatalf("unexpected headers from json string: %#v", headers)
	}

	brokers, err := resolveKafkaBrokers(map[string]any{"brokers": "k1:9092,k2:9092"})
	if err != nil {
		t.Fatalf("resolve brokers error: %v", err)
	}
	if len(brokers) != 2 {
		t.Fatalf("expected 2 brokers, got %d", len(brokers))
	}

	if _, err := resolveKafkaBrokers(map[string]any{}); err == nil {
		t.Fatal("expected error for missing brokers")
	}
}

func TestKafkaConfigParsersAndTimeouts(t *testing.T) {
	cfg := map[string]any{
		"enabled":      "true",
		"limit":        "7",
		"wait_seconds": "2",
	}
	if !kafkaConfigBool(cfg, false, "enabled") {
		t.Fatal("expected bool parser to parse true")
	}
	if got := kafkaConfigInt(cfg, 0, "limit"); got != 7 {
		t.Fatalf("expected int parser to parse 7, got %d", got)
	}
	if got := kafkaConfigDuration(cfg, 0, "wait_seconds"); got != 2*time.Second {
		t.Fatalf("expected duration 2s, got %s", got)
	}

	if kafkaResolveAutoOffset("earliest") != "earliest" {
		t.Fatal("expected earliest auto offset")
	}
	if kafkaResolveAutoOffset("something") != "latest" {
		t.Fatal("expected latest fallback auto offset")
	}
	if kafkaResolvePollGroupID("group-1") != "group-1" {
		t.Fatal("expected explicit group id")
	}
	if generated := kafkaResolvePollGroupID(""); generated == "" {
		t.Fatal("expected generated ephemeral group id")
	}
	if kafkaDurationMs(0, 10*time.Second) <= 0 {
		t.Fatal("expected positive duration in ms")
	}
	if kafkaPositiveDuration(0, 5*time.Second) != 5*time.Second {
		t.Fatal("expected fallback positive duration")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	ms := kafkaPollTimeoutMillis(ctx, time.Second)
	if ms <= 0 {
		t.Fatalf("expected positive poll timeout ms, got %d", ms)
	}
}

func TestKafkaSecurityConfigAndPollStop(t *testing.T) {
	conf := &kafka.ConfigMap{}
	kafkaApplySecurityConfig(conf, map[string]any{
		"use_tls":   true,
		"auth_type": "basic",
		"username":  "svc",
		"password":  "secret",
	})
	value, err := conf.Get("security.protocol", "")
	if err != nil {
		t.Fatalf("read security protocol: %v", err)
	}
	if got := strings.ToUpper(value.(string)); got != kafkaSecurityProtocolSASLSSL {
		t.Fatalf("expected %s, got %s", kafkaSecurityProtocolSASLSSL, got)
	}

	if !shouldStopKafkaPoll(context.Canceled) {
		t.Fatal("expected canceled context to stop poll")
	}
	if !shouldStopKafkaPoll(errors.New("deadline exceeded while polling")) {
		t.Fatal("expected deadline exceeded text to stop poll")
	}
	if shouldStopKafkaPoll(errors.New("temporary network error")) {
		t.Fatal("expected generic error to not stop poll")
	}
}
