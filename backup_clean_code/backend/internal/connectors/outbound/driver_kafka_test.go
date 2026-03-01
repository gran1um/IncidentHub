package outbound

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

func TestKafkaDriverSendRequiresTopic(t *testing.T) {
	driver := NewKafkaDriver()
	_, err := driver.Send(context.Background(), map[string]any{}, SendRequest{Message: "hello"})
	if err == nil {
		t.Fatal("expected error when kafka topic is missing")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "topic") {
		t.Fatalf("expected topic validation error, got: %v", err)
	}
}

func TestKafkaDriverSendRequiresBrokers(t *testing.T) {
	driver := NewKafkaDriver()
	_, err := driver.Send(context.Background(), map[string]any{"topic": "alerts"}, SendRequest{Message: "hello"})
	if err == nil {
		t.Fatal("expected error when kafka brokers are missing")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "broker") {
		t.Fatalf("expected broker validation error, got: %v", err)
	}
}

func TestParseKafkaBrokerValue(t *testing.T) {
	parsed := parseKafkaBrokerValue("k1:9092, k2:9092; k3:9092")
	if len(parsed) != 3 {
		t.Fatalf("expected 3 brokers, got %d", len(parsed))
	}
	if parsed[0] != "k1:9092" || parsed[2] != "k3:9092" {
		t.Fatalf("unexpected broker list: %#v", parsed)
	}
}

func TestKafkaPollMessageJSON(t *testing.T) {
	topic := "forum-replies"
	msg := kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: 0, Offset: 42},
		Timestamp:      time.Unix(1700000000, 0).UTC(),
		Value:          []byte(`{"author":"bot","message":"pong","external_id":"msg-1","timestamp":"2024-01-01T00:00:00Z"}`),
	}

	out := kafkaPollMessage(msg)
	if out.Author != "bot" {
		t.Fatalf("unexpected author: %q", out.Author)
	}
	if out.Content != "pong" {
		t.Fatalf("unexpected content: %q", out.Content)
	}
	if out.ExternalID != "msg-1" {
		t.Fatalf("unexpected external id: %q", out.ExternalID)
	}
	if out.Timestamp != "2024-01-01T00:00:00Z" {
		t.Fatalf("unexpected timestamp: %q", out.Timestamp)
	}
}
