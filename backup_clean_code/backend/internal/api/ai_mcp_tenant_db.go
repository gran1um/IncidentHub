package api

import (
	"context"
	"fmt"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
)

type tenantMCPTool string

const (
	tenantMCPToolOpenCaseStatuses tenantMCPTool = "open_case_statuses"
	tenantMCPToolCasesInWork      tenantMCPTool = "cases_in_work"
	tenantMCPToolRecentCases      tenantMCPTool = "recent_cases"
	tenantMCPToolRecentAlerts     tenantMCPTool = "recent_alerts"
	tenantMCPToolDashboardStats   tenantMCPTool = "dashboard_stats"
)

//nolint:gochecknoglobals // Default tool order is a static allowlist used by MCP runtime.
var tenantMCPDefaultToolOrder = []tenantMCPTool{
	tenantMCPToolOpenCaseStatuses,
	tenantMCPToolCasesInWork,
	tenantMCPToolRecentCases,
	tenantMCPToolRecentAlerts,
	tenantMCPToolDashboardStats,
}

type tenantMCPInvocation struct {
	Tool         string `json:"tool"`
	Status       string `json:"status"`
	ReadOnly     bool   `json:"read_only"`
	TenantScoped bool   `json:"tenant_scoped"`
	DurationMs   int64  `json:"duration_ms"`
	Error        string `json:"error,omitempty"`
}

type tenantMCPResult struct {
	Enabled     bool
	Data        map[string]any
	Allowlist   []string
	Invocations []tenantMCPInvocation
}

type tenantMCPServer struct {
	handler          *Handler
	enabled          bool
	orderedAllowlist []tenantMCPTool
	perToolLimit     int
}

func newTenantMCPServer(handler *Handler) *tenantMCPServer {
	if handler == nil {
		return nil
	}

	allowlist := parseTenantMCPAllowlist(handler.cfg.AI.MCPTools)
	if len(allowlist) == 0 {
		allowlist = append([]tenantMCPTool(nil), tenantMCPDefaultToolOrder...)
	}
	perToolLimit := handler.cfg.AI.MCPPerToolLimit
	if perToolLimit <= 0 {
		perToolLimit = 5
	}
	if perToolLimit > 100 {
		perToolLimit = 100
	}

	return &tenantMCPServer{
		handler:          handler,
		enabled:          handler.cfg.AI.MCPEnabled,
		orderedAllowlist: allowlist,
		perToolLimit:     perToolLimit,
	}
}

func (s *tenantMCPServer) Enabled() bool {
	return s != nil && s.enabled
}

func (s *tenantMCPServer) Allowlist() []string {
	if s == nil {
		return nil
	}
	out := make([]string, 0, len(s.orderedAllowlist))
	for _, tool := range s.orderedAllowlist {
		out = append(out, string(tool))
	}
	return out
}

func (s *tenantMCPServer) BuildTenantContext(ctx context.Context, tenantID uuid.UUID) tenantMCPResult {
	result := tenantMCPResult{
		Enabled:   s != nil && s.enabled,
		Allowlist: s.Allowlist(),
	}
	if s == nil || !s.enabled {
		return result
	}

	data := make(map[string]any, 6)
	invocations := make([]tenantMCPInvocation, 0, len(s.orderedAllowlist))
	var openStatusesCache []string

	for _, tool := range s.orderedAllowlist {
		invocations = append(invocations, s.invokeTool(ctx, tenantID, tool, data, &openStatusesCache))
	}

	result.Data = data
	result.Invocations = invocations
	return result
}

