package middleware

import (
	"strings"

	"github.com/labstack/echo/v5"
)

func SecurityHeaders() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			r := c.Response()
			csp := "default-src 'self'; connect-src 'self'; img-src 'self' data:; " +
				"style-src 'self' 'unsafe-inline'; script-src 'self'; frame-ancestors 'none';"
			if strings.HasPrefix(c.Request().URL.Path, "/dev/swagger") {
				csp = "default-src 'self'; connect-src 'self' https://unpkg.com; img-src 'self' data:; " +
					"style-src 'self' 'unsafe-inline' https://unpkg.com; script-src 'self' 'unsafe-inline' https://unpkg.com; " +
					"frame-ancestors 'none';"
			}
			r.Header().Set("X-Frame-Options", "DENY")
			r.Header().Set("X-Content-Type-Options", "nosniff")
			r.Header().Set("Referrer-Policy", "no-referrer")
			r.Header().Set("Content-Security-Policy", csp)
			return next(c)
		}
	}
}
