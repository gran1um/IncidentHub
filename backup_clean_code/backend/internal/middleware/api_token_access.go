package middleware

import (
	"net/http"
	"strings"

	"incidenthub/backend/internal/models"

	"github.com/labstack/echo/v5"
)

func APITokenAccessControl() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			identity, ok := GetIdentity(c)
			if !ok || identity.AuthType != models.IdentityAuthTypeAPIToken {
				return next(c)
			}

			path := strings.TrimSpace(c.Path())
			if path == "" {
				path = strings.TrimSpace(c.Request().URL.Path)
			}
			method := strings.ToUpper(strings.TrimSpace(c.Request().Method))
			if method == "" {
				method = http.MethodGet
			}

			if isAPITokenUserAccountRoute(path) {
				return echo.NewHTTPError(http.StatusForbidden, "api token cannot access user-account endpoints")
			}

			if identity.APITokenFullAccess {
				return next(c)
			}

			resource, action, ok := apiTokenResourceAction(path, method)
			if !ok {
				return echo.NewHTTPError(http.StatusForbidden, "api token scope does not allow this endpoint")
			}
			if !apiTokenHasPermission(identity.APITokenScopes, resource, action) {
				return echo.NewHTTPError(http.StatusForbidden, "api token scope does not allow this endpoint")
			}

			return next(c)
		}
	}
}

func isAPITokenUserAccountRoute(path string) bool {
	switch path {
	case "/api/v1/me",
		"/api/v1/auth/logout",
		"/api/v1/auth/sessions",
		"/api/v1/auth/sessions/revoke-others",
		"/api/v1/users",
		"/api/v1/users/:id",
		"/api/v1/users/:id/media/:kind/upload",
		"/api/v1/users/:id/experience-events",
		"/api/v1/users/:id/performance",
		"/api/v1/tenant-users":
		return true
	default:
		return false
	}
}

func apiTokenResourceAction(path, method string) (resource string, action string, ok bool) {
	action = "write"
	if method == http.MethodGet || method == http.MethodHead {
		action = "read"
	}

	switch {
	case path == "/api/v1/users/:id/experience-awards":
		return "experience", "write", true
	case path == "/api/v1/tenants" || path == "/api/v1/tenants/:tenantID":
		return "tenants", action, true
	case strings.HasPrefix(path, "/api/v1/alerts"):
		return "alerts", action, true
	case strings.HasPrefix(path, "/api/v1/tasks"):
		return "cases", action, true
	case strings.HasPrefix(path, "/api/v1/cases"):
		if strings.Contains(path, "/communications") {
			return "communications", action, true
		}
		if strings.Contains(path, "/ai/") {
			return "ai", action, true
		}
		return "cases", action, true
	case strings.HasPrefix(path, "/api/v1/forum") || strings.HasPrefix(path, "/api/v1/case-comments"):
		return "communications", action, true
	case strings.HasPrefix(path, "/api/v1/connectors"):
		return "connectors", action, true
	case strings.HasPrefix(path, "/api/v1/catalog"):
		return "catalog", action, true
	case strings.HasPrefix(path, "/api/v1/ai"):
		return "ai", action, true
	case path == "/api/v1/dashboard/stats":
		return "dashboard", "read", true
	case strings.HasPrefix(path, "/api/v1/system/"):
		return "system", "read", true
	case strings.HasPrefix(path, "/api/v1/search"):
		return "search", "read", true
	case strings.HasPrefix(path, "/api/v1/operations"):
		return "operations", "read", true
	case strings.HasPrefix(path, "/api/v1/case-statuses"):
		return "cases", action, true
	case strings.HasPrefix(path, "/api/v1/admin/api-tokens"):
		return "admin_tokens", action, true
	default:
		return "", "", false
	}
}

func apiTokenHasPermission(scopes []string, resource, action string) bool {
	normalizedScopes := map[string]struct{}{}
	for _, scope := range scopes {
		value := strings.ToLower(strings.TrimSpace(scope))
		if value == "" {
			continue
		}
		normalizedScopes[value] = struct{}{}
	}

	if _, ok := normalizedScopes["*"]; ok {
		return true
	}
	if _, ok := normalizedScopes["all:*"]; ok {
		return true
	}
	if _, ok := normalizedScopes[resource+":*"]; ok {
		return true
	}
	if _, ok := normalizedScopes[resource+":"+action]; ok {
		return true
	}

	// write scope implies read.
	if action == "read" {
		if _, ok := normalizedScopes[resource+":write"]; ok {
			return true
		}
	}
	return false
}
