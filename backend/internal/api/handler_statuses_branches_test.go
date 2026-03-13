package api

import (
	"context"
	"testing"

	"incidenthub/backend/internal/repository"
)

func TestLoadCaseStatusesGlobalAndTenantOverride(t *testing.T) {
	env := newAPITestEnv(t)

	_, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		Kind: "case_statuses",
		Data: map[string]any{
			"code":      "investigating",
			"label":     "Investigating",
			"order":     15,
			"is_closed": false,
		},
		CreatedBy: &env.userID,
	})
	if err != nil {
		t.Fatalf("create global status: %v", err)
	}

	globalStatuses, err := env.handler.loadCaseStatuses(context.Background(), env.tenantID)
	if err != nil {
		t.Fatalf("load global statuses: %v", err)
	}
	if len(globalStatuses) == 0 {
		t.Fatalf("expected global statuses")
	}

	_, err = env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID: &env.tenantID,
		Kind:     "case_statuses",
		Data: map[string]any{
			"code":      "incident",
			"label":     "Incident",
			"order":     10,
			"is_closed": false,
		},
		CreatedBy: &env.userID,
	})
	if err != nil {
		t.Fatalf("create tenant status: %v", err)
	}

	tenantStatuses, err := env.handler.loadCaseStatuses(context.Background(), env.tenantID)
	if err != nil {
		t.Fatalf("load tenant statuses: %v", err)
	}
	if len(tenantStatuses) == 0 || tenantStatuses[0].Code != "incident" {
		t.Fatalf("expected tenant statuses to override global set")
	}

	resolved, err := env.handler.resolveCaseStatus(context.Background(), env.tenantID, "")
	if err != nil {
		t.Fatalf("resolve default status: %v", err)
	}
	if resolved == "" {
		t.Fatalf("expected resolved status")
	}
}
