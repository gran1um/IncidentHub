package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

//nolint:gochecknoglobals // Static SLA targets by severity.
var dashboardSeverityTargetsMinutes = map[string]int{
	"critical": 60,
	"high":     240,
	"medium":   720,
	"low":      1440,
}

type DashboardCountBucket struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

type DashboardSLABySeverity struct {
	Severity             string  `json:"severity"`
	TargetMinutes        int     `json:"targetMinutes"`
	OpenCases            int     `json:"openCases"`
	ResolvedCases        int     `json:"resolvedCases"`
	AvgResolutionMinutes float64 `json:"avgResolutionMinutes"`
	BreachedCases        int     `json:"breachedCases"`
}

type DashboardAnalystResolved struct {
	UserID        string `json:"userId"`
	Username      string `json:"username"`
	DisplayName   string `json:"displayName"`
	ResolvedCases int    `json:"resolvedCases"`
}

type DashboardMetricsSnapshot struct {
	OpenCases         int                        `json:"openCases"`
	OpenAlerts        int                        `json:"openAlerts"`
	OverdueCases      int                        `json:"overdueCases"`
	CaseStatus        []DashboardCountBucket     `json:"caseStatus"`
	CaseCategory      []DashboardCountBucket     `json:"caseCategory"`
	AlertStatus       []DashboardCountBucket     `json:"alertStatus"`
	AlertCategory     []DashboardCountBucket     `json:"alertCategory"`
	SLABySeverity     []DashboardSLABySeverity   `json:"slaBySeverity"`
	ResolvedByAnalyst []DashboardAnalystResolved `json:"resolvedByAnalyst"`
}

type DashboardCustomMetricDefinition struct {
	Source            string
	Measure           string
	Statuses          []string
	Severities        []string
	Categories        []string
	CreatedFrom       *time.Time
	CreatedTo         *time.Time
	ClosedFrom        *time.Time
	ClosedTo          *time.Time
	CreatedWithinHour int
	ClosedWithinHour  int
	OverdueMinutes    int
}

func normalizeDashboardMetricList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
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

func fallbackClosedStatuses(statuses []string) []string {
	normalized := normalizeDashboardMetricList(statuses)
	if len(normalized) > 0 {
		return normalized
	}
	return []string{"closed", "resolved", "done", "false_positive"}
}

func (r *SystemRepository) TenantDashboardMetrics(
	ctx context.Context,
	tenantID uuid.UUID,
	openStatuses []string,
	closedStatuses []string,
	overdueThreshold time.Duration,
) (DashboardMetricsSnapshot, error) {
	if r == nil || r.pool == nil {
		return DashboardMetricsSnapshot{}, fmt.Errorf("postgres pool is not configured")
	}
	normalizedOpen := normalizeDashboardMetricList(openStatuses)
	normalizedClosed := fallbackClosedStatuses(closedStatuses)

	openCases, err := r.countOpenCases(ctx, tenantID, normalizedOpen, normalizedClosed)
	if err != nil {
		return DashboardMetricsSnapshot{}, err
	}
	openAlerts, err := r.countOpenAlerts(ctx, tenantID)
	if err != nil {
		return DashboardMetricsSnapshot{}, err
	}
	overdueCases, err := r.countOverdueCases(ctx, tenantID, normalizedOpen, normalizedClosed, overdueThreshold)
	if err != nil {
		return DashboardMetricsSnapshot{}, err
	}
	slaBySeverity, err := r.slaBySeverity(ctx, tenantID, normalizedOpen, normalizedClosed)
	if err != nil {
		return DashboardMetricsSnapshot{}, err
	}
	resolvedByAnalyst, err := r.resolvedByAnalyst(ctx, tenantID, normalizedClosed)
	if err != nil {
		return DashboardMetricsSnapshot{}, err
	}
	caseStatus, err := r.countCasesByStatus(ctx, tenantID)
	if err != nil {
		return DashboardMetricsSnapshot{}, err
	}
	caseCategory, err := r.countCasesByCategory(ctx, tenantID)
	if err != nil {
		return DashboardMetricsSnapshot{}, err
	}
	alertStatus, err := r.countAlertsByStatus(ctx, tenantID)
	if err != nil {
		return DashboardMetricsSnapshot{}, err
	}
	alertCategory, err := r.countAlertsByCategory(ctx, tenantID)
	if err != nil {
		return DashboardMetricsSnapshot{}, err
	}

	return DashboardMetricsSnapshot{
		OpenCases:         openCases,
		OpenAlerts:        openAlerts,
		OverdueCases:      overdueCases,
		CaseStatus:        caseStatus,
		CaseCategory:      caseCategory,
		AlertStatus:       alertStatus,
		AlertCategory:     alertCategory,
		SLABySeverity:     slaBySeverity,
		ResolvedByAnalyst: resolvedByAnalyst,
	}, nil
}

