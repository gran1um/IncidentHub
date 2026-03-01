package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/metrics"
	"incidenthub/backend/internal/repository"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

type denyLoginLimiter struct{}

func (denyLoginLimiter) Allow(string) bool { return false }

func TestSystemResourcesUsesMaxRequestsCounter(t *testing.T) {
	env := newAPITestEnv(t)
	collector := metrics.New("incidenthub_test")
	env.handler.metrics = collector
	env.handler.cfg.AI.Enabled = true
	collector.ObserveModuleOperation("api", "request", "ok", 120*time.Millisecond)
	collector.ObserveModuleOperation("api", "request", "error", 180*time.Millisecond)
	collector.ObserveModuleOperation("ai", "ping", "ok", 5*time.Millisecond)
	collector.ObserveModuleOperation("ai", "ask", "ok", 140*time.Millisecond)

	tenantID := env.tenantID
	actorID := env.identity.UserID
	for i := 0; i < 4; i++ {
		objectID := uuid.New()
		if err := env.handler.audits.Log(context.Background(), &tenantID, &actorID, "api_request", "system_resources", &objectID, map[string]any{"kind": "db-counter"}); err != nil {
			t.Fatalf("insert audit log: %v", err)
		}
	}

	metricReq := httptest.NewRequest(http.MethodGet, "/api/v1/system/resources", http.NoBody)
	metricReq.Header.Set("X-Tenant-ID", tenantID.String())
	metricRec := httptest.NewRecorder()
	metricCtx := echo.New().NewContext(metricReq, metricRec)
	mw := collector.Middleware()(func(c *echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})
	if err := mw(metricCtx); err != nil {
		t.Fatalf("record live request metric: %v", err)
	}

	c, rec := env.jsonContext(http.MethodGet, "/api/v1/system/resources", nil)
	setIdentity(c, env.identity)
	setTenant(c, tenantID)
	if err := env.handler.SystemResources(c); err != nil {
		t.Fatalf("system resources: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	payload := decodeBody[map[string]any](t, rec)
	if gotEnv, ok := payload["env"].(string); !ok || gotEnv == "" {
		t.Fatalf("expected env in system resources payload, got %#v", payload["env"])
	}
	tenantPayload, ok := payload["tenant"].(map[string]any)
	if !ok {
		t.Fatalf("tenant payload is missing or invalid: %#v", payload["tenant"])
	}
	got, ok := tenantPayload["apiRequests24h"].(float64)
	if !ok {
		t.Fatalf("tenant.apiRequests24h is missing or invalid: %#v", tenantPayload["apiRequests24h"])
	}
	if int(got) < 4 {
		t.Fatalf("expected apiRequests24h >= 4 from db/live max, got %d", int(got))
	}

	modulesPayload, ok := payload["modules"].(map[string]any)
	if !ok {
		t.Fatalf("modules payload is missing or invalid: %#v", payload["modules"])
	}
	apiPayload, ok := modulesPayload["api"].(map[string]any)
	if !ok {
		t.Fatalf("api module payload is missing or invalid: %#v", modulesPayload["api"])
	}
	apiOperation, ok := apiPayload["operation"].(map[string]any)
	if !ok {
		t.Fatalf("api operation payload is missing or invalid: %#v", apiPayload["operation"])
	}
	if got := apiOperation["avgLatencyMs"]; got != 150.0 {
		t.Fatalf("expected api avg latency 150ms, got %#v", got)
	}

	aiPayload, ok := modulesPayload["ai_model"].(map[string]any)
	if !ok {
		t.Fatalf("ai_model payload is missing or invalid: %#v", modulesPayload["ai_model"])
	}
	aiOperation, ok := aiPayload["operation"].(map[string]any)
	if !ok {
		t.Fatalf("ai_model operation payload is missing or invalid: %#v", aiPayload["operation"])
	}
	if got := aiOperation["avgLatencyMs"]; got != 140.0 {
		t.Fatalf("expected ai avg latency to exclude ping and stay at 140ms, got %#v", got)
	}
}

func TestHandlerAuthSystemAndTenantsUsersIntegration(t *testing.T) {
	env := newAPITestEnv(t)

	{
		c, rec := env.jsonContext(http.MethodGet, "/healthz", nil)
		err := env.handler.Health(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/system/health", nil)
		err := env.handler.SystemHealth(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/me", nil)
		setIdentity(c, env.identity)
		err := env.handler.Me(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/dashboard/stats", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.GetDashboardStats(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/system/resources", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.SystemResources(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	var refreshCookie *http.Cookie
	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/auth/login", map[string]any{
			"email":    "analyst-main@example.com",
			"password": "Password123!",
		})
		c.Request().RemoteAddr = "127.0.0.1:12345"
		err := env.handler.Login(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		for _, cookie := range rec.Result().Cookies() {
			if cookie.Name == env.cfg.Auth.CookieName {
				refreshCookie = cookie
				break
			}
		}
		if refreshCookie == nil || strings.TrimSpace(refreshCookie.Value) == "" {
			t.Fatalf("login response did not set refresh cookie")
		}
	}

	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/auth/refresh", nil)
		c.Request().AddCookie(refreshCookie)
		err := env.handler.Refresh(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/auth/logout", nil)
		c.Request().AddCookie(refreshCookie)
		err := env.handler.Logout(c)
		mustStatusOK(t, err, rec, http.StatusNoContent)
	}

	var createdTenantID uuid.UUID
	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/tenants", map[string]any{
			"slug":        "tenant-created",
			"name":        "Tenant Created",
			"description": "created in test",
			"max_users":   25,
		})
		setIdentity(c, env.platformAdmin)
		err := env.handler.CreateTenant(c)
		mustStatusOK(t, err, rec, http.StatusCreated)
		payload := decodeBody[map[string]any](t, rec)
		idRaw, _ := payload["id"].(string)
		parsedID, parseErr := uuid.Parse(strings.TrimSpace(idRaw))
		if parseErr != nil {
			t.Fatalf("parse tenant id from response: %v", parseErr)
		}
		createdTenantID = parsedID
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/tenants", nil)
		setIdentity(c, env.platformAdmin)
		err := env.handler.ListTenants(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/tenants", nil)
		setIdentity(c, env.identity)
		err := env.handler.ListTenants(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	{
		c, rec := env.jsonContext(http.MethodPatch, "/api/v1/tenants/"+createdTenantID.String(), map[string]any{
			"name":        "Tenant Created Updated",
			"description": "updated",
			"is_active":   true,
		})
		setPath(c, "/api/v1/tenants/:tenantID", []string{"tenantID"}, []string{createdTenantID.String()})
		setIdentity(c, env.platformAdmin)
		err := env.handler.UpdateTenant(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	var createdUserID uuid.UUID
	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/users", map[string]any{
			"username":          "new-user",
			"email":             "new-user@example.com",
			"full_name":         "New User",
			"password":          "Password123!",
			"tenant_id":         env.tenantID.String(),
			"role":              "tenant_admin",
			"is_platform_admin": false,
		})
		setIdentity(c, env.platformAdmin)
		err := env.handler.CreateUser(c)
		mustStatusOK(t, err, rec, http.StatusCreated)
		payload := decodeBody[map[string]any](t, rec)
		idRaw, _ := payload["id"].(string)
		parsedID, parseErr := uuid.Parse(strings.TrimSpace(idRaw))
		if parseErr != nil {
			t.Fatalf("parse user id from response: %v", parseErr)
		}
		createdUserID = parsedID
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/users", nil)
		setIdentity(c, env.platformAdmin)
		err := env.handler.ListUsers(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/users?tenant_id="+env.tenantID.String(), nil)
		setIdentity(c, env.identity)
		c.Request().URL.RawQuery = "tenant_id=" + env.tenantID.String()
		err := env.handler.ListUsers(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/tenant-users", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListTenantUsers(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/users/"+env.userID.String(), nil)
		setPath(c, "/api/v1/users/:id", []string{"id"}, []string{env.userID.String()})
		setIdentity(c, env.identity)
		err := env.handler.GetUser(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/users/"+createdUserID.String(), nil)
		setPath(c, "/api/v1/users/:id", []string{"id"}, []string{createdUserID.String()})
		setIdentity(c, env.identity)
		c.Request().Header.Set(config.TenantHeader, env.tenantID.String())
		err := env.handler.GetUser(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	{
		c, rec := env.jsonContext(http.MethodPatch, "/api/v1/users/"+env.userID.String(), map[string]any{
			"full_name":        "Main Analyst Updated",
			"team":             "SOC Alpha",
			"avatar_url":       "https://example.local/avatar.png",
			"cover_image":      "https://example.local/cover.png",
			"personal_link":    "https://example.local",
			"password":         "Password123!9",
			"current_password": "Password123!",
		})
		setPath(c, "/api/v1/users/:id", []string{"id"}, []string{env.userID.String()})
		setIdentity(c, env.identity)
		err := env.handler.UpdateUser(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	{
		c, _ := env.jsonContext(http.MethodPost, "/api/v1/auth/login", map[string]any{
			"email": "missing-password@example.com",
		})
		err := env.handler.Login(c)
		if err == nil {
			t.Fatalf("expected login validation error")
		}
	}

	{
		originalLimiter := env.handler.loginLimiter
		env.handler.loginLimiter = denyLoginLimiter{}
		defer func() { env.handler.loginLimiter = originalLimiter }()

		c, _ := env.jsonContext(http.MethodPost, "/api/v1/auth/login", map[string]any{
			"email":    "analyst-main@example.com",
			"password": "Password123!",
		})
		c.Request().RemoteAddr = "127.0.0.1:12345"
		err := env.handler.Login(c)
		if err == nil {
			t.Fatalf("expected rate-limited login error")
		}
	}

	{
		if _, err := env.pool.Exec(context.Background(), `UPDATE users SET is_active = FALSE WHERE id = $1`, env.userID); err != nil {
			t.Fatalf("disable user for login test: %v", err)
		}
		c, _ := env.jsonContext(http.MethodPost, "/api/v1/auth/login", map[string]any{
			"email":    "analyst-main@example.com",
			"password": "Password123!",
		})
		err := env.handler.Login(c)
		if err == nil {
			t.Fatalf("expected disabled user login error")
		}
		if _, err := env.pool.Exec(context.Background(), `UPDATE users SET is_active = TRUE WHERE id = $1`, env.userID); err != nil {
			t.Fatalf("re-enable user for login test: %v", err)
		}
	}

	{
		ldapUser, err := env.users.Create(context.Background(), repository.CreateUserParams{
			Username:        "ldap-user",
			Email:           "ldap-user@example.com",
			FullName:        "LDAP User",
			PasswordHash:    "",
			LDAPEnabled:     true,
			IsPlatformAdmin: false,
		})
		if err != nil {
			t.Fatalf("create ldap user: %v", err)
		}
		if upsertErr := env.memberships.Upsert(context.Background(), env.tenantID, ldapUser.ID, "analyst"); upsertErr != nil {
			t.Fatalf("assign ldap user membership: %v", upsertErr)
		}
		c, _ := env.jsonContext(http.MethodPost, "/api/v1/auth/login", map[string]any{
			"email":    "ldap-user@example.com",
			"password": "any-password",
		})
		err = env.handler.Login(c)
		if err == nil {
			t.Fatalf("expected ldap login failure when ldap is not configured")
		}
	}
}
