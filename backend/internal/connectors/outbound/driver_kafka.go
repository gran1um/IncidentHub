package outbound

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

const (
	kafkaSecurityProtocolPlaintext    = "PLAINTEXT"
	kafkaSecurityProtocolSSL          = "SSL"
	kafkaSecurityProtocolSASLPlain    = "SASL_PLAINTEXT"
	kafkaSecurityProtocolSASLSSL      = "SASL_SSL"
	kafkaSASLMechanismPlain           = "PLAIN"
	kafkaDefaultPollEphemeralGroupFmt = "incidenthub-kafka-poll-%d"
)

type KafkaDriver struct {
}

func NewKafkaDriver() *KafkaDriver {
	return &KafkaDriver{}
}

func (d *KafkaDriver) Kind() string {
	return "kafka"
}

func (d *KafkaDriver) Send(ctx context.Context, cfg map[string]any, req SendRequest) (SendResponse, error) {
	topic := strings.TrimSpace(firstString(cfg, "topic", "send_topic", "sendTopic"))
	if topic == "" {
		return SendResponse{}, fmt.Errorf("kafka connector topic is required")
	}
	brokers, err := resolveKafkaBrokers(cfg)
	if err != nil {
		return SendResponse{}, err
	}

	timeout := kafkaConfigDuration(cfg, 10*time.Second, "timeout", "timeout_seconds", "send_timeout", "sendTimeout")
	sendCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	messageKey := strings.TrimSpace(firstString(cfg, "message_key", "messageKey"))
	if messageKey == "" {
		messageKey = defaultString(req.ConversationID, req.ThreadID)
	}

	payload, err := kafkaSendPayload(cfg, req)
	if err != nil {
		return SendResponse{}, err
	}

	producerCfg := kafkaProducerConfig(cfg, brokers, timeout)
	producer, err := kafka.NewProducer(producerCfg)
	if err != nil {
		return SendResponse{}, fmt.Errorf("create kafka producer: %w", err)
	}
	defer producer.Close()

	msg := &kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          payload,
		Timestamp:      time.Now().UTC(),
		Headers:        kafkaHeadersFromConfig(cfg),
	}
	if strings.TrimSpace(messageKey) != "" {
		msg.Key = []byte(messageKey)
	}

	delivery := make(chan kafka.Event, 1)
	if err := producer.Produce(msg, delivery); err != nil {
		return SendResponse{}, fmt.Errorf("write kafka message: %w", err)
	}

	select {
	case event := <-delivery:
		delivered, ok := event.(*kafka.Message)
		if !ok {
			return SendResponse{}, fmt.Errorf("unexpected kafka delivery event type %T", event)
		}
		if delivered.TopicPartition.Error != nil {
			return SendResponse{}, fmt.Errorf("write kafka message: %w", delivered.TopicPartition.Error)
		}
	case <-sendCtx.Done():
		return SendResponse{}, fmt.Errorf("write kafka message: %w", sendCtx.Err())
	}

	conversationID := defaultString(req.ConversationID, messageKey)
	if conversationID == "" {
		conversationID = req.ThreadID
	}

	reply := strings.TrimSpace(firstString(cfg, "deliveryReply", "delivery_reply"))
	if reply == "" {
		reply = fmt.Sprintf("Message delivered to Kafka topic %s", topic)
	}

	return SendResponse{
		Reply:          reply,
		ConversationID: conversationID,
		Metadata: map[string]any{
			"provider": "kafka",
			"topic":    topic,
			"brokers":  brokers,
		},
	}, nil
}

