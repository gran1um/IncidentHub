package api

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"incidenthub/backend/internal/logger"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	aiAgentQueueSourceAPI             = "api"
	aiAgentQueueSourceAsyncKafka      = "async_kafka"
	aiAgentQueueSourceCaseEscalation  = "case_escalation"
	aiAgentQueueSourceCaseFromAlerts  = "case_from_alerts"
	aiAgentQueueSourceCaseCopy        = "case_copy"
	aiAgentQueueSourceAlertAutoCase   = "ai_agent_alert_case"
	aiAgentQueueWorkerWorkflowPrefix  = "ai-queue-"
	aiAgentWorkloadWorkflowPrefix     = "ai-workload-"
	aiAgentQueueCatalogRunTriggerType = "queue"
)

// Sentinel errors for AI agent queue execution (err113).
var (
	errAIAgentCaseContextUnavailable = errors.New("case context is not available for ai agent run")
)

type aiAgentQueueEventProcessSummary struct {
	MatchedAgents   int
	ProcessedAgents int
	PendingAgents   int
	FailedAgents    int
	CancelledAgents int
	LastError       string
}

func (h *Handler) StartBackgroundWorkers(ctx context.Context) {
	if h == nil {
		return
	}
	h.backgroundWorkers.Do(func() {
		h.startAIAgentQueueWorker(ctx)
		h.startConnectorHubWorker(ctx)
	})
}

func (h *Handler) startAIAgentQueueWorker(ctx context.Context) {
	if h.aiAgentQueue == nil {
		return
	}
	if !h.cfg.AI.QueueEnabled {
		return
	}
	go h.runAIAgentQueueLoop(ctx)
}

func (h *Handler) runAIAgentQueueLoop(ctx context.Context) {
	h.processNextAIAgentQueueBatch(ctx, h.aiAgentQueueBatchSize())
	ticker := time.NewTicker(h.aiAgentQueuePollInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.processNextAIAgentQueueBatch(ctx, h.aiAgentQueueBatchSize())
		}
	}
}

func (h *Handler) processNextAIAgentQueueBatch(ctx context.Context, limit int) int {
	if h == nil || h.aiAgentQueue == nil {
		return 0
	}
	h.recoverStaleAIAgentQueueProcessing(ctx)
	events, err := h.aiAgentQueue.ListByStatus(ctx, models.AIAgentQueueStatusQueued, limit)
	if err != nil {
		logger.Warnf("ai agent queue list failed: %v", err)
		return 0
	}
	processedEvents := 0
	for _, event := range events {
		workflowID := aiAgentQueueWorkerWorkflowPrefix + uuid.NewString()
		if err := h.aiAgentQueue.MarkProcessing(ctx, event.ID, workflowID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			logger.Warnf("ai agent queue mark processing failed: event=%s err=%v", event.ID.String(), err)
			continue
		}
		event.WorkflowID = workflowID
		if reloaded, loadErr := h.aiAgentQueue.GetByID(ctx, event.ID); loadErr == nil && reloaded != nil {
			event = *reloaded
		} else {
			event.AttemptCount++
		}
		if event.MaxAttempts <= 0 {
			event.MaxAttempts = h.aiAgentQueueMaxAttempts()
		}

		eventCtx, cancel := context.WithTimeout(ctx, h.aiAgentQueueRunTimeout())
		summary, processErr := h.processAIAgentQueueEvent(eventCtx, event)
		cancel()

		if processErr != nil {
			failurePayload := repository.CompleteAIAgentQueueEventParams{
				MatchedAgents:   summary.MatchedAgents,
				ProcessedAgents: summary.ProcessedAgents,
				LastError:       processErr.Error(),
			}
			if event.AttemptCount >= event.MaxAttempts {
				if markErr := h.aiAgentQueue.MarkFailed(ctx, event.ID, failurePayload); markErr != nil {
					logger.Warnf("ai agent queue mark failed error: event=%s err=%v", event.ID.String(), markErr)
				}
				continue
			}
			if markErr := h.aiAgentQueue.RequeueAfterFailure(ctx, event.ID, failurePayload); markErr != nil {
				logger.Warnf("ai agent queue requeue after failure error: event=%s err=%v", event.ID.String(), markErr)
			}
			continue
		}

		workloads, loadErr := h.aiAgentWorkloads.ListByQueueEvent(ctx, event.ID)
		if loadErr != nil {
			failurePayload := repository.CompleteAIAgentQueueEventParams{
				MatchedAgents:   summary.MatchedAgents,
				ProcessedAgents: summary.ProcessedAgents,
				LastError:       joinAIAgentQueueErrors(summary.LastError, loadErr.Error()),
			}
			if event.AttemptCount >= event.MaxAttempts {
				if markErr := h.aiAgentQueue.MarkFailed(ctx, event.ID, failurePayload); markErr != nil {
					logger.Warnf("ai agent queue mark failed after workload reload error: event=%s err=%v", event.ID.String(), markErr)
				}
			} else if markErr := h.aiAgentQueue.RequeueAfterFailure(ctx, event.ID, failurePayload); markErr != nil {
				logger.Warnf("ai agent queue requeue after workload reload error: event=%s err=%v", event.ID.String(), markErr)
			}
			continue
		}

		status, resolvedSummary, canUpdate := resolveAIAgentQueueEventStatusFromWorkloads(workloads)
		payload := repository.CompleteAIAgentQueueEventParams{
			MatchedAgents:   resolvedSummary.MatchedAgents,
			ProcessedAgents: resolvedSummary.ProcessedAgents,
			LastError:       resolvedSummary.LastError,
		}
		if !canUpdate {
			payload.LastError = joinAIAgentQueueErrors(payload.LastError, "workloads are still processing")
			if markErr := h.aiAgentQueue.RequeueAfterFailure(ctx, event.ID, payload); markErr != nil {
				logger.Warnf("ai agent queue requeue pending workload processing error: event=%s err=%v", event.ID.String(), markErr)
			}
			continue
		}
		switch status {
		case models.AIAgentQueueStatusQueued:
			if markErr := h.aiAgentQueue.RequeueAfterFailure(ctx, event.ID, payload); markErr != nil {
				logger.Warnf("ai agent queue requeue pending workloads error: event=%s err=%v", event.ID.String(), markErr)
			}
		case models.AIAgentQueueStatusFailed:
			if markErr := h.aiAgentQueue.MarkFailed(ctx, event.ID, payload); markErr != nil {
				logger.Warnf("ai agent queue mark failed from workloads error: event=%s err=%v", event.ID.String(), markErr)
			}
		default:
			if markErr := h.aiAgentQueue.MarkDone(ctx, event.ID, payload); markErr != nil {
				logger.Warnf("ai agent queue mark done error: event=%s err=%v", event.ID.String(), markErr)
				continue
			}
			processedEvents++
		}
	}
	return processedEvents
}

