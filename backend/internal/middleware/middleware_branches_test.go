package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"incidenthub/backend/internal/auth"
	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/metrics"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"
	"incidenthub/backend/internal/security"
	"incidenthub/backend/internal/testutil"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

func newMiddlewareContext(method, target string) (*echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(method, target, http.NoBody)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	return c, rec
}

func TestContextHelpers(t *testing.T) {
	c, _ := newMiddlewareContext(http.MethodGet, "/")

	if _, ok := GetIdentity(c); ok {
		t.Fatalf("identity should be missing by default")
	}
	if _, ok := GetTenantID(c); ok {
		t.Fatalf("tenant should be missing by default")
	}

	tenantID := uuid.New()
	id := models.Identity{UserID: uuid.New(), Username: "analyst"}
	setIdentity(c, id)
	setTenantID(c, tenantID)

	gotID, ok := GetIdentity(c)
	if !ok || gotID.UserID != id.UserID {
		t.Fatalf("unexpected identity helper result")
	}
	gotTenant, ok := GetTenantID(c)
	if !ok || gotTenant != tenantID {
		t.Fatalf("unexpected tenant helper result")
	}
}

func TestJWTAuthMiddlewareBranches(t *testing.T) {
	jwtSvc := auth.New(config.AuthConfig{
		JWTSecret:  "test-secret",
		Issuer:     "test-issuer",
		AccessTTL:  time.Minute,
		RefreshTTL: time.Hour,
	})
	metric := metrics.New("ih_test")
	mw := JWTAuth(jwtSvc, nil, metric)
	nextCalled := false
	next := func(c *echo.Context) error {
		nextCalled = true
		_, ok := GetIdentity(c)
		if !ok {
			t.Fatalf("identity must be set for valid token")
		}
		return c.NoContent(http.StatusNoContent)
	}

	{
		c, _ := newMiddlewareContext(http.MethodGet, "/")
		err := mw(next)(c)
		if code := httpErrorCode(t, err); code != http.StatusUnauthorized {
			t.Fatalf("expected missing auth header 401, got %d", code)
		}
	}
	{
		c, _ := newMiddlewareContext(http.MethodGet, "/")
		c.Request().Header.Set("Authorization", "Token abc")
		err := mw(next)(c)
		if code := httpErrorCode(t, err); code != http.StatusUnauthorized {
			t.Fatalf("expected invalid auth header 401, got %d", code)
		}
	}
	{
		c, _ := newMiddlewareContext(http.MethodGet, "/")
		c.Request().Header.Set("Authorization", "Bearer invalid-token")
		err := mw(next)(c)
		if code := httpErrorCode(t, err); code != http.StatusUnauthorized {
			t.Fatalf("expected invalid jwt 401, got %d", code)
		}
	}
	{
		tokens, err := jwtSvc.Generate(models.Identity{UserID: uuid.New(), Username: "analyst"})
		if err != nil {
			t.Fatalf("generate jwt: %v", err)
		}
		c, rec := newMiddlewareContext(http.MethodGet, "/")
		c.Request().Header.Set("Authorization", "Bearer "+tokens.AccessToken)
		err = mw(next)(c)
		if err != nil {
			t.Fatalf("valid jwt middleware returned error: %v", err)
		}
		if rec.Code != http.StatusNoContent {
			t.Fatalf("unexpected status for valid jwt: %d", rec.Code)
		}
		if !nextCalled {
			t.Fatalf("next handler must be called")
		}
	}
}

