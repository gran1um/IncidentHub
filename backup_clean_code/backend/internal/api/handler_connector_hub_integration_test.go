package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"
)

func TestConnectorHubExecuteAndListIntegration(t *testing.T) {
	env := newAPITestEnv(t)
	ctx := context.Background()

	caseItem, err := env.cases.Create(ctx, repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-HUB-0001",
		Title:             "Connector hub integration case",
		Description:       "Validation for connector hub execution flow",
		Source:            "manual",
		IncidentType:      "malware",
		Status:            "new",
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

	connector, method := createConnectorHubTestConnector(t, env, "mock", map[string]any{})

	var executionID string
	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/connectors/hub/execute", map[string]any{
			"connector_id": connector.ID.String(),
			"action":       "lookup_hash",
			"case_id":      caseItem.ID.String(),
			"input": map[string]any{
				"hash": "44d88612fea8a8f36de82e1278abb02f",
			},
		})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err = env.handler.ExecuteConnectorHub(c)
		mustStatusOK(t, err, rec, http.StatusAccepted)

		payload := decodeBody[map[string]any](t, rec)
		if status := strings.ToLower(strings.TrimSpace(stringFromMap(payload, "status"))); status != "queued" {
			t.Fatalf("expected queued status, got %q", payload["status"])
		}
		if got := strings.TrimSpace(stringFromMap(payload, "connector_id")); got != connector.ID.String() {
			t.Fatalf("unexpected connector_id %q", got)
		}
		if got := strings.TrimSpace(stringFromMap(payload, "method_id")); got != method.ID.String() {
			t.Fatalf("unexpected method_id %q", got)
		}
		executionID = strings.TrimSpace(stringFromMap(payload, "id"))
		if executionID == "" {
			t.Fatalf("expected execution id in payload")
		}
	}

	if processed := env.handler.processNextConnectorHubBatch(ctx, 10); processed < 1 {
		t.Fatalf("expected connector hub worker to process at least one execution")
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/connectors/hub/executions?connector_id="+connector.ID.String()+"&case_id="+caseItem.ID.String(), nil)
		c.Request().URL.RawQuery = "connector_id=" + connector.ID.String() + "&case_id=" + caseItem.ID.String()
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err = env.handler.ListConnectorHubExecutions(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		payload := decodeBody[[]map[string]any](t, rec)
		if len(payload) == 0 {
			t.Fatalf("expected connector hub executions in list")
		}
		first := payload[0]
		if strings.TrimSpace(stringFromMap(first, "id")) != executionID {
			t.Fatalf("unexpected execution id in list: %q", first["id"])
		}
		if status := strings.ToLower(strings.TrimSpace(stringFromMap(first, "status"))); status != "completed" {
			t.Fatalf("expected completed status, got %q", first["status"])
		}
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/connectors/hub/executions/"+executionID, nil)
		setPath(c, "/api/v1/connectors/hub/executions/:executionID", []string{"executionID"}, []string{executionID})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err = env.handler.GetConnectorHubExecution(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		payload := decodeBody[map[string]any](t, rec)
		attempts := normalizeSlice(payload["attempts"])
		events := normalizeSlice(payload["events"])
		if len(attempts) == 0 {
			t.Fatalf("expected attempts in execution detail")
		}
		if len(events) == 0 {
			t.Fatalf("expected events in execution detail")
		}
	}
}

func TestConnectorHubRetryCancelAndRestartAPI(t *testing.T) {
	env := newAPITestEnv(t)
	ctx := context.Background()

	connector, _ := createConnectorHubTestConnector(t, env, "webhook", map[string]any{})

	var executionID string
	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/connectors/hub/execute", map[string]any{
			"connector_id": connector.ID.String(),
			"action":       "lookup_hash",
			"input": map[string]any{
				"hash": "44d88612fea8a8f36de82e1278abb02f",
			},
		})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ExecuteConnectorHub(c)
		mustStatusOK(t, err, rec, http.StatusAccepted)
		payload := decodeBody[map[string]any](t, rec)
		executionID = strings.TrimSpace(stringFromMap(payload, "id"))
		if executionID == "" {
			t.Fatalf("expected execution id")
		}
	}

	env.handler.processNextConnectorHubBatch(ctx, 10)

	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/connectors/hub/executions/"+executionID+"/retry", nil)
		setPath(c, "/api/v1/connectors/hub/executions/:executionID/retry", []string{"executionID"}, []string{executionID})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.RetryConnectorHubExecution(c)
		mustStatusOK(t, err, rec, http.StatusAccepted)
		payload := decodeBody[map[string]any](t, rec)
		if status := strings.ToLower(strings.TrimSpace(stringFromMap(payload, "status"))); status != "queued" {
			t.Fatalf("expected queued status after retry, got %q", payload["status"])
		}
	}

	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/connectors/hub/executions/"+executionID+"/cancel", nil)
		setPath(c, "/api/v1/connectors/hub/executions/:executionID/cancel", []string{"executionID"}, []string{executionID})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.CancelConnectorHubExecution(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		payload := decodeBody[map[string]any](t, rec)
		if status := strings.ToLower(strings.TrimSpace(stringFromMap(payload, "status"))); status != "canceled" {
			t.Fatalf("expected canceled status, got %q", payload["status"])
		}
	}

	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/connectors/hub/executions/"+executionID+"/restart", nil)
		setPath(c, "/api/v1/connectors/hub/executions/:executionID/restart", []string{"executionID"}, []string{executionID})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.RestartConnectorHubExecution(c)
		mustStatusOK(t, err, rec, http.StatusAccepted)
		payload := decodeBody[map[string]any](t, rec)
		if status := strings.ToLower(strings.TrimSpace(stringFromMap(payload, "status"))); status != "queued" {
			t.Fatalf("expected queued status for restarted execution, got %q", payload["status"])
		}
		if strings.TrimSpace(stringFromMap(payload, "id")) == executionID {
			t.Fatalf("expected restarted execution to have a new id")
		}
	}
}

func createConnectorHubTestConnector(t *testing.T, env *apiTestEnv, channel string, config map[string]any) (connectorItem *models.CatalogItem, methodItem *models.CatalogItem) {
	t.Helper()
	connector, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID: &env.tenantID,
		Kind:     "outbound_connectors",
		OwnerID:  &env.userID,
		Data: map[string]any{
			"name":      "Connector Hub Test",
			"channel":   channel,
			"direction": "outbound",
			"enabled":   true,
			"config":    config,
		},
		CreatedBy: &env.userID,
	})
	if err != nil {
		t.Fatalf("create outbound connector: %v", err)
	}
	method, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID:  &env.tenantID,
		Kind:      "connector_methods",
		RefID:     &connector.ID,
		OwnerID:   &env.userID,
		CreatedBy: &env.userID,
		Data: map[string]any{
			"name":             "Hash lookup",
			"action":           "lookup_hash",
			"message_template": "Lookup hash={{input.hash}}",
			"metadata_template": map[string]any{
				"target": "{{input.hash}}",
			},
		},
	})
	if err != nil {
		t.Fatalf("create connector method: %v", err)
	}
	return connector, method
}

func normalizeSlice(value any) []any {
	items, ok := value.([]any)
	if ok {
		return items
	}
	return []any{}
}
