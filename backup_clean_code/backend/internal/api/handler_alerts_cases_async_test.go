package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

func TestCreateAlertAsyncQueuesOperation(t *testing.T) {
	env := newAPITestEnv(t)
	queue := &asyncOpsQueueStub{enabled: true}
	env.handler.asyncOps = queue

	c, rec := env.jsonContext(http.MethodPost, "/api/v1/alerts", map[string]any{
		"title":       "Queued alert",
		"description": "async create",
		"source":      "siem",
	})
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)
	err := env.handler.CreateAlert(c)
	mustStatusOK(t, err, rec, http.StatusAccepted)

	if len(queue.operations) != 1 {
		t.Fatalf("expected one queued operation, got %d", len(queue.operations))
	}
	if queue.operations[0].Type != AsyncOperationAlertCreate {
		t.Fatalf("expected %q operation type, got %q", AsyncOperationAlertCreate, queue.operations[0].Type)
	}
	payload := decodeBody[map[string]any](t, rec)
	operationID, _ := payload["operation_id"].(string)
	if operationID == "" {
		t.Fatalf("expected operation_id in async response")
	}
	items, err := env.alerts.ListByTenant(context.Background(), env.tenantID, 10, 0)
	if err != nil {
		t.Fatalf("list alerts after async queue: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected no alerts persisted before consumer processing, got %d", len(items))
	}
}

func TestGetAsyncOperationStatus(t *testing.T) {
	env := newAPITestEnv(t)
	queue := &asyncOpsQueueStub{enabled: true}
	env.handler.asyncOps = queue

	createCtx, createRec := env.jsonContext(http.MethodPost, "/api/v1/alerts", map[string]any{
		"title":       "Queued alert for status",
		"description": "async create",
		"source":      "siem",
	})
	setIdentity(createCtx, env.identity)
	setTenant(createCtx, env.tenantID)
	err := env.handler.CreateAlert(createCtx)
	mustStatusOK(t, err, createRec, http.StatusAccepted)

	createPayload := decodeBody[map[string]any](t, createRec)
	operationID, _ := createPayload["operation_id"].(string)
	if operationID == "" {
		t.Fatalf("operation_id is required in async create response")
	}

	statusCtx, statusRec := env.jsonContext(http.MethodGet, "/api/v1/operations/"+operationID, nil)
	setPath(statusCtx, "/api/v1/operations/:operationID", []string{"operationID"}, []string{operationID})
	setIdentity(statusCtx, env.identity)
	setTenant(statusCtx, env.tenantID)
	err = env.handler.GetAsyncOperation(statusCtx)
	mustStatusOK(t, err, statusRec, http.StatusOK)

	statusPayload := decodeBody[map[string]any](t, statusRec)
	if got, _ := statusPayload["status"].(string); got != "queued" {
		t.Fatalf("expected queued operation status, got %q", got)
	}
	if got, _ := statusPayload["operation_id"].(string); got != operationID {
		t.Fatalf("unexpected operation_id in status payload: %q", got)
	}

	missingOperationID := uuid.NewString()
	notFoundCtx, _ := env.jsonContext(http.MethodGet, "/api/v1/operations/"+missingOperationID, nil)
	setPath(notFoundCtx, "/api/v1/operations/:operationID", []string{"operationID"}, []string{missingOperationID})
	setIdentity(notFoundCtx, env.identity)
	setTenant(notFoundCtx, env.tenantID)
	err = env.handler.GetAsyncOperation(notFoundCtx)
	if code := httpErrorCode(t, err); code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing operation, got %d", code)
	}
}

