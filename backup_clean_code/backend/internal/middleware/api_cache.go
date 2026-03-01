package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"incidenthub/backend/internal/cache"
	"incidenthub/backend/internal/config"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

const (
	apiCacheHeader           = "X-API-Cache"
	apiCacheNamespace        = "api_cache"
	apiCacheVersionGlobalKey = apiCacheNamespace + ":v:global"
)

type cachedHTTPResponse struct {
	Status      int    `json:"status"`
	ContentType string `json:"content_type"`
	Body        string `json:"body"`
}

func APICache(client *cache.Client, cfg config.APICacheConfig) echo.MiddlewareFunc {
	if client == nil || !cfg.Enabled {
		return passthroughMiddleware()
	}
	if cfg.TTL <= 0 {
		cfg.TTL = 30 * time.Second
	}
	if cfg.MaxBodyBytes <= 0 {
		cfg.MaxBodyBytes = 1 << 20
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if !isCacheableRequest(c.Request()) {
				return next(c)
			}

			cacheKey := buildAPICacheKey(c, client)
			if cacheKey != "" {
				if raw, err := client.Get(c.Request().Context(), cacheKey); err == nil && raw != "" {
					var payload cachedHTTPResponse
					if jsonErr := json.Unmarshal([]byte(raw), &payload); jsonErr == nil && payload.Status > 0 {
						contentType := strings.TrimSpace(payload.ContentType)
						if contentType == "" {
							contentType = echo.MIMEApplicationJSON
						}
						c.Response().Header().Set(apiCacheHeader, "HIT")
						return c.Blob(payload.Status, contentType, []byte(payload.Body))
					}
				}
			}

			writer := &cacheResponseWriter{
				ResponseWriter: c.Response(),
				body:           bytes.NewBuffer(nil),
			}
			c.SetResponse(writer)

			if err := next(c); err != nil {
				return err
			}

			resp, unwrapErr := echo.UnwrapResponse(c.Response())
			if unwrapErr != nil {
				return nil
			}
			statusCode := resp.Status
			if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
				return nil
			}
			if writer.body.Len() <= 0 || writer.body.Len() > cfg.MaxBodyBytes {
				return nil
			}

			payload := cachedHTTPResponse{
				Status:      statusCode,
				ContentType: c.Response().Header().Get(echo.HeaderContentType),
				Body:        writer.body.String(),
			}
			encoded, err := json.Marshal(payload)
			if err != nil {
				return nil
			}
			if cacheKey != "" {
				_ = client.Set(c.Request().Context(), cacheKey, string(encoded), cfg.TTL)
				c.Response().Header().Set(apiCacheHeader, "MISS")
			}
			return nil
		}
	}
}

func APICacheInvalidation(client *cache.Client, cfg config.APICacheConfig) echo.MiddlewareFunc {
	if client == nil || !cfg.Enabled {
		return passthroughMiddleware()
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if err := next(c); err != nil {
				return err
			}
			if !isInvalidationCandidate(c.Request()) {
				return nil
			}
			resp, unwrapErr := echo.UnwrapResponse(c.Response())
			if unwrapErr != nil {
				return nil
			}
			statusCode := resp.Status
			if statusCode < http.StatusOK || statusCode >= http.StatusBadRequest {
				return nil
			}

			ctx := c.Request().Context()
			_, _ = client.Incr(ctx, apiCacheVersionGlobalKey)
			if tenantID, ok := GetTenantID(c); ok {
				_, _ = client.Incr(ctx, apiCacheVersionTenantKey(tenantID))
			}
			if identity, ok := GetIdentity(c); ok {
				_, _ = client.Incr(ctx, apiCacheVersionUserKey(identity.UserID))
			}
			return nil
		}
	}
}

func passthroughMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			return next(c)
		}
	}
}

func isCacheableRequest(req *http.Request) bool {
	if req == nil {
		return false
	}
	if req.Method != http.MethodGet {
		return false
	}
	path := req.URL.Path
	if strings.HasPrefix(path, "/healthz") || strings.HasPrefix(path, "/metrics") || strings.HasPrefix(path, "/dev/swagger") {
		return false
	}
	if strings.HasPrefix(path, "/api/v1/operations/") {
		return false
	}
	if req.URL.Query().Get("no_cache") == "1" {
		return false
	}
	return true
}

func isInvalidationCandidate(req *http.Request) bool {
	if req == nil {
		return false
	}
	switch req.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
}

func apiCacheVersionTenantKey(tenantID uuid.UUID) string {
	return fmt.Sprintf("%s:v:tenant:%s", apiCacheNamespace, tenantID.String())
}

func apiCacheVersionUserKey(userID uuid.UUID) string {
	return fmt.Sprintf("%s:v:user:%s", apiCacheNamespace, userID.String())
}

func buildAPICacheKey(c *echo.Context, client *cache.Client) string {
	req := c.Request()
	if req == nil {
		return ""
	}
	tenantID, hasTenant := GetTenantID(c)
	identity, hasIdentity := GetIdentity(c)

	globalVersion := readCacheVersion(req.Context(), client, apiCacheVersionGlobalKey)
	tenantVersion := "0"
	if hasTenant {
		tenantVersion = readCacheVersion(req.Context(), client, apiCacheVersionTenantKey(tenantID))
	}
	userVersion := "0"
	if hasIdentity {
		userVersion = readCacheVersion(req.Context(), client, apiCacheVersionUserKey(identity.UserID))
	}

	keyRaw := strings.Join([]string{
		req.Method,
		req.URL.Path,
		req.URL.RawQuery,
		req.Header.Get(config.TenantHeader),
		globalVersion,
		tenantVersion,
		userVersion,
	}, "|")
	hash := sha256.Sum256([]byte(keyRaw))
	return apiCacheNamespace + ":resp:" + hex.EncodeToString(hash[:])
}

func readCacheVersion(ctx context.Context, client *cache.Client, key string) string {
	if client == nil || key == "" {
		return "0"
	}
	value, err := client.Get(ctx, key)
	if err != nil {
		return "0"
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "0"
	}
	return trimmed
}

type cacheResponseWriter struct {
	http.ResponseWriter
	body *bytes.Buffer
}

func (w *cacheResponseWriter) WriteHeader(code int) {
	w.ResponseWriter.WriteHeader(code)
}

func (w *cacheResponseWriter) Write(p []byte) (int, error) {
	if w.body != nil {
		_, _ = w.body.Write(p)
	}
	return w.ResponseWriter.Write(p)
}

func (w *cacheResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
