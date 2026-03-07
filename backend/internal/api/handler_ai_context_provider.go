package api

import (
	"context"
	"encoding/json"
	"fmt"
	"incidenthub/backend/internal/ai"
	"incidenthub/backend/internal/models"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type handlerTenantContextProvider struct {
	handler *Handler
}

func newHandlerTenantContextProvider(handler *Handler) ai.TenantContextProvider {
	return &handlerTenantContextProvider{handler: handler}
}

func (p *handlerTenantContextProvider) BuildContext(ctx context.Context, tenantIDRaw, question string) (ai.TenantContext, error) {
	if p == nil || p.handler == nil {
		return ai.TenantContext{}, nil
	}
	tenantID, err := uuid.Parse(strings.TrimSpace(tenantIDRaw))
	if err != nil {
		return ai.TenantContext{}, nil
	}

	data := map[string]any{
		"tenant_id":    tenantID.String(),
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"question":     strings.TrimSpace(question),
	}

	if p.handler.aiTenantMCP != nil && p.handler.aiTenantMCP.Enabled() {
		mcpResult := p.handler.aiTenantMCP.BuildTenantContext(ctx, tenantID)
		mergeContextData(data, mcpResult.Data)
		data["mcp"] = map[string]any{
			"enabled":       mcpResult.Enabled,
			"allowlist":     mcpResult.Allowlist,
			"read_only":     true,
			"tenant_scoped": true,
			"invocations":   mcpResult.Invocations,
		}
	} else {
		p.populateLegacyContext(ctx, tenantID, data)
	}
	sourceTitleParts := collectContextSourceTitleParts(data)

	sourceTitle := "Tenant operational context"
	if len(sourceTitleParts) > 0 {
		sourceTitle = sourceTitle + " (" + strings.Join(sourceTitleParts, ", ") + ")"
	}
	sources := []ai.Source{
		{
			Kind:  "postgres",
			ID:    "tenant:" + tenantID.String(),
			Title: sourceTitle,
		},
	}

	raw, marshalErr := json.Marshal(data)
	if marshalErr != nil {
		return ai.TenantContext{}, fmt.Errorf("marshal tenant ai context: %w", marshalErr)
	}
	return ai.TenantContext{
		Prompt:  truncateText(string(raw), 6000),
		Sources: sources,
	}, nil
}

func (p *handlerTenantContextProvider) populateLegacyContext(ctx context.Context, tenantID uuid.UUID, data map[string]any) {
	openStatuses, statusErr := openCaseStatusesForTenant(ctx, p.handler, tenantID)
	if statusErr == nil && len(openStatuses) > 0 {
		data["open_case_statuses"] = openStatuses
	}

	if p.handler.cases != nil {
		if inWorkCount, countErr := p.handler.cases.CountInWorkByTenant(ctx, tenantID, openStatuses); countErr == nil {
			data["cases_in_work"] = inWorkCount
		}
		if recentCases, casesErr := p.handler.cases.ListByTenant(ctx, tenantID, 5, 0); casesErr == nil {
			data["recent_cases"] = mapCasesForAIContext(recentCases)
		}
	}
	if p.handler.alerts != nil {
		if recentAlerts, alertsErr := p.handler.alerts.ListByTenant(ctx, tenantID, 5, 0); alertsErr == nil {
			data["recent_alerts"] = mapAlertsForAIContext(recentAlerts)
		}
	}
	if p.handler.system != nil {
		if stats, statsErr := p.handler.system.TenantDashboardStats(ctx, tenantID); statsErr == nil {
			data["dashboard_stats"] = map[string]any{
				"active_cases":      stats.ActiveCases,
				"alerts_24h":        stats.Alerts24h,
				"resolved_today":    stats.ResolvedToday,
				"avg_response_min":  math.Round(stats.AvgResponseMin*100) / 100,
				"source":            "postgres",
				"note":              "active_cases here follow system defaults",
				"in_work_authority": "cases_in_work + open_case_statuses",
			}
		}
	}
}

func mergeContextData(target map[string]any, source map[string]any) {
	if len(source) == 0 {
		return
	}
	for key, value := range source {
		target[key] = value
	}
}

func collectContextSourceTitleParts(data map[string]any) []string {
	out := make([]string, 0, 2)
	if inWork, ok := parseInt64(data["cases_in_work"]); ok {
		out = append(out, fmt.Sprintf("cases_in_work=%d", inWork))
	}
	stats, ok := data["dashboard_stats"].(map[string]any)
	if !ok {
		return out
	}
	if alerts24h, ok := parseInt64(stats["alerts_24h"]); ok {
		out = append(out, fmt.Sprintf("alerts_24h=%d", alerts24h))
	}
	return out
}

func parseInt64(value any) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int8:
		return int64(typed), true
	case int16:
		return int64(typed), true
	case int32:
		return int64(typed), true
	case int64:
		return typed, true
	case uint:
		if uint64(typed) > uint64(math.MaxInt64) {
			return 0, false
		}
		return int64(typed), true //nolint:gosec // bounded above by MaxInt64 check
	case uint8:
		return int64(typed), true
	case uint16:
		return int64(typed), true
	case uint32:
		return int64(typed), true
	case uint64:
		if typed > uint64(math.MaxInt64) {
			return 0, false
		}
		return int64(typed), true
	case float32:
		return int64(typed), true
	case float64:
		return int64(typed), true
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func openCaseStatusesForTenant(ctx context.Context, handler *Handler, tenantID uuid.UUID) ([]string, error) {
	if handler == nil || handler.catalog == nil {
		return nil, fmt.Errorf("catalog repository unavailable")
	}
	statuses, err := handler.loadCaseStatuses(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(statuses))
	for _, item := range statuses {
		if item.IsClosed {
			continue
		}
		code := strings.TrimSpace(item.Code)
		if code == "" {
			continue
		}
		out = append(out, code)
	}
	return out, nil
}

func mapCasesForAIContext(items []models.Case) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{
			"id":          item.ID.String(),
			"case_number": strings.TrimSpace(item.CaseNumber),
			"title":       strings.TrimSpace(item.Title),
			"status":      strings.TrimSpace(item.Status),
			"severity":    strings.TrimSpace(item.Severity),
			"priority":    strings.TrimSpace(item.Priority),
			"updated_at":  item.UpdatedAt.UTC().Format(time.RFC3339),
		})
	}
	return out
}

func mapAlertsForAIContext(items []models.Alert) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		caseID := ""
		if item.CaseID != nil {
			caseID = item.CaseID.String()
		}
		out = append(out, map[string]any{
			"id":         item.ID.String(),
			"case_id":    caseID,
			"title":      strings.TrimSpace(item.Title),
			"status":     strings.TrimSpace(item.Status),
			"severity":   strings.TrimSpace(item.Severity),
			"updated_at": item.UpdatedAt.UTC().Format(time.RFC3339),
		})
	}
	return out
}

func truncateText(value string, maxLen int) string {
	if maxLen <= 0 || len(value) <= maxLen {
		return value
	}
	return value[:maxLen]
}
