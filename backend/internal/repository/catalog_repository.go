package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"incidenthub/backend/internal/models"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CatalogRepository struct {
	pool *pgxpool.Pool
}

type CatalogCreateParams struct {
	TenantID  *uuid.UUID
	Kind      string
	OwnerID   *uuid.UUID
	RefID     *uuid.UUID
	Data      map[string]any
	CreatedBy *uuid.UUID
}

type CatalogListParams struct {
	Kind          string
	TenantID      *uuid.UUID
	IncludeGlobal bool
	OwnerID       *uuid.UUID
	RefID         *uuid.UUID
	Limit         int
	Offset        int
}

type CatalogUpdateParams struct {
	Data map[string]any
}

func NewCatalogRepository(pool *pgxpool.Pool) *CatalogRepository {
	return &CatalogRepository{pool: pool}
}

func normalizeCatalogKind(kind string) string {
	return strings.TrimSpace(strings.ToLower(kind))
}

func (r *CatalogRepository) Create(ctx context.Context, p CatalogCreateParams) (*models.CatalogItem, error) {
	payload, err := json.Marshal(p.Data)
	if err != nil {
		return nil, fmt.Errorf("marshal catalog data: %w", err)
	}

	q := `
		INSERT INTO catalog_items(tenant_id, kind, owner_id, ref_id, data, created_by)
		VALUES ($1,$2,$3,$4,$5::jsonb,$6)
		RETURNING id, tenant_id, kind, owner_id, ref_id, data, created_by, created_at, updated_at
	`
	var (
		item    models.CatalogItem
		rawData []byte
	)
	if err := r.pool.QueryRow(ctx, q,
		p.TenantID,
		normalizeCatalogKind(p.Kind),
		p.OwnerID,
		p.RefID,
		payload,
		p.CreatedBy,
	).Scan(
		&item.ID,
		&item.TenantID,
		&item.Kind,
		&item.OwnerID,
		&item.RefID,
		&rawData,
		&item.CreatedBy,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("create catalog item: %w", err)
	}

	if err := json.Unmarshal(rawData, &item.Data); err != nil {
		return nil, fmt.Errorf("unmarshal catalog data: %w", err)
	}
	if item.Data == nil {
		item.Data = map[string]any{}
	}
	return &item, nil
}

func (r *CatalogRepository) List(ctx context.Context, p CatalogListParams) ([]models.CatalogItem, error) {
	if p.Limit <= 0 || p.Limit > 500 {
		p.Limit = 200
	}
	if p.Offset < 0 {
		p.Offset = 0
	}

	args := make([]any, 0, 8)
	args = append(args, normalizeCatalogKind(p.Kind))
	where := []string{"kind = $1"}
	argPos := 2

	if p.TenantID != nil {
		if p.IncludeGlobal {
			where = append(where, fmt.Sprintf("(tenant_id = $%d OR tenant_id IS NULL)", argPos))
		} else {
			where = append(where, fmt.Sprintf("tenant_id = $%d", argPos))
		}
		args = append(args, *p.TenantID)
		argPos++
	} else {
		where = append(where, "tenant_id IS NULL")
	}

	if p.OwnerID != nil {
		where = append(where, fmt.Sprintf("owner_id = $%d", argPos))
		args = append(args, *p.OwnerID)
		argPos++
	}
	if p.RefID != nil {
		where = append(where, fmt.Sprintf("ref_id = $%d", argPos))
		args = append(args, *p.RefID)
		argPos++
	}

	args = append(args, p.Limit, p.Offset)
	q := fmt.Sprintf(`
		SELECT id, tenant_id, kind, owner_id, ref_id, data, created_by, created_at, updated_at
		FROM catalog_items
		WHERE %s
		ORDER BY updated_at DESC
		LIMIT $%d OFFSET $%d
	`, strings.Join(where, " AND "), argPos, argPos+1)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list catalog items: %w", err)
	}
	return collectRows(rows, p.Limit, scanCatalogItem, "scan catalog item", "iterate catalog items")
}

