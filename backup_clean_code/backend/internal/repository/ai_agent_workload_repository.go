package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"incidenthub/backend/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AIAgentWorkloadRepository struct {
	pool *pgxpool.Pool
}

type UpsertAIAgentWorkloadParams struct {
	TenantID          uuid.UUID
	QueueEventID      uuid.UUID
	AgentID           uuid.UUID
	AgentName         string
	ExecutionPolicy   string
	ExecutionPriority int
	ExecutionIndex    int
	EntityType        string
	EntityID          uuid.UUID
	MaxAttempts       int
}

type CompleteAIAgentWorkloadParams struct {
	RunID      *uuid.UUID
	LastStage  string
	LastError  string
	WorkflowID string
}

func NewAIAgentWorkloadRepository(pool *pgxpool.Pool) *AIAgentWorkloadRepository {
	return &AIAgentWorkloadRepository{pool: pool}
}

func normalizeAIAgentWorkloadStatus(raw string) models.AIAgentWorkloadStatus {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case string(models.AIAgentWorkloadStatusQueued):
		return models.AIAgentWorkloadStatusQueued
	case string(models.AIAgentWorkloadStatusProcessing):
		return models.AIAgentWorkloadStatusProcessing
	case string(models.AIAgentWorkloadStatusDone):
		return models.AIAgentWorkloadStatusDone
	case string(models.AIAgentWorkloadStatusFailed):
		return models.AIAgentWorkloadStatusFailed
	case string(models.AIAgentWorkloadStatusCancelled), "cancelled":
		return models.AIAgentWorkloadStatusCancelled
	default:
		return ""
	}
}

func normalizeAIAgentExecutionPolicy(raw string) models.AIAgentExecutionPolicy {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case string(models.AIAgentExecutionPolicyExclusive):
		return models.AIAgentExecutionPolicyExclusive
	case string(models.AIAgentExecutionPolicyFirstMatch):
		return models.AIAgentExecutionPolicyFirstMatch
	case string(models.AIAgentExecutionPolicyFallbackChain):
		return models.AIAgentExecutionPolicyFallbackChain
	default:
		return models.AIAgentExecutionPolicyAllMatching
	}
}

func (r *AIAgentWorkloadRepository) Upsert(ctx context.Context, p UpsertAIAgentWorkloadParams) (*models.AIAgentWorkload, error) {
	entityType := normalizeAIAgentQueueEntityType(p.EntityType)
	if entityType == "" {
		return nil, fmt.Errorf("invalid ai agent workload entity_type: %q", p.EntityType)
	}
	maxAttempts := normalizeAIAgentQueueMaxAttempts(p.MaxAttempts)
	executionPolicy := normalizeAIAgentExecutionPolicy(p.ExecutionPolicy)
	executionPriority := p.ExecutionPriority
	executionIndex := p.ExecutionIndex
	if executionIndex < 0 {
		executionIndex = 0
	}
	row := r.pool.QueryRow(ctx, `
		INSERT INTO ai_agent_workloads (
			tenant_id, queue_event_id, agent_id, agent_name, execution_policy, execution_priority, execution_index, entity_type, entity_id,
			status, attempt_count, max_attempts, workflow_id, run_id, last_stage, last_error,
			closed_by_user, started_at, finished_at, closed_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'queued', 0, $10, '', NULL, '', '', false, NULL, NULL, NULL, NOW(), NOW())
		ON CONFLICT (queue_event_id, agent_id)
		DO UPDATE
		   SET agent_name = EXCLUDED.agent_name,
		       execution_policy = EXCLUDED.execution_policy,
		       execution_priority = EXCLUDED.execution_priority,
		       execution_index = EXCLUDED.execution_index,
		       max_attempts = GREATEST(ai_agent_workloads.max_attempts, EXCLUDED.max_attempts),
		       updated_at = NOW()
		RETURNING id, tenant_id, queue_event_id, agent_id, agent_name, execution_policy, execution_priority, execution_index, entity_type, entity_id,
		          status, attempt_count, max_attempts, workflow_id, run_id, last_stage, last_error,
		          closed_by_user, created_at, started_at, finished_at, closed_at, updated_at
	`, p.TenantID, p.QueueEventID, p.AgentID, strings.TrimSpace(p.AgentName), string(executionPolicy), executionPriority, executionIndex, entityType, p.EntityID, maxAttempts)
	item, err := scanAIAgentWorkload(row)
	if err != nil {
		return nil, fmt.Errorf("upsert ai agent workload: %w", err)
	}
	return item, nil
}

