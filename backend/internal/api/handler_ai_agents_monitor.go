package api

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"incidenthub/backend/internal/middleware"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

const defaultAIAgentOpsLimit = 20

type aiAgentQueueEventSnapshot struct {
	payload      map[string]any
	candidateIDs []string
}

func (h *Handler) validateAIAgentOpsOverviewDeps() error {
	if h.aiAgentQueue == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "ai agent queue repository is not configured")
	}
	if h.aiAgentWorkloads == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "ai agent workload repository is not configured")
	}
	if h.catalog == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "catalog repository is not configured")
	}
	return nil
}

func parseAIAgentOpsLimit(c *echo.Context) (int, error) {
	limit := defaultAIAgentOpsLimit
	if rawLimit := strings.TrimSpace(c.QueryParam("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil {
			return 0, echo.NewHTTPError(http.StatusBadRequest, "invalid limit")
		}
		limit = parsed
	}
	if limit <= 0 {
		limit = defaultAIAgentOpsLimit
	}
	if limit > 100 {
		limit = 100
	}
	return limit, nil
}

func (h *Handler) GetAIAgentOperationsOverview(c *echo.Context) error {
	if err := h.validateAIAgentOpsOverviewDeps(); err != nil {
		return err
	}
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}

	limit, err := parseAIAgentOpsLimit(c)
	if err != nil {
		return err
	}

	ctx := c.Request().Context()
	counts, err := h.aiAgentQueue.CountByTenantAndStatus(ctx, tenantID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to count ai agent queue events")
	}

	processingEvents, err := h.aiAgentQueue.ListByTenantAndStatus(ctx, tenantID, models.AIAgentQueueStatusProcessing, limit, 0)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list processing ai agent queue events")
	}
	queuedEvents, err := h.aiAgentQueue.ListByTenantAndStatus(ctx, tenantID, models.AIAgentQueueStatusQueued, limit, 0)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list queued ai agent queue events")
	}
	failedEvents, err := h.aiAgentQueue.ListByTenantAndStatus(ctx, tenantID, models.AIAgentQueueStatusFailed, min(limit, 30), 0)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list failed ai agent queue events")
	}

	processingSnapshots := make([]aiAgentQueueEventSnapshot, 0, len(processingEvents))
	for _, event := range processingEvents {
		snapshot := h.buildAIAgentQueueEventSnapshot(ctx, tenantID, event)
		processingSnapshots = append(processingSnapshots, snapshot)
	}
	queuedSnapshots := make([]aiAgentQueueEventSnapshot, 0, len(queuedEvents))
	for _, event := range queuedEvents {
		snapshot := h.buildAIAgentQueueEventSnapshot(ctx, tenantID, event)
		queuedSnapshots = append(queuedSnapshots, snapshot)
	}
	failedSnapshots := make([]map[string]any, 0, len(failedEvents))
	for _, event := range failedEvents {
		snapshot := h.buildAIAgentQueueEventSnapshot(ctx, tenantID, event)
		failedSnapshots = append(failedSnapshots, snapshot.payload)
	}

	agentItems, err := h.catalog.List(ctx, repository.CatalogListParams{
		Kind:     aiAgentCatalogKind,
		TenantID: &tenantID,
		Limit:    500,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list ai agents")
	}
	agents := make([]aiAgentDefinition, 0, len(agentItems))
	for _, item := range agentItems {
		agents = append(agents, parseAIAgentDefinition(item))
	}

	runs, err := h.catalog.List(ctx, repository.CatalogListParams{
		Kind:     aiAgentRunsCatalogKind,
		TenantID: &tenantID,
		Limit:    max(200, limit*20),
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list ai agent runs")
	}
	sort.SliceStable(runs, func(i, j int) bool {
		return runs[i].CreatedAt.After(runs[j].CreatedAt)
	})

	lastRunByAgent := map[string]models.CatalogItem{}
	triadCaseRows := make([]map[string]any, 0, 60)
	for _, run := range runs {
		agentID := strings.TrimSpace(stringFromMap(run.Data, "agent_id", "agentId"))
		if agentID == "" {
			agentID = strings.TrimSpace(run.RefID.String())
		}
		if agentID != "" {
			if _, exists := lastRunByAgent[agentID]; !exists {
				lastRunByAgent[agentID] = run
			}
		}
		for _, rawResult := range sliceFromAny(run.Data["results"]) {
			resultMap := normalizeMap(rawResult)
			if len(resultMap) == 0 {
				continue
			}
			triadMap := normalizeMap(firstMapValue(resultMap, "triad"))
			if len(triadMap) == 0 {
				continue
			}
			row := map[string]any{
				"run_id":                     run.ID.String(),
				"agent_id":                   agentID,
				"agent_name":                 firstNonEmptyString(stringFromMap(run.Data, "agent_name", "agentName"), stringFromMap(resultMap, "agent_name", "agentName")),
				"started_at":                 run.CreatedAt.UTC().Format(time.RFC3339),
				"case_id":                    strings.TrimSpace(stringFromMap(resultMap, "case_id", "caseId")),
				"case_number":                strings.TrimSpace(stringFromMap(resultMap, "case_number", "caseNumber")),
				"title":                      strings.TrimSpace(stringFromMap(resultMap, "title")),
				"verdict":                    strings.ToLower(strings.TrimSpace(stringFromMap(resultMap, "verdict"))),
				"confidence":                 numberOrZero(resultMap, "confidence"),
				"auto_actions_allowed":       boolValueOrDefault(resultMap, true, "auto_actions_allowed", "autoActionsAllowed"),
				"requires_human_review":      boolValueOrDefault(resultMap, false, "requires_human_review", "requiresHumanReview"),
				"reviewer_consensus":         boolValueOrDefault(resultMap, true, "reviewer_consensus", "reviewerConsensus"),
				"auto_action_min_confidence": numberOrZero(resultMap, "auto_action_min_confidence", "autoActionMinConfidence"),
				"action_blockers":            stringSliceFromMap(resultMap, "action_blockers", "actionBlockers"),
				"triad_warnings":             stringSliceFromMap(resultMap, "triad_warnings", "triadWarnings"),
				"triad":                      triadMap,
			}
			triadCaseRows = append(triadCaseRows, row)
			if len(triadCaseRows) >= 120 {
				break
			}
		}
		if len(triadCaseRows) >= 120 {
			break
		}
	}

	agentSummaries := make([]map[string]any, 0, len(agents))
	queuedByAgent := map[string]int{}
	processingByAgent := map[string]int{}
	for _, snapshot := range queuedSnapshots {
		for _, id := range snapshot.candidateIDs {
			queuedByAgent[id]++
		}
	}
	for _, snapshot := range processingSnapshots {
		for _, id := range snapshot.candidateIDs {
			processingByAgent[id]++
		}
	}
	for _, agent := range agents {
		agentID := agent.ID.String()
		lastRun, hasLastRun := lastRunByAgent[agentID]
		lastRunData := lastRun.Data
		lastRunResults := sliceFromAny(lastRunData["results"])
		lastRunErrorCount := 0
		for _, rawResult := range lastRunResults {
			resultMap := normalizeMap(rawResult)
			if strings.TrimSpace(stringFromMap(resultMap, "error")) != "" {
				lastRunErrorCount++
			}
		}
		agentSummaries = append(agentSummaries, map[string]any{
			"agent_id":            agentID,
			"name":                agent.Name,
			"enabled":             agent.Enabled,
			"triad_enabled":       agent.TriadEnabled,
			"triad_critical_only": agent.TriadCriticalOnly,
			"target_types":        append([]string{}, agent.TargetTypes...),
			"queue_matches":       queuedByAgent[agentID],
			"processing_items":    processingByAgent[agentID],
			"last_run_id":         emptyOrID(hasLastRun, lastRun.ID),
			"last_run_at":         nullableRFC3339(hasLastRun, lastRun.CreatedAt),
			"last_run_status":     firstNonEmptyString(stringFromMap(lastRunData, "status"), ternaryString(hasLastRun, "completed", "never")),
			"last_run_processed":  intOrZero(lastRunData, "processed_cases", "processedCases"),
			"last_run_successful": intOrZero(lastRunData, "successful_cases", "successfulCases"),
			"last_run_errors":     lastRunErrorCount,
		})
	}
	sort.SliceStable(agentSummaries, func(i, j int) bool {
		left := agentSummaries[i]
		right := agentSummaries[j]
		if intOrZero(left, "processing_items") != intOrZero(right, "processing_items") {
			return intOrZero(left, "processing_items") > intOrZero(right, "processing_items")
		}
		if intOrZero(left, "queue_matches") != intOrZero(right, "queue_matches") {
			return intOrZero(left, "queue_matches") > intOrZero(right, "queue_matches")
		}
		return strings.ToLower(strings.TrimSpace(stringFromMap(left, "name"))) < strings.ToLower(strings.TrimSpace(stringFromMap(right, "name")))
	})

	triadTotal := len(triadCaseRows)
	triadBlocked := 0
	triadHumanReview := 0
	triadConsensus := 0
	for _, row := range triadCaseRows {
		if !boolValueOrDefault(row, true, "auto_actions_allowed", "autoActionsAllowed") {
			triadBlocked++
		}
		if boolValueOrDefault(row, false, "requires_human_review", "requiresHumanReview") {
			triadHumanReview++
		}
		if boolValueOrDefault(row, false, "reviewer_consensus", "reviewerConsensus") {
			triadConsensus++
		}
	}
	triadConsensusRate := 0.0
	if triadTotal > 0 {
		triadConsensusRate = (float64(triadConsensus) / float64(triadTotal)) * 100
	}

	payload := map[string]any{
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"workers": map[string]any{
			"enabled":            h.cfg.AI.QueueEnabled,
			"poll_interval_ms":   h.aiAgentQueuePollInterval().Milliseconds(),
			"batch_size":         h.aiAgentQueueBatchSize(),
			"run_timeout_ms":     h.aiAgentQueueRunTimeout().Milliseconds(),
			"max_retries":        h.aiAgentQueueMaxRetries(),
			"max_attempts":       h.aiAgentQueueMaxAttempts(),
			"llm_max_concurrent": max(1, h.cfg.AI.MaxConcurrent),
		},
		"queue": map[string]any{
			"queued":     counts[models.AIAgentQueueStatusQueued],
			"processing": counts[models.AIAgentQueueStatusProcessing],
			"done":       counts[models.AIAgentQueueStatusDone],
			"failed":     counts[models.AIAgentQueueStatusFailed],
			"total":      counts[models.AIAgentQueueStatusQueued] + counts[models.AIAgentQueueStatusProcessing] + counts[models.AIAgentQueueStatusDone] + counts[models.AIAgentQueueStatusFailed],
		},
		"processing_events":    flattenQueueSnapshots(processingSnapshots),
		"queued_events":        flattenQueueSnapshots(queuedSnapshots),
		"recent_failed_events": failedSnapshots,
		"agent_runtime":        agentSummaries,
		"triad_recent_analytics": map[string]any{
			"total_cases":             triadTotal,
			"blocked_auto_actions":    triadBlocked,
			"requires_human_review":   triadHumanReview,
			"reviewer_consensus_rate": triadConsensusRate,
			"cases":                   triadCaseRows,
		},
	}
	return c.JSON(http.StatusOK, payload)
}

func (h *Handler) GetAIAgentEntityTrace(c *echo.Context) error {
	if h.aiAgentQueue == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "ai agent queue repository is not configured")
	}
	if h.aiAgentWorkloads == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "ai agent workload repository is not configured")
	}
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	entityType := strings.ToLower(strings.TrimSpace(c.Param("entityType")))
	switch entityType {
	case "case", "cases":
		entityType = "case"
	case "alert", "alerts":
		entityType = "alert"
	default:
		return echo.NewHTTPError(http.StatusBadRequest, "invalid entityType path param")
	}
	entityID, err := uuid.Parse(strings.TrimSpace(c.Param("entityID")))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid entityID path param")
	}
	limit := 8
	if rawLimit := strings.TrimSpace(c.QueryParam("limit")); rawLimit != "" {
		parsed, parseErr := strconv.Atoi(rawLimit)
		if parseErr != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid limit")
		}
		limit = parsed
	}
	if limit <= 0 {
		limit = 8
	}
	if limit > 20 {
		limit = 20
	}

	ctx := c.Request().Context()
	switch entityType {
	case "case":
		item, getErr := h.cases.GetByID(ctx, tenantID, entityID)
		if getErr != nil || item == nil {
			return echo.NewHTTPError(http.StatusNotFound, "case not found")
		}
	case "alert":
		item, getErr := h.alerts.GetByID(ctx, tenantID, entityID)
		if getErr != nil || item == nil {
			return echo.NewHTTPError(http.StatusNotFound, "alert not found")
		}
	}

	events, err := h.aiAgentQueue.ListByEntity(ctx, tenantID, entityType, entityID, limit)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list ai agent entity trace")
	}
	payloads := make([]map[string]any, 0, len(events))
	activeEventID := ""
	for _, event := range events {
		snapshot := h.buildAIAgentQueueEventSnapshot(ctx, tenantID, event)
		payloads = append(payloads, snapshot.payload)
		if activeEventID == "" && (event.Status == models.AIAgentQueueStatusQueued || event.Status == models.AIAgentQueueStatusProcessing) {
			activeEventID = event.ID.String()
		}
	}

	return c.JSON(http.StatusOK, map[string]any{
		"entity_type":     entityType,
		"entity_id":       entityID.String(),
		"events":          payloads,
		"active_event_id": activeEventID,
		"total":           len(payloads),
	})
}