func TestCreateCaseAsyncQueuesOperation(t *testing.T) {
	env := newAPITestEnv(t)
	queue := &asyncOpsQueueStub{enabled: true}
	env.handler.asyncOps = queue

	c, rec := env.jsonContext(http.MethodPost, "/api/v1/cases", map[string]any{
		"title":         "Queued case",
		"description":   "async create",
		"incident_type": "investigation",
		"priority":      "high",
		"severity":      "high",
	})
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)
	err := env.handler.CreateCase(c)
	mustStatusOK(t, err, rec, http.StatusAccepted)

	if len(queue.operations) != 1 {
		t.Fatalf("expected one queued operation, got %d", len(queue.operations))
	}
	if queue.operations[0].Type != AsyncOperationCaseCreate {
		t.Fatalf("expected %q operation type, got %q", AsyncOperationCaseCreate, queue.operations[0].Type)
	}
	var payload asyncCaseCreatePayload
	if unmarshalErr := json.Unmarshal(queue.operations[0].Payload, &payload); unmarshalErr != nil {
		t.Fatalf("decode async case create payload: %v", unmarshalErr)
	}
	if strings.TrimSpace(payload.AssignedTo) != env.secondUserID.String() {
		t.Fatalf("expected auto-assigned analyst %s in payload, got %q", env.secondUserID, payload.AssignedTo)
	}
	items, err := env.cases.ListByTenant(context.Background(), env.tenantID, 10, 0)
	if err != nil {
		t.Fatalf("list cases after async queue: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected no cases persisted before consumer processing, got %d", len(items))
	}
}

func TestDeleteAlertAsyncQueuesOperation(t *testing.T) {
	env := newAPITestEnv(t)
	created, err := env.alerts.Create(context.Background(), repository.CreateAlertParams{
		TenantID:    env.tenantID,
		Title:       "Delete me",
		Description: "queued delete",
		Source:      "siem",
		Status:      "new",
		Severity:    "medium",
		TLP:         "amber",
		PAP:         "amber",
		CreatedBy:   &env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create alert seed: %v", err)
	}

	queue := &asyncOpsQueueStub{enabled: true}
	env.handler.asyncOps = queue

	c, rec := env.jsonContext(http.MethodDelete, "/api/v1/alerts/"+created.ID.String(), nil)
	setPath(c, "/api/v1/alerts/:alertID", []string{"alertID"}, []string{created.ID.String()})
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)
	err = env.handler.DeleteAlert(c)
	mustStatusOK(t, err, rec, http.StatusAccepted)

	if len(queue.operations) != 1 {
		t.Fatalf("expected one queued operation, got %d", len(queue.operations))
	}
	if queue.operations[0].Type != AsyncOperationAlertDelete {
		t.Fatalf("expected %q operation type, got %q", AsyncOperationAlertDelete, queue.operations[0].Type)
	}
	if _, err := env.alerts.GetByID(context.Background(), env.tenantID, created.ID); err != nil {
		t.Fatalf("alert should remain until async consumer runs: %v", err)
	}
}

func TestDeleteCaseAsyncQueuesOperation(t *testing.T) {
	env := newAPITestEnv(t)
	created, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-ASYNC-1",
		Title:             "Delete case async",
		Description:       "queued delete",
		Source:            "manual",
		IncidentType:      "investigation",
		Status:            "open",
		Priority:          "medium",
		Impact:            "medium",
		Confidence:        50,
		Severity:          "medium",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create case seed: %v", err)
	}

	queue := &asyncOpsQueueStub{enabled: true}
	env.handler.asyncOps = queue

	c, rec := env.jsonContext(http.MethodDelete, "/api/v1/cases/"+created.ID.String(), nil)
	setPath(c, "/api/v1/cases/:caseID", []string{"caseID"}, []string{created.ID.String()})
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)
	err = env.handler.DeleteCase(c)
	mustStatusOK(t, err, rec, http.StatusAccepted)

	if len(queue.operations) != 1 {
		t.Fatalf("expected one queued operation, got %d", len(queue.operations))
	}
	if queue.operations[0].Type != AsyncOperationCaseDelete {
		t.Fatalf("expected %q operation type, got %q", AsyncOperationCaseDelete, queue.operations[0].Type)
	}
	if _, err := env.cases.GetByID(context.Background(), env.tenantID, created.ID); err != nil {
		t.Fatalf("case should remain until async consumer runs: %v", err)
	}
}

