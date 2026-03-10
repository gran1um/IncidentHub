package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"incidenthub/backend/internal/connectorhub"
	"incidenthub/backend/internal/connectors/outbound"
	"incidenthub/backend/internal/logger"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"
	"incidenthub/backend/internal/workflow"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (h *Handler) executeConnectorHubDispatch(ctx context.Context, input connectorHubDispatchInput) (map[string]any, error) {
	prepared, err := h.prepareConnectorHubDispatch(ctx, input)
	if err != nil {
		return map[string]any{
			"connector_id": input.ConnectorID.String(),
			"status":       "failed",
			"error":        err.Error(),
		}, err
	}
	if h.connectorHubExecutions == nil {
		return h.executeConnectorHubDispatchLegacy(ctx, prepared)
	}
	queued, err := h.enqueuePreparedConnectorHubExecution(ctx, prepared, "Execution accepted", "Execution queued for dispatch", map[string]any{
		"connector_id": prepared.Connector.ID.String(),
		"action":       prepared.ActionKey,
	})
	if err != nil {
		result := h.buildConnectorHubExecutionPayload(nil, prepared)
		result["status"] = "failed"
		result["error"] = err.Error()
		return result, err
	}
	if strings.TrimSpace(strings.ToLower(input.ExecutionMode)) != "ai_agent" {
		return h.buildConnectorHubExecutionPayload(queued, prepared), nil
	}
	execution, runErr := h.processConnectorHubExecutionInline(ctx, queued.ID)
	if execution == nil {
		result := h.buildConnectorHubExecutionPayload(queued, prepared)
		result["status"] = string(models.ConnectorHubExecutionStatusFailed)
		if runErr != nil {
			result["error"] = runErr.Error()
		}
		return result, runErr
	}
	result := h.buildConnectorHubExecutionPayload(execution, prepared)
	if runErr != nil {
		result["error"] = firstNonEmptyString(strings.TrimSpace(execution.Error), runErr.Error())
		return result, runErr
	}
	return result, nil
}

func (h *Handler) enqueuePreparedConnectorHubExecution(
	ctx context.Context,
	prepared preparedConnectorHubDispatch,
	acceptedMessage string,
	queuedMessage string,
	acceptedData map[string]any,
) (*models.ConnectorHubExecution, error) {
	return h.enqueueConnectorHubExecutionRecord(ctx, repository.CreateConnectorHubExecutionParams{
		TenantID:         prepared.Input.TenantID,
		ActorID:          prepared.Input.ActorID,
		ConnectorID:      prepared.Connector.ID,
		ConnectorName:    prepared.ConnectorName,
		ConnectorChannel: prepared.ConnectorChannel,
		MethodID:         prepared.MethodID,
		MethodName:       prepared.MethodName,
		MethodKey:        firstNonEmptyString(prepared.MethodKey, prepared.ActionKey),
		Action:           prepared.ActionKey,
		CaseID:           prepared.Input.CaseID,
		AlertID:          prepared.Input.AlertID,
		ExecutionMode:    firstNonEmptyString(prepared.Input.ExecutionMode, "manual"),
		DryRun:           prepared.Input.DryRun,
		Request:          prepared.Request,
		Metadata:         prepared.Metadata,
		MaxAttempts:      h.connectorHubMaxAttempts(),
	}, acceptedMessage, queuedMessage, acceptedData, false)
}

func (h *Handler) buildConnectorHubIdempotencyKey(params repository.CreateConnectorHubExecutionParams) string {
	meta := normalizeMap(params.Metadata)
	// Allow explicit override via metadata
	if key := strings.TrimSpace(stringFromMap(meta, "idempotency_key", "idempotencyKey")); key != "" {
		return key
	}
	parts := []string{
		"connector:" + params.ConnectorID.String(),
		"action:" + strings.ToLower(strings.TrimSpace(params.Action)),
		"mode:" + strings.ToLower(strings.TrimSpace(params.ExecutionMode)),
	}
	if params.MethodID != nil {
		parts = append(parts, "method:"+params.MethodID.String())
	}
	if params.CaseID != nil {
		parts = append(parts, "case:"+params.CaseID.String())
	}
	if params.AlertID != nil {
		parts = append(parts, "alert:"+params.AlertID.String())
	}
	target := firstNonEmptyString(
		strings.TrimSpace(stringFromMap(meta, "idempotency_target")),
		strings.TrimSpace(stringFromMap(meta, "target")),
		strings.TrimSpace(stringFromMap(meta, "indicator")),
		strings.TrimSpace(stringFromMap(meta, "observable_id", "observableId")),
	)
	if target != "" {
		parts = append(parts, "target:"+strings.ToLower(target))
	}
	return strings.Join(parts, "|")
}

