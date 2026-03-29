package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"incidenthub/backend/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AIAgentQueueRepository struct {
	pool *pgxpool.Pool
}

type EnqueueAIAgentQueueEventParams struct {
	TenantID    uuid.UUID
	ActorID     *uuid.UUID
	EntityType  string
	EntityID    uuid.UUID
	Source      string
	MaxAttempts int
}

type CompleteAIAgentQueueEventParams struct {
	MatchedAgents   int
	ProcessedAgents int
	LastError       string
}

const defaultAIAgentQueueMaxAttempts = 4

func NewAIAgentQueueRepository(pool *pgxpool.Pool) *AIAgentQueueRepository {
	return &AIAgentQueueRepository{pool: pool}
}

func normalizeAIAgentQueueEntityType(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "case":
		return "case"
	case "alert":
		return "alert"
	default:
		return ""
	}
}

func normalizeAIAgentQueueSource(raw string) string {
	source := strings.TrimSpace(strings.ToLower(raw))
	if source == "" {
		return "api"
	}
	if len(source) > 64 {
		source = source[:64]
	}
	return source
}

func normalizeAIAgentQueueMaxAttempts(raw int) int {
	maxAttempts := raw
	if maxAttempts <= 0 {
		maxAttempts = defaultAIAgentQueueMaxAttempts
	}
	if maxAttempts > 50 {
		maxAttempts = 50
	}
	return maxAttempts
}

func (r *AIAgentQueueRepository) Enqueue(ctx context.Context, p EnqueueAIAgentQueueEventParams) (*models.AIAgentQueueEvent, bool, error) {
	entityType := normalizeAIAgentQueueEntityType(p.EntityType)
	if entityType == "" {
		return nil, false, fmt.Errorf("invalid ai agent queue entity_type: %q", p.EntityType)
	}
	source := normalizeAIAgentQueueSource(p.Source)
	maxAttempts := normalizeAIAgentQueueMaxAttempts(p.MaxAttempts)

	var event models.AIAgentQueueEvent
	err := r.pool.QueryRow(ctx, `
		INSERT INTO ai_agent_queue_events (
			tenant_id,
			actor_id,
			entity_type,
			entity_id,
			source,
			status,
			last_error,
			matched_agents,
			processed_agents,
			attempt_count,
			max_attempts,
			workflow_id,
			started_at,
			finished_at,
			closed_by_user,
			closed_at,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, 'queued', '', 0, 0, 0, $6, '', NULL, NULL, false, NULL, NOW(), NOW())
		ON CONFLICT (tenant_id, entity_type, entity_id)
		DO UPDATE
		   SET actor_id = COALESCE(EXCLUDED.actor_id, ai_agent_queue_events.actor_id),
		       source = EXCLUDED.source,
		       status = 'queued',
		       last_error = '',
		       matched_agents = 0,
		       processed_agents = 0,
		       attempt_count = 0,
		       max_attempts = EXCLUDED.max_attempts,
		       workflow_id = '',
		       started_at = NULL,
		       finished_at = NULL,
		       closed_by_user = false,
		       closed_at = NULL,
		       updated_at = NOW()
		RETURNING
			id, tenant_id, actor_id, entity_type, entity_id, source, status,
			matched_agents, processed_agents, attempt_count, max_attempts,
			workflow_id, last_error, closed_by_user,
			created_at, started_at, finished_at, closed_at, updated_at
	`, p.TenantID, p.ActorID, entityType, p.EntityID, source, maxAttempts).Scan(
		&event.ID,
		&event.TenantID,
		&event.ActorID,
		&event.EntityType,
		&event.EntityID,
		&event.Source,
		&event.Status,
		&event.MatchedAgents,
		&event.ProcessedAgents,
		&event.AttemptCount,
		&event.MaxAttempts,
		&event.WorkflowID,
		&event.LastError,
		&event.ClosedByUser,
		&event.CreatedAt,
		&event.StartedAt,
		&event.FinishedAt,
		&event.ClosedAt,
		&event.UpdatedAt,
	)
	if err != nil {
		return nil, false, fmt.Errorf("enqueue ai agent queue event: %w", err)
	}
	return &event, true, nil
}

