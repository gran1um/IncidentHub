package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"incidenthub/backend/internal/models"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRequestsLast24hByTenant(t *testing.T) {
	collector := New("incidenthub_test")
	now := time.Now().UTC()

	collector.recordRequest("tenant-a", now.Add(-1*time.Hour))
	collector.recordRequest("tenant-a", now.Add(-2*time.Hour))
	collector.recordRequest("tenant-b", now.Add(-1*time.Hour))
	collector.recordRequest("", now.Add(-1*time.Hour))
	collector.recordRequest("tenant-a", now.Add(-30*time.Hour))

	if got := collector.RequestsLast24h("tenant-a"); got != 2 {
		t.Fatalf("expected 2 requests for tenant-a in last 24h, got %d", got)
	}
	if got := collector.RequestsLast24h("tenant-b"); got != 1 {
		t.Fatalf("expected 1 request for tenant-b in last 24h, got %d", got)
	}
	if got := collector.RequestsLast24h(""); got != 4 {
		t.Fatalf("expected 4 total requests in last 24h, got %d", got)
	}
}

func TestNormalizeTenantBucket(t *testing.T) {
	if got := normalizeTenantBucket("  "); got != globalTenantBucket {
		t.Fatalf("expected empty tenant to use global bucket, got %q", got)
	}
	if got := normalizeTenantBucket(" tenant-1 "); got != "tenant-1" {
		t.Fatalf("expected trimmed tenant id, got %q", got)
	}
}

func TestMiddlewareResolvesTenantFromContext(t *testing.T) {
	collector := New("incidenthub_test")
	middleware := collector.Middleware()
	tenantID := uuid.New()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/resources", http.NoBody)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.Set("tenant_id", tenantID)

	handler := middleware(func(c *echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})
	if err := handler(ctx); err != nil {
		t.Fatalf("middleware handler returned error: %v", err)
	}

	if got := collector.RequestsLast24h(tenantID.String()); got != 1 {
		t.Fatalf("expected context tenant bucket to receive request, got %d", got)
	}
}

func TestMiddlewareResolvesTenantFromIdentity(t *testing.T) {
	collector := New("incidenthub_test")
	middleware := collector.Middleware()
	tenantID := uuid.New()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/resources", http.NoBody)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.Set("identity", models.Identity{
		UserID:   uuid.New(),
		TenantID: &tenantID,
	})

	handler := middleware(func(c *echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})
	if err := handler(ctx); err != nil {
		t.Fatalf("middleware handler returned error: %v", err)
	}

	if got := collector.RequestsLast24h(tenantID.String()); got != 1 {
		t.Fatalf("expected identity tenant bucket to receive request, got %d", got)
	}
}

func TestModuleMetricsAndGlobalCollector(t *testing.T) {
	collector := New("incidenthub_test")
	SetGlobal(collector)
	defer SetGlobal(nil)

	ObserveModuleOperation("cache", "get", "ok", 15*time.Millisecond)
	ObserveModuleOperation("cache", "get", "error", 7*time.Millisecond)
	SetModuleHealth("cache", "ok", 12)
	SetModuleHealth("ai", "ok", 0.23)
	SetModuleHealth("search", "error", 45)

	if got := testutil.ToFloat64(collector.ModuleOperations.WithLabelValues("cache", "get", "ok")); got != 1 {
		t.Fatalf("expected module operation counter cache/get/ok = 1, got %v", got)
	}
	if got := testutil.ToFloat64(collector.ModuleOperations.WithLabelValues("cache", "get", "error")); got != 1 {
		t.Fatalf("expected module operation counter cache/get/error = 1, got %v", got)
	}
	if got := testutil.ToFloat64(collector.ModuleHealthStatus.WithLabelValues("cache")); got != 1 {
		t.Fatalf("expected cache health status gauge = 1, got %v", got)
	}
	if got := testutil.ToFloat64(collector.ModuleHealthStatus.WithLabelValues("search")); got != -1 {
		t.Fatalf("expected search health status gauge = -1, got %v", got)
	}
	if got := testutil.ToFloat64(collector.ModuleHealthLatency.WithLabelValues("cache")); got != 12 {
		t.Fatalf("expected cache latency gauge = 12, got %v", got)
	}
	if got := testutil.ToFloat64(collector.ModuleHealthLatency.WithLabelValues("ai")); got != 0.23 {
		t.Fatalf("expected ai latency gauge = 0.23, got %v", got)
	}
}

func TestModuleOperationSnapshot(t *testing.T) {
	collector := New("incidenthub_test")

	collector.ObserveModuleOperation("api", "request", "ok", 100*time.Millisecond)
	collector.ObserveModuleOperation("api", "request", "error", 300*time.Millisecond)
	collector.ObserveModuleOperation("cache", "get", "ok", 50*time.Millisecond)

	snapshot := collector.ModuleOperationSnapshot("api", 60*time.Second)
	if snapshot.Operations != 2 {
		t.Fatalf("expected 2 operations for api, got %d", snapshot.Operations)
	}
	if snapshot.Errors != 1 {
		t.Fatalf("expected 1 error for api, got %d", snapshot.Errors)
	}
	if snapshot.AvgLatencyMs < 199 || snapshot.AvgLatencyMs > 201 {
		t.Fatalf("expected avg latency around 200ms, got %.2f", snapshot.AvgLatencyMs)
	}
	if snapshot.RequestsPerSecond <= 0 {
		t.Fatalf("expected positive rps, got %.4f", snapshot.RequestsPerSecond)
	}

	total := collector.ModuleOperationSnapshot("*", 60*time.Second)
	if total.Operations != 3 {
		t.Fatalf("expected aggregate operations to be 3, got %d", total.Operations)
	}
	if total.Module != "*" {
		t.Fatalf("expected aggregate module marker '*', got %q", total.Module)
	}
}

func TestModuleOperationSnapshotByOperation(t *testing.T) {
	collector := New("incidenthub_test")

	collector.ObserveModuleOperation("ai", "ping", "ok", 5*time.Millisecond)
	collector.ObserveModuleOperation("ai", "ask", "ok", 120*time.Millisecond)
	collector.ObserveModuleOperation("ai", "ask", "error", 180*time.Millisecond)

	ask := collector.ModuleOperationSnapshotByOperation("ai", "ask", 60*time.Second)
	if ask.Operations != 2 {
		t.Fatalf("expected 2 ask operations, got %d", ask.Operations)
	}
	if ask.Errors != 1 {
		t.Fatalf("expected 1 ask error, got %d", ask.Errors)
	}
	if ask.AvgLatencyMs < 149 || ask.AvgLatencyMs > 151 {
		t.Fatalf("expected ask latency around 150ms, got %.2f", ask.AvgLatencyMs)
	}

	ping := collector.ModuleOperationSnapshotByOperation("ai", "ping", 60*time.Second)
	if ping.Operations != 1 {
		t.Fatalf("expected 1 ping operation, got %d", ping.Operations)
	}
	if ping.AvgLatencyMs < 4 || ping.AvgLatencyMs > 6 {
		t.Fatalf("expected ping latency around 5ms, got %.2f", ping.AvgLatencyMs)
	}
}
