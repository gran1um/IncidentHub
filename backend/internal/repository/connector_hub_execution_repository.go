package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"incidenthub/backend/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ConnectorHubExecutionRepository struct {
	pool *pgxpool.Pool
}

type ConnectorHubExecutionListParams struct {
	TenantID      uuid.UUID
	ConnectorID   *uuid.UUID
	CaseID        *uuid.UUID
	AlertID       *uuid.UUID
	Action        string
	Status        string
	ExecutionMode string
	Limit         int
}

type CreateConnectorHubExecutionParams struct {
	TenantID         uuid.UUID
	ActorID          *uuid.UUID
	ConnectorID      uuid.UUID
	ConnectorName    string
	ConnectorChannel string
	MethodID         *uuid.UUID
	MethodName       string
	MethodKey        string
	Action           string
	CaseID           *uuid.UUID
	AlertID          *uuid.UUID
	ExecutionMode    string
	DryRun           bool
	Request          map[string]any
	Metadata         map[string]any
	MaxAttempts      int
	IdempotencyKey   string
}

type AddConnectorHubExecutionEventParams struct {
	ExecutionID uuid.UUID
	AttemptNo   *int
	EventType   string
	Status      string
	Message     string
	Data        map[string]any
}

type CreateConnectorHubExecutionAttemptParams struct {
	ExecutionID uuid.UUID
	AttemptNo   int
	Status      string
	Request     map[string]any
	StartedAt   time.Time
}

type FinishConnectorHubExecutionAttemptParams struct {
	Status         string
	Response       map[string]any
	Error          string
	ProviderStatus string
	ExternalID     string
	CorrelationID  string
	FinishedAt     time.Time
}

type ConnectorHubRecoveredExecution struct {
	ID           uuid.UUID
	Status       models.ConnectorHubExecutionStatus
	AttemptCount int
}

func NewConnectorHubExecutionRepository(pool *pgxpool.Pool) *ConnectorHubExecutionRepository {
	return &ConnectorHubExecutionRepository{pool: pool}
}

func normalizeConnectorHubExecutionStatus(raw string) models.ConnectorHubExecutionStatus {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case string(models.ConnectorHubExecutionStatusAccepted):
		return models.ConnectorHubExecutionStatusAccepted
	case string(models.ConnectorHubExecutionStatusQueued):
		return models.ConnectorHubExecutionStatusQueued
	case string(models.ConnectorHubExecutionStatusDispatching):
		return models.ConnectorHubExecutionStatusDispatching
	case string(models.ConnectorHubExecutionStatusProviderAccepted):
		return models.ConnectorHubExecutionStatusProviderAccepted
	case string(models.ConnectorHubExecutionStatusRetryScheduled):
		return models.ConnectorHubExecutionStatusRetryScheduled
	case string(models.ConnectorHubExecutionStatusCompleted):
		return models.ConnectorHubExecutionStatusCompleted
	case string(models.ConnectorHubExecutionStatusDryRun):
		return models.ConnectorHubExecutionStatusDryRun
	case string(models.ConnectorHubExecutionStatusFailed):
		return models.ConnectorHubExecutionStatusFailed
	case string(models.ConnectorHubExecutionStatusCancelled):
		return models.ConnectorHubExecutionStatusCancelled
	case "cancelled":
		return models.ConnectorHubExecutionStatusCancelled
	case string(models.ConnectorHubExecutionStatusDeadLetter):
		return models.ConnectorHubExecutionStatusDeadLetter
	default:
		return ""
	}
}

func connectorHubDBStatus(status models.ConnectorHubExecutionStatus) string {
	if status == models.ConnectorHubExecutionStatusCancelled {
		return "cancelled"
	}
	return string(status)
}

func normalizeConnectorHubExecutionMode(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "ai_agent":
		return "ai_agent"
	case "automation":
		return "automation"
	case "system":
		return "system"
	default:
		return "manual"
	}
}

func normalizeConnectorHubMaxAttempts(value int) int {
	if value <= 0 {
		return 3
	}
	if value > 10 {
		return 10
	}
	return value
}

func normalizeConnectorHubJSONMap(input map[string]any) map[string]any {
	if input == nil {
		return map[string]any{}
	}
	return input
}