func (d *KafkaDriver) Poll(ctx context.Context, cfg map[string]any, req PollRequest) (PollResponse, error) {
	brokers, err := resolveKafkaBrokers(cfg)
	if err != nil {
		return PollResponse{}, err
	}
	topic := strings.TrimSpace(firstString(cfg, "poll_topic", "pollTopic", "read_topic", "readTopic", "topic"))
	if topic == "" {
		return PollResponse{}, fmt.Errorf("kafka connector poll topic is required")
	}

	pollTimeout := kafkaConfigDuration(cfg, 3*time.Second, "poll_timeout", "pollTimeout", "poll_timeout_seconds", "pollTimeoutSeconds")
	readTimeout := kafkaConfigDuration(cfg, 10*time.Second, "timeout", "timeout_seconds", "poll_read_timeout", "pollReadTimeout")
	maxMessages := kafkaConfigInt(cfg, 20, "poll_max_messages", "pollMaxMessages", "max_messages", "maxMessages", "limit")
	if maxMessages <= 0 {
		maxMessages = 20
	}
	groupID := strings.TrimSpace(firstString(cfg, "poll_group_id", "pollGroupId", "group_id", "groupId"))
	startOffset := strings.ToLower(defaultString(firstString(cfg, "start_offset", "startOffset"), "latest"))
	commit := kafkaConfigBool(cfg, true, "commit", "auto_commit", "autoCommit")
	effectiveGroupID := kafkaResolvePollGroupID(groupID)

	consumerCfg := kafkaConsumerConfig(cfg, brokers, effectiveGroupID, startOffset, pollTimeout, readTimeout)
	consumer, err := kafka.NewConsumer(consumerCfg)
	if err != nil {
		return PollResponse{}, fmt.Errorf("create kafka consumer: %w", err)
	}
	defer func() { _ = consumer.Close() }()

	if groupID != "" {
		if err := consumer.SubscribeTopics([]string{topic}, nil); err != nil {
			return PollResponse{}, fmt.Errorf("subscribe kafka topic: %w", err)
		}
	} else {
		if err := kafkaAssignTopicPartitions(consumer, topic, startOffset, readTimeout); err != nil {
			return PollResponse{}, fmt.Errorf("assign kafka partitions: %w", err)
		}
	}

	readCtx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	messages := make([]PollMessage, 0, maxMessages)
	nextCursor := strings.TrimSpace(req.Cursor)

pollLoop:
	for len(messages) < maxMessages {
		if err := readCtx.Err(); err != nil {
			break
		}
		timeoutMs := kafkaPollTimeoutMillis(readCtx, pollTimeout)
		if timeoutMs <= 0 {
			break
		}
		event := consumer.Poll(timeoutMs)
		if event == nil {
			break
		}

		switch typed := event.(type) {
		case *kafka.Message:
			messages = append(messages, kafkaPollMessage(*typed))
			nextCursor = strconv.FormatInt(int64(typed.TopicPartition.Offset), 10)
			if groupID != "" && commit {
				if _, err := consumer.CommitMessage(typed); err != nil {
					return PollResponse{}, fmt.Errorf("commit kafka poll message: %w", err)
				}
			}
		case kafka.Error:
			if typed.Code() == kafka.ErrTimedOut || typed.IsTimeout() {
				break pollLoop
			}
			if shouldStopKafkaPoll(typed) {
				break pollLoop
			}
			return PollResponse{}, fmt.Errorf("poll kafka messages: %w", typed)
		}
	}

	conversationID := defaultString(req.ConversationID, firstString(cfg, "conversation_id", "conversationId"))

	return PollResponse{
		Messages:       messages,
		ConversationID: conversationID,
		NextCursor:     nextCursor,
		Metadata: map[string]any{
			"provider": "kafka",
			"topic":    topic,
			"brokers":  brokers,
		},
	}, nil
}

func kafkaSendPayload(cfg map[string]any, req SendRequest) ([]byte, error) {
	if kafkaConfigBool(cfg, false, "raw_payload", "rawPayload") {
		return []byte(req.Message), nil
	}
	payload := map[string]any{
		"thread_id":       req.ThreadID,
		"conversation_id": req.ConversationID,
		"author":          req.Author,
		"message":         req.Message,
		"metadata":        req.Metadata,
		"timestamp":       time.Now().UTC().Format(time.RFC3339Nano),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode kafka payload: %w", err)
	}
	return raw, nil
}

