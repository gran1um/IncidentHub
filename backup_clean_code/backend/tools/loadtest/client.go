package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type apiClient struct {
	baseURL string
	client  *http.Client
}

func newAPIClient(cfg config) *apiClient {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.InsecureSkipTLS, //nolint:gosec // explicitly controlled by CLI flag for local perf tests
		},
		MaxIdleConns:        cfg.Workers * 4,
		MaxIdleConnsPerHost: cfg.Workers * 2,
		IdleConnTimeout:     90 * time.Second,
	}
	return &apiClient{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		client: &http.Client{
			Timeout:   cfg.HTTPTimeout,
			Transport: transport,
		},
	}
}

func (c *apiClient) login(ctx context.Context, email string, password string) (authSession, error) {
	payload := map[string]string{
		"email":    email,
		"password": password,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return authSession{}, fmt.Errorf("marshal login payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/auth/login", bytes.NewReader(raw))
	if err != nil {
		return authSession{}, fmt.Errorf("build login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return authSession{}, fmt.Errorf("request login: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return authSession{}, fmt.Errorf("login failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	type membership struct {
		TenantID string `json:"tenant_id"`
	}
	type identity struct {
		TenantID string `json:"tenant_id"`
	}
	var parsed struct {
		AccessToken string       `json:"access_token"`
		Memberships []membership `json:"memberships"`
		Identity    identity     `json:"identity"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return authSession{}, fmt.Errorf("decode login response: %w", err)
	}
	if strings.TrimSpace(parsed.AccessToken) == "" {
		return authSession{}, fmt.Errorf("login response missing access_token")
	}

	tenantID := strings.TrimSpace(parsed.Identity.TenantID)
	if tenantID == "" && len(parsed.Memberships) > 0 {
		tenantID = strings.TrimSpace(parsed.Memberships[0].TenantID)
	}
	return authSession{
		Token:    parsed.AccessToken,
		TenantID: tenantID,
	}, nil
}

func (c *apiClient) request(
	ctx context.Context,
	method string,
	path string,
	body []byte,
	contentType string,
	token string,
	tenantID string,
	requireTenant bool,
) (*http.Response, error) {
	endpoint := path
	if !strings.HasPrefix(path, "http://") && !strings.HasPrefix(path, "https://") {
		endpoint = c.baseURL + path
	}

	var reader io.Reader = http.NoBody
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(token) != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if requireTenant && strings.TrimSpace(tenantID) != "" {
		req.Header.Set("X-Tenant-ID", tenantID)
	}
	if len(body) > 0 {
		if strings.TrimSpace(contentType) == "" {
			contentType = "application/json"
		}
		req.Header.Set("Content-Type", contentType)
	}

	return c.client.Do(req)
}