func TestUpdateAlertAsyncQueuesOperation(t *testing.T) {
	env := newAPITestEnv(t)
	created, err := env.alerts.Create(context.Background(), repository.CreateAlertParams{
		TenantID:    env.tenantID,
		Title:       "Update me",
		Description: "queued update",
		Source:      "siem",
		Status:      "new",
		Severity:    "medium",
		TLP:         "amber",
		PAP:         "amber",
		CreatedBy:   &env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create alert seed: %v", err)
	}

	queue := &asyncOpsQueueStub{enabled: true}
	env.handler.asyncOps = queue

	c, rec := env.jsonContext(http.MethodPatch, "/api/v1/alerts/"+created.ID.String(), map[string]any{
		"status":   "triaged",
		"severity": "high",
	})
	setPath(c, "/api/v1/alerts/:alertID", []string{"alertID"}, []string{created.ID.String()})
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)
	err = env.handler.UpdateAlert(c)
	mustStatusOK(t, err, rec, http.StatusAccepted)

	if len(queue.operations) != 1 {
		t.Fatalf("expected one queued operation, got %d", len(queue.operations))
	}
	if queue.operations[0].Type != AsyncOperationAlertUpdate {
		t.Fatalf("expected %q operation type, got %q", AsyncOperationAlertUpdate, queue.operations[0].Type)
	}
	reloaded, err := env.alerts.GetByID(context.Background(), env.tenantID, created.ID)
	if err != nil {
		t.Fatalf("load alert after queued update: %v", err)
	}
	if reloaded.Status != "new" {
		t.Fatalf("expected alert status to remain unchanged before worker processing, got %q", reloaded.Status)
	}
}

func TestUpdateCaseAsyncQueuesOperation(t *testing.T) {
	env := newAPITestEnv(t)
	created, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-UPDATE-ASYNC",
		Title:             "Update case async",
		Description:       "queued update",
		Source:            "manual",
		IncidentType:      "investigation",
		Status:            "open",
		Priority:          "medium",
		Impact:            "medium",
		Confidence:        50,
		Severity:          "medium",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create case seed: %v", err)
	}

	queue := &asyncOpsQueueStub{enabled: true}
	env.handler.asyncOps = queue

	c, rec := env.jsonContext(http.MethodPatch, "/api/v1/cases/"+created.ID.String(), map[string]any{
		"status":   "resolved",
		"priority": "high",
	})
	setPath(c, "/api/v1/cases/:caseID", []string{"caseID"}, []string{created.ID.String()})
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)
	err = env.handler.UpdateCase(c)
	mustStatusOK(t, err, rec, http.StatusAccepted)

	if len(queue.operations) != 1 {
		t.Fatalf("expected one queued operation, got %d", len(queue.operations))
	}
	if queue.operations[0].Type != AsyncOperationCaseUpdate {
		t.Fatalf("expected %q operation type, got %q", AsyncOperationCaseUpdate, queue.operations[0].Type)
	}
	reloaded, err := env.cases.GetByID(context.Background(), env.tenantID, created.ID)
	if err != nil {
		t.Fatalf("load case after queued update: %v", err)
	}
	if reloaded.Status != "open" {
		t.Fatalf("expected case status to remain unchanged before worker processing, got %q", reloaded.Status)
	}
}

func TestUpdateCaseRejectsStaleExpectedUpdatedAt(t *testing.T) {
	env := newAPITestEnv(t)
	created, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-UPDATE-STALE",
		Title:             "Stale update",
		Description:       "must be rejected",
		Source:            "manual",
		IncidentType:      "investigation",
		Status:            "open",
		Priority:          "medium",
		Impact:            "medium",
		Confidence:        50,
		Severity:          "medium",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create case seed: %v", err)
	}

	c, _ := env.jsonContext(http.MethodPatch, "/api/v1/cases/"+created.ID.String(), map[string]any{
		"title":               "Should fail",
		"expected_updated_at": "2020-01-01T00:00:00Z",
	})
	setPath(c, "/api/v1/cases/:caseID", []string{"caseID"}, []string{created.ID.String()})
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)
	err = env.handler.UpdateCase(c)
	if code := httpErrorCode(t, err); code != http.StatusConflict {
		t.Fatalf("expected conflict for stale case update version, got %d", code)
	}
}