func kafkaPollMessage(msg kafka.Message) PollMessage {
	topic := ""
	if msg.TopicPartition.Topic != nil {
		topic = strings.TrimSpace(*msg.TopicPartition.Topic)
	}
	out := PollMessage{
		ExternalID: fmt.Sprintf("%s:%d:%d", topic, msg.TopicPartition.Partition, msg.TopicPartition.Offset),
		Author:     "external",
		Content:    strings.TrimSpace(string(msg.Value)),
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	}
	if !msg.Timestamp.IsZero() {
		out.Timestamp = msg.Timestamp.UTC().Format(time.RFC3339)
	}
	for _, header := range msg.Headers {
		switch strings.ToLower(strings.TrimSpace(header.Key)) {
		case "author":
			if value := strings.TrimSpace(string(header.Value)); value != "" {
				out.Author = value
			}
		case "external_id":
			if value := strings.TrimSpace(string(header.Value)); value != "" {
				out.ExternalID = value
			}
		}
	}
	var payload map[string]any
	if err := json.Unmarshal(msg.Value, &payload); err == nil {
		out.Author = defaultString(firstString(payload, "author", "from"), out.Author)
		out.Content = defaultString(firstString(payload, "content", "message", "text"), out.Content)
		if timestamp := firstString(payload, "timestamp", "created_at", "createdAt"); strings.TrimSpace(timestamp) != "" {
			out.Timestamp = timestamp
		}
		if externalID := firstString(payload, "external_id", "externalId", "id"); strings.TrimSpace(externalID) != "" {
			out.ExternalID = externalID
		}
	}
	return out
}

func resolveKafkaBrokers(cfg map[string]any) ([]string, error) {
	brokers := kafkaConfigBrokers(cfg)
	if len(brokers) == 0 {
		return nil, fmt.Errorf("kafka connector brokers are required")
	}
	return brokers, nil
}

func kafkaConfigBrokers(cfg map[string]any) []string {
	candidates := []string{"brokers", "bootstrap_servers", "bootstrapServers", "broker"}
	for _, key := range candidates {
		if cfg == nil {
			continue
		}
		value, exists := cfg[key]
		if !exists {
			continue
		}
		brokers := parseKafkaBrokerValue(value)
		if len(brokers) > 0 {
			return brokers
		}
	}
	return nil
}

func parseKafkaBrokerValue(value any) []string {
	switch typed := value.(type) {
	case string:
		parts := strings.FieldsFunc(typed, func(r rune) bool {
			return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t'
		})
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			item := strings.TrimSpace(part)
			if item != "" {
				out = append(out, item)
			}
		}
		return out
	case []string:
		out := make([]string, 0, len(typed))
		for _, entry := range typed {
			item := strings.TrimSpace(entry)
			if item != "" {
				out = append(out, item)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(typed))
		for _, entry := range typed {
			item := strings.TrimSpace(fmt.Sprint(entry))
			if item != "" && item != "<nil>" {
				out = append(out, item)
			}
		}
		return out
	default:
		return nil
	}
}

func kafkaHeadersFromConfig(cfg map[string]any) []kafka.Header {
	headers := make([]kafka.Header, 0)
	value, ok := cfg["headers"]
	if !ok {
		value = cfg["defaultHeaders"]
	}
	switch typed := value.(type) {
	case map[string]any:
		for key, item := range typed {
			name := strings.TrimSpace(key)
			if name == "" {
				continue
			}
			headers = append(headers, kafka.Header{Key: name, Value: []byte(strings.TrimSpace(fmt.Sprint(item)))})
		}
	case string:
		parsed := map[string]string{}
		if json.Unmarshal([]byte(typed), &parsed) == nil {
			for key, item := range parsed {
				name := strings.TrimSpace(key)
				if name == "" {
					continue
				}
				headers = append(headers, kafka.Header{Key: name, Value: []byte(strings.TrimSpace(item))})
			}
		}
	}
	return headers
}

