package api

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"incidenthub/backend/internal/middleware"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

type activityLivestreamItem struct {
	ID          string `json:"id"`
	Entity      string `json:"entity"`
	EntityID    string `json:"entity_id"`
	Action      string `json:"action"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Severity    string `json:"severity,omitempty"`
	Status      string `json:"status,omitempty"`
	Source      string `json:"source,omitempty"`
	AssigneeID  string `json:"assignee_id,omitempty"`
	CaseID      string `json:"case_id,omitempty"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type activityLivestreamResponse struct {
	Items       []activityLivestreamItem `json:"items"`
	GeneratedAt string                   `json:"generated_at"`
	Limit       int                      `json:"limit"`
	Offset      int                      `json:"offset"`
	HasMore     bool                     `json:"has_more"`
}

func (h *Handler) GetActivityLivestream(c *echo.Context) error {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}

	limit, offset, err := parseActivityLivestreamPagination(c)
	if err != nil {
		return err
	}

	includeCases, includeAlerts, includeTasks := parseLivestreamKinds(c.QueryParam("types"))
	query := strings.ToLower(strings.TrimSpace(c.QueryParam("q")))

	assigneeFilter, err := parseActivityLivestreamAssigneeFilter(c)
	if err != nil {
		return err
	}

	since, err := parseActivityLivestreamSince(c)
	if err != nil {
		return err
	}

	targetMatches := offset + limit + 1
	if targetMatches < 200 {
		targetMatches = 200
	}

	items, err := h.collectActivityLivestreamItems(
		c.Request().Context(),
		tenantID,
		targetMatches,
		includeCases,
		includeAlerts,
		includeTasks,
		query,
		since,
		assigneeFilter,
	)
	if err != nil {
		return err
	}

	sortActivityLivestreamItems(items)
	items, hasMore := paginateActivityLivestreamItems(items, offset, limit)

	return c.JSON(http.StatusOK, activityLivestreamResponse{
		Items:       items,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Limit:       limit,
		Offset:      offset,
		HasMore:     hasMore,
	})
}

func parseActivityLivestreamPagination(c *echo.Context) (limit int, offset int, _ error) {
	limit = 100
	if raw := strings.TrimSpace(c.QueryParam("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			return 0, 0, echo.NewHTTPError(http.StatusBadRequest, "invalid limit")
		}
		if parsed > 500 {
			parsed = 500
		}
		limit = parsed
	}
	offset = 0
	if raw := strings.TrimSpace(c.QueryParam("offset")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			return 0, 0, echo.NewHTTPError(http.StatusBadRequest, "invalid offset")
		}
		offset = parsed
	}
	return limit, offset, nil
}

func parseActivityLivestreamAssigneeFilter(c *echo.Context) (string, error) {
	raw := strings.TrimSpace(c.QueryParam("assignee_id"))
	if raw == "" {
		return "", nil
	}
	parsed, err := uuid.Parse(raw)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusBadRequest, "invalid assignee_id")
	}
	return parsed.String(), nil
}

func parseActivityLivestreamSince(c *echo.Context) (time.Time, error) {
	raw := strings.TrimSpace(c.QueryParam("since"))
	if raw == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, echo.NewHTTPError(http.StatusBadRequest, "invalid since")
	}
	return parsed.UTC(), nil
}

func buildActivityLivestreamItem(
	entity string,
	id uuid.UUID,
	createdAt time.Time,
	updatedAt time.Time,
	title string,
	description string,
	severity string,
	status string,
	source string,
) activityLivestreamItem {
	return activityLivestreamItem{
		ID:          entity + ":" + id.String(),
		Entity:      entity,
		EntityID:    id.String(),
		Action:      activityAction(createdAt, updatedAt),
		Title:       title,
		Description: description,
		Severity:    strings.ToLower(strings.TrimSpace(severity)),
		Status:      strings.TrimSpace(status),
		Source:      strings.TrimSpace(source),
		CreatedAt:   createdAt.UTC().Format(time.RFC3339),
		UpdatedAt:   updatedAt.UTC().Format(time.RFC3339),
	}
}

func sortActivityLivestreamItems(items []activityLivestreamItem) {
	sort.SliceStable(items, func(i, j int) bool {
		leftUpdated, leftErr := time.Parse(time.RFC3339, items[i].UpdatedAt)
		rightUpdated, rightErr := time.Parse(time.RFC3339, items[j].UpdatedAt)
		if leftErr == nil && rightErr == nil {
			if !leftUpdated.Equal(rightUpdated) {
				return leftUpdated.After(rightUpdated)
			}
		}
		return items[i].EntityID < items[j].EntityID
	})
}

func paginateActivityLivestreamItems(items []activityLivestreamItem, offset int, limit int) ([]activityLivestreamItem, bool) {
	hasMore := len(items) > offset+limit
	if offset >= len(items) {
		return []activityLivestreamItem{}, hasMore
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end], hasMore
}