func (h *Handler) processAIAgentQueueEvent(ctx context.Context, event models.AIAgentQueueEvent) (aiAgentQueueEventProcessSummary, error) {
	if h.catalog == nil {
		return aiAgentQueueEventProcessSummary{}, fmt.Errorf("catalog repository is not configured")
	}
	if h.cases == nil || h.alerts == nil {
		return aiAgentQueueEventProcessSummary{}, fmt.Errorf("cases/alerts repositories are not configured")
	}
	if h.aiAgentWorkloads == nil {
		return aiAgentQueueEventProcessSummary{}, fmt.Errorf("ai agent workload repository is not configured")
	}

	entityType := strings.ToLower(strings.TrimSpace(event.EntityType))
	var caseItem *models.Case
	var alertItem *models.Alert

	switch entityType {
	case "case":
		item, err := h.cases.GetByID(ctx, event.TenantID, event.EntityID)
		if err != nil || item == nil {
			return aiAgentQueueEventProcessSummary{}, nil
		}
		caseItem = item
	case "alert":
		item, err := h.alerts.GetByID(ctx, event.TenantID, event.EntityID)
		if err != nil || item == nil {
			return aiAgentQueueEventProcessSummary{}, nil
		}
		alertItem = item
	default:
		return aiAgentQueueEventProcessSummary{}, fmt.Errorf("unsupported ai agent queue entity_type %q", event.EntityType)
	}

	workloads, err := h.aiAgentWorkloads.ListByQueueEvent(ctx, event.ID)
	if err != nil {
		return aiAgentQueueEventProcessSummary{}, err
	}

	if len(workloads) == 0 {
		agents, loadErr := h.loadMatchingAIAgentsForQueueEvent(ctx, event, caseItem, alertItem)
		if loadErr != nil {
			return aiAgentQueueEventProcessSummary{}, loadErr
		}
		selectedAgents := selectAIAgentQueueExecutionPlan(agents)
		for index, agent := range selectedAgents {
			if _, upsertErr := h.aiAgentWorkloads.Upsert(ctx, repository.UpsertAIAgentWorkloadParams{
				TenantID:          event.TenantID,
				QueueEventID:      event.ID,
				AgentID:           agent.ID,
				AgentName:         agent.Name,
				ExecutionPolicy:   agent.ExecutionPolicy,
				ExecutionPriority: agent.ExecutionPriority,
				ExecutionIndex:    index,
				EntityType:        event.EntityType,
				EntityID:          event.EntityID,
				MaxAttempts:       event.MaxAttempts,
			}); upsertErr != nil {
				return aiAgentQueueEventProcessSummary{}, upsertErr
			}
		}
		workloads, err = h.aiAgentWorkloads.ListByQueueEvent(ctx, event.ID)
		if err != nil {
			return aiAgentQueueEventProcessSummary{}, err
		}
	}

	if len(workloads) == 0 {
		return aiAgentQueueEventProcessSummary{}, nil
	}

	agentByID, err := h.loadAIAgentDefinitionsByID(ctx, event.TenantID, workloads)
	if err != nil {
		return aiAgentQueueEventProcessSummary{}, err
	}

	errorsList := make([]string, 0)
	for _, workload := range workloads {
		switch workload.Status {
		case models.AIAgentWorkloadStatusDone, models.AIAgentWorkloadStatusCancelled:
			continue
		case models.AIAgentWorkloadStatusFailed:
			if workload.AttemptCount >= workload.MaxAttempts {
				continue
			}
		case models.AIAgentWorkloadStatusQueued, models.AIAgentWorkloadStatusProcessing:
			// handled below
		}
		if workload.Status != models.AIAgentWorkloadStatusQueued {
			continue
		}

		action, reason := resolveAIAgentWorkloadDispatchAction(workload, workloads)
		switch action {
		case aiAgentWorkloadDispatchExecute:
			// proceed
		case aiAgentWorkloadDispatchWait:
			continue
		case aiAgentWorkloadDispatchCancel:
			if _, cancelErr := h.aiAgentWorkloads.Cancel(ctx, workload.ID, "dispatch", reason); cancelErr != nil {
				errorsList = append(errorsList, cancelErr.Error())
			}
			if refreshed, loadErr := h.aiAgentWorkloads.ListByQueueEvent(ctx, event.ID); loadErr == nil {
				workloads = refreshed
			} else {
				errorsList = append(errorsList, loadErr.Error())
			}
			continue
		}

		agent, ok := agentByID[workload.AgentID]
		if !ok {
			if _, markErr := h.aiAgentWorkloads.MarkFailed(ctx, workload.ID, repository.CompleteAIAgentWorkloadParams{
				LastStage: "dispatch",
				LastError: "ai agent configuration is no longer available",
			}); markErr != nil {
				errorsList = append(errorsList, markErr.Error())
			} else {
				errorsList = append(errorsList, fmt.Sprintf("agent %s is no longer available", workload.AgentID.String()))
			}
			if refreshed, loadErr := h.aiAgentWorkloads.ListByQueueEvent(ctx, event.ID); loadErr == nil {
				workloads = refreshed
			} else {
				errorsList = append(errorsList, loadErr.Error())
			}
			continue
		}

		claimed, claimErr := h.aiAgentWorkloads.MarkProcessing(ctx, workload.ID, aiAgentWorkloadWorkflowPrefix+uuid.NewString())
		if claimErr != nil {
			if errors.Is(claimErr, pgx.ErrNoRows) {
				continue
			}
			errorsList = append(errorsList, claimErr.Error())
			continue
		}
		if execErr := h.executeAIAgentQueueWorkload(ctx, event, *claimed, agent, caseItem, alertItem); execErr != nil {
			errorsList = append(errorsList, execErr.Error())
		}
		if refreshed, loadErr := h.aiAgentWorkloads.ListByQueueEvent(ctx, event.ID); loadErr == nil {
			workloads = refreshed
		} else {
			errorsList = append(errorsList, loadErr.Error())
		}
	}

	workloads, err = h.aiAgentWorkloads.ListByQueueEvent(ctx, event.ID)
	if err != nil {
		return aiAgentQueueEventProcessSummary{}, err
	}
	summary := summarizeAIAgentQueueWorkloads(workloads)
	if len(errorsList) > 0 {
		summary.LastError = joinAIAgentQueueErrors(summary.LastError, errorsList...)
	}
	return summary, nil
}

