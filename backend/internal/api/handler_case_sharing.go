package api

import (
	"net/http"
	"time"

	"incidenthub/backend/internal/middleware"
	"incidenthub/backend/internal/repository"

	"github.com/labstack/echo/v5"
)

func (h *Handler) ListCaseShares(c *echo.Context) error {
	requestedTenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	ownerTenantID, caseID, err := h.resolveCaseInTenant(c)
	if err != nil {
		return err
	}

	shares, err := h.cases.ListShares(c.Request().Context(), ownerTenantID, caseID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list case shares")
	}

	result := make([]map[string]any, 0, len(shares))
	for _, share := range shares {
		entry := map[string]any{
			"case_id":          share.CaseID.String(),
			"owner_tenant_id":  share.OwnerTenantID.String(),
			"shared_tenant_id": share.SharedTenantID.String(),
			"created_at":       share.CreatedAt.Format(time.RFC3339Nano),
			"updated_at":       share.UpdatedAt.Format(time.RFC3339Nano),
		}
		if share.CreatedBy != nil {
			entry["created_by"] = share.CreatedBy.String()
		}
		if tenant, tenantErr := h.tenants.GetByID(c.Request().Context(), share.SharedTenantID); tenantErr == nil {
			entry["shared_tenant_slug"] = tenant.Slug
			entry["shared_tenant_name"] = tenant.Name
			entry["shared_tenant_active"] = tenant.IsActive
		}
		result = append(result, entry)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"case_id":             caseID.String(),
		"owner_tenant_id":     ownerTenantID.String(),
		"requested_tenant_id": requestedTenantID.String(),
		"shares":              result,
	})
}

func (h *Handler) ShareCaseAcrossTenants(c *echo.Context) error {
	identity, _ := middleware.GetIdentity(c)
	requestedTenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	ownerTenantID, caseID, err := h.resolveCaseInTenant(c)
	if err != nil {
		return err
	}
	if ownerTenantID != requestedTenantID && !identity.IsPlatformAdmin {
		return echo.NewHTTPError(http.StatusForbidden, "only owner tenant can share case")
	}

	var req shareCaseRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	targetTenantID, err := h.resolveTargetTenantID(c.Request().Context(), req.TargetTenantID, req.TargetTenantSlug)
	if err != nil {
		return err
	}
	if targetTenantID == ownerTenantID {
		return echo.NewHTTPError(http.StatusBadRequest, "target tenant must differ from owner tenant")
	}
	targetTenant, err := h.tenants.GetByID(c.Request().Context(), targetTenantID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "target tenant not found")
	}
	if !targetTenant.IsActive {
		return echo.NewHTTPError(http.StatusConflict, "target tenant is inactive")
	}

	if shareErr := h.cases.ShareWithTenant(c.Request().Context(), ownerTenantID, caseID, targetTenantID, &identity.UserID); shareErr != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to share case")
	}
	if h.caseEvents != nil {
		_, _ = h.caseEvents.Create(c.Request().Context(), repository.CreateCaseEventParams{
			TenantID:  ownerTenantID,
			CaseID:    caseID,
			EventType: "case_shared_tenant",
			Title:     "Case shared across tenants",
			Body:      "Case shared with tenant " + targetTenant.Slug,
			ActorID:   &identity.UserID,
			Metadata: map[string]any{
				"shared_tenant_id":   targetTenantID.String(),
				"shared_tenant_slug": targetTenant.Slug,
			},
		})
	}
	_ = h.audits.Log(c.Request().Context(), &ownerTenantID, &identity.UserID, "case_share_tenant", "case", &caseID, map[string]any{
		"shared_tenant_id":   targetTenantID.String(),
		"shared_tenant_slug": targetTenant.Slug,
	})

	return c.JSON(http.StatusCreated, map[string]any{
		"case_id":            caseID.String(),
		"owner_tenant_id":    ownerTenantID.String(),
		"shared_tenant_id":   targetTenantID.String(),
		"shared_tenant_slug": targetTenant.Slug,
		"shared_tenant_name": targetTenant.Name,
		"mode":               "collaborative_single_case",
	})
}
