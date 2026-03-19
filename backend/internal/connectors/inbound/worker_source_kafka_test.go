package inbound

import (
	"testing"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

func TestKafkaMessageToRecordsArrayPayload(t *testing.T) {
	topic := "incidenthub-alerts"
	message := kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: 1, Offset: 9},
		Timestamp:      time.Unix(1700000000, 0).UTC(),
		Value: []byte(`{
			"data": {
				"items": [
					{"id":"a-1","title":"Alert One"},
					{"title":"Alert Two"}
				]
			}
		}`),
	}

	records := kafkaMessageToRecords(message, "data.items")
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if records[0]["id"] != "a-1" {
		t.Fatalf("unexpected first id: %v", records[0]["id"])
	}
	if records[1]["id"] == "" {
		t.Fatal("expected generated id for second record")
	}
	if records[0]["kafka_topic"] != topic {
		t.Fatalf("unexpected kafka topic: %v", records[0]["kafka_topic"])
	}
}

func TestKafkaMessageToRecordsPlainText(t *testing.T) {
	topic := "incidenthub-alerts"
	message := kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: 0, Offset: 3},
		Value:          []byte("raw message"),
	}

	records := kafkaMessageToRecords(message, "")
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0]["description"] != "raw message" {
		t.Fatalf("unexpected description: %v", records[0]["description"])
	}
}