func (s *tenantMCPServer) invokeTool(
	ctx context.Context,
	tenantID uuid.UUID,
	tool tenantMCPTool,
	data map[string]any,
	openStatusesCache *[]string,
) tenantMCPInvocation {
	invocation := tenantMCPInvocation{
		Tool:         string(tool),
		Status:       "ok",
		ReadOnly:     true,
		TenantScoped: true,
	}
	startedAt := time.Now()
	var err error

	switch tool {
	case tenantMCPToolOpenCaseStatuses:
		var statuses []string
		statuses, err = openCaseStatusesForTenant(ctx, s.handler, tenantID)
		if err == nil {
			*openStatusesCache = statuses
			if len(statuses) > 0 {
				data[string(tenantMCPToolOpenCaseStatuses)] = statuses
			}
		}
	case tenantMCPToolCasesInWork:
		if s.handler.cases == nil {
			err = fmt.Errorf("cases repository unavailable")
			break
		}
		if len(*openStatusesCache) == 0 {
			var statuses []string
			statuses, err = openCaseStatusesForTenant(ctx, s.handler, tenantID)
			if err != nil {
				break
			}
			*openStatusesCache = statuses
		}
		var inWork int
		inWork, err = s.handler.cases.CountInWorkByTenant(ctx, tenantID, *openStatusesCache)
		if err == nil {
			data[string(tenantMCPToolCasesInWork)] = inWork
		}
	case tenantMCPToolRecentCases:
		if s.handler.cases == nil {
			err = fmt.Errorf("cases repository unavailable")
			break
		}
		var cases []models.Case
		cases, err = s.handler.cases.ListByTenant(ctx, tenantID, s.perToolLimit, 0)
		if err == nil {
			data[string(tenantMCPToolRecentCases)] = mapCasesForAIContext(cases)
		}
	case tenantMCPToolRecentAlerts:
		if s.handler.alerts == nil {
			err = fmt.Errorf("alerts repository unavailable")
			break
		}
		var alerts []models.Alert
		alerts, err = s.handler.alerts.ListByTenant(ctx, tenantID, s.perToolLimit, 0)
		if err == nil {
			data[string(tenantMCPToolRecentAlerts)] = mapAlertsForAIContext(alerts)
		}
	case tenantMCPToolDashboardStats:
		if s.handler.system == nil {
			err = fmt.Errorf("system repository unavailable")
			break
		}
		var dashboardStats repository.DashboardStats
		dashboardStats, err = s.handler.system.TenantDashboardStats(ctx, tenantID)
		if err == nil {
			data[string(tenantMCPToolDashboardStats)] = map[string]any{
				"active_cases":      dashboardStats.ActiveCases,
				"alerts_24h":        dashboardStats.Alerts24h,
				"resolved_today":    dashboardStats.ResolvedToday,
				"avg_response_min":  math.Round(dashboardStats.AvgResponseMin*100) / 100,
				"source":            "postgres",
				"note":              "active_cases here follow system defaults",
				"in_work_authority": "cases_in_work + open_case_statuses",
			}
		}
	default:
		err = fmt.Errorf("unsupported MCP tool %q", tool)
	}

	invocation.DurationMs = time.Since(startedAt).Milliseconds()
	if invocation.DurationMs < 0 {
		invocation.DurationMs = 0
	}
	if err != nil {
		invocation.Status = "error"
		invocation.Error = err.Error()
	}
	return invocation
}

func parseTenantMCPAllowlist(raw string) []tenantMCPTool {
	normalized := strings.NewReplacer(";", ",", "\n", ",", "\t", ",").Replace(raw)
	parts := strings.Split(normalized, ",")
	out := make([]tenantMCPTool, 0, len(parts))
	seen := make(map[tenantMCPTool]struct{}, len(parts))
	for _, part := range parts {
		candidate := tenantMCPTool(strings.ToLower(strings.TrimSpace(part)))
		if candidate == "" {
			continue
		}
		switch candidate {
		case tenantMCPToolOpenCaseStatuses,
			tenantMCPToolCasesInWork,
			tenantMCPToolRecentCases,
			tenantMCPToolRecentAlerts,
			tenantMCPToolDashboardStats:
		default:
			continue
		}
		if _, exists := seen[candidate]; exists {
			continue
		}
		seen[candidate] = struct{}{}
		out = append(out, candidate)
	}
	return out
}
