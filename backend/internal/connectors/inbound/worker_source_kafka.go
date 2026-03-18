package inbound

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

const (
	inboundKafkaEphemeralGroupIDFmt = "incidenthub-inbound-poll-%d"
	kafkaSecurityProtocolPlaintext  = "PLAINTEXT"
	kafkaSecurityProtocolSSL        = "SSL"
	kafkaSecurityProtocolSASLPlain  = "SASL_PLAINTEXT"
	kafkaSecurityProtocolSASLSSL    = "SASL_SSL"
	kafkaSASLMechanismPlain         = "PLAIN"
)

func (w *Worker) fetchKafkaRecords(ctx context.Context, cfg inboundConnectorConfig) ([]map[string]any, error) {
	kafkaCfg := cfg.Kafka
	brokers := kafkaCfg.Brokers
	if len(brokers) == 0 {
		return nil, fmt.Errorf("inbound connector %q requires at least one Kafka broker", cfg.DisplayName)
	}
	if strings.TrimSpace(kafkaCfg.Topic) == "" {
		return nil, fmt.Errorf("inbound connector %q requires Kafka topic", cfg.DisplayName)
	}

	maxMessages := kafkaCfg.MaxMessages
	if maxMessages <= 0 {
		maxMessages = 100
	}
	pollTimeout := firstPositiveDuration(kafkaCfg.PollTimeout, cfg.Timeout, 5*time.Second)
	if pollTimeout <= 0 {
		pollTimeout = 5 * time.Second
	}
	dialTimeout := firstPositiveDuration(kafkaCfg.DialTimeout, 10*time.Second)
	groupID := strings.TrimSpace(kafkaCfg.GroupID)
	effectiveGroupID := groupID
	if effectiveGroupID == "" {
		effectiveGroupID = fmt.Sprintf(inboundKafkaEphemeralGroupIDFmt, time.Now().UnixNano())
	}

	consumerCfg := inboundKafkaConsumerConfig(kafkaCfg, brokers, effectiveGroupID, pollTimeout, dialTimeout)
	consumer, err := kafka.NewConsumer(consumerCfg)
	if err != nil {
		return nil, fmt.Errorf("create Kafka consumer for inbound connector %q: %w", cfg.DisplayName, err)
	}
	defer func() { _ = consumer.Close() }()

	if groupID != "" {
		if err := consumer.SubscribeTopics([]string{kafkaCfg.Topic}, nil); err != nil {
			return nil, fmt.Errorf("subscribe Kafka topic for inbound connector %q: %w", cfg.DisplayName, err)
		}
	} else {
		if err := inboundKafkaAssignTopicPartitions(consumer, kafkaCfg.Topic, kafkaCfg.StartOffset, dialTimeout); err != nil {
			return nil, fmt.Errorf("assign Kafka topic partitions for inbound connector %q: %w", cfg.DisplayName, err)
		}
	}

	records := make([]map[string]any, 0, maxMessages)
pollLoop:
	for len(records) < maxMessages {
		if err := ctx.Err(); err != nil {
			if shouldStopKafkaPolling(err) {
				break
			}
			return nil, fmt.Errorf("poll Kafka messages for inbound connector %q: %w", cfg.DisplayName, err)
		}
		event := consumer.Poll(inboundKafkaPollTimeoutMillis(ctx, pollTimeout))
		if event == nil {
			break
		}

		switch typed := event.(type) {
		case *kafka.Message:
			messageRecords := kafkaMessageToRecords(*typed, cfg.ArrayPath)
			for _, record := range messageRecords {
				records = append(records, record)
				if len(records) >= maxMessages {
					break
				}
			}
			if groupID != "" && kafkaCfg.Commit {
				if _, err := consumer.CommitMessage(typed); err != nil {
					return nil, fmt.Errorf("commit Kafka message for inbound connector %q: %w", cfg.DisplayName, err)
				}
			}
		case kafka.Error:
			if typed.Code() == kafka.ErrTimedOut || typed.IsTimeout() {
				break pollLoop
			}
			if shouldStopKafkaPolling(typed) {
				break pollLoop
			}
			return nil, fmt.Errorf("fetch Kafka message for inbound connector %q: %w", cfg.DisplayName, typed)
		}
	}

	return records, nil
}