func (h *Handler) RestartAIAgentQueueEvent(c *echo.Context) error {
	if h.aiAgentQueue == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "ai agent queue repository is not configured")
	}
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	identity, hasIdentity := middleware.GetIdentity(c)
	if !hasIdentity {
		return echo.NewHTTPError(http.StatusUnauthorized, "identity required")
	}
	eventID, err := uuid.Parse(strings.TrimSpace(c.Param("eventID")))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid eventID path param")
	}
	ctx := c.Request().Context()
	item, err := h.aiAgentQueue.GetByID(ctx, eventID)
	if err != nil || item == nil {
		return echo.NewHTTPError(http.StatusNotFound, "ai agent queue event not found")
	}
	if item.TenantID != tenantID {
		return echo.NewHTTPError(http.StatusNotFound, "ai agent queue event not found in tenant")
	}
	if h.aiAgentWorkloads != nil {
		if workloads, loadErr := h.aiAgentWorkloads.ListByQueueEvent(ctx, eventID); loadErr == nil {
			for _, workload := range workloads {
				if _, restartErr := h.aiAgentWorkloads.Restart(ctx, workload.ID, h.aiAgentQueueMaxAttempts()); restartErr != nil {
					return echo.NewHTTPError(http.StatusInternalServerError, "failed to restart ai agent workloads")
				}
			}
		}
	}
	restarted, err := h.aiAgentQueue.Restart(ctx, eventID, h.aiAgentQueueMaxAttempts())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to restart ai agent queue event")
	}
	_ = h.audits.Log(ctx, &tenantID, &identity.UserID, "ai_agent_queue_event_restart", "ai_agent_queue_event", &eventID, map[string]any{
		"event_id":      eventID.String(),
		"entity_type":   strings.ToLower(strings.TrimSpace(restarted.EntityType)),
		"entity_id":     restarted.EntityID.String(),
		"attempt_count": restarted.AttemptCount,
		"max_attempts":  restarted.MaxAttempts,
	})
	return c.JSON(http.StatusOK, map[string]any{
		"event": h.buildAIAgentQueueEventSnapshot(ctx, tenantID, *restarted).payload,
	})
}

