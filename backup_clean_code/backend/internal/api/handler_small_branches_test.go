package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"incidenthub/backend/internal/cache"
	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/connectors/inbound"
	"incidenthub/backend/internal/repository"

	"github.com/google/uuid"
)

func TestCollectModulesHealthOperational(t *testing.T) {
	env := newAPITestEnv(t)

	cacheClient, err := cache.New(context.Background(), config.RedisConfig{
		Addr: "localhost:6379",
	})
	if err != nil {
		t.Skipf("redis is unavailable for module-health operational test: %v", err)
	}
	defer func() { _ = cacheClient.Close() }()

	env.handler.cache = cacheClient
	env.handler.cfg.Elastic.Enabled = false
	env.handler.cfg.S3.Enabled = false
	env.handler.cfg.AI.Enabled = false

	status, modules := env.handler.collectModulesHealth(context.Background())
	if status != "ok" {
		t.Fatalf("expected healthy module status, got %s with modules=%v", status, modules)
	}
}

func TestListCaseAIAnalysesAndResolveAISessionBranches(t *testing.T) {
	env := newAPITestEnv(t)

	caseItem, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-AI-BRANCH-1",
		Title:             "AI branch case",
		Description:       "AI branch checks",
		Source:            "manual",
		IncidentType:      "generic",
		Status:            "open",
		Priority:          "medium",
		Impact:            "system",
		Confidence:        10,
		Severity:          "low",
		TLP:               "green",
		PAP:               "green",
		ResolutionSummary: "",
		CreatedBy:         env.userID,
	})
	if err != nil {
		t.Fatalf("create ai branch case: %v", err)
	}

	{
		originalRepo := env.handler.caseAIAnalyses
		env.handler.caseAIAnalyses = nil
		c, _ := env.jsonContext(http.MethodGet, "/api/v1/cases/"+caseItem.ID.String()+"/ai/analyses", nil)
		setPath(c, "/api/v1/cases/:caseID/ai/analyses", []string{"caseID"}, []string{caseItem.ID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListCaseAIAnalyses(c)
		if code := httpErrorCode(t, err); code != http.StatusServiceUnavailable {
			t.Fatalf("expected case ai repository unavailable, got %d", code)
		}
		env.handler.caseAIAnalyses = originalRepo
	}

	{
		c, _ := env.jsonContext(http.MethodGet, "/api/v1/cases/not-uuid/ai/analyses", nil)
		setPath(c, "/api/v1/cases/:caseID/ai/analyses", []string{"caseID"}, []string{"not-uuid"})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListCaseAIAnalyses(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("expected invalid case id for ai analyses, got %d", code)
		}
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/cases/"+caseItem.ID.String()+"/ai/analyses?limit=9999", nil)
		c.Request().URL.RawQuery = "limit=9999"
		setPath(c, "/api/v1/cases/:caseID/ai/analyses", []string{"caseID"}, []string{caseItem.ID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListCaseAIAnalyses(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	{
		c, _ := env.jsonContext(http.MethodGet, "/api/v1/ai/messages", nil)
		_, err := env.handler.resolveAISession(c, env.tenantID, env.userID, "bad-session-id")
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("expected invalid session id branch, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodGet, "/api/v1/ai/messages", nil)
		_, err := env.handler.resolveAISession(c, env.tenantID, env.userID, uuid.NewString())
		if code := httpErrorCode(t, err); code != http.StatusNotFound {
			t.Fatalf("expected ai session not found branch, got %d", code)
		}
	}

	{
		original := env.handler.aiChats
		env.handler.aiChats = nil
		c, _ := env.jsonContext(http.MethodGet, "/api/v1/ai/sessions", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListAISessions(c)
		if code := httpErrorCode(t, err); code != http.StatusServiceUnavailable {
			t.Fatalf("expected ai sessions repository unavailable, got %d", code)
		}
		env.handler.aiChats = original
	}

	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/ai/sessions", map[string]any{"title": "Branch Session"})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.CreateAISession(c)
		mustStatusOK(t, err, rec, http.StatusCreated)
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/ai/sessions?limit=5", nil)
		c.Request().URL.RawQuery = "limit=5"
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListAISessions(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	{
		c, _ := env.jsonContext(http.MethodPatch, "/api/v1/ai/sessions/reorder", map[string]any{
			"session_ids": []string{"bad-session-id"},
		})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ReorderAISessions(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("expected invalid ai session id on reorder, got %d", code)
		}
	}

	{
		original := env.handler.aiChats
		env.handler.aiChats = nil
		sessionID := uuid.NewString()
		c, _ := env.jsonContext(http.MethodDelete, "/api/v1/ai/sessions/"+sessionID, nil)
		setPath(c, "/api/v1/ai/sessions/:sessionID", []string{"sessionID"}, []string{sessionID})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.DeleteAISession(c)
		if code := httpErrorCode(t, err); code != http.StatusServiceUnavailable {
			t.Fatalf("expected ai sessions repository unavailable on delete, got %d", code)
		}
		env.handler.aiChats = original
	}

	{
		c, _ := env.jsonContext(http.MethodDelete, "/api/v1/ai/sessions/not-uuid", nil)
		setPath(c, "/api/v1/ai/sessions/:sessionID", []string{"sessionID"}, []string{"not-uuid"})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.DeleteAISession(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("expected invalid ai session id on delete, got %d", code)
		}
	}

	{
		missingID := uuid.NewString()
		c, _ := env.jsonContext(http.MethodDelete, "/api/v1/ai/sessions/"+missingID, nil)
		setPath(c, "/api/v1/ai/sessions/:sessionID", []string{"sessionID"}, []string{missingID})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.DeleteAISession(c)
		if code := httpErrorCode(t, err); code != http.StatusNotFound {
			t.Fatalf("expected missing ai session on delete, got %d", code)
		}
	}
}

func TestRunInboundConnectorExtraBranches(t *testing.T) {
	env := newAPITestEnv(t)

	env.handler.inboundWorker = inbound.NewWorker(
		config.InboundConnectorsConfig{
			Enabled:      true,
			BatchLimit:   50,
			HTTPTimeout:  time.Second,
			PollInterval: time.Minute,
		},
		env.catalog,
		env.alerts,
		repository.NewConnectorIngestStateRepository(env.pool),
		repository.NewInboundConnectorRunRepository(env.pool),
		nil,
	)

	{
		c, _ := env.jsonContext(http.MethodPost, "/api/v1/connectors/inbound/run", nil)
		err := env.handler.RunInboundConnectors(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("expected tenant required on inbound run, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodPost, "/api/v1/connectors/inbound/not-uuid/run", nil)
		setPath(c, "/api/v1/connectors/inbound/:connectorID/run", []string{"connectorID"}, []string{"not-uuid"})
		setTenant(c, env.tenantID)
		err := env.handler.RunInboundConnector(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("expected invalid connector id branch, got %d", code)
		}
	}
}

func TestResolveAskLanguageByQuestionText(t *testing.T) {
	if lang := resolveAskLanguage("привет, сколько кейсов в работе?", "en"); lang != "ru" {
		t.Fatalf("expected ru language for cyrillic question, got %s", lang)
	}
	if lang := resolveAskLanguage("how many cases are in work?", "ru"); lang != "en" {
		t.Fatalf("expected en language for latin question, got %s", lang)
	}
	if lang := resolveAskLanguage("12345 ???", "ru"); lang != "ru" {
		t.Fatalf("expected fallback language when question has no letters, got %s", lang)
	}
}
