package middleware

import (
	"errors"
	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v5"
)

func ResolveTenant(cfg config.App, memberships *repository.MembershipRepository, tenants *repository.TenantRepository) echo.MiddlewareFunc {
	_ = cfg
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			identity, ok := GetIdentity(c)
			if !ok {
				return echo.NewHTTPError(401, "authentication required")
			}

			tenantRaw := c.Request().Header.Get(config.TenantHeader)
			if tenantRaw == "" {
				if identity.IsPlatformAdmin {
					return next(c)
				}
				return echo.NewHTTPError(400, "tenant header is required")
			}
			tenantID, err := uuid.Parse(tenantRaw)
			if err != nil {
				return echo.NewHTTPError(400, "invalid tenant id")
			}
			if tenants == nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "tenant repository not configured")
			}
			tenant, err := tenants.GetByID(c.Request().Context(), tenantID)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return echo.NewHTTPError(http.StatusNotFound, "tenant not found")
				}
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to resolve tenant")
			}
			if !tenant.IsActive {
				return echo.NewHTTPError(http.StatusLocked, "tenant is inactive")
			}

			if identity.AuthType == models.IdentityAuthTypeAPIToken {
				if identity.TenantID == nil || *identity.TenantID != tenantID {
					return echo.NewHTTPError(403, "api token is not valid for requested tenant")
				}
				identity.TenantID = &tenantID
				if identity.TenantRole == "" {
					identity.TenantRole = models.TenantRoleAdmin
				}
				setIdentity(c, identity)
				setTenantID(c, tenantID)
				return next(c)
			}
			if identity.IsPlatformAdmin {
				identity.TenantID = &tenantID
				if identity.TenantRole == "" {
					identity.TenantRole = models.TenantRoleAdmin
				}
				setIdentity(c, identity)
				setTenantID(c, tenantID)
				return next(c)
			}

			if identity.TenantID != nil && *identity.TenantID != tenantID {
				return echo.NewHTTPError(403, "tenant switch is not allowed for this user")
			}
			m, err := memberships.Get(c.Request().Context(), tenantID, identity.UserID)
			if err != nil || !m.IsActive {
				return echo.NewHTTPError(403, "no access to requested tenant")
			}
			identity.TenantID = &tenantID
			identity.TenantRole = m.Role
			setIdentity(c, identity)

			setTenantID(c, tenantID)
			return next(c)
		}
	}
}
