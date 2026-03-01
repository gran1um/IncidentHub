package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

func produceKafkaMessage(
	ctx context.Context,
	producer *kafka.Producer,
	topic string,
	key []byte,
	value []byte,
	headers []kafka.Header,
	produceTimeout time.Duration,
) error {
	topicName := topic
	msg := &kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topicName, Partition: kafka.PartitionAny},
		Key:            key,
		Value:          value,
		Headers:        headers,
	}
	delivery := make(chan kafka.Event, 1)
	defer close(delivery)
	if err := producer.Produce(msg, delivery); err != nil {
		return err
	}

	sendCtx := ctx
	if sendCtx == nil {
		sendCtx = context.Background()
	}
	if _, hasDeadline := sendCtx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		sendCtx, cancel = context.WithTimeout(sendCtx, produceTimeout)
		defer cancel()
	}

	select {
	case event := <-delivery:
		delivered, ok := event.(*kafka.Message)
		if !ok {
			return fmt.Errorf("unexpected delivery event type %T", event)
		}
		if delivered.TopicPartition.Error != nil {
			return delivered.TopicPartition.Error
		}
		return nil
	case <-sendCtx.Done():
		return sendCtx.Err()
	}
}

func publishKafkaDLQMessage(
	ctx context.Context,
	producer *kafka.Producer,
	produceTimeout time.Duration,
	dlqTopic string,
	sourceTopic string,
	original *kafka.Message,
	key []byte,
	entryKey string,
	entry any,
	attempt int,
	failureErr error,
	marshalErrPrefix string,
) error {
	if strings.TrimSpace(dlqTopic) == "" {
		return nil
	}

	errorText := ""
	if failureErr != nil {
		errorText = failureErr.Error()
	}

	dlqPayload := map[string]any{
		entryKey:       entry,
		"error":        errorText,
		"attempt":      attempt,
		"failed_at":    time.Now().UTC().Format(time.RFC3339Nano),
		"source_topic": sourceTopic,
		"source_partition": func() int32 {
			if original == nil {
				return -1
			}
			return original.TopicPartition.Partition
		}(),
		"source_offset": func() int64 {
			if original == nil {
				return -1
			}
			return int64(original.TopicPartition.Offset)
		}(),
	}

	raw, err := json.Marshal(dlqPayload)
	if err != nil {
		return fmt.Errorf("%s: %w", marshalErrPrefix, err)
	}

	return produceKafkaMessage(ctx, producer, dlqTopic, key, raw, nil, produceTimeout)
}
