package api

import (
	"context"
	"encoding/json"
	"fmt"
	"incidenthub/backend/internal/middleware"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

type caseStatusDefinition struct {
	ID       string `json:"id,omitempty"`
	Code     string `json:"code"`
	Label    string `json:"label"`
	Order    int    `json:"order"`
	IsClosed bool   `json:"is_closed"`
	Color    string `json:"color,omitempty"`
	TenantID string `json:"tenant_id,omitempty"`
}

func (h *Handler) ListCaseStatuses(c *echo.Context) error {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	items, err := h.loadCaseStatuses(c.Request().Context(), tenantID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load case statuses")
	}
	return c.JSON(http.StatusOK, map[string]any{"statuses": items})
}

func (h *Handler) UpsertCaseStatuses(c *echo.Context) error {
	identity, _ := middleware.GetIdentity(c)
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	var req upsertCaseStatusesRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if len(req.Statuses) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "statuses must not be empty")
	}

	normalized := make([]caseStatusDefinition, 0, len(req.Statuses))
	seen := make(map[string]struct{}, len(req.Statuses))
	for idx, input := range req.Statuses {
		code := normalizeCaseStatusCode(input.Code)
		if code == "" {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("invalid case status code at index %d", idx))
		}
		if _, exists := seen[code]; exists {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("duplicate case status code: %s", code))
		}
		seen[code] = struct{}{}
		label := strings.TrimSpace(input.Label)
		if label == "" {
			label = humanizeCaseStatusCode(code)
		}
		order := input.Order
		if order <= 0 {
			order = (idx + 1) * 10
		}
		color := strings.TrimSpace(input.Color)
		normalized = append(normalized, caseStatusDefinition{
			Code:     code,
			Label:    label,
			Order:    order,
			IsClosed: input.IsClosed,
			Color:    color,
			TenantID: tenantID.String(),
		})
	}

	existing, err := h.catalog.List(c.Request().Context(), repository.CatalogListParams{
		Kind:     "case_statuses",
		TenantID: &tenantID,
		Limit:    500,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load existing statuses")
	}
	for _, item := range existing {
		if item.TenantID == nil {
			continue
		}
		_ = h.catalog.Delete(c.Request().Context(), "case_statuses", item.ID, &tenantID)
	}

	for _, status := range normalized {
		data := map[string]any{
			"code":      status.Code,
			"label":     status.Label,
			"order":     status.Order,
			"is_closed": status.IsClosed,
			"color":     status.Color,
		}
		if _, createErr := h.catalog.Create(c.Request().Context(), repository.CatalogCreateParams{
			TenantID:  &tenantID,
			Kind:      "case_statuses",
			Data:      data,
			CreatedBy: &identity.UserID,
		}); createErr != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to save case statuses")
		}
	}

	_ = h.audits.Log(c.Request().Context(), &tenantID, &identity.UserID, "case_statuses_update", "catalog", nil, map[string]any{
		"count": len(normalized),
	})

	items, err := h.loadCaseStatuses(c.Request().Context(), tenantID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load updated statuses")
	}
	return c.JSON(http.StatusOK, map[string]any{"statuses": items})
}

func (h *Handler) loadCaseStatuses(ctx context.Context, tenantID uuid.UUID) ([]caseStatusDefinition, error) {
	items, err := h.catalog.List(ctx, repository.CatalogListParams{
		Kind:          "case_statuses",
		TenantID:      &tenantID,
		IncludeGlobal: true,
		Limit:         500,
	})
	if err != nil {
		return nil, err
	}

	if len(items) == 0 {
		defaults := defaultCaseStatuses()
		defaultsCopy := make([]caseStatusDefinition, len(defaults))
		copy(defaultsCopy, defaults)
		for idx := range defaultsCopy {
			defaultsCopy[idx].TenantID = tenantID.String()
		}
		return defaultsCopy, nil
	}

	globalMap := make(map[string]caseStatusDefinition)
	tenantMap := make(map[string]caseStatusDefinition)

	for _, item := range items {
		status := parseCaseStatusCatalogItem(item)
		if status.Code == "" {
			continue
		}
		if item.TenantID == nil {
			if _, exists := globalMap[status.Code]; !exists {
				globalMap[status.Code] = status
			}
			continue
		}
		tenantMap[status.Code] = status
	}

	if len(globalMap) == 0 && len(tenantMap) == 0 {
		defaults := defaultCaseStatuses()
		defaultsCopy := make([]caseStatusDefinition, len(defaults))
		copy(defaultsCopy, defaults)
		for idx := range defaultsCopy {
			defaultsCopy[idx].TenantID = tenantID.String()
		}
		return defaultsCopy, nil
	}

	// Tenant configuration replaces global catalog statuses completely.
	// Global defaults are used only when tenant-specific list is absent.
	if len(tenantMap) > 0 {
		result := make([]caseStatusDefinition, 0, len(tenantMap))
		for _, status := range tenantMap {
			result = append(result, status)
		}
		sortCaseStatuses(result)
		return result, nil
	}

	result := make([]caseStatusDefinition, 0, len(globalMap))
	for _, status := range globalMap {
		result = append(result, status)
	}
	sortCaseStatuses(result)
	return result, nil
}

