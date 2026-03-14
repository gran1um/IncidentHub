package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"incidenthub/backend/internal/repository"
	"incidenthub/backend/internal/workflow"
	"incidenthub/backend/internal/workflowvault"

	"github.com/google/uuid"
)

func TestAIAgentEnrichmentCreatesDurableConnectorHubExecutionAndCaseTimeline(t *testing.T) {
	env := newAPITestEnv(t)

	caseItem, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-AI-ENRICH-1",
		Title:             "AI enrichment durable execution",
		Description:       "Validate AI connector execution path",
		Source:            "manual",
		IncidentType:      "malware",
		Status:            "open",
		Priority:          "high",
		Impact:            "endpoint",
		Confidence:        70,
		Severity:          "high",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.userID,
	})
	if err != nil {
		t.Fatalf("create case: %v", err)
	}

	connector, _ := createConnectorHubTestConnector(t, env, "mock", map[string]any{})
	agentID := uuid.New()
	results := env.handler.runAIAgentEnrichment(context.Background(), env.tenantID, []string{connector.ID.String()}, aiAgentDefinition{
		ID:   agentID,
		Name: "Durable enrichment agent",
	}, *caseItem, nil, "Inspect observables")
	if len(results) != 1 {
		t.Fatalf("expected one enrichment result, got %d", len(results))
	}
	if status := strings.ToLower(strings.TrimSpace(stringFromMap(results[0], "status"))); status != "completed" {
		t.Fatalf("expected completed enrichment status, got %q", results[0]["status"])
	}
	executionID := strings.TrimSpace(stringFromMap(results[0], "execution_id", "id"))
	if executionID == "" {
		t.Fatalf("expected execution_id in enrichment result")
	}

	executions, err := env.handler.connectorHubExecutions.List(context.Background(), repository.ConnectorHubExecutionListParams{
		TenantID:      env.tenantID,
		CaseID:        &caseItem.ID,
		ExecutionMode: "ai_agent",
		Limit:         20,
	})
	if err != nil {
		t.Fatalf("list connector hub executions: %v", err)
	}
	if len(executions) == 0 {
		t.Fatalf("expected durable connector hub execution for case")
	}
	if executions[0].Status != "completed" {
		t.Fatalf("expected completed durable execution, got %q", executions[0].Status)
	}
	if executions[0].ID.String() != executionID {
		t.Fatalf("expected execution id %q, got %q", executionID, executions[0].ID.String())
	}

	timeline, err := env.caseEvents.ListByCase(context.Background(), env.tenantID, caseItem.ID, 20, 0)
	if err != nil {
		t.Fatalf("list case timeline: %v", err)
	}
	found := false
	for _, event := range timeline {
		if strings.TrimSpace(event.EventType) != "connector_execution" {
			continue
		}
		if strings.TrimSpace(stringFromMap(event.Metadata, "execution_id")) != executionID {
			continue
		}
		found = true
		break
	}
	if !found {
		t.Fatalf("expected connector_execution timeline event for execution %s", executionID)
	}
}

