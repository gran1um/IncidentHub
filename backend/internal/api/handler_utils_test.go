package api

import (
	"context"
	"errors"
	"fmt"
	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/metrics"
	"testing"
	"time"

	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/search"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

type healthCheckSearchStub struct {
	delay time.Duration
	err   error
}

func (s *healthCheckSearchStub) IndexDocument(context.Context, string, string, any) error {
	return nil
}

func (s *healthCheckSearchStub) DeleteDocument(context.Context, string, string) error {
	return nil
}

func (s *healthCheckSearchStub) Search(context.Context, string, string, []string, int) ([]search.Hit, error) {
	return nil, nil
}

func (s *healthCheckSearchStub) Ping(context.Context) error {
	if s.delay > 0 {
		time.Sleep(s.delay)
	}
	return s.err
}

func (s *healthCheckSearchStub) Enabled() bool {
	return true
}

func TestTrimOptional(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		if got := trimOptional(nil); got != nil {
			t.Fatalf("expected nil, got %v", *got)
		}
	})

	t.Run("empty after trim", func(t *testing.T) {
		value := "   "
		if got := trimOptional(&value); got != nil {
			t.Fatalf("expected nil, got %v", *got)
		}
	})

	t.Run("non-empty", func(t *testing.T) {
		value := "  Alice Doe  "
		got := trimOptional(&value)
		if got == nil || *got != "Alice Doe" {
			t.Fatalf("expected Alice Doe, got %v", got)
		}
	})
}

func TestLowerOptional(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		if got := lowerOptional(nil); got != nil {
			t.Fatalf("expected nil, got %v", *got)
		}
	})

	t.Run("empty after trim", func(t *testing.T) {
		value := "   "
		if got := lowerOptional(&value); got != nil {
			t.Fatalf("expected nil, got %v", *got)
		}
	})

	t.Run("lowercases value", func(t *testing.T) {
		value := "  In_Progress  "
		got := lowerOptional(&value)
		if got == nil || *got != "in_progress" {
			t.Fatalf("expected in_progress, got %v", got)
		}
	})
}

func TestParseRole(t *testing.T) {
	t.Run("empty defaults to analyst", func(t *testing.T) {
		got, err := parseRole("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != models.TenantRoleAnalyst {
			t.Fatalf("expected analyst, got %s", got)
		}
	})

	t.Run("valid role", func(t *testing.T) {
		got, err := parseRole("tenant_admin")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != models.TenantRoleAdmin {
			t.Fatalf("expected tenant_admin, got %s", got)
		}
	})

	t.Run("invalid role", func(t *testing.T) {
		if _, err := parseRole("superadmin"); err == nil {
			t.Fatal("expected error for invalid role")
		}
	})
}

