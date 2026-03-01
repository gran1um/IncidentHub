package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"incidenthub/backend/internal/middleware"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v5"
)

type asyncOperationStatusResponse struct {
	OperationID   string `json:"operation_id"`
	Status        string `json:"status"`
	Resource      string `json:"resource"`
	ResourceID    string `json:"resource_id,omitempty"`
	OperationType string `json:"operation_type"`
	AttemptCount  int    `json:"attempt_count"`
	MaxAttempts   int    `json:"max_attempts"`
	Error         string `json:"error,omitempty"`
	QueuedAt      string `json:"queued_at"`
	StartedAt     string `json:"started_at,omitempty"`
	FinishedAt    string `json:"finished_at,omitempty"`
	UpdatedAt     string `json:"updated_at"`
}

func (h *Handler) GetAsyncOperation(c *echo.Context) error {
	if h.asyncOperations == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "async operation tracking is not configured")
	}
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	operationID, err := uuid.Parse(strings.TrimSpace(c.Param("operationID")))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid operation id")
	}

	item, err := h.asyncOperations.GetByID(c.Request().Context(), tenantID, operationID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, "operation not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load operation")
	}
	return c.JSON(http.StatusOK, toAsyncOperationStatusResponse(item))
}

func toAsyncOperationStatusResponse(item *models.AsyncOperationRecord) asyncOperationStatusResponse {
	if item == nil {
		return asyncOperationStatusResponse{}
	}
	response := asyncOperationStatusResponse{
		OperationID:   item.OperationID.String(),
		Status:        string(item.Status),
		Resource:      item.Resource,
		OperationType: item.OperationType,
		AttemptCount:  item.AttemptCount,
		MaxAttempts:   item.MaxAttempts,
		Error:         strings.TrimSpace(item.LastError),
		QueuedAt:      item.QueuedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:     item.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	if item.ResourceID != nil {
		response.ResourceID = item.ResourceID.String()
	}
	if item.StartedAt != nil {
		response.StartedAt = item.StartedAt.UTC().Format(time.RFC3339Nano)
	}
	if item.FinishedAt != nil {
		response.FinishedAt = item.FinishedAt.UTC().Format(time.RFC3339Nano)
	}
	return response
}

func (h *Handler) recordQueuedAsyncOperation(ctx context.Context, operation AsyncOperation, resource string) error {
	if h.asyncOperations == nil {
		return nil
	}
	operationID, err := uuid.Parse(strings.TrimSpace(operation.OperationID))
	if err != nil {
		return err
	}
	tenantID, actorID, err := parseAsyncTenantAndActor(operation)
	if err != nil {
		return err
	}
	var resourceID *uuid.UUID
	if strings.TrimSpace(operation.ResourceID) != "" {
		parsed, parseErr := uuid.Parse(strings.TrimSpace(operation.ResourceID))
		if parseErr != nil {
			return parseErr
		}
		resourceID = &parsed
	}
	maxAttempts := 1
	if queueCapabilities, ok := h.asyncOps.(interface{ MaxAttempts() int }); ok {
		if value := queueCapabilities.MaxAttempts(); value > 0 {
			maxAttempts = value
		}
	}
	return h.asyncOperations.Create(ctx, repository.CreateAsyncOperationParams{
		OperationID:   operationID,
		TenantID:      tenantID,
		ActorID:       actorID,
		Resource:      resource,
		ResourceID:    resourceID,
		OperationType: string(operation.Type),
		Status:        models.AsyncOperationStatusQueued,
		MaxAttempts:   maxAttempts,
		Payload:       operation.Payload,
	})
}

func (h *Handler) markAsyncOperationFailed(ctx context.Context, operation AsyncOperation, operationErr error) {
	if h.asyncOperations == nil {
		return
	}
	operationID, err := uuid.Parse(strings.TrimSpace(operation.OperationID))
	if err != nil {
		return
	}
	lastError := ""
	if operationErr != nil {
		lastError = operationErr.Error()
	}
	_ = h.asyncOperations.MarkFailed(ctx, operationID, 1, lastError)
}
