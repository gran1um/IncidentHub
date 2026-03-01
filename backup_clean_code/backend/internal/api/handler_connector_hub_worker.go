package api

import (
	"context"
	"strings"
	"time"

	"incidenthub/backend/internal/connectorhub"
	"incidenthub/backend/internal/logger"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"
)

func (h *Handler) processConnectorHubExecution(ctx context.Context, execution models.ConnectorHubExecution) error {
	attemptNo := execution.AttemptCount
	if attemptNo <= 0 {
		attemptNo = 1
	}
	startedAt := time.Now().UTC()
	attempt, err := h.connectorHubExecutions.CreateAttempt(ctx, repository.CreateConnectorHubExecutionAttemptParams{
		ExecutionID: execution.ID,
		AttemptNo:   attemptNo,
		Status:      string(models.ConnectorHubExecutionStatusDispatching),
		Request:     execution.Request,
		StartedAt:   startedAt,
	})
	if err != nil {
		return err
	}
	attemptNoValue := attempt.AttemptNo
	if _, eventErr := h.connectorHubExecutions.AddEvent(ctx, repository.AddConnectorHubExecutionEventParams{
		ExecutionID: execution.ID,
		AttemptNo:   &attemptNoValue,
		EventType:   "dispatching",
		Status:      string(models.ConnectorHubExecutionStatusDispatching),
		Message:     "Worker picked execution for dispatch",
		Data: map[string]any{
			"attempt_no": attempt.AttemptNo,
		},
	}); eventErr != nil {
		logger.Warnf("connector hub add dispatching event failed: execution=%s err=%v", execution.ID.String(), eventErr)
	}

	if execution.DryRun {
		finishedAt := time.Now().UTC()
		response := map[string]any{
			"reply":           strings.TrimSpace(stringFromMap(execution.Request, "message")),
			"conversation_id": strings.TrimSpace(stringFromMap(execution.Request, "conversation_id", "conversationId")),
			"metadata":        normalizeMap(firstMapValue(execution.Request, "metadata")),
		}
		if _, finishErr := h.connectorHubExecutions.FinishAttempt(ctx, attempt.ID, repository.FinishConnectorHubExecutionAttemptParams{
			Status:     string(models.ConnectorHubExecutionStatusDryRun),
			Response:   response,
			FinishedAt: finishedAt,
		}); finishErr != nil {
			return finishErr
		}
		completedExecution, markErr := h.connectorHubExecutions.MarkCompleted(ctx, execution.ID, models.ConnectorHubExecutionStatusDryRun, response, "dry_run", "", "", finishedAt)
		if markErr != nil {
			return markErr
		}
		h.emitConnectorHubExecutionCaseTimelineEvent(ctx, completedExecution)
		if _, eventErr := h.connectorHubExecutions.AddEvent(ctx, repository.AddConnectorHubExecutionEventParams{
			ExecutionID: execution.ID,
			AttemptNo:   &attemptNoValue,
			EventType:   "dry_run",
			Status:      string(models.ConnectorHubExecutionStatusDryRun),
			Message:     "Dry-run completed without provider dispatch",
		}); eventErr != nil {
			logger.Warnf("connector hub add dry-run event failed: execution=%s err=%v", execution.ID.String(), eventErr)
		}
		return nil
	}

	if h.outbound == nil {
		return h.failConnectorHubExecution(ctx, execution, attempt, "outbound connector service is not configured", "", "", "")
	}
	connector, err := h.resolveOutboundConnectorForTenant(ctx, execution.TenantID, execution.ConnectorID)
	if err != nil {
		return h.failConnectorHubExecution(ctx, execution, attempt, err.Error(), "", "", "")
	}
	request := connectorhub.RequestFromPayload(execution.Request)
	reply, err := h.outbound.Send(ctx, *connector, request)
	if err != nil {
		return h.failConnectorHubExecution(ctx, execution, attempt, err.Error(), "", "", "")
	}
	response := connectorhub.BuildSendResponsePayload(reply)
	providerStatus := firstNonEmptyString(strings.TrimSpace(stringFromMap(response, "provider_status")), "accepted")
	externalID := strings.TrimSpace(stringFromMap(response, "external_id"))
	correlationID := firstNonEmptyString(strings.TrimSpace(stringFromMap(response, "correlation_id")), strings.TrimSpace(reply.ConversationID))
	if _, acceptErr := h.connectorHubExecutions.MarkProviderAccepted(ctx, execution.ID, response, providerStatus, externalID, correlationID); acceptErr != nil {
		return acceptErr
	}
	if _, eventErr := h.connectorHubExecutions.AddEvent(ctx, repository.AddConnectorHubExecutionEventParams{
		ExecutionID: execution.ID,
		AttemptNo:   &attemptNoValue,
		EventType:   "provider_accepted",
		Status:      string(models.ConnectorHubExecutionStatusProviderAccepted),
		Message:     "Provider accepted execution request",
		Data: map[string]any{
			"provider_status": providerStatus,
			"external_id":     externalID,
			"correlation_id":  correlationID,
		},
	}); eventErr != nil {
		logger.Warnf("connector hub add provider accepted event failed: execution=%s err=%v", execution.ID.String(), eventErr)
	}
	finishedAt := time.Now().UTC()
	if _, finishErr := h.connectorHubExecutions.FinishAttempt(ctx, attempt.ID, repository.FinishConnectorHubExecutionAttemptParams{
		Status:         string(models.ConnectorHubExecutionStatusCompleted),
		Response:       response,
		ProviderStatus: providerStatus,
		ExternalID:     externalID,
		CorrelationID:  correlationID,
		FinishedAt:     finishedAt,
	}); finishErr != nil {
		return finishErr
	}
	completedExecution, err := h.connectorHubExecutions.MarkCompleted(ctx, execution.ID, models.ConnectorHubExecutionStatusCompleted, response, providerStatus, externalID, correlationID, finishedAt)
	if err != nil {
		return err
	}
	h.emitConnectorHubExecutionCaseTimelineEvent(ctx, completedExecution)
	if _, eventErr := h.connectorHubExecutions.AddEvent(ctx, repository.AddConnectorHubExecutionEventParams{
		ExecutionID: execution.ID,
		AttemptNo:   &attemptNoValue,
		EventType:   "completed",
		Status:      string(models.ConnectorHubExecutionStatusCompleted),
		Message:     "Execution completed successfully",
		Data: map[string]any{
			"provider_status": providerStatus,
			"external_id":     externalID,
		},
	}); eventErr != nil {
		logger.Warnf("connector hub add completed event failed: execution=%s err=%v", execution.ID.String(), eventErr)
	}
	return nil
}