func (h *Handler) enqueueConnectorHubExecutionRecord(
	ctx context.Context,
	params repository.CreateConnectorHubExecutionParams,
	acceptedMessage string,
	queuedMessage string,
	acceptedData map[string]any,
	disableIdempotency bool,
) (*models.ConnectorHubExecution, error) {
	if h.connectorHubExecutions == nil {
		return nil, fmt.Errorf("connector hub execution repository is not configured")
	}
	// Compute idempotency key and reuse existing execution if possible (unless disabled).
	idempotencyKey := ""
	if !disableIdempotency {
		idempotencyKey = h.buildConnectorHubIdempotencyKey(params)
		params.IdempotencyKey = idempotencyKey
		if strings.TrimSpace(idempotencyKey) != "" {
			if existing, err := h.connectorHubExecutions.GetByTenantAndIdempotencyKey(ctx, params.TenantID, idempotencyKey); err == nil && existing != nil {
				if _, eventErr := h.connectorHubExecutions.AddEvent(ctx, repository.AddConnectorHubExecutionEventParams{
					ExecutionID: existing.ID,
					EventType:   "idempotent_reuse",
					Status:      string(existing.Status),
					Message:     "Connector hub execution reused by idempotency key",
					Data: map[string]any{
						"idempotency_key": idempotencyKey,
					},
				}); eventErr != nil {
					logger.Warnf("connector hub add idempotent_reuse event failed: execution=%s err=%v", existing.ID.String(), eventErr)
				}
				return existing, nil
			}
		}
	} else {
		params.IdempotencyKey = strings.TrimSpace(params.IdempotencyKey)
	}
	execution, err := h.connectorHubExecutions.Create(ctx, params)
	if err != nil {
		logger.Errorf("connector hub execution create failed: %v", err)
		return nil, err
	}
	if _, eventErr := h.connectorHubExecutions.AddEvent(ctx, repository.AddConnectorHubExecutionEventParams{
		ExecutionID: execution.ID,
		EventType:   "accepted",
		Status:      string(models.ConnectorHubExecutionStatusAccepted),
		Message:     firstNonEmptyString(acceptedMessage, "Execution accepted"),
		Data:        normalizeMap(acceptedData),
	}); eventErr != nil {
		logger.Warnf("connector hub add accepted event failed: execution=%s err=%v", execution.ID.String(), eventErr)
	}
	queued, err := h.connectorHubExecutions.MarkQueued(ctx, execution.ID)
	if err != nil {
		return nil, err
	}
	queuedEventData := map[string]any{
		"attempt_count": queued.AttemptCount,
		"max_attempts":  queued.MaxAttempts,
	}
	for key, value := range normalizeMap(acceptedData) {
		queuedEventData[key] = value
	}
	if strings.TrimSpace(idempotencyKey) != "" {
		queuedEventData["idempotency_key"] = idempotencyKey
	}
	if _, eventErr := h.connectorHubExecutions.AddEvent(ctx, repository.AddConnectorHubExecutionEventParams{
		ExecutionID: queued.ID,
		EventType:   "queued",
		Status:      string(models.ConnectorHubExecutionStatusQueued),
		Message:     firstNonEmptyString(queuedMessage, "Execution queued for dispatch"),
		Data:        queuedEventData,
	}); eventErr != nil {
		logger.Warnf("connector hub add queued event failed: execution=%s err=%v", queued.ID.String(), eventErr)
	}
	return queued, nil
}

func (h *Handler) executeConnectorHubDispatchLegacy(ctx context.Context, prepared preparedConnectorHubDispatch) (map[string]any, error) {
	result := h.buildConnectorHubExecutionPayload(nil, prepared)
	if prepared.Input.DryRun {
		result["status"] = string(models.ConnectorHubExecutionStatusDryRun)
		result["response"] = map[string]any{
			"reply":           prepared.Message,
			"conversation_id": prepared.ConversationID,
			"metadata":        prepared.Metadata,
		}
		return result, nil
	}
	responsePayload, sendErr := h.dispatchPreparedConnectorHub(ctx, prepared)
	if sendErr != nil {
		result["status"] = string(models.ConnectorHubExecutionStatusFailed)
		result["error"] = sendErr.Error()
		return result, sendErr
	}
	result["status"] = string(models.ConnectorHubExecutionStatusCompleted)
	result["response"] = responsePayload
	return result, nil
}

