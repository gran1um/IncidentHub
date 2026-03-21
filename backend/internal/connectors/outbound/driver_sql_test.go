package outbound

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/testutil"
)

func TestSQLDriver_PostgresSelect(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Ensure test database exists and migrations are applied.
	pool := testutil.OpenTestPool(t)

	setupCtx, setupCancel := context.WithTimeout(ctx, 10*time.Second)
	defer setupCancel()
	if _, err := pool.Exec(setupCtx, `
		CREATE TABLE IF NOT EXISTS outbound_sql_driver_test (
			hash text,
			verdict text
		);
		TRUNCATE TABLE outbound_sql_driver_test;
		INSERT INTO outbound_sql_driver_test (hash, verdict) VALUES
			('abc', 'clean'),
			('def', 'malicious');
	`); err != nil {
		t.Fatalf("prepare sql driver test table: %v", err)
	}

	svc := NewService(config.OutboundConnectorsConfig{})
	connector := models.CatalogItem{
		Data: map[string]any{
			"channel": "sql",
			"config": map[string]any{
				"dsn": testutil.TestDatabaseURL(),
			},
		},
	}

	resp, err := svc.Send(ctx, connector, SendRequest{
		ConversationID: "sql-test-1",
		Message:        "SELECT hash, verdict FROM outbound_sql_driver_test ORDER BY hash ASC",
	})
	if err != nil {
		t.Fatalf("sql driver send failed: %v", err)
	}
	if resp.ConversationID != "sql-test-1" {
		t.Fatalf("unexpected conversation id: %q", resp.ConversationID)
	}

	var rows []map[string]any
	if err := json.Unmarshal([]byte(resp.Reply), &rows); err != nil {
		t.Fatalf("decode sql driver reply: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if got := rows[0]["hash"]; got != "abc" {
		t.Fatalf("unexpected first hash: %v", got)
	}
	if got := rows[1]["verdict"]; got != "malicious" {
		t.Fatalf("unexpected second verdict: %v", got)
	}

	rowCount, _ := resp.Metadata["row_count"].(int)
	if rowCount != 2 {
		t.Fatalf("expected row_count=2 in metadata, got %d", rowCount)
	}
}

func TestSQLDriver_RejectsNonSelect(t *testing.T) {
	svc := NewService(config.OutboundConnectorsConfig{})
	connector := models.CatalogItem{
		Data: map[string]any{
			"channel": "sql",
			"config": map[string]any{
				"dsn": testutil.TestDatabaseURL(),
			},
		},
	}

	_, err := svc.Send(context.Background(), connector, SendRequest{Message: "UPDATE outbound_sql_driver_test SET verdict = 'x'"})
	if err == nil {
		t.Fatal("expected error for non-SELECT query, got nil")
	}
}
