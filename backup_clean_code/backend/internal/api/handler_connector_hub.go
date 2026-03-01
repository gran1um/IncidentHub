package api

import (
	"net/http"
	"strings"

	"incidenthub/backend/internal/connectorhub"
	"incidenthub/backend/internal/logger"
	"incidenthub/backend/internal/middleware"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

type connectorHubExecuteRequest struct {
	ConnectorID string         `json:"connector_id"`
	MethodID    string         `json:"method_id"`
	Action      string         `json:"action"`
	CaseID      string         `json:"case_id"`
	AlertID     string         `json:"alert_id"`
	Message     string         `json:"message"`
	Input       map[string]any `json:"input"`
	Metadata    map[string]any `json:"metadata"`
	DryRun      bool           `json:"dry_run"`
}

type connectorHubDispatchInput struct {
	TenantID      uuid.UUID
	ActorID       *uuid.UUID
	ConnectorID   uuid.UUID
	Method        *models.CatalogItem
	Action        string
	CaseID        *uuid.UUID
	AlertID       *uuid.UUID
	Message       string
	Input         map[string]any
	Metadata      map[string]any
	Author        string
	Conversation  string
	DryRun        bool
	ExecutionMode string
}

type preparedConnectorHubDispatch struct {
	Input            connectorHubDispatchInput
	Connector        *models.CatalogItem
	MethodID         *uuid.UUID
	MethodName       string
	MethodKey        string
	ActionKey        string
	Message          string
	Metadata         map[string]any
	Request          map[string]any
	ThreadID         string
	ConversationID   string
	Author           string
	ConnectorName    string
	ConnectorChannel string
}

func (h *Handler) ExecuteConnectorHub(c *echo.Context) error {
	if h.connectorHubExecutions == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "connector hub execution repository is not configured")
	}
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	identity, _ := middleware.GetIdentity(c)

	input, err := h.buildConnectorHubDispatchInput(c.Request().Context(), tenantID, &identity.UserID, identity.Username, c)
	if err != nil {
		return err
	}
	prepared, err := h.prepareConnectorHubDispatch(c.Request().Context(), input)
	if err != nil {
		return connectorHubHTTPError(err)
	}
	queued, err := h.enqueuePreparedConnectorHubExecution(c.Request().Context(), prepared, "Execution accepted", "Execution queued for dispatch", map[string]any{
		"connector_id": prepared.Connector.ID.String(),
		"action":       prepared.ActionKey,
	})
	if err != nil {
		logger.Errorf("connector hub enqueue error: %v", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to queue connector hub execution: "+err.Error())
	}
	return c.JSON(http.StatusAccepted, queued)
}

func (h *Handler) ListConnectorHubExecutions(c *echo.Context) error {
	if h.connectorHubExecutions == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "connector hub execution repository is not configured")
	}
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	limit := 50
	if rawLimit := strings.TrimSpace(c.QueryParam("limit")); rawLimit != "" {
		if parsed, err := parsePositiveInt(rawLimit); err == nil {
			limit = parsed
		}
	}
	connectorID, err := parseOptionalUUID(strings.TrimSpace(c.QueryParam("connector_id")))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid connector_id")
	}
	caseID, err := parseOptionalUUID(strings.TrimSpace(c.QueryParam("case_id")))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid case_id")
	}
	alertID, err := parseOptionalUUID(strings.TrimSpace(c.QueryParam("alert_id")))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid alert_id")
	}
	items, err := h.connectorHubExecutions.List(c.Request().Context(), repository.ConnectorHubExecutionListParams{
		TenantID:      tenantID,
		ConnectorID:   connectorID,
		CaseID:        caseID,
		AlertID:       alertID,
		Action:        connectorhub.NormalizeActionKey(c.QueryParam("action")),
		Status:        strings.TrimSpace(strings.ToLower(c.QueryParam("status"))),
		ExecutionMode: strings.TrimSpace(strings.ToLower(c.QueryParam("execution_mode"))),
		Limit:         limit,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list connector hub executions")
	}
	return c.JSON(http.StatusOK, items)
}

