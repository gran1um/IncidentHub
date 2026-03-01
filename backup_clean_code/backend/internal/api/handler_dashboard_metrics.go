package api

import (
	"context"
	"fmt"
	"incidenthub/backend/internal/middleware"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

const (
	dashboardMetricCatalogKind = "dashboard_metrics"
)

type dashboardMetricFiltersRequest struct {
	Statuses          []string `json:"statuses"`
	Severities        []string `json:"severities"`
	Categories        []string `json:"categories"`
	CreatedFrom       string   `json:"created_from"`
	CreatedTo         string   `json:"created_to"`
	ClosedFrom        string   `json:"closed_from"`
	ClosedTo          string   `json:"closed_to"`
	CreatedWithinHour *int     `json:"created_within_hours"`
	ClosedWithinHour  *int     `json:"closed_within_hours"`
	OverdueMinutes    *int     `json:"overdue_minutes"`
}

type upsertDashboardMetricRequest struct {
	Name        string                        `json:"name"`
	Description string                        `json:"description"`
	Source      string                        `json:"source"`
	Measure     string                        `json:"measure"`
	Enabled     *bool                         `json:"enabled"`
	Filters     dashboardMetricFiltersRequest `json:"filters"`
}

type dashboardMetricFilters struct {
	Statuses          []string `json:"statuses"`
	Severities        []string `json:"severities"`
	Categories        []string `json:"categories"`
	CreatedFrom       string   `json:"created_from"`
	CreatedTo         string   `json:"created_to"`
	ClosedFrom        string   `json:"closed_from"`
	ClosedTo          string   `json:"closed_to"`
	CreatedWithinHour int      `json:"created_within_hours"`
	ClosedWithinHour  int      `json:"closed_within_hours"`
	OverdueMinutes    int      `json:"overdue_minutes"`
}

type dashboardMetricConfig struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Source      string                 `json:"source"`
	Measure     string                 `json:"measure"`
	Enabled     bool                   `json:"enabled"`
	Filters     dashboardMetricFilters `json:"filters"`
}

type dashboardMetricPayload struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Source      string                 `json:"source"`
	Measure     string                 `json:"measure"`
	Enabled     bool                   `json:"enabled"`
	Filters     dashboardMetricFilters `json:"filters"`
	Value       float64                `json:"value"`
	Error       string                 `json:"error,omitempty"`
	CreatedAt   string                 `json:"created_at,omitempty"`
	UpdatedAt   string                 `json:"updated_at,omitempty"`
}

func (h *Handler) GetDashboardMetrics(c *echo.Context) error {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	if h.system == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "system repository unavailable")
	}

	overdueMinutes := 24 * 60
	if raw := strings.TrimSpace(c.QueryParam("overdue_minutes")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid overdue_minutes")
		}
		if parsed > 60*24*30 {
			parsed = 60 * 24 * 30
		}
		overdueMinutes = parsed
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 6*time.Second)
	defer cancel()

	openStatuses, closedStatuses := h.deriveCaseStatusSets(ctx, tenantID)
	snapshot, err := h.system.TenantDashboardMetrics(ctx, tenantID, openStatuses, closedStatuses, time.Duration(overdueMinutes)*time.Minute)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load dashboard metrics")
	}
	customMetrics, err := h.evaluateDashboardMetrics(ctx, tenantID, openStatuses, closedStatuses)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load custom dashboard metrics")
	}

	return c.JSON(http.StatusOK, map[string]any{
		"open_cases":          snapshot.OpenCases,
		"open_alerts":         snapshot.OpenAlerts,
		"overdue_cases":       snapshot.OverdueCases,
		"overdue_threshold_m": overdueMinutes,
		"sla_by_severity":     snapshot.SLABySeverity,
		"resolved_by_analyst": snapshot.ResolvedByAnalyst,
		"cases_by_status":     snapshot.CaseStatus,
		"cases_by_category":   snapshot.CaseCategory,
		"alerts_by_status":    snapshot.AlertStatus,
		"alerts_by_category":  snapshot.AlertCategory,
		"custom_metrics":      customMetrics,
		"generated_at":        time.Now().UTC().Format(time.RFC3339),
	})
}

func (h *Handler) ListDashboardCustomMetrics(c *echo.Context) error {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 6*time.Second)
	defer cancel()
	openStatuses, closedStatuses := h.deriveCaseStatusSets(ctx, tenantID)
	items, err := h.evaluateDashboardMetrics(ctx, tenantID, openStatuses, closedStatuses)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list custom dashboard metrics")
	}
	return c.JSON(http.StatusOK, map[string]any{
		"items": items,
	})
}