func (h *Handler) resolveCaseStatus(ctx context.Context, tenantID uuid.UUID, requested string) (string, error) {
	statuses, err := h.loadCaseStatuses(ctx, tenantID)
	if err != nil {
		return "", err
	}
	normalizedRequested := normalizeCaseStatusCode(requested)
	if normalizedRequested == "" {
		for _, item := range statuses {
			if !item.IsClosed {
				return item.Code, nil
			}
		}
		if len(statuses) > 0 {
			return statuses[0].Code, nil
		}
		return "new", nil
	}

	for _, item := range statuses {
		if item.Code == normalizedRequested {
			return normalizedRequested, nil
		}
	}
	return "", fmt.Errorf("unknown case status %q", normalizedRequested)
}

func defaultCaseStatuses() []caseStatusDefinition {
	return []caseStatusDefinition{
		{Code: "new", Label: "New", Order: 10, IsClosed: false, Color: "#3b82f6"},
		{Code: "open", Label: "Open", Order: 20, IsClosed: false, Color: "#0ea5e9"},
		{Code: "analysis", Label: "Analysis", Order: 30, IsClosed: false, Color: "#2563eb"},
		{Code: "response", Label: "Response", Order: 40, IsClosed: false, Color: "#ea580c"},
		{Code: "post_incident", Label: "Post-incident", Order: 50, IsClosed: false, Color: "#7c3aed"},
		{Code: "infrastructure_hardening", Label: "Infrastructure Hardening", Order: 60, IsClosed: false, Color: "#0891b2"},
		{Code: "review", Label: "Review", Order: 70, IsClosed: false, Color: "#475569"},
		{Code: "resolved", Label: "Resolved", Order: 80, IsClosed: true, Color: "#22c55e"},
		{Code: "closed", Label: "Closed", Order: 90, IsClosed: true, Color: "#64748b"},
	}
}

func parseCaseStatusCatalogItem(item models.CatalogItem) caseStatusDefinition {
	status := caseStatusDefinition{
		ID:    item.ID.String(),
		Order: 1000,
	}
	if item.TenantID != nil {
		status.TenantID = item.TenantID.String()
	}
	code := normalizeCaseStatusCode(stringFromMap(item.Data, "code", "id", "status"))
	status.Code = code
	status.Label = strings.TrimSpace(stringFromMap(item.Data, "label", "name"))
	if status.Label == "" {
		status.Label = humanizeCaseStatusCode(code)
	}
	if parsedOrder, ok := intFromMap(item.Data, "order", "rank", "position"); ok {
		status.Order = parsedOrder
	}
	if parsedClosed, ok := boolFromMap(item.Data, "is_closed", "closed"); ok {
		status.IsClosed = parsedClosed
	}
	status.Color = strings.TrimSpace(stringFromMap(item.Data, "color"))
	return status
}

func sortCaseStatuses(items []caseStatusDefinition) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Order == items[j].Order {
			return items[i].Code < items[j].Code
		}
		return items[i].Order < items[j].Order
	})
}

func normalizeCaseStatusCode(raw string) string {
	normalized := strings.TrimSpace(strings.ToLower(raw))
	if normalized == "" {
		return ""
	}
	normalized = strings.ReplaceAll(normalized, " ", "_")
	builder := strings.Builder{}
	for _, r := range normalized {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			builder.WriteRune(r)
		}
	}
	return strings.Trim(builder.String(), "_-")
}

func humanizeCaseStatusCode(code string) string {
	trimmed := strings.TrimSpace(code)
	if trimmed == "" {
		return "Status"
	}
	parts := strings.Fields(strings.ReplaceAll(trimmed, "_", " "))
	for i := range parts {
		parts[i] = strings.ToUpper(parts[i][:1]) + strings.ToLower(parts[i][1:])
	}
	return strings.Join(parts, " ")
}

func boolFromMap(payload map[string]any, keys ...string) (flag bool, found bool) {
	for _, key := range keys {
		value, ok := payload[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case bool:
			return typed, true
		case string:
			normalized := strings.TrimSpace(strings.ToLower(typed))
			if normalized == "true" || normalized == "1" || normalized == "yes" {
				return true, true
			}
			if normalized == "false" || normalized == "0" || normalized == "no" {
				return false, true
			}
		case json.Number:
			i, err := typed.Int64()
			if err == nil {
				return i != 0, true
			}
		case float64:
			return typed != 0, true
		case int:
			return typed != 0, true
		}
	}
	return false, false
}

func intFromMap(payload map[string]any, keys ...string) (int, bool) {
	for _, key := range keys {
		value, ok := payload[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case int:
			return typed, true
		case int64:
			return int(typed), true
		case float64:
			return int(typed), true
		case json.Number:
			i, err := typed.Int64()
			if err == nil {
				return int(i), true
			}
		case string:
			parsed, err := strconv.Atoi(strings.TrimSpace(typed))
			if err == nil {
				return parsed, true
			}
		}
	}
	return 0, false
}
