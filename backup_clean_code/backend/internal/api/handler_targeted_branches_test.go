package api

import (
	"context"
	"net/http"
	"testing"

	"incidenthub/backend/internal/repository"

	"github.com/google/uuid"
)

func TestResolveCaseInTenantErrorBranches(t *testing.T) {
	env := newAPITestEnv(t)

	{
		c, _ := env.jsonContext(http.MethodGet, "/api/v1/cases/"+uuid.NewString(), nil)
		setPath(c, "/api/v1/cases/:caseID", []string{"caseID"}, []string{uuid.NewString()})
		_, _, err := env.handler.resolveCaseInTenant(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("expected missing tenant error, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodGet, "/api/v1/cases/not-uuid", nil)
		setPath(c, "/api/v1/cases/:caseID", []string{"caseID"}, []string{"not-uuid"})
		setTenant(c, env.tenantID)
		_, _, err := env.handler.resolveCaseInTenant(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("expected invalid caseID error, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodGet, "/api/v1/cases/"+uuid.NewString(), nil)
		setPath(c, "/api/v1/cases/:caseID", []string{"caseID"}, []string{uuid.NewString()})
		setTenant(c, env.tenantID)
		_, _, err := env.handler.resolveCaseInTenant(c)
		if code := httpErrorCode(t, err); code != http.StatusNotFound {
			t.Fatalf("expected case not found error, got %d", code)
		}
	}
}

func TestResolveOutboundConnectorForTenantBranches(t *testing.T) {
	env := newAPITestEnv(t)

	inboundConnector, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID:  &env.tenantID,
		Kind:      "inbound_connectors",
		CreatedBy: &env.userID,
		Data: map[string]any{
			"name":      "inbound-cfg",
			"direction": "inbound",
			"config": map[string]any{
				"source_type": "http",
				"url":         "https://example.local/feed",
			},
		},
	})
	if err != nil {
		t.Fatalf("create inbound connector: %v", err)
	}

	disabledOutbound, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID:  &env.tenantID,
		Kind:      "outbound_connectors",
		CreatedBy: &env.userID,
		Data: map[string]any{
			"name":      "disabled",
			"direction": "outbound",
			"channel":   "mock",
			"enabled":   false,
		},
	})
	if err != nil {
		t.Fatalf("create disabled outbound connector: %v", err)
	}

	if _, err := env.handler.resolveOutboundConnectorForTenant(context.Background(), env.tenantID, inboundConnector.ID); httpErrorCode(t, err) != http.StatusNotFound {
		t.Fatalf("expected inbound connector to be unavailable")
	}
	if _, err := env.handler.resolveOutboundConnectorForTenant(context.Background(), env.tenantID, disabledOutbound.ID); httpErrorCode(t, err) != http.StatusBadRequest {
		t.Fatalf("expected disabled connector rejection")
	}
	if _, err := env.handler.resolveOutboundConnectorForTenant(context.Background(), env.tenantID, uuid.New()); httpErrorCode(t, err) != http.StatusNotFound {
		t.Fatalf("expected not found connector")
	}
}

