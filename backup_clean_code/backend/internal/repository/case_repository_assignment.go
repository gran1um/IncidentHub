package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *CaseRepository) CountOpenByAssignees(
	ctx context.Context,
	tenantID uuid.UUID,
	assigneeIDs []uuid.UUID,
	openStatuses []string,
) (map[uuid.UUID]int, error) {
	result := make(map[uuid.UUID]int, len(assigneeIDs))
	if len(assigneeIDs) == 0 {
		return result, nil
	}
	for _, assigneeID := range assigneeIDs {
		result[assigneeID] = 0
	}

	normalizedOpen := normalizeStatusCodes(openStatuses)
	if len(normalizedOpen) > 0 {
		q := `
			SELECT assigned_to, COUNT(*)
			FROM cases
			WHERE tenant_id = $1
			  AND assigned_to = ANY($2::uuid[])
			  AND LOWER(status) = ANY($3::text[])
			GROUP BY assigned_to
		`
		rows, err := r.pool.Query(ctx, q, tenantID, assigneeIDs, normalizedOpen)
		if err != nil {
			return nil, fmt.Errorf("count open cases by assignee using open statuses: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var assigneeID uuid.UUID
			var count int
			if err := rows.Scan(&assigneeID, &count); err != nil {
				return nil, fmt.Errorf("scan open case workload row: %w", err)
			}
			result[assigneeID] = count
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("iterate open case workload rows: %w", err)
		}
		return result, nil
	}

	closedFallback := normalizeStatusCodes(defaultClosedCaseStatuses)
	q := `
		SELECT assigned_to, COUNT(*)
		FROM cases
		WHERE tenant_id = $1
		  AND assigned_to = ANY($2::uuid[])
		  AND NOT (LOWER(status) = ANY($3::text[]))
		GROUP BY assigned_to
	`
	rows, err := r.pool.Query(ctx, q, tenantID, assigneeIDs, closedFallback)
	if err != nil {
		return nil, fmt.Errorf("count open cases by assignee using closed fallback: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var assigneeID uuid.UUID
		var count int
		if err := rows.Scan(&assigneeID, &count); err != nil {
			return nil, fmt.Errorf("scan fallback open case workload row: %w", err)
		}
		result[assigneeID] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate fallback open case workload rows: %w", err)
	}
	return result, nil
}

func (r *CaseRepository) FindOpenSimilarAssignee(
	ctx context.Context,
	tenantID uuid.UUID,
	assigneeIDs []uuid.UUID,
	source string,
	incidentType string,
	openStatuses []string,
) (*uuid.UUID, error) {
	if len(assigneeIDs) == 0 {
		return nil, nil
	}

	normalizedSource := strings.ToLower(strings.TrimSpace(source))
	normalizedIncidentType := strings.ToLower(strings.TrimSpace(incidentType))
	if normalizedSource == "" && normalizedIncidentType == "" {
		return nil, nil
	}

	args := make([]any, 0, 6)
	args = append(args, tenantID, assigneeIDs)
	statusPos := 3
	args = append(args, normalizeStatusCodes(openStatuses))
	where := "tenant_id = $1 AND assigned_to = ANY($2::uuid[])"

	normalizedOpen := normalizeStatusCodes(openStatuses)
	if len(normalizedOpen) > 0 {
		where += fmt.Sprintf(" AND LOWER(status) = ANY($%d::text[])", statusPos)
	} else {
		args[statusPos-1] = normalizeStatusCodes(defaultClosedCaseStatuses)
		where += fmt.Sprintf(" AND NOT (LOWER(status) = ANY($%d::text[]))", statusPos)
	}

	nextPos := statusPos + 1
	if normalizedSource != "" {
		args = append(args, normalizedSource)
		where += fmt.Sprintf(" AND LOWER(COALESCE(source, '')) = $%d", nextPos)
		nextPos++
	}
	if normalizedIncidentType != "" {
		args = append(args, normalizedIncidentType)
		where += fmt.Sprintf(" AND LOWER(COALESCE(incident_type, '')) = $%d", nextPos)
	}

	q := fmt.Sprintf(`
		SELECT assigned_to
		FROM cases
		WHERE %s
		ORDER BY updated_at DESC
		LIMIT 1
	`, where)

	var assigneeID uuid.UUID
	if err := r.pool.QueryRow(ctx, q, args...).Scan(&assigneeID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find similar open case assignee: %w", err)
	}
	return &assigneeID, nil
}