func (h *Handler) CloseAIAgentQueueEvent(c *echo.Context) error {
	if h.aiAgentQueue == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "ai agent queue repository is not configured")
	}
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	identity, hasIdentity := middleware.GetIdentity(c)
	if !hasIdentity {
		return echo.NewHTTPError(http.StatusUnauthorized, "identity required")
	}
	eventID, err := uuid.Parse(strings.TrimSpace(c.Param("eventID")))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid eventID path param")
	}
	ctx := c.Request().Context()
	item, err := h.aiAgentQueue.GetByID(ctx, eventID)
	if err != nil || item == nil {
		return echo.NewHTTPError(http.StatusNotFound, "ai agent queue event not found")
	}
	if item.TenantID != tenantID {
		return echo.NewHTTPError(http.StatusNotFound, "ai agent queue event not found in tenant")
	}
	if h.aiAgentWorkloads != nil {
		if workloads, loadErr := h.aiAgentWorkloads.ListByQueueEvent(ctx, eventID); loadErr == nil {
			for _, workload := range workloads {
				if workload.Status == models.AIAgentWorkloadStatusDone || workload.Status == models.AIAgentWorkloadStatusCancelled {
					continue
				}
				if _, closeErr := h.aiAgentWorkloads.CloseByUser(ctx, workload.ID); closeErr != nil {
					return echo.NewHTTPError(http.StatusInternalServerError, "failed to close ai agent workloads")
				}
			}
		}
	}
	closed, err := h.aiAgentQueue.CloseByUser(ctx, eventID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to close ai agent queue event")
	}
	_ = h.audits.Log(ctx, &tenantID, &identity.UserID, "ai_agent_queue_event_close", "ai_agent_queue_event", &eventID, map[string]any{
		"event_id":      eventID.String(),
		"entity_type":   strings.ToLower(strings.TrimSpace(closed.EntityType)),
		"entity_id":     closed.EntityID.String(),
		"attempt_count": closed.AttemptCount,
		"max_attempts":  closed.MaxAttempts,
		"closed_at":     nullableTimeRFC3339(closed.ClosedAt),
	})
	return c.JSON(http.StatusOK, map[string]any{
		"event": h.buildAIAgentQueueEventSnapshot(ctx, tenantID, *closed).payload,
	})
}

