package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"incidenthub/backend/internal/ai"
	"incidenthub/backend/internal/auth"
	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"
	"incidenthub/backend/internal/security"
	"incidenthub/backend/internal/storage"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

type failingRefreshStore struct {
	base refreshTokenStore
}

func (s failingRefreshStore) Create(context.Context, models.RefreshToken) error {
	return errors.New("failed to persist refresh token")
}

func (s failingRefreshStore) GetActiveByHash(ctx context.Context, tokenHash string) (*models.RefreshToken, error) {
	return s.base.GetActiveByHash(ctx, tokenHash)
}

func (s failingRefreshStore) RevokeByHash(ctx context.Context, tokenHash string) error {
	return s.base.RevokeByHash(ctx, tokenHash)
}

func (s failingRefreshStore) RevokeAllByUser(ctx context.Context, userID uuid.UUID) error {
	return s.base.RevokeAllByUser(ctx, userID)
}

func (s failingRefreshStore) RevokeAllByUserExceptHash(ctx context.Context, userID uuid.UUID, keepTokenHash string) (int64, error) {
	return s.base.RevokeAllByUserExceptHash(ctx, userID, keepTokenHash)
}

func (s failingRefreshStore) ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]models.RefreshToken, error) {
	return s.base.ListByUser(ctx, userID, limit)
}

func httpErrorCode(t *testing.T, err error) int {
	t.Helper()
	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected *echo.HTTPError, got %T (%v)", err, err)
	}
	return httpErr.Code
}