func kafkaProducerConfig(cfg map[string]any, brokers []string, timeout time.Duration) *kafka.ConfigMap {
	requestTimeout := kafkaConfigDuration(cfg, timeout, "request_timeout", "requestTimeout", "timeout", "timeout_seconds")
	conf := &kafka.ConfigMap{
		"bootstrap.servers":  strings.Join(brokers, ","),
		"message.timeout.ms": kafkaDurationMs(timeout, 10*time.Second),
		"request.timeout.ms": kafkaDurationMs(requestTimeout, 10*time.Second),
	}
	if clientID := strings.TrimSpace(firstString(cfg, "client_id", "clientId")); clientID != "" {
		_ = conf.SetKey("client.id", clientID)
	}
	kafkaApplySecurityConfig(conf, cfg)
	return conf
}

func kafkaConsumerConfig(cfg map[string]any, brokers []string, groupID, startOffset string, pollTimeout, readTimeout time.Duration) *kafka.ConfigMap {
	sessionTimeout := kafkaConfigDuration(cfg, 45*time.Second, "session_timeout", "sessionTimeout")
	conf := &kafka.ConfigMap{
		"bootstrap.servers":    strings.Join(brokers, ","),
		"group.id":             groupID,
		"enable.auto.commit":   false,
		"auto.offset.reset":    kafkaResolveAutoOffset(startOffset),
		"session.timeout.ms":   kafkaDurationMs(sessionTimeout, 45*time.Second),
		"socket.timeout.ms":    kafkaDurationMs(readTimeout, 10*time.Second),
		"max.poll.interval.ms": kafkaDurationMs(kafkaPositiveDuration(readTimeout+pollTimeout, 5*time.Minute), 5*time.Minute),
	}
	if clientID := strings.TrimSpace(firstString(cfg, "client_id", "clientId")); clientID != "" {
		_ = conf.SetKey("client.id", clientID)
	}
	kafkaApplySecurityConfig(conf, cfg)
	return conf
}

func kafkaResolvePollGroupID(groupID string) string {
	if trimmed := strings.TrimSpace(groupID); trimmed != "" {
		return trimmed
	}
	return fmt.Sprintf(kafkaDefaultPollEphemeralGroupFmt, time.Now().UnixNano())
}

