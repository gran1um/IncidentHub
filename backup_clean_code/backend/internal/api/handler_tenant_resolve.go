package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

func (h *Handler) resolveTargetTenantID(ctx context.Context, targetTenantIDRaw, targetTenantSlug string) (uuid.UUID, error) {
	trimmedID := strings.TrimSpace(targetTenantIDRaw)
	trimmedSlug := strings.TrimSpace(targetTenantSlug)
	if trimmedID == "" && trimmedSlug == "" {
		return uuid.Nil, echo.NewHTTPError(http.StatusBadRequest, "target_tenant_id or target_tenant_slug is required")
	}
	if trimmedID != "" {
		targetTenantID, err := uuid.Parse(trimmedID)
		if err != nil {
			return uuid.Nil, echo.NewHTTPError(http.StatusBadRequest, "invalid target_tenant_id")
		}
		return targetTenantID, nil
	}
	targetTenant, err := h.tenants.GetBySlug(ctx, trimmedSlug)
	if err != nil {
		return uuid.Nil, echo.NewHTTPError(http.StatusNotFound, "target tenant not found")
	}
	return targetTenant.ID, nil
}