func (h *Handler) RestartAIAgentWorkload(c *echo.Context) error {
	if h.aiAgentWorkloads == nil || h.aiAgentQueue == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "ai agent workload repository is not configured")
	}
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	identity, hasIdentity := middleware.GetIdentity(c)
	if !hasIdentity {
		return echo.NewHTTPError(http.StatusUnauthorized, "identity required")
	}
	workloadID, err := uuid.Parse(strings.TrimSpace(c.Param("workloadID")))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid workloadID path param")
	}
	ctx := c.Request().Context()
	item, err := h.aiAgentWorkloads.GetByID(ctx, workloadID)
	if err != nil || item == nil || item.TenantID != tenantID {
		return echo.NewHTTPError(http.StatusNotFound, "ai agent workload not found")
	}
	if item.Status == models.AIAgentWorkloadStatusProcessing {
		return echo.NewHTTPError(http.StatusConflict, "cannot restart workload while it is processing")
	}
	restarted, err := h.aiAgentWorkloads.Restart(ctx, workloadID, h.aiAgentQueueMaxAttempts())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to restart ai agent workload")
	}
	if syncErr := h.syncAIAgentQueueEventFromWorkloads(ctx, restarted.QueueEventID); syncErr != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to sync parent ai agent queue event")
	}
	parent, _ := h.aiAgentQueue.GetByID(ctx, restarted.QueueEventID)
	_ = h.audits.Log(ctx, &tenantID, &identity.UserID, "ai_agent_workload_restart", "ai_agent_workload", &workloadID, map[string]any{
		"workload_id":    workloadID.String(),
		"queue_event_id": restarted.QueueEventID.String(),
		"agent_id":       restarted.AgentID.String(),
		"entity_type":    strings.ToLower(strings.TrimSpace(restarted.EntityType)),
		"entity_id":      restarted.EntityID.String(),
		"parent_status":  ternaryString(parent != nil, strings.ToLower(strings.TrimSpace(string(parent.Status))), ""),
	})
	payload := h.buildAIAgentWorkloadSnapshot(ctx, tenantID, *restarted)
	return c.JSON(http.StatusOK, map[string]any{"workload": payload})
}