func (h *Handler) GetConnectorHubExecution(c *echo.Context) error {
	execution, err := h.loadConnectorHubExecutionForRequest(c)
	if err != nil {
		return err
	}
	attempts, loadAttemptsErr := h.connectorHubExecutions.ListAttempts(c.Request().Context(), execution.ID, 20)
	if loadAttemptsErr != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list connector hub attempts")
	}
	events, loadEventsErr := h.connectorHubExecutions.ListEvents(c.Request().Context(), execution.ID, 50)
	if loadEventsErr != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list connector hub events")
	}
	return c.JSON(http.StatusOK, map[string]any{
		"execution": execution,
		"attempts":  attempts,
		"events":    events,
	})
}

func (h *Handler) ListConnectorHubExecutionEvents(c *echo.Context) error {
	execution, err := h.loadConnectorHubExecutionForRequest(c)
	if err != nil {
		return err
	}
	limit := 100
	if rawLimit := strings.TrimSpace(c.QueryParam("limit")); rawLimit != "" {
		if parsed, parseErr := parsePositiveInt(rawLimit); parseErr == nil {
			limit = parsed
		}
	}
	events, err := h.connectorHubExecutions.ListEvents(c.Request().Context(), execution.ID, limit)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list connector hub events")
	}
	return c.JSON(http.StatusOK, events)
}

func (h *Handler) RetryConnectorHubExecution(c *echo.Context) error {
	execution, err := h.loadConnectorHubExecutionForRequest(c)
	if err != nil {
		return err
	}
	updated, err := h.connectorHubExecutions.Retry(c.Request().Context(), execution.ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to retry connector hub execution")
	}
	if _, err := h.connectorHubExecutions.AddEvent(c.Request().Context(), repository.AddConnectorHubExecutionEventParams{
		ExecutionID: updated.ID,
		EventType:   "manual_retry",
		Status:      string(updated.Status),
		Message:     "Execution manually requeued",
		Data: map[string]any{
			"attempt_count": updated.AttemptCount,
			"max_attempts":  updated.MaxAttempts,
		},
	}); err != nil {
		logger.Warnf("connector hub add retry event failed: execution=%s err=%v", updated.ID.String(), err)
	}
	return c.JSON(http.StatusAccepted, updated)
}

func (h *Handler) CancelConnectorHubExecution(c *echo.Context) error {
	execution, err := h.loadConnectorHubExecutionForRequest(c)
	if err != nil {
		return err
	}
	updated, err := h.connectorHubExecutions.Cancel(c.Request().Context(), execution.ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to cancel connector hub execution")
	}
	if _, err := h.connectorHubExecutions.AddEvent(c.Request().Context(), repository.AddConnectorHubExecutionEventParams{
		ExecutionID: updated.ID,
		EventType:   "canceled",
		Status:      string(updated.Status),
		Message:     "Execution canceled by user",
	}); err != nil {
		logger.Warnf("connector hub add cancel event failed: execution=%s err=%v", updated.ID.String(), err)
	}
	return c.JSON(http.StatusOK, updated)
}

func (h *Handler) RestartConnectorHubExecution(c *echo.Context) error {
	execution, err := h.loadConnectorHubExecutionForRequest(c)
	if err != nil {
		return err
	}
	identity, _ := middleware.GetIdentity(c)
	metadata := normalizeMap(execution.Metadata)
	metadata["restarted_from_execution_id"] = execution.ID.String()
	queued, err := h.enqueueConnectorHubExecutionRecord(c.Request().Context(), repository.CreateConnectorHubExecutionParams{
		TenantID:         execution.TenantID,
		ActorID:          &identity.UserID,
		ConnectorID:      execution.ConnectorID,
		ConnectorName:    execution.ConnectorName,
		ConnectorChannel: execution.ConnectorChannel,
		MethodID:         execution.MethodID,
		MethodName:       execution.MethodName,
		MethodKey:        execution.MethodKey,
		Action:           execution.Action,
		CaseID:           execution.CaseID,
		AlertID:          execution.AlertID,
		ExecutionMode:    execution.ExecutionMode,
		DryRun:           execution.DryRun,
		Request:          execution.Request,
		Metadata:         metadata,
		MaxAttempts:      execution.MaxAttempts,
	}, "Execution accepted after restart", "Restarted execution queued for dispatch", map[string]any{
		"source_execution_id": execution.ID.String(),
	}, true)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to restart connector hub execution")
	}
	return c.JSON(http.StatusAccepted, queued)
}
