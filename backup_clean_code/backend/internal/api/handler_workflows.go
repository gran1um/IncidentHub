package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"incidenthub/backend/internal/middleware"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"
	"incidenthub/backend/internal/workflow"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v5"
)

const workflowHistoryRetentionLimit = 1000

// Sentinel errors for workflow execution (err113).
var (
	errWorkflowRunStartFailed       = errors.New("failed to start workflow run")
	errWorkflowRuntimeNotConfigured = errors.New("workflow runtime is not configured")
	errWorkflowRunFinishFailed      = errors.New("failed to finish workflow run")
)

type workflowTestRunRequest struct {
	Input      map[string]any `json:"input"`
	Definition map[string]any `json:"definition"`
}

type workflowRunRequest struct {
	Input map[string]any `json:"input"`
}

func (h *Handler) TestRunWorkflow(c *echo.Context) error {
	if h.catalog == nil || h.workflowRuns == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "workflow runtime is not configured")
	}
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	identity, _ := middleware.GetIdentity(c)
	workflowID, err := uuid.Parse(strings.TrimSpace(c.Param("workflowID")))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid workflow id")
	}
	item, kind, err := h.resolveWorkflowCatalogItem(c.Request().Context(), tenantID, workflowID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, "workflow not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load workflow")
	}

	var req workflowTestRunRequest
	if bindErr := c.Bind(&req); bindErr != nil && !errors.Is(bindErr, io.EOF) {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	definitionValue := pickWorkflowDefinitionSource(req.Definition, item.Data)
	definition, err := workflow.NormalizeDefinition(definitionValue)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	finished, ok, err := h.executeWorkflowRun(c.Request().Context(), executeWorkflowRunParams{
		TenantID:     tenantID,
		WorkflowKind: kind,
		WorkflowID:   item.ID,
		Trigger:      "manual_test",
		Definition:   definition,
		Input:        req.Input,
		WorkflowData: item.Data,
		CreatedBy:    &identity.UserID,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{
		"ok":              ok,
		"retention_limit": workflowHistoryRetentionLimit,
		"run":             workflowRunToPayload(*finished),
	})
}

func (h *Handler) RunWorkflow(c *echo.Context) error {
	if h.catalog == nil || h.workflowRuns == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "workflow runtime is not configured")
	}
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	identity, _ := middleware.GetIdentity(c)
	workflowID, err := uuid.Parse(strings.TrimSpace(c.Param("workflowID")))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid workflow id")
	}
	item, kind, err := h.resolveWorkflowCatalogItem(c.Request().Context(), tenantID, workflowID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, "workflow not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load workflow")
	}

	var req workflowRunRequest
	if bindErr := c.Bind(&req); bindErr != nil && !errors.Is(bindErr, io.EOF) {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	definition, err := workflow.NormalizeDefinition(pickWorkflowDefinitionSource(nil, item.Data))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	finished, ok, err := h.executeWorkflowRun(c.Request().Context(), executeWorkflowRunParams{
		TenantID:     tenantID,
		WorkflowKind: kind,
		WorkflowID:   item.ID,
		Trigger:      "manual",
		Definition:   definition,
		Input:        req.Input,
		WorkflowData: item.Data,
		CreatedBy:    &identity.UserID,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]any{
		"ok":              ok,
		"retention_limit": workflowHistoryRetentionLimit,
		"run":             workflowRunToPayload(*finished),
	})
}

type executeWorkflowRunParams struct {
	TenantID     uuid.UUID
	WorkflowKind string
	WorkflowID   uuid.UUID
	Trigger      string
	Definition   *workflow.Definition
	Input        map[string]any
	WorkflowData map[string]any
	CreatedBy    *uuid.UUID
}

