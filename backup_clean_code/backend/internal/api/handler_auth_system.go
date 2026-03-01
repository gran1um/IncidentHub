package api

import (
	"context"
	"errors"
	"incidenthub/backend/internal/auth"
	"incidenthub/backend/internal/middleware"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/security"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v5"
)

func (h *Handler) Health(c *echo.Context) error {
	status, modules := h.collectModulesHealth(c.Request().Context())
	return c.JSON(http.StatusOK, map[string]any{
		"status":        status,
		"time":          time.Now().UTC().Format(time.RFC3339),
		"modules":       modules,
		"uptimeSeconds": int(time.Since(h.startedAt).Seconds()),
	})
}

func (h *Handler) Me(c *echo.Context) error {
	identity, ok := middleware.GetIdentity(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}
	memberships, err := h.memberships.ListByUser(c.Request().Context(), identity.UserID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load memberships")
	}
	return c.JSON(http.StatusOK, map[string]any{"identity": identity, "memberships": memberships})
}

func (h *Handler) SystemHealth(c *echo.Context) error {
	status, modules := h.collectModulesHealth(c.Request().Context())
	return c.JSON(http.StatusOK, map[string]any{
		"env":           strings.ToLower(strings.TrimSpace(h.cfg.Env)),
		"status":        status,
		"timestamp":     time.Now().UTC().Format(time.RFC3339),
		"modules":       modules,
		"uptimeSeconds": int(time.Since(h.startedAt).Seconds()),
	})
}

func (h *Handler) GetDashboardStats(c *echo.Context) error {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	if h.system == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "system repository unavailable")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	stats, err := h.system.TenantDashboardStats(ctx, tenantID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load dashboard stats")
	}
	return c.JSON(http.StatusOK, stats)
}

func (h *Handler) SystemResources(c *echo.Context) error {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	if h.system == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "system repository unavailable")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	status, modules := h.collectModulesHealth(ctx)
	dashboard, err := h.system.TenantDashboardStats(ctx, tenantID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load dashboard stats")
	}
	resources, err := h.system.TenantResourceStats(ctx, tenantID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load resource stats")
	}
	if h.metrics != nil {
		liveRequests := int(h.metrics.RequestsLast24h(tenantID.String()))
		if liveRequests > resources.APIRequests24h {
			resources.APIRequests24h = liveRequests
		}
	}
	host := h.collectHostResources()

	return c.JSON(http.StatusOK, map[string]any{
		"env":           strings.ToLower(strings.TrimSpace(h.cfg.Env)),
		"status":        status,
		"timestamp":     time.Now().UTC().Format(time.RFC3339),
		"modules":       modules,
		"host":          host,
		"postgresPool":  h.system.PoolStats(),
		"dashboard":     dashboard,
		"tenant":        resources,
		"uptimeSeconds": int(time.Since(h.startedAt).Seconds()),
	})
}