func TestCatalogAndTenantValidationBranches(t *testing.T) {
	env := newAPITestEnv(t)

	{
		c, _ := env.jsonContext(http.MethodPost, "/api/v1/tenants", map[string]any{
			"slug": "tenant-main",
			"name": "Duplicate Slug",
		})
		setIdentity(c, env.platformAdmin)
		err := env.handler.CreateTenant(c)
		if code := httpErrorCode(t, err); code != http.StatusConflict {
			t.Fatalf("expected tenant conflict, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodPost, "/api/v1/tenants", map[string]any{
			"slug":                "tenant-new-branch",
			"name":                "Tenant New",
			"responsible_user_id": "not-uuid",
		})
		setIdentity(c, env.platformAdmin)
		err := env.handler.CreateTenant(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("expected bad request for responsible_user_id, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodPatch, "/api/v1/tenants/not-a-uuid", map[string]any{"name": "x"})
		setPath(c, "/api/v1/tenants/:tenantID", []string{"tenantID"}, []string{"not-a-uuid"})
		setIdentity(c, env.platformAdmin)
		err := env.handler.UpdateTenant(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("expected invalid tenant id error, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodPatch, "/api/v1/tenants/"+env.tenantID.String(), map[string]any{
			"responsible_user_id": "bad-uuid",
		})
		setPath(c, "/api/v1/tenants/:tenantID", []string{"tenantID"}, []string{env.tenantID.String()})
		setIdentity(c, env.platformAdmin)
		err := env.handler.UpdateTenant(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("expected invalid responsible id error, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodGet, "/api/v1/catalog/connectors?owner_id=bad-uuid", nil)
		c.Request().URL.RawQuery = "owner_id=bad-uuid"
		setPath(c, "/api/v1/catalog/:kind", []string{"kind"}, []string{"connectors"})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListCatalogItems(c)
		if code := httpErrorCode(t, err); code != http.StatusGone {
			t.Fatalf("expected deprecated kind error, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodPost, "/api/v1/catalog/inbound_connectors", map[string]any{
			"data": map[string]any{
				"name":        "bad inbound",
				"source_type": "http",
			},
		})
		setPath(c, "/api/v1/catalog/:kind", []string{"kind"}, []string{"inbound_connectors"})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.CreateCatalogItem(c)
		if code := httpErrorCode(t, err); code != http.StatusGone {
			t.Fatalf("expected inbound connector removal error, got %d", code)
		}
	}

	{
		viewerIdentity := env.identity
		viewerIdentity.TenantRole = "viewer"
		c, _ := env.jsonContext(http.MethodPost, "/api/v1/catalog/rate_limits", map[string]any{
			"data": map[string]any{"name": "rl-1"},
		})
		setPath(c, "/api/v1/catalog/:kind", []string{"kind"}, []string{"rate_limits"})
		setIdentity(c, viewerIdentity)
		setTenant(c, env.tenantID)
		err := env.handler.CreateCatalogItem(c)
		if code := httpErrorCode(t, err); code != http.StatusForbidden {
			t.Fatalf("expected forbidden for viewer catalog create, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodPost, "/api/v1/catalog/outbound_connectors", map[string]any{
			"owner_id": "bad-owner-id",
			"data": map[string]any{
				"name":      "bad owner connector",
				"direction": "outbound",
				"channel":   "mock",
			},
		})
		setPath(c, "/api/v1/catalog/:kind", []string{"kind"}, []string{"outbound_connectors"})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.CreateCatalogItem(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("expected invalid owner_id error, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodPost, "/api/v1/catalog/api_tokens", map[string]any{
			"data": map[string]any{
				"name":  "legacy-token",
				"token": "tok_legacy",
			},
		})
		setPath(c, "/api/v1/catalog/:kind", []string{"kind"}, []string{"api_tokens"})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.CreateCatalogItem(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("expected api_tokens create to be rejected, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodPatch, "/api/v1/catalog/api_tokens/"+uuid.NewString(), map[string]any{
			"data": map[string]any{
				"name": "legacy-token-updated",
			},
		})
		setPath(c, "/api/v1/catalog/:kind/:itemID", []string{"kind", "itemID"}, []string{"api_tokens", uuid.NewString()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.UpdateCatalogItem(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("expected api_tokens update to be rejected, got %d", code)
		}
	}

	{
		shift1, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
			TenantID:  &env.tenantID,
			Kind:      "shifts",
			CreatedBy: &env.userID,
			Data: map[string]any{
				"month": 2,
				"year":  2026,
				"user":  env.userID.String(),
			},
		})
		if err != nil {
			t.Fatalf("create shift 1: %v", err)
		}
		_, err = env.catalog.Create(context.Background(), repository.CatalogCreateParams{
			TenantID:  &env.tenantID,
			Kind:      "shifts",
			CreatedBy: &env.userID,
			Data: map[string]any{
				"month": 3,
				"year":  2026,
				"user":  env.userID.String(),
			},
		})
		if err != nil {
			t.Fatalf("create shift 2: %v", err)
		}

		c, rec := env.jsonContext(http.MethodGet, "/api/v1/catalog/shifts?month=2&year=2026", nil)
		c.Request().URL.RawQuery = "month=2&year=2026"
		setPath(c, "/api/v1/catalog/:kind", []string{"kind"}, []string{"shifts"})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err = env.handler.ListCatalogItems(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		items := decodeBody[[]map[string]any](t, rec)
		if len(items) == 0 || items[0]["id"] != shift1.ID.String() {
			t.Fatalf("expected filtered shift payload")
		}
	}

	{
		c, _ := env.jsonContext(http.MethodPatch, "/api/v1/catalog/notifications/not-a-uuid", map[string]any{
			"data": map[string]any{"enabled": true},
		})
		setPath(c, "/api/v1/catalog/:kind/:itemID", []string{"kind", "itemID"}, []string{"notifications", "not-a-uuid"})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.UpdateCatalogItem(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("expected invalid catalog item id error, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodDelete, "/api/v1/catalog/connectors/not-a-uuid", nil)
		setPath(c, "/api/v1/catalog/:kind/:itemID", []string{"kind", "itemID"}, []string{"connectors", "not-a-uuid"})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.DeleteCatalogItem(c)
		if code := httpErrorCode(t, err); code != http.StatusGone {
			t.Fatalf("expected deprecated kind error, got %d", code)
		}
	}
}

func TestAttachmentAndAIValidationBranches(t *testing.T) {
	env := newAPITestEnv(t)

	caseItem, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-ERR-1",
		Title:             "Case for validation paths",
		Description:       "Validation case",
		Source:            "manual",
		IncidentType:      "generic",
		Status:            "open",
		Priority:          "medium",
		Impact:            "system",
		Confidence:        20,
		Severity:          "low",
		TLP:               "green",
		PAP:               "green",
		ResolutionSummary: "",
		CreatedBy:         env.userID,
	})
	if err != nil {
		t.Fatalf("create case: %v", err)
	}

	{
		c, _ := env.jsonContext(http.MethodGet, "/api/v1/cases/"+caseItem.ID.String()+"/attachments/not-uuid/download", nil)
		setPath(
			c,
			"/api/v1/cases/:caseID/attachments/:attachmentID/download",
			[]string{"caseID", "attachmentID"},
			[]string{caseItem.ID.String(), "not-uuid"},
		)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		handlerErr := env.handler.GetCaseAttachmentDownloadURL(c)
		if code := httpErrorCode(t, handlerErr); code != http.StatusBadRequest {
			t.Fatalf("expected invalid attachment id error, got %d", code)
		}
	}

	{
		originalStorage := env.handler.artifacts
		env.handler.artifacts = nil

		c, _ := env.jsonContext(http.MethodGet, "/api/v1/cases/"+caseItem.ID.String()+"/attachments/"+uuid.NewString()+"/download", nil)
		setPath(
			c,
			"/api/v1/cases/:caseID/attachments/:attachmentID/download",
			[]string{"caseID", "attachmentID"},
			[]string{caseItem.ID.String(), uuid.NewString()},
		)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		handlerErr := env.handler.GetCaseAttachmentDownloadURL(c)
		if code := httpErrorCode(t, handlerErr); code != http.StatusServiceUnavailable {
			t.Fatalf("expected storage unavailable error, got %d", code)
		}
		env.handler.artifacts = originalStorage
	}

	attachment, err := env.attachments.Create(context.Background(), repository.CreateAttachmentParams{
		TenantID:      env.tenantID,
		CaseID:        caseItem.ID,
		FileName:      "artifact.bin",
		ContentType:   "application/octet-stream",
		FileSizeBytes: 10,
		StorageKey:    "tenant/x/case/y/artifact.bin",
		UploadedBy:    env.userID,
	})
	if err != nil {
		t.Fatalf("create attachment: %v", err)
	}

	{
		c, _ := env.jsonContext(http.MethodGet, "/api/v1/cases/"+caseItem.ID.String()+"/attachments/"+uuid.NewString()+"/download", nil)
		randomID := uuid.NewString()
		setPath(
			c,
			"/api/v1/cases/:caseID/attachments/:attachmentID/download",
			[]string{"caseID", "attachmentID"},
			[]string{caseItem.ID.String(), randomID},
		)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.GetCaseAttachmentDownloadURL(c)
		if code := httpErrorCode(t, err); code != http.StatusNotFound {
			t.Fatalf("expected attachment not found error, got %d", code)
		}
	}

	{
		env.storageStub.presignErr = storage.ErrStorageDisabled

		c, _ := env.jsonContext(http.MethodGet, "/api/v1/cases/"+caseItem.ID.String()+"/attachments/"+attachment.ID.String()+"/download", nil)
		setPath(
			c,
			"/api/v1/cases/:caseID/attachments/:attachmentID/download",
			[]string{"caseID", "attachmentID"},
			[]string{caseItem.ID.String(), attachment.ID.String()},
		)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.GetCaseAttachmentDownloadURL(c)
		if code := httpErrorCode(t, err); code != http.StatusServiceUnavailable {
			t.Fatalf("expected storage disabled error, got %d", code)
		}
		env.storageStub.presignErr = nil
	}

	{
		env.storageStub.presignErr = errors.New("presign fail")

		c, _ := env.jsonContext(http.MethodGet, "/api/v1/cases/"+caseItem.ID.String()+"/attachments/"+attachment.ID.String()+"/download", nil)
		setPath(
			c,
			"/api/v1/cases/:caseID/attachments/:attachmentID/download",
			[]string{"caseID", "attachmentID"},
			[]string{caseItem.ID.String(), attachment.ID.String()},
		)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.GetCaseAttachmentDownloadURL(c)
		if code := httpErrorCode(t, err); code != http.StatusBadGateway {
			t.Fatalf("expected bad gateway error for presign failure, got %d", code)
		}
		env.storageStub.presignErr = nil
	}

	{
		original := env.handler.ai
		env.handler.ai = nil
		c, _ := env.jsonContext(http.MethodPost, "/api/v1/ai/ask", map[string]any{
			"question": "Any guidance?",
		})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.AskAI(c)
		if code := httpErrorCode(t, err); code != http.StatusServiceUnavailable {
			t.Fatalf("expected ai unavailable error, got %d", code)
		}
		env.handler.ai = original
	}

	{
		c, _ := env.jsonContext(http.MethodPost, "/api/v1/ai/ask", map[string]any{"question": ""})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.AskAI(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("expected empty question error, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodPost, "/api/v1/ai/ask", map[string]any{
			"session_id": "invalid-session-id",
			"question":   "Should fail with invalid session id",
		})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.AskAI(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("expected invalid session_id error, got %d", code)
		}
	}

	{
		originalChatRepo := env.handler.aiChats
		env.handler.aiChats = nil
		defer func() { env.handler.aiChats = originalChatRepo }()

		env.handler.ai = ai.NewService(config.AIConfig{
			Enabled: true,
			Model:   "sec-mini-rag",
			TopK:    2,
			Timeout: 3 * time.Second,
		}, nil)

		c, _ := env.jsonContext(http.MethodPost, "/api/v1/ai/ask", map[string]any{
			"question": "Summarize this tenant security context",
		})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.AskAI(c)
		if code := httpErrorCode(t, err); code != http.StatusBadGateway {
			t.Fatalf("expected ai request to fail without fallbacks, got %d", code)
		}
	}
}

func TestLoginAndUserValidationBranches(t *testing.T) {
	env := newAPITestEnv(t)

	{
		c, _ := env.jsonContext(http.MethodPost, "/api/v1/auth/login", map[string]any{
			"email":    "analyst-main@example.com",
			"password": "wrong-password",
		})
		err := env.handler.Login(c)
		if code := httpErrorCode(t, err); code != http.StatusUnauthorized {
			t.Fatalf("expected invalid credentials error, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodPost, "/api/v1/auth/login", map[string]any{
			"email":    "absent-user@example.com",
			"password": "Password123!",
		})
		err := env.handler.Login(c)
		if code := httpErrorCode(t, err); code != http.StatusUnauthorized {
			t.Fatalf("expected missing user login error, got %d", code)
		}
	}

	{
		var originalHash string
		if err := env.pool.QueryRow(context.Background(), `SELECT password_hash FROM users WHERE id = $1`, env.userID).Scan(&originalHash); err != nil {
			t.Fatalf("read original hash: %v", err)
		}
		if _, err := env.pool.Exec(context.Background(), `UPDATE users SET password_hash = 'not-a-valid-hash' WHERE id = $1`, env.userID); err != nil {
			t.Fatalf("set invalid hash: %v", err)
		}
		c, _ := env.jsonContext(http.MethodPost, "/api/v1/auth/login", map[string]any{
			"email":    "analyst-main@example.com",
			"password": "Password123!",
		})
		err := env.handler.Login(c)
		if code := httpErrorCode(t, err); code != http.StatusInternalServerError {
			t.Fatalf("expected password verify error, got %d", code)
		}
		if _, err := env.pool.Exec(context.Background(), `UPDATE users SET password_hash = $2 WHERE id = $1`, env.userID, originalHash); err != nil {
			t.Fatalf("restore original hash: %v", err)
		}
	}

	{
		hash, err := security.HashPassword("Password123!")
		if err != nil {
			t.Fatalf("hash password for ldap user: %v", err)
		}
		ldapUser, err := env.users.Create(context.Background(), repository.CreateUserParams{
			Username:        "ldap-invalid-user",
			Email:           "ldap-invalid@example.com",
			FullName:        "LDAP Invalid",
			PasswordHash:    hash,
			LDAPEnabled:     true,
			IsPlatformAdmin: false,
		})
		if err != nil {
			t.Fatalf("create ldap user: %v", err)
		}
		if upsertErr := env.memberships.Upsert(context.Background(), env.tenantID, ldapUser.ID, "analyst"); upsertErr != nil {
			t.Fatalf("assign ldap user membership: %v", upsertErr)
		}
		originalLDAP := env.handler.ldap
		env.handler.ldap = auth.NewLDAPAuthenticator(config.LDAPConfig{Enabled: true})
		defer func() { env.handler.ldap = originalLDAP }()

		c, _ := env.jsonContext(http.MethodPost, "/api/v1/auth/login", map[string]any{
			"email":    "ldap-invalid@example.com",
			"password": "Password123!",
		})
		err = env.handler.Login(c)
		if code := httpErrorCode(t, err); code != http.StatusUnauthorized {
			t.Fatalf("expected ldap invalid credentials error, got %d", code)
		}
	}

	{
		originalRefresh := env.handler.refresh
		env.handler.refresh = failingRefreshStore{base: originalRefresh}
		defer func() { env.handler.refresh = originalRefresh }()

		c, _ := env.jsonContext(http.MethodPost, "/api/v1/auth/login", map[string]any{
			"email":    "analyst-main@example.com",
			"password": "Password123!",
		})
		err := env.handler.Login(c)
		if code := httpErrorCode(t, err); code != http.StatusInternalServerError {
			t.Fatalf("expected refresh create error, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodPatch, "/api/v1/users/"+env.userID.String(), map[string]any{
			"password": "short",
		})
		setPath(c, "/api/v1/users/:id", []string{"id"}, []string{env.userID.String()})
		setIdentity(c, env.identity)
		err := env.handler.UpdateUser(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("expected short password error, got %d", code)
		}
	}
}

func TestJSONAndCommunicationNormalizationBranches(t *testing.T) {
	numberMap := map[string]any{
		"id":     json.Number("10"),
		"nested": map[string]any{"v": json.Number("3.14")},
		"list":   []any{json.Number("1"), map[string]any{"k": json.Number("2")}},
	}
	normalized := normalizeJSONValue(numberMap).(map[string]any)
	if normalized["id"] != int64(10) {
		t.Fatalf("expected integer normalization")
	}
	nested := normalized["nested"].(map[string]any)
	if nested["v"] != 3.14 {
		t.Fatalf("expected float normalization")
	}

	if out := normalizeCommunicationMap(struct {
		A string `json:"a"`
	}{A: "x"}); out["a"] != "x" {
		t.Fatalf("expected struct-to-map normalization")
	}
}
