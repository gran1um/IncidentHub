package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DashboardStats struct {
	ActiveCases    int     `json:"activeCases"`
	Alerts24h      int     `json:"alertsCount"`
	ResolvedToday  int     `json:"resolvedToday"`
	AvgResponseMin float64 `json:"avgResponseMin"`
}

type TenantResourceStats struct {
	ActiveUsers    int `json:"activeUsers"`
	OpenCases      int `json:"openCases"`
	Alerts24h      int `json:"alerts24h"`
	Observables    int `json:"observables"`
	ForumThreads   int `json:"forumThreads"`
	RateLimits     int `json:"rateLimits"`
	APITokens      int `json:"apiTokens"`
	APIRequests24h int `json:"apiRequests24h"`
}

type PoolStats struct {
	TotalConns      int64 `json:"totalConns"`
	IdleConns       int64 `json:"idleConns"`
	AcquiredConns   int64 `json:"acquiredConns"`
	MaxConns        int64 `json:"maxConns"`
	AcquireCount    int64 `json:"acquireCount"`
	AcquireDuration int64 `json:"acquireDurationMs"`
}

type SystemRepository struct {
	pool *pgxpool.Pool
}

func NewSystemRepository(pool *pgxpool.Pool) *SystemRepository {
	return &SystemRepository{pool: pool}
}

type caseStatusMetricDefinition struct {
	Code     string
	IsClosed bool
}

func (r *SystemRepository) Ping(ctx context.Context) error {
	if r == nil || r.pool == nil {
		return fmt.Errorf("postgres pool is not configured")
	}
	var one int
	if err := r.pool.QueryRow(ctx, "SELECT 1").Scan(&one); err != nil {
		return fmt.Errorf("ping postgres: %w", err)
	}
	return nil
}

func (r *SystemRepository) PoolStats() PoolStats {
	if r == nil || r.pool == nil {
		return PoolStats{}
	}
	stats := r.pool.Stat()
	return PoolStats{
		TotalConns:      int64(stats.TotalConns()),
		IdleConns:       int64(stats.IdleConns()),
		AcquiredConns:   int64(stats.AcquiredConns()),
		MaxConns:        int64(stats.MaxConns()),
		AcquireCount:    stats.AcquireCount(),
		AcquireDuration: stats.AcquireDuration().Milliseconds(),
	}
}

func (r *SystemRepository) TenantDashboardStats(ctx context.Context, tenantID uuid.UUID) (DashboardStats, error) {
	if r == nil || r.pool == nil {
		return DashboardStats{}, fmt.Errorf("postgres pool is not configured")
	}
	openStatuses, closedStatuses := r.resolveCaseStatusSets(ctx, tenantID)
	activeCases, err := r.countOpenCases(ctx, tenantID, openStatuses, closedStatuses)
	if err != nil {
		return DashboardStats{}, fmt.Errorf("count open cases for dashboard stats: %w", err)
	}
	closedStatuses = fallbackClosedStatuses(closedStatuses)
	q := `
		SELECT
			(SELECT COUNT(*) FROM alerts WHERE tenant_id = $1 AND created_at >= NOW() - INTERVAL '24 hours') AS alerts_24h,
			(SELECT COUNT(*) FROM cases WHERE tenant_id = $1 AND LOWER(COALESCE(status, '')) = ANY($2::text[]) AND updated_at::date = CURRENT_DATE) AS resolved_today,
			(SELECT COALESCE(AVG(EXTRACT(EPOCH FROM (COALESCE(closed_at, updated_at) - created_at)) / 60.0), 0)
			 FROM cases
			 WHERE tenant_id = $1
			   AND LOWER(COALESCE(status, '')) = ANY($2::text[])) AS avg_response_min
	`

	var out DashboardStats
	if err := r.pool.QueryRow(ctx, q, tenantID, closedStatuses).Scan(&out.Alerts24h, &out.ResolvedToday, &out.AvgResponseMin); err != nil {
		return DashboardStats{}, fmt.Errorf("query dashboard stats: %w", err)
	}
	out.ActiveCases = activeCases
	return out, nil
}

