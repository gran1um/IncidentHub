package api

import (
	"net/http"
	"testing"
)

func TestHandlerAPIAccessTokensCRUD(t *testing.T) {
	env := newAPITestEnv(t)

	var tokenID string
	var plainToken string
	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/admin/api-tokens", map[string]any{
			"name":        "Automation token",
			"description": "Token for case/alert automation",
			"scopes":      []string{"alerts:write", "cases:read", "cases:read"},
			"full_access": false,
		})
		setPath(c, "/api/v1/admin/api-tokens", nil, nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.CreateAPIAccessToken(c)
		mustStatusOK(t, err, rec, http.StatusCreated)
		payload := decodeBody[map[string]any](t, rec)
		tokenID, _ = payload["id"].(string)
		plainToken, _ = payload["token"].(string)
		if tokenID == "" {
			t.Fatalf("expected token id in response")
		}
		if plainToken == "" {
			t.Fatalf("expected plain token in create response")
		}
		if _, exists := payload["token_hash"]; exists {
			t.Fatalf("token hash must not be exposed")
		}
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/admin/api-tokens", nil)
		setPath(c, "/api/v1/admin/api-tokens", nil, nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListAPIAccessTokens(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		items := decodeBody[[]map[string]any](t, rec)
		if len(items) == 0 {
			t.Fatalf("expected at least one api token")
		}
		if _, exists := items[0]["token"]; exists {
			t.Fatalf("list endpoint must not expose plain token")
		}
	}

	{
		c, rec := env.jsonContext(http.MethodDelete, "/api/v1/admin/api-tokens/"+tokenID, nil)
		setPath(c, "/api/v1/admin/api-tokens/:tokenID", []string{"tokenID"}, []string{tokenID})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.RevokeAPIAccessToken(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	if plainToken == "" {
		t.Fatalf("plain token should stay available for follow-up checks")
	}
}

func TestHandlerAPIAccessTokenValidation(t *testing.T) {
	env := newAPITestEnv(t)

	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/admin/api-tokens", map[string]any{
			"name":        "invalid",
			"full_access": false,
			"scopes":      []string{},
		})
		setPath(c, "/api/v1/admin/api-tokens", nil, nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.CreateAPIAccessToken(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("expected 400 for missing scopes, got %d body=%s", code, rec.Body.String())
		}
	}
	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/admin/api-tokens", map[string]any{
			"name":        "invalid",
			"full_access": false,
			"scopes":      []string{"users:write"},
		})
		setPath(c, "/api/v1/admin/api-tokens", nil, nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.CreateAPIAccessToken(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("expected 400 for invalid scope resource, got %d body=%s", code, rec.Body.String())
		}
	}
}