func TestJWTAuthMiddlewareAcceptsAPIToken(t *testing.T) {
	pool := testutil.OpenTestPool(t)
	testutil.ResetPublicTables(t, pool)
	ctx := context.Background()

	tenants := repository.NewTenantRepository(pool)
	users := repository.NewUserRepository(pool)
	apiTokens := repository.NewAPIAccessTokenRepository(pool)

	tenant, err := tenants.Create(ctx, repository.CreateTenantParams{
		Slug:        "jwt-api-token-tenant",
		Name:        "JWT API Token Tenant",
		Description: "tenant",
		MaxUsers:    10,
		IsActive:    true,
	})
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	passwordHash, err := security.HashPassword("Password123!")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	user, err := users.Create(ctx, repository.CreateUserParams{
		Username:        "jwt-api-token-user",
		Email:           "jwt-api-token-user@example.com",
		FullName:        "JWT API Token User",
		PasswordHash:    passwordHash,
		IsPlatformAdmin: false,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	plainToken := "ihat_test_token_123"
	digest := sha256.Sum256([]byte(plainToken))
	tokenHash := hex.EncodeToString(digest[:])
	_, err = apiTokens.Create(ctx, repository.CreateAPIAccessTokenParams{
		TenantID:    tenant.ID,
		Name:        "integration",
		Description: "integration token",
		TokenHash:   tokenHash,
		TokenPrefix: "ihat_test",
		Scopes:      []string{"alerts:read"},
		CreatedBy:   user.ID,
	})
	if err != nil {
		t.Fatalf("create api token: %v", err)
	}

	jwtSvc := auth.New(config.AuthConfig{
		JWTSecret:  "test-secret",
		Issuer:     "test-issuer",
		AccessTTL:  time.Minute,
		RefreshTTL: time.Hour,
	})
	mw := JWTAuth(jwtSvc, apiTokens, metrics.New("ih_test_token"))
	nextCalled := false
	next := func(c *echo.Context) error {
		nextCalled = true
		id, ok := GetIdentity(c)
		if !ok {
			t.Fatalf("identity must be set")
		}
		if id.AuthType != models.IdentityAuthTypeAPIToken {
			t.Fatalf("expected api_token auth type, got %q", id.AuthType)
		}
		if id.TenantID == nil || *id.TenantID != tenant.ID {
			t.Fatalf("unexpected tenant in identity")
		}
		if id.UserID != user.ID {
			t.Fatalf("unexpected user id in identity")
		}
		return c.NoContent(http.StatusNoContent)
	}

	c, rec := newMiddlewareContext(http.MethodGet, "/")
	c.Request().Header.Set("Authorization", "Bearer "+plainToken)
	if err := mw(next)(c); err != nil {
		t.Fatalf("jwt middleware with api token returned error: %v", err)
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if !nextCalled {
		t.Fatalf("next handler must be called for valid api token")
	}
}

func TestRBACMiddlewares(t *testing.T) {
	next := func(c *echo.Context) error { return c.NoContent(http.StatusNoContent) }

	{
		c, _ := newMiddlewareContext(http.MethodGet, "/")
		err := RequireAuthenticated()(next)(c)
		if code := httpErrorCode(t, err); code != http.StatusUnauthorized {
			t.Fatalf("RequireAuthenticated expected 401, got %d", code)
		}
	}
	{
		c, rec := newMiddlewareContext(http.MethodGet, "/")
		setIdentity(c, models.Identity{UserID: uuid.New(), Username: "u"})
		if err := RequireAuthenticated()(next)(c); err != nil {
			t.Fatalf("RequireAuthenticated valid identity failed: %v", err)
		}
		if rec.Code != http.StatusNoContent {
			t.Fatalf("unexpected status: %d", rec.Code)
		}
	}

	{
		c, _ := newMiddlewareContext(http.MethodGet, "/")
		setIdentity(c, models.Identity{UserID: uuid.New(), Username: "u"})
		err := RequirePlatformAdmin()(next)(c)
		if code := httpErrorCode(t, err); code != http.StatusForbidden {
			t.Fatalf("RequirePlatformAdmin expected 403, got %d", code)
		}
	}
	{
		c, rec := newMiddlewareContext(http.MethodGet, "/")
		setIdentity(c, models.Identity{UserID: uuid.New(), Username: "u", IsPlatformAdmin: true})
		if err := RequirePlatformAdmin()(next)(c); err != nil {
			t.Fatalf("RequirePlatformAdmin platform user failed: %v", err)
		}
		if rec.Code != http.StatusNoContent {
			t.Fatalf("unexpected status: %d", rec.Code)
		}
	}

	{
		c, _ := newMiddlewareContext(http.MethodGet, "/")
		setIdentity(c, models.Identity{UserID: uuid.New(), Username: "u", TenantRole: models.TenantRoleAnalyst})
		err := RequireTenantAdminOrPlatformAdmin()(next)(c)
		if code := httpErrorCode(t, err); code != http.StatusForbidden {
			t.Fatalf("RequireTenantAdminOrPlatformAdmin expected 403, got %d", code)
		}
	}
	{
		c, rec := newMiddlewareContext(http.MethodGet, "/")
		setIdentity(c, models.Identity{UserID: uuid.New(), Username: "u", TenantRole: models.TenantRoleAdmin})
		if err := RequireTenantAdminOrPlatformAdmin()(next)(c); err != nil {
			t.Fatalf("RequireTenantAdminOrPlatformAdmin tenant admin failed: %v", err)
		}
		if rec.Code != http.StatusNoContent {
			t.Fatalf("unexpected status: %d", rec.Code)
		}
	}

	{
		c, _ := newMiddlewareContext(http.MethodGet, "/")
		setIdentity(c, models.Identity{UserID: uuid.New(), Username: "u", TenantRole: models.TenantRoleViewer})
		err := RequireAnalystOrHigher()(next)(c)
		if code := httpErrorCode(t, err); code != http.StatusForbidden {
			t.Fatalf("RequireAnalystOrHigher expected 403, got %d", code)
		}
	}
	{
		c, rec := newMiddlewareContext(http.MethodGet, "/")
		setIdentity(c, models.Identity{UserID: uuid.New(), Username: "u", TenantRole: models.TenantRoleAnalyst})
		if err := RequireAnalystOrHigher()(next)(c); err != nil {
			t.Fatalf("RequireAnalystOrHigher analyst failed: %v", err)
		}
		if rec.Code != http.StatusNoContent {
			t.Fatalf("unexpected status: %d", rec.Code)
		}
	}
}

func TestSecurityHeadersMiddleware(t *testing.T) {
	next := func(c *echo.Context) error { return c.NoContent(http.StatusNoContent) }
	c, rec := newMiddlewareContext(http.MethodGet, "/")
	if err := SecurityHeaders()(next)(c); err != nil {
		t.Fatalf("security headers middleware error: %v", err)
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("missing X-Frame-Options header")
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("missing X-Content-Type-Options header")
	}
	if rec.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("missing Referrer-Policy header")
	}
	if rec.Header().Get("Content-Security-Policy") == "" {
		t.Fatalf("missing Content-Security-Policy header")
	}

	swaggerCtx, swaggerRec := newMiddlewareContext(http.MethodGet, "/dev/swagger")
	if err := SecurityHeaders()(next)(swaggerCtx); err != nil {
		t.Fatalf("security headers middleware error for swagger: %v", err)
	}
	if !strings.Contains(swaggerRec.Header().Get("Content-Security-Policy"), "unpkg.com") {
		t.Fatalf("swagger csp must allow unpkg assets")
	}
}

func TestResolveTenantMiddlewareBranches(t *testing.T) {
	pool := testutil.OpenTestPool(t)
	testutil.ResetPublicTables(t, pool)

	tenants := repository.NewTenantRepository(pool)
	users := repository.NewUserRepository(pool)
	memberships := repository.NewMembershipRepository(pool)

	tenant, err := tenants.Create(context.Background(), repository.CreateTenantParams{
		Slug:        "mw-tenant",
		Name:        "Middleware Tenant",
		Description: "mw",
		MaxUsers:    10,
		IsActive:    true,
	})
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	secondaryTenant, err := tenants.Create(context.Background(), repository.CreateTenantParams{
		Slug:        "mw-tenant-2",
		Name:        "Middleware Tenant 2",
		Description: "mw2",
		MaxUsers:    10,
		IsActive:    true,
	})
	if err != nil {
		t.Fatalf("create secondary tenant: %v", err)
	}

	passwordHash, err := security.HashPassword("Password123!")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	user, err := users.Create(context.Background(), repository.CreateUserParams{
		Username:        "mw-user",
		Email:           "mw-user@example.com",
		FullName:        "MW User",
		PasswordHash:    passwordHash,
		IsPlatformAdmin: false,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	if err := memberships.Upsert(context.Background(), tenant.ID, user.ID, models.TenantRoleAnalyst); err != nil {
		t.Fatalf("upsert membership: %v", err)
	}
	if err := memberships.Upsert(context.Background(), secondaryTenant.ID, user.ID, models.TenantRoleAnalyst); err != nil {
		t.Fatalf("upsert secondary membership: %v", err)
	}

	cfg := config.App{}
	next := func(c *echo.Context) error { return c.NoContent(http.StatusNoContent) }
	mw := ResolveTenant(cfg, memberships, tenants)

	{
		c, _ := newMiddlewareContext(http.MethodGet, "/")
		err := mw(next)(c)
		if code := httpErrorCode(t, err); code != http.StatusUnauthorized {
			t.Fatalf("ResolveTenant expected 401 without identity, got %d", code)
		}
	}
	{
		c, _ := newMiddlewareContext(http.MethodGet, "/")
		setIdentity(c, models.Identity{UserID: user.ID, Username: user.Username})
		err := mw(next)(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("ResolveTenant expected tenant header required, got %d", code)
		}
	}
	{
		c, rec := newMiddlewareContext(http.MethodGet, "/")
		setIdentity(c, models.Identity{UserID: uuid.New(), Username: "platform", IsPlatformAdmin: true})
		if err := mw(next)(c); err != nil {
			t.Fatalf("ResolveTenant platform-admin branch failed: %v", err)
		}
		if rec.Code != http.StatusNoContent {
			t.Fatalf("unexpected platform-admin status: %d", rec.Code)
		}
	}
	{
		c, rec := newMiddlewareContext(http.MethodGet, "/")
		setIdentity(c, models.Identity{
			UserID:     user.ID,
			Username:   "api-token:tenant",
			AuthType:   models.IdentityAuthTypeAPIToken,
			TenantID:   &tenant.ID,
			TenantRole: models.TenantRoleAdmin,
		})
		c.Request().Header.Set(config.TenantHeader, tenant.ID.String())
		if err := mw(next)(c); err != nil {
			t.Fatalf("ResolveTenant api-token branch failed: %v", err)
		}
		if rec.Code != http.StatusNoContent {
			t.Fatalf("unexpected api-token status: %d", rec.Code)
		}
	}
	{
		c, _ := newMiddlewareContext(http.MethodGet, "/")
		setIdentity(c, models.Identity{
			UserID:     user.ID,
			Username:   "api-token:tenant",
			AuthType:   models.IdentityAuthTypeAPIToken,
			TenantID:   &tenant.ID,
			TenantRole: models.TenantRoleAdmin,
		})
		c.Request().Header.Set(config.TenantHeader, secondaryTenant.ID.String())
		err := mw(next)(c)
		if code := httpErrorCode(t, err); code != http.StatusForbidden {
			t.Fatalf("ResolveTenant expected tenant mismatch forbidden 403 for api-token, got %d", code)
		}
	}
	{
		c, _ := newMiddlewareContext(http.MethodGet, "/")
		setIdentity(c, models.Identity{UserID: user.ID, Username: user.Username})
		c.Request().Header.Set(config.TenantHeader, "bad-uuid")
		err := mw(next)(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("ResolveTenant expected invalid tenant id 400, got %d", code)
		}
	}
	{
		c, _ := newMiddlewareContext(http.MethodGet, "/")
		setIdentity(c, models.Identity{UserID: user.ID, Username: user.Username, TenantID: &tenant.ID})
		c.Request().Header.Set(config.TenantHeader, secondaryTenant.ID.String())
		err := mw(next)(c)
		if code := httpErrorCode(t, err); code != http.StatusForbidden {
			t.Fatalf("ResolveTenant expected tenant switch forbidden 403, got %d", code)
		}
	}
	{
		c, _ := newMiddlewareContext(http.MethodGet, "/")
		setIdentity(c, models.Identity{UserID: uuid.New(), Username: "other"})
		c.Request().Header.Set(config.TenantHeader, tenant.ID.String())
		err := mw(next)(c)
		if code := httpErrorCode(t, err); code != http.StatusForbidden {
			t.Fatalf("ResolveTenant expected membership forbidden 403, got %d", code)
		}
	}
	{
		c, rec := newMiddlewareContext(http.MethodGet, "/")
		setIdentity(c, models.Identity{UserID: user.ID, Username: user.Username})
		c.Request().Header.Set(config.TenantHeader, tenant.ID.String())
		if err := mw(next)(c); err != nil {
			t.Fatalf("ResolveTenant active membership failed: %v", err)
		}
		if rec.Code != http.StatusNoContent {
			t.Fatalf("unexpected active membership status: %d", rec.Code)
		}
		id, ok := GetIdentity(c)
		if !ok || id.TenantRole != models.TenantRoleAnalyst || id.TenantID == nil || *id.TenantID != tenant.ID {
			t.Fatalf("identity should be updated with tenant role")
		}
		if gotTenant, ok := GetTenantID(c); !ok || gotTenant != tenant.ID {
			t.Fatalf("tenant context should be set")
		}
	}
	{
		inactiveTenant, err := tenants.Create(context.Background(), repository.CreateTenantParams{
			Slug:        "mw-tenant-inactive",
			Name:        "Middleware Tenant Inactive",
			Description: "mw inactive",
			MaxUsers:    10,
			IsActive:    true,
		})
		if err != nil {
			t.Fatalf("create inactive tenant: %v", err)
		}
		isActive := false
		if _, updateErr := tenants.Update(context.Background(), inactiveTenant.ID, repository.UpdateTenantParams{IsActive: &isActive}); updateErr != nil {
			t.Fatalf("disable inactive tenant: %v", updateErr)
		}
		if upsertErr := memberships.Upsert(context.Background(), inactiveTenant.ID, user.ID, models.TenantRoleAnalyst); upsertErr != nil {
			t.Fatalf("upsert inactive tenant membership: %v", upsertErr)
		}

		c, _ := newMiddlewareContext(http.MethodGet, "/")
		setIdentity(c, models.Identity{UserID: user.ID, Username: user.Username})
		c.Request().Header.Set(config.TenantHeader, inactiveTenant.ID.String())
		err = mw(next)(c)
		if code := httpErrorCode(t, err); code != http.StatusLocked {
			t.Fatalf("ResolveTenant expected inactive tenant to return 423, got %d", code)
		}
	}
}

func TestAPITokenAccessControlBranches(t *testing.T) {
	next := func(c *echo.Context) error { return c.NoContent(http.StatusNoContent) }
	mw := APITokenAccessControl()

	{
		c, _ := newMiddlewareContext(http.MethodGet, "/api/v1/users")
		c.SetPath("/api/v1/users")
		setIdentity(c, models.Identity{AuthType: models.IdentityAuthTypeAPIToken, APITokenFullAccess: true})
		err := mw(next)(c)
		if code := httpErrorCode(t, err); code != http.StatusForbidden {
			t.Fatalf("expected 403 for user-account route, got %d", code)
		}
	}
	{
		c, _ := newMiddlewareContext(http.MethodPost, "/api/v1/auth/logout")
		c.SetPath("/api/v1/auth/logout")
		setIdentity(c, models.Identity{AuthType: models.IdentityAuthTypeAPIToken, APITokenFullAccess: true})
		err := mw(next)(c)
		if code := httpErrorCode(t, err); code != http.StatusForbidden {
			t.Fatalf("expected 403 for logout route, got %d", code)
		}
	}
	{
		c, rec := newMiddlewareContext(http.MethodGet, "/api/v1/alerts")
		c.SetPath("/api/v1/alerts")
		setIdentity(c, models.Identity{AuthType: models.IdentityAuthTypeAPIToken, APITokenFullAccess: true})
		if err := mw(next)(c); err != nil {
			t.Fatalf("full access api token should pass: %v", err)
		}
		if rec.Code != http.StatusNoContent {
			t.Fatalf("unexpected full access status: %d", rec.Code)
		}
	}
	{
		c, rec := newMiddlewareContext(http.MethodGet, "/api/v1/alerts")
		c.SetPath("/api/v1/alerts")
		setIdentity(c, models.Identity{AuthType: models.IdentityAuthTypeAPIToken, APITokenScopes: []string{"alerts:write"}})
		if err := mw(next)(c); err != nil {
			t.Fatalf("write scope should imply read: %v", err)
		}
		if rec.Code != http.StatusNoContent {
			t.Fatalf("unexpected status: %d", rec.Code)
		}
	}
	{
		c, _ := newMiddlewareContext(http.MethodPost, "/api/v1/cases")
		c.SetPath("/api/v1/cases")
		setIdentity(c, models.Identity{AuthType: models.IdentityAuthTypeAPIToken, APITokenScopes: []string{"alerts:write"}})
		err := mw(next)(c)
		if code := httpErrorCode(t, err); code != http.StatusForbidden {
			t.Fatalf("expected 403 for missing scope, got %d", code)
		}
	}
	{
		c, rec := newMiddlewareContext(http.MethodPost, "/api/v1/users/abc/experience-awards")
		c.SetPath("/api/v1/users/:id/experience-awards")
		setIdentity(c, models.Identity{AuthType: models.IdentityAuthTypeAPIToken, APITokenScopes: []string{"experience:write"}})
		if err := mw(next)(c); err != nil {
			t.Fatalf("experience scope should allow awards endpoint: %v", err)
		}
		if rec.Code != http.StatusNoContent {
			t.Fatalf("unexpected status for experience endpoint: %d", rec.Code)
		}
	}
}

func httpErrorCode(t *testing.T, err error) int {
	t.Helper()
	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected *echo.HTTPError, got %T (%v)", err, err)
	}
	return httpErr.Code
}