func (r *CatalogRepository) ListAcrossTenants(ctx context.Context, kind string, limit int) ([]models.CatalogItem, error) {
	if limit <= 0 || limit > 2000 {
		limit = 500
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, kind, owner_id, ref_id, data, created_by, created_at, updated_at
		FROM catalog_items
		WHERE kind = $1
		  AND tenant_id IS NOT NULL
		ORDER BY updated_at DESC
		LIMIT $2
	`, normalizeCatalogKind(kind), limit)
	if err != nil {
		return nil, fmt.Errorf("list catalog items across tenants: %w", err)
	}
	return collectRows(rows, limit, scanCatalogItem, "scan catalog item", "iterate catalog items")
}

func (r *CatalogRepository) GetByID(ctx context.Context, kind string, id uuid.UUID, tenantID *uuid.UUID) (*models.CatalogItem, error) {
	args := []any{normalizeCatalogKind(kind), id}
	where := "kind = $1 AND id = $2"
	if tenantID != nil {
		where += " AND tenant_id = $3"
		args = append(args, *tenantID)
	}
	q := fmt.Sprintf(`
		SELECT id, tenant_id, kind, owner_id, ref_id, data, created_by, created_at, updated_at
		FROM catalog_items
		WHERE %s
	`, where)

	row := r.pool.QueryRow(ctx, q, args...)
	item, err := scanCatalogItem(row)
	if err != nil {
		return nil, fmt.Errorf("get catalog item: %w", err)
	}
	return item, nil
}

func (r *CatalogRepository) Update(ctx context.Context, kind string, id uuid.UUID, tenantID *uuid.UUID, p CatalogUpdateParams) (*models.CatalogItem, error) {
	payload, err := json.Marshal(p.Data)
	if err != nil {
		return nil, fmt.Errorf("marshal catalog data: %w", err)
	}

	args := []any{normalizeCatalogKind(kind), id, payload}
	where := "kind = $1 AND id = $2"
	if tenantID != nil {
		where += " AND tenant_id = $4"
		args = append(args, *tenantID)
	}

	q := fmt.Sprintf(`
		UPDATE catalog_items
		SET
			data = data || $3::jsonb,
			updated_at = NOW()
		WHERE %s
		RETURNING id, tenant_id, kind, owner_id, ref_id, data, created_by, created_at, updated_at
	`, where)

	var (
		item    models.CatalogItem
		rawData []byte
	)
	if err := r.pool.QueryRow(ctx, q, args...).Scan(
		&item.ID,
		&item.TenantID,
		&item.Kind,
		&item.OwnerID,
		&item.RefID,
		&rawData,
		&item.CreatedBy,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("update catalog item: %w", err)
	}
	if err := json.Unmarshal(rawData, &item.Data); err != nil {
		return nil, fmt.Errorf("unmarshal catalog item: %w", err)
	}
	if item.Data == nil {
		item.Data = map[string]any{}
	}
	return &item, nil
}

func (r *CatalogRepository) Delete(ctx context.Context, kind string, id uuid.UUID, tenantID *uuid.UUID) error {
	args := []any{normalizeCatalogKind(kind), id}
	where := "kind = $1 AND id = $2"
	if tenantID != nil {
		where += " AND tenant_id = $3"
		args = append(args, *tenantID)
	}
	q := fmt.Sprintf(`DELETE FROM catalog_items WHERE %s`, where)

	cmd, err := r.pool.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("delete catalog item: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("delete catalog item: no rows affected")
	}
	return nil
}

func (r *CatalogRepository) ListByKindAllTenants(ctx context.Context, kind string, limit, offset int) ([]models.CatalogItem, error) {
	if limit <= 0 || limit > 1000 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}

	q := `
		SELECT id, tenant_id, kind, owner_id, ref_id, data, created_by, created_at, updated_at
		FROM catalog_items
		WHERE kind = $1 AND tenant_id IS NOT NULL
		ORDER BY updated_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.pool.Query(ctx, q, normalizeCatalogKind(kind), limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list catalog items by kind for all tenants: %w", err)
	}
	return collectRows(rows, limit, scanCatalogItem, "scan catalog item by kind for all tenants", "iterate catalog items by kind for all tenants")
}

func scanCatalogItem(scanner rowScanner) (*models.CatalogItem, error) {
	var (
		item    models.CatalogItem
		rawData []byte
	)
	if err := scanner.Scan(
		&item.ID,
		&item.TenantID,
		&item.Kind,
		&item.OwnerID,
		&item.RefID,
		&rawData,
		&item.CreatedBy,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(rawData, &item.Data); err != nil {
		return nil, fmt.Errorf("unmarshal catalog item data: %w", err)
	}
	if item.Data == nil {
		item.Data = map[string]any{}
	}
	return &item, nil
}