func (r *SystemRepository) TenantResourceStats(ctx context.Context, tenantID uuid.UUID) (TenantResourceStats, error) {
	if r == nil || r.pool == nil {
		return TenantResourceStats{}, fmt.Errorf("postgres pool is not configured")
	}
	openStatuses, closedStatuses := r.resolveCaseStatusSets(ctx, tenantID)
	openCases, err := r.countOpenCases(ctx, tenantID, openStatuses, closedStatuses)
	if err != nil {
		return TenantResourceStats{}, fmt.Errorf("count open cases for tenant resource stats: %w", err)
	}
	q := `
		SELECT
			(SELECT COUNT(*) FROM tenant_memberships tm WHERE tm.tenant_id = $1 AND tm.is_active = TRUE) AS active_users,
			(SELECT COUNT(*) FROM alerts a WHERE a.tenant_id = $1 AND a.created_at >= NOW() - INTERVAL '24 hours') AS alerts_24h,
			(SELECT COUNT(*) FROM case_observables o WHERE o.tenant_id = $1) AS observables,
			(SELECT COUNT(*) FROM catalog_items ci WHERE ci.tenant_id = $1 AND ci.kind = 'forum_thread') AS forum_threads,
			(SELECT COUNT(*) FROM catalog_items ci WHERE ci.tenant_id = $1 AND ci.kind = 'rate_limits') AS rate_limits,
			(SELECT COUNT(*) FROM api_access_tokens t WHERE t.tenant_id = $1 AND t.revoked_at IS NULL) AS api_tokens,
			(SELECT COUNT(*) FROM audit_logs al WHERE al.tenant_id = $1 AND al.action = 'api_request' AND al.created_at >= NOW() - INTERVAL '24 hours') AS api_requests_24h
	`

	var out TenantResourceStats
	if err := r.pool.QueryRow(ctx, q, tenantID).Scan(
		&out.ActiveUsers,
		&out.Alerts24h,
		&out.Observables,
		&out.ForumThreads,
		&out.RateLimits,
		&out.APITokens,
		&out.APIRequests24h,
	); err != nil {
		return TenantResourceStats{}, fmt.Errorf("query tenant resource stats: %w", err)
	}
	out.OpenCases = openCases
	return out, nil
}

func (r *SystemRepository) resolveCaseStatusSets(ctx context.Context, tenantID uuid.UUID) (openStatuses []string, closedStatuses []string) {
	statuses, err := r.loadCaseStatusMetricDefinitions(ctx, tenantID, true)
	if err != nil {
		return nil, normalizeStatusCodes(defaultClosedCaseStatuses)
	}
	if len(statuses) == 0 {
		statuses, err = r.loadCaseStatusMetricDefinitions(ctx, tenantID, false)
		if err != nil {
			return nil, normalizeStatusCodes(defaultClosedCaseStatuses)
		}
	}
	if len(statuses) == 0 {
		return nil, normalizeStatusCodes(defaultClosedCaseStatuses)
	}

	open := make([]string, 0, len(statuses))
	closed := make([]string, 0, len(statuses))
	seenOpen := make(map[string]struct{}, len(statuses))
	seenClosed := make(map[string]struct{}, len(statuses))

	for _, item := range statuses {
		code := strings.ToLower(strings.TrimSpace(item.Code))
		if code == "" {
			continue
		}
		if item.IsClosed {
			if _, exists := seenClosed[code]; exists {
				continue
			}
			seenClosed[code] = struct{}{}
			closed = append(closed, code)
			continue
		}
		if _, exists := seenOpen[code]; exists {
			continue
		}
		seenOpen[code] = struct{}{}
		open = append(open, code)
	}
	if len(closed) == 0 {
		closed = normalizeStatusCodes(defaultClosedCaseStatuses)
	}
	return open, closed
}

func (r *SystemRepository) loadCaseStatusMetricDefinitions(ctx context.Context, tenantID uuid.UUID, tenantOnly bool) ([]caseStatusMetricDefinition, error) {
	if r == nil || r.pool == nil {
		return nil, fmt.Errorf("postgres pool is not configured")
	}
	query := `
		SELECT
			LOWER(TRIM(COALESCE(data->>'code', data->>'id', data->>'status', ''))) AS code,
			(
				CASE
					WHEN data ? 'is_closed' THEN LOWER(COALESCE(data->>'is_closed', 'false'))
					WHEN data ? 'isClosed' THEN LOWER(COALESCE(data->>'isClosed', 'false'))
					WHEN data ? 'closed' THEN LOWER(COALESCE(data->>'closed', 'false'))
					ELSE 'false'
				END
			) IN ('true', '1', 'yes', 'y') AS is_closed
		FROM catalog_items
		WHERE kind = 'case_statuses'
	`
	args := make([]any, 0, 1)
	if tenantOnly {
		query += " AND tenant_id = $1"
		args = append(args, tenantID)
	} else {
		query += " AND tenant_id IS NULL"
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query case status metric definitions: %w", err)
	}
	defer rows.Close()

	out := make([]caseStatusMetricDefinition, 0)
	for rows.Next() {
		var item caseStatusMetricDefinition
		if scanErr := rows.Scan(&item.Code, &item.IsClosed); scanErr != nil {
			return nil, fmt.Errorf("scan case status metric definitions: %w", scanErr)
		}
		out = append(out, item)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("iterate case status metric definitions: %w", rowsErr)
	}
	return out, nil
}

