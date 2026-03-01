package api

import (
	"math"
	"net/http"
	"strings"
	"time"

	"incidenthub/backend/internal/middleware"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

type caseListResponderStatusesPayload struct {
	Queued    int `json:"queued"`
	Running   int `json:"running"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
}

type caseListSummaryPayload struct {
	CaseID              string                           `json:"case_id"`
	OpenTasks           int                              `json:"open_tasks"`
	ClosedTasks         int                              `json:"closed_tasks"`
	TotalTasks          int                              `json:"total_tasks"`
	TaskProgressPercent int                              `json:"task_progress_percent"`
	NextOpenTaskDueAt   *string                          `json:"next_open_task_due_at"`
	OverdueOpenTasks    int                              `json:"overdue_open_tasks"`
	ResponderStatuses   caseListResponderStatusesPayload `json:"responder_statuses"`
}

func parseCaseIDsCSV(raw string, limit int) ([]uuid.UUID, []string, error) {
	items := strings.Split(strings.TrimSpace(raw), ",")
	if len(items) == 0 {
		return []uuid.UUID{}, []string{}, nil
	}
	seen := make(map[uuid.UUID]struct{}, len(items))
	caseIDs := make([]uuid.UUID, 0, len(items))
	caseRefs := make([]string, 0, len(items))
	for _, item := range items {
		candidate := strings.TrimSpace(item)
		if candidate == "" {
			continue
		}
		parsed, err := uuid.Parse(candidate)
		if err != nil {
			return nil, nil, echo.NewHTTPError(http.StatusBadRequest, "invalid case_ids")
		}
		if _, exists := seen[parsed]; exists {
			continue
		}
		seen[parsed] = struct{}{}
		caseIDs = append(caseIDs, parsed)
		caseRefs = append(caseRefs, strings.ToLower(parsed.String()))
		if limit > 0 && len(caseIDs) >= limit {
			break
		}
	}
	return caseIDs, caseRefs, nil
}

func (h *Handler) ListCaseListSummaries(c *echo.Context) error {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	caseIDs, caseRefs, err := parseCaseIDsCSV(c.QueryParam("case_ids"), 200)
	if err != nil {
		return err
	}
	if len(caseIDs) == 0 {
		return c.JSON(http.StatusOK, []caseListSummaryPayload{})
	}

	taskSummary, err := h.tasks.CountByCaseIDs(c.Request().Context(), tenantID, caseIDs)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load case task summary")
	}
	workflowSummary, err := h.workflowRuns.CountStatusesByCaseIDs(c.Request().Context(), tenantID, caseRefs)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load case responder summary")
	}

	out := make([]caseListSummaryPayload, 0, len(caseIDs))
	for _, caseID := range caseIDs {
		taskCounts := taskSummary[caseID]
		total := taskCounts.OpenCount + taskCounts.ClosedCount
		progress := 0
		if total > 0 {
			progress = int(math.Round((float64(taskCounts.ClosedCount) * 100.0) / float64(total)))
		}
		workflowCounts := workflowSummary[strings.ToLower(caseID.String())]
		nextOpenTaskDueAt := (*string)(nil)
		if taskCounts.NextOpenDueAt != nil {
			value := taskCounts.NextOpenDueAt.UTC().Format(time.RFC3339)
			nextOpenTaskDueAt = &value
		}
		out = append(out, caseListSummaryPayload{
			CaseID:              caseID.String(),
			OpenTasks:           taskCounts.OpenCount,
			ClosedTasks:         taskCounts.ClosedCount,
			TotalTasks:          total,
			TaskProgressPercent: progress,
			NextOpenTaskDueAt:   nextOpenTaskDueAt,
			OverdueOpenTasks:    taskCounts.OverdueOpenCount,
			ResponderStatuses: caseListResponderStatusesPayload{
				Queued:    workflowCounts.Queued,
				Running:   workflowCounts.Running,
				Completed: workflowCounts.Completed,
				Failed:    workflowCounts.Failed,
			},
		})
	}

	return c.JSON(http.StatusOK, out)
}
