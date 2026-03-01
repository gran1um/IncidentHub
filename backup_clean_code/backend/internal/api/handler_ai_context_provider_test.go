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
)

func TestHandlerTenantContextProviderBuildsPostgresContext(t *testing.T) {
	env := newAPITestEnv(t)

	_, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-AI-CONTEXT-001",
		Title:             "AI Context Case",
		Description:       "Context builder case",
		Source:            "manual",
		IncidentType:      "generic",
		Status:            "open",
		Priority:          "medium",
		Impact:            "system",
		Confidence:        60,
		Severity:          "medium",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.userID,
	})
	if err != nil {
		t.Fatalf("create case for context provider: %v", err)
	}

	provider := newHandlerTenantContextProvider(env.handler)
	ctxPayload, err := provider.BuildContext(context.Background(), env.tenantID.String(), "сколько кейсов в работе")
	if err != nil {
		t.Fatalf("build context: %v", err)
	}
	if strings.TrimSpace(ctxPayload.Prompt) == "" {
		t.Fatalf("expected non-empty postgres context prompt")
	}
	if !strings.Contains(ctxPayload.Prompt, "\"cases_in_work\":") {
		t.Fatalf("expected cases_in_work in postgres context, got %s", ctxPayload.Prompt)
	}
	if len(ctxPayload.Sources) == 0 || ctxPayload.Sources[0].Kind != "postgres" {
		t.Fatalf("expected postgres source, got %+v", ctxPayload.Sources)
	}
}

func TestAskAIIncludesPostgresContextSource(t *testing.T) {
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
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"choices":[{"message":{"role":"assistant","content":%q}}]}`, askFixture)
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

	_, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-AI-CONTEXT-002",
		Title:             "Another context case",
		Description:       "Used for source merge",
		Source:            "manual",
		IncidentType:      "generic",
		Status:            "open",
		Priority:          "medium",
		Impact:            "system",
		Confidence:        50,
		Severity:          "medium",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.userID,
	})
	if err != nil {
		t.Fatalf("create case: %v", err)
	}

	c, rec := env.jsonContext(http.MethodPost, "/api/v1/ai/ask", map[string]any{
		"question": "сколько кейсов в работе?",
	})
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)

	err = env.handler.AskAI(c)
	mustStatusOK(t, err, rec, http.StatusOK)

	payload := decodeBody[map[string]any](t, rec)
	sourcesAny, ok := payload["sources"].([]any)
	if !ok || len(sourcesAny) == 0 {
		t.Fatalf("expected sources in ai response, got %#v", payload["sources"])
	}

	foundPostgres := false
	for _, raw := range sourcesAny {
		item, castOK := raw.(map[string]any)
		if !castOK {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(stringFromAny(item["kind"])), "postgres") {
			foundPostgres = true
			break
		}
	}
	if !foundPostgres {
		t.Fatalf("expected postgres source in ai response, sources=%#v", sourcesAny)
	}
}

func stringFromAny(value any) string {
	if value == nil {
		return ""
	}
	if typed, ok := value.(string); ok {
		return typed
	}
	return ""
}
