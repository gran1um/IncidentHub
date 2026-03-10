package api

import (
	"context"
	"time"

	"incidenthub/backend/internal/logger"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"
)

func (h *Handler) recoverStaleConnectorHubDispatches(ctx context.Context) {
	if h == nil || h.connectorHubExecutions == nil {
		return
	}
	staleBefore := time.Now().UTC().Add(-h.connectorHubRunTimeout())
	retryAt := time.Now().UTC().Add(h.connectorHubRetryBaseBackoff())
	recovered, err := h.connectorHubExecutions.RecoverStaleDispatching(ctx, staleBefore, retryAt)
	if err != nil {
		logger.Warnf("connector hub recover stale dispatching failed: %v", err)
		return
	}
	for _, item := range recovered {
		message := "Dispatch timeout exceeded"
		eventType := "retry_scheduled"
		if item.Status == models.ConnectorHubExecutionStatusDeadLetter {
			message = "Dispatch timeout exceeded; execution moved to dead letter"
			eventType = "dead_letter"
		}
		if _, err := h.connectorHubExecutions.AddEvent(ctx, repository.AddConnectorHubExecutionEventParams{
			ExecutionID: item.ID,
			EventType:   eventType,
			Status:      string(item.Status),
			Message:     message,
		}); err != nil {
			logger.Warnf("connector hub add stale recovery event failed: execution=%s err=%v", item.ID.String(), err)
		}
	}
}