func (r *SystemRepository) QueryReadOnly(ctx context.Context, query string, args []any, limit int) ([]map[string]any, error) {
	if r == nil || r.pool == nil {
		return nil, fmt.Errorf("postgres pool is not configured")
	}
	normalized, err := sanitizeReadOnlyQuery(query)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	limitedQuery := normalized
	if !strings.Contains(strings.ToLower(normalized), " limit ") {
		limitedQuery = fmt.Sprintf("SELECT * FROM (%s) AS workflow_query LIMIT %d", normalized, limit)
	}

	rows, err := r.pool.Query(ctx, limitedQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("execute postgres read-only query: %w", err)
	}
	defer rows.Close()

	fields := rows.FieldDescriptions()
	result := make([]map[string]any, 0)
	for rows.Next() {
		values, valuesErr := rows.Values()
		if valuesErr != nil {
			return nil, fmt.Errorf("scan postgres query values: %w", valuesErr)
		}
		item := make(map[string]any, len(fields))
		for i, field := range fields {
			item[field.Name] = normalizeDBValue(values[i])
		}
		result = append(result, item)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate postgres query rows: %w", rows.Err())
	}
	return result, nil
}

type FactoryResetLocalSummary struct {
	AdminTenantID uuid.UUID      `json:"admin_tenant_id"`
	KeptUserIDs   []uuid.UUID    `json:"kept_user_ids"`
	Deleted       map[string]int `json:"deleted"`
}