func (r *SystemRepository) EvaluateDashboardCustomMetric(
	ctx context.Context,
	tenantID uuid.UUID,
	openStatuses []string,
	closedStatuses []string,
	def DashboardCustomMetricDefinition,
) (float64, error) {
	normalizedSource := strings.ToLower(strings.TrimSpace(def.Source))
	if normalizedSource != "cases" && normalizedSource != "alerts" {
		return 0, fmt.Errorf("unsupported metric source %q", def.Source)
	}
	if normalizedSource == "alerts" {
		return r.evaluateAlertDashboardMetric(ctx, tenantID, def)
	}
	return r.evaluateCaseDashboardMetric(ctx, tenantID, openStatuses, closedStatuses, def)
}

func (r *SystemRepository) countOpenCases(ctx context.Context, tenantID uuid.UUID, openStatuses []string, closedStatuses []string) (int, error) {
	var (
		q    string
		args []any
	)
	if len(openStatuses) > 0 {
		q = `
			SELECT COUNT(*)
			FROM cases
			WHERE tenant_id = $1
			  AND LOWER(COALESCE(status, '')) = ANY($2::text[])
		`
		args = []any{tenantID, openStatuses}
	} else {
		q = `
			SELECT COUNT(*)
			FROM cases
			WHERE tenant_id = $1
			  AND NOT (LOWER(COALESCE(status, '')) = ANY($2::text[]))
		`
		args = []any{tenantID, closedStatuses}
	}
	var count int
	if err := r.pool.QueryRow(ctx, q, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("query open cases: %w", err)
	}
	return count, nil
}

func (r *SystemRepository) countOpenAlerts(ctx context.Context, tenantID uuid.UUID) (int, error) {
	const q = `
		SELECT COUNT(*)
		FROM alerts
		WHERE tenant_id = $1
		  AND LOWER(COALESCE(status, '')) <> 'closed'
	`
	var count int
	if err := r.pool.QueryRow(ctx, q, tenantID).Scan(&count); err != nil {
		return 0, fmt.Errorf("query open alerts: %w", err)
	}
	return count, nil
}

func (r *SystemRepository) countOverdueCases(
	ctx context.Context,
	tenantID uuid.UUID,
	openStatuses []string,
	closedStatuses []string,
	overdueThreshold time.Duration,
) (int, error) {
	thresholdMinutes := int(overdueThreshold / time.Minute)
	if thresholdMinutes <= 0 {
		thresholdMinutes = 24 * 60
	}
	var (
		q    string
		args []any
	)
	if len(openStatuses) > 0 {
		q = `
			SELECT COUNT(*)
			FROM cases
			WHERE tenant_id = $1
			  AND created_at <= NOW() - ($2 * INTERVAL '1 minute')
			  AND LOWER(COALESCE(status, '')) = ANY($3::text[])
		`
		args = []any{tenantID, thresholdMinutes, openStatuses}
	} else {
		q = `
			SELECT COUNT(*)
			FROM cases
			WHERE tenant_id = $1
			  AND created_at <= NOW() - ($2 * INTERVAL '1 minute')
			  AND NOT (LOWER(COALESCE(status, '')) = ANY($3::text[]))
		`
		args = []any{tenantID, thresholdMinutes, closedStatuses}
	}
	var count int
	if err := r.pool.QueryRow(ctx, q, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("query overdue cases: %w", err)
	}
	return count, nil
}