func (h *Handler) failConnectorHubExecution(
	ctx context.Context,
	execution models.ConnectorHubExecution,
	attempt *models.ConnectorHubExecutionAttempt,
	message string,
	providerStatus string,
	externalID string,
	correlationID string,
) error {
	finishedAt := time.Now().UTC()
	attemptNoValue := attempt.AttemptNo
	if _, finishErr := h.connectorHubExecutions.FinishAttempt(ctx, attempt.ID, repository.FinishConnectorHubExecutionAttemptParams{
		Status:         string(models.ConnectorHubExecutionStatusFailed),
		Error:          strings.TrimSpace(message),
		ProviderStatus: strings.TrimSpace(providerStatus),
		ExternalID:     strings.TrimSpace(externalID),
		CorrelationID:  strings.TrimSpace(correlationID),
		FinishedAt:     finishedAt,
	}); finishErr != nil {
		return finishErr
	}
	if execution.AttemptCount >= execution.MaxAttempts {
		deadLetterExecution, err := h.connectorHubExecutions.MarkDeadLetter(ctx, execution.ID, message, providerStatus, externalID, correlationID, finishedAt)
		if err != nil {
			return err
		}
		h.emitConnectorHubExecutionCaseTimelineEvent(ctx, deadLetterExecution)
		if _, err := h.connectorHubExecutions.AddEvent(ctx, repository.AddConnectorHubExecutionEventParams{
			ExecutionID: execution.ID,
			AttemptNo:   &attemptNoValue,
			EventType:   "dead_letter",
			Status:      string(models.ConnectorHubExecutionStatusDeadLetter),
			Message:     strings.TrimSpace(message),
		}); err != nil {
			logger.Warnf("connector hub add dead-letter event failed: execution=%s err=%v", execution.ID.String(), err)
		}
		return nil
	}
	nextAttemptAt := time.Now().UTC().Add(h.connectorHubRetryBackoff(execution.AttemptCount))
	if _, err := h.connectorHubExecutions.ScheduleRetry(ctx, execution.ID, nextAttemptAt, message, providerStatus, externalID, correlationID); err != nil {
		return err
	}
	if _, err := h.connectorHubExecutions.AddEvent(ctx, repository.AddConnectorHubExecutionEventParams{
		ExecutionID: execution.ID,
		AttemptNo:   &attemptNoValue,
		EventType:   "retry_scheduled",
		Status:      string(models.ConnectorHubExecutionStatusRetryScheduled),
		Message:     strings.TrimSpace(message),
		Data: map[string]any{
			"next_attempt_at": nextAttemptAt.Format(time.RFC3339),
		},
	}); err != nil {
		logger.Warnf("connector hub add retry event failed: execution=%s err=%v", execution.ID.String(), err)
	}
	return nil
}