func (h *Handler) loadMatchingAIAgentsForQueueEvent(
	ctx context.Context,
	event models.AIAgentQueueEvent,
	caseItem *models.Case,
	alertItem *models.Alert,
) ([]aiAgentDefinition, error) {
	items, err := h.catalog.List(ctx, repository.CatalogListParams{
		Kind:     aiAgentCatalogKind,
		TenantID: &event.TenantID,
		Limit:    500,
	})
	if err != nil {
		return nil, fmt.Errorf("list ai agents for queue dispatch: %w", err)
	}
	entityType := strings.ToLower(strings.TrimSpace(event.EntityType))
	caseTags := []string{}
	if caseItem != nil {
		caseTags = h.loadCaseMetaTags(ctx, event.TenantID, caseItem.ID)
	}
	matched := make([]aiAgentDefinition, 0)
	for _, item := range items {
		agent := parseAIAgentDefinition(item)
		if !agent.Enabled {
			continue
		}
		if !aiAgentSupportsTargetType(agent, entityType) {
			continue
		}
		switch entityType {
		case "case":
			if caseItem == nil {
				continue
			}
			if len(agent.CaseTags) > 0 && !containsAnyCaseTag(caseTags, agent.CaseTags) {
				continue
			}
		case "alert":
			if alertItem == nil {
				continue
			}
			if len(agent.AlertSources) > 0 {
				source := strings.ToLower(strings.TrimSpace(alertItem.Source))
				if !slices.Contains(agent.AlertSources, source) {
					continue
				}
			}
			if alertItem.CaseID == nil && !agent.AutoCreateCaseFromAlert {
				continue
			}
		}
		matched = append(matched, agent)
	}
	return matched, nil
}

func (h *Handler) loadAIAgentDefinitionsByID(ctx context.Context, tenantID uuid.UUID, workloads []models.AIAgentWorkload) (map[uuid.UUID]aiAgentDefinition, error) {
	if len(workloads) == 0 {
		return map[uuid.UUID]aiAgentDefinition{}, nil
	}
	items, err := h.catalog.List(ctx, repository.CatalogListParams{
		Kind:     aiAgentCatalogKind,
		TenantID: &tenantID,
		Limit:    500,
	})
	if err != nil {
		return nil, fmt.Errorf("list ai agents for queue workloads: %w", err)
	}
	requested := make(map[uuid.UUID]struct{}, len(workloads))
	for _, workload := range workloads {
		requested[workload.AgentID] = struct{}{}
	}
	agents := make(map[uuid.UUID]aiAgentDefinition, len(requested))
	for _, item := range items {
		if _, ok := requested[item.ID]; !ok {
			continue
		}
		agent := parseAIAgentDefinition(item)
		agents[agent.ID] = agent
	}
	return agents, nil
}

func sortAIAgentQueueAgents(items []aiAgentDefinition) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].ExecutionPriority != items[j].ExecutionPriority {
			return items[i].ExecutionPriority > items[j].ExecutionPriority
		}
		leftName := strings.ToLower(strings.TrimSpace(items[i].Name))
		rightName := strings.ToLower(strings.TrimSpace(items[j].Name))
		if leftName != rightName {
			return leftName < rightName
		}
		return items[i].ID.String() < items[j].ID.String()
	})
}

func selectAIAgentQueueExecutionPlan(agents []aiAgentDefinition) []aiAgentDefinition {
	if len(agents) == 0 {
		return nil
	}
	ordered := append([]aiAgentDefinition(nil), agents...)
	sortAIAgentQueueAgents(ordered)
	exclusive := make([]aiAgentDefinition, 0, len(ordered))
	firstMatch := make([]aiAgentDefinition, 0, len(ordered))
	fallback := make([]aiAgentDefinition, 0, len(ordered))
	for _, agent := range ordered {
		switch normalizeAIAgentExecutionPolicy(agent.ExecutionPolicy) {
		case string(models.AIAgentExecutionPolicyExclusive):
			exclusive = append(exclusive, agent)
		case string(models.AIAgentExecutionPolicyFirstMatch):
			firstMatch = append(firstMatch, agent)
		case string(models.AIAgentExecutionPolicyFallbackChain):
			fallback = append(fallback, agent)
		}
	}
	if len(exclusive) > 0 {
		return exclusive[:1]
	}
	if len(firstMatch) > 0 {
		return firstMatch[:1]
	}
	if len(fallback) > 0 {
		return fallback
	}
	return ordered
}

type aiAgentWorkloadDispatchAction string

const (
	aiAgentWorkloadDispatchExecute aiAgentWorkloadDispatchAction = "execute"
	aiAgentWorkloadDispatchWait    aiAgentWorkloadDispatchAction = "wait"
	aiAgentWorkloadDispatchCancel  aiAgentWorkloadDispatchAction = "cancel"
)

