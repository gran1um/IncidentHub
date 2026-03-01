package repository

import (
	"context"
	"testing"
)

func TestSystemRepositoryRespectsTenantCaseStatusClosedFlags(t *testing.T) {
	env := newRepositoryTestEnv(t)
	ctx := context.Background()

	statuses := []struct {
		code     string
		label    string
		order    int
		isClosed bool
	}{
		{code: "triage", label: "Triage", order: 10, isClosed: false},
		{code: "investigation", label: "Investigation", order: 20, isClosed: false},
		{code: "mitigated", label: "Mitigated", order: 90, isClosed: true},
	}
	for _, status := range statuses {
		_, err := env.catalog.Create(ctx, CatalogCreateParams{
			TenantID: &env.tenantID,
			Kind:     "case_statuses",
			Data: map[string]any{
				"code":      status.code,
				"label":     status.label,
				"order":     status.order,
				"is_closed": status.isClosed,
			},
			CreatedBy: &env.userID,
		})
		if err != nil {
			t.Fatalf("create tenant case status %s: %v", status.code, err)
		}
	}

	openCase := env.createCase("CASE-METRICS-OPEN")
	openStatus := "triage"
	if _, err := env.cases.Update(ctx, env.tenantID, openCase.ID, UpdateCaseParams{Status: &openStatus}); err != nil {
		t.Fatalf("set open case status: %v", err)
	}

	closedCase := env.createCase("CASE-METRICS-CLOSED")
	closedStatus := "mitigated"
	if _, err := env.cases.Update(ctx, env.tenantID, closedCase.ID, UpdateCaseParams{Status: &closedStatus}); err != nil {
		t.Fatalf("set closed case status: %v", err)
	}

	dashboardStats, err := env.system.TenantDashboardStats(ctx, env.tenantID)
	if err != nil {
		t.Fatalf("tenant dashboard stats: %v", err)
	}
	if dashboardStats.ActiveCases != 1 {
		t.Fatalf("expected exactly 1 active case, got %d", dashboardStats.ActiveCases)
	}
	if dashboardStats.ResolvedToday != 1 {
		t.Fatalf("expected exactly 1 resolved case today, got %d", dashboardStats.ResolvedToday)
	}

	resourceStats, err := env.system.TenantResourceStats(ctx, env.tenantID)
	if err != nil {
		t.Fatalf("tenant resource stats: %v", err)
	}
	if resourceStats.OpenCases != 1 {
		t.Fatalf("expected exactly 1 open case in resource stats, got %d", resourceStats.OpenCases)
	}
}