func (h *Handler) executeWorkflowRun(ctx context.Context, p executeWorkflowRunParams) (*models.WorkflowRun, bool, error) {
	run, err := h.workflowRuns.Create(ctx, repository.CreateWorkflowRunParams{
		TenantID:     p.TenantID,
		WorkflowID:   p.WorkflowID,
		WorkflowKind: p.WorkflowKind,
		Trigger:      p.Trigger,
		Status:       models.WorkflowRunStatusRunning,
		Input:        p.Input,
		CreatedBy:    p.CreatedBy,
	})
	if err != nil {
		return nil, false, errWorkflowRunStartFailed
	}

	var (
		result map[string]any
		runErr error
	)
	if h.workflowRuntime == nil || p.Definition == nil {
		return nil, false, errWorkflowRuntimeNotConfigured
	}
	definitionWithCreds := h.applyWorkflowCredentials(ctx, p.TenantID, p.Definition)
	resolvedDefinition, err := h.resolveWorkflowDefinitionVaultRefs(ctx, p.TenantID, definitionWithCreds)
	if err != nil {
		runErr = fmt.Errorf("resolve workflow vault secrets: %w", err)
	} else {
		result, runErr = h.workflowRuntime.Execute(ctx, resolvedDefinition, p.Input, workflow.ExecuteMeta{
			TenantID:   p.TenantID,
			WorkflowID: p.WorkflowID,
			RunID:      run.ID,
		})
	}
	if runErr != nil {
		finished, finishErr := h.workflowRuns.Finish(ctx, run.ID, p.TenantID, repository.FinishWorkflowRunParams{
			Status: models.WorkflowRunStatusFailed,
			Result: result,
			Error:  runErr.Error(),
		})
		if finishErr != nil {
			return nil, false, errWorkflowRunFinishFailed
		}
		return finished, false, nil
	}

	finished, err := h.workflowRuns.Finish(ctx, run.ID, p.TenantID, repository.FinishWorkflowRunParams{
		Status: models.WorkflowRunStatusSuccess,
		Result: result,
	})
	if err != nil {
		return nil, false, errWorkflowRunFinishFailed
	}

	return finished, true, nil
}

func (h *Handler) ListWorkflowRuns(c *echo.Context) error {
	if h.catalog == nil || h.workflowRuns == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "workflow runtime is not configured")
	}
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	workflowID, err := uuid.Parse(strings.TrimSpace(c.Param("workflowID")))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid workflow id")
	}
	if _, _, resolveErr := h.resolveWorkflowCatalogItem(c.Request().Context(), tenantID, workflowID); resolveErr != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, "workflow not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load workflow")
	}

	limit := 50
	if rawLimit := strings.TrimSpace(c.QueryParam("limit")); rawLimit != "" {
		parsed, parseErr := strconv.Atoi(rawLimit)
		if parseErr != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid limit")
		}
		limit = parsed
	}
	if limit <= 0 || limit > workflowHistoryRetentionLimit {
		limit = workflowHistoryRetentionLimit
	}

	runs, err := h.workflowRuns.ListByWorkflow(c.Request().Context(), tenantID, workflowID, limit)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list workflow runs")
	}
	payload := make([]map[string]any, 0, len(runs))
	for _, run := range runs {
		payload = append(payload, workflowRunToPayload(run))
	}
	return c.JSON(http.StatusOK, map[string]any{
		"runs":            payload,
		"retention_limit": workflowHistoryRetentionLimit,
	})
}

func (h *Handler) resolveWorkflowCatalogItem(ctx context.Context, tenantID, workflowID uuid.UUID) (*models.CatalogItem, string, error) {
	item, err := h.catalog.GetByID(ctx, "workflows", workflowID, &tenantID)
	if err != nil {
		return nil, "", err
	}
	return item, "workflows", nil
}

func workflowRunToPayload(run models.WorkflowRun) map[string]any {
	out := map[string]any{
		"id":            run.ID.String(),
		"tenant_id":     run.TenantID.String(),
		"workflow_id":   run.WorkflowID.String(),
		"workflow_kind": run.WorkflowKind,
		"trigger":       run.Trigger,
		"status":        string(run.Status),
		"started_at":    run.StartedAt,
		"duration_ms":   run.DurationMS,
		"input":         run.Input,
		"result":        run.Result,
		"error":         run.Error,
		"created_at":    run.CreatedAt,
		"updated_at":    run.UpdatedAt,
	}
	if run.FinishedAt != nil {
		out["finished_at"] = *run.FinishedAt
	}
	if run.CreatedBy != nil {
		out["created_by"] = run.CreatedBy.String()
	}
	return out
}

func pickWorkflowDefinitionSource(requestDefinition map[string]any, data map[string]any) any {
	if len(requestDefinition) > 0 {
		return requestDefinition
	}
	if data == nil {
		return nil
	}
	if value, ok := data["definition"]; ok {
		return value
	}
	if value, ok := data["workflow_definition"]; ok {
		return value
	}
	if value, ok := data["workflow"]; ok {
		return value
	}
	return nil
}
