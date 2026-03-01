package repository

import (
	"context"
	"fmt"
	"incidenthub/backend/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ObservableRepository struct {
	pool *pgxpool.Pool
}

type CreateObservableParams struct {
	TenantID  uuid.UUID
	CaseID    uuid.UUID
	Type      string
	Value     string
	Verdict   string
	Source    string
	Tags      []string
	CreatedBy uuid.UUID
}

type UpdateObservableParams struct {
	Type    *string
	Value   *string
	Verdict *string
	Source  *string
	Tags    *[]string
}

func NewObservableRepository(pool *pgxpool.Pool) *ObservableRepository {
	return &ObservableRepository{pool: pool}
}

func (r *ObservableRepository) Create(ctx context.Context, p CreateObservableParams) (*models.Observable, error) {
	q := `
		INSERT INTO case_observables(tenant_id, case_id, type, value, verdict, source, tags, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id, tenant_id, case_id, type, value, verdict, source, tags, created_by, created_at, updated_at
	`
	var item models.Observable
	if err := r.pool.QueryRow(ctx, q,
		p.TenantID,
		p.CaseID,
		p.Type,
		p.Value,
		p.Verdict,
		p.Source,
		p.Tags,
		p.CreatedBy,
	).Scan(
		&item.ID,
		&item.TenantID,
		&item.CaseID,
		&item.Type,
		&item.Value,
		&item.Verdict,
		&item.Source,
		&item.Tags,
		&item.CreatedBy,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("create observable: %w", err)
	}
	return &item, nil
}

func (r *ObservableRepository) ListByCase(ctx context.Context, tenantID, caseID uuid.UUID, limit, offset int) ([]models.Observable, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	q := `
		SELECT id, tenant_id, case_id, type, value, verdict, source, tags, created_by, created_at, updated_at
		FROM case_observables
		WHERE tenant_id=$1 AND case_id=$2
		ORDER BY updated_at DESC
		LIMIT $3 OFFSET $4
	`

	rows, err := r.pool.Query(ctx, q, tenantID, caseID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list observables: %w", err)
	}
	defer rows.Close()

	items := make([]models.Observable, 0, limit)
	for rows.Next() {
		var item models.Observable
		if err := rows.Scan(
			&item.ID,
			&item.TenantID,
			&item.CaseID,
			&item.Type,
			&item.Value,
			&item.Verdict,
			&item.Source,
			&item.Tags,
			&item.CreatedBy,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan observables: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate observables: %w", err)
	}
	return items, nil
}

func (r *ObservableRepository) Update(ctx context.Context, tenantID, caseID, observableID uuid.UUID, p UpdateObservableParams) (*models.Observable, error) {
	q := `
		UPDATE case_observables
		SET
			type = COALESCE($4, type),
			value = COALESCE($5, value),
			verdict = COALESCE($6, verdict),
			source = COALESCE($7, source),
			tags = COALESCE($8, tags),
			updated_at = NOW()
		WHERE tenant_id = $1 AND case_id = $2 AND id = $3
		RETURNING id, tenant_id, case_id, type, value, verdict, source, tags, created_by, created_at, updated_at
	`
	var item models.Observable
	if err := r.pool.QueryRow(ctx, q,
		tenantID,
		caseID,
		observableID,
		p.Type,
		p.Value,
		p.Verdict,
		p.Source,
		p.Tags,
	).Scan(
		&item.ID,
		&item.TenantID,
		&item.CaseID,
		&item.Type,
		&item.Value,
		&item.Verdict,
		&item.Source,
		&item.Tags,
		&item.CreatedBy,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("update observable: %w", err)
	}
	return &item, nil
}

func (r *ObservableRepository) Delete(ctx context.Context, tenantID, caseID, observableID uuid.UUID) error {
	cmd, err := r.pool.Exec(ctx, `DELETE FROM case_observables WHERE tenant_id=$1 AND case_id=$2 AND id=$3`, tenantID, caseID, observableID)
	if err != nil {
		return fmt.Errorf("delete observable: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("delete observable: no rows affected")
	}
	return nil
}
