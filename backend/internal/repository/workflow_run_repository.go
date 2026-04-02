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

const workflowRunRetentionLimit = 1000

type WorkflowRunRepository struct {
	pool *pgxpool.Pool
}

type CaseWorkflowStatusSummary struct {
	Queued    int
	Running   int
	Completed int
	Failed    int
}

type CreateWorkflowRunParams struct {
	TenantID     uuid.UUID
	WorkflowID   uuid.UUID
	WorkflowKind string
	Trigger      string
	Status       models.WorkflowRunStatus
	Input        map[string]any
	CreatedBy    *uuid.UUID
}

type FinishWorkflowRunParams struct {
	Status models.WorkflowRunStatus
	Result map[string]any
	Error  string
}

func NewWorkflowRunRepository(pool *pgxpool.Pool) *WorkflowRunRepository {
	return &WorkflowRunRepository{pool: pool}
}

func (r *WorkflowRunRepository) Create(ctx context.Context, p CreateWorkflowRunParams) (*models.WorkflowRun, error) {
	status := normalizeWorkflowRunStatus(p.Status)
	trigger := strings.TrimSpace(strings.ToLower(p.Trigger))
	if trigger == "" {
		trigger = "manual_test"
	}
	kind := strings.TrimSpace(strings.ToLower(p.WorkflowKind))
	if kind == "" {
		kind = "workflow"
	}
	input := p.Input
	if input == nil {
		input = map[string]any{}
	}
	inputRaw, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal workflow run input: %w", err)
	}

	row := r.pool.QueryRow(ctx, `
		INSERT INTO workflow_runs(
			tenant_id, workflow_id, workflow_kind, trigger, status, input, created_by
		)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7)
		RETURNING
			id, tenant_id, workflow_id, workflow_kind, trigger, status,
			started_at, finished_at, duration_ms, input, result, error,
			created_by, created_at, updated_at
	`, p.TenantID, p.WorkflowID, kind, trigger, string(status), inputRaw, p.CreatedBy)

	item, err := scanWorkflowRun(row)
	if err != nil {
		return nil, fmt.Errorf("create workflow run: %w", err)
	}
	if err := r.TrimHistory(ctx, p.TenantID, p.WorkflowID, workflowRunRetentionLimit); err != nil {
		return nil, fmt.Errorf("trim workflow run history: %w", err)
	}
	return item, nil
}

func (r *WorkflowRunRepository) Finish(ctx context.Context, runID, tenantID uuid.UUID, p FinishWorkflowRunParams) (*models.WorkflowRun, error) {
	status := normalizeWorkflowRunStatus(p.Status)
	result := p.Result
	if result == nil {
		result = map[string]any{}
	}
	resultRaw, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("marshal workflow run result: %w", err)
	}
	message := strings.TrimSpace(p.Error)

	row := r.pool.QueryRow(ctx, `
		UPDATE workflow_runs
		SET
			status = $3,
			result = $4::jsonb,
			error = $5,
			finished_at = NOW(),
			duration_ms = GREATEST(0, FLOOR(EXTRACT(EPOCH FROM (NOW() - started_at)) * 1000)::INT),
			updated_at = NOW()
		WHERE id = $1
		  AND tenant_id = $2
		RETURNING
			id, tenant_id, workflow_id, workflow_kind, trigger, status,
			started_at, finished_at, duration_ms, input, result, error,
			created_by, created_at, updated_at
	`, runID, tenantID, string(status), resultRaw, message)
	item, err := scanWorkflowRun(row)
	if err != nil {
		return nil, fmt.Errorf("finish workflow run: %w", err)
	}
	if err := r.TrimHistory(ctx, tenantID, item.WorkflowID, workflowRunRetentionLimit); err != nil {
		return nil, fmt.Errorf("trim workflow run history: %w", err)
	}
	return item, nil
}

func (r *WorkflowRunRepository) ListByWorkflow(ctx context.Context, tenantID, workflowID uuid.UUID, limit int) ([]models.WorkflowRun, error) {
	if limit <= 0 {
		limit = 100
	} else if limit > workflowRunRetentionLimit {
		limit = workflowRunRetentionLimit
	}
	rows, err := r.pool.Query(ctx, `
		SELECT
			id, tenant_id, workflow_id, workflow_kind, trigger, status,
			started_at, finished_at, duration_ms, input, result, error,
			created_by, created_at, updated_at
		FROM workflow_runs
		WHERE tenant_id = $1
		  AND workflow_id = $2
		ORDER BY started_at DESC, created_at DESC
		LIMIT $3
	`, tenantID, workflowID, limit)
	if err != nil {
		return nil, fmt.Errorf("list workflow runs: %w", err)
	}
	defer rows.Close()
	return collectWorkflowRuns(rows)
}

