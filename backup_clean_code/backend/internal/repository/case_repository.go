package repository

import (
	"context"
	"fmt"
	"incidenthub/backend/internal/models"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CaseRepository struct {
	pool *pgxpool.Pool
}

var caseCustomSortKeyPattern = regexp.MustCompile(`^[a-z0-9_][a-z0-9_:\-]{0,63}$`)

//nolint:gochecknoglobals // Static search fields used in SQL query composition.
var caseListSearchFields = []string{
	"COALESCE(case_number, '')",
	"title",
	"description",
	"source",
	"status",
	"severity",
	"incident_type",
	"COALESCE(impact, '')",
}

type CreateCaseParams struct {
	ID                *uuid.UUID
	TenantID          uuid.UUID
	CaseNumber        string
	Title             string
	Description       string
	Source            string
	IncidentType      string
	Status            string
	Priority          string
	Impact            string
	Confidence        int
	Severity          string
	TLP               string
	PAP               string
	DetectedAt        *string
	OccurredAt        *string
	ClosedAt          *string
	ResolutionSummary string
	CreatedBy         uuid.UUID
	AssignedTo        *uuid.UUID
}

type UpdateCaseParams struct {
	CaseNumber        *string
	Title             *string
	Description       *string
	Source            *string
	IncidentType      *string
	Status            *string
	Priority          *string
	Impact            *string
	Confidence        *int
	Severity          *string
	TLP               *string
	PAP               *string
	DetectedAt        *string
	OccurredAt        *string
	ClosedAt          *string
	ResolutionSummary *string
	AssignedTo        *uuid.UUID
	ClearAssignedTo   bool
}

func NewCaseRepository(pool *pgxpool.Pool) *CaseRepository {
	return &CaseRepository{pool: pool}
}

func (r *CaseRepository) Create(ctx context.Context, p CreateCaseParams) (*models.Case, error) {
	q := `
		INSERT INTO cases(
			id, tenant_id, case_number, title, description, source, incident_type, status, priority, impact, confidence,
			severity, tlp, pap, detected_at, occurred_at, closed_at, resolution_summary, created_by, assigned_to
		)
		VALUES (COALESCE($1, gen_random_uuid()),$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15::timestamptz,$16::timestamptz,$17::timestamptz,$18,$19,$20)
		RETURNING
			id, tenant_id, case_number, title, description, source, incident_type, status, priority, impact, confidence,
			severity, tlp, pap, detected_at, occurred_at, closed_at, resolution_summary, created_by, assigned_to, created_at, updated_at
	`
	var c models.Case
	if err := r.pool.QueryRow(ctx, q,
		p.ID,
		p.TenantID,
		p.CaseNumber,
		p.Title,
		p.Description,
		p.Source,
		p.IncidentType,
		p.Status,
		p.Priority,
		p.Impact,
		p.Confidence,
		p.Severity,
		p.TLP,
		p.PAP,
		p.DetectedAt,
		p.OccurredAt,
		p.ClosedAt,
		p.ResolutionSummary,
		p.CreatedBy,
		p.AssignedTo,
	).Scan(
		&c.ID,
		&c.TenantID,
		&c.CaseNumber,
		&c.Title,
		&c.Description,
		&c.Source,
		&c.IncidentType,
		&c.Status,
		&c.Priority,
		&c.Impact,
		&c.Confidence,
		&c.Severity,
		&c.TLP,
		&c.PAP,
		&c.DetectedAt,
		&c.OccurredAt,
		&c.ClosedAt,
		&c.ResolutionSummary,
		&c.CreatedBy,
		&c.AssignedTo,
		&c.CreatedAt,
		&c.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("create case: %w", err)
	}
	return &c, nil
}

func (r *CaseRepository) CreateWithLinkedAlerts(ctx context.Context, p CreateCaseParams, alertIDs []uuid.UUID) (*models.Case, []models.Alert, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("begin case+alerts transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	createdCase, err := createCaseTx(ctx, tx, p)
	if err != nil {
		return nil, nil, err
	}

	if len(alertIDs) == 0 {
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return nil, nil, fmt.Errorf("commit case transaction: %w", commitErr)
		}
		committed = true
		return createdCase, []models.Alert{}, nil
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
	rows, err := tx.Query(ctx, q, p.TenantID, alertIDs, createdCase.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("bind alerts to created case: %w", err)
	}
	defer rows.Close()

	updatedAlerts, err := scanAlertRows(rows)
	if err != nil {
		return nil, nil, err
	}
	if len(updatedAlerts) != len(alertIDs) {
		return nil, nil, fmt.Errorf("bind alerts to created case: expected %d updates, got %d", len(alertIDs), len(updatedAlerts))
	}

	if commitErr := tx.Commit(ctx); commitErr != nil {
		return nil, nil, fmt.Errorf("commit case+alerts transaction: %w", commitErr)
	}
	committed = true
	return createdCase, updatedAlerts, nil
}

func createCaseTx(ctx context.Context, tx pgx.Tx, p CreateCaseParams) (*models.Case, error) {
	q := `
		INSERT INTO cases(
			id, tenant_id, case_number, title, description, source, incident_type, status, priority, impact, confidence,
			severity, tlp, pap, detected_at, occurred_at, closed_at, resolution_summary, created_by, assigned_to
		)
		VALUES (COALESCE($1, gen_random_uuid()),$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15::timestamptz,$16::timestamptz,$17::timestamptz,$18,$19,$20)
		RETURNING
			id, tenant_id, case_number, title, description, source, incident_type, status, priority, impact, confidence,
			severity, tlp, pap, detected_at, occurred_at, closed_at, resolution_summary, created_by, assigned_to, created_at, updated_at
	`
	var c models.Case
	if err := tx.QueryRow(ctx, q,
		p.ID,
		p.TenantID,
		p.CaseNumber,
		p.Title,
		p.Description,
		p.Source,
		p.IncidentType,
		p.Status,
		p.Priority,
		p.Impact,
		p.Confidence,
		p.Severity,
		p.TLP,
		p.PAP,
		p.DetectedAt,
		p.OccurredAt,
		p.ClosedAt,
		p.ResolutionSummary,
		p.CreatedBy,
		p.AssignedTo,
	).Scan(
		&c.ID,
		&c.TenantID,
		&c.CaseNumber,
		&c.Title,
		&c.Description,
		&c.Source,
		&c.IncidentType,
		&c.Status,
		&c.Priority,
		&c.Impact,
		&c.Confidence,
		&c.Severity,
		&c.TLP,
		&c.PAP,
		&c.DetectedAt,
		&c.OccurredAt,
		&c.ClosedAt,
		&c.ResolutionSummary,
		&c.CreatedBy,
		&c.AssignedTo,
		&c.CreatedAt,
		&c.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("create case: %w", err)
	}
	return &c, nil
}

func (r *CaseRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]models.Case, error) {
	return r.ListByTenantWithAssignedSorted(ctx, tenantID, limit, offset, "all", nil, "updated_at", "desc")
}

func (r *CaseRepository) ListByTenantWithAssigned(
	ctx context.Context,
	tenantID uuid.UUID,
	limit,
	offset int,
	assigned string,
	assignedTo *uuid.UUID,
) ([]models.Case, error) {
	return r.ListByTenantWithAssignedSorted(ctx, tenantID, limit, offset, assigned, assignedTo, "updated_at", "desc")
}

func (r *CaseRepository) ListByTenantWithAssignedSorted(
	ctx context.Context,
	tenantID uuid.UUID,
	limit,
	offset int,
	assigned string,
	assignedTo *uuid.UUID,
	sortBy,
	sortOrder string,
) ([]models.Case, error) {
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

func (r *CaseRepository) ListByTenantWithAssignedSortedAndSearch(
	ctx context.Context,
	tenantID uuid.UUID,
	limit,
	offset int,
	assigned string,
	assignedTo *uuid.UUID,
	sortBy,
	sortOrder string,
	search ListSearchParams,
) ([]models.Case, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	normalizedSearch := normalizeListSearchParams(search)
	whereClause, whereArgs := buildAssignedFilterClause(assigned, assignedTo)
	args := make([]any, 0, 4)
	args = append(args, tenantID)
	args = append(args, whereArgs...)
	searchClause, searchArgs := buildListSearchWhereClause(caseListSearchFields, normalizedSearch, len(args)+1)
	args = append(args, searchArgs...)
	tagClause, tagArgs := buildCaseMetaTagsFilterClause(normalizedSearch.CaseMetaTagsAny, len(args)+1)
	args = append(args, tagArgs...)
	limitPos := len(args) + 1
	offsetPos := len(args) + 2
	args = append(args, limit, offset)
	orderClause := buildCaseOrderClause(sortBy, sortOrder)
	q := fmt.Sprintf(`
		SELECT
			id, tenant_id, COALESCE(case_number, ''), title, description, source, incident_type, status, priority, impact, confidence,
			severity, tlp, pap, detected_at, occurred_at, closed_at, resolution_summary, created_by, assigned_to, created_at, updated_at
		FROM cases
			WHERE (
				tenant_id = $1
				OR EXISTS (
				SELECT 1
				FROM case_tenant_shares cs
					WHERE cs.case_id = cases.id
					  AND cs.shared_tenant_id = $1
				)
			)%s%s%s
			ORDER BY %s
			LIMIT $%d OFFSET $%d
		`, whereClause, searchClause, tagClause, orderClause, limitPos, offsetPos)
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list cases: %w", err)
	}
	defer rows.Close()

	out := make([]models.Case, 0, limit)
	for rows.Next() {
		var c models.Case
		if err := rows.Scan(
			&c.ID,
			&c.TenantID,
			&c.CaseNumber,
			&c.Title,
			&c.Description,
			&c.Source,
			&c.IncidentType,
			&c.Status,
			&c.Priority,
			&c.Impact,
			&c.Confidence,
			&c.Severity,
			&c.TLP,
			&c.PAP,
			&c.DetectedAt,
			&c.OccurredAt,
			&c.ClosedAt,
			&c.ResolutionSummary,
			&c.CreatedBy,
			&c.AssignedTo,
			&c.CreatedAt,
			&c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan cases: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cases: %w", err)
	}
	return out, nil
}

func buildCaseOrderClause(sortBy, sortOrder string) string {
	if customFieldKey, ok := parseCaseCustomSortKey(sortBy); ok {
		order := "DESC"
		if strings.EqualFold(strings.TrimSpace(sortOrder), "asc") {
			order = "ASC"
		}
		escapedKey := strings.ReplaceAll(customFieldKey, "'", "''")
		return fmt.Sprintf(`
			COALESCE((
				SELECT LOWER(COALESCE(
					cm.data->'custom_fields'->>'%s',
					cm.data->'customFields'->>'%s',
					''
				))
				FROM catalog_items cm
				WHERE cm.kind = 'case_meta'
				  AND cm.ref_id = cases.id
				  AND cm.tenant_id = cases.tenant_id
				ORDER BY cm.updated_at DESC
				LIMIT 1
			), '') %s
		`, escapedKey, escapedKey, order)
	}

	column := "updated_at"
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "created_at":
		column = "created_at"
	case "severity":
		column = "severity"
	case "status":
		column = "status"
	case "title":
		column = "title"
	case "case_number":
		column = "COALESCE(case_number, '')"
	case "updated_at":
		column = "updated_at"
	}

	order := "DESC"
	if strings.EqualFold(strings.TrimSpace(sortOrder), "asc") {
		order = "ASC"
	}
	return column + " " + order
}

func parseCaseCustomSortKey(sortBy string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(sortBy))
	if !strings.HasPrefix(normalized, "cf:") {
		return "", false
	}
	key := strings.TrimSpace(strings.TrimPrefix(normalized, "cf:"))
	if !caseCustomSortKeyPattern.MatchString(key) {
		return "", false
	}
	return key, true
}