func (r *SystemRepository) FactoryResetLocal(ctx context.Context) (*FactoryResetLocalSummary, error) {
	if r == nil || r.pool == nil {
		return nil, fmt.Errorf("postgres pool is not configured")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin factory reset transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var adminTenantID uuid.UUID
	if scanErr := tx.QueryRow(ctx, `SELECT id FROM tenants WHERE LOWER(slug) = 'admin' ORDER BY created_at ASC LIMIT 1`).Scan(&adminTenantID); scanErr != nil {
		return nil, fmt.Errorf("resolve admin tenant: %w", scanErr)
	}

	rows, err := tx.Query(ctx, `SELECT id FROM users WHERE is_platform_admin = TRUE ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("list kept platform admins: %w", err)
	}
	keptUserIDs := make([]uuid.UUID, 0)
	for rows.Next() {
		var userID uuid.UUID
		if scanErr := rows.Scan(&userID); scanErr != nil {
			rows.Close()
			return nil, fmt.Errorf("scan kept platform admins: %w", scanErr)
		}
		keptUserIDs = append(keptUserIDs, userID)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate kept platform admins: %w", rowsErr)
	}
	rows.Close()
	if len(keptUserIDs) == 0 {
		return nil, fmt.Errorf("at least one platform admin is required for local factory reset")
	}

	deleted := make(map[string]int)
	deleteCount := func(label, query string, args ...any) error {
		wrapped := "WITH deleted AS (" + query + " RETURNING 1) SELECT COUNT(*) FROM deleted"
		var count int
		if err := tx.QueryRow(ctx, wrapped, args...).Scan(&count); err != nil {
			return fmt.Errorf("delete %s: %w", label, err)
		}
		deleted[label] = count
		return nil
	}

	queries := []struct {
		label string
		query string
		args  []any
	}{
		{label: "connector_hub_execution_events", query: `DELETE FROM connector_hub_execution_events`},
		{label: "connector_hub_execution_attempts", query: `DELETE FROM connector_hub_execution_attempts`},
		{label: "connector_hub_executions", query: `DELETE FROM connector_hub_executions`},
		{label: "forum_external_bindings", query: `DELETE FROM forum_external_bindings`},
		{label: "connector_ingest_states", query: `DELETE FROM connector_ingest_states`},
		{label: "inbound_connector_runs", query: `DELETE FROM inbound_connector_runs`},
		{label: "workflow_runs", query: `DELETE FROM workflow_runs`},
		{label: "ai_agent_queue_events", query: `DELETE FROM ai_agent_queue_events`},
		{label: "ai_agent_workloads", query: `DELETE FROM ai_agent_workloads`},
		{label: "ai_chat_messages", query: `DELETE FROM ai_chat_messages`},
		{label: "ai_chat_sessions", query: `DELETE FROM ai_chat_sessions`},
		{label: "case_ai_analyses", query: `DELETE FROM case_ai_analyses`},
		{label: "async_operations", query: `DELETE FROM async_operations`},
		{label: "audit_logs", query: `DELETE FROM audit_logs`},
		{label: "user_notification_settings", query: `DELETE FROM user_notification_settings`},
		{label: "telegram_notification_bots", query: `DELETE FROM telegram_notification_bots`},
		{label: "api_access_tokens", query: `DELETE FROM api_access_tokens`},
		{label: "refresh_tokens", query: `DELETE FROM refresh_tokens`},
		{label: "user_xp_events", query: `DELETE FROM user_xp_events`},
		{label: "connector_recipient_aliases", query: `DELETE FROM connector_recipient_aliases`},
		{label: "case_tenant_shares", query: `DELETE FROM case_tenant_shares`},
		{label: "case_timeline_events", query: `DELETE FROM case_timeline_events`},
		{label: "case_attachments", query: `DELETE FROM case_attachments`},
		{label: "case_pages", query: `DELETE FROM case_pages`},
		{label: "case_observables", query: `DELETE FROM case_observables`},
		{label: "tasks", query: `DELETE FROM tasks`},
		{label: "alerts", query: `DELETE FROM alerts`},
		{label: "cases", query: `DELETE FROM cases`},
		{label: "catalog_items", query: `DELETE FROM catalog_items WHERE tenant_id IS NOT NULL`},
		{label: "tenant_memberships", query: `DELETE FROM tenant_memberships WHERE tenant_id <> $1 OR user_id <> ALL($2::uuid[])`, args: []any{adminTenantID, keptUserIDs}},
		{label: "tenants", query: `DELETE FROM tenants WHERE id <> $1`, args: []any{adminTenantID}},
		{label: "users", query: `DELETE FROM users WHERE id <> ALL($1::uuid[])`, args: []any{keptUserIDs}},
	}
	for _, item := range queries {
		if err := deleteCount(item.label, item.query, item.args...); err != nil {
			return nil, err
		}
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO tenant_memberships(tenant_id, user_id, role, is_active)
		SELECT $1, unnest($2::uuid[]), 'tenant_admin', TRUE
		ON CONFLICT (tenant_id, user_id)
		DO UPDATE SET role = EXCLUDED.role, is_active = TRUE, updated_at = NOW()
	`, adminTenantID, keptUserIDs); err != nil {
		return nil, fmt.Errorf("restore admin memberships after local factory reset: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit local factory reset: %w", err)
	}

	return &FactoryResetLocalSummary{
		AdminTenantID: adminTenantID,
		KeptUserIDs:   keptUserIDs,
		Deleted:       deleted,
	}, nil
}

func sanitizeReadOnlyQuery(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", fmt.Errorf("sql query is required")
	}
	if strings.Contains(trimmed, ";") {
		return "", fmt.Errorf("only a single SQL statement is allowed")
	}
	normalized := strings.ToLower(trimmed)
	if !strings.HasPrefix(normalized, "select ") && !strings.HasPrefix(normalized, "with ") && !strings.HasPrefix(normalized, "explain ") {
		return "", fmt.Errorf("only read-only SQL statements are allowed")
	}
	forbiddenTokens := []string{
		" insert ",
		" update ",
		" delete ",
		" drop ",
		" alter ",
		" truncate ",
		" create ",
		" grant ",
		" revoke ",
		" copy ",
	}
	padded := " " + normalized + " "
	for _, token := range forbiddenTokens {
		if strings.Contains(padded, token) {
			return "", fmt.Errorf("read-only SQL query contains forbidden operation")
		}
	}
	return trimmed, nil
}

func normalizeDBValue(value any) any {
	switch typed := value.(type) {
	case nil:
		return nil
	case []byte:
		return string(typed)
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = normalizeDBValue(item)
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, normalizeDBValue(item))
		}
		return out
	default:
		return typed
	}
}
