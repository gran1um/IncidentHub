package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"incidenthub/backend/internal/models"
)

func TestAIAgentQueueRepositoryEnqueueAndRequeueIntegration(t *testing.T) {
	env := newRepositoryTestEnv(t)
	repo := NewAIAgentQueueRepository(env.pool)

	event, created, err := repo.Enqueue(context.Background(), EnqueueAIAgentQueueEventParams{
		TenantID:   env.tenantID,
		ActorID:    &env.userID,
		EntityType: "case",
		EntityID:   env.createCase("CASE-AI-QUEUE-1").ID,
		Source:     "api",
	})
	if err != nil {
		t.Fatalf("enqueue ai queue event: %v", err)
	}
	if !created {
		t.Fatalf("expected first enqueue to create event")
	}
	if event.Status != models.AIAgentQueueStatusQueued {
		t.Fatalf("expected queued status, got %q", event.Status)
	}

	if markErr := repo.MarkProcessing(context.Background(), event.ID, "wf-queue-1"); markErr != nil {
		t.Fatalf("mark processing: %v", markErr)
	}

	requeued, requeuedCreated, err := repo.Enqueue(context.Background(), EnqueueAIAgentQueueEventParams{
		TenantID:   env.tenantID,
		EntityType: "case",
		EntityID:   event.EntityID,
		Source:     "api_retry",
	})
	if err != nil {
		t.Fatalf("re-enqueue ai queue event: %v", err)
	}
	if !requeuedCreated {
		t.Fatalf("expected enqueue upsert to report created=true")
	}
	if requeued.ID != event.ID {
		t.Fatalf("expected re-enqueue to reuse same event id, got old=%s new=%s", event.ID.String(), requeued.ID.String())
	}
	if requeued.Status != models.AIAgentQueueStatusQueued {
		t.Fatalf("expected re-enqueued event status queued, got %q", requeued.Status)
	}
	if strings.TrimSpace(requeued.WorkflowID) != "" {
		t.Fatalf("expected workflow id reset after re-enqueue, got %q", requeued.WorkflowID)
	}

	queued, err := repo.ListByStatus(context.Background(), models.AIAgentQueueStatusQueued, 50)
	if err != nil {
		t.Fatalf("list queued events: %v", err)
	}
	if len(queued) == 0 {
		t.Fatalf("expected at least one queued event")
	}
}

func TestAIAgentQueueRepositoryMarkDoneAndFailedIntegration(t *testing.T) {
	env := newRepositoryTestEnv(t)
	repo := NewAIAgentQueueRepository(env.pool)

	alert, err := env.alerts.Create(context.Background(), CreateAlertParams{
		TenantID:    env.tenantID,
		Title:       "Queue repo alert",
		Description: "Queue repo test",
		Source:      "siem",
		Status:      "new",
		Severity:    "medium",
		TLP:         "amber",
		PAP:         "amber",
		CreatedBy:   &env.userID,
	})
	if err != nil {
		t.Fatalf("create alert seed: %v", err)
	}

	event, _, err := repo.Enqueue(context.Background(), EnqueueAIAgentQueueEventParams{
		TenantID:   env.tenantID,
		ActorID:    &env.userID,
		EntityType: "alert",
		EntityID:   alert.ID,
		Source:     "api",
	})
	if err != nil {
		t.Fatalf("enqueue alert event: %v", err)
	}

	if markErr := repo.MarkProcessing(context.Background(), event.ID, "wf-queue-2"); markErr != nil {
		t.Fatalf("mark processing: %v", markErr)
	}
	if doneErr := repo.MarkDone(context.Background(), event.ID, CompleteAIAgentQueueEventParams{
		MatchedAgents:   2,
		ProcessedAgents: 2,
	}); doneErr != nil {
		t.Fatalf("mark done: %v", doneErr)
	}
	storedDone, err := repo.GetByID(context.Background(), event.ID)
	if err != nil {
		t.Fatalf("get done event: %v", err)
	}
	if storedDone.Status != models.AIAgentQueueStatusDone {
		t.Fatalf("expected done status, got %q", storedDone.Status)
	}
	if storedDone.MatchedAgents != 2 || storedDone.ProcessedAgents != 2 {
		t.Fatalf("unexpected done counters matched=%d processed=%d", storedDone.MatchedAgents, storedDone.ProcessedAgents)
	}

	if _, _, enqueueErr := repo.Enqueue(context.Background(), EnqueueAIAgentQueueEventParams{
		TenantID:   env.tenantID,
		ActorID:    &env.userID,
		EntityType: "alert",
		EntityID:   alert.ID,
		Source:     "api_retry",
	}); enqueueErr != nil {
		t.Fatalf("re-enqueue event for failed status flow: %v", enqueueErr)
	}
	if markErr := repo.MarkProcessing(context.Background(), event.ID, "wf-queue-3"); markErr != nil {
		t.Fatalf("mark processing second run: %v", markErr)
	}
	if failedErr := repo.MarkFailed(context.Background(), event.ID, CompleteAIAgentQueueEventParams{
		MatchedAgents:   3,
		ProcessedAgents: 1,
		LastError:       strings.Repeat("x", 5000),
	}); failedErr != nil {
		t.Fatalf("mark failed: %v", failedErr)
	}
	storedFailed, err := repo.GetByID(context.Background(), event.ID)
	if err != nil {
		t.Fatalf("get failed event: %v", err)
	}
	if storedFailed.Status != models.AIAgentQueueStatusFailed {
		t.Fatalf("expected failed status, got %q", storedFailed.Status)
	}
	if storedFailed.MatchedAgents != 3 || storedFailed.ProcessedAgents != 1 {
		t.Fatalf("unexpected failed counters matched=%d processed=%d", storedFailed.MatchedAgents, storedFailed.ProcessedAgents)
	}
	if len(storedFailed.LastError) > 3000 {
		t.Fatalf("expected last error to be trimmed to 3000 chars, got %d", len(storedFailed.LastError))
	}
}

