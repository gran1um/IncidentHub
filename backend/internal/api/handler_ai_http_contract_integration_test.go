package api

import (
	"bytes"
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

func TestAIHTTPEndpointContractsWithStructuredFixtures(t *testing.T) {
	env := newAPITestEnv(t)

	const askFixture = `{
  "executive_summary": "Impossible-travel sign-in pattern indicates probable credential misuse.",
  "confidence": "High",
  "recommended_action": "Escalate",
  "findings": [
    {
      "title": "Impossible travel login",
      "description": "Successful sign-ins from distant geographies in a short time window.",
      "miter": { "id": "T1078" },
      "recommendations": ["Reset active sessions", "Review risky sign-ins"],
      "indicators_of_compromise": ["user:analyst-main@example.com"]
    }
  ],
  "next_steps": ["Reset active sessions", "Review risky sign-ins", "Audit OAuth grants"]
}`

	const caseAnalyzeFixture = `{
  "classification": "suspicious",
  "confidence_level": "High",
  "assessment": "Case data is consistent with likely account compromise and requires containment.",
  "next_steps": ["Reset OAuth grants", "Force sign-out of active sessions"],
  "findings": [
    {
      "title": "Suspicious OAuth consent",
      "description": "Privileged consent recorded for unfamiliar application in tenant context."
    }
  ]
}`

	openAI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/models":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"data":[{"id":"gpt-4o-mini"}]}`)
			return
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
			if strings.TrimSpace(req.Model) == "" {
				http.Error(w, "missing model", http.StatusBadRequest)
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
		Timeout:         5 * time.Second,
		HistoryMessages: 20,
		MaxContextChars: 5000,
	}, env.searchStub)
	env.handler.ai.SetTenantContextProvider(newHandlerTenantContextProvider(env.handler))
	env.handler.cfg.AI.Enabled = true
	env.handler.cfg.AI.Model = "gpt-4o-mini"
	env.handler.cfg.AI.Timeout = 5 * time.Second
	env.handler.Register(env.e)

	httpServer := httptest.NewServer(env.e)
	defer httpServer.Close()

	token := loginAndGetAccessToken(t, httpServer.URL+"/api/v1/auth/login", "analyst-main@example.com", "Password123!")
	tenantHeader := env.tenantID.String()

	askStatus, askBody := doJSONRequest(t, httpServer.Client(), http.MethodPost, httpServer.URL+"/api/v1/ai/ask", map[string]any{
		"question": "Investigate suspicious user login behavior and OAuth abuse",
		"language": "en",
	}, token, tenantHeader)
	if askStatus != http.StatusOK {
		t.Fatalf("unexpected /ai/ask status: got=%d body=%s", askStatus, askBody.String())
	}
	var askPayload map[string]any
	if err := json.Unmarshal(askBody.Bytes(), &askPayload); err != nil {
		t.Fatalf("decode /ai/ask response: %v", err)
	}

	if strings.TrimSpace(stringValue(askPayload, "session_id")) == "" {
		t.Fatalf("expected session_id in /ai/ask response: %v", askPayload)
	}
	if strings.TrimSpace(stringValue(askPayload, "user_message_id")) == "" {
		t.Fatalf("expected user_message_id in /ai/ask response: %v", askPayload)
	}
	answerText := stringValue(askPayload, "answer")
	if !strings.Contains(answerText, "Summary: Impossible-travel sign-in pattern indicates probable credential misuse.") {
		t.Fatalf("unexpected answer summary in /ai/ask response: %q", answerText)
	}
	if !strings.Contains(answerText, "MITER T1078") {
		t.Fatalf("expected MITER mapping in /ai/ask response answer: %q", answerText)
	}
	recommendations := anyToStrings(askPayload["recommendations"])
	if len(recommendations) == 0 {
		t.Fatalf("expected recommendations in /ai/ask response: %v", askPayload)
	}
	if !containsString(recommendations, "Reset active sessions") {
		t.Fatalf("expected structured recommendation in /ai/ask response: %v", recommendations)
	}
	if confidence, ok := askPayload["confidence"].(float64); !ok || confidence <= 0 {
		t.Fatalf("expected positive confidence in /ai/ask response: %v", askPayload["confidence"])
	}
	assistantMessage, ok := askPayload["assistant_message"].(map[string]any)
	if !ok || strings.TrimSpace(stringValue(assistantMessage, "id")) == "" {
		t.Fatalf("expected assistant_message payload in /ai/ask response: %v", askPayload)
	}

	createdCase, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-AI-CONTRACT-0001",
		Title:             "Contract test case for AI analyze endpoint",
		Description:       "Validate structured JSON fixture contract",
		Source:            "manual",
		IncidentType:      "identity",
		Status:            "open",
		Priority:          "high",
		Impact:            "user_account",
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

	analyzeStatus, analyzeBody := doJSONRequest(
		t,
		httpServer.Client(),
		http.MethodPost,
		httpServer.URL+"/api/v1/cases/"+createdCase.ID.String()+"/ai/analyze",
		nil,
		token,
		tenantHeader,
	)
	if analyzeStatus != http.StatusOK {
		t.Fatalf("unexpected /cases/:id/ai/analyze status: got=%d body=%s", analyzeStatus, analyzeBody.String())
	}
	var analyzePayload map[string]any
	if err := json.Unmarshal(analyzeBody.Bytes(), &analyzePayload); err != nil {
		t.Fatalf("decode /cases/:id/ai/analyze response: %v", err)
	}

	if strings.TrimSpace(stringValue(analyzePayload, "id")) == "" {
		t.Fatalf("expected id in /cases/:id/ai/analyze response: %v", analyzePayload)
	}
	if verdict := strings.ToLower(strings.TrimSpace(stringValue(analyzePayload, "verdict"))); verdict != "suspicious" {
		t.Fatalf("unexpected verdict in /cases/:id/ai/analyze response: %q", verdict)
	}
	if status := strings.ToLower(strings.TrimSpace(stringValue(analyzePayload, "status"))); status != "completed" {
		t.Fatalf("unexpected status in /cases/:id/ai/analyze response: %q", status)
	}
	if summary := stringValue(analyzePayload, "summary"); !strings.Contains(summary, "likely account compromise") {
		t.Fatalf("unexpected summary in /cases/:id/ai/analyze response: %q", summary)
	}
	analyzeRecommendations := anyToStrings(analyzePayload["recommendations"])
	if !containsString(analyzeRecommendations, "Reset OAuth grants") {
		t.Fatalf("expected mapped recommendations from fixture: %v", analyzeRecommendations)
	}
	findings := anyToStrings(analyzePayload["findings"])
	if len(findings) == 0 || !strings.Contains(strings.ToLower(strings.Join(findings, " ")), "suspicious oauth consent") {
		t.Fatalf("expected mapped findings from fixture: %v", findings)
	}
	if confidence, ok := analyzePayload["confidence"].(float64); !ok || confidence < 60 {
		t.Fatalf("expected mapped confidence in /cases/:id/ai/analyze response: %v", analyzePayload["confidence"])
	}
}

