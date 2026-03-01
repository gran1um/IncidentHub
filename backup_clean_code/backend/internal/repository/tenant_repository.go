package repository

import (
	"context"
	"errors"
	"fmt"
	"incidenthub/backend/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CreateTenantParams struct {
	Slug        string
	Name        string
	Description string
	MaxUsers    int
	Responsible *uuid.UUID
	IsActive    bool
}

type UpdateTenantParams struct {
	Name        *string
	Description *string
	IsActive    *bool
	MaxUsers    *int
	Responsible *uuid.UUID
}

type TenantRepository struct {
	pool *pgxpool.Pool
}

func NewTenantRepository(pool *pgxpool.Pool) *TenantRepository {
	return &TenantRepository{pool: pool}
}

func (r *TenantRepository) Create(ctx context.Context, p CreateTenantParams) (*models.Tenant, error) {
	maxUsers := p.MaxUsers
	if maxUsers <= 0 {
		maxUsers = 50
	}
	isActive := p.IsActive
	if !p.IsActive {
		isActive = true
	}
	q := `
		INSERT INTO tenants(slug, name, description, max_users, responsible_user_id, is_active)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, slug, name, description, is_active, max_users, responsible_user_id, created_at, updated_at
	`
	var t models.Tenant
	if err := r.pool.QueryRow(ctx, q, p.Slug, p.Name, p.Description, maxUsers, p.Responsible, isActive).Scan(
		&t.ID,
		&t.Slug,
		&t.Name,
		&t.Description,
		&t.IsActive,
		&t.MaxUsers,
		&t.ResponsibleUserID,
		&t.CreatedAt,
		&t.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("create tenant: %w", err)
	}
	return &t, nil
}

func (r *TenantRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Tenant, error) {
	q := `
		SELECT id, slug, name, description, is_active, max_users, responsible_user_id, created_at, updated_at
		FROM tenants WHERE id=$1
	`
	var t models.Tenant
	if err := r.pool.QueryRow(ctx, q, id).Scan(
		&t.ID,
		&t.Slug,
		&t.Name,
		&t.Description,
		&t.IsActive,
		&t.MaxUsers,
		&t.ResponsibleUserID,
		&t.CreatedAt,
		&t.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("get tenant by id: %w", err)
	}
	return &t, nil
}

func (r *TenantRepository) GetBySlug(ctx context.Context, slug string) (*models.Tenant, error) {
	q := `
		SELECT id, slug, name, description, is_active, max_users, responsible_user_id, created_at, updated_at
		FROM tenants WHERE slug=$1
	`
	var t models.Tenant
	if err := r.pool.QueryRow(ctx, q, slug).Scan(
		&t.ID,
		&t.Slug,
		&t.Name,
		&t.Description,
		&t.IsActive,
		&t.MaxUsers,
		&t.ResponsibleUserID,
		&t.CreatedAt,
		&t.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("get tenant by slug: %w", err)
	}
	return &t, nil
}

func (r *TenantRepository) List(ctx context.Context, limit, offset int) ([]models.Tenant, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	q := `
		SELECT id, slug, name, description, is_active, max_users, responsible_user_id, created_at, updated_at
		FROM tenants
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.pool.Query(ctx, q, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list tenants: %w", err)
	}
	defer rows.Close()

	out := make([]models.Tenant, 0, limit)
	for rows.Next() {
		var t models.Tenant
		if err := rows.Scan(
			&t.ID,
			&t.Slug,
			&t.Name,
			&t.Description,
			&t.IsActive,
			&t.MaxUsers,
			&t.ResponsibleUserID,
			&t.CreatedAt,
			&t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan tenant: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tenants: %w", err)
	}
	return out, nil
}

func (r *TenantRepository) Ensure(ctx context.Context, p CreateTenantParams) (*models.Tenant, error) {
	t, err := r.GetBySlug(ctx, p.Slug)
	if err == nil {
		return t, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	return r.Create(ctx, p)
}

func (r *TenantRepository) Update(ctx context.Context, tenantID uuid.UUID, p UpdateTenantParams) (*models.Tenant, error) {
	q := `
		UPDATE tenants
		SET
			name = COALESCE($2, name),
			description = COALESCE($3, description),
			is_active = COALESCE($4, is_active),
			max_users = COALESCE($5, max_users),
			responsible_user_id = COALESCE($6, responsible_user_id),
			updated_at = NOW()
		WHERE id = $1
		RETURNING id, slug, name, description, is_active, max_users, responsible_user_id, created_at, updated_at
	`
	var t models.Tenant
	if err := r.pool.QueryRow(ctx, q,
		tenantID,
		p.Name,
		p.Description,
		p.IsActive,
		p.MaxUsers,
		p.Responsible,
	).Scan(
		&t.ID,
		&t.Slug,
		&t.Name,
		&t.Description,
		&t.IsActive,
		&t.MaxUsers,
		&t.ResponsibleUserID,
		&t.CreatedAt,
		&t.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("update tenant: %w", err)
	}
	return &t, nil
}
