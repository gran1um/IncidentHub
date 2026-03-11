package api

import (
	"context"
	"fmt"
	"strings"

	"incidenthub/backend/internal/logger"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"

	"github.com/google/uuid"
)

func (h *Handler) emitConnectorHubExecutionCaseTimelineEvent(ctx context.Context, execution *models.ConnectorHubExecution) {
	if h == nil || h.caseEvents == nil || execution == nil || execution.CaseID == nil {
		return
	}
	var actorID *uuid.UUID
	if execution.ActorID != nil && *execution.ActorID != uuid.Nil {
		actorID = execution.ActorID
	}
	actionLabel := firstNonEmptyString(strings.TrimSpace(execution.MethodName), strings.TrimSpace(execution.Action), strings.TrimSpace(execution.MethodKey), "connector action")
	connectorLabel := firstNonEmptyString(strings.TrimSpace(execution.ConnectorName), execution.ConnectorID.String())
	statusLabel := strings.ReplaceAll(strings.TrimSpace(string(execution.Status)), "_", " ")
	title := "Connector execution completed"
	switch execution.Status {
	case models.ConnectorHubExecutionStatusDryRun:
		title = "Connector dry-run completed"
	case models.ConnectorHubExecutionStatusDeadLetter, models.ConnectorHubExecutionStatusFailed:
		title = "Connector execution failed"
	case models.ConnectorHubExecutionStatusCancelled:
		title = "Connector execution canceled"
	default:
		// keep default title
	}
	body := fmt.Sprintf("%s via %s finished with status %s.", actionLabel, connectorLabel, statusLabel)
	preview := firstNonEmptyString(
		strings.TrimSpace(stringFromMap(execution.Response, "reply")),
		strings.TrimSpace(execution.Error),
		strings.TrimSpace(stringFromMap(execution.Request, "message")),
	)
	if preview != "" {
		body = body + " " + truncateAgentText(preview, 900)
	}
	metadata := map[string]any{
		"execution_id":      execution.ID.String(),
		"connector_id":      execution.ConnectorID.String(),
		"connector_name":    execution.ConnectorName,
		"connector_channel": execution.ConnectorChannel,
		"action":            execution.Action,
		"method_name":       execution.MethodName,
		"method_key":        execution.MethodKey,
		"status":            execution.Status,
		"execution_mode":    execution.ExecutionMode,
		"provider_status":   execution.ProviderStatus,
		"external_id":       execution.ExternalID,
		"correlation_id":    execution.CorrelationID,
		"attempt_count":     execution.AttemptCount,
		"max_attempts":      execution.MaxAttempts,
	}
	if actor := syntheticActorFromConnectorExecution(*execution); len(actor) > 0 {
		metadata["actor"] = actor
		actorID = nil
	}
	if strings.TrimSpace(execution.Error) != "" {
		metadata["error"] = execution.Error
	}
	if _, err := h.caseEvents.Create(ctx, repository.CreateCaseEventParams{
		TenantID:  execution.TenantID,
		CaseID:    *execution.CaseID,
		EventType: "connector_execution",
		Title:     title,
		Body:      body,
		ActorID:   actorID,
		Metadata:  metadata,
	}); err != nil {
		logger.Warnf("connector hub case timeline event failed: execution=%s err=%v", execution.ID.String(), err)
	}
}