func TestCatalogUpdateDeleteExtraBranches(t *testing.T) {
	env := newAPITestEnv(t)
	viewer := env.identity
	viewer.TenantRole = "viewer"

	notification, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID:  &env.tenantID,
		Kind:      "rate_limits",
		OwnerID:   &env.secondUserID,
		CreatedBy: &env.userID,
		Data: map[string]any{
			"name": "owned-by-other",
		},
	})
	if err != nil {
		t.Fatalf("create notification: %v", err)
	}

	{
		c, _ := env.jsonContext(http.MethodPatch, "/api/v1/catalog/rate_limits/"+notification.ID.String(), map[string]any{
			"data": map[string]any{"enabled": false},
		})
		setPath(c, "/api/v1/catalog/:kind/:itemID", []string{"kind", "itemID"}, []string{"rate_limits", notification.ID.String()})
		setIdentity(c, viewer)
		setTenant(c, env.tenantID)
		updateErr := env.handler.UpdateCatalogItem(c)
		if code := httpErrorCode(t, updateErr); code != http.StatusForbidden {
			t.Fatalf("expected forbidden update, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodPatch, "/api/v1/catalog/notifications/"+uuid.NewString(), map[string]any{
			"data": map[string]any{"enabled": true},
		})
		setPath(c, "/api/v1/catalog/:kind/:itemID", []string{"kind", "itemID"}, []string{"notifications", uuid.NewString()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		updateErr := env.handler.UpdateCatalogItem(c)
		if code := httpErrorCode(t, updateErr); code != http.StatusNotFound {
			t.Fatalf("expected not found update, got %d", code)
		}
	}

	inboundConnector, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID:  &env.tenantID,
		Kind:      "inbound_connectors",
		CreatedBy: &env.userID,
		Data: map[string]any{
			"name":      "inbound-validate",
			"direction": "inbound",
			"config": map[string]any{
				"source_type": "http",
				"url":         "https://example.local/feed",
			},
		},
	})
	if err != nil {
		t.Fatalf("create inbound connector for update validation: %v", err)
	}

	{
		c, _ := env.jsonContext(http.MethodPatch, "/api/v1/catalog/inbound_connectors/"+inboundConnector.ID.String(), map[string]any{
			"data": map[string]any{
				"config": map[string]any{
					"source_type": "http",
					"url":         "",
				},
			},
		})
		setPath(
			c,
			"/api/v1/catalog/:kind/:itemID",
			[]string{"kind", "itemID"},
			[]string{"inbound_connectors", inboundConnector.ID.String()},
		)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		updateErr := env.handler.UpdateCatalogItem(c)
		if code := httpErrorCode(t, updateErr); code != http.StatusGone {
			t.Fatalf("expected inbound connector removal error, got %d", code)
		}
	}

	outboundConnector, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID:  &env.tenantID,
		Kind:      "outbound_connectors",
		CreatedBy: &env.userID,
		Data: map[string]any{
			"name":      "delete-me",
			"direction": "outbound",
			"channel":   "mock",
		},
	})
	if err != nil {
		t.Fatalf("create outbound connector: %v", err)
	}

	{
		c, _ := env.jsonContext(http.MethodDelete, "/api/v1/catalog/connectors/"+outboundConnector.ID.String(), nil)
		setPath(c, "/api/v1/catalog/:kind/:itemID", []string{"kind", "itemID"}, []string{"connectors", outboundConnector.ID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		deleteErr := env.handler.DeleteCatalogItem(c)
		if code := httpErrorCode(t, deleteErr); code != http.StatusGone {
			t.Fatalf("expected deprecated kind error, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodDelete, "/api/v1/catalog/connectors/"+uuid.NewString(), nil)
		setPath(c, "/api/v1/catalog/:kind/:itemID", []string{"kind", "itemID"}, []string{"connectors", uuid.NewString()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		deleteErr := env.handler.DeleteCatalogItem(c)
		if code := httpErrorCode(t, deleteErr); code != http.StatusGone {
			t.Fatalf("expected deprecated kind error, got %d", code)
		}
	}
}

func TestCreateForumThreadBranchCoverage(t *testing.T) {
	env := newAPITestEnv(t)

	{
		c, _ := env.jsonContext(http.MethodPost, "/api/v1/forum/threads", map[string]any{
			"title": "missing case",
		})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		handlerErr := env.handler.CreateForumThread(c)
		if code := httpErrorCode(t, handlerErr); code != http.StatusBadRequest {
			t.Fatalf("expected case_id required error, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodPost, "/api/v1/forum/threads", map[string]any{
			"title":   "invalid case",
			"case_id": "not-uuid",
		})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		handlerErr := env.handler.CreateForumThread(c)
		if code := httpErrorCode(t, handlerErr); code != http.StatusBadRequest {
			t.Fatalf("expected invalid case_id error, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodPost, "/api/v1/forum/threads", map[string]any{
			"title":   "missing case",
			"case_id": uuid.NewString(),
		})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		handlerErr := env.handler.CreateForumThread(c)
		if code := httpErrorCode(t, handlerErr); code != http.StatusNotFound {
			t.Fatalf("expected case not found error, got %d", code)
		}
	}

	caseItem, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-FORUM-1",
		Title:             "Forum case",
		Description:       "Case for forum thread branch",
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
		t.Fatalf("create forum branch case: %v", err)
	}

	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/forum/threads", map[string]any{
			"title":   "thread one",
			"case_id": caseItem.ID.String(),
		})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		handlerErr := env.handler.CreateForumThread(c)
		mustStatusOK(t, handlerErr, rec, http.StatusCreated)
	}

	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/forum/threads", map[string]any{
			"title":   "thread one duplicate",
			"case_id": caseItem.ID.String(),
		})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		handlerErr := env.handler.CreateForumThread(c)
		mustStatusOK(t, handlerErr, rec, http.StatusOK)
	}
}

