package api

import (
	"incidenthub/backend/internal/auth"
	"incidenthub/backend/internal/middleware"
	"incidenthub/backend/internal/models"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
)

const (
	refreshSessionStatusActive  = "active"
	refreshSessionStatusExpired = "expired"
	refreshSessionStatusRevoked = "revoked"
)

func (h *Handler) ListAuthSessions(c *echo.Context) error {
	identity, ok := middleware.GetIdentity(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}

	limit := parseSessionListLimit(c.QueryParam("limit"))
	items, err := h.refresh.ListByUser(c.Request().Context(), identity.UserID, limit)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load sessions")
	}

	now := time.Now().UTC()
	currentTokenHash := h.currentRefreshHash(c)
	responseItems := make([]authSessionResponse, 0, len(items))
	activeSessions := 0

	for _, item := range items {
		status := refreshSessionStatus(item, now)
		if status == refreshSessionStatusActive {
			activeSessions++
		}

		isCurrent := currentTokenHash != "" && item.TokenHash == currentTokenHash
		var revokedAt *string
		if item.RevokedAt != nil {
			value := item.RevokedAt.UTC().Format(time.RFC3339)
			revokedAt = &value
		}

		responseItems = append(responseItems, authSessionResponse{
			ID:               item.ID.String(),
			UserAgent:        item.UserAgent,
			IPAddress:        item.IPAddress,
			CreatedAt:        item.CreatedAt.UTC().Format(time.RFC3339),
			ExpiresAt:        item.ExpiresAt.UTC().Format(time.RFC3339),
			RevokedAt:        revokedAt,
			Status:           status,
			IsCurrent:        isCurrent,
			DurationSeconds:  sessionDurationSeconds(item, now),
			RemainingSeconds: sessionRemainingSeconds(item, now),
		})
	}

	return c.JSON(http.StatusOK, listAuthSessionsResponse{
		Sessions:              responseItems,
		ActiveSessions:        activeSessions,
		SessionTimeoutSeconds: maxInt64(int64(h.cfg.Auth.RefreshTTL.Seconds()), 0),
	})
}

func (h *Handler) RevokeOtherAuthSessions(c *echo.Context) error {
	identity, ok := middleware.GetIdentity(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}

	currentTokenHash, err := h.requireActiveRefreshHash(c)
	if err != nil {
		return err
	}

	revokedCount, err := h.refresh.RevokeAllByUserExceptHash(c.Request().Context(), identity.UserID, currentTokenHash)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to revoke sessions")
	}

	if h.audits != nil {
		_ = h.audits.Log(c.Request().Context(), nil, &identity.UserID, "revoke_other_sessions", "session", nil, map[string]any{
			"revoked_count": revokedCount,
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"revoked": revokedCount,
	})
}

func parseSessionListLimit(raw string) int {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 25
	}
	value, err := strconv.Atoi(trimmed)
	if err != nil || value <= 0 {
		return 25
	}
	if value > 200 {
		return 200
	}
	return value
}

func (h *Handler) currentRefreshHash(c *echo.Context) string {
	cookie, err := c.Cookie(h.cfg.Auth.CookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return ""
	}
	return auth.RefreshHash(cookie.Value)
}

func (h *Handler) requireActiveRefreshHash(c *echo.Context) (string, error) {
	currentTokenHash := h.currentRefreshHash(c)
	if currentTokenHash == "" {
		return "", echo.NewHTTPError(http.StatusUnauthorized, "refresh cookie required")
	}

	if _, err := h.refresh.GetActiveByHash(c.Request().Context(), currentTokenHash); err != nil {
		return "", echo.NewHTTPError(http.StatusUnauthorized, "current session is not active")
	}

	return currentTokenHash, nil
}

func refreshSessionStatus(item models.RefreshToken, now time.Time) string {
	if item.RevokedAt != nil {
		return refreshSessionStatusRevoked
	}
	if !item.ExpiresAt.After(now) {
		return refreshSessionStatusExpired
	}
	return refreshSessionStatusActive
}

func sessionDurationSeconds(item models.RefreshToken, now time.Time) int64 {
	endedAt := now
	switch refreshSessionStatus(item, now) {
	case refreshSessionStatusRevoked:
		if item.RevokedAt != nil {
			endedAt = item.RevokedAt.UTC()
		}
	case refreshSessionStatusExpired:
		endedAt = item.ExpiresAt.UTC()
	}
	duration := endedAt.Sub(item.CreatedAt.UTC())
	if duration < 0 {
		return 0
	}
	return int64(duration.Seconds())
}

func sessionRemainingSeconds(item models.RefreshToken, now time.Time) int64 {
	if refreshSessionStatus(item, now) != refreshSessionStatusActive {
		return 0
	}
	remaining := item.ExpiresAt.UTC().Sub(now)
	if remaining <= 0 {
		return 0
	}
	return int64(remaining.Seconds())
}
