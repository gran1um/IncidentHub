package repository

import (
	"context"
	"fmt"
	"incidenthub/backend/internal/models"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AlertRepository struct {
	pool *pgxpool.Pool
}

//nolint:gochecknoglobals // Static search fields used in SQL query composition.
var alertListSearchFields = []string{
	"title",
	"description",
	"source",
	"status",
	"severity",
	"COALESCE(tlp, '')",
	"COALESCE(pap, '')",
}

type CreateAlertParams struct {
	ID          *uuid.UUID
	TenantID    uuid.UUID
	CaseID      *uuid.UUID
	Title       string
	Description string
	Source      string
	Status      string
	Severity    string
	TLP         string
	PAP         string
	CreatedBy   *uuid.UUID
	AssignedTo  *uuid.UUID
}

type UpdateAlertParams struct {
	CaseID      *uuid.UUID
	Title       *string
	Description *string
	Source      *string
	Status      *string
	Severity    *string
	TLP         *string
	PAP         *string
	AssignedTo  *uuid.UUID
}

func NewAlertRepository(pool *pgxpool.Pool) *AlertRepository {
	return &AlertRepository{pool: pool}
}

func (r *AlertRepository) Create(ctx context.Context, p CreateAlertParams) (*models.Alert, error) {
	q := `
		INSERT INTO alerts(id, tenant_id, case_id, title, description, source, status, severity, tlp, pap, created_by, assigned_to)
		VALUES (COALESCE($1, gen_random_uuid()),$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING id, tenant_id, case_id, title, description, source, status, severity, tlp, pap, created_by, assigned_to, created_at, updated_at
	`
	item, err := scanAlert(
		r.pool.QueryRow(ctx, q,
			p.ID,
			p.TenantID,
			p.CaseID,
			p.Title,
			p.Description,
			p.Source,
			p.Status,
			p.Severity,
			p.TLP,
			p.PAP,
			p.CreatedBy,
			p.AssignedTo,
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create alert: %w", err)
	}
	return item, nil
}

func (r *AlertRepository) Delete(ctx context.Context, tenantID, alertID uuid.UUID) (bool, error) {
	result, err := r.pool.Exec(ctx, `DELETE FROM alerts WHERE tenant_id = $1 AND id = $2`, tenantID, alertID)
	if err != nil {
		return false, fmt.Errorf("delete alert: %w", err)
	}
	return result.RowsAffected() > 0, nil
}

func (r *AlertRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]models.Alert, error) {
	return r.ListByTenantWithAssignedSorted(ctx, tenantID, limit, offset, "all", nil, "updated_at", "desc")
}

func (r *AlertRepository) ListByTenantWithAssigned(
	ctx context.Context,
	tenantID uuid.UUID,
	limit,
	offset int,
	assigned string,
	assignedTo *uuid.UUID,
) ([]models.Alert, error) {
	return r.ListByTenantWithAssignedSorted(ctx, tenantID, limit, offset, assigned, assignedTo, "updated_at", "desc")
}

func (r *AlertRepository) ListByTenantWithAssignedSorted(
	ctx context.Context,
	tenantID uuid.UUID,
	limit,
	offset int,
	assigned string,
	assignedTo *uuid.UUID,
	sortBy,
	sortOrder string,
) ([]models.Alert, error) {
	return r.ListByTenantWithAssignedSortedAndSearch(
		ctx,
		tenantID,
		limit,
		offset,
		assigned,
		assignedTo,
		sortBy,
		sortOrder,
		ListSearchParams{},
	)
}

func (r *AlertRepository) ListByTenantWithAssignedSortedAndSearch(
	ctx context.Context,
	tenantID uuid.UUID,
	limit,
	offset int,
	assigned string,
	assignedTo *uuid.UUID,
	sortBy,
	sortOrder string,
	search ListSearchParams,
) ([]models.Alert, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	whereClause, whereArgs := buildAssignedFilterClause(assigned, assignedTo)
	args := make([]any, 0, 4)
	args = append(args, tenantID)
	args = append(args, whereArgs...)
	searchClause, searchArgs := buildListSearchWhereClause(alertListSearchFields, search, len(args)+1)
	args = append(args, searchArgs...)
	limitPos := len(args) + 1
	offsetPos := len(args) + 2
	args = append(args, limit, offset)
	orderClause := buildAlertOrderClause(sortBy, sortOrder)
	q := fmt.Sprintf(`
		SELECT id, tenant_id, case_id, title, description, source, status, severity, tlp, pap, created_by, assigned_to, created_at, updated_at
		FROM alerts
		WHERE tenant_id=$1%s%s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, whereClause, searchClause, orderClause, limitPos, offsetPos)
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list alerts: %w", err)
	}
	defer rows.Close()

	return scanAlertRows(rows)
}

func buildAlertOrderClause(sortBy, sortOrder string) string {
	column := "updated_at"
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "created_at":
		column = "created_at"
	case "severity":
		column = "severity"
	case "status":
		column = "status"
	case "source":
		column = "source"
	case "title":
		column = "title"
	case "updated_at":
		column = "updated_at"
	}

	order := "DESC"
	if strings.EqualFold(strings.TrimSpace(sortOrder), "asc") {
		order = "ASC"
	}
	return column + " " + order
}

func (r *AlertRepository) CountByTenant(ctx context.Context, tenantID uuid.UUID) (int, error) {
	return r.CountByTenantWithAssigned(ctx, tenantID, "all", nil)
}

func (r *AlertRepository) CountByTenantWithAssigned(
	ctx context.Context,
	tenantID uuid.UUID,
	assigned string,
	assignedTo *uuid.UUID,
) (int, error) {
	return r.CountByTenantWithAssignedAndSearch(ctx, tenantID, assigned, assignedTo, ListSearchParams{})
}

func (r *AlertRepository) CountByTenantWithAssignedAndSearch(
	ctx context.Context,
	tenantID uuid.UUID,
	assigned string,
	assignedTo *uuid.UUID,
	search ListSearchParams,
) (int, error) {
	var total int
	whereClause, whereArgs := buildAssignedFilterClause(assigned, assignedTo)
	args := make([]any, 0, 1+len(whereArgs)+2)
	args = append(args, tenantID)
	args = append(args, whereArgs...)
	searchClause, searchArgs := buildListSearchWhereClause(alertListSearchFields, search, len(args)+1)
	args = append(args, searchArgs...)
	q := fmt.Sprintf(`SELECT COUNT(*) FROM alerts WHERE tenant_id = $1%s%s`, whereClause, searchClause)
	if err := r.pool.QueryRow(ctx, q, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("count alerts: %w", err)
	}
	return total, nil
}

func buildAssignedFilterClause(assigned string, assignedTo *uuid.UUID) (clause string, args []any) {
	if assignedTo != nil {
		return " AND assigned_to = $2", []any{*assignedTo}
	}
	switch normalizeAssignedFilterMode(assigned) {
	case "assigned":
		return " AND assigned_to IS NOT NULL", nil
	case "unassigned":
		return " AND assigned_to IS NULL", nil
	default:
		return "", nil
	}
}

func normalizeAssignedFilterMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "assigned":
		return "assigned"
	case "unassigned":
		return "unassigned"
	default:
		return "all"
	}
}

func (r *AlertRepository) GetByID(ctx context.Context, tenantID, alertID uuid.UUID) (*models.Alert, error) {
	q := `
		SELECT id, tenant_id, case_id, title, description, source, status, severity, tlp, pap, created_by, assigned_to, created_at, updated_at
		FROM alerts
		WHERE tenant_id = $1 AND id = $2
	`
	item, err := scanAlert(r.pool.QueryRow(ctx, q, tenantID, alertID))
	if err != nil {
		return nil, fmt.Errorf("get alert: %w", err)
	}
	return item, nil
}

func (r *AlertRepository) ListByIDs(ctx context.Context, tenantID uuid.UUID, alertIDs []uuid.UUID) ([]models.Alert, error) {
	if len(alertIDs) == 0 {
		return []models.Alert{}, nil
	}
	q := `
		SELECT id, tenant_id, case_id, title, description, source, status, severity, tlp, pap, created_by, assigned_to, created_at, updated_at
		FROM alerts
		WHERE tenant_id = $1 AND id = ANY($2::uuid[])
		ORDER BY updated_at DESC
	`
	rows, err := r.pool.Query(ctx, q, tenantID, alertIDs)
	if err != nil {
		return nil, fmt.Errorf("list alerts by ids: %w", err)
	}
	defer rows.Close()
	return scanAlertRows(rows)
}

func (r *AlertRepository) ListByCase(ctx context.Context, tenantID, caseID uuid.UUID, limit, offset int) ([]models.Alert, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	q := `
		SELECT id, tenant_id, case_id, title, description, source, status, severity, tlp, pap, created_by, assigned_to, created_at, updated_at
		FROM alerts
		WHERE tenant_id = $1 AND case_id = $2
		ORDER BY updated_at DESC
		LIMIT $3 OFFSET $4
	`
	rows, err := r.pool.Query(ctx, q, tenantID, caseID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list alerts by case: %w", err)
	}
	defer rows.Close()
	return scanAlertRows(rows)
}

func (r *AlertRepository) BindToCase(ctx context.Context, tenantID uuid.UUID, alertIDs []uuid.UUID, caseID *uuid.UUID) ([]models.Alert, error) {
	if len(alertIDs) == 0 {
		return []models.Alert{}, nil
	}
	q := `
		UPDATE alerts
		SET
			case_id = $3,
			status = CASE WHEN status = 'new' THEN 'triaged' ELSE status END,
			updated_at = NOW()
		WHERE tenant_id = $1 AND id = ANY($2::uuid[])
		RETURNING id, tenant_id, case_id, title, description, source, status, severity, tlp, pap, created_by, assigned_to, created_at, updated_at
	`
	rows, err := r.pool.Query(ctx, q, tenantID, alertIDs, caseID)
	if err != nil {
		return nil, fmt.Errorf("bind alerts to case: %w", err)
	}
	defer rows.Close()
	return scanAlertRows(rows)
}

func (r *AlertRepository) Update(ctx context.Context, tenantID, alertID uuid.UUID, p UpdateAlertParams) (*models.Alert, error) {
	q := `
		UPDATE alerts
		SET
			case_id = COALESCE($3, case_id),
			title = COALESCE($4, title),
			description = COALESCE($5, description),
			source = COALESCE($6, source),
			status = COALESCE($7, status),
			severity = COALESCE($8, severity),
			tlp = COALESCE($9, tlp),
			pap = COALESCE($10, pap),
			assigned_to = COALESCE($11, assigned_to),
			updated_at = NOW()
		WHERE tenant_id = $1 AND id = $2
		RETURNING id, tenant_id, case_id, title, description, source, status, severity, tlp, pap, created_by, assigned_to, created_at, updated_at
	`
	item, err := scanAlert(
		r.pool.QueryRow(ctx, q,
			tenantID,
			alertID,
			p.CaseID,
			p.Title,
			p.Description,
			p.Source,
			p.Status,
			p.Severity,
			p.TLP,
			p.PAP,
			p.AssignedTo,
		),
	)
	if err != nil {
		return nil, fmt.Errorf("update alert: %w", err)
	}
	return item, nil
}

type alertScanner interface {
	Scan(dest ...any) error
}

func scanAlert(scanner alertScanner) (*models.Alert, error) {
	var item models.Alert
	if err := scanner.Scan(
		&item.ID,
		&item.TenantID,
		&item.CaseID,
		&item.Title,
		&item.Description,
		&item.Source,
		&item.Status,
		&item.Severity,
		&item.TLP,
		&item.PAP,
		&item.CreatedBy,
		&item.AssignedTo,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &item, nil
}

func scanAlertRows(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]models.Alert, error) {
	out := make([]models.Alert, 0)
	for rows.Next() {
		item, err := scanAlert(rows)
		if err != nil {
			return nil, fmt.Errorf("scan alerts: %w", err)
		}
		out = append(out, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate alerts: %w", err)
	}
	return out, nil
}