func TestCaseOpsAndConnectorRunValidationBranches(t *testing.T) {
	env := newAPITestEnv(t)

	caseItem, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-BRANCH-OPS",
		Title:             "Ops branch case",
		Description:       "Ops branch checks",
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
		c, _ := env.jsonContext(http.MethodPatch, "/api/v1/tasks/not-uuid", map[string]any{"status": "done"})
		setPath(c, "/api/v1/tasks/:taskID", []string{"taskID"}, []string{"not-uuid"})
		setTenant(c, env.tenantID)
		handlerErr := env.handler.UpdateTask(c)
		if code := httpErrorCode(t, handlerErr); code != http.StatusBadRequest {
			t.Fatalf("expected invalid task id, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodPatch, "/api/v1/tasks/"+uuid.NewString(), map[string]any{"assignee_id": "bad"})
		setPath(c, "/api/v1/tasks/:taskID", []string{"taskID"}, []string{uuid.NewString()})
		setTenant(c, env.tenantID)
		handlerErr := env.handler.UpdateTask(c)
		if code := httpErrorCode(t, handlerErr); code != http.StatusBadRequest {
			t.Fatalf("expected invalid assignee id, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodPatch, "/api/v1/tasks/"+uuid.NewString(), map[string]any{"due_date": "not-a-timestamp"})
		setPath(c, "/api/v1/tasks/:taskID", []string{"taskID"}, []string{uuid.NewString()})
		setTenant(c, env.tenantID)
		handlerErr := env.handler.UpdateTask(c)
		if code := httpErrorCode(t, handlerErr); code != http.StatusBadRequest {
			t.Fatalf("expected invalid due_date, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodDelete, "/api/v1/tasks/"+uuid.NewString(), nil)
		setPath(c, "/api/v1/tasks/:taskID", []string{"taskID"}, []string{uuid.NewString()})
		setTenant(c, env.tenantID)
		handlerErr := env.handler.DeleteTask(c)
		if code := httpErrorCode(t, handlerErr); code != http.StatusNotFound {
			t.Fatalf("expected task not found, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodPost, "/api/v1/cases/"+caseItem.ID.String()+"/attachments", map[string]any{
			"file_name":       "artifact.bin",
			"storage_key":     "x",
			"file_size_bytes": -1,
		})
		setPath(c, "/api/v1/cases/:caseID/attachments", []string{"caseID"}, []string{caseItem.ID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		handlerErr := env.handler.CreateCaseAttachment(c)
		if code := httpErrorCode(t, handlerErr); code != http.StatusBadRequest {
			t.Fatalf("expected negative file size error, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodPost, "/api/v1/connectors/inbound/not-uuid/run", nil)
		setPath(c, "/api/v1/connectors/inbound/:connectorID/run", []string{"connectorID"}, []string{"not-uuid"})
		setTenant(c, env.tenantID)
		handlerErr := env.handler.RunInboundConnector(c)
		if code := httpErrorCode(t, handlerErr); code != http.StatusServiceUnavailable {
			t.Fatalf("expected inbound worker unavailable, got %d", code)
		}
	}

	alertItem, err := env.alerts.Create(context.Background(), repository.CreateAlertParams{
		TenantID:    env.tenantID,
		Title:       "branch alert",
		Description: "for update branch",
		Source:      "siem",
		Status:      "new",
		Severity:    "medium",
		TLP:         "amber",
		PAP:         "amber",
	})
	if err != nil {
		t.Fatalf("create alert: %v", err)
	}

	{
		c, _ := env.jsonContext(http.MethodPatch, "/api/v1/alerts/"+alertItem.ID.String(), map[string]any{
			"assigned_to": "bad-uuid",
		})
		setPath(c, "/api/v1/alerts/:alertID", []string{"alertID"}, []string{alertItem.ID.String()})
		setTenant(c, env.tenantID)
		setIdentity(c, env.identity)
		handlerErr := env.handler.UpdateAlert(c)
		if code := httpErrorCode(t, handlerErr); code != http.StatusBadRequest {
			t.Fatalf("expected invalid assigned_to error, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodPatch, "/api/v1/cases/"+caseItem.ID.String(), map[string]any{
			"status":      "invalid-status",
			"confidence":  150,
			"detected_at": "not-date",
		})
		setPath(c, "/api/v1/cases/:caseID", []string{"caseID"}, []string{caseItem.ID.String()})
		setTenant(c, env.tenantID)
		setIdentity(c, env.identity)
		handlerErr := env.handler.UpdateCase(c)
		if code := httpErrorCode(t, handlerErr); code != http.StatusBadRequest {
			t.Fatalf("expected invalid case update payload error, got %d", code)
		}
	}
}
