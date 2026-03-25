package middleware

import (
	"incidenthub/backend/internal/models"

	"github.com/labstack/echo/v5"
)

func RequirePlatformAdmin() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			identity, ok := GetIdentity(c)
			if !ok {
				return echo.NewHTTPError(401, "authentication required")
			}
			if !identity.IsPlatformAdmin {
				return echo.NewHTTPError(403, "platform admin role required")
			}
			return next(c)
		}
	}
}

func RequireTenantAdminOrPlatformAdmin() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			identity, ok := GetIdentity(c)
			if !ok {
				return echo.NewHTTPError(401, "authentication required")
			}
			if identity.IsPlatformAdmin {
				return next(c)
			}
			if identity.TenantRole != models.TenantRoleAdmin {
				return echo.NewHTTPError(403, "tenant admin role required")
			}
			return next(c)
		}
	}
}

func RequireAnalystOrHigher() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			identity, ok := GetIdentity(c)
			if !ok {
				return echo.NewHTTPError(401, "authentication required")
			}
			if identity.IsPlatformAdmin {
				return next(c)
			}
			switch identity.TenantRole {
			case models.TenantRoleAdmin, models.TenantRoleAnalyst:
				return next(c)
			default:
				return echo.NewHTTPError(403, "insufficient role")
			}
		}
	}
}
