package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"

	"incidenthub/backend/internal/cache"
	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/models"
)

func TestAPICacheStoresAndServesGETFromRedis(t *testing.T) {
	client := newTestAPICacheClient(t)
	tenantID := uuid.New()
	identity := models.Identity{UserID: uuid.New(), TenantID: &tenantID, TenantRole: models.TenantRoleAnalyst}

	e := echo.New()
	e.Use(testContextMiddleware(identity, tenantID))
	e.Use(APICacheInvalidation(client, config.APICacheConfig{Enabled: true}))
	e.Use(APICache(client, config.APICacheConfig{
		Enabled:      true,
		TTL:          time.Minute,
		MaxBodyBytes: 4096,
	}))

	getCalls := 0
	e.GET("/api/v1/cases", func(c *echo.Context) error {
		getCalls++
		return c.JSON(http.StatusOK, map[string]any{
			"call":  getCalls,
			"value": "cached-response",
		})
	})

	rec1 := doAPICacheRequest(t, e, http.MethodGet, "/api/v1/cases?limit=10", tenantID)
	if got := rec1.Header().Get(apiCacheHeader); got != "MISS" {
		t.Fatalf("expected first get to be MISS, got %q", got)
	}
	payload1 := decodeJSONBody(t, rec1)
	if got := int(payload1["call"].(float64)); got != 1 {
		t.Fatalf("expected first payload call=1, got %d", got)
	}

	rec2 := doAPICacheRequest(t, e, http.MethodGet, "/api/v1/cases?limit=10", tenantID)
	if got := rec2.Header().Get(apiCacheHeader); got != "HIT" {
		t.Fatalf("expected second get to be HIT, got %q", got)
	}
	payload2 := decodeJSONBody(t, rec2)
	if got := int(payload2["call"].(float64)); got != 1 {
		t.Fatalf("expected cached payload call=1, got %d", got)
	}
	if getCalls != 1 {
		t.Fatalf("expected handler to execute once, got %d calls", getCalls)
	}
}

func TestAPICacheInvalidationAfterMutation(t *testing.T) {
	client := newTestAPICacheClient(t)
	tenantID := uuid.New()
	identity := models.Identity{UserID: uuid.New(), TenantID: &tenantID, TenantRole: models.TenantRoleAnalyst}

	e := echo.New()
	e.Use(testContextMiddleware(identity, tenantID))
	e.Use(APICacheInvalidation(client, config.APICacheConfig{Enabled: true}))
	e.Use(APICache(client, config.APICacheConfig{
		Enabled:      true,
		TTL:          time.Minute,
		MaxBodyBytes: 4096,
	}))

	getCalls := 0
	currentValue := "before"
	e.GET("/api/v1/alerts", func(c *echo.Context) error {
		getCalls++
		return c.JSON(http.StatusOK, map[string]any{
			"call":  getCalls,
			"value": currentValue,
		})
	})
	e.POST("/api/v1/alerts", func(c *echo.Context) error {
		currentValue = "after"
		return c.JSON(http.StatusCreated, map[string]any{"ok": true})
	})

	rec1 := doAPICacheRequest(t, e, http.MethodGet, "/api/v1/alerts?severity=high", tenantID)
	if got := rec1.Header().Get(apiCacheHeader); got != "MISS" {
		t.Fatalf("expected initial get miss, got %q", got)
	}
	firstPayload := decodeJSONBody(t, rec1)
	if got := firstPayload["value"].(string); got != "before" {
		t.Fatalf("expected initial value=before, got %q", got)
	}

	rec2 := doAPICacheRequest(t, e, http.MethodGet, "/api/v1/alerts?severity=high", tenantID)
	if got := rec2.Header().Get(apiCacheHeader); got != "HIT" {
		t.Fatalf("expected cached get hit, got %q", got)
	}
	if getCalls != 1 {
		t.Fatalf("expected one get handler call before mutation, got %d", getCalls)
	}

	recPost := doAPICacheRequest(t, e, http.MethodPost, "/api/v1/alerts", tenantID)
	if recPost.Code != http.StatusCreated {
		t.Fatalf("expected post 201, got %d", recPost.Code)
	}

	rec3 := doAPICacheRequest(t, e, http.MethodGet, "/api/v1/alerts?severity=high", tenantID)
	if got := rec3.Header().Get(apiCacheHeader); got != "MISS" {
		t.Fatalf("expected miss after mutation invalidation, got %q", got)
	}
	thirdPayload := decodeJSONBody(t, rec3)
	if got := thirdPayload["value"].(string); got != "after" {
		t.Fatalf("expected fresh value=after, got %q", got)
	}
	if getCalls != 2 {
		t.Fatalf("expected second get handler call after mutation, got %d", getCalls)
	}

	globalVersion, err := client.Get(context.Background(), apiCacheVersionGlobalKey)
	if err != nil {
		t.Fatalf("read global cache version: %v", err)
	}
	if globalVersion != "1" {
		t.Fatalf("expected global cache version 1, got %q", globalVersion)
	}
	tenantVersion, err := client.Get(context.Background(), apiCacheVersionTenantKey(tenantID))
	if err != nil {
		t.Fatalf("read tenant cache version: %v", err)
	}
	if tenantVersion != "1" {
		t.Fatalf("expected tenant cache version 1, got %q", tenantVersion)
	}
	userVersion, err := client.Get(context.Background(), apiCacheVersionUserKey(identity.UserID))
	if err != nil {
		t.Fatalf("read user cache version: %v", err)
	}
	if userVersion != "1" {
		t.Fatalf("expected user cache version 1, got %q", userVersion)
	}
}

func newTestAPICacheClient(t *testing.T) *cache.Client {
	t.Helper()
	redisServer, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	t.Cleanup(redisServer.Close)

	client, err := cache.New(context.Background(), config.RedisConfig{
		Addr: redisServer.Addr(),
		DB:   0,
	})
	if err != nil {
		t.Fatalf("create cache client: %v", err)
	}
	t.Cleanup(func() {
		_ = client.Close()
	})
	return client
}

func testContextMiddleware(identity models.Identity, tenantID uuid.UUID) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			setIdentity(c, identity)
			setTenantID(c, tenantID)
			return next(c)
		}
	}
}

func doAPICacheRequest(t *testing.T, e *echo.Echo, method, target string, tenantID uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, http.NoBody)
	req.Header.Set(config.TenantHeader, tenantID.String())
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func decodeJSONBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response body: %v body=%s", err, rec.Body.String())
	}
	return payload
}