func TestWorkflowVaultSecuresWorkflowDefinitionAndResolvesSecrets(t *testing.T) {
	env := newAPITestEnv(t)

	definition := map[string]any{
		"version":     1,
		"entryNodeId": "node-1",
		"nodes": []any{
			map[string]any{
				"id":    "node-1",
				"type":  "redis",
				"label": "Redis",
				"config": map[string]any{
					"operation": "get",
					"addr":      "localhost:6379",
					"key":       "incidenthub:test:key",
					"password":  "super-secret-password",
				},
			},
		},
		"edges": []any{},
	}

	c, rec := env.jsonContext(http.MethodPost, "/api/v1/catalog/workflows", map[string]any{
		"data": map[string]any{
			"name":        "Vaultized workflow",
			"description": "Workflow with secure credentials",
			"definition":  definition,
		},
	})
	setPath(c, "/api/v1/catalog/:kind", []string{"kind"}, []string{"workflows"})
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)
	err := env.handler.CreateCatalogItem(c)
	mustStatusOK(t, err, rec, http.StatusCreated)
	payload := decodeBody[map[string]any](t, rec)
	workflowID := strings.TrimSpace(stringFromMap(payload, "id"))
	if workflowID == "" {
		t.Fatalf("expected workflow id in create payload")
	}

	storedItem, err := env.catalog.GetByID(context.Background(), "workflows", uuid.MustParse(workflowID), &env.tenantID)
	if err != nil {
		t.Fatalf("load stored workflow: %v", err)
	}
	storedDefinition, ok := storedItem.Data["definition"].(map[string]any)
	if !ok {
		t.Fatalf("expected stored definition map, got %T", storedItem.Data["definition"])
	}
	nodes, _ := storedDefinition["nodes"].([]any)
	if len(nodes) != 1 {
		t.Fatalf("expected one stored workflow node, got %d", len(nodes))
	}
	nodeMap, _ := nodes[0].(map[string]any)
	configMap, _ := nodeMap["config"].(map[string]any)
	passwordValue := strings.TrimSpace(stringFromMap(configMap, "password"))
	if passwordValue == "" || !strings.HasPrefix(strings.ToLower(passwordValue), workflowvault.RefPrefix) {
		t.Fatalf("expected vault reference in stored workflow config, got %q", passwordValue)
	}
	if strings.Contains(passwordValue, "super-secret-password") {
		t.Fatalf("stored workflow definition leaked plaintext secret")
	}

	vaultItems, err := env.catalog.List(context.Background(), repository.CatalogListParams{
		Kind:     workflowVaultSecretKind,
		TenantID: &env.tenantID,
		Limit:    20,
	})
	if err != nil {
		t.Fatalf("list workflow vault secrets: %v", err)
	}
	if len(vaultItems) == 0 {
		t.Fatalf("expected stored workflow vault secret")
	}

	normalizedDefinition, err := workflow.NormalizeDefinition(storedDefinition)
	if err != nil {
		t.Fatalf("normalize stored workflow definition: %v", err)
	}
	resolvedDefinition, err := env.handler.resolveWorkflowDefinitionVaultRefs(context.Background(), env.tenantID, normalizedDefinition)
	if err != nil {
		t.Fatalf("resolve workflow vault refs: %v", err)
	}
	resolvedPassword := strings.TrimSpace(stringFromMap(resolvedDefinition.Nodes[0].Config, "password"))
	if resolvedPassword != "super-secret-password" {
		t.Fatalf("expected resolved password to match original secret, got %q", resolvedPassword)
	}

	secretID := strings.TrimSpace(stringFromMap(workflowVaultSecretItemToPayload(vaultItems[0]), "id"))
	if secretID == "" {
		t.Fatalf("expected secret id in vault payload")
	}

	listCtx, listRec := env.jsonContext(http.MethodGet, "/api/v1/workflows/vault/secrets", nil)
	setIdentity(listCtx, env.identity)
	setTenant(listCtx, env.tenantID)
	err = env.handler.ListWorkflowVaultSecrets(listCtx)
	mustStatusOK(t, err, listRec, http.StatusOK)
	listedSecrets := decodeBody[[]map[string]any](t, listRec)
	if len(listedSecrets) == 0 {
		t.Fatalf("expected workflow vault secrets in dedicated API")
	}

	deleteCtx, deleteRec := env.jsonContext(http.MethodDelete, "/api/v1/workflows/vault/secrets/"+secretID, nil)
	setPath(deleteCtx, "/api/v1/workflows/vault/secrets/:secretID", []string{"secretID"}, []string{secretID})
	setIdentity(deleteCtx, env.identity)
	setTenant(deleteCtx, env.tenantID)
	err = env.handler.DeleteWorkflowVaultSecret(deleteCtx)
	mustStatusOK(t, err, deleteRec, http.StatusNoContent)

	remainingSecrets, err := env.catalog.List(context.Background(), repository.CatalogListParams{
		Kind:     workflowVaultSecretKind,
		TenantID: &env.tenantID,
		Limit:    20,
	})
	if err != nil {
		t.Fatalf("list workflow vault secrets after delete: %v", err)
	}
	if len(remainingSecrets) != 0 {
		t.Fatalf("expected vault secrets to be deleted, got %d", len(remainingSecrets))
	}
}