func (h *Handler) CloseAIAgentWorkload(c *echo.Context) error {
	if h.aiAgentWorkloads == nil || h.aiAgentQueue == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "ai agent workload repository is not configured")
	}
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	identity, hasIdentity := middleware.GetIdentity(c)
	if !hasIdentity {
		return echo.NewHTTPError(http.StatusUnauthorized, "identity required")
	}
	workloadID, err := uuid.Parse(strings.TrimSpace(c.Param("workloadID")))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid workloadID path param")
	}
	ctx := c.Request().Context()
	item, err := h.aiAgentWorkloads.GetByID(ctx, workloadID)
	if err != nil || item == nil || item.TenantID != tenantID {
		return echo.NewHTTPError(http.StatusNotFound, "ai agent workload not found")
	}
	if item.Status == models.AIAgentWorkloadStatusProcessing {
		return echo.NewHTTPError(http.StatusConflict, "cannot close workload while it is processing")
	}
	closed, err := h.aiAgentWorkloads.CloseByUser(ctx, workloadID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to close ai agent workload")
	}
	if syncErr := h.syncAIAgentQueueEventFromWorkloads(ctx, closed.QueueEventID); syncErr != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to sync parent ai agent queue event")
	}
	parent, _ := h.aiAgentQueue.GetByID(ctx, closed.QueueEventID)
	_ = h.audits.Log(ctx, &tenantID, &identity.UserID, "ai_agent_workload_close", "ai_agent_workload", &workloadID, map[string]any{
		"workload_id":    workloadID.String(),
		"queue_event_id": closed.QueueEventID.String(),
		"agent_id":       closed.AgentID.String(),
		"entity_type":    strings.ToLower(strings.TrimSpace(closed.EntityType)),
		"entity_id":      closed.EntityID.String(),
		"parent_status":  ternaryString(parent != nil, strings.ToLower(strings.TrimSpace(string(parent.Status))), ""),
	})
	payload := h.buildAIAgentWorkloadSnapshot(ctx, tenantID, *closed)
	return c.JSON(http.StatusOK, map[string]any{"workload": payload})
}

func flattenQueueSnapshots(items []aiAgentQueueEventSnapshot) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, item.payload)
	}
	return out
}

func (h *Handler) buildAIAgentQueueEventSnapshot(
	ctx context.Context,
	tenantID uuid.UUID,
	event models.AIAgentQueueEvent,
) aiAgentQueueEventSnapshot {
	payload := map[string]any{
		"id":               event.ID.String(),
		"entity_type":      strings.ToLower(strings.TrimSpace(event.EntityType)),
		"entity_id":        event.EntityID.String(),
		"source":           strings.TrimSpace(event.Source),
		"status":           strings.ToLower(strings.TrimSpace(string(event.Status))),
		"workflow_id":      strings.TrimSpace(event.WorkflowID),
		"matched_agents":   event.MatchedAgents,
		"processed_agents": event.ProcessedAgents,
		"attempt_count":    event.AttemptCount,
		"max_attempts":     event.MaxAttempts,
		"retries_left":     max(0, event.MaxAttempts-event.AttemptCount),
		"last_error":       strings.TrimSpace(event.LastError),
		"closed_by_user":   event.ClosedByUser,
		"created_at":       event.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at":       event.UpdatedAt.UTC().Format(time.RFC3339),
		"started_at":       nullableTimeRFC3339(event.StartedAt),
		"finished_at":      nullableTimeRFC3339(event.FinishedAt),
		"closed_at":        nullableTimeRFC3339(event.ClosedAt),
	}

	var caseItem *models.Case
	var alertItem *models.Alert
	switch strings.ToLower(strings.TrimSpace(event.EntityType)) {
	case "case":
		caseItem, _ = h.cases.GetByID(ctx, tenantID, event.EntityID)
		if caseItem != nil {
			payload["entity_reference"] = firstNonEmptyString(strings.TrimSpace(caseItem.CaseNumber), caseItem.ID.String())
			payload["entity_title"] = strings.TrimSpace(caseItem.Title)
			payload["entity_severity"] = strings.ToLower(strings.TrimSpace(caseItem.Severity))
			payload["entity_status"] = strings.ToLower(strings.TrimSpace(caseItem.Status))
		}
	case "alert":
		alertItem, _ = h.alerts.GetByID(ctx, tenantID, event.EntityID)
		if alertItem != nil {
			payload["entity_reference"] = firstNonEmptyString(strings.TrimSpace(alertItem.ID.String()), event.EntityID.String())
			payload["entity_title"] = strings.TrimSpace(alertItem.Title)
			payload["entity_severity"] = strings.ToLower(strings.TrimSpace(alertItem.Severity))
			payload["entity_status"] = strings.ToLower(strings.TrimSpace(alertItem.Status))
		}
	}
	if strings.TrimSpace(stringFromMap(payload, "entity_reference")) == "" {
		payload["entity_reference"] = event.EntityID.String()
	}
	if strings.TrimSpace(stringFromMap(payload, "entity_title")) == "" {
		payload["entity_title"] = "Entity " + event.EntityID.String()
	}

	candidates := []aiAgentDefinition{}
	matched, err := h.loadMatchingAIAgentsForQueueEvent(ctx, event, caseItem, alertItem)
	if err == nil {
		candidates = selectAIAgentQueueExecutionPlan(matched)
	}
	candidateAgents := make([]map[string]any, 0, len(candidates))
	candidateIDs := make([]string, 0, len(candidates))
	for _, agent := range candidates {
		candidateIDs = append(candidateIDs, agent.ID.String())
		candidateAgents = append(candidateAgents, map[string]any{
			"id":                  agent.ID.String(),
			"name":                agent.Name,
			"enabled":             agent.Enabled,
			"triad_enabled":       agent.TriadEnabled,
			"triad_critical_only": agent.TriadCriticalOnly,
			"target_types":        append([]string{}, agent.TargetTypes...),
			"execution_policy":    agent.ExecutionPolicy,
			"execution_priority":  agent.ExecutionPriority,
		})
	}
	payload["candidate_agents"] = candidateAgents
	payload["candidate_count"] = len(candidateAgents)
	workloadPayloads := h.buildAIAgentWorkloadSnapshots(ctx, tenantID, event.ID)
	payload["workloads"] = workloadPayloads
	payload["workload_count"] = len(workloadPayloads)
	payload["execution_policy"] = string(aiAgentQueueExecutionPolicyFromWorkloadsFromPayload(workloadPayloads))
	return aiAgentQueueEventSnapshot{
		payload:      payload,
		candidateIDs: candidateIDs,
	}
}