func TestAIAgentQueueRepositoryTenantStatusViewsIntegration(t *testing.T) {
	env := newRepositoryTestEnv(t)
	repo := NewAIAgentQueueRepository(env.pool)

	caseOne := env.createCase("CASE-AI-QUEUE-VIEW-1")
	caseTwo := env.createCase("CASE-AI-QUEUE-VIEW-2")
	caseThree := env.createCase("CASE-AI-QUEUE-VIEW-3")

	eventQueued, _, err := repo.Enqueue(context.Background(), EnqueueAIAgentQueueEventParams{
		TenantID:   env.tenantID,
		ActorID:    &env.userID,
		EntityType: "case",
		EntityID:   caseOne.ID,
		Source:     "api",
	})
	if err != nil {
		t.Fatalf("enqueue queued event: %v", err)
	}

	eventProcessing, _, err := repo.Enqueue(context.Background(), EnqueueAIAgentQueueEventParams{
		TenantID:   env.tenantID,
		ActorID:    &env.userID,
		EntityType: "case",
		EntityID:   caseTwo.ID,
		Source:     "api",
	})
	if err != nil {
		t.Fatalf("enqueue processing event: %v", err)
	}
	if markErr := repo.MarkProcessing(context.Background(), eventProcessing.ID, "wf-monitor-1"); markErr != nil {
		t.Fatalf("mark processing event: %v", markErr)
	}

	eventDone, _, err := repo.Enqueue(context.Background(), EnqueueAIAgentQueueEventParams{
		TenantID:   env.tenantID,
		ActorID:    &env.userID,
		EntityType: "case",
		EntityID:   caseThree.ID,
		Source:     "api",
	})
	if err != nil {
		t.Fatalf("enqueue done event: %v", err)
	}
	if markErr := repo.MarkProcessing(context.Background(), eventDone.ID, "wf-monitor-2"); markErr != nil {
		t.Fatalf("mark processing done event: %v", markErr)
	}
	if doneErr := repo.MarkDone(context.Background(), eventDone.ID, CompleteAIAgentQueueEventParams{
		MatchedAgents:   1,
		ProcessedAgents: 1,
	}); doneErr != nil {
		t.Fatalf("mark done event: %v", doneErr)
	}

	queuedItems, err := repo.ListByTenantAndStatus(context.Background(), env.tenantID, models.AIAgentQueueStatusQueued, 10, 0)
	if err != nil {
		t.Fatalf("list queued by tenant: %v", err)
	}
	if len(queuedItems) != 1 || queuedItems[0].ID != eventQueued.ID {
		t.Fatalf("unexpected queued items: %+v", queuedItems)
	}

	processingItems, err := repo.ListByTenantAndStatus(context.Background(), env.tenantID, models.AIAgentQueueStatusProcessing, 10, 0)
	if err != nil {
		t.Fatalf("list processing by tenant: %v", err)
	}
	if len(processingItems) != 1 || processingItems[0].ID != eventProcessing.ID {
		t.Fatalf("unexpected processing items: %+v", processingItems)
	}

	counts, err := repo.CountByTenantAndStatus(context.Background(), env.tenantID)
	if err != nil {
		t.Fatalf("count by tenant and status: %v", err)
	}
	if counts[models.AIAgentQueueStatusQueued] != 1 {
		t.Fatalf("expected queued count 1, got %d", counts[models.AIAgentQueueStatusQueued])
	}
	if counts[models.AIAgentQueueStatusProcessing] != 1 {
		t.Fatalf("expected processing count 1, got %d", counts[models.AIAgentQueueStatusProcessing])
	}
	if counts[models.AIAgentQueueStatusDone] != 1 {
		t.Fatalf("expected done count 1, got %d", counts[models.AIAgentQueueStatusDone])
	}
	if counts[models.AIAgentQueueStatusFailed] != 0 {
		t.Fatalf("expected failed count 0, got %d", counts[models.AIAgentQueueStatusFailed])
	}
}