func TestCopyCaseSyncCreatesDuplicate(t *testing.T) {
	env := newAPITestEnv(t)
	sourceCase, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-COPY-SOURCE",
		Title:             "Source case title",
		Description:       "source description",
		Source:            "manual",
		IncidentType:      "investigation",
		Status:            "open",
		Priority:          "high",
		Impact:            "medium",
		Confidence:        72,
		Severity:          "high",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "initial summary",
		CreatedBy:         env.identity.UserID,
		AssignedTo:        &env.secondUserID,
	})
	if err != nil {
		t.Fatalf("create case seed: %v", err)
	}

	c, rec := env.jsonContext(http.MethodPost, "/api/v1/cases/"+sourceCase.ID.String()+"/copy", nil)
	setPath(c, "/api/v1/cases/:caseID/copy", []string{"caseID"}, []string{sourceCase.ID.String()})
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)
	err = env.handler.CopyCase(c)
	mustStatusOK(t, err, rec, http.StatusCreated)

	createdCopy := decodeBody[models.Case](t, rec)
	if createdCopy.ID == sourceCase.ID {
		t.Fatalf("expected copied case id to differ from source")
	}
	if createdCopy.CaseNumber == sourceCase.CaseNumber {
		t.Fatalf("expected copied case number to be regenerated")
	}
	if createdCopy.Title != "Copy of "+sourceCase.Title {
		t.Fatalf("unexpected copied case title: %q", createdCopy.Title)
	}
	if createdCopy.AssignedTo == nil || *createdCopy.AssignedTo != env.secondUserID {
		t.Fatalf("expected assigned_to to be preserved on case copy")
	}
}

func TestCopyCaseAsyncQueuesOperation(t *testing.T) {
	env := newAPITestEnv(t)
	sourceCase, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-COPY-ASYNC-SOURCE",
		Title:             "Async source title",
		Description:       "source description",
		Source:            "manual",
		IncidentType:      "investigation",
		Status:            "open",
		Priority:          "high",
		Impact:            "medium",
		Confidence:        67,
		Severity:          "high",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "summary",
		CreatedBy:         env.identity.UserID,
		AssignedTo:        &env.secondUserID,
	})
	if err != nil {
		t.Fatalf("create case seed: %v", err)
	}

	queue := &asyncOpsQueueStub{enabled: true}
	env.handler.asyncOps = queue

	c, rec := env.jsonContext(http.MethodPost, "/api/v1/cases/"+sourceCase.ID.String()+"/copy", nil)
	setPath(c, "/api/v1/cases/:caseID/copy", []string{"caseID"}, []string{sourceCase.ID.String()})
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)
	err = env.handler.CopyCase(c)
	mustStatusOK(t, err, rec, http.StatusAccepted)

	if len(queue.operations) != 1 {
		t.Fatalf("expected one queued operation, got %d", len(queue.operations))
	}
	if queue.operations[0].Type != AsyncOperationCaseCreate {
		t.Fatalf("expected %q operation type, got %q", AsyncOperationCaseCreate, queue.operations[0].Type)
	}

	var payload asyncCaseCreatePayload
	if unmarshalErr := json.Unmarshal(queue.operations[0].Payload, &payload); unmarshalErr != nil {
		t.Fatalf("decode async case copy payload: %v", err)
	}
	if payload.CaseID == sourceCase.ID.String() {
		t.Fatalf("expected copied async case id to differ from source")
	}
	if payload.CaseNumber == sourceCase.CaseNumber {
		t.Fatalf("expected copied async case number to be regenerated")
	}
	if payload.Title != "Copy of "+sourceCase.Title {
		t.Fatalf("unexpected copied async case title: %q", payload.Title)
	}
	if strings.TrimSpace(payload.AssignedTo) != env.secondUserID.String() {
		t.Fatalf("expected copied assignee %s in payload, got %q", env.secondUserID, payload.AssignedTo)
	}
}