func (h *Handler) buildAIAgentWorkloadSnapshots(ctx context.Context, tenantID uuid.UUID, queueEventID uuid.UUID) []map[string]any {
	if h == nil || h.aiAgentWorkloads == nil {
		return []map[string]any{}
	}
	items, err := h.aiAgentWorkloads.ListByQueueEvent(ctx, queueEventID)
	if err != nil {
		return []map[string]any{}
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, h.buildAIAgentWorkloadSnapshot(ctx, tenantID, item))
	}
	return out
}

func (h *Handler) buildAIAgentWorkloadSnapshot(ctx context.Context, tenantID uuid.UUID, item models.AIAgentWorkload) map[string]any {
	payload := map[string]any{
		"id":                 item.ID.String(),
		"queue_event_id":     item.QueueEventID.String(),
		"agent_id":           item.AgentID.String(),
		"agent_name":         strings.TrimSpace(item.AgentName),
		"execution_policy":   string(item.ExecutionPolicy),
		"execution_priority": item.ExecutionPriority,
		"execution_index":    item.ExecutionIndex,
		"entity_type":        strings.ToLower(strings.TrimSpace(item.EntityType)),
		"entity_id":          item.EntityID.String(),
		"status":             strings.ToLower(strings.TrimSpace(string(item.Status))),
		"attempt_count":      item.AttemptCount,
		"max_attempts":       item.MaxAttempts,
		"retries_left":       max(0, item.MaxAttempts-item.AttemptCount),
		"workflow_id":        strings.TrimSpace(item.WorkflowID),
		"run_id":             aiAgentRunID(item.RunID),
		"last_stage":         strings.TrimSpace(item.LastStage),
		"last_error":         strings.TrimSpace(item.LastError),
		"closed_by_user":     item.ClosedByUser,
		"created_at":         item.CreatedAt.UTC().Format(time.RFC3339),
		"started_at":         nullableTimeRFC3339(item.StartedAt),
		"finished_at":        nullableTimeRFC3339(item.FinishedAt),
		"closed_at":          nullableTimeRFC3339(item.ClosedAt),
		"updated_at":         item.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if item.RunID != nil && *item.RunID != uuid.Nil && h.catalog != nil {
		if run, err := h.catalog.GetByID(ctx, aiAgentRunsCatalogKind, *item.RunID, &tenantID); err == nil && run != nil {
			payload["run"] = buildAIAgentRunSnapshot(*run)
		}
	}
	return payload
}

func buildAIAgentRunSnapshot(run models.CatalogItem) map[string]any {
	payload := map[string]any{
		"id":          run.ID.String(),
		"status":      strings.TrimSpace(stringFromMap(run.Data, "status", "run_status")),
		"started_at":  firstNonEmptyString(stringFromMap(run.Data, "started_at", "startedAt"), run.CreatedAt.UTC().Format(time.RFC3339)),
		"finished_at": strings.TrimSpace(stringFromMap(run.Data, "finished_at", "finishedAt")),
	}
	if results := sliceFromAny(run.Data["results"]); len(results) > 0 {
		first := normalizeMap(results[0])
		if len(first) > 0 {
			payload["result"] = map[string]any{
				"case_id":               strings.TrimSpace(stringFromMap(first, "case_id", "caseId")),
				"case_number":           strings.TrimSpace(stringFromMap(first, "case_number", "caseNumber")),
				"title":                 strings.TrimSpace(stringFromMap(first, "title")),
				"verdict":               strings.TrimSpace(stringFromMap(first, "verdict")),
				"confidence":            numberOrZero(first, "confidence"),
				"summary":               strings.TrimSpace(stringFromMap(first, "summary")),
				"error":                 strings.TrimSpace(stringFromMap(first, "error")),
				"auto_actions_allowed":  boolValueOrDefault(first, true, "auto_actions_allowed", "autoActionsAllowed"),
				"requires_human_review": boolValueOrDefault(first, false, "requires_human_review", "requiresHumanReview"),
				"reviewer_consensus":    boolValueOrDefault(first, true, "reviewer_consensus", "reviewerConsensus"),
				"action_blockers":       stringSliceFromMap(first, "action_blockers", "actionBlockers"),
				"stage_timeline":        buildAIAgentStageTimelineSnapshot(first),
				"connector_timeline":    buildAIAgentConnectorTimelineSnapshot(first),
			}
		}
	}
	return payload
}

func buildAIAgentStageTimelineSnapshot(result map[string]any) []map[string]any {
	raw := sliceFromAny(firstMapValue(result, "stage_timeline", "stageTimeline"))
	if len(raw) == 0 {
		raw = deriveAIAgentStageTimelineFromLegacyResult(result)
	}
	out := make([]map[string]any, 0, len(raw))
	for idx, item := range raw {
		stage := normalizeMap(item)
		if len(stage) == 0 {
			continue
		}
		out = append(out, map[string]any{
			"id":                      firstNonEmptyString(stringFromMap(stage, "id"), fmt.Sprintf("stage-%d", idx+1)),
			"name":                    firstNonEmptyString(stringFromMap(stage, "name"), fmt.Sprintf("Stage %d", idx+1)),
			"description":             strings.TrimSpace(stringFromMap(stage, "description")),
			"prompt":                  strings.TrimSpace(stringFromMap(stage, "prompt")),
			"status":                  strings.TrimSpace(stringFromMap(stage, "status")),
			"execution_index":         intOrZero(stage, "execution_index", "executionIndex"),
			"connector_ids":           stringSliceFromMap(stage, "connector_ids", "connectorIds"),
			"connector_count":         intOrZero(stage, "connector_count", "connectorCount"),
			"connector_success_count": intOrZero(stage, "connector_success_count", "connectorSuccessCount"),
			"connector_error_count":   intOrZero(stage, "connector_error_count", "connectorErrorCount"),
			"case_tags":               stringSliceFromMap(stage, "case_tags", "caseTags"),
			"started_at":              strings.TrimSpace(stringFromMap(stage, "started_at", "startedAt")),
			"finished_at":             strings.TrimSpace(stringFromMap(stage, "finished_at", "finishedAt")),
			"duration_ms":             intOrZero(stage, "duration_ms", "durationMs"),
			"verdict":                 strings.TrimSpace(stringFromMap(stage, "verdict")),
			"confidence":              numberOrZero(stage, "confidence"),
			"error":                   strings.TrimSpace(stringFromMap(stage, "error")),
		})
	}
	return out
}

func deriveAIAgentStageTimelineFromLegacyResult(result map[string]any) []any {
	out := make([]any, 0)
	for idx, raw := range sliceFromAny(firstMapValue(result, "plan_stages", "planStages")) {
		stage := normalizeMap(raw)
		if len(stage) == 0 {
			continue
		}
		out = append(out, map[string]any{
			"id":              firstNonEmptyString(stringFromMap(stage, "id"), fmt.Sprintf("stage-%d", idx+1)),
			"name":            firstNonEmptyString(stringFromMap(stage, "name"), fmt.Sprintf("Stage %d", idx+1)),
			"description":     strings.TrimSpace(stringFromMap(stage, "description")),
			"prompt":          strings.TrimSpace(stringFromMap(stage, "prompt")),
			"status":          "completed",
			"execution_index": idx,
			"connector_count": intOrZero(stage, "enrichment_count", "enrichmentCount"),
			"case_tags":       stringSliceFromMap(stage, "case_tags", "caseTags"),
		})
	}
	hasSummary := strings.TrimSpace(stringFromMap(result, "summary")) != ""
	hasError := strings.TrimSpace(stringFromMap(result, "error")) != ""
	hasVerdict := strings.TrimSpace(stringFromMap(result, "verdict")) != ""
	if len(out) == 0 && (hasSummary || hasError || hasVerdict) {
		status := "completed"
		if hasError {
			status = "failed"
		}
		out = append(out, map[string]any{
			"id":              "analysis",
			"name":            "Analysis",
			"status":          status,
			"execution_index": 0,
			"verdict":         strings.TrimSpace(stringFromMap(result, "verdict")),
			"confidence":      numberOrZero(result, "confidence"),
			"error":           strings.TrimSpace(stringFromMap(result, "error")),
		})
	}
	return out
}

func buildAIAgentConnectorTimelineSnapshot(result map[string]any) []map[string]any {
	raw := sliceFromAny(firstMapValue(result, "connector_timeline", "connectorTimeline"))
	if len(raw) == 0 {
		raw = sliceFromAny(firstMapValue(result, "enrichment"))
	}
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		connector := normalizeMap(item)
		if len(connector) == 0 {
			continue
		}
		status := strings.TrimSpace(stringFromMap(connector, "status"))
		if status == "" {
			if strings.TrimSpace(stringFromMap(connector, "error")) != "" {
				status = "failed"
			} else {
				status = "completed"
			}
		}
		out = append(out, map[string]any{
			"stage_id":        strings.TrimSpace(stringFromMap(connector, "stage_id", "stageId")),
			"stage_name":      strings.TrimSpace(stringFromMap(connector, "stage_name", "stageName")),
			"execution_index": intOrZero(connector, "execution_index", "executionIndex"),
			"connector_id":    strings.TrimSpace(stringFromMap(connector, "connector_id", "connectorId")),
			"name":            strings.TrimSpace(stringFromMap(connector, "name")),
			"channel":         strings.TrimSpace(stringFromMap(connector, "channel")),
			"status":          status,
			"execution_id":    strings.TrimSpace(stringFromMap(connector, "execution_id", "executionId")),
			"reply":           strings.TrimSpace(stringFromMap(connector, "reply")),
			"error":           strings.TrimSpace(stringFromMap(connector, "error")),
			"conversation_id": strings.TrimSpace(stringFromMap(connector, "conversation_id", "conversationId")),
			"cursor":          strings.TrimSpace(stringFromMap(connector, "cursor")),
			"metadata":        normalizeMap(firstMapValue(connector, "metadata")),
			"started_at":      strings.TrimSpace(stringFromMap(connector, "started_at", "startedAt")),
			"finished_at":     strings.TrimSpace(stringFromMap(connector, "finished_at", "finishedAt")),
			"duration_ms":     intOrZero(connector, "duration_ms", "durationMs"),
		})
	}
	return out
}

