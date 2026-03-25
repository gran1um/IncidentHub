package middleware

import (
	"net/http"
	"strings"
	"time"

	"incidenthub/backend/internal/metrics"
	"incidenthub/backend/internal/tracing"

	"github.com/labstack/echo/v5"
	"go.opentelemetry.io/otel/attribute"
)

func RequestObservability() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			method := strings.TrimSpace(c.Request().Method)
			if method == "" {
				method = http.MethodGet
			}
			route := strings.TrimSpace(c.Path())
			if route == "" {
				route = strings.TrimSpace(c.Request().URL.Path)
			}
			if route == "" {
				route = "/"
			}

			ctx, span, startedAt := tracing.StartModuleOperation(
				c.Request().Context(),
				"api",
				"request",
				attribute.String("http.method", method),
				attribute.String("http.route", route),
				attribute.String("http.target", strings.TrimSpace(c.Request().URL.Path)),
			)
			c.SetRequest(c.Request().WithContext(ctx))

			err := next(c)
			statusCode := http.StatusOK
			if resp, unwrapErr := echo.UnwrapResponse(c.Response()); unwrapErr == nil && resp.Status > 0 {
				statusCode = resp.Status
			}
			span.SetAttributes(attribute.Int("http.status_code", statusCode))
			if err != nil {
				span.Error(err)
			}
			span.End()

			status := "ok"
			if statusCode >= http.StatusInternalServerError || err != nil {
				status = "error"
			}
			metrics.ObserveModuleOperation("api", "request", status, time.Since(startedAt))
			return err
		}
	}
}