func resolveAIAgentWorkloadDispatchAction(workload models.AIAgentWorkload, workloads []models.AIAgentWorkload) (action aiAgentWorkloadDispatchAction, reason string) {
	if workload.ExecutionPolicy != models.AIAgentExecutionPolicyFallbackChain {
		return aiAgentWorkloadDispatchExecute, ""
	}
	for _, item := range workloads {
		if item.ID == workload.ID || item.ExecutionPolicy != models.AIAgentExecutionPolicyFallbackChain {
			continue
		}
		if item.ExecutionIndex >= workload.ExecutionIndex {
			continue
		}
		switch item.Status {
		case models.AIAgentWorkloadStatusDone:
			label := firstNonEmptyString(strings.TrimSpace(item.AgentName), item.AgentID.String())
			return aiAgentWorkloadDispatchCancel, fmt.Sprintf("fallback chain completed by %s", label)
		case models.AIAgentWorkloadStatusQueued, models.AIAgentWorkloadStatusProcessing:
			return aiAgentWorkloadDispatchWait, ""
		case models.AIAgentWorkloadStatusFailed, models.AIAgentWorkloadStatusCancelled:
			continue
		}
	}
	return aiAgentWorkloadDispatchExecute, ""
}

func (h *Handler) executeAIAgentQueueWorkload(
	ctx context.Context,
	event models.AIAgentQueueEvent,
	workload models.AIAgentWorkload,
	agent aiAgentDefinition,
	caseItem *models.Case,
	alertItem *models.Alert,
) error {
	actorID := h.resolveAIAgentQueueActorID(ctx, event, caseItem, alertItem)
	finalize := func(runID *uuid.UUID, result map[string]any, execErr error) error {
		lastStage := aiAgentWorkloadLastStage(result)
		if execErr == nil {
			if _, err := h.aiAgentWorkloads.MarkDone(ctx, workload.ID, repository.CompleteAIAgentWorkloadParams{
				RunID:      runID,
				LastStage:  lastStage,
				WorkflowID: workload.WorkflowID,
			}); err != nil {
				return err
			}
			return nil
		}
		payload := repository.CompleteAIAgentWorkloadParams{
			RunID:      runID,
			LastStage:  lastStage,
			LastError:  execErr.Error(),
			WorkflowID: workload.WorkflowID,
		}
		if workload.AttemptCount >= workload.MaxAttempts {
			_, err := h.aiAgentWorkloads.MarkFailed(ctx, workload.ID, payload)
			return err
		}
		_, err := h.aiAgentWorkloads.MarkRetryQueued(ctx, workload.ID, payload)
		return err
	}

	resolvedCase := caseItem
	if strings.EqualFold(event.EntityType, "alert") {
		item, err := h.resolveCaseForAIAgentAlert(ctx, event, agent, alertItem, actorID)
		if err != nil {
			result := map[string]any{
				"alert_id":     event.EntityID.String(),
				"entity_type":  "alert",
				"error":        err.Error(),
				"completed_at": time.Now().UTC().Format(time.RFC3339),
			}
			runID, persistErr := h.persistAIAgentQueueRun(ctx, event, agent, actorID, result)
			if persistErr != nil {
				return finalize(runID, result, persistErr)
			}
			if finalizeErr := finalize(runID, result, fmt.Errorf("resolve case for alert: %w", err)); finalizeErr != nil {
				return finalizeErr
			}
			return fmt.Errorf("resolve case for alert: %w", err)
		}
		if item == nil {
			if _, err := h.aiAgentWorkloads.MarkDone(ctx, workload.ID, repository.CompleteAIAgentWorkloadParams{
				LastStage:  "dispatch",
				WorkflowID: workload.WorkflowID,
			}); err != nil {
				return err
			}
			return nil
		}
		resolvedCase = item
	}
	if resolvedCase == nil {
		result := map[string]any{
			"entity_type":  event.EntityType,
			"error":        "case context is not available for ai agent run",
			"completed_at": time.Now().UTC().Format(time.RFC3339),
		}
		runID, persistErr := h.persistAIAgentQueueRun(ctx, event, agent, actorID, result)
		if persistErr != nil {
			return finalize(runID, result, persistErr)
		}
		execErr := errAIAgentCaseContextUnavailable
		if finalizeErr := finalize(runID, result, execErr); finalizeErr != nil {
			return finalizeErr
		}
		return execErr
	}

	runtimeAI, runtimeErr := h.resolveAIAgentService(agent)
	if runtimeErr != nil {
		result := map[string]any{
			"case_id":      resolvedCase.ID.String(),
			"case_number":  strings.TrimSpace(resolvedCase.CaseNumber),
			"title":        resolvedCase.Title,
			"status":       strings.ToLower(strings.TrimSpace(resolvedCase.Status)),
			"error":        runtimeErr.Error(),
			"completed_at": time.Now().UTC().Format(time.RFC3339),
		}
		runID, persistErr := h.persistAIAgentQueueRun(ctx, event, agent, actorID, result)
		if persistErr != nil {
			return finalize(runID, result, persistErr)
		}
		execErr := fmt.Errorf("resolve ai agent service: %w", runtimeErr)
		if finalizeErr := finalize(runID, result, execErr); finalizeErr != nil {
			return finalizeErr
		}
		return execErr
	}

	language := normalizeAILanguage(agent.Language)
	result := h.runAIAgentForCase(ctx, runtimeAI, event.TenantID, actorID, agent, *resolvedCase, false, language)
	runErr := strings.TrimSpace(stringFromMap(result, "error"))
	autoActionsAllowed := true
	if parsed, ok := boolFromMap(result, "auto_actions_allowed", "autoActionsAllowed"); ok {
		autoActionsAllowed = parsed
	}
	requiresHumanReview := false
	if parsed, ok := boolFromMap(result, "requires_human_review", "requiresHumanReview"); ok {
		requiresHumanReview = parsed
	}
	if runErr == "" {
		if autoActionsAllowed && !requiresHumanReview {
			if h.shouldAutoCloseCaseByAIAgent(agent, stringFromMap(result, "verdict")) {
				closed, closeErr := h.closeCaseByAIAgent(ctx, event.TenantID, *resolvedCase, agent, actorID, result)
				if closeErr != nil {
					result["error"] = closeErr.Error()
				} else {
					result["auto_closed"] = closed
				}
			}
		} else {
			result["auto_closed"] = false
			result["auto_close_skipped_reason"] = "auto actions are blocked by policy"
		}
	}
	if strings.TrimSpace(stringFromMap(result, "error")) == "" && len(agent.NotificationConnectorIDs) > 0 {
		if autoActionsAllowed && !requiresHumanReview {
			result["notification_results"] = h.sendAIAgentNotifications(ctx, event.TenantID, agent, *resolvedCase, result)
		} else {
			result["notification_results"] = []map[string]any{}
			result["notifications_skipped_reason"] = "auto actions are blocked by policy"
		}
	}
	runID, persistErr := h.persistAIAgentQueueRun(ctx, event, agent, actorID, result)
	if persistErr != nil {
		return finalize(runID, result, persistErr)
	}
	if runErr := strings.TrimSpace(stringFromMap(result, "error")); runErr != "" {
		execErr := fmt.Errorf("ai agent run failed: %s", runErr)
		if finalizeErr := finalize(runID, result, execErr); finalizeErr != nil {
			return finalizeErr
		}
		return execErr
	}
	return finalize(runID, result, nil)
}

