package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"incidenthub/backend/internal/middleware"
	"incidenthub/backend/internal/repository"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

type createAPIAccessTokenRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Scopes      []string `json:"scopes"`
	FullAccess  bool     `json:"full_access"`
	ExpiresAt   string   `json:"expires_at"`
}

//nolint:gochecknoglobals // Static allowlist used for request validation.
var allowedAPIAccessResources = map[string]struct{}{
	"alerts":         {},
	"cases":          {},
	"communications": {},
	"connectors":     {},
	"catalog":        {},
	"ai":             {},
	"dashboard":      {},
	"system":         {},
	"search":         {},
	"operations":     {},
	"tenants":        {},
	"experience":     {},
	"admin_tokens":   {},
}

func (h *Handler) ListAPIAccessTokens(c *echo.Context) error {
	if h.apiTokens == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "api token repository is not configured")
	}
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	items, err := h.apiTokens.ListByTenant(c.Request().Context(), tenantID, false, 300)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list api access tokens")
	}

	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{
			"id":           item.ID.String(),
			"name":         item.Name,
			"description":  item.Description,
			"token_prefix": item.TokenPrefix,
			"scopes":       item.Scopes,
			"full_access":  item.FullAccess,
			"created_by":   item.CreatedBy.String(),
			"expires_at":   item.ExpiresAt,
			"last_used_at": item.LastUsedAt,
			"created_at":   item.CreatedAt,
			"updated_at":   item.UpdatedAt,
		})
	}
	return c.JSON(http.StatusOK, out)
}

func (h *Handler) CreateAPIAccessToken(c *echo.Context) error {
	if h.apiTokens == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "api token repository is not configured")
	}
	identity, _ := middleware.GetIdentity(c)
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}

	var req createAPIAccessTokenRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name is required")
	}

	var expiresAt *time.Time
	if strings.TrimSpace(req.ExpiresAt) != "" {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(req.ExpiresAt))
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "expires_at must be RFC3339")
		}
		utc := parsed.UTC()
		if utc.Before(time.Now().UTC()) {
			return echo.NewHTTPError(http.StatusBadRequest, "expires_at must be in the future")
		}
		expiresAt = &utc
	}

	fullAccess := req.FullAccess
	scopes, err := normalizeAPITokenScopes(req.Scopes)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if !fullAccess && len(scopes) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "scopes are required when full_access=false")
	}
	if fullAccess {
		scopes = []string{}
	}

	plainToken, prefix, tokenHash, err := generateAPIAccessTokenSecret()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate api token")
	}

	item, err := h.apiTokens.Create(c.Request().Context(), repository.CreateAPIAccessTokenParams{
		TenantID:    tenantID,
		Name:        name,
		Description: strings.TrimSpace(req.Description),
		TokenHash:   tokenHash,
		TokenPrefix: prefix,
		Scopes:      scopes,
		FullAccess:  fullAccess,
		CreatedBy:   identity.UserID,
		ExpiresAt:   expiresAt,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to create api access token")
	}

	return c.JSON(http.StatusCreated, map[string]any{
		"id":           item.ID.String(),
		"name":         item.Name,
		"description":  item.Description,
		"token_prefix": item.TokenPrefix,
		"token":        plainToken,
		"scopes":       item.Scopes,
		"full_access":  item.FullAccess,
		"created_by":   item.CreatedBy.String(),
		"expires_at":   item.ExpiresAt,
		"created_at":   item.CreatedAt,
		"updated_at":   item.UpdatedAt,
	})
}

func (h *Handler) RevokeAPIAccessToken(c *echo.Context) error {
	if h.apiTokens == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "api token repository is not configured")
	}
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}

	tokenID, err := uuid.Parse(strings.TrimSpace(c.Param("tokenID")))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid token id")
	}

	if err := h.apiTokens.Revoke(c.Request().Context(), tenantID, tokenID); err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "api access token not found")
	}
	return c.JSON(http.StatusOK, map[string]any{"success": true})
}

func normalizeAPITokenScopes(raw []string) ([]string, error) {
	if len(raw) == 0 {
		return []string{}, nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(raw))
	for _, scope := range raw {
		normalized := strings.ToLower(strings.TrimSpace(scope))
		if normalized == "" {
			continue
		}
		if normalized == "*" || normalized == "all:*" {
			continue
		}
		parts := strings.SplitN(normalized, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("scope must be in <resource>:<read|write|*> format")
		}
		resource := strings.TrimSpace(parts[0])
		action := strings.TrimSpace(parts[1])
		if _, ok := allowedAPIAccessResources[resource]; !ok {
			return nil, fmt.Errorf("unsupported scope resource: %s", resource)
		}
		if action != "read" && action != "write" && action != "*" {
			return nil, fmt.Errorf("unsupported scope action: %s", action)
		}
		key := resource + ":" + action
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	sort.Strings(out)
	return out, nil
}

func generateAPIAccessTokenSecret() (plainToken string, prefix string, hash string, err error) {
	payload := make([]byte, 32)
	if _, err = rand.Read(payload); err != nil {
		return "", "", "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	plainToken = "ihat_" + encoded
	prefix = plainToken
	if len(prefix) > 16 {
		prefix = prefix[:16]
	}
	digest := sha256.Sum256([]byte(plainToken))
	hash = hex.EncodeToString(digest[:])
	return plainToken, prefix, hash, nil
}
