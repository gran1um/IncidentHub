package api

import (
	"net/http"
	"strings"

	"incidenthub/backend/internal/middleware"

	"github.com/labstack/echo/v5"
)

const localFactoryResetConfirmationPhrase = "RESET LOCAL DATA"

type factoryResetLocalRequest struct {
	Confirmation string `json:"confirmation"`
}

func (h *Handler) FactoryResetLocal(c *echo.Context) error {
	identity, _ := middleware.GetIdentity(c)
	if !identity.IsPlatformAdmin {
		return echo.NewHTTPError(http.StatusForbidden, "platform admin required")
	}
	if !h.isLocalFactoryResetEnabled() {
		return echo.NewHTTPError(http.StatusForbidden, "local factory reset is only available in local or dev environments")
	}
	if h.system == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "system repository is not configured")
	}

	var req factoryResetLocalRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if strings.TrimSpace(req.Confirmation) != localFactoryResetConfirmationPhrase {
		return echo.NewHTTPError(http.StatusBadRequest, "confirmation phrase mismatch")
	}

	summary, err := h.system.FactoryResetLocal(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	_ = h.audits.Log(c.Request().Context(), nil, &identity.UserID, "factory_reset_local", "system", nil, map[string]any{
		"admin_tenant_id": summary.AdminTenantID.String(),
		"kept_user_ids":   summary.KeptUserIDs,
		"deleted":         summary.Deleted,
	})
	return c.JSON(http.StatusOK, map[string]any{
		"ok":                  true,
		"confirmation_phrase": localFactoryResetConfirmationPhrase,
		"summary":             summary,
	})
}

func (h *Handler) isLocalFactoryResetEnabled() bool {
	env := strings.ToLower(strings.TrimSpace(h.cfg.Env))
	switch env {
	case "dev", "development", "local", "test":
		return true
	default:
		return false
	}
}