func (r *SystemRepository) slaBySeverity(
	ctx context.Context,
	tenantID uuid.UUID,
	openStatuses []string,
	closedStatuses []string,
) ([]DashboardSLABySeverity, error) {
	metricBySeverity := map[string]DashboardSLABySeverity{
		"critical": {Severity: "critical", TargetMinutes: dashboardSeverityTargetsMinutes["critical"]},
		"high":     {Severity: "high", TargetMinutes: dashboardSeverityTargetsMinutes["high"]},
		"medium":   {Severity: "medium", TargetMinutes: dashboardSeverityTargetsMinutes["medium"]},
		"low":      {Severity: "low", TargetMinutes: dashboardSeverityTargetsMinutes["low"]},
	}

	severityMinutesExpr := `
		CASE LOWER(COALESCE(c.severity, 'medium'))
			WHEN 'critical' THEN 60
			WHEN 'high' THEN 240
			WHEN 'medium' THEN 720
			WHEN 'low' THEN 1440
			ELSE 720
		END
	`

	var (
		q    string
		args []any
	)
	if len(openStatuses) > 0 {
		q = fmt.Sprintf(`
			SELECT
				LOWER(COALESCE(c.severity, 'medium')) AS severity,
				COUNT(*) FILTER (WHERE LOWER(COALESCE(c.status, '')) = ANY($2::text[])) AS open_cases,
				COUNT(*) FILTER (WHERE LOWER(COALESCE(c.status, '')) = ANY($3::text[])) AS resolved_cases,
				COALESCE(
					AVG(EXTRACT(EPOCH FROM (COALESCE(c.closed_at, c.updated_at) - c.created_at)) / 60.0)
					FILTER (WHERE LOWER(COALESCE(c.status, '')) = ANY($3::text[])),
					0
				) AS avg_resolution_minutes,
				COUNT(*) FILTER (
					WHERE (
						LOWER(COALESCE(c.status, '')) = ANY($2::text[])
						AND c.created_at <= NOW() - ((%s) * INTERVAL '1 minute')
					)
					OR (
						LOWER(COALESCE(c.status, '')) = ANY($3::text[])
						AND EXTRACT(EPOCH FROM (COALESCE(c.closed_at, c.updated_at) - c.created_at)) / 60.0 > (%s)
					)
				) AS breached_cases
			FROM cases c
			WHERE c.tenant_id = $1
			GROUP BY 1
		`, severityMinutesExpr, severityMinutesExpr)
		args = []any{tenantID, openStatuses, closedStatuses}
	} else {
		q = fmt.Sprintf(`
			SELECT
				LOWER(COALESCE(c.severity, 'medium')) AS severity,
				COUNT(*) FILTER (WHERE NOT (LOWER(COALESCE(c.status, '')) = ANY($2::text[]))) AS open_cases,
				COUNT(*) FILTER (WHERE LOWER(COALESCE(c.status, '')) = ANY($2::text[])) AS resolved_cases,
				COALESCE(
					AVG(EXTRACT(EPOCH FROM (COALESCE(c.closed_at, c.updated_at) - c.created_at)) / 60.0)
					FILTER (WHERE LOWER(COALESCE(c.status, '')) = ANY($2::text[])),
					0
				) AS avg_resolution_minutes,
				COUNT(*) FILTER (
					WHERE (
						NOT (LOWER(COALESCE(c.status, '')) = ANY($2::text[]))
						AND c.created_at <= NOW() - ((%s) * INTERVAL '1 minute')
					)
					OR (
						LOWER(COALESCE(c.status, '')) = ANY($2::text[])
						AND EXTRACT(EPOCH FROM (COALESCE(c.closed_at, c.updated_at) - c.created_at)) / 60.0 > (%s)
					)
				) AS breached_cases
			FROM cases c
			WHERE c.tenant_id = $1
			GROUP BY 1
		`, severityMinutesExpr, severityMinutesExpr)
		args = []any{tenantID, closedStatuses}
	}

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query SLA by severity: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			severity     string
			openCases    int
			resolved     int
			avgMinutes   float64
			breachedCase int
		)
		if err := rows.Scan(&severity, &openCases, &resolved, &avgMinutes, &breachedCase); err != nil {
			return nil, fmt.Errorf("scan SLA by severity row: %w", err)
		}
		normalizedSeverity := strings.ToLower(strings.TrimSpace(severity))
		entry, exists := metricBySeverity[normalizedSeverity]
		if !exists {
			entry = DashboardSLABySeverity{
				Severity:      normalizedSeverity,
				TargetMinutes: dashboardSeverityTargetsMinutes["medium"],
			}
		}
		entry.OpenCases = openCases
		entry.ResolvedCases = resolved
		entry.AvgResolutionMinutes = avgMinutes
		entry.BreachedCases = breachedCase
		metricBySeverity[normalizedSeverity] = entry
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate SLA by severity rows: %w", rows.Err())
	}

	orderedSeverities := []string{"critical", "high", "medium", "low"}
	out := make([]DashboardSLABySeverity, 0, len(metricBySeverity))
	for _, severity := range orderedSeverities {
		out = append(out, metricBySeverity[severity])
		delete(metricBySeverity, severity)
	}
	for _, entry := range metricBySeverity {
		out = append(out, entry)
	}
	return out, nil
}