func (h *Handler) processConnectorHubExecutionInline(ctx context.Context, executionID uuid.UUID) (*models.ConnectorHubExecution, error) {
	if h.connectorHubExecutions == nil {
		return nil, fmt.Errorf("connector hub execution repository is not configured")
	}
	for {
		execution, err := h.connectorHubExecutions.GetByID(ctx, executionID)
		if err != nil {
			return nil, err
		}
		if connectorhub.IsExecutionTerminalStatus(execution.Status) {
			return execution, connectorhub.ExecutionTerminalError(execution)
		}
		claimed, err := h.connectorHubExecutions.ClaimReadyByID(ctx, executionID)
		if err == nil {
			runCtx, cancel := context.WithTimeout(ctx, h.connectorHubRunTimeout())
			processErr := h.processConnectorHubExecution(runCtx, *claimed)
			cancel()
			if processErr != nil {
				updated, loadErr := h.connectorHubExecutions.GetByID(ctx, executionID)
				if loadErr == nil {
					return updated, processErr
				}
				return claimed, processErr
			}
			continue
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return execution, err
		}
		waitFor := 150 * time.Millisecond
		if execution.NextAttemptAt != nil {
			untilNext := time.Until(execution.NextAttemptAt.UTC())
			if untilNext > 0 {
				if untilNext < time.Second {
					waitFor = untilNext
				} else {
					waitFor = time.Second
				}
			}
		}
		select {
		case <-ctx.Done():
			return execution, ctx.Err()
		case <-time.After(waitFor):
		}
	}
}

func (h *Handler) buildConnectorHubExecutionPayload(execution *models.ConnectorHubExecution, prepared preparedConnectorHubDispatch) map[string]any {
	result := map[string]any{
		"status":            string(models.ConnectorHubExecutionStatusAccepted),
		"connector_id":      prepared.Input.ConnectorID.String(),
		"connector_name":    prepared.ConnectorName,
		"connector_channel": prepared.ConnectorChannel,
		"action":            prepared.ActionKey,
		"request":           prepared.Request,
		"metadata":          prepared.Metadata,
		"method_name":       prepared.MethodName,
		"method_key":        firstNonEmptyString(prepared.MethodKey, prepared.ActionKey),
	}
	if prepared.MethodID != nil {
		result["method_id"] = prepared.MethodID.String()
	}
	if prepared.Input.CaseID != nil {
		result["case_id"] = prepared.Input.CaseID.String()
	}
	if prepared.Input.AlertID != nil {
		result["alert_id"] = prepared.Input.AlertID.String()
	}
	if execution == nil {
		return result
	}
	result["id"] = execution.ID.String()
	result["status"] = string(execution.Status)
	result["connector_id"] = execution.ConnectorID.String()
	result["connector_name"] = execution.ConnectorName
	result["connector_channel"] = execution.ConnectorChannel
	result["action"] = execution.Action
	result["request"] = execution.Request
	result["response"] = execution.Response
	result["metadata"] = execution.Metadata
	if actor := syntheticActorFromConnectorExecution(*execution); len(actor) > 0 {
		result["actor"] = actor
	}
	result["method_name"] = execution.MethodName
	result["method_key"] = execution.MethodKey
	result["provider_status"] = execution.ProviderStatus
	result["external_id"] = execution.ExternalID
	result["correlation_id"] = execution.CorrelationID
	result["attempt_count"] = execution.AttemptCount
	result["max_attempts"] = execution.MaxAttempts
	result["execution_mode"] = execution.ExecutionMode
	result["dry_run"] = execution.DryRun
	result["created_at"] = execution.CreatedAt.Format(time.RFC3339)
	result["updated_at"] = execution.UpdatedAt.Format(time.RFC3339)
	if strings.TrimSpace(execution.Error) != "" {
		result["error"] = execution.Error
	}
	if execution.MethodID != nil {
		result["method_id"] = execution.MethodID.String()
	}
	if execution.CaseID != nil {
		result["case_id"] = execution.CaseID.String()
	}
	if execution.AlertID != nil {
		result["alert_id"] = execution.AlertID.String()
	}
	if execution.NextAttemptAt != nil {
		result["next_attempt_at"] = execution.NextAttemptAt.UTC().Format(time.RFC3339)
	}
	if execution.FinishedAt != nil {
		result["finished_at"] = execution.FinishedAt.UTC().Format(time.RFC3339)
	}
	return result
}