func loginAndGetAccessToken(t *testing.T, loginURL, email, password string) string {
	t.Helper()
	status, body := doJSONRequest(t, http.DefaultClient, http.MethodPost, loginURL, map[string]any{
		"email":    email,
		"password": password,
	}, "", "")
	if status != http.StatusOK {
		t.Fatalf("login failed: status=%d body=%s", status, body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(body.Bytes(), &payload); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	token := strings.TrimSpace(stringValue(payload, "access_token"))
	if token == "" {
		t.Fatalf("missing access_token in login response: %v", payload)
	}
	return token
}

func doJSONRequest(
	t *testing.T,
	client *http.Client,
	method, url string,
	payload any,
	bearerToken, tenantID string,
) (int, *bytes.Buffer) {
	t.Helper()

	var bodyReader io.Reader = http.NoBody
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal request payload: %v", err)
		}
		bodyReader = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(bearerToken) != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}
	if strings.TrimSpace(tenantID) != "" {
		req.Header.Set(config.TenantHeader, tenantID)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("execute request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	return resp.StatusCode, bytes.NewBuffer(raw)
}

func stringValue(payload map[string]any, key string) string {
	value, ok := payload[key]
	if !ok || value == nil {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return text
}

func anyToStrings(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return []string{}
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(text)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func containsString(items []string, target string) bool {
	want := strings.TrimSpace(target)
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), want) {
			return true
		}
	}
	return false
}