func inboundKafkaConsumerConfig(cfg inboundKafkaSourceConfig, brokers []string, groupID string, pollTimeout, dialTimeout time.Duration) *kafka.ConfigMap {
	sessionTimeout := firstPositiveDuration(dialTimeout, 45*time.Second)
	maxPollInterval := firstPositiveDuration(pollTimeout*3, 5*time.Minute)
	conf := &kafka.ConfigMap{
		"bootstrap.servers":    strings.Join(brokers, ","),
		"group.id":             groupID,
		"enable.auto.commit":   false,
		"auto.offset.reset":    inboundKafkaAutoOffset(cfg.StartOffset),
		"session.timeout.ms":   inboundKafkaDurationMs(sessionTimeout),
		"socket.timeout.ms":    inboundKafkaDurationMs(dialTimeout),
		"max.poll.interval.ms": inboundKafkaDurationMs(maxPollInterval),
	}
	if clientID := strings.TrimSpace(cfg.ClientID); clientID != "" {
		_ = conf.SetKey("client.id", clientID)
	}
	inboundKafkaApplySecurityConfig(conf, cfg)
	return conf
}

func inboundKafkaApplySecurityConfig(conf *kafka.ConfigMap, cfg inboundKafkaSourceConfig) {
	if conf == nil {
		return
	}
	authType := strings.ToLower(strings.TrimSpace(cfg.Auth.Type))
	if authType == "" && strings.TrimSpace(cfg.Auth.Username) != "" {
		authType = "basic"
	}
	username := strings.TrimSpace(cfg.Auth.Username)
	password := strings.TrimSpace(cfg.Auth.Password)
	hasSASL := false
	switch authType {
	case "basic", "plain", "sasl", "sasl_plain":
		hasSASL = username != "" || password != ""
	}

	securityProtocol := kafkaSecurityProtocolPlaintext
	switch {
	case cfg.UseTLS && hasSASL:
		securityProtocol = kafkaSecurityProtocolSASLSSL
	case cfg.UseTLS:
		securityProtocol = kafkaSecurityProtocolSSL
	case hasSASL:
		securityProtocol = kafkaSecurityProtocolSASLPlain
	}
	_ = conf.SetKey("security.protocol", securityProtocol)

	if cfg.UseTLS && cfg.SkipTLSVerify {
		_ = conf.SetKey("enable.ssl.certificate.verification", false)
		_ = conf.SetKey("ssl.endpoint.identification.algorithm", "none")
	}
	if hasSASL {
		_ = conf.SetKey("sasl.mechanisms", kafkaSASLMechanismPlain)
		_ = conf.SetKey("sasl.username", username)
		_ = conf.SetKey("sasl.password", password)
	}
}

func inboundKafkaAssignTopicPartitions(consumer *kafka.Consumer, topic, startOffset string, metadataTimeout time.Duration) error {
	if consumer == nil {
		return fmt.Errorf("consumer is required")
	}
	metadata, err := consumer.GetMetadata(&topic, false, inboundKafkaDurationMs(firstPositiveDuration(metadataTimeout, 10*time.Second)))
	if err != nil {
		return fmt.Errorf("load metadata for topic %q: %w", topic, err)
	}

	topicMetadata, ok := metadata.Topics[topic]
	if !ok {
		return fmt.Errorf("kafka topic %q metadata not found", topic)
	}
	if topicMetadata.Error.Code() != kafka.ErrNoError {
		return fmt.Errorf("kafka topic %q metadata error: %w", topic, topicMetadata.Error)
	}

	offset := kafka.OffsetEnd
	if inboundKafkaAutoOffset(startOffset) == "earliest" {
		offset = kafka.OffsetBeginning
	}
	assignments := make([]kafka.TopicPartition, 0, len(topicMetadata.Partitions))
	for _, partition := range topicMetadata.Partitions {
		assignments = append(assignments, kafka.TopicPartition{
			Topic:     &topic,
			Partition: partition.ID,
			Offset:    offset,
		})
	}
	if len(assignments) == 0 {
		return fmt.Errorf("kafka topic %q has no partitions", topic)
	}
	if err := consumer.Assign(assignments); err != nil {
		return fmt.Errorf("assign topic partitions for topic %q: %w", topic, err)
	}
	return nil
}