func (r *WorkflowRunRepository) TrimHistory(ctx context.Context, tenantID, workflowID uuid.UUID, keep int) error {
	if keep <= 0 {
		keep = workflowRunRetentionLimit
	}
	_, err := r.pool.Exec(ctx, `
		DELETE FROM workflow_runs
		WHERE id IN (
			SELECT id
			FROM workflow_runs
			WHERE tenant_id = $1
			  AND workflow_id = $2
			ORDER BY started_at DESC, created_at DESC
			OFFSET $3
		)
	`, tenantID, workflowID, keep)
	if err != nil {
		return fmt.Errorf("trim workflow runs history: %w", err)
	}
	return nil
}

func (r *WorkflowRunRepository) CountStatusesByCaseIDs(ctx context.Context, tenantID uuid.UUID, caseIDs []string) (map[string]CaseWorkflowStatusSummary, error) {
	out := make(map[string]CaseWorkflowStatusSummary)
	if len(caseIDs) == 0 {
		return out, nil
	}

	rows, err := r.pool.Query(ctx, `
		WITH scoped_runs AS (
			SELECT
				LOWER(
					COALESCE(
						NULLIF(BTRIM(input->>'case_id'), ''),
						NULLIF(BTRIM(input->>'caseId'), ''),
						NULLIF(BTRIM(input->'case'->>'id'), ''),
						NULLIF(BTRIM(input->'case'->>'case_id'), ''),
						NULLIF(BTRIM(input->'case'->>'caseId'), ''),
						NULLIF(BTRIM(input->'context'->>'case_id'), ''),
						NULLIF(BTRIM(input->'context'->>'caseId'), '')
					)
				) AS case_ref,
				LOWER(COALESCE(status, '')) AS status
			FROM workflow_runs
			WHERE tenant_id = $1
		)
		SELECT case_ref, status, COUNT(*)
		FROM scoped_runs
		WHERE case_ref = ANY($2::text[])
		GROUP BY case_ref, status
	`, tenantID, caseIDs)
	if err != nil {
		return nil, fmt.Errorf("count workflow statuses by case ids: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			caseRef string
			status  string
			count   int
		)
		if err := rows.Scan(&caseRef, &status, &count); err != nil {
			return nil, fmt.Errorf("scan workflow statuses by case ids: %w", err)
		}
		if caseRef == "" {
			continue
		}
		current := out[caseRef]
		switch status {
		case "queued":
			current.Queued += count
		case "running":
			current.Running += count
		case "success", "completed", "done":
			current.Completed += count
		case "failed", "error":
			current.Failed += count
		}
		out[caseRef] = current
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workflow statuses by case ids: %w", err)
	}
	return out, nil
}

type workflowRunScanner interface {
	Scan(dest ...any) error
}

func scanWorkflowRun(scanner workflowRunScanner) (*models.WorkflowRun, error) {
	var (
		item      models.WorkflowRun
		status    string
		inputRaw  []byte
		resultRaw []byte
	)
	err := scanner.Scan(
		&item.ID,
		&item.TenantID,
		&item.WorkflowID,
		&item.WorkflowKind,
		&item.Trigger,
		&status,
		&item.StartedAt,
		&item.FinishedAt,
		&item.DurationMS,
		&inputRaw,
		&resultRaw,
		&item.Error,
		&item.CreatedBy,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	item.Status = normalizeWorkflowRunStatus(models.WorkflowRunStatus(status))
	if len(inputRaw) == 0 {
		item.Input = map[string]any{}
	} else if err := json.Unmarshal(inputRaw, &item.Input); err != nil {
		return nil, fmt.Errorf("unmarshal workflow run input: %w", err)
	}
	if len(resultRaw) == 0 {
		item.Result = map[string]any{}
	} else if err := json.Unmarshal(resultRaw, &item.Result); err != nil {
		return nil, fmt.Errorf("unmarshal workflow run result: %w", err)
	}
	return &item, nil
}

func collectWorkflowRuns(rows pgx.Rows) ([]models.WorkflowRun, error) {
	out := make([]models.WorkflowRun, 0)
	for rows.Next() {
		item, err := scanWorkflowRun(rows)
		if err != nil {
			return nil, fmt.Errorf("scan workflow run: %w", err)
		}
		out = append(out, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workflow runs: %w", err)
	}
	return out, nil
}

func normalizeWorkflowRunStatus(input models.WorkflowRunStatus) models.WorkflowRunStatus {
	status := models.WorkflowRunStatus(strings.TrimSpace(strings.ToLower(string(input))))
	switch status {
	case models.WorkflowRunStatusQueued, models.WorkflowRunStatusRunning, models.WorkflowRunStatusSuccess, models.WorkflowRunStatusFailed:
		return status
	default:
		return models.WorkflowRunStatusQueued
	}
}