func buildCaseMetaTagsFilterClause(tags []string, startPos int) (clause string, args []any) {
	normalized := normalizeListSearchTags(tags)
	if len(normalized) == 0 {
		return "", nil
	}
	return fmt.Sprintf(`
		AND EXISTS (
			SELECT 1
			FROM catalog_items cm
			WHERE cm.kind = 'case_meta'
			  AND cm.ref_id = cases.id
			  AND cm.tenant_id = cases.tenant_id
			  AND EXISTS (
				SELECT 1
				FROM jsonb_array_elements_text(COALESCE(cm.data->'tags', '[]'::jsonb)) AS t(value)
				WHERE LOWER(t.value) = ANY($%d::text[])
			  )
		)
	`, startPos), []any{normalized}
}

func (r *CaseRepository) CountByTenant(ctx context.Context, tenantID uuid.UUID) (int, error) {
	return r.CountByTenantWithAssigned(ctx, tenantID, "all", nil)
}

func (r *CaseRepository) CountByTenantWithAssigned(
	ctx context.Context,
	tenantID uuid.UUID,
	assigned string,
	assignedTo *uuid.UUID,
) (int, error) {
	return r.CountByTenantWithAssignedAndSearch(ctx, tenantID, assigned, assignedTo, ListSearchParams{})
}

