package outbound

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/models"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/redis/go-redis/v9"
)

func TestOutboundConnectorDriversIntegration(t *testing.T) {
	ctx := context.Background()
	svc := NewService(config.OutboundConnectorsConfig{})

	t.Run("mock_driver", func(t *testing.T) {
		connector := models.CatalogItem{
			Data: map[string]any{
				"channel": "mock",
				"config":  map[string]any{},
			},
		}

		sendResp, err := svc.Send(ctx, connector, SendRequest{
			ConversationID: "mock-conv-1",
			Message:        "integration outbound mock",
		})
		if err != nil {
			t.Fatalf("mock send failed: %v", err)
		}
		if !strings.Contains(sendResp.Reply, "integration outbound mock") {
			t.Fatalf("unexpected mock send reply: %q", sendResp.Reply)
		}

		pollResp, err := svc.Poll(ctx, connector, PollRequest{ConversationID: "mock-conv-1"})
		if err != nil {
			t.Fatalf("mock poll failed: %v", err)
		}
		if len(pollResp.Messages) == 0 {
			t.Fatal("expected at least one mock poll message")
		}
	})

	t.Run("webhook_driver", func(t *testing.T) {
		sendRequests := 0
		pollRequests := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/send":
				sendRequests++
				if r.Method != http.MethodPost {
					http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
					return
				}
				if r.Header.Get("X-Integration-Key") != "outbound-suite" {
					http.Error(w, "missing integration header", http.StatusBadRequest)
					return
				}
				var payload map[string]any
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					http.Error(w, "bad payload", http.StatusBadRequest)
					return
				}
				if strings.TrimSpace(fmt.Sprint(payload["message"])) == "" {
					http.Error(w, "missing message", http.StatusBadRequest)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{
					"reply":           "accepted",
					"conversation_id": "webhook-conv-1",
					"cursor":          "cursor-1",
				})
			case "/poll":
				pollRequests++
				if r.Method != http.MethodGet {
					http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
					return
				}
				if r.Header.Get("X-Integration-Key") != "outbound-suite" {
					http.Error(w, "missing integration header", http.StatusBadRequest)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{
					"messages": []map[string]any{
						{
							"id":        "wb-msg-1",
							"author":    "webhook-user",
							"content":   "webhook inbound message",
							"timestamp": "2026-02-20T00:00:00Z",
						},
					},
					"next_cursor": "cursor-2",
				})
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		connector := models.CatalogItem{
			Data: map[string]any{
				"channel": "webhook",
				"config": map[string]any{
					"url":      server.URL + "/send",
					"poll_url": server.URL + "/poll",
					"headers": map[string]any{
						"X-Integration-Key": "outbound-suite",
					},
				},
			},
		}

		sendResp, err := svc.Send(ctx, connector, SendRequest{
			ConversationID: "webhook-conv-1",
			Message:        "trigger webhook delivery",
		})
		if err != nil {
			t.Fatalf("webhook send failed: %v", err)
		}
		if sendResp.ConversationID != "webhook-conv-1" {
			t.Fatalf("unexpected webhook conversation id: %q", sendResp.ConversationID)
		}

		pollResp, err := svc.Poll(ctx, connector, PollRequest{ConversationID: "webhook-conv-1"})
		if err != nil {
			t.Fatalf("webhook poll failed: %v", err)
		}
		if len(pollResp.Messages) != 1 {
			t.Fatalf("expected 1 webhook poll message, got %d", len(pollResp.Messages))
		}
		if pollResp.Messages[0].Content != "webhook inbound message" {
			t.Fatalf("unexpected webhook poll content: %q", pollResp.Messages[0].Content)
		}
		if sendRequests != 1 || pollRequests != 1 {
			t.Fatalf("unexpected webhook request counters: send=%d poll=%d", sendRequests, pollRequests)
		}
	})

	t.Run("email_driver", func(t *testing.T) {
		sendRequests := 0
		pollRequests := 0
		lastSubject := ""
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/mail/send":
				sendRequests++
				var payload map[string]any
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					http.Error(w, "bad payload", http.StatusBadRequest)
					return
				}
				if strings.TrimSpace(fmt.Sprint(payload["subject"])) == "" {
					http.Error(w, "missing subject", http.StatusBadRequest)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{
					"reply":           "mail accepted",
					"conversation_id": "mail-conv-1",
					"cursor":          "mail-cursor-1",
				})
			case "/mail/poll":
				pollRequests++
				lastSubject = strings.TrimSpace(r.URL.Query().Get("subject"))
				_ = json.NewEncoder(w).Encode(map[string]any{
					"messages": []map[string]any{
						{
							"id":        "mail-1",
							"author":    "target@example.com",
							"subject":   "Incident update",
							"content":   "Email inbound message",
							"timestamp": "2026-02-20T00:00:00Z",
						},
					},
					"next_cursor": "mail-cursor-2",
				})
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		connector := models.CatalogItem{
			Data: map[string]any{
				"channel": "email",
				"config": map[string]any{
					"send_url": server.URL + "/mail/send",
					"poll_url": server.URL + "/mail/poll",
				},
			},
		}

		sendResp, err := svc.Send(ctx, connector, SendRequest{
			ConversationID: "mail-conv-1",
			Message:        "Email outbound integration message",
			Metadata: map[string]any{
				"to":      []string{"target@example.com"},
				"subject": "Incident update",
			},
		})
		if err != nil {
			t.Fatalf("email send failed: %v", err)
		}
		if sendResp.ConversationID != "mail-conv-1" {
			t.Fatalf("unexpected email conversation id: %q", sendResp.ConversationID)
		}

		pollResp, err := svc.Poll(ctx, connector, PollRequest{
			ConversationID: "mail-conv-1",
			Metadata: map[string]any{
				"subject": "Incident update",
			},
		})
		if err != nil {
			t.Fatalf("email poll failed: %v", err)
		}
		if len(pollResp.Messages) != 1 {
			t.Fatalf("expected 1 email poll message, got %d", len(pollResp.Messages))
		}
		if pollResp.Messages[0].Content != "Email inbound message" {
			t.Fatalf("unexpected email poll content: %q", pollResp.Messages[0].Content)
		}
		if lastSubject != "Incident update" {
			t.Fatalf("unexpected email poll subject filter: %q", lastSubject)
		}
		if sendRequests != 1 || pollRequests != 1 {
			t.Fatalf("unexpected email request counters: send=%d poll=%d", sendRequests, pollRequests)
		}
	})

	t.Run("time_driver", func(t *testing.T) {
		sendRequests := 0
		pollRequests := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/time/send":
				sendRequests++
				var payload map[string]any
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					http.Error(w, "bad payload", http.StatusBadRequest)
					return
				}
				if strings.TrimSpace(fmt.Sprint(payload["recipient"])) == "" {
					http.Error(w, "missing recipient", http.StatusBadRequest)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{
					"reply":           "time accepted",
					"conversation_id": "time-conv-1",
					"cursor":          "time-cursor-1",
				})
			case "/time/poll":
				pollRequests++
				_ = json.NewEncoder(w).Encode(map[string]any{
					"messages": []map[string]any{
						{
							"id":        "time-1",
							"author":    "time-user",
							"content":   "Time inbound message",
							"timestamp": "2026-02-20T00:00:00Z",
						},
					},
					"next_cursor": "time-cursor-2",
				})
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		connector := models.CatalogItem{
			Data: map[string]any{
				"channel": "time",
				"config": map[string]any{
					"send_url": server.URL + "/time/send",
					"poll_url": server.URL + "/time/poll",
				},
			},
		}

		sendResp, err := svc.Send(ctx, connector, SendRequest{
			ConversationID: "time-user-1",
			Message:        "Time outbound integration message",
			Metadata: map[string]any{
				"recipient": "time-user-1",
			},
		})
		if err != nil {
			t.Fatalf("time send failed: %v", err)
		}
		if sendResp.ConversationID != "time-conv-1" {
			t.Fatalf("unexpected time conversation id: %q", sendResp.ConversationID)
		}

		pollResp, err := svc.Poll(ctx, connector, PollRequest{ConversationID: "time-conv-1"})
		if err != nil {
			t.Fatalf("time poll failed: %v", err)
		}
		if len(pollResp.Messages) != 1 {
			t.Fatalf("expected 1 time poll message, got %d", len(pollResp.Messages))
		}
		if pollResp.Messages[0].Content != "Time inbound message" {
			t.Fatalf("unexpected time poll content: %q", pollResp.Messages[0].Content)
		}
		if sendRequests != 1 || pollRequests != 1 {
			t.Fatalf("unexpected time request counters: send=%d poll=%d", sendRequests, pollRequests)
		}
	})

	t.Run("telegram_driver", func(t *testing.T) {
		lastSendChatID := ""
		lastSendText := ""
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/sendMessage") {
				var payload map[string]any
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					http.Error(w, "bad payload", http.StatusBadRequest)
					return
				}
				lastSendChatID = strings.TrimSpace(fmt.Sprint(payload["chat_id"]))
				lastSendText = strings.TrimSpace(fmt.Sprint(payload["text"]))
				_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "description": "sent"})
				return
			}
			if strings.Contains(r.URL.Path, "/getUpdates") {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"ok": true,
					"result": []map[string]any{
						{
							"update_id": 3001,
							"message": map[string]any{
								"message_id": 11,
								"date":       1700000000,
								"text":       "telegram inbound message",
								"from": map[string]any{
									"username": "telegram-user",
								},
								"chat": map[string]any{
									"id": 777001,
								},
							},
						},
					},
				})
				return
			}
			http.NotFound(w, r)
		}))
		defer server.Close()

		connector := models.CatalogItem{
			Data: map[string]any{
				"channel": "telegram",
				"config": map[string]any{
					"botToken":   "integration-telegram-token",
					"apiBaseURL": server.URL,
				},
			},
		}

		sendResp, err := svc.Send(ctx, connector, SendRequest{
			ConversationID: "777001",
			Message:        "telegram outbound integration",
			Metadata: map[string]any{
				"chat_id": "777001",
			},
		})
		if err != nil {
			t.Fatalf("telegram send failed: %v", err)
		}
		if sendResp.ConversationID != "777001" {
			t.Fatalf("unexpected telegram conversation id: %q", sendResp.ConversationID)
		}
		if lastSendChatID != "777001" {
			t.Fatalf("unexpected telegram chat id sent: %q", lastSendChatID)
		}
		if !strings.Contains(lastSendText, "integration") {
			t.Fatalf("unexpected telegram text sent: %q", lastSendText)
		}

		pollResp, err := svc.Poll(ctx, connector, PollRequest{
			ConversationID: "777001",
		})
		if err != nil {
			t.Fatalf("telegram poll failed: %v", err)
		}
		if len(pollResp.Messages) != 1 {
			t.Fatalf("expected 1 telegram poll message, got %d", len(pollResp.Messages))
		}
		if pollResp.Messages[0].Content != "telegram inbound message" {
			t.Fatalf("unexpected telegram poll message content: %q", pollResp.Messages[0].Content)
		}
	})

	t.Run("redis_driver", func(t *testing.T) {
		redisServer, err := miniredis.Run()
		if err != nil {
			t.Fatalf("start miniredis: %v", err)
		}
		defer redisServer.Close()

		seedClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
		defer func() { _ = seedClient.Close() }()

		connector := models.CatalogItem{
			Data: map[string]any{
				"channel": "redis",
				"config": map[string]any{
					"addr":     redisServer.Addr(),
					"send_key": "integration.outbound.redis.send",
					"poll_key": "integration.outbound.redis.poll",
				},
			},
		}

		_, err = svc.Send(ctx, connector, SendRequest{
			ConversationID: "redis-conv-1",
			Author:         "soc",
			Message:        "redis outbound integration message",
		})
		if err != nil {
			t.Fatalf("redis send failed: %v", err)
		}

		outboundQueueLen, err := seedClient.LLen(ctx, "integration.outbound.redis.send").Result()
		if err != nil {
			t.Fatalf("read outbound redis queue len: %v", err)
		}
		if outboundQueueLen != 1 {
			t.Fatalf("expected 1 outbound redis message, got %d", outboundQueueLen)
		}

		if pushErr := seedClient.RPush(ctx, "integration.outbound.redis.poll",
			`{"id":"redis-msg-1","author":"external-user","message":"redis inbound integration","timestamp":"2026-02-20T00:00:00Z"}`,
		).Err(); pushErr != nil {
			t.Fatalf("seed redis poll queue: %v", pushErr)
		}

		pollResp, err := svc.Poll(ctx, connector, PollRequest{ConversationID: "redis-conv-1"})
		if err != nil {
			t.Fatalf("redis poll failed: %v", err)
		}
		if len(pollResp.Messages) != 1 {
			t.Fatalf("expected 1 redis poll message, got %d", len(pollResp.Messages))
		}
		if pollResp.Messages[0].Content != "redis inbound integration" {
			t.Fatalf("unexpected redis poll message: %q", pollResp.Messages[0].Content)
		}
	})

	t.Run("kafka_driver", func(t *testing.T) {
		brokers := outboundIntegrationKafkaBrokers()
		if len(brokers) == 0 {
			t.Skip("no kafka brokers configured for integration test")
		}

		sendTopic := fmt.Sprintf("incidenthub.outbound.send.integration.%d", time.Now().UnixNano())
		pollTopic := fmt.Sprintf("incidenthub.outbound.poll.integration.%d", time.Now().UnixNano())

		opCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
		defer cancel()

		if err := ensureOutboundKafkaTopic(opCtx, brokers, sendTopic); err != nil {
			if shouldSkipOutboundKafka(err) {
				t.Skipf("kafka is unavailable: %v", err)
			}
			t.Fatalf("create kafka send topic: %v", err)
		}
		if err := ensureOutboundKafkaTopic(opCtx, brokers, pollTopic); err != nil {
			if shouldSkipOutboundKafka(err) {
				t.Skipf("kafka is unavailable: %v", err)
			}
			t.Fatalf("create kafka poll topic: %v", err)
		}

		sendConnector := models.CatalogItem{
			Data: map[string]any{
				"channel": "kafka",
				"config": map[string]any{
					"brokers":     brokers,
					"topic":       sendTopic,
					"message_key": "kafka-conv-1",
				},
			},
		}

		sendResp, err := svc.Send(opCtx, sendConnector, SendRequest{
			ConversationID: "kafka-conv-1",
			Author:         "soc",
			Message:        "kafka outbound integration payload",
		})
		if err != nil {
			t.Fatalf("kafka send failed: %v", err)
		}
		if sendResp.ConversationID != "kafka-conv-1" {
			t.Fatalf("unexpected kafka send conversation id: %q", sendResp.ConversationID)
		}

		consumedValue, err := consumeOutboundKafkaMessage(opCtx, brokers, sendTopic, fmt.Sprintf("ih-outbound-send-%d", time.Now().UnixNano()))
		if err != nil {
			if shouldSkipOutboundKafka(err) {
				t.Skipf("kafka is unavailable during consume: %v", err)
			}
			t.Fatalf("consume outbound kafka message: %v", err)
		}
		var consumedPayload map[string]any
		if unmarshalErr := json.Unmarshal(consumedValue, &consumedPayload); unmarshalErr != nil {
			t.Fatalf("decode consumed kafka payload: %v", unmarshalErr)
		}
		if got := strings.TrimSpace(fmt.Sprint(consumedPayload["message"])); got != "kafka outbound integration payload" {
			t.Fatalf("unexpected consumed kafka message: %q", got)
		}

		payload := []byte(`{
			"author": "external-user",
			"message": "kafka inbound integration",
			"external_id": "kafka-ext-1",
			"timestamp": "2026-02-20T00:00:00Z"
		}`)
		if produceErr := produceOutboundKafkaMessage(opCtx, brokers, pollTopic, payload); produceErr != nil {
			if shouldSkipOutboundKafka(produceErr) {
				t.Skipf("kafka is unavailable during produce: %v", produceErr)
			}
			t.Fatalf("produce kafka poll fixture: %v", produceErr)
		}

		pollConnector := models.CatalogItem{
			Data: map[string]any{
				"channel": "kafka",
				"config": map[string]any{
					"brokers":         brokers,
					"poll_topic":      pollTopic,
					"start_offset":    "earliest",
					"session_timeout": "10s",
					"max_messages":    5,
					"poll_max_items":  5,
				},
			},
		}

		pollResp, err := svc.Poll(opCtx, pollConnector, PollRequest{
			ConversationID: "kafka-conv-poll",
		})
		if err != nil {
			t.Fatalf("kafka poll failed: %v", err)
		}
		if len(pollResp.Messages) == 0 {
			t.Fatal("expected at least one kafka poll message")
		}
		if pollResp.Messages[0].Content != "kafka inbound integration" {
			t.Fatalf("unexpected kafka poll message content: %q", pollResp.Messages[0].Content)
		}
	})
}

