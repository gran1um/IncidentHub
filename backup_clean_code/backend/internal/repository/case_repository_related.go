package repository

import (
	"context"
	"fmt"
	"incidenthub/backend/internal/models"
	"strings"

	"github.com/google/uuid"
)

const relatedCaseObservablePairSeparator = "\x1f"

type RelatedObservable struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type RelatedField struct {
	Field string `json:"field"`
	Value string `json:"value"`
}

type RelatedCase struct {
	Case               models.Case         `json:"case"`
	MatchCount         int                 `json:"match_count"`
	MatchedObservables []RelatedObservable `json:"matched_observables"`
	MatchedFields      []RelatedField      `json:"matched_fields"`
}

func (r *CaseRepository) ListRelatedCasesByObservables(
	ctx context.Context,
	requestedTenantID uuid.UUID,
	caseID uuid.UUID,
	activeRecentOnly bool,
	limit int,
) ([]RelatedCase, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	activeRecentClause := ""
	if activeRecentOnly {
		activeRecentClause = `
			AND (
				LOWER(TRIM(c.status)) NOT IN ('closed', 'resolved')
				OR c.updated_at >= NOW() - INTERVAL '30 days'
			)
		`
	}

	q := fmt.Sprintf(`
		WITH source_observables AS (
			SELECT DISTINCT LOWER(type) AS type_norm, LOWER(value) AS value_norm
			FROM case_observables
			WHERE case_id = $1
		),
		matches AS (
			SELECT
				co.case_id,
				COUNT(*)::int AS match_count,
				ARRAY_AGG(co.type || E'\x1f' || co.value ORDER BY co.type, co.value) AS matched_pairs
			FROM case_observables co
			INNER JOIN source_observables so
				ON LOWER(co.type) = so.type_norm
				AND LOWER(co.value) = so.value_norm
			WHERE co.case_id <> $1
			GROUP BY co.case_id
		)
		SELECT
			c.id, c.tenant_id, COALESCE(c.case_number, ''), c.title, c.description, c.source, c.incident_type, c.status, c.priority, c.impact, c.confidence,
			c.severity, c.tlp, c.pap, c.detected_at, c.occurred_at, c.closed_at, c.resolution_summary, c.created_by, c.assigned_to, c.created_at, c.updated_at,
			m.match_count, m.matched_pairs
		FROM matches m
		INNER JOIN cases c ON c.id = m.case_id
		WHERE (
			c.tenant_id = $2
			OR EXISTS (
				SELECT 1
				FROM case_tenant_shares cs
				WHERE cs.case_id = c.id
				  AND cs.shared_tenant_id = $2
			)
		)%s
		ORDER BY m.match_count DESC, c.updated_at DESC
		LIMIT $3
	`, activeRecentClause)

	rows, err := r.pool.Query(ctx, q, caseID, requestedTenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("list related cases by observables: %w", err)
	}
	defer rows.Close()

	out := make([]RelatedCase, 0, limit)
	for rows.Next() {
		var (
			item         RelatedCase
			matchedPairs []string
		)
		if err := rows.Scan(
			&item.Case.ID,
			&item.Case.TenantID,
			&item.Case.CaseNumber,
			&item.Case.Title,
			&item.Case.Description,
			&item.Case.Source,
			&item.Case.IncidentType,
			&item.Case.Status,
			&item.Case.Priority,
			&item.Case.Impact,
			&item.Case.Confidence,
			&item.Case.Severity,
			&item.Case.TLP,
			&item.Case.PAP,
			&item.Case.DetectedAt,
			&item.Case.OccurredAt,
			&item.Case.ClosedAt,
			&item.Case.ResolutionSummary,
			&item.Case.CreatedBy,
			&item.Case.AssignedTo,
			&item.Case.CreatedAt,
			&item.Case.UpdatedAt,
			&item.MatchCount,
			&matchedPairs,
		); err != nil {
			return nil, fmt.Errorf("scan related case row: %w", err)
		}

		item.MatchedObservables = make([]RelatedObservable, 0, len(matchedPairs))
		for _, pair := range matchedPairs {
			parts := strings.SplitN(pair, relatedCaseObservablePairSeparator, 2)
			if len(parts) != 2 {
				continue
			}
			item.MatchedObservables = append(item.MatchedObservables, RelatedObservable{
				Type:  parts[0],
				Value: parts[1],
			})
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate related case rows: %w", err)
	}
	return out, nil
}

func (r *CaseRepository) ListRelatedCasesByField(
	ctx context.Context,
	requestedTenantID uuid.UUID,
	caseID uuid.UUID,
	linkBy string,
	activeRecentOnly bool,
	limit int,
) ([]RelatedCase, error) {
	fieldColumn, ok := resolveRelatedCaseFieldColumn(linkBy)
	if !ok {
		return nil, fmt.Errorf("unsupported link field %q", linkBy)
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	activeRecentClause := ""
	if activeRecentOnly {
		activeRecentClause = `
			AND (
				LOWER(TRIM(c.status)) NOT IN ('closed', 'resolved')
				OR c.updated_at >= NOW() - INTERVAL '30 days'
			)
		`
	}

	q := fmt.Sprintf(`
		WITH source_case AS (
			SELECT LOWER(TRIM(%s)) AS match_value
			FROM cases
			WHERE id = $1
		)
		SELECT
			c.id, c.tenant_id, COALESCE(c.case_number, ''), c.title, c.description, c.source, c.incident_type, c.status, c.priority, c.impact, c.confidence,
			c.severity, c.tlp, c.pap, c.detected_at, c.occurred_at, c.closed_at, c.resolution_summary, c.created_by, c.assigned_to, c.created_at, c.updated_at,
			sc.match_value
		FROM source_case sc
		INNER JOIN cases c
			ON c.id <> $1
			AND LOWER(TRIM(c.%s)) = sc.match_value
		WHERE sc.match_value <> ''
			AND (
				c.tenant_id = $2
				OR EXISTS (
					SELECT 1
					FROM case_tenant_shares cs
					WHERE cs.case_id = c.id
					  AND cs.shared_tenant_id = $2
				)
			)%s
		ORDER BY c.updated_at DESC
		LIMIT $3
	`, fieldColumn, fieldColumn, activeRecentClause)

	rows, err := r.pool.Query(ctx, q, caseID, requestedTenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("list related cases by field: %w", err)
	}
	defer rows.Close()

	out := make([]RelatedCase, 0, limit)
	for rows.Next() {
		var (
			item       RelatedCase
			matchValue string
		)
		if err := rows.Scan(
			&item.Case.ID,
			&item.Case.TenantID,
			&item.Case.CaseNumber,
			&item.Case.Title,
			&item.Case.Description,
			&item.Case.Source,
			&item.Case.IncidentType,
			&item.Case.Status,
			&item.Case.Priority,
			&item.Case.Impact,
			&item.Case.Confidence,
			&item.Case.Severity,
			&item.Case.TLP,
			&item.Case.PAP,
			&item.Case.DetectedAt,
			&item.Case.OccurredAt,
			&item.Case.ClosedAt,
			&item.Case.ResolutionSummary,
			&item.Case.CreatedBy,
			&item.Case.AssignedTo,
			&item.Case.CreatedAt,
			&item.Case.UpdatedAt,
			&matchValue,
		); err != nil {
			return nil, fmt.Errorf("scan related field case row: %w", err)
		}
		item.MatchCount = 1
		item.MatchedFields = []RelatedField{
			{
				Field: linkBy,
				Value: matchValue,
			},
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate related field case rows: %w", err)
	}
	return out, nil
}

func resolveRelatedCaseFieldColumn(linkBy string) (string, bool) {
	switch strings.TrimSpace(strings.ToLower(linkBy)) {
	case "incident_type":
		return "incident_type", true
	case "source":
		return "source", true
	case "severity":
		return "severity", true
	case "priority":
		return "priority", true
	default:
		return "", false
	}
}