func TestNormalizeTags(t *testing.T) {
	input := []string{" IOC ", "ioc", "Phishing", " ", "phishing", "TLP:Amber"}
	got := normalizeTags(input)
	want := []string{"ioc", "phishing", "tlp:amber"}
	if len(got) != len(want) {
		t.Fatalf("expected %d tags, got %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}

func TestNormalizeCatalogKind(t *testing.T) {
	if got := normalizeCatalogKind("  Case_Connectors "); got != "case_connectors" {
		t.Fatalf("expected case_connectors, got %q", got)
	}
}

func TestStringFromMap(t *testing.T) {
	input := map[string]any{
		"thread_id": "abc",
		"threadId":  "def",
	}
	if got := stringFromMap(input, "thread_id", "threadId"); got != "abc" {
		t.Fatalf("expected abc, got %q", got)
	}
	if got := stringFromMap(input, "missing", "threadId"); got != "def" {
		t.Fatalf("expected def, got %q", got)
	}
}

func TestMatchesAllSearchTerms(t *testing.T) {
	t.Run("matches single word", func(t *testing.T) {
		if !matchesAllSearchTerms("smoke", "Smoke Case 2026", "description") {
			t.Fatal("expected single-word query to match")
		}
	})

	t.Run("matches multi-word across fields", func(t *testing.T) {
		if !matchesAllSearchTerms("smoke case", "Smoke", "Case 2026") {
			t.Fatal("expected multi-word query to match across fields")
		}
	})

	t.Run("requires all words", func(t *testing.T) {
		if matchesAllSearchTerms("smoke malware", "Smoke Case 2026") {
			t.Fatal("expected query to fail when one word is missing")
		}
	})

	t.Run("empty query returns false", func(t *testing.T) {
		if matchesAllSearchTerms("   ", "anything") {
			t.Fatal("expected empty query to return false")
		}
	})
}

func TestCatalogItemToPayload(t *testing.T) {
	t.Run("keeps regular fields", func(t *testing.T) {
		tenantID := uuid.New()
		ownerID := uuid.New()
		refID := uuid.New()
		item := models.CatalogItem{
			ID:       uuid.New(),
			TenantID: &tenantID,
			Kind:     "forum_thread",
			OwnerID:  &ownerID,
			RefID:    &refID,
			Data: map[string]any{
				"title": "Thread A",
			},
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}

		payload := catalogItemToPayload(item)
		if payload["id"] == "" {
			t.Fatalf("expected id in payload")
		}
		if payload["tenant_id"] != tenantID.String() {
			t.Fatalf("expected tenant_id %s, got %v", tenantID.String(), payload["tenant_id"])
		}
		if payload["title"] != "Thread A" {
			t.Fatalf("expected title Thread A, got %v", payload["title"])
		}
	})

	t.Run("redacts legacy api token secrets", func(t *testing.T) {
		item := models.CatalogItem{
			ID:   uuid.New(),
			Kind: "api_tokens",
			Data: map[string]any{
				"name":   "legacy token",
				"token":  "raw-secret-value",
				"secret": "another-secret",
			},
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}

		payload := catalogItemToPayload(item)
		if _, ok := payload["token"]; ok {
			t.Fatalf("token value must be redacted from payload")
		}
		if _, ok := payload["secret"]; ok {
			t.Fatalf("secret value must be redacted from payload")
		}
		if payload["auth_supported"] != false {
			t.Fatalf("expected auth_supported=false for legacy api_tokens payload")
		}
	})
}

func TestCanMutateCatalog(t *testing.T) {
	handler := &Handler{}
	platform := models.Identity{UserID: uuid.New(), IsPlatformAdmin: true}
	if !handler.canMutateCatalog(platform, "rate_limits", true) {
		t.Fatal("platform admin should mutate any catalog kind")
	}

	admin := models.Identity{UserID: uuid.New(), TenantRole: models.TenantRoleAdmin}
	if !handler.canMutateCatalog(admin, "automations", true) {
		t.Fatal("tenant admin should mutate admin catalog kinds")
	}

	analyst := models.Identity{UserID: uuid.New(), TenantRole: models.TenantRoleAnalyst}
	if !handler.canMutateCatalog(analyst, "notifications", false) {
		t.Fatal("analyst should mutate own notifications flow")
	}
	if !handler.canMutateCatalog(analyst, "workflows", true) {
		t.Fatal("analyst should create unified workflows")
	}
	if handler.canMutateCatalog(analyst, "analyzer_workflows", true) {
		t.Fatal("analyst should not create deprecated analyzer workflows")
	}
	if handler.canMutateCatalog(analyst, "responder_workflows", true) {
		t.Fatal("analyst should not create deprecated responder workflows")
	}
	if handler.canMutateCatalog(analyst, "action_modules", true) {
		t.Fatal("analyst should not create deprecated action modules")
	}
	if !handler.canMutateCatalog(analyst, "case_meta", true) {
		t.Fatal("analyst should mutate case metadata used by case edit flow")
	}
	if !handler.canMutateCatalog(analyst, "alert_meta", true) {
		t.Fatal("analyst should mutate alert metadata used by alert edit flow")
	}
	if handler.canMutateCatalog(analyst, "rate_limits", true) {
		t.Fatal("analyst should not create rate limits")
	}
}

func TestCheckModule(t *testing.T) {
	h := &Handler{}

	t.Run("disabled module", func(t *testing.T) {
		module := h.checkModule(context.Background(), true, func(context.Context) error {
			return errors.New("must not execute")
		})
		if module.Status != "disabled" {
			t.Fatalf("expected disabled status, got %s", module.Status)
		}
		if module.ResponseMs != 0 {
			t.Fatalf("expected 0 responseMs for disabled module, got %.2f", module.ResponseMs)
		}
	})

	t.Run("healthy module", func(t *testing.T) {
		module := h.checkModule(context.Background(), false, func(context.Context) error {
			return nil
		})
		if module.Status != "ok" {
			t.Fatalf("expected ok status, got %s", module.Status)
		}
		if module.ResponseMs <= 0 {
			t.Fatalf("expected positive responseMs, got %.2f", module.ResponseMs)
		}
	})

	t.Run("measures elapsed duration", func(t *testing.T) {
		module := h.checkModule(context.Background(), false, func(context.Context) error {
			time.Sleep(25 * time.Millisecond)
			return nil
		})
		if module.Status != "ok" {
			t.Fatalf("expected ok status, got %s", module.Status)
		}
		if module.ResponseMs < 20 {
			t.Fatalf("expected measured responseMs to include the sleep, got %.2f", module.ResponseMs)
		}
	})

	t.Run("failed module", func(t *testing.T) {
		module := h.checkModule(context.Background(), false, func(context.Context) error {
			return errors.New("boom")
		})
		if module.Status != "error" {
			t.Fatalf("expected error status, got %s", module.Status)
		}
		if module.Message == "" {
			t.Fatal("expected module error message")
		}
	})
}

func TestDurationMilliseconds(t *testing.T) {
	t.Run("keeps sub millisecond precision", func(t *testing.T) {
		got := durationMilliseconds(250 * time.Microsecond)
		if got <= 0 || got >= 1 {
			t.Fatalf("expected sub millisecond value, got %.2f", got)
		}
	})

	t.Run("caps tiny positive durations above zero", func(t *testing.T) {
		got := durationMilliseconds(time.Nanosecond)
		if got != 0.01 {
			t.Fatalf("expected 0.01ms minimum for tiny positive durations, got %.2f", got)
		}
	})

	t.Run("keeps zero duration at zero", func(t *testing.T) {
		if got := durationMilliseconds(0); got != 0 {
			t.Fatalf("expected zero duration to stay zero, got %.2f", got)
		}
	})
}

func TestCollectModulesHealth(t *testing.T) {
	h := &Handler{
		cfg: config.App{
			Elastic: config.ElasticConfig{Enabled: false},
			S3:      config.S3Config{Enabled: false},
		},
	}

	status, modules := h.collectModulesHealth(context.Background())
	if status != "degraded" {
		t.Fatalf("expected degraded overall status with missing postgres/redis, got %s", status)
	}
	if modules["api"].Status != "ok" {
		t.Fatalf("expected api module ok, got %s", modules["api"].Status)
	}
	if modules["elasticsearch"].Status != "disabled" {
		t.Fatalf("expected elasticsearch disabled, got %s", modules["elasticsearch"].Status)
	}
	if modules["s3"].Status != "disabled" {
		t.Fatalf("expected s3 disabled, got %s", modules["s3"].Status)
	}
	if modules["ai_model"].Status != "disabled" {
		t.Fatalf("expected ai_model disabled, got %s", modules["ai_model"].Status)
	}
	if modules["postgres"].Status != "error" {
		t.Fatalf("expected postgres error, got %s", modules["postgres"].Status)
	}
	if modules["redis"].Status != "error" {
		t.Fatalf("expected redis error, got %s", modules["redis"].Status)
	}
}

func TestCollectModulesHealthRecordsMeasuredLatency(t *testing.T) {
	collector := metrics.New("incidenthub_test")
	h := &Handler{
		cfg: config.App{
			Elastic: config.ElasticConfig{Enabled: true},
		},
		search:  &healthCheckSearchStub{delay: 30 * time.Millisecond},
		metrics: collector,
	}

	_, modules := h.collectModulesHealth(context.Background())
	elasticsearch := modules["elasticsearch"]

	if elasticsearch.Status != "ok" {
		t.Fatalf("expected elasticsearch health check to succeed, got %s", elasticsearch.Status)
	}
	if elasticsearch.ResponseMs < 25 {
		t.Fatalf("expected elasticsearch responseMs to include ping delay, got %.2f", elasticsearch.ResponseMs)
	}
	if got := testutil.ToFloat64(collector.ModuleHealthLatency.WithLabelValues("elasticsearch")); got < 25 {
		t.Fatalf("expected collector to store elasticsearch health latency, got %.2f", got)
	}
}

func TestIsForumThreadUniqueViolation(t *testing.T) {
	match := &pgconn.PgError{Code: "23505", ConstraintName: "uq_catalog_forum_thread_case"}
	if !isForumThreadUniqueViolation(match) {
		t.Fatal("expected unique violation to match forum thread constraint")
	}

	wrapped := fmt.Errorf("wrapped: %w", match)
	if !isForumThreadUniqueViolation(wrapped) {
		t.Fatal("expected wrapped unique violation to match")
	}

	other := &pgconn.PgError{Code: "23505", ConstraintName: "another_constraint"}
	if isForumThreadUniqueViolation(other) {
		t.Fatal("expected different constraint to be ignored")
	}

	otherCode := &pgconn.PgError{Code: "22001", ConstraintName: "uq_catalog_forum_thread_case"}
	if isForumThreadUniqueViolation(otherCode) {
		t.Fatal("expected different postgres error code to be ignored")
	}
}

func TestCollectHostResources(t *testing.T) {
	h := &Handler{startedAt: time.Now().Add(-10 * time.Second)}
	stats := h.collectHostResources()

	if stats.CPUPercent < 0 {
		t.Fatalf("expected non-negative cpu percent, got %f", stats.CPUPercent)
	}
	if stats.CPUCoresTotal < 1 {
		t.Fatalf("expected at least one cpu core, got %d", stats.CPUCoresTotal)
	}
	if stats.CPUCoresUsed < 0 {
		t.Fatalf("expected non-negative used cores, got %f", stats.CPUCoresUsed)
	}
	if stats.MemoryUsedMB < 0 {
		t.Fatalf("expected non-negative memory MB, got %f", stats.MemoryUsedMB)
	}
	if stats.MemoryTotalMB < stats.MemoryUsedMB {
		t.Fatalf("expected total memory >= used memory, got used=%f total=%f", stats.MemoryUsedMB, stats.MemoryTotalMB)
	}
	if stats.MemoryUsedPercent < 0 {
		t.Fatalf("expected non-negative memory percent, got %f", stats.MemoryUsedPercent)
	}
	if stats.DiskUsedPercent < 0 {
		t.Fatalf("expected non-negative disk percent, got %f", stats.DiskUsedPercent)
	}
	if stats.DiskTotalGB < stats.DiskUsedGB {
		t.Fatalf("expected total disk >= used disk, got used=%f total=%f", stats.DiskUsedGB, stats.DiskTotalGB)
	}
}