func marshalConnectorHubJSONMap(input map[string]any) ([]byte, error) {
	normalized := normalizeConnectorHubJSONMap(input)
	raw, err := json.Marshal(normalized)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func unmarshalConnectorHubJSONMap(raw []byte) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}

func (r *ConnectorHubExecutionRepository) Create(ctx context.Context, p CreateConnectorHubExecutionParams) (*models.ConnectorHubExecution, error) {
	requestRaw, err := marshalConnectorHubJSONMap(p.Request)
	if err != nil {
		return nil, fmt.Errorf("marshal connector hub execution request: %w", err)
	}
	metadataRaw, err := marshalConnectorHubJSONMap(p.Metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal connector hub execution metadata: %w", err)
	}
	row := r.pool.QueryRow(ctx, `
		INSERT INTO connector_hub_executions (
			tenant_id, actor_id, connector_id, connector_name, connector_channel,
			method_id, method_name, method_key, action, case_id, alert_id,
			execution_mode, dry_run, status, request, response, metadata, error,
			provider_status, external_id, correlation_id, idempotency_key, attempt_count, max_attempts,
			next_attempt_at, cancelled_by_user, cancelled_at, created_at, started_at, finished_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, 'accepted', $14::jsonb, '{}'::jsonb, $15::jsonb, '', '', '', '', $16, 0, $17, NOW(), false, NULL, NOW(), NULL, NULL, NOW())
		RETURNING
			id, tenant_id, actor_id, connector_id, connector_name, connector_channel,
			method_id, method_name, method_key, action, case_id, alert_id, execution_mode,
			dry_run, status, request, response, metadata, error, provider_status, external_id,
			correlation_id, idempotency_key, attempt_count, max_attempts, next_attempt_at, cancelled_by_user,
			cancelled_at, created_at, started_at, finished_at, updated_at
	`,
		p.TenantID,
		p.ActorID,
		p.ConnectorID,
		strings.TrimSpace(p.ConnectorName),
		strings.TrimSpace(p.ConnectorChannel),
		p.MethodID,
		strings.TrimSpace(p.MethodName),
		strings.TrimSpace(p.MethodKey),
		strings.TrimSpace(p.Action),
		p.CaseID,
		p.AlertID,
		normalizeConnectorHubExecutionMode(p.ExecutionMode),
		p.DryRun,
		requestRaw,
		metadataRaw,
		strings.TrimSpace(p.IdempotencyKey),
		normalizeConnectorHubMaxAttempts(p.MaxAttempts),
	)
	item, err := scanConnectorHubExecution(row)
	if err != nil {
		return nil, fmt.Errorf("create connector hub execution: %w", err)
	}
	return item, nil
}

func (r *ConnectorHubExecutionRepository) MarkQueued(ctx context.Context, id uuid.UUID) (*models.ConnectorHubExecution, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE connector_hub_executions
		SET status = 'queued',
		    next_attempt_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
		RETURNING
			id, tenant_id, actor_id, connector_id, connector_name, connector_channel,
			method_id, method_name, method_key, action, case_id, alert_id, execution_mode,
			dry_run, status, request, response, metadata, error, provider_status, external_id,
			correlation_id, idempotency_key, attempt_count, max_attempts, next_attempt_at, cancelled_by_user,
			cancelled_at, created_at, started_at, finished_at, updated_at
	`, id)
	item, err := scanConnectorHubExecution(row)
	if err != nil {
		return nil, fmt.Errorf("mark connector hub execution queued: %w", err)
	}
	return item, nil
}

func (r *ConnectorHubExecutionRepository) GetByTenantAndIdempotencyKey(ctx context.Context, tenantID uuid.UUID, key string) (*models.ConnectorHubExecution, error) {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return nil, pgx.ErrNoRows
	}
	row := r.pool.QueryRow(ctx, `
		SELECT
			id, tenant_id, actor_id, connector_id, connector_name, connector_channel,
			method_id, method_name, method_key, action, case_id, alert_id, execution_mode,
			dry_run, status, request, response, metadata, error, provider_status, external_id,
			correlation_id, idempotency_key, attempt_count, max_attempts, next_attempt_at, cancelled_by_user,
			cancelled_at, created_at, started_at, finished_at, updated_at
		FROM connector_hub_executions
		WHERE tenant_id = $1 AND idempotency_key = $2
		ORDER BY created_at DESC
		LIMIT 1
	`, tenantID, trimmed)
	item, err := scanConnectorHubExecution(row)
	if err != nil {
		return nil, fmt.Errorf("get connector hub execution by idempotency key: %w", err)
	}
	return item, nil
}

func (r *ConnectorHubExecutionRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.ConnectorHubExecution, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT
			id, tenant_id, actor_id, connector_id, connector_name, connector_channel,
			method_id, method_name, method_key, action, case_id, alert_id, execution_mode,
			dry_run, status, request, response, metadata, error, provider_status, external_id,
			correlation_id, idempotency_key, attempt_count, max_attempts, next_attempt_at, cancelled_by_user,
			cancelled_at, created_at, started_at, finished_at, updated_at
		FROM connector_hub_executions
		WHERE id = $1
	`, id)
	item, err := scanConnectorHubExecution(row)
	if err != nil {
		return nil, fmt.Errorf("get connector hub execution: %w", err)
	}
	return item, nil
}