func (r *CaseRepository) CountByTenantWithAssignedAndSearch(
	ctx context.Context,
	tenantID uuid.UUID,
	assigned string,
	assignedTo *uuid.UUID,
	search ListSearchParams,
) (int, error) {
	var total int
	normalizedSearch := normalizeListSearchParams(search)
	whereClause, whereArgs := buildAssignedFilterClause(assigned, assignedTo)
	args := make([]any, 0, 1+len(whereArgs)+2)
	args = append(args, tenantID)
	args = append(args, whereArgs...)
	searchClause, searchArgs := buildListSearchWhereClause(caseListSearchFields, normalizedSearch, len(args)+1)
	args = append(args, searchArgs...)
	tagClause, tagArgs := buildCaseMetaTagsFilterClause(normalizedSearch.CaseMetaTagsAny, len(args)+1)
	args = append(args, tagArgs...)
	q := fmt.Sprintf(`
			SELECT COUNT(*)
			FROM cases
		WHERE (
			tenant_id = $1
			OR EXISTS (
				SELECT 1
				FROM case_tenant_shares cs
					WHERE cs.case_id = cases.id
					  AND cs.shared_tenant_id = $1
				)
			)%s%s%s
		`, whereClause, searchClause, tagClause)
	if err := r.pool.QueryRow(ctx, q, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("count cases: %w", err)
	}
	return total, nil
}

