package inbound

import (
	"incidenthub/backend/internal/models"
	"net/http"
	"testing"
)

func TestExtractRecords(t *testing.T) {
	payload := map[string]any{
		"data": map[string]any{
			"items": []any{
				map[string]any{"id": "a1", "title": "One"},
				map[string]any{"id": "a2", "title": "Two"},
			},
		},
	}

	records := extractRecords(payload, "data.items")
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if records[0]["id"] != "a1" {
		t.Fatalf("expected first id a1, got %v", records[0]["id"])
	}
}

func TestResolveExternalIDFallsBackToHash(t *testing.T) {
	cfg := inboundConnectorConfig{IDField: "external_id"}
	record := map[string]any{"title": "sample", "value": 123}

	id := resolveExternalID(cfg, record)
	if id == "" {
		t.Fatal("expected fallback id to be generated")
	}

	id2 := resolveExternalID(cfg, record)
	if id != id2 {
		t.Fatalf("expected deterministic fallback id, got %q and %q", id, id2)
	}
}

func TestIsInboundConnector(t *testing.T) {
	item := models.CatalogItem{
		Data: map[string]any{
			"direction": "inbound",
			"enabled":   true,
		},
	}
	if !isInboundConnector(item) {
		t.Fatal("expected connector to be inbound")
	}
	if !isConnectorEnabled(item.Data) {
		t.Fatal("expected connector to be enabled")
	}
}