func (h *Handler) Login(c *echo.Context) error {
	var req loginRequest
	if err := c.Bind(&req); err != nil {
		h.authAttempt("bad_request")
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || strings.TrimSpace(req.Password) == "" {
		h.authAttempt("bad_request")
		return echo.NewHTTPError(http.StatusBadRequest, "email and password are required")
	}
	if h.loginLimiter != nil && !h.loginLimiter.Allow(c.RealIP()+":"+req.Email) {
		h.authAttempt("rate_limited")
		return echo.NewHTTPError(http.StatusTooManyRequests, "login rate limit exceeded")
	}

	user, err := h.users.GetByEmail(c.Request().Context(), req.Email)
	if err != nil {
		h.authAttempt("invalid_credentials")
		if errors.Is(err, pgx.ErrNoRows) {
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid credentials")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load user")
	}
	if !user.IsActive {
		h.authAttempt("inactive_user")
		return echo.NewHTTPError(http.StatusForbidden, "user disabled")
	}

	if user.LDAPEnabled {
		if h.ldap == nil {
			h.authAttempt("ldap_unavailable")
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid credentials")
		}
		if ldapErr := h.ldap.Authenticate(c.Request().Context(), user.Username, req.Password); ldapErr != nil {
			h.authAttempt("ldap_invalid")
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid credentials")
		}
	} else {
		ok, verifyErr := security.VerifyPassword(req.Password, user.PasswordHash)
		if verifyErr != nil {
			h.authAttempt("verify_error")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to verify credentials")
		}
		if !ok {
			h.authAttempt("invalid_credentials")
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid credentials")
		}
	}

	memberships, err := h.memberships.ListByUser(c.Request().Context(), user.ID)
	if err != nil {
		h.authAttempt("membership_error")
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load memberships")
	}

	identity := models.Identity{UserID: user.ID, Username: user.Username, IsPlatformAdmin: user.IsPlatformAdmin}
	if len(memberships) > 0 {
		identity.TenantID = &memberships[0].TenantID
		identity.TenantRole = memberships[0].Role
	}

	tokens, err := h.jwt.Generate(identity)
	if err != nil {
		h.authAttempt("token_error")
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate token")
	}

	if err := h.refresh.Create(c.Request().Context(), models.RefreshToken{
		UserID:    user.ID,
		TokenHash: auth.RefreshHash(tokens.RefreshToken),
		ExpiresAt: tokens.RefreshTokenExpiresAt,
		UserAgent: c.Request().UserAgent(),
		IPAddress: c.RealIP(),
	}); err != nil {
		h.authAttempt("refresh_store_error")
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to issue session")
	}

	h.setRefreshCookie(c, tokens.RefreshToken, tokens.RefreshTokenExpiresAt)
	_ = h.users.TouchLogin(c.Request().Context(), user.ID)
	_ = h.audits.Log(c.Request().Context(), nil, &user.ID, "login", "session", nil, map[string]any{"ip": c.RealIP()})
	h.authAttempt("success")

	return c.JSON(http.StatusOK, loginResponse{
		AccessToken:           tokens.AccessToken,
		AccessTokenExpiresAt:  tokens.AccessTokenExpiresAt.Format(time.RFC3339),
		RefreshTokenExpiresAt: tokens.RefreshTokenExpiresAt.Format(time.RFC3339),
		Identity:              identity,
		Memberships:           memberships,
	})
}

func (h *Handler) Refresh(c *echo.Context) error {
	cookie, err := c.Cookie(h.cfg.Auth.CookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "refresh cookie required")
	}

	record, err := h.refresh.GetActiveByHash(c.Request().Context(), auth.RefreshHash(cookie.Value))
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid refresh token")
	}

	user, err := h.users.GetByID(c.Request().Context(), record.UserID)
	if err != nil || !user.IsActive {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid refresh token")
	}

	memberships, _ := h.memberships.ListByUser(c.Request().Context(), user.ID)
	identity := models.Identity{UserID: user.ID, Username: user.Username, IsPlatformAdmin: user.IsPlatformAdmin}
	if len(memberships) > 0 {
		identity.TenantID = &memberships[0].TenantID
		identity.TenantRole = memberships[0].Role
	}

	tokens, err := h.jwt.Generate(identity)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate token")
	}
	_ = h.refresh.RevokeByHash(c.Request().Context(), auth.RefreshHash(cookie.Value))
	if err := h.refresh.Create(c.Request().Context(), models.RefreshToken{
		UserID:    user.ID,
		TokenHash: auth.RefreshHash(tokens.RefreshToken),
		ExpiresAt: tokens.RefreshTokenExpiresAt,
		UserAgent: c.Request().UserAgent(),
		IPAddress: c.RealIP(),
	}); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to rotate session")
	}

	h.setRefreshCookie(c, tokens.RefreshToken, tokens.RefreshTokenExpiresAt)
	return c.JSON(http.StatusOK, loginResponse{
		AccessToken:           tokens.AccessToken,
		AccessTokenExpiresAt:  tokens.AccessTokenExpiresAt.Format(time.RFC3339),
		RefreshTokenExpiresAt: tokens.RefreshTokenExpiresAt.Format(time.RFC3339),
		Identity:              identity,
		Memberships:           memberships,
	})
}

func (h *Handler) Logout(c *echo.Context) error {
	cookie, _ := c.Cookie(h.cfg.Auth.CookieName)
	if cookie != nil && strings.TrimSpace(cookie.Value) != "" {
		_ = h.refresh.RevokeByHash(c.Request().Context(), auth.RefreshHash(cookie.Value))
	}
	h.setRefreshCookie(c, "", time.Unix(0, 0))
	return c.NoContent(http.StatusNoContent)
}