func (h *Handler) prepareConnectorHubDispatch(ctx context.Context, input connectorHubDispatchInput) (preparedConnectorHubDispatch, error) {
	prepared := preparedConnectorHubDispatch{Input: input}
	connector, err := h.resolveOutboundConnectorForTenant(ctx, input.TenantID, input.ConnectorID)
	if err != nil {
		return prepared, err
	}
	prepared.Connector = connector
	prepared.ConnectorName = stringFromMap(connector.Data, "name")
	prepared.ConnectorChannel = strings.ToLower(strings.TrimSpace(stringFromMap(connector.Data, "channel", "type", "provider")))

	method := input.Method
	if method == nil && strings.TrimSpace(input.Action) != "" {
		resolvedMethod, resolveErr := h.findConnectorMethodByAction(ctx, input.TenantID, connector.ID, connectorhub.NormalizeActionKey(input.Action))
		if resolveErr != nil {
			return prepared, resolveErr
		}
		method = resolvedMethod
	}
	if method != nil {
		prepared.MethodID = &method.ID
		prepared.MethodName = strings.TrimSpace(stringFromMap(method.Data, "name", "title"))
		prepared.MethodKey = connectorhub.NormalizeActionKey(stringFromMap(method.Data, "action", "action_key", "actionKey", "slug", "key", "name"))
	}
	prepared.ActionKey = connectorhub.NormalizeActionKey(input.Action)
	if prepared.ActionKey == "" {
		prepared.ActionKey = prepared.MethodKey
	}
	if prepared.ActionKey == "" {
		prepared.ActionKey = "connector_action"
	}

	scope := connectorhub.BuildScope(connectorhub.ScopeInput{
		Input:   input.Input,
		CaseID:  optionalUUIDString(input.CaseID),
		AlertID: optionalUUIDString(input.AlertID),
	}, prepared.ActionKey, prepared.MethodName)
	prepared.Message = strings.TrimSpace(input.Message)
	if prepared.Message == "" && method != nil {
		messageTemplate := firstNonEmptyString(
			stringFromMap(method.Data, "message_template", "messageTemplate", "prompt_template", "promptTemplate"),
			stringFromMap(method.Data, "bodyTemplate", "body_template"),
		)
		if messageTemplate != "" {
			prepared.Message = strings.TrimSpace(workflow.RenderTemplate(messageTemplate, scope))
		}
	}
	if prepared.Message == "" {
		payload, _ := json.Marshal(scope["input"])
		prepared.Message = fmt.Sprintf("connector_hub action=%s payload=%s", prepared.ActionKey, string(payload))
	}

	prepared.Metadata = normalizeMap(input.Metadata)
	if prepared.Metadata == nil {
		prepared.Metadata = map[string]any{}
	}
	prepared.Metadata["source"] = firstNonEmptyString(input.ExecutionMode, "manual")
	prepared.Metadata["action"] = prepared.ActionKey
	prepared.Metadata["input"] = normalizeMap(input.Input)
	if prepared.MethodID != nil {
		prepared.Metadata["method_id"] = prepared.MethodID.String()
	}
	if prepared.MethodName != "" {
		prepared.Metadata["method_name"] = prepared.MethodName
	}
	if input.CaseID != nil {
		prepared.Metadata["case_id"] = input.CaseID.String()
	}
	if input.AlertID != nil {
		prepared.Metadata["alert_id"] = input.AlertID.String()
	}
	if method != nil {
		methodMetadataTemplate := normalizeMap(firstMapValue(method.Data, "metadata_template", "metadataTemplate"))
		prepared.Metadata = connectorhub.MergeMetadata(prepared.Metadata, connectorhub.RenderTemplateValue(methodMetadataTemplate, scope))
	}
	if strings.TrimSpace(strings.ToLower(input.ExecutionMode)) == "ai_agent" {
		if actor := aiSyntheticActorFromMetadata(prepared.Metadata); len(actor) > 0 {
			prepared.Metadata["actor"] = actor
		}
	}

	prepared.Author = firstNonEmptyString(strings.TrimSpace(input.Author), "connector-hub")
	prepared.ThreadID = input.ConnectorID.String()
	if input.CaseID != nil {
		prepared.ThreadID = input.CaseID.String()
	} else if input.AlertID != nil {
		prepared.ThreadID = input.AlertID.String()
	}
	prepared.ConversationID = strings.TrimSpace(input.Conversation)
	if prepared.ConversationID == "" {
		prepared.ConversationID = "connector-hub:" + prepared.ActionKey
		if input.CaseID != nil {
			prepared.ConversationID += ":case:" + input.CaseID.String()
		} else if input.AlertID != nil {
			prepared.ConversationID += ":alert:" + input.AlertID.String()
		}
	}
	prepared.Request = map[string]any{
		"thread_id":       prepared.ThreadID,
		"conversation_id": prepared.ConversationID,
		"author":          prepared.Author,
		"message":         prepared.Message,
		"metadata":        prepared.Metadata,
	}
	return prepared, nil
}

func (h *Handler) dispatchPreparedConnectorHub(ctx context.Context, prepared preparedConnectorHubDispatch) (map[string]any, error) {
	if h.outbound == nil {
		return map[string]any{}, fmt.Errorf("outbound connector service is not configured")
	}
	reply, err := h.outbound.Send(ctx, *prepared.Connector, outbound.SendRequest{
		ThreadID:       prepared.ThreadID,
		ConversationID: prepared.ConversationID,
		Author:         prepared.Author,
		Message:        prepared.Message,
		Metadata:       prepared.Metadata,
	})
	if err != nil {
		return map[string]any{}, err
	}
	return connectorhub.BuildSendResponsePayload(reply), nil
}