func inboundKafkaAutoOffset(startOffset string) string {
	if strings.EqualFold(strings.TrimSpace(startOffset), "earliest") {
		return "earliest"
	}
	return "latest"
}

func inboundKafkaDurationMs(value time.Duration) int {
	if value <= 0 {
		value = time.Second
	}
	ms := int(value / time.Millisecond)
	if ms <= 0 {
		return 1
	}
	return ms
}

func inboundKafkaPollTimeoutMillis(ctx context.Context, timeout time.Duration) int {
	if timeout <= 0 {
		timeout = time.Second
	}
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return 1
		}
		if remaining < timeout {
			timeout = remaining
		}
	}
	return inboundKafkaDurationMs(timeout)
}

func shouldStopKafkaPolling(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	text := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(text, "deadline exceeded") || strings.Contains(text, "context canceled")
}

func kafkaMessageToRecords(message kafka.Message, arrayPath string) []map[string]any {
	baseID := kafkaMessageID(message)
	topic := ""
	if message.TopicPartition.Topic != nil {
		topic = strings.TrimSpace(*message.TopicPartition.Topic)
	}
	baseRecord := map[string]any{
		"id":              baseID,
		"title":           fmt.Sprintf("Kafka message %s", topic),
		"source":          "kafka",
		"kafka_topic":     topic,
		"kafka_partition": message.TopicPartition.Partition,
		"kafka_offset":    int64(message.TopicPartition.Offset),
		"kafka_timestamp": kafkaMessageTimestamp(message),
	}
	if len(message.Key) > 0 {
		baseRecord["kafka_key"] = string(message.Key)
	}

	trimmedValue := strings.TrimSpace(string(message.Value))
	if trimmedValue == "" {
		return []map[string]any{baseRecord}
	}

	var payload any
	if err := json.Unmarshal(message.Value, &payload); err != nil {
		baseRecord["description"] = trimmedValue
		return []map[string]any{baseRecord}
	}

	records := extractRecords(payload, arrayPath)
	if len(records) == 0 {
		baseRecord["payload"] = payload
		return []map[string]any{baseRecord}
	}

	for index, record := range records {
		if record == nil {
			record = map[string]any{}
			records[index] = record
		}
		for key, value := range baseRecord {
			if _, exists := record[key]; !exists {
				record[key] = value
			}
		}
		if strings.TrimSpace(extractString(record, "id")) == "" {
			id := baseID
			if len(records) > 1 {
				id = fmt.Sprintf("%s-%d", baseID, index)
			}
			record["id"] = id
		}
	}

	return records
}

func kafkaMessageID(message kafka.Message) string {
	topic := ""
	if message.TopicPartition.Topic != nil {
		topic = strings.TrimSpace(*message.TopicPartition.Topic)
	}
	return fmt.Sprintf("%s:%d:%d", topic, message.TopicPartition.Partition, message.TopicPartition.Offset)
}

func kafkaMessageTimestamp(message kafka.Message) string {
	if message.Timestamp.IsZero() {
		return time.Now().UTC().Format(time.RFC3339Nano)
	}
	return message.Timestamp.UTC().Format(time.RFC3339Nano)
}