func (r *ConnectorHubExecutionRepository) List(ctx context.Context, p ConnectorHubExecutionListParams) ([]models.ConnectorHubExecution, error) {
	limit := p.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	args := []any{p.TenantID}
	conditions := []string{"tenant_id = $1"}
	if p.ConnectorID != nil {
		args = append(args, *p.ConnectorID)
		conditions = append(conditions, fmt.Sprintf("connector_id = $%d", len(args)))
	}
	if p.CaseID != nil {
		args = append(args, *p.CaseID)
		conditions = append(conditions, fmt.Sprintf("case_id = $%d", len(args)))
	}
	if p.AlertID != nil {
		args = append(args, *p.AlertID)
		conditions = append(conditions, fmt.Sprintf("alert_id = $%d", len(args)))
	}
	if action := strings.TrimSpace(strings.ToLower(p.Action)); action != "" {
		args = append(args, action)
		conditions = append(conditions, fmt.Sprintf("LOWER(action) = $%d", len(args)))
	}
	if status := strings.TrimSpace(strings.ToLower(p.Status)); status != "" {
		args = append(args, status)
		conditions = append(conditions, fmt.Sprintf("LOWER(status) = $%d", len(args)))
	}
	if mode := strings.TrimSpace(strings.ToLower(p.ExecutionMode)); mode != "" {
		args = append(args, mode)
		conditions = append(conditions, fmt.Sprintf("LOWER(execution_mode) = $%d", len(args)))
	}
	args = append(args, limit)
	query := fmt.Sprintf(`
		SELECT
			id, tenant_id, actor_id, connector_id, connector_name, connector_channel,
			method_id, method_name, method_key, action, case_id, alert_id, execution_mode,
			dry_run, status, request, response, metadata, error, provider_status, external_id,
			correlation_id, idempotency_key, attempt_count, max_attempts, next_attempt_at, cancelled_by_user,
			cancelled_at, created_at, started_at, finished_at, updated_at
		FROM connector_hub_executions
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d
	`, strings.Join(conditions, " AND "), len(args))
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list connector hub executions: %w", err)
	}
	defer rows.Close()
	items := make([]models.ConnectorHubExecution, 0)
	for rows.Next() {
		item, scanErr := scanConnectorHubExecution(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan connector hub execution: %w", scanErr)
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate connector hub executions: %w", err)
	}
	return items, nil
}

func (r *ConnectorHubExecutionRepository) ListEvents(ctx context.Context, executionID uuid.UUID, limit int) ([]models.ConnectorHubExecutionEvent, error) {
	return listByExecutionID(
		ctx,
		r.pool,
		`
			SELECT id, execution_id, attempt_no, event_type, status, message, data, created_at
			FROM connector_hub_execution_events
			WHERE execution_id = $1
			ORDER BY created_at DESC
			LIMIT $2
		`,
		executionID,
		limit,
		500,
		100,
		scanConnectorHubExecutionEvent,
		"list connector hub execution events",
		"scan connector hub execution event",
		"iterate connector hub execution events",
	)
}

