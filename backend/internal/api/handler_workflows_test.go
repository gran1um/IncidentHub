package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"incidenthub/backend/internal/repository"
)

func TestWorkflowTestRunAndHistory(t *testing.T) {
	env := newAPITestEnv(t)

	workflow, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID:  &env.tenantID,
		Kind:      "workflows",
		OwnerID:   &env.userID,
		CreatedBy: &env.userID,
		Data: map[string]any{
			"name":        "IOC Enrichment",
			"description": "Analyzer chain",
			"enabled":     true,
			"status":      "published",
			"definition": map[string]any{
				"entryNodeId": "trigger_1",
				"nodes": []map[string]any{
					{"id": "trigger_1", "type": "trigger", "label": "Trigger"},
					{"id": "analyzer_1", "type": "analyzer", "label": "Analyzer"},
				},
				"edges": []map[string]any{
					{"source": "trigger_1", "target": "analyzer_1", "label": "next"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create workflow catalog item: %v", err)
	}

	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/workflows/"+workflow.ID.String()+"/test-run", map[string]any{
			"input": map[string]any{
				"case_id":   "CASE-100",
				"triggered": "manual",
			},
		})
		setPath(c, "/api/v1/workflows/:workflowID/test-run", []string{"workflowID"}, []string{workflow.ID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.TestRunWorkflow(c)
		mustStatusOK(t, err, rec, http.StatusOK)

		payload := decodeBody[map[string]any](t, rec)
		ok, _ := payload["ok"].(bool)
		if !ok {
			t.Fatalf("expected successful workflow test run, got payload=%v", payload)
		}
		run, _ := payload["run"].(map[string]any)
		if run == nil {
			t.Fatalf("expected run payload in test run response")
		}
		if got, _ := run["status"].(string); got != "success" {
			t.Fatalf("expected success run status, got %q", got)
		}
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/workflows/"+workflow.ID.String()+"/runs?limit=10", nil)
		setPath(c, "/api/v1/workflows/:workflowID/runs", []string{"workflowID"}, []string{workflow.ID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListWorkflowRuns(c)
		mustStatusOK(t, err, rec, http.StatusOK)

		payload := decodeBody[map[string]any](t, rec)
		runs, _ := payload["runs"].([]any)
		if len(runs) != 1 {
			t.Fatalf("expected exactly one workflow run in history, got %d", len(runs))
		}
	}
}

func TestWorkflowTestRunFailureIsStoredInHistory(t *testing.T) {
	env := newAPITestEnv(t)

	workflow, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID:  &env.tenantID,
		Kind:      "workflows",
		OwnerID:   &env.userID,
		CreatedBy: &env.userID,
		Data: map[string]any{
			"name":        "Looping responder",
			"description": "Cycle test",
			"enabled":     true,
			"status":      "draft",
			"definition": map[string]any{
				"entryNodeId": "a",
				"nodes": []map[string]any{
					{"id": "a", "type": "trigger", "label": "A"},
					{"id": "b", "type": "responder", "label": "B"},
				},
				"edges": []map[string]any{
					{"source": "a", "target": "b", "label": "next"},
					{"source": "b", "target": "a", "label": "next"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create looping workflow catalog item: %v", err)
	}

	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/workflows/"+workflow.ID.String()+"/test-run", nil)
		setPath(c, "/api/v1/workflows/:workflowID/test-run", []string{"workflowID"}, []string{workflow.ID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.TestRunWorkflow(c)
		mustStatusOK(t, err, rec, http.StatusOK)

		payload := decodeBody[map[string]any](t, rec)
		ok, _ := payload["ok"].(bool)
		if ok {
			t.Fatalf("expected failed workflow test run")
		}
		run, _ := payload["run"].(map[string]any)
		if run == nil {
			t.Fatalf("expected run payload in test run response")
		}
		if got, _ := run["status"].(string); got != "failed" {
			t.Fatalf("expected failed run status, got %q", got)
		}
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/workflows/"+workflow.ID.String()+"/runs", nil)
		setPath(c, "/api/v1/workflows/:workflowID/runs", []string{"workflowID"}, []string{workflow.ID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListWorkflowRuns(c)
		mustStatusOK(t, err, rec, http.StatusOK)

		payload := decodeBody[map[string]any](t, rec)
		runs, _ := payload["runs"].([]any)
		if len(runs) != 1 {
			t.Fatalf("expected one stored run after failure, got %d", len(runs))
		}
		run, _ := runs[0].(map[string]any)
		if got, _ := run["status"].(string); got != "failed" {
			t.Fatalf("expected failed status in history, got %q", got)
		}
	}
}

func TestWorkflowRunUsesStoredDefinitionAndManualTrigger(t *testing.T) {
	env := newAPITestEnv(t)

	workflowItem, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID:  &env.tenantID,
		Kind:      "workflows",
		OwnerID:   &env.userID,
		CreatedBy: &env.userID,
		Data: map[string]any{
			"name":        "Production analyzer workflow",
			"description": "Stored definition only",
			"enabled":     true,
			"status":      "published",
			"definition": map[string]any{
				"entryNodeId": "trigger_1",
				"nodes": []map[string]any{
					{"id": "trigger_1", "type": "trigger", "label": "Start"},
					{"id": "analyzer_1", "type": "analyzer", "label": "Analyzer"},
				},
				"edges": []map[string]any{
					{"source": "trigger_1", "target": "analyzer_1", "label": "next"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create workflow catalog item: %v", err)
	}

	c, rec := env.jsonContext(http.MethodPost, "/api/v1/workflows/"+workflowItem.ID.String()+"/run", map[string]any{
		"input": map[string]any{
			"case_id": "CASE-200",
		},
		// Production run should ignore request-level definition override and use stored one.
		"definition": map[string]any{
			"entryNodeId": "broken_node",
			"nodes":       []map[string]any{{"id": "broken_node", "type": "unknown_node"}},
		},
	})
	setPath(c, "/api/v1/workflows/:workflowID/run", []string{"workflowID"}, []string{workflowItem.ID.String()})
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)

	callErr := env.handler.RunWorkflow(c)
	mustStatusOK(t, callErr, rec, http.StatusOK)

	payload := decodeBody[map[string]any](t, rec)
	ok, _ := payload["ok"].(bool)
	if !ok {
		t.Fatalf("expected successful workflow production run, got payload=%v", payload)
	}
	run, _ := payload["run"].(map[string]any)
	if run == nil {
		t.Fatalf("expected run payload in workflow run response")
	}
	if got, _ := run["status"].(string); got != "success" {
		t.Fatalf("expected success run status, got %q", got)
	}
	if got, _ := run["trigger"].(string); got != "manual" {
		t.Fatalf("expected run trigger manual, got %q", got)
	}
}

func TestWorkflowRunFailureIsStoredInHistory(t *testing.T) {
	env := newAPITestEnv(t)

	workflowItem, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID:  &env.tenantID,
		Kind:      "workflows",
		OwnerID:   &env.userID,
		CreatedBy: &env.userID,
		Data: map[string]any{
			"name":        "Looping production workflow",
			"description": "Cycle test",
			"enabled":     true,
			"status":      "published",
			"definition": map[string]any{
				"entryNodeId": "a",
				"nodes": []map[string]any{
					{"id": "a", "type": "trigger", "label": "A"},
					{"id": "b", "type": "responder", "label": "B"},
				},
				"edges": []map[string]any{
					{"source": "a", "target": "b", "label": "next"},
					{"source": "b", "target": "a", "label": "next"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create looping workflow catalog item: %v", err)
	}

	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/workflows/"+workflowItem.ID.String()+"/run", nil)
		setPath(c, "/api/v1/workflows/:workflowID/run", []string{"workflowID"}, []string{workflowItem.ID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.RunWorkflow(c)
		mustStatusOK(t, err, rec, http.StatusOK)

		payload := decodeBody[map[string]any](t, rec)
		ok, _ := payload["ok"].(bool)
		if ok {
			t.Fatalf("expected failed workflow production run")
		}
		run, _ := payload["run"].(map[string]any)
		if run == nil {
			t.Fatalf("expected run payload in workflow run response")
		}
		if got, _ := run["status"].(string); got != "failed" {
			t.Fatalf("expected failed run status, got %q", got)
		}
		if got, _ := run["trigger"].(string); got != "manual" {
			t.Fatalf("expected run trigger manual, got %q", got)
		}
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/workflows/"+workflowItem.ID.String()+"/runs", nil)
		setPath(c, "/api/v1/workflows/:workflowID/runs", []string{"workflowID"}, []string{workflowItem.ID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListWorkflowRuns(c)
		mustStatusOK(t, err, rec, http.StatusOK)

		payload := decodeBody[map[string]any](t, rec)
		runs, _ := payload["runs"].([]any)
		if len(runs) != 1 {
			t.Fatalf("expected one stored run after failure, got %d", len(runs))
		}
		run, _ := runs[0].(map[string]any)
		if got, _ := run["status"].(string); got != "failed" {
			t.Fatalf("expected failed status in history, got %q", got)
		}
		if got, _ := run["trigger"].(string); got != "manual" {
			t.Fatalf("expected run trigger manual in history, got %q", got)
		}
	}
}

func TestWorkflowPostgresQueryNodeExecutesReadOnlySQL(t *testing.T) {
	env := newAPITestEnv(t)

	workflowItem, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID:  &env.tenantID,
		Kind:      "workflows",
		OwnerID:   &env.userID,
		CreatedBy: &env.userID,
		Data: map[string]any{
			"name":        "Postgres query workflow",
			"description": "Read only SQL",
			"enabled":     true,
			"status":      "published",
			"definition": map[string]any{
				"entryNodeId": "trigger_1",
				"nodes": []map[string]any{
					{"id": "trigger_1", "type": "trigger", "label": "Start", "config": map[string]any{"event": "manual"}},
					{"id": "sql_1", "type": "postgres_query", "label": "SQL", "config": map[string]any{"sql": "SELECT 1 AS one", "limit": 10}},
				},
				"edges": []map[string]any{
					{"source": "trigger_1", "target": "sql_1", "label": "next"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create postgres workflow catalog item: %v", err)
	}

	c, rec := env.jsonContext(http.MethodPost, "/api/v1/workflows/"+workflowItem.ID.String()+"/test-run", map[string]any{
		"input": map[string]any{
			"source": "unit-test",
		},
	})
	setPath(c, "/api/v1/workflows/:workflowID/test-run", []string{"workflowID"}, []string{workflowItem.ID.String()})
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)

	callErr := env.handler.TestRunWorkflow(c)
	mustStatusOK(t, callErr, rec, http.StatusOK)

	payload := decodeBody[map[string]any](t, rec)
	ok, _ := payload["ok"].(bool)
	if !ok {
		t.Fatalf("expected postgres workflow run to succeed, got payload=%v", payload)
	}
	run, _ := payload["run"].(map[string]any)
	result, _ := run["result"].(map[string]any)
	finalOutput, _ := result["final_output"].(map[string]any)
	if rowCount, _ := finalOutput["postgres_row_count"].(float64); rowCount < 1 {
		t.Fatalf("expected postgres_row_count >= 1, got result=%v", result)
	}
}

func TestWorkflowTelegramSendNodeSendsMessage(t *testing.T) {
	env := newAPITestEnv(t)

	lastChatID := ""
	lastMessage := ""
	telegramServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/sendMessage") {
			http.Error(w, "unexpected endpoint", http.StatusNotFound)
			return
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode telegram payload: %v", err)
		}
		lastChatID = strings.TrimSpace(fmt.Sprint(payload["chat_id"]))
		lastMessage = strings.TrimSpace(fmt.Sprint(payload["text"]))
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer telegramServer.Close()

	connector, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID:  &env.tenantID,
		Kind:      "outbound_connectors",
		OwnerID:   &env.userID,
		CreatedBy: &env.userID,
		Data: map[string]any{
			"name":      "workflow-telegram",
			"direction": "outbound",
			"channel":   "telegram",
			"enabled":   true,
			"config": map[string]any{
				"botToken":   "workflow-test-token",
				"apiBaseURL": telegramServer.URL,
			},
		},
	})
	if err != nil {
		t.Fatalf("create telegram connector: %v", err)
	}

	workflowItem, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID:  &env.tenantID,
		Kind:      "workflows",
		OwnerID:   &env.userID,
		CreatedBy: &env.userID,
		Data: map[string]any{
			"name":        "Telegram send workflow",
			"description": "Send telegram message",
			"enabled":     true,
			"status":      "published",
			"definition": map[string]any{
				"entryNodeId": "trigger_1",
				"nodes": []map[string]any{
					{"id": "trigger_1", "type": "trigger", "label": "Start", "config": map[string]any{"event": "manual"}},
					{
						"id":    "tg_1",
						"type":  "telegram_send",
						"label": "Telegram",
						"config": map[string]any{
							"connectorId": connector.ID.String(),
							"recipient":   "777001",
							"message":     "Workflow ping {{input.case_id}}",
						},
					},
				},
				"edges": []map[string]any{
					{"source": "trigger_1", "target": "tg_1", "label": "next"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create telegram workflow catalog item: %v", err)
	}

	c, rec := env.jsonContext(http.MethodPost, "/api/v1/workflows/"+workflowItem.ID.String()+"/test-run", map[string]any{
		"input": map[string]any{
			"case_id": "CASE-777",
		},
	})
	setPath(c, "/api/v1/workflows/:workflowID/test-run", []string{"workflowID"}, []string{workflowItem.ID.String()})
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)

	callErr := env.handler.TestRunWorkflow(c)
	mustStatusOK(t, callErr, rec, http.StatusOK)

	payload := decodeBody[map[string]any](t, rec)
	ok, _ := payload["ok"].(bool)
	if !ok {
		t.Fatalf("expected telegram workflow run to succeed, got payload=%v", payload)
	}
	if lastChatID != "777001" {
		t.Fatalf("expected telegram chat_id=777001, got %q", lastChatID)
	}
	if !strings.Contains(lastMessage, "CASE-777") {
		t.Fatalf("expected telegram message to include case id, got %q", lastMessage)
	}
}