func syncAIAgentQueueEventPayloadStatus(items []models.AIAgentWorkload) (models.AIAgentQueueStatus, repository.CompleteAIAgentQueueEventParams, bool) {
	status, summary, canUpdate := resolveAIAgentQueueEventStatusFromWorkloads(items)
	payload := repository.CompleteAIAgentQueueEventParams{
		MatchedAgents:   summary.MatchedAgents,
		ProcessedAgents: summary.ProcessedAgents,
		LastError:       summary.LastError,
	}
	return status, payload, canUpdate
}

func (h *Handler) syncAIAgentQueueEventFromWorkloads(ctx context.Context, eventID uuid.UUID) error {
	if h == nil || h.aiAgentQueue == nil || h.aiAgentWorkloads == nil {
		return nil
	}
	items, err := h.aiAgentWorkloads.ListByQueueEvent(ctx, eventID)
	if err != nil {
		return err
	}
	status, payload, canUpdate := syncAIAgentQueueEventPayloadStatus(items)
	if !canUpdate {
		return nil
	}
	switch status {
	case models.AIAgentQueueStatusQueued:
		if err := h.aiAgentQueue.RequeueAfterFailure(ctx, eventID, payload); err != nil {
			return err
		}
	case models.AIAgentQueueStatusFailed:
		if err := h.aiAgentQueue.MarkFailed(ctx, eventID, payload); err != nil {
			return err
		}
	default:
		if err := h.aiAgentQueue.MarkDone(ctx, eventID, payload); err != nil {
			return err
		}
	}
	return nil
}