func (r *SystemRepository) resolvedByAnalyst(ctx context.Context, tenantID uuid.UUID, closedStatuses []string) ([]DashboardAnalystResolved, error) {
	const q = `
		SELECT
			COALESCE(u.id::text, '') AS user_id,
			COALESCE(u.username, '') AS username,
			COALESCE(NULLIF(TRIM(u.full_name), ''), NULLIF(TRIM(u.username), ''), 'Unassigned') AS display_name,
			COUNT(*) AS resolved_cases
		FROM cases c
		LEFT JOIN users u ON u.id = COALESCE(c.assigned_to, c.created_by)
		WHERE c.tenant_id = $1
		  AND LOWER(COALESCE(c.status, '')) = ANY($2::text[])
		GROUP BY 1, 2, 3
		ORDER BY resolved_cases DESC, display_name ASC
		LIMIT 20
	`
	rows, err := r.pool.Query(ctx, q, tenantID, closedStatuses)
	if err != nil {
		return nil, fmt.Errorf("query resolved cases by analyst: %w", err)
	}
	defer rows.Close()

	out := make([]DashboardAnalystResolved, 0, 20)
	for rows.Next() {
		var item DashboardAnalystResolved
		if err := rows.Scan(&item.UserID, &item.Username, &item.DisplayName, &item.ResolvedCases); err != nil {
			return nil, fmt.Errorf("scan resolved cases by analyst row: %w", err)
		}
		out = append(out, item)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate resolved cases by analyst rows: %w", rows.Err())
	}
	return out, nil
}

func (r *SystemRepository) countCasesByStatus(ctx context.Context, tenantID uuid.UUID) ([]DashboardCountBucket, error) {
	const q = `
		SELECT LOWER(COALESCE(status, 'unknown')) AS key, COUNT(*) AS count
		FROM cases
		WHERE tenant_id = $1
		GROUP BY 1
		ORDER BY count DESC, key ASC
		LIMIT 50
	`
	return r.queryCountBuckets(ctx, q, tenantID, "unknown")
}