func (r *CaseRepository) ExistsInTenant(ctx context.Context, caseID, tenantID uuid.UUID) (bool, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
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
		)
	`, caseID, tenantID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check case tenant access: %w", err)
	}
	return exists, nil
}

func (r *CaseRepository) GetByID(ctx context.Context, tenantID, caseID uuid.UUID) (*models.Case, error) {
	q := `
		SELECT
			id, tenant_id, COALESCE(case_number, ''), title, description, source, incident_type, status, priority, impact, confidence,
			severity, tlp, pap, detected_at, occurred_at, closed_at, resolution_summary, created_by, assigned_to, created_at, updated_at
		FROM cases
		WHERE tenant_id=$1 AND id=$2
	`
	var c models.Case
	if err := r.pool.QueryRow(ctx, q, tenantID, caseID).Scan(
		&c.ID,
		&c.TenantID,
		&c.CaseNumber,
		&c.Title,
		&c.Description,
		&c.Source,
		&c.IncidentType,
		&c.Status,
		&c.Priority,
		&c.Impact,
		&c.Confidence,
		&c.Severity,
		&c.TLP,
		&c.PAP,
		&c.DetectedAt,
		&c.OccurredAt,
		&c.ClosedAt,
		&c.ResolutionSummary,
		&c.CreatedBy,
		&c.AssignedTo,
		&c.CreatedAt,
		&c.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("get case: %w", err)
	}
	return &c, nil
}

func (r *CaseRepository) Update(ctx context.Context, tenantID, caseID uuid.UUID, p UpdateCaseParams) (*models.Case, error) {
	q := `
		UPDATE cases
		SET
			case_number = COALESCE($3, case_number),
			title = COALESCE($4, title),
			description = COALESCE($5, description),
			source = COALESCE($6, source),
			incident_type = COALESCE($7, incident_type),
			status = COALESCE($8, status),
			priority = COALESCE($9, priority),
			impact = COALESCE($10, impact),
			confidence = COALESCE($11, confidence),
			severity = COALESCE($12, severity),
			tlp = COALESCE($13, tlp),
			pap = COALESCE($14, pap),
			detected_at = COALESCE($15::timestamptz, detected_at),
			occurred_at = COALESCE($16::timestamptz, occurred_at),
			closed_at = COALESCE($17::timestamptz, closed_at),
			resolution_summary = COALESCE($18, resolution_summary),
			assigned_to = CASE WHEN $20 THEN NULL ELSE COALESCE($19, assigned_to) END,
			updated_at = NOW()
		WHERE tenant_id = $1 AND id = $2
		RETURNING
			id, tenant_id, COALESCE(case_number, ''), title, description, source, incident_type, status, priority, impact, confidence,
			severity, tlp, pap, detected_at, occurred_at, closed_at, resolution_summary, created_by, assigned_to, created_at, updated_at
	`
	var c models.Case
	if err := r.pool.QueryRow(ctx, q,
		tenantID,
		caseID,
		p.CaseNumber,
		p.Title,
		p.Description,
		p.Source,
		p.IncidentType,
		p.Status,
		p.Priority,
		p.Impact,
		p.Confidence,
		p.Severity,
		p.TLP,
		p.PAP,
		p.DetectedAt,
		p.OccurredAt,
		p.ClosedAt,
		p.ResolutionSummary,
		p.AssignedTo,
		p.ClearAssignedTo,
	).Scan(
		&c.ID,
		&c.TenantID,
		&c.CaseNumber,
		&c.Title,
		&c.Description,
		&c.Source,
		&c.IncidentType,
		&c.Status,
		&c.Priority,
		&c.Impact,
		&c.Confidence,
		&c.Severity,
		&c.TLP,
		&c.PAP,
		&c.DetectedAt,
		&c.OccurredAt,
		&c.ClosedAt,
		&c.ResolutionSummary,
		&c.CreatedBy,
		&c.AssignedTo,
		&c.CreatedAt,
		&c.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("update case: %w", err)
	}
	return &c, nil
}

func (r *CaseRepository) Delete(ctx context.Context, tenantID, caseID uuid.UUID) (bool, error) {
	result, err := r.pool.Exec(ctx, `DELETE FROM cases WHERE tenant_id = $1 AND id = $2`, tenantID, caseID)
	if err != nil {
		return false, fmt.Errorf("delete case: %w", err)
	}
	return result.RowsAffected() > 0, nil
}