func aiAgentQueueExecutionPolicyFromWorkloadsFromPayload(workloads []map[string]any) models.AIAgentExecutionPolicy {
	for _, workload := range workloads {
		policy := normalizeAIAgentExecutionPolicy(stringFromMap(workload, "execution_policy", "executionPolicy"))
		if policy != "" {
			return models.AIAgentExecutionPolicy(policy)
		}
	}
	return models.AIAgentExecutionPolicyAllMatching
}

func aiAgentRunID(value *uuid.UUID) string {
	if value == nil || *value == uuid.Nil {
		return ""
	}
	return value.String()
}

func nullableTimeRFC3339(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC().Format(time.RFC3339)
}

func nullableRFC3339(ok bool, value time.Time) any {
	if !ok {
		return nil
	}
	if value.IsZero() {
		return nil
	}
	return value.UTC().Format(time.RFC3339)
}

func emptyOrID(ok bool, value uuid.UUID) string {
	if !ok || value == uuid.Nil {
		return ""
	}
	return value.String()
}

func sliceFromAny(value any) []any {
	switch typed := value.(type) {
	case []any:
		return typed
	case []map[string]any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, item)
		}
		return out
	default:
		return nil
	}
}

func boolValueOrDefault(payload map[string]any, fallback bool, keys ...string) bool {
	if parsed, ok := boolFromMap(payload, keys...); ok {
		return parsed
	}
	return fallback
}

func numberOrZero(payload map[string]any, keys ...string) float64 {
	if parsed, ok := floatFromMap(payload, keys...); ok {
		return parsed
	}
	return 0
}

func intOrZero(payload map[string]any, keys ...string) int {
	if parsed, ok := intFromMap(payload, keys...); ok {
		return parsed
	}
	return 0
}

func ternaryString(condition bool, left, right string) string {
	if condition {
		return left
	}
	return right
}
