package api

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
	"time"

	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"
	"incidenthub/backend/internal/security"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

func hasSearchIndexCall(stub *apiSearchStub, kind, id string) bool {
	for _, call := range stub.indexes {
		if call.Kind == kind && call.ID == id {
			return true
		}
	}
	return false
}

func hasSearchDeleteCall(stub *apiSearchStub, kind, id string) bool {
	for _, call := range stub.deletes {
		if call.Kind == kind && call.ID == id {
			return true
		}
	}
	return false
}

func TestHandlerAlertsCasesStatusesAndOpsIntegration(t *testing.T) {
	env := newAPITestEnv(t)

	var firstAlert models.Alert
	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/alerts", map[string]any{
			"title":       "Suspicious Login",
			"description": "Unexpected login from new ASN",
			"source":      "siem",
		})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.CreateAlert(c)
		mustStatusOK(t, err, rec, http.StatusCreated)
		firstAlert = decodeBody[models.Alert](t, rec)
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/alerts", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListAlerts(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/alerts/"+firstAlert.ID.String(), nil)
		setPath(c, "/api/v1/alerts/:alertID", []string{"alertID"}, []string{firstAlert.ID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.GetAlert(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	{
		c, rec := env.jsonContext(http.MethodPatch, "/api/v1/alerts/"+firstAlert.ID.String(), map[string]any{
			"status":   "triaged",
			"severity": "high",
		})
		setPath(c, "/api/v1/alerts/:alertID", []string{"alertID"}, []string{firstAlert.ID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.UpdateAlert(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}
	if !hasSearchIndexCall(env.searchStub, "alerts", firstAlert.ID.String()) {
		t.Fatalf("expected alert update to reindex alert %s", firstAlert.ID.String())
	}

	var mainCase models.Case
	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/cases", map[string]any{
			"title":         "Credential compromise investigation",
			"description":   "SOC initiated case",
			"severity":      "high",
			"priority":      "high",
			"incident_type": "account_compromise",
		})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.CreateCase(c)
		mustStatusOK(t, err, rec, http.StatusCreated)
		mainCase = decodeBody[models.Case](t, rec)
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/cases", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListCases(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/cases/"+mainCase.ID.String(), nil)
		setPath(c, "/api/v1/cases/:caseID", []string{"caseID"}, []string{mainCase.ID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.GetCase(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	{
		c, rec := env.jsonContext(http.MethodPatch, "/api/v1/cases/"+mainCase.ID.String(), map[string]any{
			"assigned_to": env.secondUserID.String(),
		})
		setPath(c, "/api/v1/cases/:caseID", []string{"caseID"}, []string{mainCase.ID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.UpdateCase(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		assignedCase := decodeBody[models.Case](t, rec)
		if assignedCase.AssignedTo == nil || *assignedCase.AssignedTo != env.secondUserID {
			t.Fatalf("expected case assignee %s, got %#v", env.secondUserID, assignedCase.AssignedTo)
		}
	}

	{
		c, rec := env.jsonContext(http.MethodPatch, "/api/v1/cases/"+mainCase.ID.String(), map[string]any{
			"assigned_to": "",
		})
		setPath(c, "/api/v1/cases/:caseID", []string{"caseID"}, []string{mainCase.ID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.UpdateCase(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		clearedCase := decodeBody[models.Case](t, rec)
		if clearedCase.AssignedTo != nil {
			t.Fatalf("expected case assignee cleared, got %#v", clearedCase.AssignedTo)
		}
	}

	{
		c, rec := env.jsonContext(http.MethodPatch, "/api/v1/cases/"+mainCase.ID.String(), map[string]any{
			"status":             "open",
			"priority":           "medium",
			"resolution_summary": "investigation in progress",
		})
		setPath(c, "/api/v1/cases/:caseID", []string{"caseID"}, []string{mainCase.ID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.UpdateCase(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}
	if !hasSearchIndexCall(env.searchStub, "cases", mainCase.ID.String()) {
		t.Fatalf("expected case update to reindex case %s", mainCase.ID.String())
	}

	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/alerts/bulk/link-case", map[string]any{
			"case_id":   mainCase.ID.String(),
			"alert_ids": []string{firstAlert.ID.String()},
		})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.BindAlertsToCase(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	var secondAlert models.Alert
	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/alerts", map[string]any{
			"title":       "Malicious Binary",
			"description": "Hash matched malware feed",
			"source":      "edr",
			"severity":    "critical",
		})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.CreateAlert(c)
		mustStatusOK(t, err, rec, http.StatusCreated)
		secondAlert = decodeBody[models.Alert](t, rec)
	}

	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/alerts/bulk/create-case", map[string]any{
			"alert_ids": []string{secondAlert.ID.String()},
			"case": map[string]any{
				"title":         "Case from selected alerts",
				"description":   "Bulk-generated case",
				"incident_type": "malware",
				"priority":      "high",
			},
		})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.CreateCaseFromAlerts(c)
		mustStatusOK(t, err, rec, http.StatusCreated)
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/case-statuses", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListCaseStatuses(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	{
		c, rec := env.jsonContext(http.MethodPut, "/api/v1/case-statuses", map[string]any{
			"statuses": []map[string]any{
				{"code": "incident", "label": "Incident", "order": 10, "is_closed": false, "color": "#ffcc00"},
				{"code": "resolved", "label": "Resolved", "order": 20, "is_closed": true, "color": "#44aa44"},
			},
		})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.UpsertCaseStatuses(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	var task models.Task
	{
		dueAt := time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339)
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/tasks", map[string]any{
			"case_id":     mainCase.ID.String(),
			"title":       "Collect forensic artifacts",
			"description": "Memory + disk triage",
			"status":      "new",
			"assignee_id": env.secondUserID.String(),
			"due_date":    dueAt,
		})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.CreateTask(c)
		mustStatusOK(t, err, rec, http.StatusCreated)
		task = decodeBody[models.Task](t, rec)
		if task.DueDate == nil || task.DueDate.IsZero() {
			t.Fatalf("expected task due date to be persisted")
		}
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/tasks", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListTasks(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/tasks?case_id="+mainCase.ID.String(), nil)
		c.Request().URL.RawQuery = "case_id=" + mainCase.ID.String()
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListTasks(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	{
		c, rec := env.jsonContext(http.MethodPatch, "/api/v1/tasks/"+task.ID.String(), map[string]any{
			"status":   "done",
			"due_date": "",
		})
		setPath(c, "/api/v1/tasks/:taskID", []string{"taskID"}, []string{task.ID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.UpdateTask(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		updated := decodeBody[models.Task](t, rec)
		if updated.DueDate != nil {
			t.Fatalf("expected task due date to be cleared, got %v", updated.DueDate)
		}
	}

	{
		c, rec := env.jsonContext(http.MethodDelete, "/api/v1/tasks/"+task.ID.String(), nil)
		setPath(c, "/api/v1/tasks/:taskID", []string{"taskID"}, []string{task.ID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.DeleteTask(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	var observableID uuid.UUID
	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/cases/"+mainCase.ID.String()+"/observables", map[string]any{
			"type":    "domain",
			"value":   "evil.example.com",
			"verdict": "suspicious",
			"source":  "analyst",
			"tags":    []string{"ioc", "phishing"},
		})
		setPath(c, "/api/v1/cases/:caseID/observables", []string{"caseID"}, []string{mainCase.ID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.CreateCaseObservable(c)
		mustStatusOK(t, err, rec, http.StatusCreated)
		created := decodeBody[models.Observable](t, rec)
		observableID = created.ID
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/cases/"+mainCase.ID.String()+"/observables", nil)
		setPath(c, "/api/v1/cases/:caseID/observables", []string{"caseID"}, []string{mainCase.ID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListCaseObservables(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	{
		c, rec := env.jsonContext(http.MethodPatch, "/api/v1/cases/"+mainCase.ID.String()+"/observables/"+observableID.String(), map[string]any{
			"verdict": "malicious",
		})
		setPath(
			c,
			"/api/v1/cases/:caseID/observables/:observableID",
			[]string{"caseID", "observableID"},
			[]string{mainCase.ID.String(), observableID.String()},
		)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.UpdateCaseObservable(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}
	if !hasSearchIndexCall(env.searchStub, "observables", observableID.String()) {
		t.Fatalf("expected observable update to reindex observable %s", observableID.String())
	}

	{
		c, rec := env.jsonContext(http.MethodDelete, "/api/v1/cases/"+mainCase.ID.String()+"/observables/"+observableID.String(), nil)
		setPath(
			c,
			"/api/v1/cases/:caseID/observables/:observableID",
			[]string{"caseID", "observableID"},
			[]string{mainCase.ID.String(), observableID.String()},
		)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.DeleteCaseObservable(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}
	if !hasSearchDeleteCall(env.searchStub, "observables", observableID.String()) {
		t.Fatalf("expected observable delete to remove observable %s from index", observableID.String())
	}

	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/cases/"+mainCase.ID.String()+"/events", map[string]any{
			"event_type": "note",
			"title":      "Initial triage",
			"body":       "Suspicious indicators confirmed",
			"metadata":   map[string]any{"stage": "triage"},
		})
		setPath(c, "/api/v1/cases/:caseID/events", []string{"caseID"}, []string{mainCase.ID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.CreateCaseEvent(c)
		mustStatusOK(t, err, rec, http.StatusCreated)
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/cases/"+mainCase.ID.String()+"/events", nil)
		setPath(c, "/api/v1/cases/:caseID/events", []string{"caseID"}, []string{mainCase.ID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListCaseEvents(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/cases/"+mainCase.ID.String()+"/pages", map[string]any{
			"title": "Containment Plan",
			"body":  "Block IOC and isolate impacted host.",
		})
		setPath(c, "/api/v1/cases/:caseID/pages", []string{"caseID"}, []string{mainCase.ID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.CreateCasePage(c)
		mustStatusOK(t, err, rec, http.StatusCreated)
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/cases/"+mainCase.ID.String()+"/pages", nil)
		setPath(c, "/api/v1/cases/:caseID/pages", []string{"caseID"}, []string{mainCase.ID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListCasePages(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	var attachment models.Attachment
	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/cases/"+mainCase.ID.String()+"/attachments", map[string]any{
			"file_name":       "evidence.txt",
			"content_type":    "text/plain",
			"file_size_bytes": 14,
			"storage_key":     "tenant/x/case/y/evidence.txt",
			"checksum_sha256": strings.Repeat("a", 64),
		})
		setPath(c, "/api/v1/cases/:caseID/attachments", []string{"caseID"}, []string{mainCase.ID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.CreateCaseAttachment(c)
		mustStatusOK(t, err, rec, http.StatusCreated)
		attachment = decodeBody[models.Attachment](t, rec)
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/cases/"+mainCase.ID.String()+"/attachments", nil)
		setPath(c, "/api/v1/cases/:caseID/attachments", []string{"caseID"}, []string{mainCase.ID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListCaseAttachments(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/cases/"+mainCase.ID.String()+"/attachments/"+attachment.ID.String()+"/download", nil)
		setPath(
			c,
			"/api/v1/cases/:caseID/attachments/:attachmentID/download",
			[]string{"caseID", "attachmentID"},
			[]string{mainCase.ID.String(), attachment.ID.String()},
		)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.GetCaseAttachmentDownloadURL(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	{
		payload := bytes.NewBuffer(nil)
		writer := multipart.NewWriter(payload)
		part, err := writer.CreateFormFile("file", "artifact.bin")
		if err != nil {
			t.Fatalf("create multipart part: %v", err)
		}
		if _, writeErr := part.Write([]byte("binary evidence payload")); writeErr != nil {
			t.Fatalf("write multipart content: %v", writeErr)
		}
		if closeErr := writer.Close(); closeErr != nil {
			t.Fatalf("close multipart writer: %v", closeErr)
		}

		c, rec := env.multipartContext("/api/v1/cases/"+mainCase.ID.String()+"/attachments/upload", payload, writer.FormDataContentType())
		setPath(c, "/api/v1/cases/:caseID/attachments/upload", []string{"caseID"}, []string{mainCase.ID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err = env.handler.UploadCaseAttachment(c)
		mustStatusOK(t, err, rec, http.StatusCreated)
	}
}

func TestCaseReviewAndFinalCloseTransitions(t *testing.T) {
	env := newAPITestEnv(t)

	created, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-REVIEW-FLOW",
		Title:             "Review flow",
		Description:       "validate close transitions",
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

	analystIdentity := env.identity
	analystIdentity.TenantRole = models.TenantRoleAnalyst

	{
		c, rec := env.jsonContext(http.MethodPatch, "/api/v1/cases/"+created.ID.String(), map[string]any{
			"status": "review",
		})
		setPath(c, "/api/v1/cases/:caseID", []string{"caseID"}, []string{created.ID.String()})
		setIdentity(c, analystIdentity)
		setTenant(c, env.tenantID)
		err := env.handler.UpdateCase(c)
		httpErr, ok := err.(*echo.HTTPError)
		if !ok || httpErr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for missing review assignee, got err=%v code=%d body=%s", err, rec.Code, rec.Body.String())
		}
	}

	{
		c, rec := env.jsonContext(http.MethodPatch, "/api/v1/cases/"+created.ID.String(), map[string]any{
			"status":      "review",
			"assigned_to": env.identity.UserID.String(),
		})
		setPath(c, "/api/v1/cases/:caseID", []string{"caseID"}, []string{created.ID.String()})
		setIdentity(c, analystIdentity)
		setTenant(c, env.tenantID)
		err := env.handler.UpdateCase(c)
		httpErr, ok := err.(*echo.HTTPError)
		if !ok || httpErr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 when analyst self-assigns review, got err=%v code=%d body=%s", err, rec.Code, rec.Body.String())
		}
	}

	{
		c, rec := env.jsonContext(http.MethodPatch, "/api/v1/cases/"+created.ID.String(), map[string]any{
			"status":      "review",
			"assigned_to": env.secondUserID.String(),
		})
		setPath(c, "/api/v1/cases/:caseID", []string{"caseID"}, []string{created.ID.String()})
		setIdentity(c, analystIdentity)
		setTenant(c, env.tenantID)
		err := env.handler.UpdateCase(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		updated := decodeBody[models.Case](t, rec)
		if !strings.EqualFold(updated.Status, "review") {
			t.Fatalf("expected review status, got %q", updated.Status)
		}
		if updated.AssignedTo == nil || *updated.AssignedTo != env.secondUserID {
			t.Fatalf("expected review assignee %s, got %#v", env.secondUserID, updated.AssignedTo)
		}
	}

	{
		c, rec := env.jsonContext(http.MethodPatch, "/api/v1/cases/"+created.ID.String(), map[string]any{
			"status": "closed",
		})
		setPath(c, "/api/v1/cases/:caseID", []string{"caseID"}, []string{created.ID.String()})
		setIdentity(c, analystIdentity)
		setTenant(c, env.tenantID)
		err := env.handler.UpdateCase(c)
		httpErr, ok := err.(*echo.HTTPError)
		if !ok || httpErr.Code != http.StatusForbidden {
			t.Fatalf("expected 403 when analyst tries final close, got err=%v code=%d body=%s", err, rec.Code, rec.Body.String())
		}
	}

	{
		c, rec := env.jsonContext(http.MethodPatch, "/api/v1/cases/"+created.ID.String(), map[string]any{
			"status": "closed",
		})
		setPath(c, "/api/v1/cases/:caseID", []string{"caseID"}, []string{created.ID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.UpdateCase(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		updated := decodeBody[models.Case](t, rec)
		if !strings.EqualFold(updated.Status, "closed") {
			t.Fatalf("expected closed status, got %q", updated.Status)
		}
	}
}

func TestCaseFinalCloseRequiresConfiguredApprovals(t *testing.T) {
	env := newAPITestEnv(t)

	created, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-REVIEW-APPROVAL",
		Title:             "Review with approvals",
		Description:       "validate approval gate",
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
	})
	if err != nil {
		t.Fatalf("create case seed: %v", err)
	}

	_, err = env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID:  &env.tenantID,
		Kind:      "case_meta",
		OwnerID:   &env.identity.UserID,
		RefID:     &created.ID,
		CreatedBy: &env.identity.UserID,
		Data: map[string]any{
			"custom_fields": map[string]any{
				"closure_required_approvals": 2,
				"closure_approver_ids": []string{
					env.identity.UserID.String(),
					env.secondUserID.String(),
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("seed case_meta closure policy: %v", err)
	}

	closeCase := func(expectedStatus int) {
		t.Helper()
		c, rec := env.jsonContext(http.MethodPatch, "/api/v1/cases/"+created.ID.String(), map[string]any{
			"status": "closed",
		})
		setPath(c, "/api/v1/cases/:caseID", []string{"caseID"}, []string{created.ID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		updateErr := env.handler.UpdateCase(c)
		if expectedStatus == http.StatusOK {
			mustStatusOK(t, updateErr, rec, http.StatusOK)
			updated := decodeBody[models.Case](t, rec)
			if !strings.EqualFold(updated.Status, "closed") {
				t.Fatalf("expected closed status, got %q", updated.Status)
			}
			return
		}
		httpErr, ok := updateErr.(*echo.HTTPError)
		if !ok || httpErr.Code != expectedStatus {
			t.Fatalf("expected status %d, got err=%v code=%d body=%s", expectedStatus, updateErr, rec.Code, rec.Body.String())
		}
	}

	closeCase(http.StatusConflict)

	_, err = env.caseEvents.Create(context.Background(), repository.CreateCaseEventParams{
		TenantID:  env.tenantID,
		CaseID:    created.ID,
		EventType: "closure_approval",
		Title:     "Closure approved",
		Body:      "Approved by second analyst",
		ActorID:   &env.secondUserID,
		Metadata:  map[string]any{"approved": true},
	})
	if err != nil {
		t.Fatalf("create second analyst approval: %v", err)
	}
	closeCase(http.StatusConflict)

	_, err = env.caseEvents.Create(context.Background(), repository.CreateCaseEventParams{
		TenantID:  env.tenantID,
		CaseID:    created.ID,
		EventType: "closure_approval",
		Title:     "Closure approved",
		Body:      "Approved by tenant admin",
		ActorID:   &env.identity.UserID,
		Metadata:  map[string]any{"approved": true},
	})
	if err != nil {
		t.Fatalf("create tenant admin approval: %v", err)
	}
	closeCase(http.StatusOK)
}

func TestCreateCaseAutoAssignmentByWorkloadAndSimilarity(t *testing.T) {
	env := newAPITestEnv(t)
	passwordHash, err := security.HashPassword("Password123!")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	thirdUser, err := env.users.Create(context.Background(), repository.CreateUserParams{
		Username:        "analyst-third",
		Email:           "analyst-third@example.com",
		FullName:        "Third Analyst",
		PasswordHash:    passwordHash,
		IsPlatformAdmin: false,
	})
	if err != nil {
		t.Fatalf("create third user: %v", err)
	}
	if upsertErr := env.memberships.Upsert(context.Background(), env.tenantID, thirdUser.ID, models.TenantRoleAnalyst); upsertErr != nil {
		t.Fatalf("upsert third user membership: %v", upsertErr)
	}

	_, err = env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-AUTO-ASSIGN-001",
		Title:             "Seed 1",
		Description:       "seed second user",
		Source:            "siem",
		IncidentType:      "malware",
		Status:            "open",
		Priority:          "high",
		Impact:            "medium",
		Confidence:        70,
		Severity:          "high",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.identity.UserID,
		AssignedTo:        &env.secondUserID,
	})
	if err != nil {
		t.Fatalf("seed case 1: %v", err)
	}
	_, err = env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-AUTO-ASSIGN-002",
		Title:             "Seed 2",
		Description:       "seed second user",
		Source:            "edr",
		IncidentType:      "endpoint",
		Status:            "open",
		Priority:          "high",
		Impact:            "medium",
		Confidence:        70,
		Severity:          "high",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.identity.UserID,
		AssignedTo:        &env.secondUserID,
	})
	if err != nil {
		t.Fatalf("seed case 2: %v", err)
	}
	_, err = env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-AUTO-ASSIGN-003",
		Title:             "Seed 3",
		Description:       "seed third user",
		Source:            "cloud",
		IncidentType:      "iam",
		Status:            "open",
		Priority:          "medium",
		Impact:            "medium",
		Confidence:        50,
		Severity:          "medium",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.identity.UserID,
		AssignedTo:        &thirdUser.ID,
	})
	if err != nil {
		t.Fatalf("seed case 3: %v", err)
	}

	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/cases", map[string]any{
			"title":         "Autodistribution by load",
			"description":   "expect least loaded analyst",
			"source":        "manual",
			"incident_type": "triage",
			"severity":      "medium",
			"priority":      "medium",
		})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.CreateCase(c)
		mustStatusOK(t, err, rec, http.StatusCreated)
		created := decodeBody[models.Case](t, rec)
		if created.AssignedTo == nil || *created.AssignedTo != thirdUser.ID {
			t.Fatalf("expected least-loaded analyst %s, got %#v", thirdUser.ID, created.AssignedTo)
		}
	}

	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/cases", map[string]any{
			"title":         "Autodistribution by similarity",
			"description":   "expect same analyst as similar activity",
			"source":        "siem",
			"incident_type": "malware",
			"severity":      "high",
			"priority":      "high",
		})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.CreateCase(c)
		mustStatusOK(t, err, rec, http.StatusCreated)
		created := decodeBody[models.Case](t, rec)
		if created.AssignedTo == nil || *created.AssignedTo != env.secondUserID {
			t.Fatalf("expected similar-activity analyst %s, got %#v", env.secondUserID, created.AssignedTo)
		}
	}
}

func TestCreateCaseFromAlertsAutoAssignmentBySimilarity(t *testing.T) {
	env := newAPITestEnv(t)
	_, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-AUTO-FROM-ALERT-001",
		Title:             "Seed similar case",
		Description:       "seed similar",
		Source:            "siem",
		IncidentType:      "phishing",
		Status:            "open",
		Priority:          "high",
		Impact:            "medium",
		Confidence:        70,
		Severity:          "high",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.identity.UserID,
		AssignedTo:        &env.secondUserID,
	})
	if err != nil {
		t.Fatalf("seed similar case: %v", err)
	}

	seedAlert, err := env.alerts.Create(context.Background(), repository.CreateAlertParams{
		TenantID:    env.tenantID,
		Title:       "Phishing alert",
		Description: "mail compromise signal",
		Source:      "siem",
		Status:      "new",
		Severity:    "high",
		TLP:         "amber",
		PAP:         "amber",
		CreatedBy:   &env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("seed alert: %v", err)
	}

	c, rec := env.jsonContext(http.MethodPost, "/api/v1/alerts/bulk/create-case", map[string]any{
		"alert_ids": []string{seedAlert.ID.String()},
		"case": map[string]any{
			"title":         "Case from phishing alert",
			"incident_type": "phishing",
		},
	})
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)
	err = env.handler.CreateCaseFromAlerts(c)
	mustStatusOK(t, err, rec, http.StatusCreated)

	payload := decodeBody[map[string]any](t, rec)
	caseRaw, ok := payload["case"].(map[string]any)
	if !ok {
		t.Fatalf("expected case payload object, got %#v", payload["case"])
	}
	assignedID, _ := caseRaw["assigned_to"].(string)
	if strings.TrimSpace(assignedID) != env.secondUserID.String() {
		t.Fatalf("expected similar-activity assignee %s for case-from-alerts, got %q", env.secondUserID, assignedID)
	}
}

func TestEscalateCaseRequiresTargetTenantMembership(t *testing.T) {
	env := newAPITestEnv(t)
	sourceCase, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-ESC-FORBIDDEN",
		Title:             "Escalation forbidden",
		Description:       "source",
		Source:            "siem",
		IncidentType:      "phishing",
		Status:            "open",
		Priority:          "high",
		Impact:            "medium",
		Confidence:        70,
		Severity:          "high",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.identity.UserID,
		AssignedTo:        &env.secondUserID,
	})
	if err != nil {
		t.Fatalf("create source case: %v", err)
	}

	c, rec := env.jsonContext(http.MethodPost, "/api/v1/cases/"+sourceCase.ID.String()+"/escalate", map[string]any{
		"target_tenant_id": env.secondTenant.String(),
		"handoff_type":     "employee_client",
		"summary":          "Escalate to fraud team",
	})
	setPath(c, "/api/v1/cases/:caseID/escalate", []string{"caseID"}, []string{sourceCase.ID.String()})
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)
	err = env.handler.EscalateCase(c)
	httpErr, ok := err.(*echo.HTTPError)
	if !ok || httpErr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for escalation without target membership, got err=%v code=%d body=%s", err, rec.Code, rec.Body.String())
	}
}

func TestEscalateCaseCreatesTargetCaseAndCopiesContext(t *testing.T) {
	env := newAPITestEnv(t)
	if err := env.memberships.Upsert(context.Background(), env.secondTenant, env.identity.UserID, models.TenantRoleAnalyst); err != nil {
		t.Fatalf("upsert requester membership in target tenant: %v", err)
	}
	if err := env.memberships.Upsert(context.Background(), env.secondTenant, env.secondUserID, models.TenantRoleAnalyst); err != nil {
		t.Fatalf("upsert assignee membership in target tenant: %v", err)
	}

	sourceCase, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-ESC-SOURCE",
		Title:             "Escalation source case",
		Description:       "Source description",
		Source:            "siem",
		IncidentType:      "employee_client",
		Status:            "analysis",
		Priority:          "high",
		Impact:            "medium",
		Confidence:        65,
		Severity:          "high",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.identity.UserID,
		AssignedTo:        &env.secondUserID,
	})
	if err != nil {
		t.Fatalf("create source case: %v", err)
	}
	_, err = env.observables.Create(context.Background(), repository.CreateObservableParams{
		TenantID:  env.tenantID,
		CaseID:    sourceCase.ID,
		Type:      "email",
		Value:     "user@example.com",
		Verdict:   "suspicious",
		Source:    "manual",
		Tags:      []string{"phishing"},
		CreatedBy: env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create source observable: %v", err)
	}

	c, rec := env.jsonContext(http.MethodPost, "/api/v1/cases/"+sourceCase.ID.String()+"/escalate", map[string]any{
		"target_tenant_id":    env.secondTenant.String(),
		"target_assignee_id":  env.secondUserID.String(),
		"handoff_type":        "employee_client",
		"summary":             "Escalating with IOC context",
		"include_observables": true,
	})
	setPath(c, "/api/v1/cases/:caseID/escalate", []string{"caseID"}, []string{sourceCase.ID.String()})
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)
	err = env.handler.EscalateCase(c)
	mustStatusOK(t, err, rec, http.StatusCreated)

	payload := decodeBody[map[string]any](t, rec)
	targetCaseRaw, ok := payload["target_case"].(map[string]any)
	if !ok {
		t.Fatalf("expected target_case payload, got %#v", payload["target_case"])
	}
	targetCaseIDRaw, _ := targetCaseRaw["id"].(string)
	targetCaseID, parseErr := uuid.Parse(strings.TrimSpace(targetCaseIDRaw))
	if parseErr != nil {
		t.Fatalf("parse target case id: %v", parseErr)
	}
	targetAssignedTo, _ := targetCaseRaw["assigned_to"].(string)
	if strings.TrimSpace(targetAssignedTo) != env.secondUserID.String() {
		t.Fatalf("expected target assignee %s, got %q", env.secondUserID, targetAssignedTo)
	}
	if got, _ := payload["handoff_type"].(string); got != "employee_client" {
		t.Fatalf("unexpected handoff_type: %q", got)
	}

	targetCase, err := env.cases.GetByID(context.Background(), env.secondTenant, targetCaseID)
	if err != nil {
		t.Fatalf("load target case by id: %v", err)
	}
	if targetCase.TenantID != env.secondTenant {
		t.Fatalf("target case should belong to second tenant, got %s", targetCase.TenantID)
	}

	targetObservables, err := env.observables.ListByCase(context.Background(), env.secondTenant, targetCaseID, 100, 0)
	if err != nil {
		t.Fatalf("list target observables: %v", err)
	}
	if len(targetObservables) == 0 {
		t.Fatalf("expected copied observables in target case")
	}

	sourceEvents, err := env.caseEvents.ListByCase(context.Background(), env.tenantID, sourceCase.ID, 100, 0)
	if err != nil {
		t.Fatalf("list source case events: %v", err)
	}
	foundSourceEscalationEvent := false
	for _, item := range sourceEvents {
		if item.EventType == "case_escalated" {
			foundSourceEscalationEvent = true
			break
		}
	}
	if !foundSourceEscalationEvent {
		t.Fatalf("expected case_escalated event on source case")
	}

	targetEvents, err := env.caseEvents.ListByCase(context.Background(), env.secondTenant, targetCaseID, 100, 0)
	if err != nil {
		t.Fatalf("list target case events: %v", err)
	}
	foundTargetReceivedEvent := false
	for _, item := range targetEvents {
		if item.EventType == "case_escalation_received" {
			foundTargetReceivedEvent = true
			break
		}
	}
	if !foundTargetReceivedEvent {
		t.Fatalf("expected case_escalation_received event on target case")
	}
}

func TestShareCaseAcrossTenantsAllowsSingleEntityCollaboration(t *testing.T) {
	env := newAPITestEnv(t)

	sourceCase, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-SHARE-ONE",
		Title:             "Shared case",
		Description:       "single-entity collaboration",
		Source:            "siem",
		IncidentType:      "employee_client",
		Status:            "open",
		Priority:          "high",
		Impact:            "medium",
		Confidence:        70,
		Severity:          "high",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.identity.UserID,
		AssignedTo:        &env.secondUserID,
	})
	if err != nil {
		t.Fatalf("create source case: %v", err)
	}

	shareCtx, shareRec := env.jsonContext(http.MethodPost, "/api/v1/cases/"+sourceCase.ID.String()+"/share", map[string]any{
		"target_tenant_id": env.secondTenant.String(),
	})
	setPath(shareCtx, "/api/v1/cases/:caseID/share", []string{"caseID"}, []string{sourceCase.ID.String()})
	setIdentity(shareCtx, env.identity)
	setTenant(shareCtx, env.tenantID)
	err = env.handler.ShareCaseAcrossTenants(shareCtx)
	mustStatusOK(t, err, shareRec, http.StatusCreated)

	secondIdentity := models.Identity{
		UserID:          env.secondUserID,
		Username:        "analyst-second",
		IsPlatformAdmin: false,
	}

	getCtx, getRec := env.jsonContext(http.MethodGet, "/api/v1/cases/"+sourceCase.ID.String(), nil)
	setPath(getCtx, "/api/v1/cases/:caseID", []string{"caseID"}, []string{sourceCase.ID.String()})
	setIdentity(getCtx, secondIdentity)
	setTenant(getCtx, env.secondTenant)
	err = env.handler.GetCase(getCtx)
	mustStatusOK(t, err, getRec, http.StatusOK)
	got := decodeBody[map[string]any](t, getRec)
	if gotID, _ := got["id"].(string); strings.TrimSpace(gotID) != sourceCase.ID.String() {
		t.Fatalf("expected shared case id %s, got %q", sourceCase.ID, gotID)
	}

	updateCtx, updateRec := env.jsonContext(http.MethodPatch, "/api/v1/cases/"+sourceCase.ID.String(), map[string]any{
		"title": "Updated by second tenant",
	})
	setPath(updateCtx, "/api/v1/cases/:caseID", []string{"caseID"}, []string{sourceCase.ID.String()})
	setIdentity(updateCtx, secondIdentity)
	setTenant(updateCtx, env.secondTenant)
	err = env.handler.UpdateCase(updateCtx)
	mustStatusOK(t, err, updateRec, http.StatusOK)

	reloaded, err := env.cases.GetByID(context.Background(), env.tenantID, sourceCase.ID)
	if err != nil {
		t.Fatalf("reload owner case: %v", err)
	}
	if reloaded.Title != "Updated by second tenant" {
		t.Fatalf("expected shared case to be updated by second tenant, got title=%q", reloaded.Title)
	}
}

func TestListCasesIncludesSharedCasesForTargetTenant(t *testing.T) {
	env := newAPITestEnv(t)

	sourceCase, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-SHARE-LIST",
		Title:             "Shared in list",
		Description:       "must appear in target tenant list",
		Source:            "siem",
		IncidentType:      "employee_client",
		Status:            "open",
		Priority:          "medium",
		Impact:            "medium",
		Confidence:        55,
		Severity:          "medium",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create source case: %v", err)
	}
	if shareErr := env.cases.ShareWithTenant(context.Background(), env.tenantID, sourceCase.ID, env.secondTenant, &env.identity.UserID); shareErr != nil {
		t.Fatalf("share case: %v", shareErr)
	}

	secondIdentity := models.Identity{
		UserID:          env.secondUserID,
		Username:        "analyst-second",
		IsPlatformAdmin: false,
	}
	c, rec := env.jsonContext(http.MethodGet, "/api/v1/cases", nil)
	setIdentity(c, secondIdentity)
	setTenant(c, env.secondTenant)
	err = env.handler.ListCases(c)
	mustStatusOK(t, err, rec, http.StatusOK)

	items := decodeBody[[]map[string]any](t, rec)
	found := false
	for _, item := range items {
		if gotID, _ := item["id"].(string); strings.TrimSpace(gotID) == sourceCase.ID.String() {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected shared case %s to appear in target tenant list", sourceCase.ID)
	}
}

func TestHandlerCaseListSummaryIntegration(t *testing.T) {
	env := newAPITestEnv(t)

	var createdCase models.Case
	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/cases", map[string]any{
			"title":         "Summary coverage case",
			"description":   "Case summary integration",
			"severity":      "high",
			"incident_type": "summary_flow",
		})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.CreateCase(c)
		mustStatusOK(t, err, rec, http.StatusCreated)
		createdCase = decodeBody[models.Case](t, rec)
	}

	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/tasks", map[string]any{
			"case_id":     createdCase.ID.String(),
			"title":       "Open task",
			"description": "pending step",
			"status":      "new",
		})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.CreateTask(c)
		mustStatusOK(t, err, rec, http.StatusCreated)
	}
	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/tasks", map[string]any{
			"case_id":     createdCase.ID.String(),
			"title":       "Closed task",
			"description": "done step",
			"status":      "done",
		})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.CreateTask(c)
		mustStatusOK(t, err, rec, http.StatusCreated)
	}

	workflowCatalogItem, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID: &env.tenantID,
		Kind:     "workflows",
		Data: map[string]any{
			"name":    "summary flow",
			"enabled": true,
			"nodes":   []any{},
			"edges":   []any{},
		},
		CreatedBy: &env.userID,
	})
	if err != nil {
		t.Fatalf("create workflow catalog item: %v", err)
	}

	_, err = env.handler.workflowRuns.Create(context.Background(), repository.CreateWorkflowRunParams{
		TenantID:   env.tenantID,
		WorkflowID: workflowCatalogItem.ID,
		Trigger:    "manual",
		Status:     models.WorkflowRunStatusQueued,
		Input: map[string]any{
			"case_id": createdCase.ID.String(),
		},
		CreatedBy: &env.userID,
	})
	if err != nil {
		t.Fatalf("create queued workflow run: %v", err)
	}
	_, err = env.handler.workflowRuns.Create(context.Background(), repository.CreateWorkflowRunParams{
		TenantID:   env.tenantID,
		WorkflowID: workflowCatalogItem.ID,
		Trigger:    "manual",
		Status:     models.WorkflowRunStatusFailed,
		Input: map[string]any{
			"case_id": createdCase.ID.String(),
		},
		CreatedBy: &env.userID,
	})
	if err != nil {
		t.Fatalf("create failed workflow run: %v", err)
	}

	c, rec := env.jsonContext(http.MethodGet, "/api/v1/cases/summary?case_ids="+createdCase.ID.String(), nil)
	c.Request().URL.RawQuery = "case_ids=" + createdCase.ID.String()
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)
	err = env.handler.ListCaseListSummaries(c)
	mustStatusOK(t, err, rec, http.StatusOK)

	type summaryPayload struct {
		CaseID              string `json:"case_id"`
		OpenTasks           int    `json:"open_tasks"`
		ClosedTasks         int    `json:"closed_tasks"`
		TotalTasks          int    `json:"total_tasks"`
		TaskProgressPercent int    `json:"task_progress_percent"`
		ResponderStatuses   struct {
			Queued    int `json:"queued"`
			Running   int `json:"running"`
			Completed int `json:"completed"`
			Failed    int `json:"failed"`
		} `json:"responder_statuses"`
	}
	items := decodeBody[[]summaryPayload](t, rec)
	if len(items) != 1 {
		t.Fatalf("expected one summary item, got %d", len(items))
	}
	item := items[0]
	if item.CaseID != createdCase.ID.String() {
		t.Fatalf("expected case id %s, got %s", createdCase.ID.String(), item.CaseID)
	}
	if item.OpenTasks != 1 || item.ClosedTasks != 1 || item.TotalTasks != 2 {
		t.Fatalf("unexpected task summary %+v", item)
	}
	if item.TaskProgressPercent != 50 {
		t.Fatalf("expected progress 50, got %d", item.TaskProgressPercent)
	}
	if item.ResponderStatuses.Queued != 1 || item.ResponderStatuses.Failed != 1 {
		t.Fatalf("unexpected responder statuses %+v", item.ResponderStatuses)
	}
}

func TestHandlerActivityLivestreamIntegration(t *testing.T) {
	env := newAPITestEnv(t)

	createdCase, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:     env.tenantID,
		CaseNumber:   "CASE-LIVESTREAM-1",
		Title:        "Livestream case",
		Description:  "Case for livestream feed",
		Source:       "manual",
		IncidentType: "investigation",
		Status:       "new",
		Priority:     "high",
		Impact:       "high",
		Confidence:   90,
		Severity:     "high",
		TLP:          "amber",
		PAP:          "amber",
		CreatedBy:    env.userID,
		AssignedTo:   &env.secondUserID,
	})
	if err != nil {
		t.Fatalf("create case: %v", err)
	}

	createdAlert, err := env.alerts.Create(context.Background(), repository.CreateAlertParams{
		TenantID:    env.tenantID,
		CaseID:      &createdCase.ID,
		Title:       "Livestream alert",
		Description: "Alert for livestream feed",
		Source:      "siem",
		Status:      "new",
		Severity:    "critical",
		TLP:         "amber",
		PAP:         "amber",
		CreatedBy:   &env.userID,
		AssignedTo:  &env.secondUserID,
	})
	if err != nil {
		t.Fatalf("create alert: %v", err)
	}

	_, err = env.tasks.Create(context.Background(), repository.CreateTaskParams{
		CaseID:      createdCase.ID,
		TenantID:    env.tenantID,
		Title:       "Livestream task",
		Description: "Task for livestream feed",
		Status:      "new",
		AssigneeID:  &env.secondUserID,
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	secondAlert, err := env.alerts.Create(context.Background(), repository.CreateAlertParams{
		TenantID:    env.tenantID,
		CaseID:      &createdCase.ID,
		Title:       "Livestream alert page 2",
		Description: "Second alert for livestream pagination",
		Source:      "siem",
		Status:      "new",
		Severity:    "high",
		TLP:         "amber",
		PAP:         "amber",
		CreatedBy:   &env.userID,
		AssignedTo:  &env.secondUserID,
	})
	if err != nil {
		t.Fatalf("create second alert: %v", err)
	}

	c, rec := env.jsonContext(http.MethodGet, "/api/v1/activity/livestream?types=case,alert,task&limit=20", nil)
	c.Request().URL.RawQuery = "types=case,alert,task&limit=20"
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)
	err = env.handler.GetActivityLivestream(c)
	mustStatusOK(t, err, rec, http.StatusOK)

	type itemPayload struct {
		Entity     string `json:"entity"`
		EntityID   string `json:"entity_id"`
		AssigneeID string `json:"assignee_id"`
	}
	type responsePayload struct {
		Items   []itemPayload `json:"items"`
		Limit   int           `json:"limit"`
		Offset  int           `json:"offset"`
		HasMore bool          `json:"has_more"`
	}
	payload := decodeBody[responsePayload](t, rec)
	if len(payload.Items) < 3 {
		t.Fatalf("expected at least 3 livestream items, got %d", len(payload.Items))
	}
	seenEntities := map[string]bool{}
	for _, item := range payload.Items {
		if item.AssigneeID == env.secondUserID.String() {
			seenEntities[item.Entity] = true
		}
	}
	if !seenEntities["case"] {
		t.Fatalf("expected case activity for assignee %s", env.secondUserID)
	}
	if !seenEntities["alert"] {
		t.Fatalf("expected alert activity for assignee %s", env.secondUserID)
	}
	if !seenEntities["task"] {
		t.Fatalf("expected task activity for assignee %s", env.secondUserID)
	}

	filteredCtx, filteredRec := env.jsonContext(http.MethodGet, "/api/v1/activity/livestream?types=alert&assignee_id="+env.secondUserID.String(), nil)
	filteredCtx.Request().URL.RawQuery = "types=alert&assignee_id=" + env.secondUserID.String()
	setIdentity(filteredCtx, env.identity)
	setTenant(filteredCtx, env.tenantID)
	err = env.handler.GetActivityLivestream(filteredCtx)
	mustStatusOK(t, err, filteredRec, http.StatusOK)

	filteredPayload := decodeBody[responsePayload](t, filteredRec)
	if len(filteredPayload.Items) == 0 {
		t.Fatalf("expected filtered livestream response with alert items")
	}
	foundAlert := false
	for _, item := range filteredPayload.Items {
		if item.Entity != "alert" {
			t.Fatalf("expected only alert entities, got %s", item.Entity)
		}
		if item.AssigneeID != env.secondUserID.String() {
			t.Fatalf("expected assignee %s, got %s", env.secondUserID, item.AssigneeID)
		}
		if strings.EqualFold(createdAlert.ID.String(), item.EntityID) {
			foundAlert = true
		}
	}
	if !foundAlert {
		t.Fatalf("expected created alert %s in filtered stream", createdAlert.ID)
	}

	pagedFirstCtx, pagedFirstRec := env.jsonContext(
		http.MethodGet,
		"/api/v1/activity/livestream?types=alert&assignee_id="+env.secondUserID.String()+"&limit=1&offset=0",
		nil,
	)
	pagedFirstCtx.Request().URL.RawQuery = "types=alert&assignee_id=" + env.secondUserID.String() + "&limit=1&offset=0"
	setIdentity(pagedFirstCtx, env.identity)
	setTenant(pagedFirstCtx, env.tenantID)
	err = env.handler.GetActivityLivestream(pagedFirstCtx)
	mustStatusOK(t, err, pagedFirstRec, http.StatusOK)
	pagedFirstPayload := decodeBody[responsePayload](t, pagedFirstRec)
	if len(pagedFirstPayload.Items) != 1 {
		t.Fatalf("expected first page size 1, got %d", len(pagedFirstPayload.Items))
	}
	if pagedFirstPayload.Limit != 1 || pagedFirstPayload.Offset != 0 {
		t.Fatalf("unexpected first page limit/offset: %+v", pagedFirstPayload)
	}
	if !pagedFirstPayload.HasMore {
		t.Fatalf("expected has_more=true on first page")
	}

	pagedSecondCtx, pagedSecondRec := env.jsonContext(
		http.MethodGet,
		"/api/v1/activity/livestream?types=alert&assignee_id="+env.secondUserID.String()+"&limit=1&offset=1",
		nil,
	)
	pagedSecondCtx.Request().URL.RawQuery = "types=alert&assignee_id=" + env.secondUserID.String() + "&limit=1&offset=1"
	setIdentity(pagedSecondCtx, env.identity)
	setTenant(pagedSecondCtx, env.tenantID)
	err = env.handler.GetActivityLivestream(pagedSecondCtx)
	mustStatusOK(t, err, pagedSecondRec, http.StatusOK)
	pagedSecondPayload := decodeBody[responsePayload](t, pagedSecondRec)
	if len(pagedSecondPayload.Items) != 1 {
		t.Fatalf("expected second page size 1, got %d", len(pagedSecondPayload.Items))
	}
	if pagedSecondPayload.Offset != 1 {
		t.Fatalf("expected second page offset=1, got %d", pagedSecondPayload.Offset)
	}
	if pagedSecondPayload.Items[0].EntityID == pagedFirstPayload.Items[0].EntityID {
		t.Fatalf("expected different items on page 1 and page 2, got same id %s", pagedSecondPayload.Items[0].EntityID)
	}
	if pagedSecondPayload.Items[0].EntityID != createdAlert.ID.String() && pagedSecondPayload.Items[0].EntityID != secondAlert.ID.String() {
		t.Fatalf("unexpected alert id on page 2: %s", pagedSecondPayload.Items[0].EntityID)
	}
}