func TestUpdateCaseAsyncRejectsAnalystFinalClose(t *testing.T) {
	env := newAPITestEnv(t)
	created, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-UPDATE-ASYNC-REVIEW",
		Title:             "Update case async review",
		Description:       "queued update",
		Source:            "manual",
		IncidentType:      "investigation",
		Status:            "review",
		Priority:          "medium",
		Impact:            "medium",
		Confidence:        50,
		Severity:          "medium",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.identity.UserID,
		AssignedTo:        &env.secondUserID,
	})
	if err != nil {
		t.Fatalf("create case seed: %v", err)
	}

	queue := &asyncOpsQueueStub{enabled: true}
	env.handler.asyncOps = queue

	analystIdentity := env.identity
	analystIdentity.TenantRole = models.TenantRoleAnalyst
	c, rec := env.jsonContext(http.MethodPatch, "/api/v1/cases/"+created.ID.String(), map[string]any{
		"status": "closed",
	})
	setPath(c, "/api/v1/cases/:caseID", []string{"caseID"}, []string{created.ID.String()})
	setIdentity(c, analystIdentity)
	setTenant(c, env.tenantID)
	err = env.handler.UpdateCase(c)
	httpErr, ok := err.(*echo.HTTPError)
	if !ok || httpErr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for analyst final close in async mode, got err=%v code=%d body=%s", err, rec.Code, rec.Body.String())
	}
	if len(queue.operations) != 0 {
		t.Fatalf("expected no queued operations on forbidden close, got %d", len(queue.operations))
	}
}

