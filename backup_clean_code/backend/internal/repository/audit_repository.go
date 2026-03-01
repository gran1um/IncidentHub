package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"incidenthub/backend/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuditRepository struct {
	pool *pgxpool.Pool
}

func NewAuditRepository(pool *pgxpool.Pool) *AuditRepository {
	return &AuditRepository{pool: pool}
}

func (r *AuditRepository) Log(ctx context.Context, tenantID, actorID *uuid.UUID, action, objectType string, objectID *uuid.UUID, details map[string]any) error {
	if details == nil {
		details = map[string]any{}
	}
	b, err := json.Marshal(details)
	if err != nil {
		return fmt.Errorf("marshal audit details: %w", err)
	}

	q := `
		INSERT INTO audit_logs(tenant_id, actor_id, action, object_type, object_id, details)
		VALUES ($1,$2,$3,$4,$5,$6::jsonb)
	`
	if _, err := r.pool.Exec(ctx, q, tenantID, actorID, action, objectType, objectID, string(b)); err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}

func (r *AuditRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]models.AuditLog, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	q := `
		SELECT id, tenant_id, actor_id, action, object_type, object_id, details, created_at
		FROM audit_logs
		WHERE tenant_id=$1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.pool.Query(ctx, q, tenantID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list audit logs by tenant: %w", err)
	}
	defer rows.Close()

	out := make([]models.AuditLog, 0, limit)
	for rows.Next() {
		var item models.AuditLog
		var detailsRaw []byte
		if err := rows.Scan(&item.ID, &item.TenantID, &item.ActorID, &item.Action, &item.ObjectType, &item.ObjectID, &detailsRaw, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan audit row: %w", err)
		}
		if err := json.Unmarshal(detailsRaw, &item.Details); err != nil {
			item.Details = map[string]any{}
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audit rows: %w", err)
	}
	return out, nil
}