func TestHandlerSOCAccessPolicyEnforcementIntegration(t *testing.T) {
	env := newAPITestEnv(t)
	ctx := context.Background()

	analystIdentity := env.identity
	analystIdentity.UserID = env.secondUserID
	analystIdentity.TenantRole = models.TenantRoleAnalyst

	_, err := env.catalog.Create(ctx, repository.CatalogCreateParams{
		TenantID:  &env.tenantID,
		Kind:      "soc_access_policies",
		OwnerID:   &env.userID,
		CreatedBy: &env.userID,
		Data: map[string]any{
			"allowed_case_tags": []any{"prod-only"},
			"max_cases_in_work": 1,
		},
	})
	if err != nil {
		t.Fatalf("create soc policy: %v", err)
	}

	firstCase, err := env.cases.Create(ctx, repository.CreateCaseParams{
		TenantID:     env.tenantID,
		CaseNumber:   "CASE-SOC-POLICY-001",
		Title:        "First in-work case",
		Description:  "seed",
		Source:       "manual",
		IncidentType: "policy",
		Status:       "open",
		Priority:     "high",
		Impact:       "high",
		Confidence:   90,
		Severity:     "high",
		TLP:          "amber",
		PAP:          "amber",
		CreatedBy:    env.userID,
		AssignedTo:   &env.secondUserID,
	})
	if err != nil {
		t.Fatalf("create first case: %v", err)
	}
	_, err = env.catalog.Create(ctx, repository.CatalogCreateParams{
		TenantID:  &env.tenantID,
		Kind:      "case_meta",
		OwnerID:   &env.userID,
		RefID:     &firstCase.ID,
		CreatedBy: &env.userID,
		Data: map[string]any{
			"case_id": firstCase.ID.String(),
			"tags":    []any{"prod-only"},
		},
	})
	if err != nil {
		t.Fatalf("create first case meta: %v", err)
	}

	blockedCase, err := env.cases.Create(ctx, repository.CreateCaseParams{
		TenantID:     env.tenantID,
		CaseNumber:   "CASE-SOC-POLICY-002",
		Title:        "Blocked case",
		Description:  "seed",
		Source:       "manual",
		IncidentType: "policy",
		Status:       "open",
		Priority:     "high",
		Impact:       "high",
		Confidence:   90,
		Severity:     "high",
		TLP:          "amber",
		PAP:          "amber",
		CreatedBy:    env.userID,
	})
	if err != nil {
		t.Fatalf("create blocked case: %v", err)
	}
	_, err = env.catalog.Create(ctx, repository.CatalogCreateParams{
		TenantID:  &env.tenantID,
		Kind:      "case_meta",
		OwnerID:   &env.userID,
		RefID:     &blockedCase.ID,
		CreatedBy: &env.userID,
		Data: map[string]any{
			"case_id": blockedCase.ID.String(),
			"tags":    []any{"secret"},
		},
	})
	if err != nil {
		t.Fatalf("create blocked case meta: %v", err)
	}

	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/cases", map[string]any{
			"title":       "Overloaded assignee case",
			"description": "must be rejected",
			"status":      "open",
			"assigned_to": env.secondUserID.String(),
		})
		setIdentity(c, analystIdentity)
		setTenant(c, env.tenantID)
		err := env.handler.CreateCase(c)
		if httpErrorCode(t, err) != http.StatusConflict {
			t.Fatalf("expected 409 when assignee exceeds workload limit, got err=%v code=%d body=%s", err, rec.Code, rec.Body.String())
		}
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/cases/"+blockedCase.ID.String(), nil)
		setPath(c, "/api/v1/cases/:caseID", []string{"caseID"}, []string{blockedCase.ID.String()})
		setIdentity(c, analystIdentity)
		setTenant(c, env.tenantID)
		err := env.handler.GetCase(c)
		if httpErrorCode(t, err) != http.StatusNotFound {
			t.Fatalf("expected 404 for blocked case tag policy, got err=%v code=%d body=%s", err, rec.Code, rec.Body.String())
		}
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/cases", nil)
		setIdentity(c, analystIdentity)
		setTenant(c, env.tenantID)
		err := env.handler.ListCases(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		items := decodeBody[[]models.Case](t, rec)
		seenFirst := false
		for _, item := range items {
			if item.ID == firstCase.ID {
				seenFirst = true
			}
			if item.ID == blockedCase.ID {
				t.Fatalf("blocked case %s should be filtered by tag policy", blockedCase.ID)
			}
		}
		if !seenFirst {
			t.Fatalf("expected visible case %s in filtered response", firstCase.ID)
		}
	}
}
