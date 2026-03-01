package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"incidenthub/backend/internal/ai"
	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/repository"

	"github.com/google/uuid"
)

func TestHandlerAIAndCaseAnalysisIntegration(t *testing.T) {
	env := newAPITestEnv(t)

	const askFixture = `{
  "executive_summary": "Test summary.",
  "confidence": "High",
  "recommended_action": "Investigate",
  "findings": [
    {
      "title": "Impossible travel login",
      "description": "Successful sign-ins from distant geographies in a short time window.",
      "miter": { "id": "T1078" }
    }
  ],
  "next_steps": ["Reset sessions"]
}`

	const caseAnalyzeFixture = `{
  "classification": "suspicious",
  "confidence_level": "High",
  "assessment": "Case data is consistent with likely account compromise and requires containment.",
  "next_steps": ["Reset OAuth grants"],
  "findings": [
    {
      "title": "Suspicious OAuth consent",
      "description": "Privileged consent recorded for unfamiliar application in tenant context."
    }
  ]
}`

	openAI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/chat/completions":
			if got := strings.TrimSpace(r.Header.Get("Authorization")); got != "Bearer test-openai-key" {
				http.Error(w, "missing auth", http.StatusUnauthorized)
				return
			}
			var req struct {
				Model    string `json:"model"`
				Messages []struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"messages"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "invalid body", http.StatusBadRequest)
				return
			}
			lastUserPrompt := ""
			for i := len(req.Messages) - 1; i >= 0; i-- {
				if strings.EqualFold(strings.TrimSpace(req.Messages[i].Role), "user") {
					lastUserPrompt = req.Messages[i].Content
					break
				}
			}
			fixture := askFixture
			if strings.Contains(lastUserPrompt, "Case Context:") {
				fixture = caseAnalyzeFixture
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"choices":[{"message":{"role":"assistant","content":%q}}]}`, fixture)
			return
		case "/models":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"data":[{"id":"gpt-4o-mini"}]}`)
			return
		default:
			http.NotFound(w, r)
			return
		}
	}))
	defer openAI.Close()

	env.handler.ai = ai.NewService(config.AIConfig{
		Enabled:         true,
		Provider:        "openai",
		Endpoint:        openAI.URL,
		APIKey:          "test-openai-key",
		Model:           "gpt-4o-mini",
		TopK:            6,
		Timeout:         3 * time.Second,
		HistoryMessages: 20,
		MaxContextChars: 5000,
	}, env.searchStub)
	env.handler.ai.SetTenantContextProvider(newHandlerTenantContextProvider(env.handler))
	env.handler.cfg.AI.Enabled = true
	env.handler.cfg.AI.Model = "gpt-4o-mini"
	env.handler.cfg.AI.Timeout = 3 * time.Second

	var sessionID string
	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/ai/sessions?limit=10", nil)
		c.Request().URL.RawQuery = "limit=10"
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListAISessions(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		sessions := decodeBody[[]map[string]any](t, rec)
		if len(sessions) != 0 {
			t.Fatalf("expected no ai sessions before first create, got %d", len(sessions))
		}
	}

	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/ai/sessions", map[string]any{
			"title": "Threat Hunting Chat",
		})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.CreateAISession(c)
		mustStatusOK(t, err, rec, http.StatusCreated)
		payload := decodeBody[map[string]any](t, rec)
		if v, ok := payload["id"].(string); ok {
			sessionID = v
		}
		if sessionID == "" {
			t.Fatalf("empty created ai session id")
		}
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/ai/session", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.GetAISession(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		payload := decodeBody[map[string]any](t, rec)
		defaultSessionID := ""
		if v, ok := payload["id"].(string); ok {
			defaultSessionID = v
		}
		if defaultSessionID == "" {
			t.Fatalf("empty default ai session id")
		}
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/ai/messages?session_id="+sessionID, nil)
		c.Request().URL.RawQuery = "session_id=" + sessionID + "&limit=20"
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListAIMessages(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/ai/ask", map[string]any{
			"session_id": sessionID,
			"question":   "What should I do with suspicious IOC in this tenant?",
			"language":   "en",
		})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.AskAI(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	createdCase, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-AI-0001",
		Title:             "AI analysis case",
		Description:       "Case to verify analysis flow",
		Source:            "manual",
		IncidentType:      "malware",
		Status:            "open",
		Priority:          "high",
		Impact:            "server",
		Confidence:        80,
		Severity:          "high",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.userID,
	})
	if err != nil {
		t.Fatalf("create setup case: %v", err)
	}

	_, err = env.tasks.Create(context.Background(), repository.CreateTaskParams{
		CaseID:      createdCase.ID,
		TenantID:    env.tenantID,
		Title:       "Contain host",
		Description: "Isolate impacted host from network",
		Status:      "new",
		AssigneeID:  &env.userID,
	})
	if err != nil {
		t.Fatalf("create setup task: %v", err)
	}

	_, err = env.observables.Create(context.Background(), repository.CreateObservableParams{
		TenantID:  env.tenantID,
		CaseID:    createdCase.ID,
		Type:      "hash",
		Value:     "44d88612fea8a8f36de82e1278abb02f",
		Verdict:   "malicious",
		Source:    "edr",
		Tags:      []string{"ioc"},
		CreatedBy: env.userID,
	})
	if err != nil {
		t.Fatalf("create setup observable: %v", err)
	}

	_, err = env.caseEvents.Create(context.Background(), repository.CreateCaseEventParams{
		TenantID:  env.tenantID,
		CaseID:    createdCase.ID,
		EventType: "note",
		Title:     "Triage note",
		Body:      "Malicious hash found on endpoint",
		ActorID:   &env.userID,
		Metadata:  map[string]any{"source": "integration"},
	})
	if err != nil {
		t.Fatalf("create setup event: %v", err)
	}

	_, err = env.casePages.Create(context.Background(), repository.CreateCasePageParams{
		TenantID:  env.tenantID,
		CaseID:    createdCase.ID,
		Title:     "Response plan",
		Body:      "Containment + eradication",
		CreatedBy: env.userID,
	})
	if err != nil {
		t.Fatalf("create setup page: %v", err)
	}

	_, err = env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID:  &env.tenantID,
		Kind:      "case_comment",
		OwnerID:   &env.userID,
		RefID:     &createdCase.ID,
		CreatedBy: &env.userID,
		Data: map[string]any{
			"case_id":    createdCase.ID.String(),
			"content":    "Need immediate containment",
			"created_at": time.Now().UTC().Format(time.RFC3339),
		},
	})
	if err != nil {
		t.Fatalf("create setup comment: %v", err)
	}

	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/cases/"+createdCase.ID.String()+"/ai/analyze", nil)
		setPath(c, "/api/v1/cases/:caseID/ai/analyze", []string{"caseID"}, []string{createdCase.ID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.AnalyzeCaseWithAI(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/cases/"+createdCase.ID.String()+"/ai/analyses", nil)
		setPath(c, "/api/v1/cases/:caseID/ai/analyses", []string{"caseID"}, []string{createdCase.ID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListCaseAIAnalyses(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/ai/messages", nil)
		c.Request().URL.RawQuery = "session_id=" + uuid.NewString()
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListAIMessages(c)
		if err == nil {
			t.Fatalf("expected list ai messages error for missing session")
		}
		_ = rec
	}

	{
		c, rec := env.jsonContext(http.MethodDelete, "/api/v1/ai/sessions/"+sessionID, nil)
		setPath(c, "/api/v1/ai/sessions/:sessionID", []string{"sessionID"}, []string{sessionID})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.DeleteAISession(c)
		mustStatusOK(t, err, rec, http.StatusNoContent)
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/ai/sessions?limit=10", nil)
		c.Request().URL.RawQuery = "limit=10"
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListAISessions(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		sessions := decodeBody[[]map[string]any](t, rec)
		for _, session := range sessions {
			if session["id"] == sessionID {
				t.Fatalf("expected deleted ai session to be absent from list")
			}
		}
	}

	var firstOrderID, secondOrderID string
	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/ai/sessions", map[string]any{"title": "Order #1"})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.CreateAISession(c)
		mustStatusOK(t, err, rec, http.StatusCreated)
		payload := decodeBody[map[string]any](t, rec)
		firstOrderID = stringFromAny(payload["id"])
		if firstOrderID == "" {
			t.Fatalf("expected first ordered session id")
		}
	}
	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/ai/sessions", map[string]any{"title": "Order #2"})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.CreateAISession(c)
		mustStatusOK(t, err, rec, http.StatusCreated)
		payload := decodeBody[map[string]any](t, rec)
		secondOrderID = stringFromAny(payload["id"])
		if secondOrderID == "" {
			t.Fatalf("expected second ordered session id")
		}
	}
	{
		c, rec := env.jsonContext(http.MethodPatch, "/api/v1/ai/sessions/reorder", map[string]any{
			"session_ids": []string{firstOrderID, secondOrderID},
		})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ReorderAISessions(c)
		mustStatusOK(t, err, rec, http.StatusNoContent)
	}
	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/ai/sessions?limit=10", nil)
		c.Request().URL.RawQuery = "limit=10"
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListAISessions(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		sessions := decodeBody[[]map[string]any](t, rec)
		if len(sessions) < 2 {
			t.Fatalf("expected reordered sessions in list, got %d", len(sessions))
		}
		if stringFromAny(sessions[0]["id"]) != firstOrderID {
			t.Fatalf("expected first session to be reordered to the top")
		}
	}
}
