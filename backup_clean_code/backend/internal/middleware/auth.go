package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"incidenthub/backend/internal/auth"
	"incidenthub/backend/internal/metrics"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"
	"strings"

	"github.com/labstack/echo/v5"
)

func JWTAuth(jwtSvc *auth.Service, apiTokens *repository.APIAccessTokenRepository, metric *metrics.Collector) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			header := strings.TrimSpace(c.Request().Header.Get("Authorization"))
			if header == "" {
				if metric != nil {
					metric.DataAccessDenied.WithLabelValues("missing_authorization_header").Inc()
				}
				return echo.NewHTTPError(401, "missing authorization header")
			}
			parts := strings.SplitN(header, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				if metric != nil {
					metric.DataAccessDenied.WithLabelValues("invalid_authorization_header").Inc()
				}
				return echo.NewHTTPError(401, "invalid authorization header")
			}

			claims, err := jwtSvc.ParseAccess(parts[1])
			if err == nil {
				identity, identityErr := auth.IdentityFromClaims(claims)
				if identityErr != nil {
					if metric != nil {
						metric.DataAccessDenied.WithLabelValues("invalid_claims").Inc()
					}
					return echo.NewHTTPError(401, "invalid token claims")
				}
				identity.AuthType = models.IdentityAuthTypeJWT
				setIdentity(c, identity)
				return next(c)
			}

			if apiTokens == nil {
				if metric != nil {
					metric.DataAccessDenied.WithLabelValues("invalid_jwt").Inc()
				}
				return echo.NewHTTPError(401, "invalid token")
			}

			tokenHash := hashAccessToken(parts[1])
			apiToken, tokenErr := apiTokens.GetActiveByHash(c.Request().Context(), tokenHash)
			if tokenErr != nil {
				if metric != nil {
					metric.DataAccessDenied.WithLabelValues("invalid_api_token").Inc()
				}
				return echo.NewHTTPError(401, "invalid token")
			}

			tenantID := apiToken.TenantID
			tokenID := apiToken.ID
			identity := models.Identity{
				UserID:             apiToken.CreatedBy,
				Username:           "api-token:" + strings.TrimSpace(apiToken.Name),
				TenantID:           &tenantID,
				TenantRole:         models.TenantRoleAdmin,
				AuthType:           models.IdentityAuthTypeAPIToken,
				APITokenID:         &tokenID,
				APITokenScopes:     append([]string{}, apiToken.Scopes...),
				APITokenFullAccess: apiToken.FullAccess,
			}
			setIdentity(c, identity)
			_ = apiTokens.TouchLastUsed(c.Request().Context(), apiToken.ID)
			return next(c)
		}
	}
}

func RequireAuthenticated() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if _, ok := GetIdentity(c); !ok {
				return echo.NewHTTPError(401, "authentication required")
			}
			return next(c)
		}
	}
}

func hashAccessToken(token string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(digest[:])
}
