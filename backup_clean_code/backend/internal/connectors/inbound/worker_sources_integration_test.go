package inbound

import (
	"bytes"
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
	"incidenthub/backend/internal/testutil"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/v9"
)

func TestInboundConnectorSourcesIntegration(t *testing.T) {
	t.Run("http_source", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{
					{
						"id":          "http-int-1",
						"title":       "HTTP integration alert",
						"description": "from http source",
						"source":      "http",
						"severity":    "high",
					},
				},
			})
		}))
		defer server.Close()

		cfg := parseInboundIntegrationConfig(t, "HTTP integration source", map[string]any{
			"source_type": "http",
			"url":         server.URL,
			"method":      http.MethodGet,
			"array_path":  "items",
		})

		worker := &Worker{
			cfg:        config.InboundConnectorsConfig{HTTPTimeout: 3 * time.Second},
			httpClient: server.Client(),
		}
		records, err := worker.fetchHTTPRecords(context.Background(), cfg)
		if err != nil {
			t.Fatalf("fetch HTTP records: %v", err)
		}
		if len(records) != 1 {
			t.Fatalf("expected 1 HTTP record, got %d", len(records))
		}
		if got := extractString(records[0], "id"); got != "http-int-1" {
			t.Fatalf("unexpected HTTP record id: %q", got)
		}
	})

	t.Run("redis_source", func(t *testing.T) {
		redisAddr, closeRedis := startInboundIntegrationRedis(t)
		defer closeRedis()

		key := "incidenthub.inbound.integration.redis"
		seedClient := redis.NewClient(&redis.Options{Addr: redisAddr})
		defer func() { _ = seedClient.Close() }()

		if err := seedClient.RPush(context.Background(), key,
			`{"id":"redis-int-1","title":"Redis integration alert","description":"from redis source"}`,
		).Err(); err != nil {
			t.Fatalf("seed redis payload: %v", err)
		}

		cfg := parseInboundIntegrationConfig(t, "Redis integration source", map[string]any{
			"source_type":  "redis",
			"addr":         redisAddr,
			"key":          key,
			"max_messages": 10,
			"pop_from":     "left",
		})

		worker := &Worker{}
		records, err := worker.fetchRedisRecords(context.Background(), cfg)
		if err != nil {
			t.Fatalf("fetch Redis records: %v", err)
		}
		if len(records) != 1 {
			t.Fatalf("expected 1 Redis record, got %d", len(records))
		}
		if got := extractString(records[0], "id"); got != "redis-int-1" {
			t.Fatalf("unexpected Redis record id: %q", got)
		}
	})

	t.Run("sql_source", func(t *testing.T) {
		pool := testutil.OpenTestPool(t)
		tableName := fmt.Sprintf("inbound_sql_integration_%d", time.Now().UnixNano())

		if _, err := pool.Exec(context.Background(), fmt.Sprintf(`
			CREATE TABLE %s (
				id text primary key,
				title text not null,
				description text not null
			)
		`, tableName)); err != nil {
			t.Fatalf("create SQL integration table: %v", err)
		}
		t.Cleanup(func() {
			_, _ = pool.Exec(context.Background(), fmt.Sprintf("DROP TABLE IF EXISTS %s", tableName))
		})

		if _, err := pool.Exec(context.Background(), fmt.Sprintf(`
			INSERT INTO %s(id, title, description) VALUES
				('sql-int-1', 'SQL integration alert 1', 'from sql source one'),
				('sql-int-2', 'SQL integration alert 2', 'from sql source two')
		`, tableName)); err != nil {
			t.Fatalf("seed SQL integration table: %v", err)
		}

		cfg := parseInboundIntegrationConfig(t, "SQL integration source", map[string]any{
			"source_type": "sql",
			"driver":      "pgx",
			"dsn":         testutil.TestDatabaseURL(),
			"query":       fmt.Sprintf("SELECT id, title, description FROM %s ORDER BY id", tableName),
			"max_rows":    10,
		})

		worker := &Worker{}
		records, err := worker.fetchSQLRecords(context.Background(), cfg)
		if err != nil {
			t.Fatalf("fetch SQL records: %v", err)
		}
		if len(records) != 2 {
			t.Fatalf("expected 2 SQL records, got %d", len(records))
		}
		if got := extractString(records[0], "id"); got != "sql-int-1" {
			t.Fatalf("unexpected first SQL id: %q", got)
		}
	})

	t.Run("kafka_source", func(t *testing.T) {
		brokers := inboundIntegrationKafkaBrokers()
		if len(brokers) == 0 {
			t.Skip("no kafka brokers configured for integration")
		}

		topic := fmt.Sprintf("incidenthub.inbound.integration.%d", time.Now().UnixNano())
		opCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		if err := ensureInboundKafkaTopic(opCtx, brokers, topic); err != nil {
			if shouldSkipInboundKafka(err) {
				t.Skipf("kafka unavailable: %v", err)
			}
			t.Fatalf("create inbound kafka topic: %v", err)
		}
		if err := produceInboundKafkaMessage(opCtx, brokers, topic, []byte(`{"items":[{"id":"kafka-int-1","title":"Kafka integration alert","description":"from kafka source"}]}`)); err != nil {
			if shouldSkipInboundKafka(err) {
				t.Skipf("kafka unavailable: %v", err)
			}
			t.Fatalf("produce inbound kafka payload: %v", err)
		}

		cfg := parseInboundIntegrationConfig(t, "Kafka integration source", map[string]any{
			"source_type":  "kafka",
			"brokers":      brokers,
			"topic":        topic,
			"start_offset": "earliest",
			"max_messages": 10,
			"array_path":   "items",
			"commit":       false,
			"client_id":    "incidenthub-inbound-integration",
		})

		worker := &Worker{}
		fetchCtx, fetchCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer fetchCancel()

		records, err := worker.fetchKafkaRecords(fetchCtx, cfg)
		if err != nil {
			t.Fatalf("fetch Kafka records: %v", err)
		}
		if len(records) == 0 {
			t.Fatal("expected at least one Kafka record")
		}
		if got := extractString(records[0], "id"); got != "kafka-int-1" {
			t.Fatalf("unexpected Kafka record id: %q", got)
		}
	})

	t.Run("s3_source", func(t *testing.T) {
		rawEndpoint := strings.TrimSpace(os.Getenv("INCIDENTHUB_TEST_S3_ENDPOINT"))
		if rawEndpoint == "" {
			rawEndpoint = "http://localhost:9000"
		}
		accessKey := strings.TrimSpace(os.Getenv("INCIDENTHUB_TEST_S3_ACCESS_KEY"))
		if accessKey == "" {
			accessKey = strings.TrimSpace(os.Getenv("S3_ACCESS_KEY"))
		}
		if accessKey == "" {
			accessKey = "incidenthub"
		}
		secretKey := strings.TrimSpace(os.Getenv("INCIDENTHUB_TEST_S3_SECRET_KEY"))
		if secretKey == "" {
			secretKey = strings.TrimSpace(os.Getenv("S3_SECRET_KEY"))
		}
		if secretKey == "" {
			secretKey = "incidenthub123"
		}

		endpointHost, secure, err := normalizeS3Endpoint(rawEndpoint, false)
		if err != nil {
			t.Fatalf("normalize S3 endpoint: %v", err)
		}

		client, err := minio.New(endpointHost, &minio.Options{
			Creds:        credentials.NewStaticV4(accessKey, secretKey, ""),
			Secure:       secure,
			Region:       "us-east-1",
			BucketLookup: minio.BucketLookupPath,
		})
		if err != nil {
			if shouldSkipInboundS3(err) {
				t.Skipf("s3 unavailable: %v", err)
			}
			t.Fatalf("init S3 client: %v", err)
		}

		bucket := fmt.Sprintf("ih-inbound-int-%d", time.Now().UnixNano())
		ctx := context.Background()
		if makeBucketErr := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{Region: "us-east-1"}); makeBucketErr != nil {
			if shouldSkipInboundS3(makeBucketErr) {
				t.Skipf("s3 unavailable: %v", makeBucketErr)
			}
			t.Fatalf("create S3 bucket: %v", makeBucketErr)
		}
		t.Cleanup(func() {
			_ = client.RemoveObject(context.Background(), bucket, "feed/alerts.json", minio.RemoveObjectOptions{})
			_ = client.RemoveBucket(context.Background(), bucket)
		})

		payload := []byte(`{"items":[{"id":"s3-int-1","title":"S3 integration alert","description":"from s3 source"}]}`)
		if _, putErr := client.PutObject(ctx, bucket, "feed/alerts.json", bytes.NewReader(payload), int64(len(payload)), minio.PutObjectOptions{
			ContentType: "application/json",
		}); putErr != nil {
			if shouldSkipInboundS3(putErr) {
				t.Skipf("s3 unavailable: %v", putErr)
			}
			t.Fatalf("upload S3 integration object: %v", putErr)
		}

		cfg := parseInboundIntegrationConfig(t, "S3 integration source", map[string]any{
			"source_type":       "s3",
			"endpoint":          rawEndpoint,
			"bucket":            bucket,
			"region":            "us-east-1",
			"use_ssl":           false,
			"path_style":        true,
			"object_pattern":    "feed/*.json",
			"array_path":        "items",
			"access_key_id":     accessKey,
			"secret_access_key": secretKey,
		})

		worker := &Worker{}
		records, err := worker.fetchS3Records(context.Background(), cfg)
		if err != nil {
			t.Fatalf("fetch S3 records: %v", err)
		}
		if len(records) != 1 {
			t.Fatalf("expected 1 S3 record, got %d", len(records))
		}
		if got := extractString(records[0], "id"); got != "s3-int-1" {
			t.Fatalf("unexpected S3 record id: %q", got)
		}
	})
}

func parseInboundIntegrationConfig(t *testing.T, name string, cfg map[string]any) inboundConnectorConfig {
	t.Helper()
	parsed, err := parseInboundConnectorConfig(map[string]any{
		"data": map[string]any{
			"name":   name,
			"config": cfg,
		},
	})
	if err != nil {
		t.Fatalf("parse inbound integration config for %q: %v", name, err)
	}
	return parsed
}

func startInboundIntegrationRedis(t *testing.T) (addr string, cleanup func()) {
	t.Helper()
	server, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	return server.Addr(), func() { server.Close() }
}

func inboundIntegrationKafkaBrokers() []string {
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

func ensureInboundKafkaTopic(ctx context.Context, brokers []string, topic string) error {
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

func produceInboundKafkaMessage(ctx context.Context, brokers []string, topic string, value []byte) error {
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

func shouldSkipInboundKafka(err error) bool {
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

func shouldSkipInboundS3(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(text, "connection refused") ||
		strings.Contains(text, "no such host") ||
		strings.Contains(text, "dial tcp") ||
		strings.Contains(text, "timed out") ||
		strings.Contains(text, "timeout")
}
