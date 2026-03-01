package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"incidenthub/backend/internal/tracing"

	"github.com/jackc/pgx/v5/pgxpool"
)

const migrationsAdvisoryLockKey int64 = 417241006313230915

func ApplyMigrations(ctx context.Context, pool *pgxpool.Pool, migrationDir string) (err error) {
	ctx, span, startedAt := tracing.StartModuleOperation(ctx, "database", "apply_migrations")
	defer func() {
		tracing.FinishModuleOperation(span, startedAt, "database", "apply_migrations", err)
	}()

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}
	defer conn.Release()

	if _, execErr := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`); execErr != nil {
		return fmt.Errorf("create schema_migrations: %w", execErr)
	}
	if _, lockErr := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, migrationsAdvisoryLockKey); lockErr != nil {
		return fmt.Errorf("acquire schema_migrations advisory lock: %w", lockErr)
	}
	defer func() {
		_, _ = conn.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, migrationsAdvisoryLockKey)
	}()

	entries, err := os.ReadDir(migrationDir)
	if err != nil {
		return fmt.Errorf("read migration dir: %w", err)
	}

	files := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		files = append(files, e.Name())
	}
	sort.Strings(files)

	for _, file := range files {
		var exists bool
		err := conn.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, file).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", file, err)
		}
		if exists {
			continue
		}

		body, err := os.ReadFile(filepath.Join(migrationDir, file))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", file, err)
		}

		tx, err := conn.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin migration tx %s: %w", file, err)
		}
		if _, err := tx.Exec(ctx, string(body)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply migration %s: %w", file, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES ($1)`, file); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("mark migration %s: %w", file, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", file, err)
		}
	}

	return nil
}
