package repository

import (
	"context"
	"fmt"
	"incidenthub/backend/internal/models"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TaskRepository struct {
	pool *pgxpool.Pool
}

type TaskCaseSummary struct {
	OpenCount        int
	ClosedCount      int
	OverdueOpenCount int
	NextOpenDueAt    *time.Time
}

type CreateTaskParams struct {
	CaseID      uuid.UUID
	TenantID    uuid.UUID
	Title       string
	Description string
	Status      string
	AssigneeID  *uuid.UUID
	DueDate     *time.Time
}

type UpdateTaskParams struct {
	Title       *string
	Description *string
	Status      *string
	AssigneeID  *uuid.UUID
	DueDate     *time.Time
	DueDateSet  bool
}

func NewTaskRepository(pool *pgxpool.Pool) *TaskRepository {
	return &TaskRepository{pool: pool}
}

func (r *TaskRepository) Create(ctx context.Context, p CreateTaskParams) (*models.Task, error) {
	q := `
		INSERT INTO tasks(case_id, tenant_id, title, description, status, assignee_id, due_date)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, case_id, tenant_id, title, description, status, assignee_id, due_date, created_at, updated_at
	`
	var t models.Task
	if err := r.pool.QueryRow(ctx, q,
		p.CaseID,
		p.TenantID,
		p.Title,
		p.Description,
		p.Status,
		p.AssigneeID,
		p.DueDate,
	).Scan(
		&t.ID,
		&t.CaseID,
		&t.TenantID,
		&t.Title,
		&t.Description,
		&t.Status,
		&t.AssigneeID,
		&t.DueDate,
		&t.CreatedAt,
		&t.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}
	return &t, nil
}

func (r *TaskRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]models.Task, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	q := `
		SELECT id, case_id, tenant_id, title, description, status, assignee_id, due_date, created_at, updated_at
		FROM tasks
		WHERE tenant_id=$1
		ORDER BY updated_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.pool.Query(ctx, q, tenantID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	out := make([]models.Task, 0, limit)
	for rows.Next() {
		var t models.Task
		if err := rows.Scan(
			&t.ID,
			&t.CaseID,
			&t.TenantID,
			&t.Title,
			&t.Description,
			&t.Status,
			&t.AssigneeID,
			&t.DueDate,
			&t.CreatedAt,
			&t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan tasks: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tasks: %w", err)
	}
	return out, nil
}

func (r *TaskRepository) ListByCase(ctx context.Context, tenantID, caseID uuid.UUID, limit, offset int) ([]models.Task, error) {
	return listByCase(
		ctx,
		r.pool,
		`
			SELECT id, case_id, tenant_id, title, description, status, assignee_id, due_date, created_at, updated_at
			FROM tasks
			WHERE tenant_id=$1 AND case_id=$2
			ORDER BY updated_at DESC
			LIMIT $3 OFFSET $4
		`,
		tenantID,
		caseID,
		limit,
		offset,
		scanTask,
		"list tasks by case",
		"scan tasks by case",
		"iterate tasks by case",
	)
}

func (r *TaskRepository) CountByCaseIDs(ctx context.Context, tenantID uuid.UUID, caseIDs []uuid.UUID) (map[uuid.UUID]TaskCaseSummary, error) {
	out := make(map[uuid.UUID]TaskCaseSummary)
	if len(caseIDs) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT
			case_id,
			COUNT(*) FILTER (
				WHERE LOWER(COALESCE(status, '')) NOT IN ('done', 'completed', 'closed', 'canceled', 'cancelled')
			) AS open_count,
			COUNT(*) FILTER (
				WHERE LOWER(COALESCE(status, '')) IN ('done', 'completed', 'closed', 'canceled', 'cancelled')
			) AS closed_count,
			COUNT(*) FILTER (
				WHERE LOWER(COALESCE(status, '')) NOT IN ('done', 'completed', 'closed', 'canceled', 'cancelled')
				  AND due_date IS NOT NULL
				  AND due_date < NOW()
			) AS overdue_open_count,
			MIN(due_date) FILTER (
				WHERE LOWER(COALESCE(status, '')) NOT IN ('done', 'completed', 'closed', 'canceled', 'cancelled')
				  AND due_date IS NOT NULL
			) AS next_open_due_at
		FROM tasks
		WHERE tenant_id = $1
		  AND case_id = ANY($2::uuid[])
		GROUP BY case_id
	`, tenantID, caseIDs)
	if err != nil {
		return nil, fmt.Errorf("count tasks by case ids: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			caseID  uuid.UUID
			open    int
			closed  int
			overdue int
			nextDue *time.Time
		)
		if err := rows.Scan(&caseID, &open, &closed, &overdue, &nextDue); err != nil {
			return nil, fmt.Errorf("scan task summary by case ids: %w", err)
		}
		out[caseID] = TaskCaseSummary{
			OpenCount:        open,
			ClosedCount:      closed,
			OverdueOpenCount: overdue,
			NextOpenDueAt:    nextDue,
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate task summary by case ids: %w", err)
	}
	return out, nil
}

func (r *TaskRepository) Update(ctx context.Context, tenantID, taskID uuid.UUID, p UpdateTaskParams) (*models.Task, error) {
	q := `
		UPDATE tasks
		SET
			title = COALESCE($3, title),
			description = COALESCE($4, description),
			status = COALESCE($5, status),
			assignee_id = COALESCE($6, assignee_id),
			due_date = CASE WHEN $7 THEN $8 ELSE due_date END,
			updated_at = NOW()
		WHERE tenant_id = $1 AND id = $2
		RETURNING id, case_id, tenant_id, title, description, status, assignee_id, due_date, created_at, updated_at
	`
	var t models.Task
	if err := r.pool.QueryRow(ctx, q,
		tenantID,
		taskID,
		p.Title,
		p.Description,
		p.Status,
		p.AssigneeID,
		p.DueDateSet,
		p.DueDate,
	).Scan(
		&t.ID,
		&t.CaseID,
		&t.TenantID,
		&t.Title,
		&t.Description,
		&t.Status,
		&t.AssigneeID,
		&t.DueDate,
		&t.CreatedAt,
		&t.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("update task: %w", err)
	}
	return &t, nil
}

func (r *TaskRepository) Delete(ctx context.Context, tenantID, taskID uuid.UUID) error {
	cmd, err := r.pool.Exec(ctx, `DELETE FROM tasks WHERE tenant_id=$1 AND id=$2`, tenantID, taskID)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("delete task: no rows affected")
	}
	return nil
}

func scanTask(scanner rowScanner) (*models.Task, error) {
	var item models.Task
	if err := scanner.Scan(
		&item.ID,
		&item.CaseID,
		&item.TenantID,
		&item.Title,
		&item.Description,
		&item.Status,
		&item.AssigneeID,
		&item.DueDate,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &item, nil
}