func (r *AIAgentQueueRepository) ListByStatus(ctx context.Context, status models.AIAgentQueueStatus, limit int) ([]models.AIAgentQueueEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, actor_id, entity_type, entity_id, source, status,
		       matched_agents, processed_agents, attempt_count, max_attempts,
		       workflow_id, last_error, closed_by_user,
		       created_at, started_at, finished_at, closed_at, updated_at
		FROM ai_agent_queue_events
		WHERE status = $1
		ORDER BY created_at ASC
		LIMIT $2
	`, string(status), limit)
	if err != nil {
		return nil, fmt.Errorf("list ai agent queue events by status: %w", err)
	}
	defer rows.Close()

	events := make([]models.AIAgentQueueEvent, 0, limit)
	for rows.Next() {
		item, scanErr := scanAIAgentQueueEvent(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		events = append(events, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ai agent queue events: %w", err)
	}
	return events, nil
}

func (r *AIAgentQueueRepository) ListByTenantAndStatus(
	ctx context.Context,
	tenantID uuid.UUID,
	status models.AIAgentQueueStatus,
	limit int,
	offset int,
) ([]models.AIAgentQueueEvent, error) {
	return listByTenantAndStatus(
		ctx,
		r.pool,
		`
			SELECT id, tenant_id, actor_id, entity_type, entity_id, source, status,
			       matched_agents, processed_agents, attempt_count, max_attempts,
			       workflow_id, last_error, closed_by_user,
			       created_at, started_at, finished_at, closed_at, updated_at
			FROM ai_agent_queue_events
			WHERE tenant_id = $1
			  AND status = $2
			ORDER BY created_at ASC
			LIMIT $3 OFFSET $4
		`,
		tenantID,
		string(status),
		limit,
		offset,
		scanAIAgentQueueEvent,
		"list ai agent queue events by tenant and status",
		"scan ai agent queue event",
		"iterate ai agent queue events by tenant and status",
	)
}

func (r *AIAgentQueueRepository) CountByTenantAndStatus(
	ctx context.Context,
	tenantID uuid.UUID,
) (map[models.AIAgentQueueStatus]int, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT status, COUNT(*)
		FROM ai_agent_queue_events
		WHERE tenant_id = $1
		GROUP BY status
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("count ai agent queue events by tenant and status: %w", err)
	}
	defer rows.Close()

	out := map[models.AIAgentQueueStatus]int{
		models.AIAgentQueueStatusQueued:     0,
		models.AIAgentQueueStatusProcessing: 0,
		models.AIAgentQueueStatusDone:       0,
		models.AIAgentQueueStatusFailed:     0,
	}
	for rows.Next() {
		var rawStatus string
		var count int
		if err := rows.Scan(&rawStatus, &count); err != nil {
			return nil, fmt.Errorf("scan ai agent queue count row: %w", err)
		}
		status := models.AIAgentQueueStatus(strings.TrimSpace(strings.ToLower(rawStatus)))
		out[status] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ai agent queue count rows: %w", err)
	}
	return out, nil
}

func (r *AIAgentQueueRepository) ListByEntity(
	ctx context.Context,
	tenantID uuid.UUID,
	entityType string,
	entityID uuid.UUID,
	limit int,
) ([]models.AIAgentQueueEvent, error) {
	normalizedEntityType := normalizeAIAgentQueueEntityType(entityType)
	if normalizedEntityType == "" {
		return nil, fmt.Errorf("invalid ai agent queue entity_type: %q", entityType)
	}
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, actor_id, entity_type, entity_id, source, status,
		       matched_agents, processed_agents, attempt_count, max_attempts,
		       workflow_id, last_error, closed_by_user,
		       created_at, started_at, finished_at, closed_at, updated_at
		FROM ai_agent_queue_events
		WHERE tenant_id = $1
		  AND entity_type = $2
		  AND entity_id = $3
		ORDER BY updated_at DESC, created_at DESC
		LIMIT $4
	`, tenantID, normalizedEntityType, entityID, limit)
	if err != nil {
		return nil, fmt.Errorf("list ai agent queue events by entity: %w", err)
	}
	defer rows.Close()

	events := make([]models.AIAgentQueueEvent, 0, limit)
	for rows.Next() {
		item, scanErr := scanAIAgentQueueEvent(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		events = append(events, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ai agent queue events by entity: %w", err)
	}
	return events, nil
}

func (r *AIAgentQueueRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.AIAgentQueueEvent, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, actor_id, entity_type, entity_id, source, status,
		       matched_agents, processed_agents, attempt_count, max_attempts,
		       workflow_id, last_error, closed_by_user,
		       created_at, started_at, finished_at, closed_at, updated_at
		FROM ai_agent_queue_events
		WHERE id = $1
	`, id)
	item, err := scanAIAgentQueueEvent(row)
	if err != nil {
		return nil, fmt.Errorf("get ai agent queue event: %w", err)
	}
	return item, nil
}

func (r *AIAgentQueueRepository) MarkProcessing(ctx context.Context, id uuid.UUID, workflowID string) error {
	workflowID = strings.TrimSpace(workflowID)
	tag, err := r.pool.Exec(ctx, `
		UPDATE ai_agent_queue_events
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
	`, id, workflowID)
	if err != nil {
		return fmt.Errorf("mark ai agent queue event processing: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *AIAgentQueueRepository) MarkDone(ctx context.Context, id uuid.UUID, p CompleteAIAgentQueueEventParams) error {
	if p.MatchedAgents < 0 {
		p.MatchedAgents = 0
	}
	if p.ProcessedAgents < 0 {
		p.ProcessedAgents = 0
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE ai_agent_queue_events
		SET status = 'done',
		    matched_agents = $2,
		    processed_agents = $3,
		    last_error = '',
		    closed_by_user = false,
		    closed_at = NULL,
		    finished_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
	`, id, p.MatchedAgents, p.ProcessedAgents)
	if err != nil {
		return fmt.Errorf("mark ai agent queue event done: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

type aiAgentQueueCompletionUpdate struct {
	MatchedAgents   int
	ProcessedAgents int
	LastError       string
}

func normalizeAIAgentQueueCompletionUpdate(p CompleteAIAgentQueueEventParams) aiAgentQueueCompletionUpdate {
	matched := p.MatchedAgents
	if matched < 0 {
		matched = 0
	}
	processed := p.ProcessedAgents
	if processed < 0 {
		processed = 0
	}
	return aiAgentQueueCompletionUpdate{
		MatchedAgents:   matched,
		ProcessedAgents: processed,
		LastError:       trimAIAgentQueueError(p.LastError),
	}
}

func (r *AIAgentQueueRepository) updateCompletionWithStatus(
	ctx context.Context,
	id uuid.UUID,
	status models.AIAgentQueueStatus,
	update aiAgentQueueCompletionUpdate,
	resetWorkflow bool,
	resetStartedAt bool,
	setFinishedAtNow bool,
) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE ai_agent_queue_events
		SET status = $2,
		    matched_agents = $3,
		    processed_agents = $4,
		    last_error = $5,
		    workflow_id = CASE WHEN $6 THEN '' ELSE workflow_id END,
		    started_at = CASE WHEN $7 THEN NULL ELSE started_at END,
		    finished_at = CASE WHEN $8 THEN NOW() ELSE NULL END,
		    closed_by_user = false,
		    closed_at = NULL,
		    updated_at = NOW()
		WHERE id = $1
	`, id, string(status), update.MatchedAgents, update.ProcessedAgents, update.LastError, resetWorkflow, resetStartedAt, setFinishedAtNow)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *AIAgentQueueRepository) MarkFailed(ctx context.Context, id uuid.UUID, p CompleteAIAgentQueueEventParams) error {
	update := normalizeAIAgentQueueCompletionUpdate(p)
	if err := r.updateCompletionWithStatus(ctx, id, models.AIAgentQueueStatusFailed, update, false, false, true); err != nil {
		return fmt.Errorf("mark ai agent queue event failed: %w", err)
	}
	return nil
}

func (r *AIAgentQueueRepository) RequeueAfterFailure(ctx context.Context, id uuid.UUID, p CompleteAIAgentQueueEventParams) error {
	update := normalizeAIAgentQueueCompletionUpdate(p)
	if err := r.updateCompletionWithStatus(ctx, id, models.AIAgentQueueStatusQueued, update, true, true, false); err != nil {
		return fmt.Errorf("requeue ai agent queue event after failure: %w", err)
	}
	return nil
}

func (r *AIAgentQueueRepository) Restart(ctx context.Context, id uuid.UUID, maxAttempts int) (*models.AIAgentQueueEvent, error) {
	maxAttempts = normalizeAIAgentQueueMaxAttempts(maxAttempts)
	row := r.pool.QueryRow(ctx, `
		UPDATE ai_agent_queue_events
		SET status = 'queued',
		    matched_agents = 0,
		    processed_agents = 0,
		    attempt_count = 0,
		    max_attempts = $2,
		    workflow_id = '',
		    last_error = '',
		    started_at = NULL,
		    finished_at = NULL,
		    closed_by_user = false,
		    closed_at = NULL,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id, tenant_id, actor_id, entity_type, entity_id, source, status,
		          matched_agents, processed_agents, attempt_count, max_attempts,
		          workflow_id, last_error, closed_by_user,
		          created_at, started_at, finished_at, closed_at, updated_at
	`, id, maxAttempts)
	item, err := scanAIAgentQueueEvent(row)
	if err != nil {
		return nil, fmt.Errorf("restart ai agent queue event: %w", err)
	}
	return item, nil
}

func (r *AIAgentQueueRepository) CloseByUser(ctx context.Context, id uuid.UUID) (*models.AIAgentQueueEvent, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE ai_agent_queue_events
		SET status = 'done',
		    workflow_id = '',
		    last_error = '',
		    finished_at = NOW(),
		    closed_by_user = true,
		    closed_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id, tenant_id, actor_id, entity_type, entity_id, source, status,
		          matched_agents, processed_agents, attempt_count, max_attempts,
		          workflow_id, last_error, closed_by_user,
		          created_at, started_at, finished_at, closed_at, updated_at
	`, id)
	item, err := scanAIAgentQueueEvent(row)
	if err != nil {
		return nil, fmt.Errorf("close ai agent queue event: %w", err)
	}
	return item, nil
}

type RecoverAIAgentQueueStaleResult struct {
	Requeued int
	Failed   int
}

func (r *AIAgentQueueRepository) RecoverStaleProcessing(ctx context.Context, staleBefore time.Time) (RecoverAIAgentQueueStaleResult, error) {
	rows, err := r.pool.Query(ctx, `
		UPDATE ai_agent_queue_events
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
		return RecoverAIAgentQueueStaleResult{}, fmt.Errorf("recover stale ai agent queue events: %w", err)
	}
	defer rows.Close()

	result := RecoverAIAgentQueueStaleResult{}
	for rows.Next() {
		var rawStatus string
		if err := rows.Scan(&rawStatus); err != nil {
			return RecoverAIAgentQueueStaleResult{}, fmt.Errorf("scan stale ai agent queue status: %w", err)
		}
		switch strings.ToLower(strings.TrimSpace(rawStatus)) {
		case string(models.AIAgentQueueStatusFailed):
			result.Failed++
		default:
			result.Requeued++
		}
	}
	if err := rows.Err(); err != nil {
		return RecoverAIAgentQueueStaleResult{}, fmt.Errorf("iterate stale ai agent queue statuses: %w", err)
	}
	return result, nil
}

func trimAIAgentQueueError(raw string) string {
	lastError := strings.TrimSpace(raw)
	if len(lastError) > 3000 {
		lastError = lastError[:3000]
	}
	return lastError
}

func scanAIAgentQueueEvent(scanner rowScanner) (*models.AIAgentQueueEvent, error) {
	var item models.AIAgentQueueEvent
	var status string
	if err := scanner.Scan(
		&item.ID,
		&item.TenantID,
		&item.ActorID,
		&item.EntityType,
		&item.EntityID,
		&item.Source,
		&status,
		&item.MatchedAgents,
		&item.ProcessedAgents,
		&item.AttemptCount,
		&item.MaxAttempts,
		&item.WorkflowID,
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
	item.Status = models.AIAgentQueueStatus(strings.TrimSpace(strings.ToLower(status)))
	return &item, nil
}