func (h *Handler) persistAIAgentQueueRun(
	ctx context.Context,
	event models.AIAgentQueueEvent,
	agent aiAgentDefinition,
	actorID *uuid.UUID,
	result map[string]any,
) (*uuid.UUID, error) {
	if h.catalog == nil {
		return nil, fmt.Errorf("catalog repository is not configured")
	}
	if result == nil {
		result = map[string]any{}
	}
	runErr := strings.TrimSpace(stringFromMap(result, "error"))
	status := "completed"
	successfulCases := 1
	if runErr != "" {
		status = "completed_with_error"
		successfulCases = 0
	}
	startedAt := time.Now().UTC()
	finishedAt := startedAt
	runData := map[string]any{
		"trigger":               aiAgentQueueCatalogRunTriggerType,
		"queue_event_id":        event.ID.String(),
		"queue_workflow_id":     strings.TrimSpace(event.WorkflowID),
		"queue_entity_type":     strings.ToLower(strings.TrimSpace(event.EntityType)),
		"queue_entity_id":       event.EntityID.String(),
		"queue_source":          strings.TrimSpace(event.Source),
		"agent_id":              agent.ID.String(),
		"agent_name":            agent.Name,
		"agent_description":     agent.Description,
		"agent_tags":            append([]string{}, agent.CaseTags...),
		"execution_policy":      agent.ExecutionPolicy,
		"execution_priority":    agent.ExecutionPriority,
		"status":                status,
		"started_at":            startedAt.Format(time.RFC3339),
		"finished_at":           finishedAt.Format(time.RFC3339),
		"dry_run":               false,
		"matched_cases_total":   1,
		"processed_cases":       1,
		"successful_cases":      successfulCases,
		"requested_case_ids":    []string{},
		"results":               []map[string]any{result},
		"enrichment_connectors": append([]string{}, agent.EnrichmentConnectorIDs...),
	}
	ownerID, createdBy := normalizeAIAgentRunActor(actorID)
	runItem, err := h.catalog.Create(ctx, repository.CatalogCreateParams{
		TenantID:  &event.TenantID,
		Kind:      aiAgentRunsCatalogKind,
		OwnerID:   ownerID,
		RefID:     &agent.ID,
		Data:      runData,
		CreatedBy: createdBy,
	})
	if err != nil {
		return nil, fmt.Errorf("persist ai agent queue run: %w", err)
	}
	if createdBy != nil {
		_ = h.audits.Log(ctx, &event.TenantID, createdBy, "ai_agent_run", "catalog_item", &agent.ID, map[string]any{
			"agent_id":              agent.ID.String(),
			"trigger":               aiAgentQueueCatalogRunTriggerType,
			"queue_event_id":        event.ID.String(),
			"run_id":                runItem.ID.String(),
			"run_status":            status,
			"processed_cases":       1,
			"successful_cases":      successfulCases,
			"runtime_error_present": runErr != "",
		})
	}
	runID := runItem.ID
	return &runID, nil
}

