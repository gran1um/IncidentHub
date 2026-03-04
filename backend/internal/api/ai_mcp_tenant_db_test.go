package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"incidenthub/backend/internal/repository"
)

func TestHandlerTenantContextProviderMCPAllowlist(t *testing.T) {
	env := newAPITestEnv(t)
	env.handler.cfg.AI.MCPEnabled = true
	env.handler.cfg.AI.MCPTools = "recent_cases"
	env.handler.cfg.AI.MCPPerToolLimit = 5
	env.handler.aiTenantMCP = newTenantMCPServer(env.handler)

	_, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-MCP-ALLOW-001",
		Title:             "MCP allowlist case",
		Description:       "Allowlist test case",
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
		t.Fatalf("create setup case: %v", err)
	}

	provider := newHandlerTenantContextProvider(env.handler)
	ctxPayload, err := provider.BuildContext(context.Background(), env.tenantID.String(), "show tenant context")
	if err != nil {
		t.Fatalf("build context: %v", err)
	}

	payload := decodePromptJSON(t, ctxPayload.Prompt)
	if _, ok := payload["recent_cases"]; !ok {
		t.Fatalf("expected recent_cases in MCP context: %v", payload)
	}
	if _, ok := payload["recent_alerts"]; ok {
		t.Fatalf("did not expect recent_alerts when not allowed: %v", payload)
	}
	if _, ok := payload["dashboard_stats"]; ok {
		t.Fatalf("did not expect dashboard_stats when not allowed: %v", payload)
	}
	if _, ok := payload["cases_in_work"]; ok {
		t.Fatalf("did not expect cases_in_work when not allowed: %v", payload)
	}

	mcp, ok := payload["mcp"].(map[string]any)
	if !ok {
		t.Fatalf("expected mcp metadata block: %v", payload["mcp"])
	}
	allowlist := toStringSlice(mcp["allowlist"])
	if len(allowlist) != 1 || allowlist[0] != "recent_cases" {
		t.Fatalf("unexpected MCP allowlist: %v", allowlist)
	}
}

func TestTenantMCPServerEnforcesTenantScopeAndMetadata(t *testing.T) {
	env := newAPITestEnv(t)
	env.handler.cfg.AI.MCPEnabled = true
	env.handler.cfg.AI.MCPTools = "open_case_statuses,cases_in_work,recent_cases,recent_alerts,dashboard_stats"
	env.handler.cfg.AI.MCPPerToolLimit = 5
	env.handler.aiTenantMCP = newTenantMCPServer(env.handler)

	mainCase, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-MCP-SCOPE-MAIN",
		Title:             "Main tenant case",
		Description:       "Main tenant setup",
		Source:            "manual",
		IncidentType:      "generic",
		Status:            "open",
		Priority:          "high",
		Impact:            "system",
		Confidence:        70,
		Severity:          "high",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.userID,
	})
	if err != nil {
		t.Fatalf("create main tenant case: %v", err)
	}

	_, err = env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.secondTenant,
		CaseNumber:        "CASE-MCP-SCOPE-SECOND",
		Title:             "Second tenant case",
		Description:       "Second tenant setup",
		Source:            "manual",
		IncidentType:      "generic",
		Status:            "open",
		Priority:          "low",
		Impact:            "system",
		Confidence:        50,
		Severity:          "low",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.secondUserID,
	})
	if err != nil {
		t.Fatalf("create second tenant case: %v", err)
	}

	result := env.handler.aiTenantMCP.BuildTenantContext(context.Background(), env.tenantID)
	if !result.Enabled {
		t.Fatalf("expected MCP server to be enabled")
	}
	if len(result.Invocations) == 0 {
		t.Fatalf("expected invocation metadata")
	}
	for _, invocation := range result.Invocations {
		if !invocation.ReadOnly {
			t.Fatalf("expected read_only=true for tool %s", invocation.Tool)
		}
		if !invocation.TenantScoped {
			t.Fatalf("expected tenant_scoped=true for tool %s", invocation.Tool)
		}
		if !strings.EqualFold(invocation.Status, "ok") {
			t.Fatalf("expected successful invocation for tool %s, got status=%s error=%s", invocation.Tool, invocation.Status, invocation.Error)
		}
	}

	recentCasesRaw, ok := result.Data["recent_cases"]
	if !ok {
		t.Fatalf("expected recent_cases in MCP result data: %v", result.Data)
	}
	recentCases, ok := recentCasesRaw.([]map[string]any)
	if !ok {
		t.Fatalf("unexpected recent_cases payload type: %T", recentCasesRaw)
	}
	if len(recentCases) == 0 {
		t.Fatalf("expected at least one recent case in MCP result")
	}
	hasMainCase := false
	for _, item := range recentCases {
		caseID, _ := item["id"].(string)
		if caseID == mainCase.ID.String() {
			hasMainCase = true
			break
		}
	}
	if !hasMainCase {
		t.Fatalf("expected main tenant case in recent_cases: %v", recentCases)
	}
	for _, item := range recentCases {
		caseNumber, _ := item["case_number"].(string)
		if strings.EqualFold(strings.TrimSpace(caseNumber), "CASE-MCP-SCOPE-SECOND") {
			t.Fatalf("unexpected second tenant case in tenant-scoped MCP result: %v", recentCases)
		}
	}
}

func TestTenantMCPServerRespectsPerToolLimit(t *testing.T) {
	env := newAPITestEnv(t)
	env.handler.cfg.AI.MCPEnabled = true
	env.handler.cfg.AI.MCPTools = "recent_cases"
	env.handler.cfg.AI.MCPPerToolLimit = 1
	env.handler.aiTenantMCP = newTenantMCPServer(env.handler)

	for idx := 1; idx <= 3; idx++ {
		_, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
			TenantID:          env.tenantID,
			CaseNumber:        fmt.Sprintf("CASE-MCP-LIMIT-%03d", idx),
			Title:             "MCP per-tool limit case",
			Description:       "Per-tool limit setup",
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
			t.Fatalf("create case %d: %v", idx, err)
		}
	}

	result := env.handler.aiTenantMCP.BuildTenantContext(context.Background(), env.tenantID)
	recentCasesRaw, ok := result.Data["recent_cases"]
	if !ok {
		t.Fatalf("expected recent_cases data from MCP result")
	}
	recentCases, ok := recentCasesRaw.([]map[string]any)
	if !ok {
		t.Fatalf("unexpected recent_cases payload type: %T", recentCasesRaw)
	}
	if len(recentCases) != 1 {
		t.Fatalf("expected per-tool limit of 1, got %d items", len(recentCases))
	}
}

func decodePromptJSON(t *testing.T, raw string) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("decode prompt json: %v; payload=%s", err, raw)
	}
	return payload
}

func toStringSlice(value any) []string {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		text, ok := item.(string)
		if !ok {
			continue
		}
		out = append(out, text)
	}
	return out
}