func (r *AIAgentWorkloadRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.AIAgentWorkload, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, queue_event_id, agent_id, agent_name, execution_policy, execution_priority, execution_index, entity_type, entity_id,
		       status, attempt_count, max_attempts, workflow_id, run_id, last_stage, last_error,
		       closed_by_user, created_at, started_at, finished_at, closed_at, updated_at
		FROM ai_agent_workloads
		WHERE id = $1
	`, id)
	item, err := scanAIAgentWorkload(row)
	if err != nil {
		return nil, fmt.Errorf("get ai agent workload: %w", err)
	}
	return item, nil
}

func (r *AIAgentWorkloadRepository) ListByQueueEvent(ctx context.Context, queueEventID uuid.UUID) ([]models.AIAgentWorkload, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, queue_event_id, agent_id, agent_name, execution_policy, execution_priority, execution_index, entity_type, entity_id,
		       status, attempt_count, max_attempts, workflow_id, run_id, last_stage, last_error,
		       closed_by_user, created_at, started_at, finished_at, closed_at, updated_at
		FROM ai_agent_workloads
		WHERE queue_event_id = $1
		ORDER BY execution_index ASC, execution_priority DESC, created_at ASC, agent_name ASC
	`, queueEventID)
	if err != nil {
		return nil, fmt.Errorf("list ai agent workloads by queue event: %w", err)
	}
	defer rows.Close()

	items := make([]models.AIAgentWorkload, 0)
	for rows.Next() {
		item, scanErr := scanAIAgentWorkload(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ai agent workloads: %w", err)
	}
	return items, nil
}

func (r *AIAgentWorkloadRepository) ListByTenantAndStatus(
	ctx context.Context,
	tenantID uuid.UUID,
	status models.AIAgentWorkloadStatus,
	limit int,
	offset int,
) ([]models.AIAgentWorkload, error) {
	return listByTenantAndStatus(
		ctx,
		r.pool,
		`
			SELECT id, tenant_id, queue_event_id, agent_id, agent_name, execution_policy, execution_priority, execution_index, entity_type, entity_id,
			       status, attempt_count, max_attempts, workflow_id, run_id, last_stage, last_error,
			       closed_by_user, created_at, started_at, finished_at, closed_at, updated_at
			FROM ai_agent_workloads
			WHERE tenant_id = $1
			  AND status = $2
			ORDER BY created_at ASC
			LIMIT $3 OFFSET $4
		`,
		tenantID,
		string(status),
		limit,
		offset,
		scanAIAgentWorkload,
		"list ai agent workloads by tenant and status",
		"scan ai agent workload",
		"iterate ai agent workloads by tenant and status",
	)
}

func (r *AIAgentWorkloadRepository) MarkProcessing(ctx context.Context, id uuid.UUID, workflowID string) (*models.AIAgentWorkload, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE ai_agent_workloads
		SET status = 'processing',
		    attempt_count = attempt_count + 1,
		    workflow_id = $2,
		    started_at = NOW(),
		    finished_at = NULL,
		    closed_by_user = false,
		    closed_at = NULL,
		    updated_at = NOW()
		WHERE id = $1
		  AND status = 'queued'
		RETURNING id, tenant_id, queue_event_id, agent_id, agent_name, execution_policy, execution_priority, execution_index, entity_type, entity_id,
		          status, attempt_count, max_attempts, workflow_id, run_id, last_stage, last_error,
		          closed_by_user, created_at, started_at, finished_at, closed_at, updated_at
	`, id, strings.TrimSpace(workflowID))
	item, err := scanAIAgentWorkload(row)
	if err != nil {
		return nil, fmt.Errorf("mark ai agent workload processing: %w", err)
	}
	return item, nil
}

func (r *AIAgentWorkloadRepository) MarkDone(ctx context.Context, id uuid.UUID, p CompleteAIAgentWorkloadParams) (*models.AIAgentWorkload, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE ai_agent_workloads
		SET status = 'done',
		    workflow_id = COALESCE(NULLIF($2, ''), workflow_id),
		    run_id = COALESCE($3, run_id),
		    last_stage = COALESCE(NULLIF($4, ''), last_stage),
		    last_error = '',
		    closed_by_user = false,
		    closed_at = NULL,
		    finished_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id, tenant_id, queue_event_id, agent_id, agent_name, execution_policy, execution_priority, execution_index, entity_type, entity_id,
		          status, attempt_count, max_attempts, workflow_id, run_id, last_stage, last_error,
		          closed_by_user, created_at, started_at, finished_at, closed_at, updated_at
	`, id, strings.TrimSpace(p.WorkflowID), p.RunID, strings.TrimSpace(p.LastStage))
	item, err := scanAIAgentWorkload(row)
	if err != nil {
		return nil, fmt.Errorf("mark ai agent workload done: %w", err)
	}
	return item, nil
}

