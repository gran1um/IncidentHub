package api

import (
	"errors"
	"net/http"
	"strings"

	"incidenthub/backend/internal/logger"

	"github.com/labstack/echo/v5"
)

type errorResponse struct {
	Error string `json:"error"`
}

func customHTTPErrorHandler(c *echo.Context, err error) {
	if resp, unwrapErr := echo.UnwrapResponse(c.Response()); unwrapErr == nil && resp.Committed {
		return
	}

	httpErr := &echo.HTTPError{}
	ok := errors.As(err, &httpErr)
	if !ok {
		logger.Errorf("internal server error: method=%s path=%s err=%v", c.Request().Method, c.Request().URL.Path, err)
		_ = c.JSON(http.StatusInternalServerError, errorResponse{Error: "internal server error"})
		return
	}

	if httpErr.Code >= 500 {
		logger.Errorf("http error %d: method=%s path=%s msg=%v", httpErr.Code, c.Request().Method, c.Request().URL.Path, httpErr.Message)
	}

	msg := strings.TrimSpace(httpErr.Message)
	if msg == "" {
		msg = "request failed"
	}
	_ = c.JSON(httpErr.Code, errorResponse{Error: msg})
}
