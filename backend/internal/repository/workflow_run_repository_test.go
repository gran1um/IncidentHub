package repository

import (
	"context"
	"fmt"
	"testing"

	"incidenthub/backend/internal/models"
)

func TestWorkflowRunRepositoryRetention(t *testing.T) {
	env := newRepositoryTestEnv(t)
	ctx := context.Background()
	repo := NewWorkflowRunRepository(env.pool)

	workflow, err := env.catalog.Create(ctx, CatalogCreateParams{
		TenantID:  &env.tenantID,
		Kind:      "analyzer_workflows",
		OwnerID:   &env.userID,
		CreatedBy: &env.userID,
		Data: map[string]any{
			"name": "Retention check workflow",
			"definition": map[string]any{
				"entryNodeId": "trigger_1",
				"nodes": []map[string]any{
					{"id": "trigger_1", "type": "trigger", "label": "Trigger"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create workflow catalog item: %v", err)
	}

	totalRuns := workflowRunRetentionLimit + 25
	for i := 0; i < totalRuns; i++ {
		run, createErr := repo.Create(ctx, CreateWorkflowRunParams{
			TenantID:     env.tenantID,
			WorkflowID:   workflow.ID,
			WorkflowKind: "analyzer_workflows",
			Trigger:      "manual_test",
			Status:       models.WorkflowRunStatusRunning,
			Input:        map[string]any{"index": i},
			CreatedBy:    &env.userID,
		})
		if createErr != nil {
			t.Fatalf("create workflow run #%d: %v", i, createErr)
		}
		if _, finishErr := repo.Finish(ctx, run.ID, env.tenantID, FinishWorkflowRunParams{
			Status: models.WorkflowRunStatusSuccess,
			Result: map[string]any{"index": i},
		}); finishErr != nil {
			t.Fatalf("finish workflow run #%d: %v", i, finishErr)
		}
	}

	var count int
	if countErr := env.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM workflow_runs
		WHERE tenant_id = $1
		  AND workflow_id = $2
	`, env.tenantID, workflow.ID).Scan(&count); countErr != nil {
		t.Fatalf("count workflow runs: %v", countErr)
	}
	if count != workflowRunRetentionLimit {
		t.Fatalf("expected retention to keep %d runs, got %d", workflowRunRetentionLimit, count)
	}

	runs, err := repo.ListByWorkflow(ctx, env.tenantID, workflow.ID, workflowRunRetentionLimit+200)
	if err != nil {
		t.Fatalf("list workflow runs: %v", err)
	}
	if len(runs) != workflowRunRetentionLimit {
		t.Fatalf("expected %d runs from repository list, got %d", workflowRunRetentionLimit, len(runs))
	}

	newest := runs[0].Result["index"]
	oldest := runs[len(runs)-1].Result["index"]
	if fmt.Sprint(newest) != fmt.Sprint(totalRuns-1) {
		t.Fatalf("expected newest run index %d, got %v", totalRuns-1, newest)
	}
	if fmt.Sprint(oldest) != fmt.Sprint(totalRuns-workflowRunRetentionLimit) {
		t.Fatalf("expected oldest kept run index %d, got %v", totalRuns-workflowRunRetentionLimit, oldest)
	}
}