func kafkaAssignTopicPartitions(consumer *kafka.Consumer, topic, startOffset string, metadataTimeout time.Duration) error {
	if consumer == nil {
		return fmt.Errorf("consumer is required")
	}
	metadata, err := consumer.GetMetadata(&topic, false, kafkaDurationMs(metadataTimeout, 10*time.Second))
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
	if kafkaResolveAutoOffset(startOffset) == "earliest" {
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

func kafkaApplySecurityConfig(conf *kafka.ConfigMap, cfg map[string]any) {
	if conf == nil {
		return
	}
	useTLS := kafkaConfigBool(cfg, false, "use_tls", "useTls", "tls", "ssl")
	skipTLSVerify := kafkaConfigBool(cfg, false, "skip_tls_verify", "skipTlsVerify", "insecure_skip_verify", "insecureSkipVerify")
	authType := strings.ToLower(strings.TrimSpace(firstString(cfg, "auth_type", "authType", "sasl_mechanism", "saslMechanism")))
	username := strings.TrimSpace(firstString(cfg, "username", "user"))
	password := strings.TrimSpace(firstString(cfg, "password", "pass"))
	if authType == "" && (username != "" || password != "") {
		authType = "basic"
	}
	hasSASL := false
	switch authType {
	case "basic", "plain", "sasl", "sasl_plain":
		hasSASL = username != "" || password != ""
	}

	securityProtocol := kafkaSecurityProtocolPlaintext
	switch {
	case useTLS && hasSASL:
		securityProtocol = kafkaSecurityProtocolSASLSSL
	case useTLS:
		securityProtocol = kafkaSecurityProtocolSSL
	case hasSASL:
		securityProtocol = kafkaSecurityProtocolSASLPlain
	}
	_ = conf.SetKey("security.protocol", securityProtocol)

	if useTLS && skipTLSVerify {
		_ = conf.SetKey("enable.ssl.certificate.verification", false)
		_ = conf.SetKey("ssl.endpoint.identification.algorithm", "none")
	}
	if hasSASL {
		_ = conf.SetKey("sasl.mechanisms", kafkaSASLMechanismPlain)
		_ = conf.SetKey("sasl.username", username)
		_ = conf.SetKey("sasl.password", password)
	}
}

func kafkaResolveAutoOffset(startOffset string) string {
	if strings.EqualFold(strings.TrimSpace(startOffset), "earliest") {
		return "earliest"
	}
	return "latest"
}

func kafkaPollTimeoutMillis(ctx context.Context, timeout time.Duration) int {
	if timeout <= 0 {
		timeout = time.Second
	}
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return 0
		}
		if remaining < timeout {
			timeout = remaining
		}
	}
	ms := int(timeout / time.Millisecond)
	if ms <= 0 {
		return 1
	}
	return ms
}

func kafkaDurationMs(value, fallback time.Duration) int {
	if value <= 0 {
		value = fallback
	}
	ms := int(value / time.Millisecond)
	if ms <= 0 {
		return int((time.Second / time.Millisecond))
	}
	return ms
}

func kafkaPositiveDuration(value, fallback time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	return fallback
}

func kafkaConfigBool(cfg map[string]any, fallback bool, keys ...string) bool {
	for _, key := range keys {
		value, exists := cfg[key]
		if !exists {
			continue
		}
		switch typed := value.(type) {
		case bool:
			return typed
		case string:
			if parsed, err := strconv.ParseBool(strings.TrimSpace(typed)); err == nil {
				return parsed
			}
		case float64:
			return typed != 0
		case int:
			return typed != 0
		}
	}
	return fallback
}

func kafkaConfigInt(cfg map[string]any, fallback int, keys ...string) int {
	for _, key := range keys {
		value, exists := cfg[key]
		if !exists {
			continue
		}
		switch typed := value.(type) {
		case int:
			return typed
		case int32:
			return int(typed)
		case int64:
			return int(typed)
		case float64:
			return int(typed)
		case float32:
			return int(typed)
		case string:
			if parsed, err := strconv.Atoi(strings.TrimSpace(typed)); err == nil {
				return parsed
			}
		}
	}
	return fallback
}

func kafkaConfigDuration(cfg map[string]any, fallback time.Duration, keys ...string) time.Duration {
	for _, key := range keys {
		value, exists := cfg[key]
		if !exists {
			continue
		}
		switch typed := value.(type) {
		case time.Duration:
			if typed > 0 {
				return typed
			}
		case int:
			if typed > 0 {
				return time.Duration(typed) * time.Second
			}
		case int64:
			if typed > 0 {
				return time.Duration(typed) * time.Second
			}
		case float64:
			if typed > 0 {
				return time.Duration(typed) * time.Second
			}
		case string:
			trimmed := strings.TrimSpace(typed)
			if trimmed == "" {
				continue
			}
			if parsed, err := time.ParseDuration(trimmed); err == nil && parsed > 0 {
				return parsed
			}
			if parsed, err := strconv.Atoi(trimmed); err == nil && parsed > 0 {
				return time.Duration(parsed) * time.Second
			}
		}
	}
	return fallback
}

func shouldStopKafkaPoll(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	text := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(text, "deadline exceeded") || strings.Contains(text, "context canceled")
}