func (r *ConnectorHubExecutionRepository) ListAttempts(ctx context.Context, executionID uuid.UUID, limit int) ([]models.ConnectorHubExecutionAttempt, error) {
	return listByExecutionID(
		ctx,
		r.pool,
		`
			SELECT id, execution_id, attempt_no, status, request, response, error,
			       provider_status, external_id, correlation_id, created_at, started_at,
			       finished_at, duration_ms, updated_at
			FROM connector_hub_execution_attempts
			WHERE execution_id = $1
			ORDER BY attempt_no DESC, created_at DESC
			LIMIT $2
		`,
		executionID,
		limit,
		200,
		20,
		scanConnectorHubExecutionAttempt,
		"list connector hub execution attempts",
		"scan connector hub execution attempt",
		"iterate connector hub execution attempts",
	)
}

func (r *ConnectorHubExecutionRepository) ClaimReadyBatch(ctx context.Context, limit int) ([]models.ConnectorHubExecution, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.pool.Query(ctx, `
		WITH picked AS (
			SELECT id
			FROM connector_hub_executions
			WHERE status IN ('accepted', 'queued', 'retry_scheduled')
			  AND COALESCE(next_attempt_at, NOW()) <= NOW()
			ORDER BY COALESCE(next_attempt_at, created_at) ASC, created_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT $1
		)
		UPDATE connector_hub_executions AS e
		SET status = 'dispatching',
		    attempt_count = e.attempt_count + 1,
		    finished_at = NULL,
		    cancelled_by_user = false,
		    cancelled_at = NULL,
		    started_at = COALESCE(e.started_at, NOW()),
		    updated_at = NOW()
		FROM picked
		WHERE e.id = picked.id
		RETURNING
			e.id, e.tenant_id, e.actor_id, e.connector_id, e.connector_name, e.connector_channel,
			e.method_id, e.method_name, e.method_key, e.action, e.case_id, e.alert_id, e.execution_mode,
			e.dry_run, e.status, e.request, e.response, e.metadata, e.error, e.provider_status, e.external_id,
			e.correlation_id, e.idempotency_key, e.attempt_count, e.max_attempts, e.next_attempt_at, e.cancelled_by_user,
			e.cancelled_at, e.created_at, e.started_at, e.finished_at, e.updated_at
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("claim connector hub execution batch: %w", err)
	}
	defer rows.Close()
	items := make([]models.ConnectorHubExecution, 0)
	for rows.Next() {
		item, scanErr := scanConnectorHubExecution(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan claimed connector hub execution: %w", scanErr)
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate claimed connector hub executions: %w", err)
	}
	return items, nil
}

func (r *ConnectorHubExecutionRepository) ClaimReadyByID(ctx context.Context, executionID uuid.UUID) (*models.ConnectorHubExecution, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE connector_hub_executions
		SET status = 'dispatching',
		    attempt_count = attempt_count + 1,
		    finished_at = NULL,
		    cancelled_by_user = false,
		    cancelled_at = NULL,
		    started_at = COALESCE(started_at, NOW()),
		    updated_at = NOW()
		WHERE id = $1
		  AND status IN ('accepted', 'queued', 'retry_scheduled')
		  AND COALESCE(next_attempt_at, NOW()) <= NOW()
		RETURNING
			id, tenant_id, actor_id, connector_id, connector_name, connector_channel,
			method_id, method_name, method_key, action, case_id, alert_id, execution_mode,
			dry_run, status, request, response, metadata, error, provider_status, external_id,
			correlation_id, idempotency_key, attempt_count, max_attempts, next_attempt_at, cancelled_by_user,
			cancelled_at, created_at, started_at, finished_at, updated_at
	`, executionID)
	item, err := scanConnectorHubExecution(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("claim connector hub execution by id: %w", err)
	}
	return item, nil
}

func (r *ConnectorHubExecutionRepository) AddEvent(ctx context.Context, p AddConnectorHubExecutionEventParams) (*models.ConnectorHubExecutionEvent, error) {
	dataRaw, err := marshalConnectorHubJSONMap(p.Data)
	if err != nil {
		return nil, fmt.Errorf("marshal connector hub execution event data: %w", err)
	}
	status := normalizeConnectorHubExecutionStatus(p.Status)
	if status == "" {
		status = models.ConnectorHubExecutionStatusAccepted
	}
	row := r.pool.QueryRow(ctx, `
		INSERT INTO connector_hub_execution_events (execution_id, attempt_no, event_type, status, message, data, created_at)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, NOW())
		RETURNING id, execution_id, attempt_no, event_type, status, message, data, created_at
	`, p.ExecutionID, p.AttemptNo, strings.TrimSpace(strings.ToLower(p.EventType)), connectorHubDBStatus(status), strings.TrimSpace(p.Message), dataRaw)
	item, err := scanConnectorHubExecutionEvent(row)
	if err != nil {
		return nil, fmt.Errorf("add connector hub execution event: %w", err)
	}
	return item, nil
}