func (h *Handler) resolveCaseForAIAgentAlert(
	ctx context.Context,
	event models.AIAgentQueueEvent,
	agent aiAgentDefinition,
	alertItem *models.Alert,
	actorID *uuid.UUID,
) (*models.Case, error) {
	if alertItem == nil {
		return nil, nil
	}
	if h.alerts != nil {
		if refreshed, err := h.alerts.GetByID(ctx, event.TenantID, alertItem.ID); err == nil && refreshed != nil {
			alertItem = refreshed
		}
	}
	if alertItem.CaseID != nil {
		item, err := h.cases.GetByID(ctx, event.TenantID, *alertItem.CaseID)
		if err == nil {
			return item, nil
		}
	}
	if !agent.AutoCreateCaseFromAlert {
		return nil, nil
	}
	createdBy, ok := resolveAIAgentCaseAuthor(actorID, alertItem)
	if !ok {
		return nil, fmt.Errorf("failed to resolve case author for alert %s", alertItem.ID.String())
	}
	status, err := h.resolveCaseStatus(ctx, event.TenantID, "")
	if err != nil {
		status = "new"
	}
	priority := severityToCasePriority(alertItem.Severity)
	description := strings.TrimSpace(alertItem.Description)
	if description == "" {
		description = "Auto-generated case from alert " + strings.TrimSpace(alertItem.Title)
	}
	title := strings.TrimSpace(alertItem.Title)
	if title == "" {
		title = "Auto case from alert"
	}
	incidentType := strings.TrimSpace(alertItem.Source)
	if incidentType == "" {
		incidentType = "alert"
	}
	createdCase, updatedAlerts, err := h.cases.CreateWithLinkedAlerts(ctx, repository.CreateCaseParams{
		TenantID:          event.TenantID,
		CaseNumber:        generateCaseNumber(),
		Title:             title,
		Description:       description,
		Source:            firstNonEmptyString(alertItem.Source, "connector"),
		IncidentType:      incidentType,
		Status:            status,
		Priority:          priority,
		Impact:            "system",
		Confidence:        50,
		Severity:          firstNonEmptyString(alertItem.Severity, "medium"),
		TLP:               firstNonEmptyString(alertItem.TLP, "amber"),
		PAP:               firstNonEmptyString(alertItem.PAP, "amber"),
		ResolutionSummary: "",
		CreatedBy:         createdBy,
		AssignedTo:        alertItem.AssignedTo,
	}, []uuid.UUID{alertItem.ID})
	if err != nil {
		return nil, fmt.Errorf("create case from alert for ai agent: %w", err)
	}
	if h.search != nil {
		_ = h.search.IndexDocument(ctx, "cases", createdCase.ID.String(), createdCase)
		for idx := range updatedAlerts {
			_ = h.search.IndexDocument(ctx, "alerts", updatedAlerts[idx].ID.String(), updatedAlerts[idx])
		}
	}
	if h.caseEvents != nil {
		_, _ = h.caseEvents.Create(ctx, repository.CreateCaseEventParams{
			TenantID:  event.TenantID,
			CaseID:    createdCase.ID,
			EventType: "alert_import",
			Title:     "Alert imported",
			Body:      "Auto-created by AI agent from alert",
			ActorID:   nil,
			Metadata: map[string]any{
				"alert_id": event.EntityID.String(),
				"source":   aiAgentQueueSourceAlertAutoCase,
				"actor":    aiAgentSyntheticActor(agent),
			},
		})
	}
	return createdCase, nil
}

func (h *Handler) shouldAutoCloseCaseByAIAgent(agent aiAgentDefinition, verdict string) bool {
	if !agent.AutoCloseCase {
		return false
	}
	normalizedVerdict := strings.ToLower(strings.TrimSpace(verdict))
	if normalizedVerdict == "" {
		return false
	}
	allowedVerdicts := normalizeTags(agent.AutoCloseVerdicts)
	if len(allowedVerdicts) == 0 {
		allowedVerdicts = []string{"benign", "resolved", "closed", "false_positive"}
	}
	return slices.Contains(allowedVerdicts, normalizedVerdict)
}

func (h *Handler) closeCaseByAIAgent(
	ctx context.Context,
	tenantID uuid.UUID,
	caseItem models.Case,
	agent aiAgentDefinition,
	_ *uuid.UUID,
	result map[string]any,
) (bool, error) {
	if h.isCaseStatusClosed(ctx, tenantID, caseItem.Status) {
		return false, nil
	}
	closedStatus := h.resolveAIAgentClosedStatus(ctx, tenantID)
	now := time.Now().UTC().Format(time.RFC3339)
	summarySuffix := fmt.Sprintf("Auto-closed by AI agent %s with verdict: %s", agent.Name, strings.TrimSpace(stringFromMap(result, "verdict")))
	resolutionSummary := strings.TrimSpace(caseItem.ResolutionSummary)
	if resolutionSummary == "" {
		resolutionSummary = summarySuffix
	} else {
		resolutionSummary = resolutionSummary + "\n" + summarySuffix
	}
	updatedCase, err := h.cases.Update(ctx, tenantID, caseItem.ID, repository.UpdateCaseParams{
		Status:            &closedStatus,
		ClosedAt:          &now,
		ResolutionSummary: &resolutionSummary,
	})
	if err != nil {
		return false, fmt.Errorf("auto-close case by ai agent: %w", err)
	}
	if updatedCase != nil {
		caseItem = *updatedCase
	}
	if h.search != nil {
		_ = h.search.IndexDocument(ctx, "cases", caseItem.ID.String(), caseItem)
	}
	if h.caseEvents != nil {
		_, _ = h.caseEvents.Create(ctx, repository.CreateCaseEventParams{
			TenantID:  tenantID,
			CaseID:    caseItem.ID,
			EventType: "ai_agent_case_closed",
			Title:     "Case auto-closed by AI agent",
			Body:      summarySuffix,
			ActorID:   nil,
			Metadata: map[string]any{
				"agent_name": agent.Name,
				"verdict":    strings.TrimSpace(stringFromMap(result, "verdict")),
				"actor":      aiAgentSyntheticActor(agent),
			},
		})
	}
	return true, nil
}

