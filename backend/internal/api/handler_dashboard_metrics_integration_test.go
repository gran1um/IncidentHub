package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"incidenthub/backend/internal/repository"

	"github.com/google/uuid"
)

func TestHandlerDashboardMetricsAndCustomMetricsIntegration(t *testing.T) {
	env := newAPITestEnv(t)

	openCase, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-METRICS-OPEN-1",
		Title:             "Open phishing case",
		Description:       "Open case for metrics",
		Source:            "siem",
		IncidentType:      "phishing",
		Status:            "open",
		Priority:          "high",
		Impact:            "user",
		Confidence:        80,
		Severity:          "critical",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create open case: %v", err)
	}
	if _, execErr := env.pool.Exec(context.Background(), `
		UPDATE cases
		SET created_at = NOW() - INTERVAL '2 day', updated_at = NOW() - INTERVAL '2 day'
		WHERE id = $1
	`, openCase.ID); execErr != nil {
		t.Fatalf("adjust open case timestamps: %v", execErr)
	}
	_, err = env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID:  &env.tenantID,
		Kind:      "case_meta",
		OwnerID:   &env.identity.UserID,
		RefID:     &openCase.ID,
		CreatedBy: &env.identity.UserID,
		Data: map[string]any{
			"category": "phishing",
		},
	})
	if err != nil {
		t.Fatalf("create case_meta: %v", err)
	}

	closedAt := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	_, err = env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-METRICS-RES-1",
		Title:             "Resolved malware case",
		Description:       "Resolved case for analyst metrics",
		Source:            "edr",
		IncidentType:      "malware",
		Status:            "resolved",
		Priority:          "medium",
		Impact:            "endpoint",
		Confidence:        70,
		Severity:          "high",
		TLP:               "amber",
		PAP:               "amber",
		ClosedAt:          &closedAt,
		ResolutionSummary: "done",
		CreatedBy:         env.identity.UserID,
		AssignedTo:        &env.secondUserID,
	})
	if err != nil {
		t.Fatalf("create resolved case: %v", err)
	}

	_, err = env.alerts.Create(context.Background(), repository.CreateAlertParams{
		TenantID:    env.tenantID,
		Title:       "Pending alert",
		Description: "Alert for dashboard metrics",
		Source:      "edr",
		Status:      "new",
		Severity:    "high",
		TLP:         "amber",
		PAP:         "amber",
		CreatedBy:   &env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create alert: %v", err)
	}

	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/dashboard/metrics/custom", map[string]any{
			"name":    "Open critical cases",
			"source":  "cases",
			"measure": "count",
			"filters": map[string]any{
				"statuses":   []string{"open"},
				"severities": []string{"critical"},
			},
		})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		if err := env.handler.CreateDashboardCustomMetric(c); err != nil {
			t.Fatalf("create custom dashboard metric: %v", err)
		}
		if rec.Code != http.StatusCreated {
			t.Fatalf("unexpected status %d: %s", rec.Code, rec.Body.String())
		}
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/dashboard/metrics", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		if err := env.handler.GetDashboardMetrics(c); err != nil {
			t.Fatalf("get dashboard metrics: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status %d: %s", rec.Code, rec.Body.String())
		}
		payload := decodeBody[map[string]any](t, rec)
		if int(payload["open_cases"].(float64)) < 1 {
			t.Fatalf("expected open_cases >= 1, got %#v", payload["open_cases"])
		}
		if int(payload["open_alerts"].(float64)) < 1 {
			t.Fatalf("expected open_alerts >= 1, got %#v", payload["open_alerts"])
		}
		if int(payload["overdue_cases"].(float64)) < 1 {
			t.Fatalf("expected overdue_cases >= 1, got %#v", payload["overdue_cases"])
		}
		customMetrics, ok := payload["custom_metrics"].([]any)
		if !ok || len(customMetrics) == 0 {
			t.Fatalf("expected custom_metrics array with entries, got %#v", payload["custom_metrics"])
		}
		firstMetric, _ := customMetrics[0].(map[string]any)
		if int(firstMetric["value"].(float64)) < 1 {
			t.Fatalf("expected custom metric value >= 1, got %#v", firstMetric["value"])
		}
	}
}

func TestHandlerDashboardCustomMetricCRUDIntegration(t *testing.T) {
	env := newAPITestEnv(t)

	var metricID string
	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/dashboard/metrics/custom", map[string]any{
			"name":        "New alerts",
			"description": "Count new alerts",
			"source":      "alerts",
			"measure":     "count",
			"filters": map[string]any{
				"statuses": []string{"new"},
			},
		})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		if err := env.handler.CreateDashboardCustomMetric(c); err != nil {
			t.Fatalf("create custom metric: %v", err)
		}
		if rec.Code != http.StatusCreated {
			t.Fatalf("unexpected status %d: %s", rec.Code, rec.Body.String())
		}
		payload := decodeBody[map[string]any](t, rec)
		metricID = payload["id"].(string)
		if _, err := uuid.Parse(metricID); err != nil {
			t.Fatalf("metric id is not uuid: %v", err)
		}
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/dashboard/metrics/custom", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		if err := env.handler.ListDashboardCustomMetrics(c); err != nil {
			t.Fatalf("list custom metrics: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status %d: %s", rec.Code, rec.Body.String())
		}
		payload := decodeBody[map[string]any](t, rec)
		items, _ := payload["items"].([]any)
		if len(items) != 1 {
			t.Fatalf("expected 1 custom metric, got %d", len(items))
		}
	}

	{
		c, rec := env.jsonContext(http.MethodPatch, "/api/v1/dashboard/metrics/custom/"+metricID, map[string]any{
			"name":    "New alerts disabled",
			"source":  "alerts",
			"measure": "count",
			"enabled": false,
			"filters": map[string]any{
				"statuses": []string{"new"},
			},
		})
		setPath(c, "/api/v1/dashboard/metrics/custom/:metricID", []string{"metricID"}, []string{metricID})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		if err := env.handler.UpdateDashboardCustomMetric(c); err != nil {
			t.Fatalf("update custom metric: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status %d: %s", rec.Code, rec.Body.String())
		}
		payload := decodeBody[map[string]any](t, rec)
		if enabled, _ := payload["enabled"].(bool); enabled {
			t.Fatalf("expected metric enabled=false after patch")
		}
	}

	{
		c, rec := env.jsonContext(http.MethodDelete, "/api/v1/dashboard/metrics/custom/"+metricID, nil)
		setPath(c, "/api/v1/dashboard/metrics/custom/:metricID", []string{"metricID"}, []string{metricID})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		if err := env.handler.DeleteDashboardCustomMetric(c); err != nil {
			t.Fatalf("delete custom metric: %v", err)
		}
		if rec.Code != http.StatusNoContent {
			t.Fatalf("unexpected status %d: %s", rec.Code, rec.Body.String())
		}
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/dashboard/metrics/custom", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		if err := env.handler.ListDashboardCustomMetrics(c); err != nil {
			t.Fatalf("list custom metrics after delete: %v", err)
		}
		payload := decodeBody[map[string]any](t, rec)
		items, _ := payload["items"].([]any)
		if len(items) != 0 {
			t.Fatalf("expected no custom metrics after delete, got %d", len(items))
		}
	}
}
