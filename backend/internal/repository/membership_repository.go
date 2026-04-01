package repository

import (
	"context"
	"fmt"
	"incidenthub/backend/internal/models"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MembershipRepository struct {
	pool *pgxpool.Pool
}

func NewMembershipRepository(pool *pgxpool.Pool) *MembershipRepository {
	return &MembershipRepository{pool: pool}
}

func (r *MembershipRepository) Upsert(ctx context.Context, tenantID, userID uuid.UUID, role models.TenantRole) error {
	q := `
		INSERT INTO tenant_memberships(tenant_id, user_id, role)
		VALUES($1,$2,$3)
		ON CONFLICT (tenant_id, user_id)
		DO UPDATE SET role=EXCLUDED.role, is_active=TRUE, updated_at=NOW()
	`
	if _, err := r.pool.Exec(ctx, q, tenantID, userID, string(role)); err != nil {
		return fmt.Errorf("upsert membership: %w", err)
	}
	return nil
}

func (r *MembershipRepository) Get(ctx context.Context, tenantID, userID uuid.UUID) (*models.Membership, error) {
	q := `
		SELECT id, tenant_id, user_id, role, is_active, created_at, updated_at
		FROM tenant_memberships
		WHERE tenant_id=$1 AND user_id=$2
	`
	var m models.Membership
	if err := r.pool.QueryRow(ctx, q, tenantID, userID).Scan(
		&m.ID,
		&m.TenantID,
		&m.UserID,
		&m.Role,
		&m.IsActive,
		&m.CreatedAt,
		&m.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("get membership: %w", err)
	}
	return &m, nil
}

func (r *MembershipRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]models.Membership, error) {
	q := `
		SELECT id, tenant_id, user_id, role, is_active, created_at, updated_at
		FROM tenant_memberships
		WHERE user_id=$1 AND is_active=TRUE
		ORDER BY created_at DESC
	`
	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("list memberships by user: %w", err)
	}
	defer rows.Close()

	out := make([]models.Membership, 0)
	for rows.Next() {
		var m models.Membership
		if err := rows.Scan(
			&m.ID,
			&m.TenantID,
			&m.UserID,
			&m.Role,
			&m.IsActive,
			&m.CreatedAt,
			&m.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan memberships: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate memberships: %w", err)
	}
	return out, nil
}

func (r *MembershipRepository) SetActive(ctx context.Context, tenantID, userID uuid.UUID, active bool) error {
	q := `
		UPDATE tenant_memberships
		SET is_active = $3, updated_at = $4
		WHERE tenant_id = $1 AND user_id = $2
	`
	res, err := r.pool.Exec(ctx, q, tenantID, userID, active, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("set membership active: %w", err)
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("set membership active: membership not found")
	}
	return nil
}

func (r *MembershipRepository) CountActiveByTenantRole(ctx context.Context, tenantID uuid.UUID, role models.TenantRole) (int, error) {
	q := `
		SELECT COUNT(*)
		FROM tenant_memberships
		WHERE tenant_id = $1 AND role = $2 AND is_active = TRUE
	`
	var count int
	if err := r.pool.QueryRow(ctx, q, tenantID, role).Scan(&count); err != nil {
		return 0, fmt.Errorf("count active tenant memberships by role: %w", err)
	}
	return count, nil
}
