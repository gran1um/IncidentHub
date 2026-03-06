package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"incidenthub/backend/internal/repository"

	"github.com/google/uuid"
)

func TestRunAndListAIAgentRunsIntegration(t *testing.T) {
	env := newAPITestEnv(t)

	taggedCase, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-AI-0001",
		Title:             "Phishing investigation",
		Description:       "Suspicious email reported by executive user",
		Source:            "manual",
		IncidentType:      "phishing",
		Status:            "open",
		Priority:          "high",
		Impact:            "user",
		Confidence:        70,
		Severity:          "high",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.userID,
	})
	if err != nil {
		t.Fatalf("create tagged case: %v", err)
	}

	untaggedCase, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-AI-0002",
		Title:             "Routine endpoint alert",
		Description:       "Case without matching ai tag",
		Source:            "manual",
		IncidentType:      "malware",
		Status:            "open",
		Priority:          "medium",
		Impact:            "endpoint",
		Confidence:        55,
		Severity:          "medium",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.userID,
	})
	if err != nil {
		t.Fatalf("create untagged case: %v", err)
	}

	if _, metaErr := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID: &env.tenantID,
		Kind:     "case_meta",
		OwnerID:  &env.userID,
		RefID:    &taggedCase.ID,
		Data: map[string]any{
			"case_id":   taggedCase.ID.String(),
			"tenant_id": env.tenantID.String(),
			"tags":      []string{"phishing", "vip"},
		},
		CreatedBy: &env.userID,
	}); metaErr != nil {
		t.Fatalf("create tagged case meta: %v", metaErr)
	}
	if _, metaErr := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID: &env.tenantID,
		Kind:     "case_meta",
		OwnerID:  &env.userID,
		RefID:    &untaggedCase.ID,
		Data: map[string]any{
			"case_id":   untaggedCase.ID.String(),
			"tenant_id": env.tenantID.String(),
			"tags":      []string{"endpoint"},
		},
		CreatedBy: &env.userID,
	}); metaErr != nil {
		t.Fatalf("create untagged case meta: %v", metaErr)
	}

	agentItem, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID: &env.tenantID,
		Kind:     "ai_agents",
		OwnerID:  &env.userID,
		Data: map[string]any{
			"name":                   "Phishing agent",
			"description":            "Automated phishing triage",
			"enabled":                true,
			"case_tags":              []string{"phishing"},
			"max_cases_per_run":      5,
			"auto_create_tasks":      false,
			"auto_comment":           false,
			"task_assignee_id":       "",
			"connector_ids":          []string{},
			"enrichmentConnectorIds": []string{},
		},
		CreatedBy: &env.userID,
	})
	if err != nil {
		t.Fatalf("create ai agent: %v", err)
	}

	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/ai/agents/"+agentItem.ID.String()+"/run", map[string]any{
			"dry_run":   true,
			"max_cases": 3,
			"language":  "en",
		})
		setPath(c, "/api/v1/ai/agents/:agentID/run", []string{"agentID"}, []string{agentItem.ID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)

		err := env.handler.RunAIAgent(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		payload := decodeBody[map[string]any](t, rec)

		if runID := fmt.Sprint(payload["run_id"]); runID != "" && runID != "<nil>" {
			t.Fatalf("expected empty run_id for dry run, got %q", runID)
		}
		processed, _ := payload["processed_cases"].(float64)
		if int(processed) != 1 {
			t.Fatalf("expected exactly one processed case for phishing tag, got %v", payload["processed_cases"])
		}
	}

	var persistedRunID uuid.UUID
	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/ai/agents/"+agentItem.ID.String()+"/run", map[string]any{
			"dry_run": false,
		})
		setPath(c, "/api/v1/ai/agents/:agentID/run", []string{"agentID"}, []string{agentItem.ID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)

		err := env.handler.RunAIAgent(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		payload := decodeBody[map[string]any](t, rec)

		runIDRaw := fmt.Sprint(payload["run_id"])
		parsedRunID, parseErr := uuid.Parse(runIDRaw)
		if parseErr != nil {
			t.Fatalf("expected persisted run id, got %q: %v", runIDRaw, parseErr)
		}
		persistedRunID = parsedRunID
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/ai/agents/"+agentItem.ID.String()+"/runs?limit=20", nil)
		setPath(c, "/api/v1/ai/agents/:agentID/runs", []string{"agentID"}, []string{agentItem.ID.String()})
		c.Request().URL.RawQuery = "limit=20"
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)

		err := env.handler.ListAIAgentRuns(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		runs := decodeBody[[]map[string]any](t, rec)
		if len(runs) == 0 {
			t.Fatalf("expected at least one ai agent run")
		}

		foundPersistedRun := false
		for _, item := range runs {
			if fmt.Sprint(item["id"]) == persistedRunID.String() {
				foundPersistedRun = true
				break
			}
		}
		if !foundPersistedRun {
			t.Fatalf("persisted run id %s not found in ai agent runs list", persistedRunID.String())
		}
	}
}

