package api

import (
	"testing"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

func TestWithRetryHeaderReplacesExistingValue(t *testing.T) {
	headers := []kafka.Header{
		{Key: "other", Value: []byte("ok")},
		{Key: kafkaRetryAttemptHeader, Value: []byte("1")},
	}
	updated := withRetryHeader(headers, 3)
	if len(updated) != 2 {
		t.Fatalf("expected 2 headers, got %d", len(updated))
	}
	if got := kafkaHeaderInt(updated); got != 3 {
		t.Fatalf("expected retry header 3, got %d", got)
	}
}

func TestKafkaHeaderIntParsesAndDefaults(t *testing.T) {
	headers := []kafka.Header{{Key: kafkaRetryAttemptHeader, Value: []byte("2")}}
	if got := kafkaHeaderInt(headers); got != 2 {
		t.Fatalf("expected parsed retry value 2, got %d", got)
	}
	if got := kafkaHeaderInt([]kafka.Header{{Key: kafkaRetryAttemptHeader, Value: []byte("invalid")}}); got != 0 {
		t.Fatalf("invalid header must fallback to 0, got %d", got)
	}
	if got := kafkaHeaderInt(nil); got != 0 {
		t.Fatalf("missing header must fallback to 0, got %d", got)
	}
}

func TestRetryBackoffIsBounded(t *testing.T) {
	queue := &KafkaAsyncOps{
		baseBackoff: 100 * time.Millisecond,
		maxBackoff:  400 * time.Millisecond,
	}
	for _, attempt := range []int{1, 2, 3, 4, 5, 6} {
		got := queue.retryBackoff(attempt)
		if got < 100*time.Millisecond {
			t.Fatalf("attempt %d: backoff too small: %s", attempt, got)
		}
		if got > 400*time.Millisecond {
			t.Fatalf("attempt %d: backoff exceeded max: %s", attempt, got)
		}
	}
}
