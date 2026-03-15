package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/connectors/outbound"
	"incidenthub/backend/internal/logger"
	"incidenthub/backend/internal/metrics"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"
	"incidenthub/backend/internal/tracing"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Sentinel errors for notifications Kafka queue (err113).
var (
	errKafkaNotificationsDepsRequired      = errors.New("notifications kafka requires settings, bots, catalog and outbound service")
	errKafkaNotificationsTopicRequired     = errors.New("notifications kafka topic is required")
	errKafkaNotificationsGroupIDRequired   = errors.New("notifications kafka group id is required")
	errKafkaNotificationsBrokersRequired   = errors.New("notifications kafka brokers are required")
	errKafkaNotificationsQueueDisabled     = errors.New("notifications kafka queue is disabled")
	errKafkaNotificationsEventIDRequired   = errors.New("notification event id is required")
	errKafkaNotificationsRequeueMessageNil = errors.New("requeue message is nil")
)

type KafkaNotificationDeliveryDependencies struct {
	Settings *repository.UserNotificationSettingsRepository
	Bots     *repository.TelegramNotificationBotRepository
	Catalog  *repository.CatalogRepository
	Outbound *outbound.Service
}

type KafkaNotificationDelivery struct {
	enabled        bool
	topic          string
	dlqTopic       string
	pollTimeout    time.Duration
	produceTimeout time.Duration
	consumer       *kafka.Consumer
	producer       *kafka.Producer
	maxAttempts    int
	baseBackoff    time.Duration
	maxBackoff     time.Duration

	settings *repository.UserNotificationSettingsRepository
	bots     *repository.TelegramNotificationBotRepository
	catalog  *repository.CatalogRepository
	outbound *outbound.Service

	telegramAPIBaseURL string
	startOnce          sync.Once
}

func NewKafkaNotificationDelivery(cfg config.NotificationDeliveryConfig, outboundCfg config.OutboundConnectorsConfig, deps KafkaNotificationDeliveryDependencies) (*KafkaNotificationDelivery, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	if deps.Settings == nil || deps.Bots == nil || deps.Catalog == nil || deps.Outbound == nil {
		return nil, errKafkaNotificationsDepsRequired
	}
	topic := strings.TrimSpace(cfg.KafkaTopic)
	if topic == "" {
		return nil, errKafkaNotificationsTopicRequired
	}
	groupID := strings.TrimSpace(cfg.KafkaGroupID)
	if groupID == "" {
		return nil, errKafkaNotificationsGroupIDRequired
	}
	brokers := cfg.BrokerList()
	if len(brokers) == 0 {
		return nil, errKafkaNotificationsBrokersRequired
	}

	produceTimeout := positiveDuration(cfg.ProduceTimeout, 5*time.Second)
	pollTimeout := positiveDuration(cfg.PollTimeout, time.Second)
	dlqTopic := strings.TrimSpace(cfg.KafkaDLQTopic)
	if dlqTopic == "" {
		dlqTopic = topic + ".dlq"
	}
	maxAttempts := cfg.RetryMaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	baseBackoff := positiveDuration(cfg.RetryBaseBackoff, 500*time.Millisecond)
	maxBackoff := positiveDuration(cfg.RetryMaxBackoff, 15*time.Second)

	bootstrapServers := strings.Join(brokers, ",")
	producer, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers": bootstrapServers,
		"message.timeout.ms": func() int {
			ms := int(produceTimeout / time.Millisecond)
			if ms <= 0 {
				return 5000
			}
			return ms
		}(),
	})
	if err != nil {
		return nil, fmt.Errorf("create notifications kafka producer: %w", err)
	}

	consumer, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":  bootstrapServers,
		"group.id":           groupID,
		"auto.offset.reset":  "earliest",
		"enable.auto.commit": false,
	})
	if err != nil {
		producer.Close()
		return nil, fmt.Errorf("create notifications kafka consumer: %w", err)
	}
	if err := consumer.SubscribeTopics([]string{topic}, nil); err != nil {
		_ = consumer.Close()
		producer.Close()
		return nil, fmt.Errorf("subscribe notifications kafka topic: %w", err)
	}

	apiBaseURL := strings.TrimSpace(outboundCfg.TelegramAPIBaseURL)
	if apiBaseURL == "" {
		apiBaseURL = "https://api.telegram.org"
	}

	return &KafkaNotificationDelivery{
		enabled:            true,
		topic:              topic,
		dlqTopic:           dlqTopic,
		pollTimeout:        pollTimeout,
		produceTimeout:     produceTimeout,
		consumer:           consumer,
		producer:           producer,
		maxAttempts:        maxAttempts,
		baseBackoff:        baseBackoff,
		maxBackoff:         maxBackoff,
		settings:           deps.Settings,
		bots:               deps.Bots,
		catalog:            deps.Catalog,
		outbound:           deps.Outbound,
		telegramAPIBaseURL: apiBaseURL,
	}, nil
}