func TestRunAIAgentUsesAgentRuntimeOverrides(t *testing.T) {
	env := newAPITestEnv(t)

	modelSeen := ""
	openAI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/chat/completions":
			var req struct {
				Model string `json:"model"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "invalid body", http.StatusBadRequest)
				return
			}
			modelSeen = req.Model
			w.Header().Set("Content-Type", "application/json")
			completion := `{
				"verdict": "suspicious",
				"confidence": 81,
				"summary": "Agent override model executed",
				"recommendations": ["Reset sessions"],
				"findings": ["Suspicious authentication chain"]
			}`
			resp := fmt.Sprintf(`{"choices":[{"message":{"role":"assistant","content":%q}}]}`, completion)
			_, _ = io.WriteString(w, resp)
		default:
			http.NotFound(w, r)
		}
	}))
	defer openAI.Close()

	taggedCase, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-AI-OVERRIDE-0001",
		Title:             "Token theft candidate",
		Description:       "Suspicious sign-in and token usage pattern",
		Source:            "manual",
		IncidentType:      "identity",
		Status:            "open",
		Priority:          "high",
		Impact:            "user",
		Confidence:        70,
		Severity:          "high",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.userID,
	})
	if err != nil {
		t.Fatalf("create tagged case: %v", err)
	}
	if _, metaErr := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID: &env.tenantID,
		Kind:     "case_meta",
		OwnerID:  &env.userID,
		RefID:    &taggedCase.ID,
		Data: map[string]any{
			"case_id":   taggedCase.ID.String(),
			"tenant_id": env.tenantID.String(),
			"tags":      []string{"identity-abuse"},
		},
		CreatedBy: &env.userID,
	}); metaErr != nil {
		t.Fatalf("create case meta: %v", metaErr)
	}

	overrideModel := "cyankiwi/Qwen3.5-27B-AWQ-4bit"
	overrideEndpoint := openAI.URL + "/v1"
	agentItem, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID: &env.tenantID,
		Kind:     "ai_agents",
		OwnerID:  &env.userID,
		Data: map[string]any{
			"name":              "Custom endpoint agent",
			"enabled":           true,
			"case_tags":         []string{"identity-abuse"},
			"max_cases_per_run": 2,
			"provider":          "openai",
			"endpoint":          overrideEndpoint,
			"model":             overrideModel,
			"auto_create_tasks": false,
			"auto_comment":      false,
		},
		CreatedBy: &env.userID,
	})
	if err != nil {
		t.Fatalf("create ai agent: %v", err)
	}

	c, rec := env.jsonContext(http.MethodPost, "/api/v1/ai/agents/"+agentItem.ID.String()+"/run", map[string]any{
		"dry_run": true,
	})
	setPath(c, "/api/v1/ai/agents/:agentID/run", []string{"agentID"}, []string{agentItem.ID.String()})
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)

	err = env.handler.RunAIAgent(c)
	mustStatusOK(t, err, rec, http.StatusOK)
	payload := decodeBody[map[string]any](t, rec)
	processed, _ := payload["processed_cases"].(float64)
	if int(processed) != 1 {
		t.Fatalf("expected one processed case, got %v", payload["processed_cases"])
	}

	if modelSeen != overrideModel {
		t.Fatalf("expected override model %q to be used, got %q", overrideModel, modelSeen)
	}

	results, ok := payload["results"].([]any)
	if !ok || len(results) != 1 {
		t.Fatalf("expected one result payload, got %v", payload["results"])
	}
	first, ok := results[0].(map[string]any)
	if !ok {
		t.Fatalf("expected map result payload, got %T", results[0])
	}
	if errText := fmt.Sprint(first["error"]); errText != "" && errText != "<nil>" {
		t.Fatalf("expected successful execution, got error %q", errText)
	}
}

func TestRunAIAgentWithRequestedAlertsIntegration(t *testing.T) {
	env := newAPITestEnv(t)

	openAI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/chat/completions":
			w.Header().Set("Content-Type", "application/json")
			completion := `{
	"verdict": "suspicious",
	"confidence": 87,
	"summary": "Alert was converted to a case and reviewed",
	"recommendations": ["Investigate the host"],
	"findings": ["EDR alert escalated into active investigation"]
}`
			resp := fmt.Sprintf(`{"choices":[{"message":{"role":"assistant","content":%q}}]}`, completion)
			_, _ = io.WriteString(w, resp)
		default:
			http.NotFound(w, r)
		}
	}))
	defer openAI.Close()

	alert, err := env.alerts.Create(context.Background(), repository.CreateAlertParams{
		TenantID:    env.tenantID,
		Title:       "Manual alert run",
		Description: "Alert should resolve into a case during manual run",
		Source:      "edr",
		Status:      "new",
		Severity:    "high",
		TLP:         "amber",
		PAP:         "amber",
		CreatedBy:   &env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create alert seed: %v", err)
	}

	agentItem, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID: &env.tenantID,
		Kind:     "ai_agents",
		OwnerID:  &env.identity.UserID,
		Data: map[string]any{
			"name":                        "Alert manual run agent",
			"enabled":                     true,
			"target_types":                []string{"alert"},
			"auto_create_case_from_alert": true,
			"provider":                    "openai",
			"endpoint":                    openAI.URL + "/v1",
			"model":                       "cyankiwi/Qwen3.5-27B-AWQ-4bit",
			"auto_create_tasks":           false,
			"auto_comment":                false,
			"max_cases_per_run":           1,
		},
		CreatedBy: &env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create ai agent: %v", err)
	}

	c, rec := env.jsonContext(http.MethodPost, "/api/v1/ai/agents/"+agentItem.ID.String()+"/run", map[string]any{
		"dry_run":   false,
		"alert_ids": []string{alert.ID.String()},
	})
	setPath(c, "/api/v1/ai/agents/:agentID/run", []string{"agentID"}, []string{agentItem.ID.String()})
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)

	err = env.handler.RunAIAgent(c)
	mustStatusOK(t, err, rec, http.StatusOK)
	payload := decodeBody[map[string]any](t, rec)

	if processed, _ := payload["processed_cases"].(float64); int(processed) != 1 {
		t.Fatalf("expected one processed case from alert_ids, got %v", payload["processed_cases"])
	}
	requestedAlerts, ok := payload["requested_alert_ids"].([]any)
	if !ok || len(requestedAlerts) != 1 || fmt.Sprint(requestedAlerts[0]) != alert.ID.String() {
		t.Fatalf("expected requested_alert_ids to echo alert id, got %#v", payload["requested_alert_ids"])
	}
	results, ok := payload["results"].([]any)
	if !ok || len(results) != 1 {
		t.Fatalf("expected single result entry, got %#v", payload["results"])
	}
	result := normalizeMap(results[0])
	if got := fmt.Sprint(result["case_id"]); got == "" || got == "<nil>" {
		t.Fatalf("expected resolved case_id in result, got %#v", result)
	}

	reloadedAlert, err := env.alerts.GetByID(context.Background(), env.tenantID, alert.ID)
	if err != nil {
		t.Fatalf("reload alert: %v", err)
	}
	if reloadedAlert.CaseID == nil {
		t.Fatalf("expected alert to be linked to created case after manual run")
	}
}