func (r *ConnectorHubExecutionRepository) CreateAttempt(ctx context.Context, p CreateConnectorHubExecutionAttemptParams) (*models.ConnectorHubExecutionAttempt, error) {
	requestRaw, err := marshalConnectorHubJSONMap(p.Request)
	if err != nil {
		return nil, fmt.Errorf("marshal connector hub execution attempt request: %w", err)
	}
	status := normalizeConnectorHubExecutionStatus(p.Status)
	if status == "" {
		status = models.ConnectorHubExecutionStatusDispatching
	}
	startedAt := p.StartedAt.UTC()
	row := r.pool.QueryRow(ctx, `
		INSERT INTO connector_hub_execution_attempts (
			execution_id, attempt_no, status, request, response, error,
			provider_status, external_id, correlation_id, created_at, started_at, finished_at, duration_ms, updated_at
		)
		VALUES ($1, $2, $3, $4::jsonb, '{}'::jsonb, '', '', '', '', NOW(), $5, NULL, 0, NOW())
		RETURNING id, execution_id, attempt_no, status, request, response, error,
		       provider_status, external_id, correlation_id, created_at, started_at,
		       finished_at, duration_ms, updated_at
	`, p.ExecutionID, p.AttemptNo, connectorHubDBStatus(status), requestRaw, startedAt)
	item, err := scanConnectorHubExecutionAttempt(row)
	if err != nil {
		return nil, fmt.Errorf("create connector hub execution attempt: %w", err)
	}
	return item, nil
}

func (r *ConnectorHubExecutionRepository) FinishAttempt(ctx context.Context, attemptID uuid.UUID, p FinishConnectorHubExecutionAttemptParams) (*models.ConnectorHubExecutionAttempt, error) {
	responseRaw, err := marshalConnectorHubJSONMap(p.Response)
	if err != nil {
		return nil, fmt.Errorf("marshal connector hub execution attempt response: %w", err)
	}
	status := normalizeConnectorHubExecutionStatus(p.Status)
	if status == "" {
		status = models.ConnectorHubExecutionStatusFailed
	}
	finishedAt := p.FinishedAt.UTC()
	row := r.pool.QueryRow(ctx, `
		UPDATE connector_hub_execution_attempts
		SET status = $2,
		    response = $3::jsonb,
		    error = $4,
		    provider_status = $5,
		    external_id = $6,
		    correlation_id = $7,
		    finished_at = $8,
		    duration_ms = GREATEST(0, FLOOR(EXTRACT(EPOCH FROM ($8 - started_at)) * 1000)::BIGINT),
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id, execution_id, attempt_no, status, request, response, error,
		       provider_status, external_id, correlation_id, created_at, started_at,
		       finished_at, duration_ms, updated_at
	`,
		attemptID,
		connectorHubDBStatus(status),
		responseRaw,
		strings.TrimSpace(p.Error),
		strings.TrimSpace(p.ProviderStatus),
		strings.TrimSpace(p.ExternalID),
		strings.TrimSpace(p.CorrelationID),
		finishedAt,
	)
	item, err := scanConnectorHubExecutionAttempt(row)
	if err != nil {
		return nil, fmt.Errorf("finish connector hub execution attempt: %w", err)
	}
	return item, nil
}