func (r *SystemRepository) countCasesByCategory(ctx context.Context, tenantID uuid.UUID) ([]DashboardCountBucket, error) {
	const q = `
		SELECT
			LOWER(NULLIF(TRIM(COALESCE(meta.data->>'category', '')), '')) AS key,
			COUNT(*) AS count
		FROM cases c
		LEFT JOIN LATERAL (
			SELECT cm.data
			FROM catalog_items cm
			WHERE cm.kind = 'case_meta'
			  AND cm.tenant_id = $1
			  AND cm.ref_id = c.id
			ORDER BY cm.updated_at DESC
			LIMIT 1
		) meta ON TRUE
		WHERE c.tenant_id = $1
		GROUP BY 1
		ORDER BY count DESC, key ASC
		LIMIT 50
	`
	return r.queryCountBuckets(ctx, q, tenantID, "uncategorized")
}

func (r *SystemRepository) countAlertsByStatus(ctx context.Context, tenantID uuid.UUID) ([]DashboardCountBucket, error) {
	const q = `
		SELECT LOWER(COALESCE(status, 'unknown')) AS key, COUNT(*) AS count
		FROM alerts
		WHERE tenant_id = $1
		GROUP BY 1
		ORDER BY count DESC, key ASC
		LIMIT 50
	`
	return r.queryCountBuckets(ctx, q, tenantID, "unknown")
}

func (r *SystemRepository) countAlertsByCategory(ctx context.Context, tenantID uuid.UUID) ([]DashboardCountBucket, error) {
	const q = `
		SELECT LOWER(NULLIF(TRIM(COALESCE(source, '')), '')) AS key, COUNT(*) AS count
		FROM alerts
		WHERE tenant_id = $1
		GROUP BY 1
		ORDER BY count DESC, key ASC
		LIMIT 50
	`
	return r.queryCountBuckets(ctx, q, tenantID, "unknown")
}

func (r *SystemRepository) queryCountBuckets(ctx context.Context, q string, tenantID uuid.UUID, fallback string) ([]DashboardCountBucket, error) {
	rows, err := r.pool.Query(ctx, q, tenantID)
	if err != nil {
		return nil, fmt.Errorf("query dashboard count buckets: %w", err)
	}
	defer rows.Close()

	out := make([]DashboardCountBucket, 0, 32)
	for rows.Next() {
		var (
			key   sql.NullString
			count int
		)
		if err := rows.Scan(&key, &count); err != nil {
			return nil, fmt.Errorf("scan dashboard count bucket row: %w", err)
		}
		normalized := strings.TrimSpace(key.String)
		if normalized == "" {
			normalized = fallback
		}
		out = append(out, DashboardCountBucket{Key: normalized, Count: count})
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate dashboard count bucket rows: %w", rows.Err())
	}
	return out, nil
}

