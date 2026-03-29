package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"incidenthub/backend/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AsyncOperationRepository struct {
	pool *pgxpool.Pool
}

type CreateAsyncOperationParams struct {
	OperationID   uuid.UUID
	TenantID      uuid.UUID
	ActorID       uuid.UUID
	Resource      string
	ResourceID    *uuid.UUID
	OperationType string
	Status        models.AsyncOperationStatus
	MaxAttempts   int
	Payload       json.RawMessage
}

func NewAsyncOperationRepository(pool *pgxpool.Pool) *AsyncOperationRepository {
	return &AsyncOperationRepository{pool: pool}
}

func (r *AsyncOperationRepository) Create(ctx context.Context, p CreateAsyncOperationParams) error {
	status := models.AsyncOperationStatus(strings.TrimSpace(string(p.Status)))
	if status == "" {
		status = models.AsyncOperationStatusQueued
	}
	maxAttempts := p.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	resource := strings.TrimSpace(strings.ToLower(p.Resource))
	if resource == "" {
		resource = "unknown"
	}
	operationType := strings.TrimSpace(strings.ToLower(p.OperationType))
	payload := p.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO async_operations(
			operation_id, tenant_id, actor_id, resource, resource_id, operation_type,
			status, attempt_count, max_attempts, payload
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 0, $8, $9)
	`, p.OperationID, p.TenantID, p.ActorID, resource, p.ResourceID, operationType, string(status), maxAttempts, payload)
	if err != nil {
		return fmt.Errorf("create async operation: %w", err)
	}
	return nil
}

func (r *AsyncOperationRepository) GetByID(ctx context.Context, tenantID, operationID uuid.UUID) (*models.AsyncOperationRecord, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT operation_id, tenant_id, actor_id, resource, resource_id, operation_type,
		       status, attempt_count, max_attempts, last_error, payload,
		       queued_at, started_at, finished_at, updated_at
		FROM async_operations
		WHERE operation_id = $1
		  AND tenant_id = $2
	`, operationID, tenantID)
	item, err := scanAsyncOperation(row)
	if err != nil {
		return nil, fmt.Errorf("get async operation: %w", err)
	}
	return item, nil
}

func (r *AsyncOperationRepository) MarkProcessing(ctx context.Context, operationID uuid.UUID, attempt int) error {
	if attempt < 1 {
		attempt = 1
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE async_operations
		SET status = 'processing',
		    attempt_count = GREATEST(attempt_count, $2),
		    started_at = COALESCE(started_at, NOW()),
		    updated_at = NOW()
		WHERE operation_id = $1
	`, operationID, attempt)
	if err != nil {
		return fmt.Errorf("mark async operation processing: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *AsyncOperationRepository) MarkRetryQueued(ctx context.Context, operationID uuid.UUID, attempt int, lastError string) error {
	if attempt < 1 {
		attempt = 1
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE async_operations
		SET status = 'queued',
		    attempt_count = GREATEST(attempt_count, $2),
		    last_error = $3,
		    updated_at = NOW()
		WHERE operation_id = $1
	`, operationID, attempt, strings.TrimSpace(lastError))
	if err != nil {
		return fmt.Errorf("mark async operation retry queued: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *AsyncOperationRepository) MarkDone(ctx context.Context, operationID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE async_operations
		SET status = 'done',
		    last_error = '',
		    finished_at = NOW(),
		    updated_at = NOW()
		WHERE operation_id = $1
	`, operationID)
	if err != nil {
		return fmt.Errorf("mark async operation done: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *AsyncOperationRepository) MarkFailed(ctx context.Context, operationID uuid.UUID, attempt int, lastError string) error {
	if attempt < 1 {
		attempt = 1
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE async_operations
		SET status = 'failed',
		    attempt_count = GREATEST(attempt_count, $2),
		    last_error = $3,
		    finished_at = NOW(),
		    updated_at = NOW()
		WHERE operation_id = $1
	`, operationID, attempt, strings.TrimSpace(lastError))
	if err != nil {
		return fmt.Errorf("mark async operation failed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

type asyncOperationScanner interface {
	Scan(dest ...any) error
}

func scanAsyncOperation(scanner asyncOperationScanner) (*models.AsyncOperationRecord, error) {
	var item models.AsyncOperationRecord
	var status string
	var payload []byte
	err := scanner.Scan(
		&item.OperationID,
		&item.TenantID,
		&item.ActorID,
		&item.Resource,
		&item.ResourceID,
		&item.OperationType,
		&status,
		&item.AttemptCount,
		&item.MaxAttempts,
		&item.LastError,
		&payload,
		&item.QueuedAt,
		&item.StartedAt,
		&item.FinishedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	item.Status = models.AsyncOperationStatus(status)
	if len(payload) == 0 {
		item.Payload = json.RawMessage(`{}`)
	} else {
		item.Payload = json.RawMessage(payload)
	}
	return &item, nil
}