func (r *ConnectorHubExecutionRepository) MarkProviderAccepted(
	ctx context.Context,
	executionID uuid.UUID,
	response map[string]any,
	providerStatus string,
	externalID string,
	correlationID string,
) (*models.ConnectorHubExecution, error) {
	responseRaw, err := marshalConnectorHubJSONMap(response)
	if err != nil {
		return nil, fmt.Errorf("marshal connector hub provider accepted response: %w", err)
	}
	row := r.pool.QueryRow(ctx, `
		UPDATE connector_hub_executions
		SET status = 'provider_accepted',
		    response = $2::jsonb,
		    provider_status = $3,
		    external_id = $4,
		    correlation_id = $5,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING
			id, tenant_id, actor_id, connector_id, connector_name, connector_channel,
			method_id, method_name, method_key, action, case_id, alert_id, execution_mode,
			dry_run, status, request, response, metadata, error, provider_status, external_id,
			correlation_id, idempotency_key, attempt_count, max_attempts, next_attempt_at, cancelled_by_user,
			cancelled_at, created_at, started_at, finished_at, updated_at
	`,
		executionID,
		responseRaw,
		strings.TrimSpace(providerStatus),
		strings.TrimSpace(externalID),
		strings.TrimSpace(correlationID),
	)
	item, err := scanConnectorHubExecution(row)
	if err != nil {
		return nil, fmt.Errorf("mark connector hub execution provider accepted: %w", err)
	}
	return item, nil
}

func (r *ConnectorHubExecutionRepository) MarkCompleted(
	ctx context.Context,
	executionID uuid.UUID,
	status models.ConnectorHubExecutionStatus,
	response map[string]any,
	providerStatus string,
	externalID string,
	correlationID string,
	finishedAt time.Time,
) (*models.ConnectorHubExecution, error) {
	responseRaw, err := marshalConnectorHubJSONMap(response)
	if err != nil {
		return nil, fmt.Errorf("marshal connector hub completed response: %w", err)
	}
	finalStatus := normalizeConnectorHubExecutionStatus(string(status))
	if finalStatus == "" {
		finalStatus = models.ConnectorHubExecutionStatusCompleted
	}
	row := r.pool.QueryRow(ctx, `
		UPDATE connector_hub_executions
		SET status = $2,
		    response = $3::jsonb,
		    error = '',
		    provider_status = $4,
		    external_id = $5,
		    correlation_id = $6,
		    next_attempt_at = NULL,
		    finished_at = $7,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING
			id, tenant_id, actor_id, connector_id, connector_name, connector_channel,
			method_id, method_name, method_key, action, case_id, alert_id, execution_mode,
			dry_run, status, request, response, metadata, error, provider_status, external_id,
			correlation_id, idempotency_key, attempt_count, max_attempts, next_attempt_at, cancelled_by_user,
			cancelled_at, created_at, started_at, finished_at, updated_at
	`,
		executionID,
		connectorHubDBStatus(finalStatus),
		responseRaw,
		strings.TrimSpace(providerStatus),
		strings.TrimSpace(externalID),
		strings.TrimSpace(correlationID),
		finishedAt.UTC(),
	)
	item, err := scanConnectorHubExecution(row)
	if err != nil {
		return nil, fmt.Errorf("mark connector hub execution completed: %w", err)
	}
	return item, nil
}

func (r *ConnectorHubExecutionRepository) ScheduleRetry(
	ctx context.Context,
	executionID uuid.UUID,
	nextAttemptAt time.Time,
	lastError string,
	providerStatus string,
	externalID string,
	correlationID string,
) (*models.ConnectorHubExecution, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE connector_hub_executions
		SET status = 'retry_scheduled',
		    error = $2,
		    provider_status = $3,
		    external_id = $4,
		    correlation_id = $5,
		    next_attempt_at = $6,
		    finished_at = NULL,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING
			id, tenant_id, actor_id, connector_id, connector_name, connector_channel,
			method_id, method_name, method_key, action, case_id, alert_id, execution_mode,
			dry_run, status, request, response, metadata, error, provider_status, external_id,
			correlation_id, idempotency_key, attempt_count, max_attempts, next_attempt_at, cancelled_by_user,
			cancelled_at, created_at, started_at, finished_at, updated_at
	`,
		executionID,
		strings.TrimSpace(lastError),
		strings.TrimSpace(providerStatus),
		strings.TrimSpace(externalID),
		strings.TrimSpace(correlationID),
		nextAttemptAt.UTC(),
	)
	item, err := scanConnectorHubExecution(row)
	if err != nil {
		return nil, fmt.Errorf("schedule connector hub execution retry: %w", err)
	}
	return item, nil
}

func (r *ConnectorHubExecutionRepository) MarkDeadLetter(
	ctx context.Context,
	executionID uuid.UUID,
	lastError string,
	providerStatus string,
	externalID string,
	correlationID string,
	finishedAt time.Time,
) (*models.ConnectorHubExecution, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE connector_hub_executions
		SET status = 'dead_letter',
		    error = $2,
		    provider_status = $3,
		    external_id = $4,
		    correlation_id = $5,
		    next_attempt_at = NULL,
		    finished_at = $6,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING
			id, tenant_id, actor_id, connector_id, connector_name, connector_channel,
			method_id, method_name, method_key, action, case_id, alert_id, execution_mode,
			dry_run, status, request, response, metadata, error, provider_status, external_id,
			correlation_id, idempotency_key, attempt_count, max_attempts, next_attempt_at, cancelled_by_user,
			cancelled_at, created_at, started_at, finished_at, updated_at
	`,
		executionID,
		strings.TrimSpace(lastError),
		strings.TrimSpace(providerStatus),
		strings.TrimSpace(externalID),
		strings.TrimSpace(correlationID),
		finishedAt.UTC(),
	)
	item, err := scanConnectorHubExecution(row)
	if err != nil {
		return nil, fmt.Errorf("mark connector hub execution dead letter: %w", err)
	}
	return item, nil
}

