package database_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/database"
	"incidenthub/backend/internal/testutil"
)

func TestNewPoolInvalidConfig(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := database.NewPool(ctx, config.PostgresConfig{
		Host:     "bad host",
		Port:     5432,
		User:     "u",
		Password: "p",
		DBName:   "db",
		SSLMode:  "disable",
	})
	if err == nil {
		t.Fatalf("expected invalid postgres config error")
	}
}

func TestApplyMigrationsReadDirError(t *testing.T) {
	pool := testutil.OpenTestPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	err := database.ApplyMigrations(ctx, pool, filepath.Join(t.TempDir(), "missing"))
	if err == nil || !strings.Contains(err.Error(), "read migration dir") {
		t.Fatalf("expected read migration dir error, got %v", err)
	}
}

func TestApplyMigrationsSuccessAndIdempotent(t *testing.T) {
	pool := testutil.OpenTestPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	dir := t.TempDir()
	sqlOne := `CREATE TABLE IF NOT EXISTS migration_test_one (id TEXT PRIMARY KEY);`
	sqlTwo := `ALTER TABLE migration_test_one ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ DEFAULT NOW();`
	if err := os.WriteFile(filepath.Join(dir, "001_create.sql"), []byte(sqlOne), 0o600); err != nil {
		t.Fatalf("write migration 1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "002_alter.sql"), []byte(sqlTwo), 0o600); err != nil {
		t.Fatalf("write migration 2: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.txt"), []byte("ignore"), 0o600); err != nil {
		t.Fatalf("write extra file: %v", err)
	}

	if err := database.ApplyMigrations(ctx, pool, dir); err != nil {
		t.Fatalf("apply migrations first run: %v", err)
	}
	if err := database.ApplyMigrations(ctx, pool, dir); err != nil {
		t.Fatalf("apply migrations second run: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version IN ('001_create.sql','002_alter.sql')`).Scan(&count); err != nil {
		t.Fatalf("count applied migrations: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 applied migrations, got %d", count)
	}
}

func TestApplyMigrationsConcurrentStart(t *testing.T) {
	pool := testutil.OpenTestPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	suffix := time.Now().UTC().UnixNano()
	tableName := fmt.Sprintf("migration_test_concurrent_%d", suffix)
	version := fmt.Sprintf("001_concurrent_%d.sql", suffix)
	dir := t.TempDir()
	sql := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (id TEXT PRIMARY KEY);
		SELECT pg_sleep(0.2);
		INSERT INTO %s(id) VALUES ('seed');
	`, tableName, tableName)
	if err := os.WriteFile(filepath.Join(dir, version), []byte(sql), 0o600); err != nil {
		t.Fatalf("write concurrent migration: %v", err)
	}

	start := make(chan struct{})
	errCh := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errCh <- database.ApplyMigrations(ctx, pool, dir)
		}()
	}
	close(start)
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent apply migrations failed: %v", err)
		}
	}

	var rows int
	if err := pool.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)).Scan(&rows); err != nil {
		t.Fatalf("count seeded rows: %v", err)
	}
	if rows != 1 {
		t.Fatalf("expected one seeded row after concurrent runs, got %d", rows)
	}

	var applied int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version = $1`, version).Scan(&applied); err != nil {
		t.Fatalf("count applied migrations: %v", err)
	}
	if applied != 1 {
		t.Fatalf("expected one schema_migrations row for %s, got %d", version, applied)
	}
}