func (h *Handler) CreateDashboardCustomMetric(c *echo.Context) error {
	identity, _ := middleware.GetIdentity(c)
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}

	var req upsertDashboardMetricRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	config, err := normalizeDashboardMetricRequest(req, nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	item, err := h.catalog.Create(c.Request().Context(), repository.CatalogCreateParams{
		TenantID:  &tenantID,
		Kind:      dashboardMetricCatalogKind,
		OwnerID:   &identity.UserID,
		Data:      config.toCatalogData(),
		CreatedBy: &identity.UserID,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create custom dashboard metric")
	}
	payload := dashboardMetricPayloadFromItem(*item, config)
	return c.JSON(http.StatusCreated, payload)
}

func (h *Handler) UpdateDashboardCustomMetric(c *echo.Context) error {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	metricID, err := uuid.Parse(strings.TrimSpace(c.Param("metricID")))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid metric id")
	}

	current, err := h.catalog.GetByID(c.Request().Context(), dashboardMetricCatalogKind, metricID, &tenantID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "dashboard metric not found")
	}
	currentConfig := parseDashboardMetricConfig(current.Data)

	var req upsertDashboardMetricRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	nextConfig, err := normalizeDashboardMetricRequest(req, &currentConfig)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	item, err := h.catalog.Update(c.Request().Context(), dashboardMetricCatalogKind, metricID, &tenantID, repository.CatalogUpdateParams{
		Data: nextConfig.toCatalogData(),
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update custom dashboard metric")
	}
	payload := dashboardMetricPayloadFromItem(*item, nextConfig)
	return c.JSON(http.StatusOK, payload)
}