func outboundIntegrationKafkaBrokers() []string {
	raw := strings.TrimSpace(os.Getenv("INCIDENTHUB_TEST_KAFKA_BROKERS"))
	if raw == "" {
		raw = "localhost:29092"
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func ensureOutboundKafkaTopic(ctx context.Context, brokers []string, topic string) error {
	admin, err := kafka.NewAdminClient(&kafka.ConfigMap{
		"bootstrap.servers": strings.Join(brokers, ","),
	})
	if err != nil {
		return err
	}
	defer admin.Close()

	results, err := admin.CreateTopics(ctx, []kafka.TopicSpecification{
		{
			Topic:             topic,
			NumPartitions:     1,
			ReplicationFactor: 1,
		},
	})
	if err != nil {
		return err
	}
	for _, result := range results {
		if result.Error.Code() == kafka.ErrNoError || result.Error.Code() == kafka.ErrTopicAlreadyExists {
			continue
		}
		return result.Error
	}
	return nil
}

func produceOutboundKafkaMessage(ctx context.Context, brokers []string, topic string, value []byte) error {
	producer, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers":  strings.Join(brokers, ","),
		"message.timeout.ms": 8000,
	})
	if err != nil {
		return err
	}
	defer producer.Close()

	delivery := make(chan kafka.Event, 1)
	if err := producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          value,
		Timestamp:      time.Now().UTC(),
	}, delivery); err != nil {
		return err
	}

	select {
	case event := <-delivery:
		delivered, ok := event.(*kafka.Message)
		if !ok {
			return fmt.Errorf("unexpected kafka delivery event type %T", event)
		}
		if delivered.TopicPartition.Error != nil {
			return delivered.TopicPartition.Error
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func consumeOutboundKafkaMessage(ctx context.Context, brokers []string, topic, groupID string) ([]byte, error) {
	consumer, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":  strings.Join(brokers, ","),
		"group.id":           groupID,
		"auto.offset.reset":  "earliest",
		"enable.auto.commit": false,
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = consumer.Close() }()

	if err := consumer.SubscribeTopics([]string{topic}, nil); err != nil {
		return nil, err
	}

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		event := consumer.Poll(500)
		if event == nil {
			continue
		}
		switch typed := event.(type) {
		case *kafka.Message:
			valueCopy := make([]byte, len(typed.Value))
			copy(valueCopy, typed.Value)
			return valueCopy, nil
		case kafka.Error:
			if typed.IsTimeout() || typed.Code() == kafka.ErrTimedOut {
				continue
			}
			return nil, typed
		}
	}
}

func shouldSkipOutboundKafka(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	text := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(text, "connection refused") ||
		strings.Contains(text, "1/1 brokers are down") ||
		strings.Contains(text, "no such host") ||
		strings.Contains(text, "dial tcp") ||
		strings.Contains(text, "timed out") ||
		strings.Contains(text, "timeout")
}
