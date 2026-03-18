package inbound

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func (w *Worker) fetchHTTPRecords(ctx context.Context, cfg inboundConnectorConfig) ([]map[string]any, error) {
	httpCfg := cfg.HTTP
	reqCtx, cancel := withOptionalTimeout(ctx, firstPositiveDuration(httpCfg.Timeout, cfg.Timeout, w.cfg.HTTPTimeout, 20*time.Second))
	defer cancel()

	var bodyReader io.Reader
	if strings.TrimSpace(httpCfg.Body) != "" {
		bodyReader = bytes.NewBufferString(httpCfg.Body)
	}

	req, err := http.NewRequestWithContext(reqCtx, httpCfg.Method, httpCfg.URL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request for inbound connector %q: %w", cfg.DisplayName, err)
	}
	for key, value := range httpCfg.Headers {
		req.Header.Set(key, value)
	}
	applyHTTPAuth(req, httpCfg.Auth)
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json")
	}
	if strings.TrimSpace(httpCfg.Body) != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request for inbound connector %q: %w", cfg.DisplayName, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("inbound connector %q returned non-success status: %s", cfg.DisplayName, resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 25*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read inbound connector response %q: %w", cfg.DisplayName, err)
	}
	if len(body) == 0 {
		return []map[string]any{}, nil
	}

	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode inbound connector response %q: %w", cfg.DisplayName, err)
	}
	return extractRecords(payload, cfg.ArrayPath), nil
}

func applyHTTPAuth(req *http.Request, auth inboundAuthConfig) {
	if req == nil {
		return
	}
	for key, value := range auth.Headers {
		if strings.TrimSpace(key) == "" {
			continue
		}
		req.Header.Set(key, value)
	}
	switch strings.ToLower(strings.TrimSpace(auth.Type)) {
	case "basic":
		req.SetBasicAuth(auth.Username, auth.Password)
	case "bearer":
		if strings.TrimSpace(auth.Token) != "" {
			req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(auth.Token))
		}
	case "api_key", "apikey":
		if strings.TrimSpace(auth.APIKey) != "" {
			header := strings.TrimSpace(auth.APIKeyHeader)
			if header == "" {
				header = "X-API-Key"
			}
			req.Header.Set(header, strings.TrimSpace(auth.APIKey))
		}
	}
}

func firstPositiveDuration(values ...time.Duration) time.Duration {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func withOptionalTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}
