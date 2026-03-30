package repository

import (
	"context"
	"errors"
	"fmt"

	"incidenthub/backend/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *CaseRepository) ShareWithTenant(
	ctx context.Context,
	ownerTenantID uuid.UUID,
	caseID uuid.UUID,
	sharedTenantID uuid.UUID,
	createdBy *uuid.UUID,
) error {
	cmd, err := r.pool.Exec(ctx, `
		INSERT INTO case_tenant_shares(case_id, owner_tenant_id, shared_tenant_id, created_by)
		SELECT c.id, c.tenant_id, $3, $4
		FROM cases c
		WHERE c.id = $1 AND c.tenant_id = $2
		ON CONFLICT (case_id, shared_tenant_id) DO UPDATE
		SET
			owner_tenant_id = EXCLUDED.owner_tenant_id,
			created_by = EXCLUDED.created_by,
			updated_at = NOW()
	`, caseID, ownerTenantID, sharedTenantID, createdBy)
	if err != nil {
		return fmt.Errorf("share case with tenant: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("share case with tenant: case not found in owner tenant")
	}
	return nil
}

func (r *CaseRepository) ListShares(ctx context.Context, ownerTenantID, caseID uuid.UUID) ([]models.CaseTenantShare, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT case_id, owner_tenant_id, shared_tenant_id, created_by, created_at, updated_at
		FROM case_tenant_shares
		WHERE owner_tenant_id = $1 AND case_id = $2
		ORDER BY created_at ASC
	`, ownerTenantID, caseID)
	if err != nil {
		return nil, fmt.Errorf("list case shares: %w", err)
	}
	defer rows.Close()

	out := make([]models.CaseTenantShare, 0, 4)
	for rows.Next() {
		var item models.CaseTenantShare
		if err := rows.Scan(
			&item.CaseID,
			&item.OwnerTenantID,
			&item.SharedTenantID,
			&item.CreatedBy,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan case share: %w", err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate case shares: %w", err)
	}
	return out, nil
}

func (r *CaseRepository) ResolveAccessibleTenant(ctx context.Context, requestedTenantID, caseID uuid.UUID) (uuid.UUID, bool, error) {
	var ownerTenantID uuid.UUID
	if err := r.pool.QueryRow(ctx, `
		SELECT c.tenant_id
		FROM cases c
		WHERE c.id = $1
		  AND (
			c.tenant_id = $2
			OR EXISTS (
				SELECT 1
				FROM case_tenant_shares cs
				WHERE cs.case_id = c.id
				  AND cs.shared_tenant_id = $2
			)
		  )
	`, caseID, requestedTenantID).Scan(&ownerTenantID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, false, nil
		}
		return uuid.Nil, false, fmt.Errorf("resolve accessible case tenant: %w", err)
	}
	return ownerTenantID, true, nil
}