func (r *SystemRepository) evaluateCaseDashboardMetric(
	ctx context.Context,
	tenantID uuid.UUID,
	openStatuses []string,
	closedStatuses []string,
	def DashboardCustomMetricDefinition,
) (float64, error) {
	normalizedMeasure := strings.ToLower(strings.TrimSpace(def.Measure))
	normalizedClosed := fallbackClosedStatuses(closedStatuses)
	normalizedOpen := normalizeDashboardMetricList(openStatuses)
	if len(normalizedOpen) == 0 {
		normalizedOpen = nil
	}

	baseConditions := []string{"c.tenant_id = $1"}
	baseArgs := []any{tenantID}
	addCaseFilters(&baseConditions, &baseArgs, "c", def)

	conditions := append([]string{}, baseConditions...)
	args := append([]any{}, baseArgs...)
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	hasStatusFilter := len(normalizeDashboardMetricList(def.Statuses)) > 0

	switch normalizedMeasure {
	case "count":
		q := fmt.Sprintf("SELECT COUNT(*)::double precision FROM cases c WHERE %s", strings.Join(conditions, " AND "))
		var value float64
		if err := r.pool.QueryRow(ctx, q, args...).Scan(&value); err != nil {
			return 0, fmt.Errorf("evaluate custom case metric count: %w", err)
		}
		return value, nil
	case "avg_resolution_minutes":
		if !hasStatusFilter {
			placeholder := addArg(normalizedClosed)
			conditions = append(conditions, fmt.Sprintf("LOWER(COALESCE(c.status, '')) = ANY(%s::text[])", placeholder))
		}
		q := fmt.Sprintf(
			"SELECT COALESCE(AVG(EXTRACT(EPOCH FROM (COALESCE(c.closed_at, c.updated_at) - c.created_at)) / 60.0), 0)::double precision FROM cases c WHERE %s",
			strings.Join(conditions, " AND "),
		)
		var value float64
		if err := r.pool.QueryRow(ctx, q, args...).Scan(&value); err != nil {
			return 0, fmt.Errorf("evaluate custom case metric avg resolution: %w", err)
		}
		return value, nil
	case "overdue_count":
		if !hasStatusFilter {
			if len(normalizedOpen) > 0 {
				placeholder := addArg(normalizedOpen)
				conditions = append(conditions, fmt.Sprintf("LOWER(COALESCE(c.status, '')) = ANY(%s::text[])", placeholder))
			} else {
				placeholder := addArg(normalizedClosed)
				conditions = append(conditions, fmt.Sprintf("NOT (LOWER(COALESCE(c.status, '')) = ANY(%s::text[]))", placeholder))
			}
		}
		overdueMinutes := def.OverdueMinutes
		if overdueMinutes <= 0 {
			overdueMinutes = 24 * 60
		}
		placeholder := addArg(overdueMinutes)
		conditions = append(conditions, fmt.Sprintf("c.created_at <= NOW() - (%s * INTERVAL '1 minute')", placeholder))
		q := fmt.Sprintf("SELECT COUNT(*)::double precision FROM cases c WHERE %s", strings.Join(conditions, " AND "))
		var value float64
		if err := r.pool.QueryRow(ctx, q, args...).Scan(&value); err != nil {
			return 0, fmt.Errorf("evaluate custom case metric overdue count: %w", err)
		}
		return value, nil
	default:
		return 0, fmt.Errorf("unsupported case metric measure %q", def.Measure)
	}
}

func addCaseFilters(conditions *[]string, args *[]any, alias string, def DashboardCustomMetricDefinition) {
	addArg := func(value any) string {
		*args = append(*args, value)
		return fmt.Sprintf("$%d", len(*args))
	}

	statuses := normalizeDashboardMetricList(def.Statuses)
	if len(statuses) > 0 {
		placeholder := addArg(statuses)
		*conditions = append(*conditions, fmt.Sprintf("LOWER(COALESCE(%s.status, '')) = ANY(%s::text[])", alias, placeholder))
	}
	severities := normalizeDashboardMetricList(def.Severities)
	if len(severities) > 0 {
		placeholder := addArg(severities)
		*conditions = append(*conditions, fmt.Sprintf("LOWER(COALESCE(%s.severity, '')) = ANY(%s::text[])", alias, placeholder))
	}
	categories := normalizeDashboardMetricList(def.Categories)
	if len(categories) > 0 {
		placeholder := addArg(categories)
		*conditions = append(*conditions, fmt.Sprintf(
			"EXISTS (SELECT 1 FROM catalog_items cm WHERE cm.kind = 'case_meta' AND cm.tenant_id = %s.tenant_id AND cm.ref_id = %s.id AND LOWER(COALESCE(cm.data->>'category', '')) = ANY(%s::text[]))",
			alias,
			alias,
			placeholder,
		))
	}

	if def.CreatedFrom != nil {
		placeholder := addArg(*def.CreatedFrom)
		*conditions = append(*conditions, fmt.Sprintf("%s.created_at >= %s", alias, placeholder))
	}
	if def.CreatedTo != nil {
		placeholder := addArg(*def.CreatedTo)
		*conditions = append(*conditions, fmt.Sprintf("%s.created_at <= %s", alias, placeholder))
	}
	if def.ClosedFrom != nil {
		placeholder := addArg(*def.ClosedFrom)
		*conditions = append(*conditions, fmt.Sprintf("COALESCE(%s.closed_at, %s.updated_at) >= %s", alias, alias, placeholder))
	}
	if def.ClosedTo != nil {
		placeholder := addArg(*def.ClosedTo)
		*conditions = append(*conditions, fmt.Sprintf("COALESCE(%s.closed_at, %s.updated_at) <= %s", alias, alias, placeholder))
	}

	if def.CreatedWithinHour > 0 {
		placeholder := addArg(def.CreatedWithinHour)
		*conditions = append(*conditions, fmt.Sprintf("%s.created_at >= NOW() - (%s * INTERVAL '1 hour')", alias, placeholder))
	}
	if def.ClosedWithinHour > 0 {
		placeholder := addArg(def.ClosedWithinHour)
		*conditions = append(*conditions, fmt.Sprintf("COALESCE(%s.closed_at, %s.updated_at) >= NOW() - (%s * INTERVAL '1 hour')", alias, alias, placeholder))
	}
}

