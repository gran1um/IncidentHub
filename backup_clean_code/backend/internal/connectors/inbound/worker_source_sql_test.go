package inbound

import (
	"context"
	"testing"
	"time"
)

func TestResolveSQLDSNAndNormalizeValue(t *testing.T) {
	dsn, err := resolveSQLDSN(inboundSQLSourceConfig{DSN: "postgres://ready"})
	if err != nil {
		t.Fatalf("unexpected dsn error: %v", err)
	}
	if dsn != "postgres://ready" {
		t.Fatalf("unexpected passthrough dsn: %q", dsn)
	}

	dsn, err = resolveSQLDSN(inboundSQLSourceConfig{
		Driver:   "pgx",
		Host:     "db.local",
		Port:     5432,
		Database: "incidenthub",
		SSLMode:  "disable",
		Auth: inboundAuthConfig{
			Username: "soc",
			Password: "secret",
		},
	})
	if err != nil {
		t.Fatalf("unexpected composed dsn error: %v", err)
	}
	if dsn == "" || dsn[:11] != "postgres://" {
		t.Fatalf("unexpected composed dsn: %q", dsn)
	}

	if _, err := resolveSQLDSN(inboundSQLSourceConfig{Driver: "mysql", Host: "db", Database: "x"}); err == nil {
		t.Fatal("expected unsupported driver error")
	}
	if _, err := resolveSQLDSN(inboundSQLSourceConfig{Driver: "pgx", Database: "x"}); err == nil {
		t.Fatal("expected missing host error")
	}

	now := time.Now().UTC().Truncate(time.Second)
	if got := normalizeSQLValue([]byte("payload")); got != "payload" {
		t.Fatalf("expected byte slice to string, got %#v", got)
	}
	if got := normalizeSQLValue(now); got != now.Format(time.RFC3339Nano) {
		t.Fatalf("expected RFC3339 time string, got %#v", got)
	}
	if got := normalizeSQLValue(nil); got != nil {
		t.Fatalf("expected nil passthrough, got %#v", got)
	}
}

func TestFetchSQLRecordsEarlyValidationErrors(t *testing.T) {
	worker := &Worker{}

	_, err := worker.fetchSQLRecords(context.Background(), inboundConnectorConfig{
		DisplayName: "SQL",
		SQL: inboundSQLSourceConfig{
			Query: "DELETE FROM alerts",
		},
	})
	if err == nil {
		t.Fatal("expected mutation query validation error")
	}

	_, err = worker.fetchSQLRecords(context.Background(), inboundConnectorConfig{
		DisplayName: "SQL",
		SQL: inboundSQLSourceConfig{
			Driver: "pgx",
			Query:  "SELECT id FROM alerts",
		},
	})
	if err == nil {
		t.Fatal("expected DSN resolution error")
	}
}