func (h *Handler) DeleteDashboardCustomMetric(c *echo.Context) error {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	metricID, err := uuid.Parse(strings.TrimSpace(c.Param("metricID")))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid metric id")
	}
	if err := h.catalog.Delete(c.Request().Context(), dashboardMetricCatalogKind, metricID, &tenantID); err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "dashboard metric not found")
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) evaluateDashboardMetrics(
	ctx context.Context,
	tenantID uuid.UUID,
	openStatuses []string,
	closedStatuses []string,
) ([]dashboardMetricPayload, error) {
	items, err := h.catalog.List(ctx, repository.CatalogListParams{
		Kind:     dashboardMetricCatalogKind,
		TenantID: &tenantID,
		Limit:    500,
	})
	if err != nil {
		return nil, err
	}
	out := make([]dashboardMetricPayload, 0, len(items))
	for _, item := range items {
		config := parseDashboardMetricConfig(item.Data)
		payload := dashboardMetricPayloadFromItem(item, config)
		if config.Enabled {
			value, evalErr := h.system.EvaluateDashboardCustomMetric(ctx, tenantID, openStatuses, closedStatuses, config.toRepositoryDefinition())
			if evalErr != nil {
				payload.Error = evalErr.Error()
			} else {
				payload.Value = value
			}
		}
		out = append(out, payload)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func (h *Handler) deriveCaseStatusSets(ctx context.Context, tenantID uuid.UUID) (openStatuses []string, closedStatuses []string) {
	statuses, err := h.loadCaseStatuses(ctx, tenantID)
	if err != nil {
		return nil, []string{"closed", "resolved", "done", "false_positive"}
	}
	open := make([]string, 0, len(statuses))
	closed := make([]string, 0, len(statuses))
	for _, status := range statuses {
		normalized := strings.ToLower(strings.TrimSpace(status.Code))
		if normalized == "" {
			continue
		}
		if status.IsClosed {
			closed = append(closed, normalized)
			continue
		}
		open = append(open, normalized)
	}
	if len(closed) == 0 {
		closed = []string{"closed", "resolved", "done", "false_positive"}
	}
	return normalizeTags(open), normalizeTags(closed)
}

func dashboardMetricPayloadFromItem(item models.CatalogItem, config dashboardMetricConfig) dashboardMetricPayload {
	payload := dashboardMetricPayload{
		ID:          item.ID.String(),
		Name:        config.Name,
		Description: config.Description,
		Source:      config.Source,
		Measure:     config.Measure,
		Enabled:     config.Enabled,
		Filters:     config.Filters,
		CreatedAt:   item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   item.UpdatedAt.Format(time.RFC3339),
	}
	return payload
}

func normalizeDashboardMetricRequest(
	req upsertDashboardMetricRequest,
	base *dashboardMetricConfig,
) (dashboardMetricConfig, error) {
	config := dashboardMetricConfig{}
	if base != nil {
		config = *base
	}

	if strings.TrimSpace(req.Name) != "" || base == nil {
		config.Name = strings.TrimSpace(req.Name)
	}
	if strings.TrimSpace(req.Description) != "" || base == nil {
		config.Description = strings.TrimSpace(req.Description)
	}
	if req.Enabled != nil {
		config.Enabled = *req.Enabled
	} else if base == nil {
		config.Enabled = true
	}

	sourceInput := strings.TrimSpace(req.Source)
	if sourceInput == "" && base != nil {
		sourceInput = base.Source
	}
	source, err := normalizeDashboardMetricSource(sourceInput)
	if err != nil {
		return dashboardMetricConfig{}, err
	}
	config.Source = source

	measureInput := strings.TrimSpace(req.Measure)
	if measureInput == "" && base != nil {
		measureInput = base.Measure
	}
	measure, err := normalizeDashboardMetricMeasure(measureInput, source)
	if err != nil {
		return dashboardMetricConfig{}, err
	}
	config.Measure = measure

	if strings.TrimSpace(config.Name) == "" {
		return dashboardMetricConfig{}, fmt.Errorf("name is required")
	}

	filters := config.Filters
	if base == nil {
		filters = dashboardMetricFilters{}
	}
	if req.Filters.Statuses != nil || base == nil {
		filters.Statuses = normalizeTags(req.Filters.Statuses)
	}
	if req.Filters.Severities != nil || base == nil {
		filters.Severities = normalizeTags(req.Filters.Severities)
	}
	if req.Filters.Categories != nil || base == nil {
		filters.Categories = normalizeTags(req.Filters.Categories)
	}
	if strings.TrimSpace(req.Filters.CreatedFrom) != "" || base == nil {
		filters.CreatedFrom = strings.TrimSpace(req.Filters.CreatedFrom)
	}
	if strings.TrimSpace(req.Filters.CreatedTo) != "" || base == nil {
		filters.CreatedTo = strings.TrimSpace(req.Filters.CreatedTo)
	}
	if strings.TrimSpace(req.Filters.ClosedFrom) != "" || base == nil {
		filters.ClosedFrom = strings.TrimSpace(req.Filters.ClosedFrom)
	}
	if strings.TrimSpace(req.Filters.ClosedTo) != "" || base == nil {
		filters.ClosedTo = strings.TrimSpace(req.Filters.ClosedTo)
	}
	if req.Filters.CreatedWithinHour != nil {
		filters.CreatedWithinHour = maxInt(*req.Filters.CreatedWithinHour)
	}
	if req.Filters.ClosedWithinHour != nil {
		filters.ClosedWithinHour = maxInt(*req.Filters.ClosedWithinHour)
	}
	if req.Filters.OverdueMinutes != nil {
		filters.OverdueMinutes = maxInt(*req.Filters.OverdueMinutes)
	}
	if filters.OverdueMinutes <= 0 {
		filters.OverdueMinutes = 24 * 60
	}
	config.Filters = filters

	if _, err := config.filtersToRepository(); err != nil {
		return dashboardMetricConfig{}, err
	}
	return config, nil
}

func parseDashboardMetricConfig(data map[string]any) dashboardMetricConfig {
	filtersRaw, _ := data["filters"].(map[string]any)
	config := dashboardMetricConfig{
		Name:        strings.TrimSpace(stringFromMap(data, "name")),
		Description: strings.TrimSpace(stringFromMap(data, "description")),
		Source:      strings.TrimSpace(stringFromMap(data, "source")),
		Measure:     strings.TrimSpace(stringFromMap(data, "measure")),
		Enabled:     true,
		Filters: dashboardMetricFilters{
			Statuses:   normalizeTags(anyStrings(filtersRaw["statuses"])),
			Severities: normalizeTags(anyStrings(filtersRaw["severities"])),
			Categories: normalizeTags(anyStrings(filtersRaw["categories"])),
			CreatedFrom: strings.TrimSpace(
				stringFromMap(filtersRaw, "created_from"),
			),
			CreatedTo: strings.TrimSpace(stringFromMap(filtersRaw, "created_to")),
			ClosedFrom: strings.TrimSpace(
				stringFromMap(filtersRaw, "closed_from"),
			),
			ClosedTo: strings.TrimSpace(stringFromMap(filtersRaw, "closed_to")),
		},
	}
	if parsedEnabled, ok := boolFromMap(data, "enabled"); ok {
		config.Enabled = parsedEnabled
	}
	if config.Source == "" {
		config.Source = "cases"
	}
	if config.Measure == "" {
		config.Measure = "count"
	}
	if parsed, ok := intFromMap(filtersRaw, "created_within_hours"); ok && parsed > 0 {
		config.Filters.CreatedWithinHour = parsed
	}
	if parsed, ok := intFromMap(filtersRaw, "closed_within_hours"); ok && parsed > 0 {
		config.Filters.ClosedWithinHour = parsed
	}
	if parsed, ok := intFromMap(filtersRaw, "overdue_minutes"); ok && parsed > 0 {
		config.Filters.OverdueMinutes = parsed
	}
	if config.Filters.OverdueMinutes <= 0 {
		config.Filters.OverdueMinutes = 24 * 60
	}
	return config
}

func (c dashboardMetricConfig) toRepositoryDefinition() repository.DashboardCustomMetricDefinition {
	definition, _ := c.filtersToRepository()
	return definition
}

func (c dashboardMetricConfig) filtersToRepository() (repository.DashboardCustomMetricDefinition, error) {
	createdFrom, err := parseOptionalMetricTime(c.Filters.CreatedFrom)
	if err != nil {
		return repository.DashboardCustomMetricDefinition{}, fmt.Errorf("invalid filters.created_from")
	}
	createdTo, err := parseOptionalMetricTime(c.Filters.CreatedTo)
	if err != nil {
		return repository.DashboardCustomMetricDefinition{}, fmt.Errorf("invalid filters.created_to")
	}
	closedFrom, err := parseOptionalMetricTime(c.Filters.ClosedFrom)
	if err != nil {
		return repository.DashboardCustomMetricDefinition{}, fmt.Errorf("invalid filters.closed_from")
	}
	closedTo, err := parseOptionalMetricTime(c.Filters.ClosedTo)
	if err != nil {
		return repository.DashboardCustomMetricDefinition{}, fmt.Errorf("invalid filters.closed_to")
	}
	if createdFrom != nil && createdTo != nil && createdFrom.After(*createdTo) {
		return repository.DashboardCustomMetricDefinition{}, fmt.Errorf("filters.created_from must be before filters.created_to")
	}
	if closedFrom != nil && closedTo != nil && closedFrom.After(*closedTo) {
		return repository.DashboardCustomMetricDefinition{}, fmt.Errorf("filters.closed_from must be before filters.closed_to")
	}
	return repository.DashboardCustomMetricDefinition{
		Source:            c.Source,
		Measure:           c.Measure,
		Statuses:          normalizeTags(c.Filters.Statuses),
		Severities:        normalizeTags(c.Filters.Severities),
		Categories:        normalizeTags(c.Filters.Categories),
		CreatedFrom:       createdFrom,
		CreatedTo:         createdTo,
		ClosedFrom:        closedFrom,
		ClosedTo:          closedTo,
		CreatedWithinHour: c.Filters.CreatedWithinHour,
		ClosedWithinHour:  c.Filters.ClosedWithinHour,
		OverdueMinutes:    c.Filters.OverdueMinutes,
	}, nil
}

func (c dashboardMetricConfig) toCatalogData() map[string]any {
	return map[string]any{
		"name":        c.Name,
		"description": c.Description,
		"source":      c.Source,
		"measure":     c.Measure,
		"enabled":     c.Enabled,
		"filters": map[string]any{
			"statuses":             normalizeTags(c.Filters.Statuses),
			"severities":           normalizeTags(c.Filters.Severities),
			"categories":           normalizeTags(c.Filters.Categories),
			"created_from":         strings.TrimSpace(c.Filters.CreatedFrom),
			"created_to":           strings.TrimSpace(c.Filters.CreatedTo),
			"closed_from":          strings.TrimSpace(c.Filters.ClosedFrom),
			"closed_to":            strings.TrimSpace(c.Filters.ClosedTo),
			"created_within_hours": maxInt(c.Filters.CreatedWithinHour),
			"closed_within_hours":  maxInt(c.Filters.ClosedWithinHour),
			"overdue_minutes":      maxInt(c.Filters.OverdueMinutes),
		},
	}
}

func normalizeDashboardMetricSource(input string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(input))
	switch normalized {
	case "cases", "alerts":
		return normalized, nil
	default:
		return "", fmt.Errorf("source must be one of: cases, alerts")
	}
}

func normalizeDashboardMetricMeasure(input, source string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(input))
	switch normalized {
	case "count":
		return normalized, nil
	case "avg_resolution_minutes", "overdue_count":
		if source != "cases" {
			return "", fmt.Errorf("measure %s is supported only for cases", normalized)
		}
		return normalized, nil
	default:
		return "", fmt.Errorf("measure must be one of: count, avg_resolution_minutes, overdue_count")
	}
}

func parseOptionalMetricTime(raw string) (*time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
		result := parsed.UTC()
		return &result, nil
	}
	if parsed, err := time.Parse("2006-01-02", trimmed); err == nil {
		result := parsed.UTC()
		return &result, nil
	}
	return nil, fmt.Errorf("invalid time")
}

func anyStrings(value any) []string {
	switch typed := value.(type) {
	case []string:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			out = append(out, strings.TrimSpace(item))
		}
		return out
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			out = append(out, strings.TrimSpace(fmt.Sprintf("%v", item)))
		}
		return out
	default:
		return []string{}
	}
}

func maxInt(value int) int {
	if value < 0 {
		return 0
	}
	return value
}
