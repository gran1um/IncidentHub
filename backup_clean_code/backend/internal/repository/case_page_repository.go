package repository

import (
	"context"
	"fmt"
	"incidenthub/backend/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CasePageRepository struct {
	pool *pgxpool.Pool
}

type CreateCasePageParams struct {
	TenantID  uuid.UUID
	CaseID    uuid.UUID
	Title     string
	Body      string
	CreatedBy uuid.UUID
}

func NewCasePageRepository(pool *pgxpool.Pool) *CasePageRepository {
	return &CasePageRepository{pool: pool}
}

func (r *CasePageRepository) Create(ctx context.Context, p CreateCasePageParams) (*models.CasePage, error) {
	q := `
		INSERT INTO case_pages(tenant_id, case_id, title, body, created_by, updated_by)
		VALUES ($1,$2,$3,$4,$5,$5)
		RETURNING id, tenant_id, case_id, title, body, created_by, updated_by, created_at, updated_at
	`

	var item models.CasePage
	if err := r.pool.QueryRow(ctx, q,
		p.TenantID,
		p.CaseID,
		p.Title,
		p.Body,
		p.CreatedBy,
	).Scan(
		&item.ID,
		&item.TenantID,
		&item.CaseID,
		&item.Title,
		&item.Body,
		&item.CreatedBy,
		&item.UpdatedBy,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("create case page: %w", err)
	}
	return &item, nil
}

func (r *CasePageRepository) ListByCase(ctx context.Context, tenantID, caseID uuid.UUID, limit, offset int) ([]models.CasePage, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	q := `
		SELECT id, tenant_id, case_id, title, body, created_by, updated_by, created_at, updated_at
		FROM case_pages
		WHERE tenant_id=$1 AND case_id=$2
		ORDER BY updated_at DESC
		LIMIT $3 OFFSET $4
	`

	rows, err := r.pool.Query(ctx, q, tenantID, caseID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list case pages: %w", err)
	}
	defer rows.Close()

	items := make([]models.CasePage, 0, limit)
	for rows.Next() {
		var item models.CasePage
		if err := rows.Scan(
			&item.ID,
			&item.TenantID,
			&item.CaseID,
			&item.Title,
			&item.Body,
			&item.CreatedBy,
			&item.UpdatedBy,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan case pages: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate case pages: %w", err)
	}
	return items, nil
}