func (r *ConnectorHubExecutionRepository) Cancel(ctx context.Context, executionID uuid.UUID) (*models.ConnectorHubExecution, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE connector_hub_executions
		SET status = 'cancelled',
		    next_attempt_at = NULL,
		    cancelled_by_user = true,
		    cancelled_at = NOW(),
		    finished_at = COALESCE(finished_at, NOW()),
		    updated_at = NOW()
		WHERE id = $1
		RETURNING
			id, tenant_id, actor_id, connector_id, connector_name, connector_channel,
			method_id, method_name, method_key, action, case_id, alert_id, execution_mode,
			dry_run, status, request, response, metadata, error, provider_status, external_id,
			correlation_id, idempotency_key, attempt_count, max_attempts, next_attempt_at, cancelled_by_user,
			cancelled_at, created_at, started_at, finished_at, updated_at
	`, executionID)
	item, err := scanConnectorHubExecution(row)
	if err != nil {
		return nil, fmt.Errorf("cancel connector hub execution: %w", err)
	}
	return item, nil
}

func (r *ConnectorHubExecutionRepository) Retry(ctx context.Context, executionID uuid.UUID) (*models.ConnectorHubExecution, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE connector_hub_executions
		SET status = 'queued',
		    error = '',
		    next_attempt_at = NOW(),
		    finished_at = NULL,
		    cancelled_by_user = false,
		    cancelled_at = NULL,
		    max_attempts = GREATEST(max_attempts, attempt_count + 1),
		    updated_at = NOW()
		WHERE id = $1
		RETURNING
			id, tenant_id, actor_id, connector_id, connector_name, connector_channel,
			method_id, method_name, method_key, action, case_id, alert_id, execution_mode,
			dry_run, status, request, response, metadata, error, provider_status, external_id,
			correlation_id, idempotency_key, attempt_count, max_attempts, next_attempt_at, cancelled_by_user,
			cancelled_at, created_at, started_at, finished_at, updated_at
	`, executionID)
	item, err := scanConnectorHubExecution(row)
	if err != nil {
		return nil, fmt.Errorf("retry connector hub execution: %w", err)
	}
	return item, nil
}

