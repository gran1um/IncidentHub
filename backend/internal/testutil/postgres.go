package testutil

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"

	"incidenthub/backend/internal/database"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultTestDatabaseURL = "postgres://incidenthub:incidenthub@localhost:5432/incidenthub_test?sslmode=disable" //nolint:gosec // Default local credentials for integration tests.
const globalTestDBLockFile = "/tmp/incidenthub-test-db.lock"
const allowUnsafeTestDBEnv = "INCIDENTHUB_ALLOW_UNSAFE_TEST_DB"
const requireDBTestsEnv = "INCIDENTHUB_REQUIRE_DB_TESTS"

func TestDatabaseURL() string {
	for _, key := range []string{"TEST_DATABASE_URL", "INCIDENTHUB_TEST_DATABASE_URL"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return defaultTestDatabaseURL
}

func OpenTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	lockHandle, err := os.OpenFile(globalTestDBLockFile, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("open global test db lock file: %v", err)
	}
	fd := lockHandle.Fd()
	if fd > uintptr(math.MaxInt) {
		_ = lockHandle.Close()
		t.Fatalf("global test db lock file descriptor overflow: %d", fd)
	}
	fdInt := int(fd)
	if flockErr := syscall.Flock(fdInt, syscall.LOCK_EX); flockErr != nil {
		_ = lockHandle.Close()
		t.Fatalf("acquire global test db lock: %v", flockErr)
	}
	t.Cleanup(func() {
		_ = syscall.Flock(fdInt, syscall.LOCK_UN)
		_ = lockHandle.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	testDBURL := TestDatabaseURL()
	poolCfg, err := pgxpool.ParseConfig(testDBURL)
	if err != nil {
		t.Fatalf("parse test database url: %v", err)
	}
	if safetyErr := validateTestDatabaseSafety(poolCfg.ConnConfig.Database, testDBURL); safetyErr != nil {
		t.Fatal(safetyErr.Error())
	}
	if ensureErr := ensureTestDatabaseExists(poolCfg); ensureErr != nil {
		skipOrFatalDB(t, "ensure test database exists", ensureErr)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		skipOrFatalDB(t, "create postgres pool", err)
	}
	t.Cleanup(pool.Close)

	if err := pool.Ping(ctx); err != nil {
		skipOrFatalDB(t, "ping postgres", err)
	}

	migrationDir := filepath.Join(repoRoot(), "migrations")
	if err := database.ApplyMigrations(ctx, pool, migrationDir); err != nil {
		skipOrFatalDB(t, "apply migrations", err)
	}

	return pool
}

func ensureTestDatabaseExists(poolCfg *pgxpool.Config) error {
	if poolCfg == nil || poolCfg.ConnConfig == nil {
		return fmt.Errorf("test database config is not initialized")
	}
	dbName := strings.TrimSpace(poolCfg.ConnConfig.Database)
	if dbName == "" {
		return fmt.Errorf("test database name is empty")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	adminConnCfg := poolCfg.ConnConfig.Copy()
	adminConnCfg.Database = "postgres"
	adminConn, err := pgx.ConnectConfig(ctx, adminConnCfg)
	if err != nil {
		return fmt.Errorf("connect postgres admin database: %w", err)
	}
	defer func() { _ = adminConn.Close(ctx) }()

	var exists bool
	if err := adminConn.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)`, dbName).Scan(&exists); err != nil {
		return fmt.Errorf("check test database existence: %w", err)
	}
	if exists {
		return nil
	}

	if _, err := adminConn.Exec(ctx, "CREATE DATABASE "+quoteIdentifier(dbName)); err != nil {
		return fmt.Errorf("create test database %q: %w", dbName, err)
	}
	return nil
}

func validateTestDatabaseSafety(databaseName, dsn string) error {
	if allowUnsafeTestDB() {
		return nil
	}

	name := strings.ToLower(strings.TrimSpace(databaseName))
	if name == "" {
		return fmt.Errorf("unsafe test database target: empty database name in %q", dsn)
	}
	if strings.Contains(name, "test") {
		return nil
	}
	switch name {
	case "incidenthub", "postgres", "template0", "template1":
		return fmt.Errorf("unsafe test database target %q (dsn=%q): configure TEST_DATABASE_URL to a dedicated *_test database", databaseName, dsn)
	default:
		return fmt.Errorf("unsafe test database target %q (dsn=%q): database name must contain \"test\" or set %s=true explicitly", databaseName, dsn, allowUnsafeTestDBEnv)
	}
}

func allowUnsafeTestDB() bool {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv(allowUnsafeTestDBEnv)))
	return raw == "1" || raw == "true" || raw == "yes"
}

func ResetPublicTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rows, err := pool.Query(ctx, `
		SELECT tablename
		FROM pg_tables
		WHERE schemaname = 'public'
		  AND tablename NOT IN ('schema_migrations')
	`)
	if err != nil {
		t.Fatalf("list public tables: %v", err)
	}
	defer rows.Close()

	tables := make([]string, 0, 64)
	for rows.Next() {
		var table string
		if scanErr := rows.Scan(&table); scanErr != nil {
			t.Fatalf("scan table name: %v", scanErr)
		}
		tables = append(tables, quoteIdentifier(table))
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate table names: %v", err)
	}
	if len(tables) == 0 {
		return
	}

	sort.Strings(tables)
	truncateSQL := fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", strings.Join(tables, ", "))
	if _, err := pool.Exec(ctx, truncateSQL); err != nil {
		t.Fatalf("truncate public tables: %v", err)
	}
}

func repoRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot resolve caller path")
	}
	// internal/testutil/postgres.go -> backend/
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func quoteIdentifier(input string) string {
	return `"` + strings.ReplaceAll(input, `"`, `""`) + `"`
}

func skipOrFatalDB(t *testing.T, action string, err error) {
	t.Helper()
	if shouldSkipDBError(err) {
		t.Skipf("%s: %v", action, err)
		return
	}
	t.Fatalf("%s: %v", action, err)
}

func shouldSkipDBError(err error) bool {
	if err == nil {
		return false
	}
	rawRequire := strings.ToLower(strings.TrimSpace(os.Getenv(requireDBTestsEnv)))
	requireDB := rawRequire == "1" || rawRequire == "true" || rawRequire == "yes"
	if requireDB {
		return false
	}
	text := strings.ToLower(err.Error())
	switch {
	case strings.Contains(text, "connect: connection refused"),
		strings.Contains(text, "server closed the connection"),
		strings.Contains(text, "failed to connect"),
		strings.Contains(text, "dial tcp"),
		strings.Contains(text, "lookup"),
		strings.Contains(text, "timeout"),
		strings.Contains(text, "no such host"),
		strings.Contains(text, "connect postgres"):
		return true
	default:
		return false
	}
}
