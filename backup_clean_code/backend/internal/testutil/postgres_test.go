package testutil

import (
	"context"
	"strings"
	"testing"
	"time"

	"incidenthub/backend/internal/repository"
)

func TestTestDatabaseURLPriority(t *testing.T) {
	t.Setenv("TEST_DATABASE_URL", "postgres://test-db-url")
	t.Setenv("INCIDENTHUB_TEST_DATABASE_URL", "postgres://fallback")
	if got := TestDatabaseURL(); got != "postgres://test-db-url" {
		t.Fatalf("unexpected primary db url: %s", got)
	}

	t.Setenv("TEST_DATABASE_URL", "")
	t.Setenv("INCIDENTHUB_TEST_DATABASE_URL", "postgres://secondary")
	if got := TestDatabaseURL(); got != "postgres://secondary" {
		t.Fatalf("unexpected secondary db url: %s", got)
	}
}

func TestValidateTestDatabaseSafety(t *testing.T) {
	t.Setenv(allowUnsafeTestDBEnv, "")
	if err := validateTestDatabaseSafety("incidenthub_test", "postgres://incidenthub_test"); err != nil {
		t.Fatalf("expected *_test database to be safe, got %v", err)
	}
	if err := validateTestDatabaseSafety("incidenthub", "postgres://incidenthub"); err == nil {
		t.Fatal("expected incidenthub database to be rejected as unsafe")
	}

	t.Setenv(allowUnsafeTestDBEnv, "true")
	if err := validateTestDatabaseSafety("incidenthub", "postgres://incidenthub"); err != nil {
		t.Fatalf("expected unsafe override to bypass protection, got %v", err)
	}
}

func TestRepoRootAndQuoteIdentifier(t *testing.T) {
	root := repoRoot()
	if !strings.Contains(root, "backend") {
		t.Fatalf("unexpected repo root path: %s", root)
	}

	if got := quoteIdentifier(`table"name`); got != `"table""name"` {
		t.Fatalf("unexpected quoted identifier: %s", got)
	}
}

func TestOpenPoolAndResetPublicTables(t *testing.T) {
	pool := OpenTestPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tenants := repository.NewTenantRepository(pool)
	_, err := tenants.Create(ctx, repository.CreateTenantParams{
		Slug:        "testutil-tenant",
		Name:        "Testutil Tenant",
		Description: "tmp",
		MaxUsers:    10,
		IsActive:    true,
	})
	if err != nil {
		t.Fatalf("seed tenant before reset: %v", err)
	}

	ResetPublicTables(t, pool)

	items, err := tenants.List(ctx, 100, 0)
	if err != nil {
		t.Fatalf("list tenants after reset: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty tenants after reset, got %d", len(items))
	}
}