func (r *AIAgentWorkloadRepository) MarkRetryQueued(ctx context.Context, id uuid.UUID, p CompleteAIAgentWorkloadParams) (*models.AIAgentWorkload, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE ai_agent_workloads
		SET status = 'queued',
		    workflow_id = '',
		    run_id = COALESCE($2, run_id),
		    last_stage = COALESCE(NULLIF($3, ''), last_stage),
		    last_error = $4,
		    started_at = NULL,
		    finished_at = NULL,
		    closed_by_user = false,
		    closed_at = NULL,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id, tenant_id, queue_event_id, agent_id, agent_name, execution_policy, execution_priority, execution_index, entity_type, entity_id,
		          status, attempt_count, max_attempts, workflow_id, run_id, last_stage, last_error,
		          closed_by_user, created_at, started_at, finished_at, closed_at, updated_at
	`, id, p.RunID, strings.TrimSpace(p.LastStage), trimAIAgentQueueError(p.LastError))
	item, err := scanAIAgentWorkload(row)
	if err != nil {
		return nil, fmt.Errorf("mark ai agent workload retry queued: %w", err)
	}
	return item, nil
}

func (r *AIAgentWorkloadRepository) MarkFailed(ctx context.Context, id uuid.UUID, p CompleteAIAgentWorkloadParams) (*models.AIAgentWorkload, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE ai_agent_workloads
		SET status = 'failed',
		    workflow_id = COALESCE(NULLIF($2, ''), workflow_id),
		    run_id = COALESCE($3, run_id),
		    last_stage = COALESCE(NULLIF($4, ''), last_stage),
		    last_error = $5,
		    closed_by_user = false,
		    closed_at = NULL,
		    finished_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id, tenant_id, queue_event_id, agent_id, agent_name, execution_policy, execution_priority, execution_index, entity_type, entity_id,
		          status, attempt_count, max_attempts, workflow_id, run_id, last_stage, last_error,
		          closed_by_user, created_at, started_at, finished_at, closed_at, updated_at
	`, id, strings.TrimSpace(p.WorkflowID), p.RunID, strings.TrimSpace(p.LastStage), trimAIAgentQueueError(p.LastError))
	item, err := scanAIAgentWorkload(row)
	if err != nil {
		return nil, fmt.Errorf("mark ai agent workload failed: %w", err)
	}
	return item, nil
}

func (r *AIAgentWorkloadRepository) Restart(ctx context.Context, id uuid.UUID, maxAttempts int) (*models.AIAgentWorkload, error) {
	maxAttempts = normalizeAIAgentQueueMaxAttempts(maxAttempts)
	row := r.pool.QueryRow(ctx, `
		UPDATE ai_agent_workloads
		SET status = 'queued',
		    attempt_count = 0,
		    max_attempts = $2,
		    workflow_id = '',
		    run_id = NULL,
		    last_stage = '',
		    last_error = '',
		    started_at = NULL,
		    finished_at = NULL,
		    closed_by_user = false,
		    closed_at = NULL,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id, tenant_id, queue_event_id, agent_id, agent_name, execution_policy, execution_priority, execution_index, entity_type, entity_id,
		          status, attempt_count, max_attempts, workflow_id, run_id, last_stage, last_error,
		          closed_by_user, created_at, started_at, finished_at, closed_at, updated_at
	`, id, maxAttempts)
	item, err := scanAIAgentWorkload(row)
	if err != nil {
		return nil, fmt.Errorf("restart ai agent workload: %w", err)
	}
	return item, nil
}