func (h *Handler) sendAIAgentNotifications(
	ctx context.Context,
	tenantID uuid.UUID,
	agent aiAgentDefinition,
	caseItem models.Case,
	result map[string]any,
) []map[string]any {
	if len(agent.NotificationConnectorIDs) == 0 {
		return nil
	}
	message := strings.TrimSpace(strings.Join([]string{
		"AI agent " + agent.Name + " finished case review.",
		"Case: " + firstNonEmptyString(caseItem.CaseNumber, caseItem.ID.String()) + " - " + strings.TrimSpace(caseItem.Title),
		"Verdict: " + firstNonEmptyString(stringFromMap(result, "verdict"), "n/a"),
		"Summary: " + firstNonEmptyString(truncateAgentText(stringFromMap(result, "summary"), 500), "n/a"),
	}, "\n"))
	results := make([]map[string]any, 0, len(agent.NotificationConnectorIDs))
	for _, rawID := range agent.NotificationConnectorIDs {
		connectorID, parseErr := uuid.Parse(strings.TrimSpace(rawID))
		if parseErr != nil {
			results = append(results, map[string]any{"connector_id": rawID, "status": "failed", "error": "invalid connector id"})
			continue
		}
		actor := aiAgentSyntheticActor(agent)
		dispatch, sendErr := h.executeConnectorHubDispatch(ctx, connectorHubDispatchInput{
			TenantID:    tenantID,
			ActorID:     nil,
			ConnectorID: connectorID,
			Action:      "ai_notification",
			CaseID:      &caseItem.ID,
			Message:     message,
			Input: map[string]any{
				"case_id":     caseItem.ID.String(),
				"case_number": strings.TrimSpace(caseItem.CaseNumber),
				"verdict":     strings.TrimSpace(stringFromMap(result, "verdict")),
				"summary":     truncateAgentText(stringFromMap(result, "summary"), 500),
				"agent_id":    agent.ID.String(),
				"agent_name":  agent.Name,
			},
			Metadata: map[string]any{
				"agent_id":   agent.ID.String(),
				"agent_name": agent.Name,
				"case_id":    caseItem.ID.String(),
				"actor":      actor,
			},
			Author:        aiAgentDisplayName(agent.Name),
			Conversation:  "ai-agent-notify-" + agent.ID.String(),
			DryRun:        false,
			ExecutionMode: "ai_agent",
		})
		baseResult := map[string]any{
			"connector_id": connectorID.String(),
			"name":         stringFromMap(dispatch, "connector_name", "name"),
			"status":       strings.TrimSpace(stringFromMap(dispatch, "status")),
			"execution_id": strings.TrimSpace(stringFromMap(dispatch, "id")),
			"metadata":     normalizeMap(firstMapValue(dispatch, "metadata")),
		}
		if sendErr != nil {
			baseResult["error"] = sendErr.Error()
			results = append(results, baseResult)
			continue
		}
		reply := stringFromMap(normalizeMap(firstMapValue(dispatch, "response")), "reply")
		if strings.TrimSpace(reply) == "" {
			reply = stringFromMap(dispatch, "reply")
		}
		if strings.TrimSpace(reply) != "" {
			baseResult["reply"] = truncateAgentText(reply, 1200)
		}
		results = append(results, baseResult)
	}
	return results
}

func aiAgentQueueExecutionPolicyFromWorkloads(workloads []models.AIAgentWorkload) models.AIAgentExecutionPolicy {
	for _, workload := range workloads {
		if workload.ExecutionPolicy != "" {
			return workload.ExecutionPolicy
		}
	}
	return models.AIAgentExecutionPolicyAllMatching
}

func resolveAIAgentQueueEventStatusFromWorkloads(workloads []models.AIAgentWorkload) (models.AIAgentQueueStatus, aiAgentQueueEventProcessSummary, bool) {
	summary := summarizeAIAgentQueueWorkloads(workloads)
	if len(workloads) == 0 {
		return models.AIAgentQueueStatusDone, summary, true
	}
	for _, item := range workloads {
		if item.Status == models.AIAgentWorkloadStatusProcessing {
			return models.AIAgentQueueStatusProcessing, summary, false
		}
	}
	if aiAgentQueueExecutionPolicyFromWorkloads(workloads) == models.AIAgentExecutionPolicyFallbackChain {
		if summary.ProcessedAgents > 0 {
			return models.AIAgentQueueStatusDone, summary, true
		}
		if summary.PendingAgents > 0 {
			return models.AIAgentQueueStatusQueued, summary, true
		}
		if summary.FailedAgents > 0 {
			return models.AIAgentQueueStatusFailed, summary, true
		}
		return models.AIAgentQueueStatusDone, summary, true
	}
	if summary.PendingAgents > 0 {
		return models.AIAgentQueueStatusQueued, summary, true
	}
	if summary.FailedAgents > 0 {
		return models.AIAgentQueueStatusFailed, summary, true
	}
	return models.AIAgentQueueStatusDone, summary, true
}

func summarizeAIAgentQueueWorkloads(workloads []models.AIAgentWorkload) aiAgentQueueEventProcessSummary {
	summary := aiAgentQueueEventProcessSummary{MatchedAgents: len(workloads)}
	errorsList := make([]string, 0, len(workloads))
	for _, workload := range workloads {
		switch workload.Status {
		case models.AIAgentWorkloadStatusDone:
			summary.ProcessedAgents++
		case models.AIAgentWorkloadStatusQueued, models.AIAgentWorkloadStatusProcessing:
			summary.PendingAgents++
		case models.AIAgentWorkloadStatusFailed:
			summary.FailedAgents++
		case models.AIAgentWorkloadStatusCancelled:
			summary.CancelledAgents++
		}
		if strings.TrimSpace(workload.LastError) != "" {
			label := firstNonEmptyString(strings.TrimSpace(workload.AgentName), workload.AgentID.String())
			errorsList = append(errorsList, label+": "+strings.TrimSpace(workload.LastError))
		}
	}
	summary.LastError = joinAIAgentQueueErrors("", errorsList...)
	return summary
}

func joinAIAgentQueueErrors(base string, values ...string) string {
	parts := make([]string, 0, 1+len(values))
	seen := map[string]struct{}{}
	appendPart := func(raw string) {
		value := strings.TrimSpace(raw)
		if value == "" {
			return
		}
		if _, exists := seen[value]; exists {
			return
		}
		seen[value] = struct{}{}
		parts = append(parts, value)
	}
	appendPart(base)
	for _, value := range values {
		appendPart(value)
	}
	return truncateAgentText(strings.Join(parts, "; "), 4000)
}

func aiAgentWorkloadLastStage(result map[string]any) string {
	if len(result) == 0 {
		return "analysis"
	}
	if stages := sliceFromAny(result["plan_stages"]); len(stages) > 0 {
		last := normalizeMap(stages[len(stages)-1])
		if value := firstNonEmptyString(stringFromMap(last, "name", "title"), stringFromMap(last, "id", "key")); value != "" {
			return value
		}
	}
	if strings.TrimSpace(stringFromMap(result, "verdict")) != "" {
		return "analysis"
	}
	if strings.TrimSpace(stringFromMap(result, "error")) != "" {
		return "analysis"
	}
	return "dispatch"
}

func (h *Handler) resolveAIAgentClosedStatus(ctx context.Context, tenantID uuid.UUID) string {
	statuses, err := h.loadCaseStatuses(ctx, tenantID)
	if err != nil {
		return "closed"
	}
	for _, item := range statuses {
		if item.IsClosed {
			return item.Code
		}
	}
	return "closed"
}

