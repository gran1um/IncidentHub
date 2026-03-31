package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

//nolint:gochecknoglobals // Default closed statuses fallback used by stats queries.
var defaultClosedCaseStatuses = []string{"closed", "resolved", "done", "false_positive"}

func (r *CaseRepository) CountInWorkByTenant(ctx context.Context, tenantID uuid.UUID, openStatuses []string) (int, error) {
	normalizedOpen := normalizeStatusCodes(openStatuses)

	var count int
	if len(normalizedOpen) > 0 {
		q := `
			SELECT COUNT(*)
			FROM cases
			WHERE tenant_id = $1
			  AND LOWER(status) = ANY($2::text[])
		`
		if err := r.pool.QueryRow(ctx, q, tenantID, normalizedOpen).Scan(&count); err != nil {
			return 0, fmt.Errorf("count in-work cases by open statuses: %w", err)
		}
		return count, nil
	}

	closedFallback := normalizeStatusCodes(defaultClosedCaseStatuses)
	q := `
		SELECT COUNT(*)
		FROM cases
		WHERE tenant_id = $1
		  AND NOT (LOWER(status) = ANY($2::text[]))
	`
	if err := r.pool.QueryRow(ctx, q, tenantID, closedFallback).Scan(&count); err != nil {
		return 0, fmt.Errorf("count in-work cases by fallback statuses: %w", err)
	}
	return count, nil
}

func (r *CaseRepository) CountOpenBySeverity(ctx context.Context, tenantID uuid.UUID, severities []string, openStatuses []string) (int, error) {
	normalizedSeverities := normalizeStatusCodes(severities)
	if len(normalizedSeverities) == 0 {
		normalizedSeverities = []string{"critical"}
	}
	normalizedOpen := normalizeStatusCodes(openStatuses)

	var count int
	if len(normalizedOpen) > 0 {
		q := `
			SELECT COUNT(*)
			FROM cases
			WHERE tenant_id = $1
			  AND LOWER(COALESCE(severity, '')) = ANY($2::text[])
			  AND LOWER(COALESCE(status, '')) = ANY($3::text[])
		`
		if err := r.pool.QueryRow(ctx, q, tenantID, normalizedSeverities, normalizedOpen).Scan(&count); err != nil {
			return 0, fmt.Errorf("count open cases by severity using open statuses: %w", err)
		}
		return count, nil
	}

	closedFallback := normalizeStatusCodes(defaultClosedCaseStatuses)
	q := `
		SELECT COUNT(*)
		FROM cases
		WHERE tenant_id = $1
		  AND LOWER(COALESCE(severity, '')) = ANY($2::text[])
		  AND NOT (LOWER(COALESCE(status, '')) = ANY($3::text[]))
	`
	if err := r.pool.QueryRow(ctx, q, tenantID, normalizedSeverities, closedFallback).Scan(&count); err != nil {
		return 0, fmt.Errorf("count open cases by severity using closed fallback: %w", err)
	}
	return count, nil
}

func (r *CaseRepository) countOpenCasesBeforeTimeField(
	ctx context.Context,
	tenantID uuid.UUID,
	cutoff time.Time,
	openStatuses []string,
	timeField string,
	openStatusesErrContext string,
	closedFallbackErrContext string,
) (int, error) {
	normalizedOpen := normalizeStatusCodes(openStatuses)

	var count int
	if len(normalizedOpen) > 0 {
		q := fmt.Sprintf(`
			SELECT COUNT(*)
			FROM cases
			WHERE tenant_id = $1
			  AND %s <= $2
			  AND LOWER(COALESCE(status, '')) = ANY($3::text[])
		`, timeField)
		if err := r.pool.QueryRow(ctx, q, tenantID, cutoff, normalizedOpen).Scan(&count); err != nil {
			return 0, fmt.Errorf("%s: %w", openStatusesErrContext, err)
		}
		return count, nil
	}

	closedFallback := normalizeStatusCodes(defaultClosedCaseStatuses)
	q := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM cases
		WHERE tenant_id = $1
		  AND %s <= $2
		  AND NOT (LOWER(COALESCE(status, '')) = ANY($3::text[]))
	`, timeField)
	if err := r.pool.QueryRow(ctx, q, tenantID, cutoff, closedFallback).Scan(&count); err != nil {
		return 0, fmt.Errorf("%s: %w", closedFallbackErrContext, err)
	}
	return count, nil
}

func (r *CaseRepository) CountOpenCreatedBefore(ctx context.Context, tenantID uuid.UUID, before time.Time, openStatuses []string) (int, error) {
	return r.countOpenCasesBeforeTimeField(
		ctx,
		tenantID,
		before,
		openStatuses,
		"created_at",
		"count open stale cases using open statuses",
		"count open stale cases using closed fallback",
	)
}

func (r *CaseRepository) CountOpenInactiveSince(ctx context.Context, tenantID uuid.UUID, since time.Time, openStatuses []string) (int, error) {
	return r.countOpenCasesBeforeTimeField(
		ctx,
		tenantID,
		since,
		openStatuses,
		"updated_at",
		"count inactive open cases using open statuses",
		"count inactive open cases using closed fallback",
	)
}

func normalizeStatusCodes(statuses []string) []string {
	if len(statuses) == 0 {
		return nil
	}
	out := make([]string, 0, len(statuses))
	seen := make(map[string]struct{}, len(statuses))
	for _, status := range statuses {
		normalized := strings.ToLower(strings.TrimSpace(status))
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}