func TestParseInboundConnectorConfig_HTTPWithAuth(t *testing.T) {
	cfg, err := parseInboundConnectorConfig(map[string]any{
		"data": map[string]any{
			"name": "Feed",
			"type": "HTTP",
			"config": map[string]any{
				"url":      "https://example.local/feed",
				"method":   http.MethodPost,
				"schedule": "*/10 * * * *",
				"http": map[string]any{
					"auth": map[string]any{
						"type":            "api_key",
						"apiKey":          "secret",
						"api_key_header":  "X-Token",
						"defaultHeaders":  map[string]any{"X-From": "test"},
						"session_token":   "",
						"secret_access_k": "",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if cfg.SourceType != "http" {
		t.Fatalf("expected http source type, got %q", cfg.SourceType)
	}
	if cfg.HTTP.URL != "https://example.local/feed" {
		t.Fatalf("unexpected URL: %q", cfg.HTTP.URL)
	}
	if cfg.HTTP.Method != http.MethodPost {
		t.Fatalf("expected POST method, got %q", cfg.HTTP.Method)
	}
	if cfg.HTTP.Auth.Type != "api_key" {
		t.Fatalf("expected api_key auth, got %q", cfg.HTTP.Auth.Type)
	}
	if cfg.HTTP.Auth.APIKeyHeader != "X-Token" {
		t.Fatalf("expected API key header X-Token, got %q", cfg.HTTP.Auth.APIKeyHeader)
	}
}

func TestParseInboundConnectorConfig_SQLRequiresQuery(t *testing.T) {
	_, err := parseInboundConnectorConfig(map[string]any{
		"data": map[string]any{
			"name": "SQL Ingest",
			"type": "SQL",
			"config": map[string]any{
				"driver": "pgx",
				"host":   "db.internal",
			},
		},
	})
	if err == nil {
		t.Fatal("expected error for missing SQL query")
	}
}

func TestParseInboundConnectorConfig_SQLQueryAndDefaults(t *testing.T) {
	cfg, err := parseInboundConnectorConfig(map[string]any{
		"data": map[string]any{
			"name": "SQL Ingest",
			"type": "SQL",
			"config": map[string]any{
				"sql": map[string]any{
					"host":     "db.internal",
					"database": "threats",
					"query":    "SELECT id, title FROM alerts",
					"auth": map[string]any{
						"username": "soc",
						"password": "pass",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if cfg.SourceType != "sql" {
		t.Fatalf("expected sql source type, got %q", cfg.SourceType)
	}
	if cfg.SQL.Driver != "pgx" {
		t.Fatalf("expected pgx SQL driver, got %q", cfg.SQL.Driver)
	}
	if cfg.SQL.Query == "" {
		t.Fatal("expected SQL query to be parsed")
	}
	if cfg.SQL.Auth.Username != "soc" {
		t.Fatalf("unexpected SQL username: %q", cfg.SQL.Auth.Username)
	}
}

func TestParseInboundConnectorConfig_S3RequiresBucket(t *testing.T) {
	_, err := parseInboundConnectorConfig(map[string]any{
		"data": map[string]any{
			"name": "S3 Ingest",
			"type": "S3",
			"config": map[string]any{
				"s3": map[string]any{
					"endpoint": "http://minio:9000",
					"region":   "us-east-1",
				},
			},
		},
	})
	if err == nil {
		t.Fatal("expected error for missing S3 bucket")
	}
}

func TestParseInboundConnectorConfig_S3RequiresEndpoint(t *testing.T) {
	_, err := parseInboundConnectorConfig(map[string]any{
		"data": map[string]any{
			"name": "S3 Ingest",
			"type": "S3",
			"config": map[string]any{
				"s3": map[string]any{
					"bucket": "incoming-alerts",
					"region": "us-east-1",
				},
			},
		},
	})
	if err == nil {
		t.Fatal("expected error for missing S3 endpoint")
	}
}

func TestValidateReadOnlySQLQuery(t *testing.T) {
	if err := validateReadOnlySQLQuery("SELECT id FROM alerts"); err != nil {
		t.Fatalf("expected SELECT query to be accepted: %v", err)
	}
	if err := validateReadOnlySQLQuery("DELETE FROM alerts"); err == nil {
		t.Fatal("expected mutation query to be rejected")
	}
	if err := validateReadOnlySQLQuery("SELECT 1; SELECT 2"); err == nil {
		t.Fatal("expected multi-statement query to be rejected")
	}
}

func TestParseInboundConnectorConfig_Kafka(t *testing.T) {
	cfg, err := parseInboundConnectorConfig(map[string]any{
		"data": map[string]any{
			"name": "Kafka Ingest",
			"type": "KAFKA",
			"config": map[string]any{
				"source_type": "kafka",
				"kafka": map[string]any{
					"brokers":      "kafka-1:9092,kafka-2:9092",
					"topic":        "incidenthub-alerts",
					"group_id":     "incidenthub-inbound",
					"client_id":    "incidenthub-worker",
					"start_offset": "earliest",
					"max_messages": 25,
					"poll_timeout": 2,
					"commit":       true,
					"auth": map[string]any{
						"type":     "basic",
						"username": "svc-inbound",
						"password": "secret",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if cfg.SourceType != "kafka" {
		t.Fatalf("expected kafka source type, got %q", cfg.SourceType)
	}
	if cfg.Kafka.Topic != "incidenthub-alerts" {
		t.Fatalf("unexpected kafka topic: %q", cfg.Kafka.Topic)
	}
	if len(cfg.Kafka.Brokers) != 2 {
		t.Fatalf("expected 2 kafka brokers, got %d", len(cfg.Kafka.Brokers))
	}
	if cfg.Kafka.GroupID != "incidenthub-inbound" {
		t.Fatalf("unexpected kafka group id: %q", cfg.Kafka.GroupID)
	}
	if cfg.Kafka.Auth.Username != "svc-inbound" {
		t.Fatalf("unexpected kafka username: %q", cfg.Kafka.Auth.Username)
	}
	if cfg.Kafka.StartOffset != "earliest" {
		t.Fatalf("expected earliest offset, got %q", cfg.Kafka.StartOffset)
	}
}

func TestParseInboundConnectorConfig_KafkaValidation(t *testing.T) {
	_, err := parseInboundConnectorConfig(map[string]any{
		"data": map[string]any{
			"name": "Kafka Ingest",
			"type": "kafka",
			"config": map[string]any{
				"source_type": "kafka",
				"kafka": map[string]any{
					"brokers": "kafka:9092",
				},
			},
		},
	})
	if err == nil {
		t.Fatal("expected error for missing kafka topic")
	}
}

func TestParseInboundConnectorConfig_Redis(t *testing.T) {
	cfg, err := parseInboundConnectorConfig(map[string]any{
		"data": map[string]any{
			"name": "Redis Ingest",
			"type": "redis",
			"config": map[string]any{
				"source_type": "redis",
				"redis": map[string]any{
					"addr":         "redis.internal:6379",
					"db":           2,
					"key":          "incidenthub.inbound.alerts",
					"max_messages": 50,
					"timeout":      3,
					"pop_from":     "right",
					"auth": map[string]any{
						"type":     "basic",
						"username": "svc",
						"password": "secret",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if cfg.SourceType != "redis" {
		t.Fatalf("expected redis source type, got %q", cfg.SourceType)
	}
	if cfg.Redis.Addr != "redis.internal:6379" {
		t.Fatalf("unexpected redis addr: %q", cfg.Redis.Addr)
	}
	if cfg.Redis.DB != 2 {
		t.Fatalf("unexpected redis db: %d", cfg.Redis.DB)
	}
	if cfg.Redis.Key != "incidenthub.inbound.alerts" {
		t.Fatalf("unexpected redis key: %q", cfg.Redis.Key)
	}
	if cfg.Redis.PopFrom != "right" {
		t.Fatalf("unexpected redis pop direction: %q", cfg.Redis.PopFrom)
	}
	if cfg.Redis.Auth.Username != "svc" {
		t.Fatalf("unexpected redis username: %q", cfg.Redis.Auth.Username)
	}
}

func TestParseInboundConnectorConfig_RedisValidation(t *testing.T) {
	_, err := parseInboundConnectorConfig(map[string]any{
		"data": map[string]any{
			"name": "Redis Ingest",
			"type": "redis",
			"config": map[string]any{
				"source_type": "redis",
				"redis": map[string]any{
					"key": "incidenthub.inbound.alerts",
				},
			},
		},
	})
	if err == nil {
		t.Fatal("expected error for missing redis addr")
	}

	_, err = parseInboundConnectorConfig(map[string]any{
		"data": map[string]any{
			"name": "Redis Ingest",
			"type": "redis",
			"config": map[string]any{
				"source_type": "redis",
				"redis": map[string]any{
					"addr": "redis:6379",
				},
			},
		},
	})
	if err == nil {
		t.Fatal("expected error for missing redis key")
	}
}
