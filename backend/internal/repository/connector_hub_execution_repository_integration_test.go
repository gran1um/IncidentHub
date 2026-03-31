package repository

import (
	"context"
	"testing"
	"time"

	"incidenthub/backend/internal/models"

	"github.com/google/uuid"
)

func TestConnectorHubRecoverStaleDispatchingPreservesTimestampType(t *testing.T) {
	env := newRepositoryTestEnv(t)
	ctx := context.Background()
	repo := NewConnectorHubExecutionRepository(env.pool)

	retryExecution := createTestConnectorHubExecution(t, ctx, repo, env.tenantID, env.userID, 3)
	deadLetterExecution := createTestConnectorHubExecution(t, ctx, repo, env.tenantID, env.userID, 1)

	if _, err := repo.ClaimReadyByID(ctx, retryExecution.ID); err != nil {
		t.Fatalf("claim retry execution: %v", err)
	}
	if _, err := repo.ClaimReadyByID(ctx, deadLetterExecution.ID); err != nil {
		t.Fatalf("claim dead-letter execution: %v", err)
	}

	retryAt := time.Now().UTC().Add(5 * time.Minute).Truncate(time.Second)
	staleBefore := time.Now().UTC().Add(time.Minute)

	recovered, err := repo.RecoverStaleDispatching(ctx, staleBefore, retryAt)
	if err != nil {
		t.Fatalf("recover stale dispatching: %v", err)
	}
	if len(recovered) != 2 {
		t.Fatalf("expected 2 recovered executions, got %d", len(recovered))
	}

	recoveredByID := make(map[uuid.UUID]models.ConnectorHubExecutionStatus, len(recovered))
	for _, item := range recovered {
		recoveredByID[item.ID] = item.Status
	}
	if recoveredByID[retryExecution.ID] != models.ConnectorHubExecutionStatusRetryScheduled {
		t.Fatalf("expected retry execution to be retry_scheduled, got %q", recoveredByID[retryExecution.ID])
	}
	if recoveredByID[deadLetterExecution.ID] != models.ConnectorHubExecutionStatusDeadLetter {
		t.Fatalf("expected dead-letter execution to be dead_letter, got %q", recoveredByID[deadLetterExecution.ID])
	}

	updatedRetry, err := repo.GetByID(ctx, retryExecution.ID)
	if err != nil {
		t.Fatalf("get retry execution: %v", err)
	}
	if updatedRetry.Status != models.ConnectorHubExecutionStatusRetryScheduled {
		t.Fatalf("expected retry execution status retry_scheduled, got %q", updatedRetry.Status)
	}
	if updatedRetry.NextAttemptAt == nil {
		t.Fatal("expected retry execution next_attempt_at to be set")
	}
	if !updatedRetry.NextAttemptAt.UTC().Equal(retryAt) {
		t.Fatalf("expected retry next_attempt_at %s, got %s", retryAt.Format(time.RFC3339), updatedRetry.NextAttemptAt.UTC().Format(time.RFC3339))
	}
	if updatedRetry.FinishedAt != nil {
		t.Fatalf("expected retry execution finished_at to stay nil, got %s", updatedRetry.FinishedAt.UTC().Format(time.RFC3339))
	}

	updatedDeadLetter, err := repo.GetByID(ctx, deadLetterExecution.ID)
	if err != nil {
		t.Fatalf("get dead-letter execution: %v", err)
	}
	if updatedDeadLetter.Status != models.ConnectorHubExecutionStatusDeadLetter {
		t.Fatalf("expected dead-letter status, got %q", updatedDeadLetter.Status)
	}
	if updatedDeadLetter.NextAttemptAt != nil {
		t.Fatalf("expected dead-letter next_attempt_at to be nil, got %s", updatedDeadLetter.NextAttemptAt.UTC().Format(time.RFC3339))
	}
	if updatedDeadLetter.FinishedAt == nil {
		t.Fatal("expected dead-letter finished_at to be set")
	}
}

func createTestConnectorHubExecution(
	t *testing.T,
	ctx context.Context,
	repo *ConnectorHubExecutionRepository,
	tenantID uuid.UUID,
	userID uuid.UUID,
	maxAttempts int,
) *models.ConnectorHubExecution {
	t.Helper()

	item, err := repo.Create(ctx, CreateConnectorHubExecutionParams{
		TenantID:         tenantID,
		ActorID:          &userID,
		ConnectorID:      uuid.New(),
		ConnectorName:    "Repository connector",
		ConnectorChannel: "mock",
		Action:           "lookup_hash",
		ExecutionMode:    "manual",
		DryRun:           false,
		Request: map[string]any{
			"hash": "44d88612fea8a8f36de82e1278abb02f",
		},
		Metadata: map[string]any{
			"source": "repository-test",
		},
		MaxAttempts:    maxAttempts,
		IdempotencyKey: uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("create connector hub execution: %v", err)
	}
	return item
}