func TestAIAgentQueueRepositoryRetryLifecycleAndManualControlsIntegration(t *testing.T) {
	env := newRepositoryTestEnv(t)
	repo := NewAIAgentQueueRepository(env.pool)

	caseItem := env.createCase("CASE-AI-QUEUE-RETRY-1")
	event, _, err := repo.Enqueue(context.Background(), EnqueueAIAgentQueueEventParams{
		TenantID:    env.tenantID,
		ActorID:     &env.userID,
		EntityType:  "case",
		EntityID:    caseItem.ID,
		Source:      "api",
		MaxAttempts: 3,
	})
	if err != nil {
		t.Fatalf("enqueue retry event: %v", err)
	}

	if markErr := repo.MarkProcessing(context.Background(), event.ID, "wf-retry-1"); markErr != nil {
		t.Fatalf("mark processing retry event: %v", markErr)
	}
	if requeueErr := repo.RequeueAfterFailure(context.Background(), event.ID, CompleteAIAgentQueueEventParams{
		MatchedAgents:   2,
		ProcessedAgents: 1,
		LastError:       "temporary llm timeout",
	}); requeueErr != nil {
		t.Fatalf("requeue after failure: %v", requeueErr)
	}
	storedQueued, err := repo.GetByID(context.Background(), event.ID)
	if err != nil {
		t.Fatalf("get queued retry event: %v", err)
	}
	if storedQueued.Status != models.AIAgentQueueStatusQueued {
		t.Fatalf("expected queued status after retry, got %q", storedQueued.Status)
	}
	if storedQueued.AttemptCount != 1 {
		t.Fatalf("expected attempt_count=1 after first processing attempt, got %d", storedQueued.AttemptCount)
	}
	if strings.TrimSpace(storedQueued.LastError) == "" {
		t.Fatalf("expected non-empty last error after retry")
	}

	restarted, err := repo.Restart(context.Background(), event.ID, 5)
	if err != nil {
		t.Fatalf("restart queue event: %v", err)
	}
	if restarted.Status != models.AIAgentQueueStatusQueued {
		t.Fatalf("expected queued status after manual restart, got %q", restarted.Status)
	}
	if restarted.AttemptCount != 0 {
		t.Fatalf("expected attempt_count reset to 0 after restart, got %d", restarted.AttemptCount)
	}
	if restarted.MaxAttempts != 5 {
		t.Fatalf("expected max_attempts=5 after restart, got %d", restarted.MaxAttempts)
	}

	closed, err := repo.CloseByUser(context.Background(), event.ID)
	if err != nil {
		t.Fatalf("close queue event: %v", err)
	}
	if closed.Status != models.AIAgentQueueStatusDone {
		t.Fatalf("expected done status after close, got %q", closed.Status)
	}
	if !closed.ClosedByUser {
		t.Fatalf("expected closed_by_user=true after close")
	}
	if closed.ClosedAt == nil {
		t.Fatalf("expected closed_at to be set after close")
	}
}

func TestAIAgentQueueRepositoryRecoverStaleProcessingIntegration(t *testing.T) {
	env := newRepositoryTestEnv(t)
	repo := NewAIAgentQueueRepository(env.pool)

	caseItem := env.createCase("CASE-AI-QUEUE-STALE-1")
	event, _, err := repo.Enqueue(context.Background(), EnqueueAIAgentQueueEventParams{
		TenantID:    env.tenantID,
		ActorID:     &env.userID,
		EntityType:  "case",
		EntityID:    caseItem.ID,
		Source:      "api",
		MaxAttempts: 2,
	})
	if err != nil {
		t.Fatalf("enqueue stale event: %v", err)
	}
	if markErr := repo.MarkProcessing(context.Background(), event.ID, "wf-stale-1"); markErr != nil {
		t.Fatalf("mark processing stale event first attempt: %v", markErr)
	}

	recovered, err := repo.RecoverStaleProcessing(context.Background(), time.Now().UTC().Add(time.Minute))
	if err != nil {
		t.Fatalf("recover stale processing first pass: %v", err)
	}
	if recovered.Requeued != 1 || recovered.Failed != 0 {
		t.Fatalf("expected stale recovery to requeue once, got %+v", recovered)
	}

	if markErr := repo.MarkProcessing(context.Background(), event.ID, "wf-stale-2"); markErr != nil {
		t.Fatalf("mark processing stale event second attempt: %v", markErr)
	}
	recovered, err = repo.RecoverStaleProcessing(context.Background(), time.Now().UTC().Add(time.Minute))
	if err != nil {
		t.Fatalf("recover stale processing second pass: %v", err)
	}
	if recovered.Requeued != 0 || recovered.Failed != 1 {
		t.Fatalf("expected stale recovery to fail event on second pass, got %+v", recovered)
	}

	stored, err := repo.GetByID(context.Background(), event.ID)
	if err != nil {
		t.Fatalf("get stale recovered event: %v", err)
	}
	if stored.Status != models.AIAgentQueueStatusFailed {
		t.Fatalf("expected failed status after max attempts reached, got %q", stored.Status)
	}
}