func (r *ConnectorHubExecutionRepository) RecoverStaleDispatching(ctx context.Context, staleBefore time.Time, retryAt time.Time) ([]ConnectorHubRecoveredExecution, error) {
	rows, err := r.pool.Query(ctx, `
		UPDATE connector_hub_executions
		SET status = CASE WHEN attempt_count >= max_attempts THEN 'dead_letter' ELSE 'retry_scheduled' END,
		    error = CASE
		        WHEN attempt_count >= max_attempts
		            THEN LEFT(CONCAT_WS('; ', NULLIF(error, ''), 'dispatch timeout exceeded'), 3000)
		        ELSE LEFT(CONCAT_WS('; ', NULLIF(error, ''), 'dispatch timeout exceeded, automatically requeued'), 3000)
		    END,
		    next_attempt_at = CASE
		        WHEN attempt_count >= max_attempts THEN NULL::timestamptz
		        ELSE $2::timestamptz
		    END,
		    finished_at = CASE WHEN attempt_count >= max_attempts THEN NOW() ELSE NULL END,
		    updated_at = NOW()
		WHERE status = 'dispatching'
		  AND COALESCE(updated_at, started_at, created_at) <= $1
		RETURNING id, status, attempt_count
	`, staleBefore.UTC(), retryAt.UTC())
	if err != nil {
		return nil, fmt.Errorf("recover stale connector hub executions: %w", err)
	}
	defer rows.Close()
	items := make([]ConnectorHubRecoveredExecution, 0)
	for rows.Next() {
		var item ConnectorHubRecoveredExecution
		var status string
		if scanErr := rows.Scan(&item.ID, &status, &item.AttemptCount); scanErr != nil {
			return nil, fmt.Errorf("scan recovered connector hub execution: %w", scanErr)
		}
		item.Status = normalizeConnectorHubExecutionStatus(status)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recovered connector hub executions: %w", err)
	}
	return items, nil
}

func scanConnectorHubExecution(scanner rowScanner) (*models.ConnectorHubExecution, error) {
	var item models.ConnectorHubExecution
	var status string
	var requestRaw []byte
	var responseRaw []byte
	var metadataRaw []byte
	if err := scanner.Scan(
		&item.ID,
		&item.TenantID,
		&item.ActorID,
		&item.ConnectorID,
		&item.ConnectorName,
		&item.ConnectorChannel,
		&item.MethodID,
		&item.MethodName,
		&item.MethodKey,
		&item.Action,
		&item.CaseID,
		&item.AlertID,
		&item.ExecutionMode,
		&item.DryRun,
		&status,
		&requestRaw,
		&responseRaw,
		&metadataRaw,
		&item.Error,
		&item.ProviderStatus,
		&item.ExternalID,
		&item.CorrelationID,
		&item.IdempotencyKey,
		&item.AttemptCount,
		&item.MaxAttempts,
		&item.NextAttemptAt,
		&item.CancelledByUser,
		&item.CancelledAt,
		&item.CreatedAt,
		&item.StartedAt,
		&item.FinishedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.Status = normalizeConnectorHubExecutionStatus(status)
	var err error
	item.Request, err = unmarshalConnectorHubJSONMap(requestRaw)
	if err != nil {
		return nil, err
	}
	item.Response, err = unmarshalConnectorHubJSONMap(responseRaw)
	if err != nil {
		return nil, err
	}
	item.Metadata, err = unmarshalConnectorHubJSONMap(metadataRaw)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func scanConnectorHubExecutionAttempt(scanner rowScanner) (*models.ConnectorHubExecutionAttempt, error) {
	var item models.ConnectorHubExecutionAttempt
	var status string
	var requestRaw []byte
	var responseRaw []byte
	if err := scanner.Scan(
		&item.ID,
		&item.ExecutionID,
		&item.AttemptNo,
		&status,
		&requestRaw,
		&responseRaw,
		&item.Error,
		&item.ProviderStatus,
		&item.ExternalID,
		&item.CorrelationID,
		&item.CreatedAt,
		&item.StartedAt,
		&item.FinishedAt,
		&item.DurationMS,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.Status = normalizeConnectorHubExecutionStatus(status)
	var err error
	item.Request, err = unmarshalConnectorHubJSONMap(requestRaw)
	if err != nil {
		return nil, err
	}
	item.Response, err = unmarshalConnectorHubJSONMap(responseRaw)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func scanConnectorHubExecutionEvent(scanner rowScanner) (*models.ConnectorHubExecutionEvent, error) {
	var item models.ConnectorHubExecutionEvent
	var status string
	var dataRaw []byte
	if err := scanner.Scan(
		&item.ID,
		&item.ExecutionID,
		&item.AttemptNo,
		&item.EventType,
		&status,
		&item.Message,
		&dataRaw,
		&item.CreatedAt,
	); err != nil {
		return nil, err
	}
	item.Status = normalizeConnectorHubExecutionStatus(status)
	var err error
	item.Data, err = unmarshalConnectorHubJSONMap(dataRaw)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

var _ = pgx.ErrNoRows