func (q *KafkaNotificationDelivery) Enabled() bool {
	return q != nil && q.enabled
}

func (q *KafkaNotificationDelivery) Enqueue(ctx context.Context, event NotificationDeliveryEvent) error {
	ctx, span, startedAt := tracing.StartModuleOperation(ctx, "notifications", "enqueue")
	var err error
	defer func() {
		tracing.FinishModuleOperation(span, startedAt, "notifications", "enqueue", err)
	}()

	if !q.Enabled() {
		err = errKafkaNotificationsQueueDisabled
		return err
	}
	if strings.TrimSpace(event.EventID) == "" {
		err = errKafkaNotificationsEventIDRequired
		return err
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal notification event: %w", err)
	}
	headers := []kafka.Header{{Key: kafkaRetryAttemptHeader, Value: []byte("0")}}
	if err := q.produceMessage(ctx, q.topic, []byte(event.EventID), payload, headers); err != nil {
		return fmt.Errorf("enqueue notification event to kafka: %w", err)
	}
	return nil
}

func (q *KafkaNotificationDelivery) Start(ctx context.Context) {
	if !q.Enabled() {
		return
	}
	q.startOnce.Do(func() {
		go q.consumeLoop(ctx)
	})
}

func (q *KafkaNotificationDelivery) Close() error {
	if q == nil {
		return nil
	}
	if q.consumer != nil {
		if err := q.consumer.Close(); err != nil {
			return err
		}
	}
	if q.producer != nil {
		timeoutMS := int((2 * time.Second) / time.Millisecond)
		q.producer.Flush(timeoutMS)
		q.producer.Close()
	}
	return nil
}

func (q *KafkaNotificationDelivery) consumeLoop(ctx context.Context) {
	pollTimeoutMS := int(q.pollTimeout / time.Millisecond)
	if pollTimeoutMS <= 0 {
		pollTimeoutMS = 1000
	}
	topicUnavailableLogged := false
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		event := q.consumer.Poll(pollTimeoutMS)
		if event == nil {
			continue
		}

		switch typed := event.(type) {
		case *kafka.Message:
			topicUnavailableLogged = false
			notificationEvent, ok := parseNotificationDeliveryMessage(typed)
			if !ok {
				metrics.ObserveModuleOperation("notifications", "parse_event", "error", 0)
				q.commitConsumedMessage(typed)
				continue
			}
			attempt := kafkaHeaderInt(typed.Headers) + 1

			processErr := q.processNotificationDelivery(notificationEvent)
			if processErr == nil {
				q.commitConsumedMessage(typed)
				continue
			}

			logger.Errorf("notification delivery failed (attempt %d/%d): %v", attempt, q.maxAttempts, processErr)
			if attempt >= q.maxAttempts {
				if err := q.publishToDLQ(ctx, typed, notificationEvent, attempt, processErr); err != nil {
					logger.Errorf("notification delivery DLQ publish failed: %v", err)
				}
				q.commitConsumedMessage(typed)
				continue
			}

			backoff := q.retryBackoff(attempt)
			if !sleepWithContext(ctx, backoff) {
				return
			}
			if err := q.requeueMessage(ctx, typed, attempt); err != nil {
				logger.Errorf("notification delivery requeue failed: %v", err)
				continue
			}
			q.commitConsumedMessage(typed)
		case kafka.Error:
			if typed.IsTimeout() || typed.Code() == kafka.ErrTimedOut {
				continue
			}
			if isNotificationsKafkaTopicUnavailable(typed) {
				if !topicUnavailableLogged {
					logger.Warnf("notifications kafka topic is unavailable, waiting for topic creation: topic=%s err=%v", q.topic, typed)
					topicUnavailableLogged = true
				}
				if !sleepWithContext(ctx, maxDuration(3*q.pollTimeout, 5*time.Second)) {
					return
				}
				continue
			}
			topicUnavailableLogged = false
			metrics.ObserveModuleOperation("notifications", "consumer_poll", "error", 0)
			logger.Errorf("notifications kafka consumer error: %v", typed)
		}
	}
}