func TestDeleteAlertAndCaseSync(t *testing.T) {
	env := newAPITestEnv(t)

	alert, err := env.alerts.Create(context.Background(), repository.CreateAlertParams{
		TenantID:    env.tenantID,
		Title:       "Sync alert delete",
		Description: "delete now",
		Source:      "siem",
		Status:      "new",
		Severity:    "medium",
		TLP:         "amber",
		PAP:         "amber",
		CreatedBy:   &env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create alert seed: %v", err)
	}
	caseItem, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-SYNC-DELETE",
		Title:             "Sync case delete",
		Description:       "delete now",
		Source:            "manual",
		IncidentType:      "investigation",
		Status:            "open",
		Priority:          "low",
		Impact:            "low",
		Confidence:        10,
		Severity:          "low",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.identity.UserID,
		DetectedAt:        stringPtr(time.Now().UTC().Format(time.RFC3339)),
	})
	if err != nil {
		t.Fatalf("create case seed: %v", err)
	}

	{
		c, rec := env.jsonContext(http.MethodDelete, "/api/v1/alerts/"+alert.ID.String(), nil)
		setPath(c, "/api/v1/alerts/:alertID", []string{"alertID"}, []string{alert.ID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.DeleteAlert(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}
	if _, err := env.alerts.GetByID(context.Background(), env.tenantID, alert.ID); err == nil {
		t.Fatal("expected alert to be deleted")
	}

	{
		c, rec := env.jsonContext(http.MethodDelete, "/api/v1/cases/"+caseItem.ID.String(), nil)
		setPath(c, "/api/v1/cases/:caseID", []string{"caseID"}, []string{caseItem.ID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.DeleteCase(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}
	if _, getErr := env.cases.GetByID(context.Background(), env.tenantID, caseItem.ID); getErr == nil {
		t.Fatal("expected case to be deleted")
	}
	if len(env.searchStub.deletes) < 2 {
		t.Fatalf("expected search delete calls for alert and case, got %d", len(env.searchStub.deletes))
	}
	hasAlertDelete := false
	hasCaseDelete := false
	for _, call := range env.searchStub.deletes {
		if call.Kind == "alerts" && call.ID == alert.ID.String() {
			hasAlertDelete = true
		}
		if call.Kind == "cases" && call.ID == caseItem.ID.String() {
			hasCaseDelete = true
		}
	}
	if !hasAlertDelete {
		t.Fatal("expected alerts index delete call")
	}
	if !hasCaseDelete {
		t.Fatal("expected cases index delete call")
	}
}

func stringPtr(value string) *string {
	return &value
}

func TestDeleteCaseNotFoundSync(t *testing.T) {
	env := newAPITestEnv(t)
	missingID := uuid.New()

	c, _ := env.jsonContext(http.MethodDelete, "/api/v1/cases/"+missingID.String(), nil)
	setPath(c, "/api/v1/cases/:caseID", []string{"caseID"}, []string{missingID.String()})
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)
	err := env.handler.DeleteCase(c)
	if code := httpErrorCode(t, err); code != http.StatusNotFound {
		t.Fatalf("expected not found for missing case delete, got %d", code)
	}
}

func TestDeleteCaseSyncMaintainsSearchConsistency(t *testing.T) {
	env := newAPITestEnv(t)

	caseItem, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-SYNC-CASCADE",
		Title:             "Sync case delete with linked entities",
		Description:       "delete case and keep search index consistent",
		Source:            "manual",
		IncidentType:      "investigation",
		Status:            "open",
		Priority:          "medium",
		Impact:            "medium",
		Confidence:        50,
		Severity:          "medium",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create case seed: %v", err)
	}

	alert, err := env.alerts.Create(context.Background(), repository.CreateAlertParams{
		TenantID:    env.tenantID,
		CaseID:      &caseItem.ID,
		Title:       "Linked alert",
		Description: "linked to deleting case",
		Source:      "siem",
		Status:      "new",
		Severity:    "high",
		TLP:         "amber",
		PAP:         "amber",
		CreatedBy:   &env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create linked alert seed: %v", err)
	}

	observable, err := env.observables.Create(context.Background(), repository.CreateObservableParams{
		TenantID:  env.tenantID,
		CaseID:    caseItem.ID,
		Type:      "ip",
		Value:     "203.0.113.10",
		Verdict:   "unknown",
		Source:    "manual",
		Tags:      []string{"seed"},
		CreatedBy: env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create linked observable seed: %v", err)
	}

	env.searchStub.indexes = nil
	env.searchStub.deletes = nil

	c, rec := env.jsonContext(http.MethodDelete, "/api/v1/cases/"+caseItem.ID.String(), nil)
	setPath(c, "/api/v1/cases/:caseID", []string{"caseID"}, []string{caseItem.ID.String()})
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)
	err = env.handler.DeleteCase(c)
	mustStatusOK(t, err, rec, http.StatusOK)

	if _, getErr := env.cases.GetByID(context.Background(), env.tenantID, caseItem.ID); getErr == nil {
		t.Fatal("expected case to be deleted")
	}
	if observables, listErr := env.observables.ListByCase(context.Background(), env.tenantID, caseItem.ID, 10, 0); listErr != nil {
		t.Fatalf("list observables after case delete: %v", listErr)
	} else if len(observables) != 0 {
		t.Fatalf("expected linked observables to be removed by cascade, got %d", len(observables))
	}

	reloadedAlert, err := env.alerts.GetByID(context.Background(), env.tenantID, alert.ID)
	if err != nil {
		t.Fatalf("load linked alert after case delete: %v", err)
	}
	if reloadedAlert.CaseID != nil {
		t.Fatalf("expected linked alert case_id to be null after case delete, got %s", reloadedAlert.CaseID.String())
	}

	hasCaseDelete := false
	hasObservableDelete := false
	for _, call := range env.searchStub.deletes {
		if call.Kind == "cases" && call.ID == caseItem.ID.String() {
			hasCaseDelete = true
		}
		if call.Kind == "observables" && call.ID == observable.ID.String() {
			hasObservableDelete = true
		}
	}
	if !hasCaseDelete {
		t.Fatalf("expected search delete for deleted case %s", caseItem.ID.String())
	}
	if !hasObservableDelete {
		t.Fatalf("expected search delete for linked observable %s", observable.ID.String())
	}

	alertReindexed := false
	for _, call := range env.searchStub.indexes {
		if call.Kind != "alerts" || call.ID != alert.ID.String() {
			continue
		}
		alertReindexed = true
		switch item := call.Doc.(type) {
		case models.Alert:
			if item.CaseID != nil {
				t.Fatalf("expected reindexed alert case_id to be nil, got %s", item.CaseID.String())
			}
		case *models.Alert:
			if item != nil && item.CaseID != nil {
				t.Fatalf("expected reindexed alert case_id to be nil, got %s", item.CaseID.String())
			}
		default:
			t.Fatalf("unexpected alert reindex payload type %T", call.Doc)
		}
	}
	if !alertReindexed {
		t.Fatalf("expected linked alert %s to be reindexed after case delete", alert.ID.String())
	}
}