func (r *SystemRepository) evaluateAlertDashboardMetric(
	ctx context.Context,
	tenantID uuid.UUID,
	def DashboardCustomMetricDefinition,
) (float64, error) {
	normalizedMeasure := strings.ToLower(strings.TrimSpace(def.Measure))
	if normalizedMeasure != "count" {
		return 0, fmt.Errorf("unsupported alert metric measure %q", def.Measure)
	}

	conditions := []string{"a.tenant_id = $1"}
	args := []any{tenantID}
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}

	statuses := normalizeDashboardMetricList(def.Statuses)
	if len(statuses) > 0 {
		placeholder := addArg(statuses)
		conditions = append(conditions, fmt.Sprintf("LOWER(COALESCE(a.status, '')) = ANY(%s::text[])", placeholder))
	}
	severities := normalizeDashboardMetricList(def.Severities)
	if len(severities) > 0 {
		placeholder := addArg(severities)
		conditions = append(conditions, fmt.Sprintf("LOWER(COALESCE(a.severity, '')) = ANY(%s::text[])", placeholder))
	}
	categories := normalizeDashboardMetricList(def.Categories)
	if len(categories) > 0 {
		placeholder := addArg(categories)
		conditions = append(conditions, fmt.Sprintf("LOWER(COALESCE(a.source, '')) = ANY(%s::text[])", placeholder))
	}
	if def.CreatedFrom != nil {
		placeholder := addArg(*def.CreatedFrom)
		conditions = append(conditions, fmt.Sprintf("a.created_at >= %s", placeholder))
	}
	if def.CreatedTo != nil {
		placeholder := addArg(*def.CreatedTo)
		conditions = append(conditions, fmt.Sprintf("a.created_at <= %s", placeholder))
	}
	if def.ClosedFrom != nil {
		placeholder := addArg(*def.ClosedFrom)
		conditions = append(conditions, fmt.Sprintf("a.updated_at >= %s", placeholder))
	}
	if def.ClosedTo != nil {
		placeholder := addArg(*def.ClosedTo)
		conditions = append(conditions, fmt.Sprintf("a.updated_at <= %s", placeholder))
	}
	if def.CreatedWithinHour > 0 {
		placeholder := addArg(def.CreatedWithinHour)
		conditions = append(conditions, fmt.Sprintf("a.created_at >= NOW() - (%s * INTERVAL '1 hour')", placeholder))
	}
	if def.ClosedWithinHour > 0 {
		placeholder := addArg(def.ClosedWithinHour)
		conditions = append(conditions, fmt.Sprintf("a.updated_at >= NOW() - (%s * INTERVAL '1 hour')", placeholder))
	}

	q := fmt.Sprintf("SELECT COUNT(*)::double precision FROM alerts a WHERE %s", strings.Join(conditions, " AND "))
	var value float64
	if err := r.pool.QueryRow(ctx, q, args...).Scan(&value); err != nil {
		return 0, fmt.Errorf("evaluate custom alert metric count: %w", err)
	}
	return value, nil
}
