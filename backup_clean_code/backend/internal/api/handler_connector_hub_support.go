package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"incidenthub/backend/internal/connectorhub"
	"incidenthub/backend/internal/middleware"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

func (h *Handler) buildConnectorHubDispatchInput(ctx context.Context, tenantID uuid.UUID, actorID *uuid.UUID, username string, c *echo.Context) (connectorHubDispatchInput, error) {
	var req connectorHubExecuteRequest
	if err := c.Bind(&req); err != nil {
		return connectorHubDispatchInput{}, echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	connectorID, err := uuid.Parse(strings.TrimSpace(req.ConnectorID))
	if err != nil {
		return connectorHubDispatchInput{}, echo.NewHTTPError(http.StatusBadRequest, "connector_id is required")
	}
	var method *models.CatalogItem
	if strings.TrimSpace(req.MethodID) != "" {
		methodID, parseErr := uuid.Parse(strings.TrimSpace(req.MethodID))
		if parseErr != nil {
			return connectorHubDispatchInput{}, echo.NewHTTPError(http.StatusBadRequest, "invalid method_id")
		}
		loaded, loadErr := h.catalog.GetByID(ctx, "connector_methods", methodID, &tenantID)
		if loadErr != nil {
			return connectorHubDispatchInput{}, echo.NewHTTPError(http.StatusNotFound, "connector method not found")
		}
		if loaded.RefID == nil || *loaded.RefID != connectorID {
			return connectorHubDispatchInput{}, echo.NewHTTPError(http.StatusBadRequest, "connector method does not belong to connector")
		}
		method = loaded
	}
	caseID, err := parseOptionalUUID(strings.TrimSpace(req.CaseID))
	if err != nil {
		return connectorHubDispatchInput{}, echo.NewHTTPError(http.StatusBadRequest, "invalid case_id")
	}
	alertID, err := parseOptionalUUID(strings.TrimSpace(req.AlertID))
	if err != nil {
		return connectorHubDispatchInput{}, echo.NewHTTPError(http.StatusBadRequest, "invalid alert_id")
	}
	inputPayload := normalizeMap(req.Input)
	metadata := normalizeMap(req.Metadata)
	if inputPayload == nil {
		inputPayload = map[string]any{}
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	return connectorHubDispatchInput{
		TenantID:      tenantID,
		ActorID:       actorID,
		ConnectorID:   connectorID,
		Method:        method,
		Action:        connectorhub.NormalizeActionKey(req.Action),
		CaseID:        caseID,
		AlertID:       alertID,
		Message:       strings.TrimSpace(req.Message),
		Input:         inputPayload,
		Metadata:      metadata,
		Author:        firstNonEmptyString(username, "connector-hub"),
		Conversation:  "",
		DryRun:        req.DryRun,
		ExecutionMode: "manual",
	}, nil
}

func (h *Handler) loadConnectorHubExecutionForRequest(c *echo.Context) (*models.ConnectorHubExecution, error) {
	if h.connectorHubExecutions == nil {
		return nil, echo.NewHTTPError(http.StatusServiceUnavailable, "connector hub execution repository is not configured")
	}
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	executionID, err := uuid.Parse(strings.TrimSpace(c.Param("executionID")))
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid executionID")
	}
	execution, err := h.connectorHubExecutions.GetByID(c.Request().Context(), executionID)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "connector hub execution not found")
	}
	if execution.TenantID != tenantID {
		return nil, echo.NewHTTPError(http.StatusNotFound, "connector hub execution not found")
	}
	return execution, nil
}

func connectorHubHTTPError(err error) error {
	if err == nil {
		return nil
	}
	httpErr := &echo.HTTPError{}
	if errors.As(err, &httpErr) {
		return httpErr
	}
	return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
}

func parsePositiveInt(raw string) (int, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, fmt.Errorf("empty")
	}
	var number int
	_, err := fmt.Sscanf(value, "%d", &number)
	if err != nil {
		return 0, err
	}
	if number <= 0 {
		return 0, fmt.Errorf("must be positive")
	}
	return number, nil
}

func (h *Handler) findConnectorMethodByAction(ctx context.Context, tenantID, connectorID uuid.UUID, actionKey string) (*models.CatalogItem, error) {
	if h.catalog == nil || actionKey == "" {
		return nil, nil
	}
	items, err := h.catalog.List(ctx, repository.CatalogListParams{
		Kind:     "connector_methods",
		TenantID: &tenantID,
		RefID:    &connectorID,
		Limit:    200,
	})
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		itemAction := connectorhub.NormalizeActionKey(stringFromMap(item.Data, "action", "action_key", "actionKey", "slug", "key", "name"))
		if itemAction == actionKey {
			return &item, nil
		}
	}
	return nil, nil
}