func (r *AIAgentWorkloadRepository) CloseByUser(ctx context.Context, id uuid.UUID) (*models.AIAgentWorkload, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE ai_agent_workloads
		SET status = 'cancelled',
		    workflow_id = '',
		    finished_at = NOW(),
		    closed_by_user = true,
		    closed_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id, tenant_id, queue_event_id, agent_id, agent_name, execution_policy, execution_priority, execution_index, entity_type, entity_id,
		          status, attempt_count, max_attempts, workflow_id, run_id, last_stage, last_error,
		          closed_by_user, created_at, started_at, finished_at, closed_at, updated_at
	`, id)
	item, err := scanAIAgentWorkload(row)
	if err != nil {
		return nil, fmt.Errorf("close ai agent workload: %w", err)
	}
	return item, nil
}

func (r *AIAgentWorkloadRepository) Cancel(ctx context.Context, id uuid.UUID, lastStage string, lastError string) (*models.AIAgentWorkload, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE ai_agent_workloads
		SET status = 'cancelled',
		    workflow_id = '',
		    last_stage = COALESCE(NULLIF($2, ''), last_stage),
		    last_error = COALESCE(NULLIF($3, ''), last_error),
		    finished_at = NOW(),
		    closed_by_user = false,
		    closed_at = NULL,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id, tenant_id, queue_event_id, agent_id, agent_name, execution_policy, execution_priority, execution_index, entity_type, entity_id,
		          status, attempt_count, max_attempts, workflow_id, run_id, last_stage, last_error,
		          closed_by_user, created_at, started_at, finished_at, closed_at, updated_at
	`, id, strings.TrimSpace(lastStage), trimAIAgentQueueError(lastError))
	item, err := scanAIAgentWorkload(row)
	if err != nil {
		return nil, fmt.Errorf("cancel ai agent workload: %w", err)
	}
	return item, nil
}

func (r *AIAgentWorkloadRepository) RecoverStaleProcessing(ctx context.Context, staleBefore time.Time) (RecoverAIAgentQueueStaleResult, error) {
	rows, err := r.pool.Query(ctx, `
		UPDATE ai_agent_workloads
		SET status = CASE WHEN attempt_count >= max_attempts THEN 'failed' ELSE 'queued' END,
		    last_error = CASE
		        WHEN attempt_count >= max_attempts
		            THEN LEFT(CONCAT_WS('; ', NULLIF(last_error, ''), 'processing timeout exceeded')
		                , 3000)
		        ELSE LEFT(CONCAT_WS('; ', NULLIF(last_error, ''), 'processing timeout exceeded, automatically requeued')
		                , 3000)
		    END,
		    workflow_id = CASE WHEN attempt_count >= max_attempts THEN workflow_id ELSE '' END,
		    started_at = CASE WHEN attempt_count >= max_attempts THEN started_at ELSE NULL END,
		    finished_at = CASE WHEN attempt_count >= max_attempts THEN NOW() ELSE NULL END,
		    closed_by_user = false,
		    closed_at = NULL,
		    updated_at = NOW()
		WHERE status = 'processing'
		  AND COALESCE(started_at, updated_at) <= $1
		RETURNING status
	`, staleBefore.UTC())
	if err != nil {
		return RecoverAIAgentQueueStaleResult{}, fmt.Errorf("recover stale ai agent workloads: %w", err)
	}
	defer rows.Close()

	result := RecoverAIAgentQueueStaleResult{}
	for rows.Next() {
		var rawStatus string
		if err := rows.Scan(&rawStatus); err != nil {
			return RecoverAIAgentQueueStaleResult{}, fmt.Errorf("scan stale ai agent workload status: %w", err)
		}
		switch normalizeAIAgentWorkloadStatus(rawStatus) {
		case models.AIAgentWorkloadStatusFailed:
			result.Failed++
		default:
			result.Requeued++
		}
	}
	if err := rows.Err(); err != nil {
		return RecoverAIAgentQueueStaleResult{}, fmt.Errorf("iterate stale ai agent workload statuses: %w", err)
	}
	return result, nil
}

func scanAIAgentWorkload(scanner rowScanner) (*models.AIAgentWorkload, error) {
	var item models.AIAgentWorkload
	var status string
	if err := scanner.Scan(
		&item.ID,
		&item.TenantID,
		&item.QueueEventID,
		&item.AgentID,
		&item.AgentName,
		&item.ExecutionPolicy,
		&item.ExecutionPriority,
		&item.ExecutionIndex,
		&item.EntityType,
		&item.EntityID,
		&status,
		&item.AttemptCount,
		&item.MaxAttempts,
		&item.WorkflowID,
		&item.RunID,
		&item.LastStage,
		&item.LastError,
		&item.ClosedByUser,
		&item.CreatedAt,
		&item.StartedAt,
		&item.FinishedAt,
		&item.ClosedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.ExecutionPolicy = normalizeAIAgentExecutionPolicy(string(item.ExecutionPolicy))
	item.Status = normalizeAIAgentWorkloadStatus(status)
	return &item, nil
}