func (h *Handler) resolveAIAgentQueueActorID(
	ctx context.Context,
	event models.AIAgentQueueEvent,
	caseItem *models.Case,
	alertItem *models.Alert,
) *uuid.UUID {
	candidates := make([]uuid.UUID, 0, 6)
	if event.ActorID != nil {
		candidates = append(candidates, *event.ActorID)
	}
	if caseItem != nil {
		if caseItem.CreatedBy != nil {
			candidates = append(candidates, *caseItem.CreatedBy)
		}
		if caseItem.AssignedTo != nil {
			candidates = append(candidates, *caseItem.AssignedTo)
		}
	}
	if alertItem != nil {
		if alertItem.CreatedBy != nil {
			candidates = append(candidates, *alertItem.CreatedBy)
		}
		if alertItem.AssignedTo != nil {
			candidates = append(candidates, *alertItem.AssignedTo)
		}
	}
	for _, id := range candidates {
		if id == uuid.Nil {
			continue
		}
		idCopy := id
		return &idCopy
	}
	if h.users != nil {
		users, err := h.users.ListByTenant(ctx, event.TenantID, 50, 0)
		if err == nil {
			for _, item := range users {
				if !item.IsActive || item.ID == uuid.Nil {
					continue
				}
				id := item.ID
				return &id
			}
		}
	}
	return nil
}

func (h *Handler) enqueueAIAgentQueueEvent(
	ctx context.Context,
	tenantID uuid.UUID,
	actorID *uuid.UUID,
	entityType string,
	entityID uuid.UUID,
	source string,
) {
	if h == nil || h.aiAgentQueue == nil {
		return
	}
	if _, _, err := h.aiAgentQueue.Enqueue(ctx, repository.EnqueueAIAgentQueueEventParams{
		TenantID:    tenantID,
		ActorID:     actorID,
		EntityType:  entityType,
		EntityID:    entityID,
		Source:      source,
		MaxAttempts: h.aiAgentQueueMaxAttempts(),
	}); err != nil {
		logger.Warnf("enqueue ai agent queue event failed: tenant=%s entity_type=%s entity_id=%s err=%v", tenantID.String(), entityType, entityID.String(), err)
	}
}

func (h *Handler) recoverStaleAIAgentQueueProcessing(ctx context.Context) {
	if h == nil || h.aiAgentQueue == nil {
		return
	}
	staleBefore := time.Now().UTC().Add(-h.aiAgentQueueRunTimeout())
	if h.aiAgentWorkloads != nil {
		result, err := h.aiAgentWorkloads.RecoverStaleProcessing(ctx, staleBefore)
		if err != nil {
			logger.Warnf("ai agent workload stale processing recovery failed: %v", err)
		} else if result.Requeued > 0 || result.Failed > 0 {
			logger.Warnf("ai agent workloads recovered stale processing: requeued=%d failed=%d", result.Requeued, result.Failed)
		}
	}
	result, err := h.aiAgentQueue.RecoverStaleProcessing(ctx, staleBefore)
	if err != nil {
		logger.Warnf("ai agent queue stale processing recovery failed: %v", err)
		return
	}
	if result.Requeued > 0 || result.Failed > 0 {
		logger.Warnf("ai agent queue recovered stale events: requeued=%d failed=%d", result.Requeued, result.Failed)
	}
}

func (h *Handler) aiAgentQueuePollInterval() time.Duration {
	interval := h.cfg.AI.QueuePoll
	if interval <= 0 {
		interval = 3 * time.Second
	}
	if interval < 500*time.Millisecond {
		interval = 500 * time.Millisecond
	}
	if interval > 30*time.Second {
		interval = 30 * time.Second
	}
	return interval
}

func (h *Handler) aiAgentQueueBatchSize() int {
	limit := h.cfg.AI.QueueBatchSize
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	return limit
}

func (h *Handler) aiAgentQueueRunTimeout() time.Duration {
	timeout := h.cfg.AI.QueueRunTimeout
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	if timeout < 5*time.Second {
		timeout = 5 * time.Second
	}
	if timeout > 10*time.Minute {
		timeout = 10 * time.Minute
	}
	return timeout
}

func (h *Handler) aiAgentQueueMaxRetries() int {
	maxRetries := h.cfg.AI.QueueMaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}
	if maxRetries > 20 {
		maxRetries = 20
	}
	return maxRetries
}

func (h *Handler) aiAgentQueueMaxAttempts() int {
	return h.aiAgentQueueMaxRetries() + 1
}

func aiAgentSupportsTargetType(agent aiAgentDefinition, targetType string) bool {
	if len(agent.TargetTypes) == 0 {
		return strings.EqualFold(strings.TrimSpace(targetType), "case")
	}
	normalized := strings.ToLower(strings.TrimSpace(targetType))
	for _, item := range agent.TargetTypes {
		if strings.ToLower(strings.TrimSpace(item)) == normalized {
			return true
		}
	}
	return false
}

func normalizeAIAgentRunActor(actorID *uuid.UUID) (ownerID *uuid.UUID, createdBy *uuid.UUID) {
	if actorID == nil || *actorID == uuid.Nil {
		return nil, nil
	}
	return actorID, actorID
}

func resolveAIAgentCaseAuthor(actorID *uuid.UUID, alertItem *models.Alert) (uuid.UUID, bool) {
	if actorID != nil && *actorID != uuid.Nil {
		return *actorID, true
	}
	if alertItem != nil {
		if alertItem.CreatedBy != nil && *alertItem.CreatedBy != uuid.Nil {
			return *alertItem.CreatedBy, true
		}
		if alertItem.AssignedTo != nil && *alertItem.AssignedTo != uuid.Nil {
			return *alertItem.AssignedTo, true
		}
	}
	return uuid.Nil, false
}

func severityToCasePriority(severity string) string {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "critical":
		return "critical"
	case "high":
		return "high"
	case "low":
		return "low"
	default:
		return "medium"
	}
}