func parseNotificationDeliveryMessage(message *kafka.Message) (NotificationDeliveryEvent, bool) {
	if message == nil {
		return NotificationDeliveryEvent{}, false
	}
	var event NotificationDeliveryEvent
	if err := json.Unmarshal(message.Value, &event); err != nil {
		logger.Warnf("drop malformed notification delivery payload: %v", err)
		return NotificationDeliveryEvent{}, false
	}
	if strings.TrimSpace(event.EventID) == "" {
		logger.Warnf("drop notification event payload without event_id")
		return NotificationDeliveryEvent{}, false
	}
	return event, true
}

func (q *KafkaNotificationDelivery) processNotificationDelivery(event NotificationDeliveryEvent) error {
	ctx, span, startedAt := tracing.StartModuleOperation(context.Background(), "notifications", "process_delivery")
	var err error
	defer func() {
		tracing.FinishModuleOperation(span, startedAt, "notifications", "process_delivery", err)
	}()

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	tenantID, err := uuid.Parse(strings.TrimSpace(event.TenantID))
	if err != nil {
		logger.Warnf("drop notification delivery with invalid tenant id (event_id=%s): %v", event.EventID, err)
		return nil
	}
	userID, err := uuid.Parse(strings.TrimSpace(event.UserID))
	if err != nil {
		logger.Warnf("drop notification delivery with invalid user id (event_id=%s): %v", event.EventID, err)
		return nil
	}

	settings, err := q.settings.GetByUser(ctx, tenantID, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	if !settings.DeliveryEnabled {
		return nil
	}
	channel := strings.ToLower(strings.TrimSpace(settings.DeliveryChannel))
	switch channel {
	case "telegram":
		return q.processTelegramDelivery(ctx, tenantID, *settings, event)
	case "email":
		return q.processEmailDelivery(ctx, tenantID, *settings, event)
	case "time":
		return q.processTimeDelivery(ctx, tenantID, *settings, event)
	default:
		return nil
	}
}

func (q *KafkaNotificationDelivery) processTelegramDelivery(ctx context.Context, tenantID uuid.UUID, settings models.UserNotificationSettings, event NotificationDeliveryEvent) error {
	if settings.TelegramBotID == nil {
		return nil
	}
	bot, err := q.bots.GetByID(ctx, tenantID, *settings.TelegramBotID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	if !bot.Enabled {
		return nil
	}

	recipient := strings.TrimSpace(settings.TelegramChatID)
	if recipient == "" {
		recipient = strings.TrimSpace(settings.TelegramUsername)
	}
	if recipient == "" {
		return nil
	}

	connector := models.CatalogItem{
		Data: map[string]any{
			"channel": "telegram",
			"config": map[string]any{
				"botToken":   bot.BotToken,
				"apiBaseURL": q.telegramAPIBaseURL,
			},
		},
	}
	messageText := composeTelegramNotificationText(event)
	metadata := map[string]any{
		"notification_id": event.NotificationID,
		"recipient":       recipient,
	}
	if recipient != "" {
		metadata["chat_id"] = recipient
	}
	_, err = q.outbound.Send(ctx, connector, outbound.SendRequest{
		Author:         "IncidentHub",
		Message:        messageText,
		ConversationID: recipient,
		Metadata:       metadata,
	})
	if err != nil {
		return fmt.Errorf("send telegram notification: %w", err)
	}
	return nil
}

func (q *KafkaNotificationDelivery) processEmailDelivery(ctx context.Context, tenantID uuid.UUID, settings models.UserNotificationSettings, event NotificationDeliveryEvent) error {
	recipient := strings.TrimSpace(settings.NotificationEmail)
	if recipient == "" {
		return nil
	}
	connector, err := q.resolveTenantConnectorByChannel(ctx, tenantID, "email")
	if err != nil {
		return err
	}
	if connector == nil {
		return nil
	}

	subject := composeNotificationSubject(event)
	_, err = q.outbound.Send(ctx, *connector, outbound.SendRequest{
		Author:         "IncidentHub",
		Message:        composeChannelNotificationText(event),
		ConversationID: recipient,
		Metadata: map[string]any{
			"notification_id": event.NotificationID,
			"to":              []string{recipient},
			"recipient":       recipient,
			"subject":         subject,
		},
	})
	if err != nil {
		return fmt.Errorf("send email notification: %w", err)
	}
	return nil
}

func (q *KafkaNotificationDelivery) processTimeDelivery(ctx context.Context, tenantID uuid.UUID, settings models.UserNotificationSettings, event NotificationDeliveryEvent) error {
	recipient := strings.TrimSpace(settings.TimeRecipient)
	if recipient == "" {
		return nil
	}
	connector, err := q.resolveTenantConnectorByChannel(ctx, tenantID, "time")
	if err != nil {
		return err
	}
	if connector == nil {
		return nil
	}

	_, err = q.outbound.Send(ctx, *connector, outbound.SendRequest{
		Author:         "IncidentHub",
		Message:        composeChannelNotificationText(event),
		ConversationID: recipient,
		Metadata: map[string]any{
			"notification_id": event.NotificationID,
			"recipient":       recipient,
			"target":          recipient,
		},
	})
	if err != nil {
		return fmt.Errorf("send time notification: %w", err)
	}
	return nil
}

func (q *KafkaNotificationDelivery) resolveTenantConnectorByChannel(ctx context.Context, tenantID uuid.UUID, channel string) (*models.CatalogItem, error) {
	if q.catalog == nil {
		return nil, nil
	}
	normalizedChannel := strings.ToLower(strings.TrimSpace(channel))
	if normalizedChannel == "" {
		return nil, nil
	}
	findIn := func(kind string) (*models.CatalogItem, error) {
		items, err := q.catalog.List(ctx, repository.CatalogListParams{
			Kind:     kind,
			TenantID: &tenantID,
			Limit:    500,
		})
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if strings.ToLower(strings.TrimSpace(fmt.Sprint(item.Data["direction"]))) == "inbound" {
				continue
			}
			if enabled, ok := item.Data["enabled"].(bool); ok && !enabled {
				continue
			}
			itemChannel := strings.ToLower(strings.TrimSpace(fmt.Sprint(item.Data["channel"])))
			if itemChannel == "" {
				itemChannel = strings.ToLower(strings.TrimSpace(fmt.Sprint(item.Data["type"])))
			}
			if itemChannel != normalizedChannel {
				continue
			}
			copied := item
			return &copied, nil
		}
		return nil, nil
	}

	connector, err := findIn("outbound_connectors")
	if err != nil {
		return nil, err
	}
	if connector != nil {
		return connector, nil
	}
	return findIn("connectors")
}

func composeTelegramNotificationText(event NotificationDeliveryEvent) string {
	title := strings.TrimSpace(event.Title)
	message := strings.TrimSpace(event.Message)
	eventType := strings.TrimSpace(strings.ToUpper(event.Type))

	if title == "" && message == "" {
		return "New IncidentHub notification"
	}
	if title == "" {
		return message
	}
	if message == "" {
		if eventType == "" {
			return "🔔 " + title
		}
		return fmt.Sprintf("🔔 [%s] %s", eventType, title)
	}
	if eventType == "" {
		return fmt.Sprintf("🔔 %s\n\n%s", title, message)
	}
	return fmt.Sprintf("🔔 [%s] %s\n\n%s", eventType, title, message)
}

func composeChannelNotificationText(event NotificationDeliveryEvent) string {
	title := strings.TrimSpace(event.Title)
	message := strings.TrimSpace(event.Message)
	eventType := strings.TrimSpace(strings.ToUpper(event.Type))
	if title == "" && message == "" {
		return "New IncidentHub notification"
	}
	if title == "" {
		return message
	}
	if message == "" {
		if eventType == "" {
			return title
		}
		return fmt.Sprintf("[%s] %s", eventType, title)
	}
	if eventType == "" {
		return title + "\n\n" + message
	}
	return fmt.Sprintf("[%s] %s\n\n%s", eventType, title, message)
}

func composeNotificationSubject(event NotificationDeliveryEvent) string {
	title := strings.TrimSpace(event.Title)
	eventType := strings.TrimSpace(strings.ToUpper(event.Type))
	if title == "" {
		if eventType == "" {
			return "IncidentHub notification"
		}
		return fmt.Sprintf("[%s] IncidentHub notification", eventType)
	}
	if eventType == "" {
		return title
	}
	return fmt.Sprintf("[%s] %s", eventType, title)
}

func (q *KafkaNotificationDelivery) produceMessage(ctx context.Context, topic string, key, value []byte, headers []kafka.Header) error {
	return produceKafkaMessage(ctx, q.producer, topic, key, value, headers, q.produceTimeout)
}

func (q *KafkaNotificationDelivery) requeueMessage(ctx context.Context, original *kafka.Message, attempt int) error {
	if original == nil {
		return errKafkaNotificationsRequeueMessageNil
	}
	headers := withRetryHeader(original.Headers, attempt)
	key := append([]byte(nil), original.Key...)
	value := append([]byte(nil), original.Value...)
	return q.produceMessage(ctx, q.topic, key, value, headers)
}

func (q *KafkaNotificationDelivery) retryBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	backoff := q.baseBackoff
	for i := 1; i < attempt; i++ {
		backoff *= 2
		if backoff >= q.maxBackoff {
			backoff = q.maxBackoff
			break
		}
	}
	if backoff <= 0 {
		backoff = q.baseBackoff
	}
	return backoff
}

func (q *KafkaNotificationDelivery) publishToDLQ(ctx context.Context, original *kafka.Message, event NotificationDeliveryEvent, attempt int, deliveryErr error) error {
	key := []byte(event.EventID)
	if strings.TrimSpace(event.EventID) == "" {
		key = nil
	}
	return publishKafkaDLQMessage(
		ctx,
		q.producer,
		q.produceTimeout,
		q.dlqTopic,
		q.topic,
		original,
		key,
		"event",
		event,
		attempt,
		deliveryErr,
		"marshal notification dlq payload",
	)
}

func (q *KafkaNotificationDelivery) commitConsumedMessage(message *kafka.Message) {
	if message == nil {
		return
	}
	if _, err := q.consumer.CommitMessage(message); err != nil {
		logger.Errorf("notification delivery commit failed: %v", err)
	}
}

func isNotificationsKafkaTopicUnavailable(err kafka.Error) bool {
	code := err.Code()
	return code == kafka.ErrUnknownTopic || code == kafka.ErrUnknownTopicOrPart
}

func maxDuration(left, right time.Duration) time.Duration {
	if left >= right {
		return left
	}
	return right
}