func (h *Handler) collectActivityLivestreamItems(
	ctx context.Context,
	tenantID uuid.UUID,
	targetMatches int,
	includeCases bool,
	includeAlerts bool,
	includeTasks bool,
	query string,
	since time.Time,
	assigneeFilter string,
) ([]activityLivestreamItem, error) {
	const fetchBatchSize = 200

	items := make([]activityLivestreamItem, 0, targetMatches*2)
	casesDone := !includeCases || h.cases == nil
	alertsDone := !includeAlerts || h.alerts == nil
	tasksDone := !includeTasks || h.tasks == nil
	caseOffset := 0
	alertOffset := 0
	taskOffset := 0

	for len(items) < targetMatches && (!casesDone || !alertsDone || !tasksDone) {
		if !casesDone && h.cases != nil {
			cases, err := h.cases.ListByTenantWithAssignedSorted(ctx, tenantID, fetchBatchSize, caseOffset, "all", nil, "updated_at", "desc")
			if err != nil {
				return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to load case stream")
			}
			for _, item := range cases {
				streamItem := buildActivityLivestreamItem(
					"case",
					item.ID,
					item.CreatedAt,
					item.UpdatedAt,
					item.Title,
					item.Description,
					item.Severity,
					item.Status,
					item.Source,
				)
				if item.AssignedTo != nil {
					streamItem.AssigneeID = item.AssignedTo.String()
				}
				if livestreamItemMatches(streamItem, query, since, assigneeFilter) {
					items = append(items, streamItem)
				}
			}
			caseOffset += len(cases)
			if len(cases) < fetchBatchSize {
				casesDone = true
			}
		}

		if !alertsDone && h.alerts != nil {
			alerts, err := h.alerts.ListByTenantWithAssignedSorted(ctx, tenantID, fetchBatchSize, alertOffset, "all", nil, "updated_at", "desc")
			if err != nil {
				return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to load alert stream")
			}
			for _, item := range alerts {
				streamItem := buildActivityLivestreamItem(
					"alert",
					item.ID,
					item.CreatedAt,
					item.UpdatedAt,
					item.Title,
					item.Description,
					item.Severity,
					item.Status,
					item.Source,
				)
				if item.AssignedTo != nil {
					streamItem.AssigneeID = item.AssignedTo.String()
				}
				if item.CaseID != nil {
					streamItem.CaseID = item.CaseID.String()
				}
				if livestreamItemMatches(streamItem, query, since, assigneeFilter) {
					items = append(items, streamItem)
				}
			}
			alertOffset += len(alerts)
			if len(alerts) < fetchBatchSize {
				alertsDone = true
			}
		}

		if !tasksDone && h.tasks != nil {
			tasks, err := h.tasks.ListByTenant(ctx, tenantID, fetchBatchSize, taskOffset)
			if err != nil {
				return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to load task stream")
			}
			for _, item := range tasks {
				streamItem := buildActivityLivestreamItem(
					"task",
					item.ID,
					item.CreatedAt,
					item.UpdatedAt,
					item.Title,
					item.Description,
					"",
					item.Status,
					"",
				)
				streamItem.CaseID = item.CaseID.String()
				if item.AssigneeID != nil {
					streamItem.AssigneeID = item.AssigneeID.String()
				}
				if livestreamItemMatches(streamItem, query, since, assigneeFilter) {
					items = append(items, streamItem)
				}
			}
			taskOffset += len(tasks)
			if len(tasks) < fetchBatchSize {
				tasksDone = true
			}
		}
	}

	return items, nil
}

func parseLivestreamKinds(raw string) (includeCases bool, includeAlerts bool, includeTasks bool) {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		return true, true, true
	}
	for _, chunk := range strings.Split(value, ",") {
		switch strings.TrimSpace(chunk) {
		case "case", "cases":
			includeCases = true
		case "alert", "alerts":
			includeAlerts = true
		case "task", "tasks":
			includeTasks = true
		}
	}
	if !includeCases && !includeAlerts && !includeTasks {
		return true, true, true
	}
	return includeCases, includeAlerts, includeTasks
}

func activityAction(createdAt, updatedAt time.Time) string {
	if updatedAt.Sub(createdAt) <= time.Second && updatedAt.Sub(createdAt) >= -time.Second {
		return "created"
	}
	return "updated"
}

func livestreamItemMatches(item activityLivestreamItem, query string, since time.Time, assigneeID string) bool {
	if assigneeID != "" && item.AssigneeID != assigneeID {
		return false
	}
	if !since.IsZero() {
		updatedAt, err := time.Parse(time.RFC3339, item.UpdatedAt)
		if err != nil || updatedAt.Before(since) {
			return false
		}
	}
	if query == "" {
		return true
	}
	haystack := strings.ToLower(strings.Join([]string{
		item.Entity,
		item.Action,
		item.EntityID,
		item.Title,
		item.Description,
		item.Severity,
		item.Status,
		item.Source,
		item.AssigneeID,
		item.CaseID,
	}, " "))
	return strings.Contains(haystack, query)
}
