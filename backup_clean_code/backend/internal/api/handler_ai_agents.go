package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"incidenthub/backend/internal/ai"
	"incidenthub/backend/internal/middleware"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

const (
	aiAgentCatalogKind     = "ai_agents"
	aiAgentRunsCatalogKind = "ai_agent_runs"
)

type runAIAgentRequest struct {
	MaxCases int      `json:"max_cases"`
	DryRun   bool     `json:"dry_run"`
	Language string   `json:"language"`
	CaseIDs  []string `json:"case_ids"`
	AlertIDs []string `json:"alert_ids"`
}

type aiAgentDefinition struct {
	ID                       uuid.UUID
	Name                     string
	Description              string
	Prompt                   string
	Model                    string
	Provider                 string
	Endpoint                 string
	Language                 string
	Enabled                  bool
	TargetTypes              []string
	CaseTags                 []string
	AlertSources             []string
	AutoCaseTags             []string
	InvestigationPlan        []aiAgentInvestigationStage
	EnrichmentConnectorIDs   []string
	NotificationConnectorIDs []string
	MaxCasesPerRun           int
	AutoCreateTasks          bool
	AutoComment              bool
	AutoCloseCase            bool
	AutoCloseVerdicts        []string
	AutoCreateCaseFromAlert  bool
	TaskAssigneeID           string
	TriadEnabled             bool
	TriadCriticalOnly        bool
	InvestigatorPrompt       string
	ReviewerPrompt           string
	ArbiterPrompt            string
	RequireReviewerConsensus bool
	AutoActionMinConfidence  float64
	ExecutionPolicy          string
	ExecutionPriority        int
}

type aiAgentInvestigationStage struct {
	ID                     string
	Name                   string
	Description            string
	Prompt                 string
	EnrichmentConnectorIDs []string
	CaseTags               []string
}

func (h *Handler) ListAIAgentRuns(c *echo.Context) error {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	agentID, err := uuid.Parse(strings.TrimSpace(c.Param("agentID")))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid agent id")
	}
	if _, getErr := h.catalog.GetByID(c.Request().Context(), aiAgentCatalogKind, agentID, &tenantID); getErr != nil {
		return echo.NewHTTPError(http.StatusNotFound, "ai agent not found")
	}
	limit := 30
	if rawLimit := strings.TrimSpace(c.QueryParam("limit")); rawLimit != "" {
		if parsed, parseErr := strconv.Atoi(rawLimit); parseErr == nil {
			limit = parsed
		}
	}
	if limit <= 0 || limit > 200 {
		limit = 30
	}
	items, err := h.catalog.List(c.Request().Context(), repository.CatalogListParams{
		Kind:     aiAgentRunsCatalogKind,
		TenantID: &tenantID,
		RefID:    &agentID,
		Limit:    limit,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list ai agent runs")
	}
	return c.JSON(http.StatusOK, mapCatalogItems(items))
}

func (h *Handler) RunAIAgent(c *echo.Context) error {
	if h.ai == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "ai service is not configured")
	}
	if h.catalog == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "catalog repository is not configured")
	}
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	identity, _ := middleware.GetIdentity(c)
	agentID, err := uuid.Parse(strings.TrimSpace(c.Param("agentID")))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid agent id")
	}

	agentItem, err := h.catalog.GetByID(c.Request().Context(), aiAgentCatalogKind, agentID, &tenantID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "ai agent not found")
	}
	agent := parseAIAgentDefinition(*agentItem)
	if !agent.Enabled {
		return echo.NewHTTPError(http.StatusBadRequest, "ai agent is disabled")
	}
	runtimeAI, runtimeErr := h.resolveAIAgentService(agent)
	if runtimeErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, runtimeErr.Error())
	}

	var req runAIAgentRequest
	if c.Request().ContentLength > 0 {
		if bindErr := c.Bind(&req); bindErr != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
		}
	}
	maxCases := agent.MaxCasesPerRun
	if req.MaxCases > 0 {
		maxCases = req.MaxCases
	}
	if maxCases <= 0 {
		maxCases = 10
	}
	if maxCases > 100 {
		maxCases = 100
	}
	language := normalizeAILanguage(req.Language)
	if language == "" {
		language = normalizeAILanguage(agent.Language)
	}

	requestedCaseIDs, caseParseErr := parseUniqueUUIDList(req.CaseIDs)
	if caseParseErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid case id in case_ids")
	}
	requestedAlertIDs, alertParseErr := parseUniqueUUIDList(req.AlertIDs)
	if alertParseErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid alert id in alert_ids")
	}

	var rawCases []models.Case
	if len(requestedCaseIDs) > 0 {
		rawCases = h.loadAIAgentRequestedCases(c.Request().Context(), tenantID, agent, requestedCaseIDs)
		if len(rawCases) == 0 && len(requestedAlertIDs) == 0 {
			return echo.NewHTTPError(http.StatusNotFound, "requested cases not found or not available for this agent")
		}
	} else if len(requestedAlertIDs) == 0 {
		searchParams := repository.ListSearchParams{}
		if len(agent.CaseTags) > 0 {
			searchParams.CaseMetaTagsAny = append([]string{}, agent.CaseTags...)
		}
		rawCases, err = h.cases.ListByTenantWithAssignedSortedAndSearch(
			c.Request().Context(),
			tenantID,
			max(maxCases*4, 120),
			0,
			"all",
			nil,
			"updated_at",
			"desc",
			searchParams,
		)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to list cases for ai agent")
		}
	}
	candidates := make([]models.Case, 0, len(rawCases)+len(requestedAlertIDs))
	seenCandidateCases := map[uuid.UUID]struct{}{}
	for _, item := range rawCases {
		if h.isCaseStatusClosed(c.Request().Context(), tenantID, item.Status) {
			continue
		}
		if _, exists := seenCandidateCases[item.ID]; exists {
			continue
		}
		seenCandidateCases[item.ID] = struct{}{}
		candidates = append(candidates, item)
		if len(candidates) >= maxCases {
			break
		}
	}
	if len(candidates) < maxCases && len(requestedAlertIDs) > 0 {
		requestedAlerts := h.loadAIAgentRequestedAlerts(c.Request().Context(), tenantID, agent, requestedAlertIDs)
		manualEvent := models.AIAgentQueueEvent{
			ID:         uuid.Nil,
			TenantID:   tenantID,
			ActorID:    &identity.UserID,
			EntityType: "alert",
			Source:     aiAgentQueueSourceAPI,
		}
		for _, alertItem := range requestedAlerts {
			if len(candidates) >= maxCases {
				break
			}
			manualEvent.EntityID = alertItem.ID
			resolvedCase, resolveErr := h.resolveCaseForAIAgentAlert(c.Request().Context(), manualEvent, agent, &alertItem, &identity.UserID)
			if resolveErr != nil || resolvedCase == nil {
				continue
			}
			if h.isCaseStatusClosed(c.Request().Context(), tenantID, resolvedCase.Status) {
				continue
			}
			if _, exists := seenCandidateCases[resolvedCase.ID]; exists {
				continue
			}
			seenCandidateCases[resolvedCase.ID] = struct{}{}
			candidates = append(candidates, *resolvedCase)
		}
		if len(candidates) == 0 {
			return echo.NewHTTPError(http.StatusNotFound, "requested alerts did not resolve to runnable cases for this agent")
		}
	}

	startedAt := time.Now().UTC()
	caseResults := make([]map[string]any, 0, len(candidates))
	processedCount := 0
	successCount := 0
	for _, caseItem := range candidates {
		actorID := identity.UserID
		result := h.runAIAgentForCase(c.Request().Context(), runtimeAI, tenantID, &actorID, agent, caseItem, req.DryRun, language)
		caseResults = append(caseResults, result)
		processedCount++
		if strings.TrimSpace(stringFromMap(result, "error")) == "" {
			successCount++
		}
	}
	finishedAt := time.Now().UTC()

	runData := map[string]any{
		"agent_id":            agent.ID.String(),
		"agent_name":          agent.Name,
		"agent_description":   agent.Description,
		"agent_tags":          append([]string{}, agent.CaseTags...),
		"started_at":          startedAt.Format(time.RFC3339),
		"finished_at":         finishedAt.Format(time.RFC3339),
		"dry_run":             req.DryRun,
		"requested_case_ids":  append([]string{}, req.CaseIDs...),
		"requested_alert_ids": append([]string{}, req.AlertIDs...),
		"matched_cases_total": len(candidates),
		"processed_cases":     processedCount,
		"successful_cases":    successCount,
		"results":             caseResults,
	}

	runID := ""
	if !req.DryRun {
		runItem, runErr := h.catalog.Create(c.Request().Context(), repository.CatalogCreateParams{
			TenantID:  &tenantID,
			Kind:      aiAgentRunsCatalogKind,
			OwnerID:   &identity.UserID,
			RefID:     &agent.ID,
			Data:      runData,
			CreatedBy: &identity.UserID,
		})
		if runErr != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to persist ai agent run")
		}
		runID = runItem.ID.String()
	}

	_ = h.audits.Log(c.Request().Context(), &tenantID, &identity.UserID, "ai_agent_run", "catalog_item", &agent.ID, map[string]any{
		"agent_id":         agent.ID.String(),
		"run_id":           runID,
		"dry_run":          req.DryRun,
		"processed_cases":  processedCount,
		"successful_cases": successCount,
	})

	payload := map[string]any{
		"run_id":                runID,
		"agent_id":              agent.ID.String(),
		"agent_name":            agent.Name,
		"dry_run":               req.DryRun,
		"started_at":            runData["started_at"],
		"finished_at":           runData["finished_at"],
		"matched_cases_total":   len(candidates),
		"processed_cases":       processedCount,
		"successful_cases":      successCount,
		"results":               caseResults,
		"requested_case_ids":    append([]string{}, req.CaseIDs...),
		"requested_alert_ids":   append([]string{}, req.AlertIDs...),
		"enrichment_connectors": append([]string{}, agent.EnrichmentConnectorIDs...),
	}
	return c.JSON(http.StatusOK, payload)
}

func parseUniqueUUIDList(rawIDs []string) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, 0, len(rawIDs))
	seen := make(map[uuid.UUID]struct{}, len(rawIDs))
	for _, rawID := range rawIDs {
		parsed, err := uuid.Parse(strings.TrimSpace(rawID))
		if err != nil {
			return nil, err
		}
		if _, ok := seen[parsed]; ok {
			continue
		}
		seen[parsed] = struct{}{}
		ids = append(ids, parsed)
	}
	return ids, nil
}

func (h *Handler) loadAIAgentRequestedCases(
	ctx context.Context,
	tenantID uuid.UUID,
	agent aiAgentDefinition,
	caseIDs []uuid.UUID,
) []models.Case {
	items := make([]models.Case, 0, len(caseIDs))
	for _, caseID := range caseIDs {
		caseItem, err := h.cases.GetByID(ctx, tenantID, caseID)
		if err != nil || caseItem == nil {
			continue
		}
		if len(agent.CaseTags) > 0 {
			caseTags := h.loadCaseMetaTags(ctx, tenantID, caseID)
			if !containsAnyCaseTag(caseTags, agent.CaseTags) {
				continue
			}
		}
		items = append(items, *caseItem)
	}
	return items
}

func (h *Handler) loadAIAgentRequestedAlerts(
	ctx context.Context,
	tenantID uuid.UUID,
	agent aiAgentDefinition,
	alertIDs []uuid.UUID,
) []models.Alert {
	items := make([]models.Alert, 0, len(alertIDs))
	for _, alertID := range alertIDs {
		alertItem, err := h.alerts.GetByID(ctx, tenantID, alertID)
		if err != nil || alertItem == nil {
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
		items = append(items, *alertItem)
	}
	return items
}

func containsAnyCaseTag(caseTags []string, expected []string) bool {
	if len(expected) == 0 {
		return true
	}
	normalized := make(map[string]struct{}, len(caseTags))
	for _, raw := range caseTags {
		tag := strings.ToLower(strings.TrimSpace(raw))
		if tag == "" {
			continue
		}
		normalized[tag] = struct{}{}
	}
	for _, raw := range expected {
		tag := strings.ToLower(strings.TrimSpace(raw))
		if tag == "" {
			continue
		}
		if _, ok := normalized[tag]; ok {
			return true
		}
	}
	return false
}

type aiAgentCaseContext struct {
	Tags        []string
	Tasks       []models.Task
	Observables []models.Observable
	Events      []models.CaseTimelineEvent
	Pages       []models.CasePage
	Comments    []string
}

func (h *Handler) loadAIAgentCaseContext(ctx context.Context, tenantID uuid.UUID, caseID uuid.UUID) aiAgentCaseContext {
	result := aiAgentCaseContext{
		Tags:        h.loadCaseMetaTags(ctx, tenantID, caseID),
		Tasks:       []models.Task{},
		Observables: []models.Observable{},
		Events:      []models.CaseTimelineEvent{},
		Pages:       []models.CasePage{},
		Comments:    h.loadCaseComments(ctx, tenantID, caseID),
	}
	if h.tasks != nil {
		if listed, err := h.tasks.ListByCase(ctx, tenantID, caseID, 200, 0); err == nil {
			result.Tasks = listed
		}
	}
	if h.observables != nil {
		if listed, err := h.observables.ListByCase(ctx, tenantID, caseID, 300, 0); err == nil {
			result.Observables = listed
		}
	}
	if h.caseEvents != nil {
		if listed, err := h.caseEvents.ListByCase(ctx, tenantID, caseID, 200, 0); err == nil {
			result.Events = listed
		}
	}
	if h.casePages != nil {
		if listed, err := h.casePages.ListByCase(ctx, tenantID, caseID, 80, 0); err == nil {
			result.Pages = listed
		}
	}
	return result
}

type aiAgentInvestigationResult struct {
	Comments          []string
	StageSummaries    []map[string]any
	Enrichment        []map[string]any
	CollectedTags     []string
	StageTimeline     []map[string]any
	ConnectorTimeline []map[string]any
}

func (h *Handler) runAIAgentInvestigationStages(
	ctx context.Context,
	tenantID uuid.UUID,
	agent aiAgentDefinition,
	caseItem models.Case,
	observables []models.Observable,
	baseComments []string,
) aiAgentInvestigationResult {
	comments := append([]string{}, baseComments...)
	stages := resolveAIAgentInvestigationStages(agent)

	stageSummaries := make([]map[string]any, 0, len(stages))
	enrichment := make([]map[string]any, 0)
	collectedTags := append([]string{}, agent.AutoCaseTags...)
	stageTimeline := make([]map[string]any, 0, len(stages)+1)
	connectorTimeline := make([]map[string]any, 0)

	for idx, stage := range stages {
		stageStartedAt := time.Now().UTC()
		stageID := strings.TrimSpace(stage.ID)
		if stageID == "" {
			stageID = fmt.Sprintf("stage-%d", idx+1)
		}
		stageName := strings.TrimSpace(stage.Name)
		if stageName == "" {
			stageName = fmt.Sprintf("Stage %d", idx+1)
		}
		if prompt := strings.TrimSpace(stage.Prompt); prompt != "" {
			comments = append(comments, "Stage "+stageName+" instruction: "+prompt)
		}
		connectorIDs := append([]string{}, stage.EnrichmentConnectorIDs...)
		if len(connectorIDs) == 0 {
			connectorIDs = append([]string{}, agent.EnrichmentConnectorIDs...)
		}
		stageEnrichment := h.runAIAgentEnrichment(ctx, tenantID, connectorIDs, agent, caseItem, observables, stage.Prompt)
		stageSuccessCount := 0
		stageErrorCount := 0
		for connectorIdx, item := range stageEnrichment {
			item["stage_id"] = stageID
			item["stage_name"] = stageName
			item["execution_index"] = connectorIdx
			reply := strings.TrimSpace(stringFromMap(item, "reply"))
			if reply != "" {
				comments = append(comments, "Connector enrichment ("+stageName+"): "+reply)
			}
			errorText := strings.TrimSpace(stringFromMap(item, "error"))
			statusText := strings.TrimSpace(stringFromMap(item, "status"))
			if errorText != "" {
				stageErrorCount++
				if statusText == "" {
					statusText = "failed"
				}
			} else {
				stageSuccessCount++
				if statusText == "" {
					statusText = "completed"
				}
			}
			enrichment = append(enrichment, item)
			connectorTimeline = append(connectorTimeline, map[string]any{
				"stage_id":        stageID,
				"stage_name":      stageName,
				"execution_index": connectorIdx,
				"connector_id":    strings.TrimSpace(stringFromMap(item, "connector_id", "connectorId")),
				"name":            strings.TrimSpace(stringFromMap(item, "name")),
				"channel":         strings.ToLower(strings.TrimSpace(stringFromMap(item, "channel"))),
				"status":          statusText,
				"execution_id":    strings.TrimSpace(stringFromMap(item, "execution_id", "executionId")),
				"reply":           truncateAgentText(reply, 1200),
				"error":           errorText,
				"conversation_id": strings.TrimSpace(stringFromMap(item, "conversation_id", "conversationId")),
				"cursor":          strings.TrimSpace(stringFromMap(item, "cursor")),
				"metadata":        normalizeMap(firstMapValue(item, "metadata")),
				"started_at":      strings.TrimSpace(stringFromMap(item, "started_at", "startedAt")),
				"finished_at":     strings.TrimSpace(stringFromMap(item, "finished_at", "finishedAt")),
				"duration_ms":     item["duration_ms"],
			})
		}
		if len(stage.CaseTags) > 0 {
			collectedTags = append(collectedTags, stage.CaseTags...)
		}
		stageCompletedAt := time.Now().UTC()
		stageStatus := "completed"
		if len(stageEnrichment) > 0 && stageErrorCount == len(stageEnrichment) {
			stageStatus = "failed"
		} else if stageErrorCount > 0 {
			stageStatus = "partial"
		}
		stageSummary := map[string]any{
			"id":               stageID,
			"name":             stageName,
			"description":      stage.Description,
			"prompt":           strings.TrimSpace(stage.Prompt),
			"enrichment_count": len(stageEnrichment),
			"case_tags":        normalizeTags(stage.CaseTags),
		}
		stageSummaries = append(stageSummaries, stageSummary)
		stageTimeline = append(stageTimeline, map[string]any{
			"id":                      stageID,
			"name":                    stageName,
			"description":             stage.Description,
			"prompt":                  strings.TrimSpace(stage.Prompt),
			"status":                  stageStatus,
			"execution_index":         idx,
			"connector_ids":           append([]string{}, connectorIDs...),
			"connector_count":         len(stageEnrichment),
			"connector_success_count": stageSuccessCount,
			"connector_error_count":   stageErrorCount,
			"case_tags":               normalizeTags(stage.CaseTags),
			"started_at":              stageStartedAt.Format(time.RFC3339),
			"finished_at":             stageCompletedAt.Format(time.RFC3339),
			"duration_ms":             stageCompletedAt.Sub(stageStartedAt).Milliseconds(),
		})
	}

	return aiAgentInvestigationResult{
		Comments:          comments,
		StageSummaries:    stageSummaries,
		Enrichment:        enrichment,
		CollectedTags:     collectedTags,
		StageTimeline:     stageTimeline,
		ConnectorTimeline: connectorTimeline,
	}
}

type aiAgentPostActionsResult struct {
	MitreAutoFilled bool
	MitreFillError  string
	CreatedTaskIDs  []string
	CommentCreated  bool
}

func (h *Handler) applyAIAgentPostAnalysisActions(
	ctx context.Context,
	tenantID uuid.UUID,
	actorID *uuid.UUID,
	agent aiAgentDefinition,
	caseItem models.Case,
	finalResult ai.CaseAnalysisResult,
	enrichment []map[string]any,
	triadMeta map[string]any,
	triadWarnings []string,
	autoActionsAllowed bool,
	actionBlockers []string,
	dryRun bool,
) aiAgentPostActionsResult {
	result := aiAgentPostActionsResult{CreatedTaskIDs: make([]string, 0)}
	if dryRun {
		return result
	}

	if filled, fillErr := h.autoFillCaseMetaMITREIfEmpty(ctx, tenantID, caseItem.ID, actorID, finalResult.MITER); fillErr != nil {
		result.MitreFillError = fillErr.Error()
	} else {
		result.MitreAutoFilled = filled
	}

	assigneeID, assigneeErr := parseOptionalUUID(agent.TaskAssigneeID)
	if assigneeErr != nil {
		assigneeID = nil
	}
	if h.tasks != nil && agent.AutoCreateTasks && autoActionsAllowed {
		recommendations := slices.Clone(finalResult.Recommendations)
		if len(recommendations) > 4 {
			recommendations = recommendations[:4]
		}
		for _, recommendation := range recommendations {
			title := aiAgentTaskTitle(recommendation)
			if title == "" {
				continue
			}
			dueAt := time.Now().UTC().Add(24 * time.Hour)
			task, taskErr := h.tasks.Create(ctx, repository.CreateTaskParams{
				CaseID:      caseItem.ID,
				TenantID:    tenantID,
				Title:       title,
				Description: "Generated by AI Agent " + agent.Name + ". Recommendation: " + recommendation,
				Status:      "open",
				AssigneeID:  assigneeID,
				DueDate:     &dueAt,
			})
			if taskErr == nil && task != nil {
				result.CreatedTaskIDs = append(result.CreatedTaskIDs, task.ID.String())
			}
		}
	}

	if h.catalog != nil && agent.AutoComment {
		commentContent := buildAIAgentComment(agent, finalResult, enrichment)
		if len(triadMeta) > 0 {
			commentContent = buildAIAgentTriadComment(commentContent, triadMeta, triadWarnings, autoActionsAllowed, actionBlockers)
		}
		if strings.TrimSpace(commentContent) != "" {
			actor := aiAgentSyntheticActor(agent)
			if _, commentErr := h.catalog.Create(ctx, repository.CatalogCreateParams{
				TenantID: &tenantID,
				Kind:     "case_comment",
				OwnerID:  nil,
				RefID:    &caseItem.ID,
				Data: map[string]any{
					"case_id":           caseItem.ID.String(),
					"author_id":         actor["author_id"],
					"authorName":        actor["author_name"],
					"author_kind":       actor["author_kind"],
					"author_avatar_key": actor["author_avatar_key"],
					"agent_id":          actor["agent_id"],
					"content":           commentContent,
					"created_at":        time.Now().UTC().Format(time.RFC3339),
					"tenant_id":         tenantID.String(),
				},
				CreatedBy: nil,
			}); commentErr == nil {
				result.CommentCreated = true
			}
		}
	}

	if h.caseAIAnalyses != nil {
		_, _ = h.caseAIAnalyses.Create(ctx, repository.CreateCaseAIAnalysisParams{
			TenantID:        tenantID,
			CaseID:          caseItem.ID,
			RequestedBy:     actorID,
			Model:           finalResult.Model,
			Status:          "completed",
			Verdict:         finalResult.Verdict,
			Confidence:      finalResult.Confidence,
			Summary:         finalResult.Summary,
			Recommendations: finalResult.Recommendations,
			Findings:        finalResult.Findings,
			Sources:         sourcesToGeneric(finalResult.Sources),
		})
	}

	return result
}

func (h *Handler) runAIAgentForCase(
	ctx context.Context,
	aiService *ai.Service,
	tenantID uuid.UUID,
	actorID *uuid.UUID,
	agent aiAgentDefinition,
	caseItem models.Case,
	dryRun bool,
	language string,
) map[string]any {
	caseCtx := h.loadAIAgentCaseContext(ctx, tenantID, caseItem.ID)
	investigation := h.runAIAgentInvestigationStages(ctx, tenantID, agent, caseItem, caseCtx.Observables, caseCtx.Comments)

	baseInput := ai.CaseAnalysisInput{
		CaseID:            caseItem.ID.String(),
		CaseNumber:        strings.TrimSpace(caseItem.CaseNumber),
		Title:             caseItem.Title,
		Description:       caseItem.Description,
		Source:            caseItem.Source,
		IncidentType:      caseItem.IncidentType,
		Status:            caseItem.Status,
		Priority:          caseItem.Priority,
		Impact:            caseItem.Impact,
		Severity:          caseItem.Severity,
		TLP:               caseItem.TLP,
		PAP:               caseItem.PAP,
		Confidence:        caseItem.Confidence,
		ResolutionSummary: caseItem.ResolutionSummary,
		Tasks:             mapTasksForAnalysis(caseCtx.Tasks),
		Observables:       mapObservablesForAnalysis(caseCtx.Observables),
		Events:            mapEventsForAnalysis(caseCtx.Events),
		Pages:             mapPagesForAnalysis(caseCtx.Pages),
		Comments:          append([]string{}, investigation.Comments...),
	}
	if strings.TrimSpace(agent.Prompt) != "" {
		baseInput.Comments = append(baseInput.Comments, "Agent instruction: "+strings.TrimSpace(agent.Prompt))
	}

	analysisStartedAt := time.Now().UTC()
	finalResult, triadMeta, triadWarnings, err := h.resolveAIAgentCaseAnalysis(ctx, aiService, tenantID, caseItem, agent, baseInput, language)
	analysisCompletedAt := time.Now().UTC()

	stageTimeline := append([]map[string]any{}, investigation.StageTimeline...)
	analysisStageName := "Analysis"
	analysisStageDescription := "LLM verdict synthesis for the case."
	if triadEnabledForCase(agent, caseItem) {
		analysisStageName = "Triad Analysis"
		analysisStageDescription = "Investigator, reviewer, and arbiter passes for the case."
	}
	analysisStageStatus := "completed"
	analysisError := ""
	if err != nil {
		analysisStageStatus = "failed"
		analysisError = err.Error()
	}
	stageTimeline = append(stageTimeline, map[string]any{
		"id":                      "analysis",
		"name":                    analysisStageName,
		"description":             analysisStageDescription,
		"prompt":                  strings.TrimSpace(agent.Prompt),
		"status":                  analysisStageStatus,
		"execution_index":         len(stageTimeline),
		"connector_ids":           []string{},
		"connector_count":         0,
		"connector_success_count": 0,
		"connector_error_count":   0,
		"case_tags":               []string{},
		"started_at":              analysisStartedAt.Format(time.RFC3339),
		"finished_at":             analysisCompletedAt.Format(time.RFC3339),
		"duration_ms":             analysisCompletedAt.Sub(analysisStartedAt).Milliseconds(),
		"verdict":                 strings.TrimSpace(finalResult.Verdict),
		"confidence":              finalResult.Confidence,
		"error":                   strings.TrimSpace(analysisError),
	})

	appliedTags := normalizeTags(append(caseCtx.Tags, investigation.CollectedTags...))
	if !dryRun && len(appliedTags) > 0 {
		if updated, updateErr := h.upsertCaseMetaTags(ctx, tenantID, caseItem.ID, actorID, appliedTags); updateErr == nil {
			appliedTags = updated
		}
	}

	if err != nil {
		response := map[string]any{
			"case_id":            caseItem.ID.String(),
			"case_number":        strings.TrimSpace(caseItem.CaseNumber),
			"title":              caseItem.Title,
			"status":             strings.ToLower(strings.TrimSpace(caseItem.Status)),
			"tags":               appliedTags,
			"plan_stages":        investigation.StageSummaries,
			"stage_timeline":     stageTimeline,
			"enrichment":         investigation.Enrichment,
			"connector_timeline": investigation.ConnectorTimeline,
			"applied_tags":       normalizeTags(investigation.CollectedTags),
			"miter":              finalResult.MITER,
			"miter_auto_filled":  false,
			"error":              err.Error(),
			"completed_at":       time.Now().UTC().Format(time.RFC3339),
		}
		if len(triadMeta) > 0 {
			response["triad"] = triadMeta
		}
		if len(triadWarnings) > 0 {
			response["triad_warnings"] = triadWarnings
		}
		return response
	}

	requiresHumanReview := false
	if parsed, ok := boolFromMap(triadMeta, "requires_human_review", "requiresHumanReview"); ok {
		requiresHumanReview = parsed
	}
	reviewerConsensus := true
	if parsed, ok := boolFromMap(triadMeta, "reviewer_consensus", "reviewerConsensus"); ok {
		reviewerConsensus = parsed
	}
	autoActionMinConfidence := agent.AutoActionMinConfidence
	if autoActionMinConfidence < 0 {
		autoActionMinConfidence = 0
	}
	if autoActionMinConfidence > 100 {
		autoActionMinConfidence = 100
	}
	if autoActionMinConfidence == 0 {
		autoActionMinConfidence = 85
	}
	autoActionsAllowed := true
	actionBlockers := make([]string, 0, 4)
	if triadEnabledForCase(agent, caseItem) && agent.RequireReviewerConsensus && !reviewerConsensus {
		autoActionsAllowed = false
		actionBlockers = append(actionBlockers, "reviewer consensus is required")
	}
	if finalResult.Confidence < autoActionMinConfidence {
		autoActionsAllowed = false
		actionBlockers = append(actionBlockers, fmt.Sprintf("confidence %.1f is below threshold %.1f", finalResult.Confidence, autoActionMinConfidence))
	}
	if requiresHumanReview {
		autoActionsAllowed = false
		actionBlockers = append(actionBlockers, "human review required")
	}
	if len(triadWarnings) > 0 {
		autoActionsAllowed = false
		for _, warning := range triadWarnings {
			if strings.TrimSpace(warning) != "" {
				actionBlockers = append(actionBlockers, warning)
			}
		}
	}
	actionBlockers = normalizeTags(actionBlockers)

	postActions := h.applyAIAgentPostAnalysisActions(
		ctx,
		tenantID,
		actorID,
		agent,
		caseItem,
		finalResult,
		investigation.Enrichment,
		triadMeta,
		triadWarnings,
		autoActionsAllowed,
		actionBlockers,
		dryRun,
	)

	response := map[string]any{
		"case_id":                    caseItem.ID.String(),
		"case_number":                strings.TrimSpace(caseItem.CaseNumber),
		"title":                      caseItem.Title,
		"status":                     strings.ToLower(strings.TrimSpace(caseItem.Status)),
		"tags":                       appliedTags,
		"verdict":                    finalResult.Verdict,
		"confidence":                 finalResult.Confidence,
		"summary":                    truncateAgentText(finalResult.Summary, 1200),
		"recommendations":            finalResult.Recommendations,
		"findings":                   finalResult.Findings,
		"miter":                      finalResult.MITER,
		"miter_auto_filled":          postActions.MitreAutoFilled,
		"sources":                    finalResult.Sources,
		"plan_stages":                investigation.StageSummaries,
		"stage_timeline":             stageTimeline,
		"enrichment":                 investigation.Enrichment,
		"connector_timeline":         investigation.ConnectorTimeline,
		"applied_tags":               normalizeTags(investigation.CollectedTags),
		"created_task_ids":           postActions.CreatedTaskIDs,
		"agent_comment_created":      postActions.CommentCreated,
		"auto_actions_allowed":       autoActionsAllowed,
		"requires_human_review":      requiresHumanReview,
		"reviewer_consensus":         reviewerConsensus,
		"auto_action_min_confidence": autoActionMinConfidence,
		"action_blockers":            actionBlockers,
		"completed_at":               time.Now().UTC().Format(time.RFC3339),
	}
	if strings.TrimSpace(postActions.MitreFillError) != "" {
		response["miter_fill_error"] = postActions.MitreFillError
	}
	if len(triadMeta) > 0 {
		response["triad"] = triadMeta
	}
	if len(triadWarnings) > 0 {
		response["triad_warnings"] = triadWarnings
	}
	return response
}

func (h *Handler) resolveAIAgentCaseAnalysis(
	ctx context.Context,
	aiService *ai.Service,
	tenantID uuid.UUID,
	caseItem models.Case,
	agent aiAgentDefinition,
	baseInput ai.CaseAnalysisInput,
	language string,
) (analysis ai.CaseAnalysisResult, meta map[string]any, warningsOut []string, err error) {
	if aiService == nil {
		return ai.CaseAnalysisResult{}, nil, nil, fmt.Errorf("ai service is not configured")
	}
	if !triadEnabledForCase(agent, caseItem) {
		result, stageErr := h.analyzeAIAgentCaseStage(ctx, aiService, tenantID, cloneAIAgentCaseInput(baseInput), language)
		return result, nil, nil, stageErr
	}

	warnings := make([]string, 0, 4)
	criticalCase := isCriticalCaseForTriad(caseItem)
	triadMeta := map[string]any{
		"enabled":       true,
		"critical_case": criticalCase,
		"critical_only": agent.TriadCriticalOnly,
		"workflow":      "investigator_reviewer_arbiter",
	}

	investigatorPrompt := firstNonEmptyString(
		strings.TrimSpace(agent.InvestigatorPrompt),
		"Perform initial investigation and identify highest risk hypotheses.",
	)
	reviewerPrompt := firstNonEmptyString(
		strings.TrimSpace(agent.ReviewerPrompt),
		"Independently review the investigator output and challenge weak assumptions.",
	)
	arbiterPrompt := firstNonEmptyString(
		strings.TrimSpace(agent.ArbiterPrompt),
		"Produce final verdict using investigator and reviewer evidence with explicit risk rationale.",
	)

	investigatorInput := cloneAIAgentCaseInput(baseInput)
	investigatorInput.Comments = appendTriadRoleComment(investigatorInput.Comments, "investigator", investigatorPrompt)
	investigatorResult, err := h.analyzeAIAgentCaseStage(ctx, aiService, tenantID, investigatorInput, language)
	if err != nil {
		return ai.CaseAnalysisResult{}, triadMeta, nil, err
	}
	triadMeta["investigator"] = triadStageMeta("investigator", investigatorResult)

	reviewerInput := cloneAIAgentCaseInput(baseInput)
	reviewerInput.Comments = appendTriadRoleComment(reviewerInput.Comments, "reviewer", reviewerPrompt)
	reviewerInput.Comments = append(reviewerInput.Comments, triadResultContext("investigator", investigatorResult))
	reviewerResult, reviewerErr := h.analyzeAIAgentCaseStage(ctx, aiService, tenantID, reviewerInput, language)

	reviewerConsensus := false
	requiresHumanReview := false
	finalResult := investigatorResult

	if reviewerErr != nil {
		requiresHumanReview = true
		warnings = append(warnings, "reviewer stage failed: "+reviewerErr.Error())
		triadMeta["reviewer"] = map[string]any{
			"role":  "reviewer",
			"error": reviewerErr.Error(),
		}
		triadMeta["arbiter"] = map[string]any{
			"role":    "arbiter",
			"skipped": "reviewer stage failed",
		}
	} else {
		triadMeta["reviewer"] = triadStageMeta("reviewer", reviewerResult)
		reviewerConsensus = triadVerdictsMatch(investigatorResult.Verdict, reviewerResult.Verdict)
		if !reviewerConsensus {
			requiresHumanReview = true
			warnings = append(warnings, "reviewer verdict does not match investigator verdict")
		}

		arbiterInput := cloneAIAgentCaseInput(baseInput)
		arbiterInput.Comments = appendTriadRoleComment(arbiterInput.Comments, "arbiter", arbiterPrompt)
		arbiterInput.Comments = append(arbiterInput.Comments,
			triadResultContext("investigator", investigatorResult),
			triadResultContext("reviewer", reviewerResult),
		)
		arbiterResult, arbiterErr := h.analyzeAIAgentCaseStage(ctx, aiService, tenantID, arbiterInput, language)
		if arbiterErr != nil {
			requiresHumanReview = true
			warnings = append(warnings, "arbiter stage failed: "+arbiterErr.Error())
			triadMeta["arbiter"] = map[string]any{
				"role":  "arbiter",
				"error": arbiterErr.Error(),
			}
			finalResult = reviewerResult
		} else {
			triadMeta["arbiter"] = triadStageMeta("arbiter", arbiterResult)
			finalResult = arbiterResult
			if !triadVerdictsMatch(arbiterResult.Verdict, investigatorResult.Verdict) && !triadVerdictsMatch(arbiterResult.Verdict, reviewerResult.Verdict) {
				requiresHumanReview = true
				warnings = append(warnings, "arbiter verdict differs from investigator and reviewer verdicts")
			}
		}
	}

	warnings = dedupeNonEmptyStrings(warnings)
	if len(warnings) > 0 {
		requiresHumanReview = true
	}
	triadMeta["reviewer_consensus"] = reviewerConsensus
	triadMeta["requires_human_review"] = requiresHumanReview
	if len(warnings) > 0 {
		triadMeta["pipeline_status"] = "degraded"
	} else {
		triadMeta["pipeline_status"] = "completed"
	}
	return finalResult, triadMeta, warnings, nil
}

func (h *Handler) analyzeAIAgentCaseStage(
	ctx context.Context,
	aiService *ai.Service,
	tenantID uuid.UUID,
	input ai.CaseAnalysisInput,
	language string,
) (ai.CaseAnalysisResult, error) {
	aiCtx, cancel := h.aiRequestContext(ctx)
	defer cancel()
	return aiService.AnalyzeCaseWithLanguage(aiCtx, tenantID.String(), input, language)
}

func cloneAIAgentCaseInput(input ai.CaseAnalysisInput) ai.CaseAnalysisInput {
	cloned := input
	cloned.Comments = append([]string{}, input.Comments...)
	return cloned
}

func appendTriadRoleComment(comments []string, role, prompt string) []string {
	out := append([]string{}, comments...)
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "" {
		role = "analyst"
	}
	line := "Triad role: " + role + "."
	if trimmedPrompt := strings.TrimSpace(prompt); trimmedPrompt != "" {
		line = line + " " + trimmedPrompt
	}
	return append(out, truncateAgentText(line, 700))
}

func triadVerdictsMatch(left, right string) bool {
	return strings.EqualFold(strings.TrimSpace(left), strings.TrimSpace(right))
}

func triadStageMeta(role string, result ai.CaseAnalysisResult) map[string]any {
	return map[string]any{
		"role":                  strings.TrimSpace(role),
		"model":                 strings.TrimSpace(result.Model),
		"verdict":               strings.TrimSpace(result.Verdict),
		"confidence":            result.Confidence,
		"summary":               truncateAgentText(result.Summary, 700),
		"recommendations_count": len(result.Recommendations),
		"findings_count":        len(result.Findings),
	}
}

func triadResultContext(role string, result ai.CaseAnalysisResult) string {
	roleName := strings.TrimSpace(role)
	if roleName == "" {
		roleName = "analyst"
	}
	builder := strings.Builder{}
	builder.WriteString("Triad ")
	builder.WriteString(roleName)
	builder.WriteString(" result: verdict=")
	builder.WriteString(strings.TrimSpace(result.Verdict))
	builder.WriteString("; confidence=")
	_, _ = fmt.Fprintf(&builder, "%.1f", result.Confidence)
	if summary := strings.TrimSpace(result.Summary); summary != "" {
		builder.WriteString("; summary=")
		builder.WriteString(truncateAgentText(summary, 450))
	}
	if len(result.Findings) > 0 {
		builder.WriteString("; findings=")
		builder.WriteString(strings.Join(trimAIAgentStrings(result.Findings, 3), " | "))
	}
	return truncateAgentText(builder.String(), 1100)
}

func isCriticalCaseForTriad(caseItem models.Case) bool {
	for _, raw := range []string{caseItem.Severity, caseItem.Priority} {
		normalized := strings.ToLower(strings.TrimSpace(raw))
		switch normalized {
		case "critical", "sev0", "sev1", "p0", "p1", "urgent", "highest", "blocker", "emergency":
			return true
		}
	}
	return false
}

func triadEnabledForCase(agent aiAgentDefinition, caseItem models.Case) bool {
	if !agent.TriadEnabled {
		return false
	}
	if !agent.TriadCriticalOnly {
		return true
	}
	return isCriticalCaseForTriad(caseItem)
}

func dedupeNonEmptyStrings(input []string) []string {
	if len(input) == 0 {
		return nil
	}
	out := make([]string, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for _, raw := range input {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func trimAIAgentStrings(input []string, limit int) []string {
	if limit <= 0 || len(input) == 0 {
		return nil
	}
	out := make([]string, 0, min(limit, len(input)))
	for _, raw := range input {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		out = append(out, value)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func (h *Handler) runAIAgentEnrichment(
	ctx context.Context,
	tenantID uuid.UUID,
	connectorIDs []string,
	agent aiAgentDefinition,
	caseItem models.Case,
	observables []models.Observable,
	stagePrompt string,
) []map[string]any {
	if len(connectorIDs) == 0 {
		return []map[string]any{}
	}
	results := make([]map[string]any, 0, len(connectorIDs))
	for _, rawID := range connectorIDs {
		startedAt := time.Now().UTC()
		connectorID, parseErr := uuid.Parse(strings.TrimSpace(rawID))
		if parseErr != nil {
			finishedAt := time.Now().UTC()
			results = append(results, map[string]any{
				"connector_id": rawID,
				"status":       "failed",
				"error":        "invalid connector id",
				"started_at":   startedAt.Format(time.RFC3339),
				"finished_at":  finishedAt.Format(time.RFC3339),
				"duration_ms":  finishedAt.Sub(startedAt).Milliseconds(),
			})
			continue
		}
		actor := aiAgentSyntheticActor(agent)
		dispatch, sendErr := h.executeConnectorHubDispatch(ctx, connectorHubDispatchInput{
			TenantID:    tenantID,
			ActorID:     nil,
			ConnectorID: connectorID,
			Action:      "ai_enrichment",
			CaseID:      &caseItem.ID,
			Message:     buildAIAgentEnrichmentPrompt(caseItem, observables, stagePrompt),
			Input: map[string]any{
				"case_id":       caseItem.ID.String(),
				"case_number":   strings.TrimSpace(caseItem.CaseNumber),
				"stage_prompt":  strings.TrimSpace(stagePrompt),
				"agent_id":      agent.ID.String(),
				"agent_name":    strings.TrimSpace(agent.Name),
				"observables":   summarizeAIAgentEnrichmentObservables(observables, 30),
				"case_severity": strings.TrimSpace(caseItem.Severity),
				"case_status":   strings.TrimSpace(caseItem.Status),
			},
			Metadata: map[string]any{
				"agent_id":   agent.ID.String(),
				"agent_name": agent.Name,
				"case_id":    caseItem.ID.String(),
				"stage_hint": strings.TrimSpace(stagePrompt),
				"actor":      actor,
			},
			Author:        aiAgentDisplayName(agent.Name),
			Conversation:  "ai-agent-" + agent.ID.String(),
			DryRun:        false,
			ExecutionMode: "ai_agent",
		})
		finishedAt := time.Now().UTC()

		responseMap := normalizeMap(firstMapValue(dispatch, "response"))
		reply := stringFromMap(responseMap, "reply")
		if strings.TrimSpace(reply) == "" {
			reply = stringFromMap(dispatch, "reply")
		}
		conversationID := stringFromMap(responseMap, "conversation_id", "conversationId")
		if strings.TrimSpace(conversationID) == "" {
			conversationID = stringFromMap(dispatch, "conversation_id", "conversationId")
		}
		cursor := stringFromMap(responseMap, "cursor")
		if strings.TrimSpace(cursor) == "" {
			cursor = stringFromMap(dispatch, "cursor")
		}
		replyMetadata := normalizeMap(firstMapValue(responseMap, "metadata"))
		if len(replyMetadata) == 0 {
			replyMetadata = normalizeMap(firstMapValue(dispatch, "metadata"))
		}
		baseResult := map[string]any{
			"connector_id":    connectorID.String(),
			"name":            stringFromMap(dispatch, "connector_name", "name"),
			"channel":         strings.ToLower(strings.TrimSpace(stringFromMap(dispatch, "connector_channel", "channel"))),
			"execution_id":    strings.TrimSpace(stringFromMap(dispatch, "id")),
			"conversation_id": strings.TrimSpace(conversationID),
			"cursor":          strings.TrimSpace(cursor),
			"metadata":        replyMetadata,
			"started_at":      startedAt.Format(time.RFC3339),
			"finished_at":     finishedAt.Format(time.RFC3339),
			"duration_ms":     finishedAt.Sub(startedAt).Milliseconds(),
		}
		if sendErr != nil {
			baseResult["status"] = "failed"
			baseResult["error"] = sendErr.Error()
			results = append(results, baseResult)
			continue
		}
		baseResult["status"] = "completed"
		baseResult["reply"] = truncateAgentText(reply, 2000)
		results = append(results, baseResult)
	}
	return results
}

func summarizeAIAgentEnrichmentObservables(observables []models.Observable, limit int) []map[string]any {
	if len(observables) == 0 || limit <= 0 {
		return []map[string]any{}
	}
	out := make([]map[string]any, 0, min(limit, len(observables)))
	for _, observable := range observables {
		if len(out) >= limit {
			break
		}
		out = append(out, map[string]any{
			"id":      observable.ID.String(),
			"type":    strings.TrimSpace(observable.Type),
			"value":   strings.TrimSpace(observable.Value),
			"verdict": strings.TrimSpace(observable.Verdict),
			"source":  strings.TrimSpace(observable.Source),
		})
	}
	return out
}

func (h *Handler) loadCaseComments(ctx context.Context, tenantID, caseID uuid.UUID) []string {
	if h.catalog == nil {
		return []string{}
	}
	items, err := h.catalog.List(ctx, repository.CatalogListParams{
		Kind:     "case_comment",
		TenantID: &tenantID,
		RefID:    &caseID,
		Limit:    200,
	})
	if err != nil {
		return []string{}
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		content := strings.TrimSpace(stringFromMap(item.Data, "content"))
		if content == "" {
			continue
		}
		out = append(out, content)
	}
	return out
}

func (h *Handler) loadCaseMetaTags(ctx context.Context, tenantID, caseID uuid.UUID) []string {
	if h.catalog == nil {
		return []string{}
	}
	items, err := h.catalog.List(ctx, repository.CatalogListParams{
		Kind:     "case_meta",
		TenantID: &tenantID,
		RefID:    &caseID,
		Limit:    1,
	})
	if err != nil || len(items) == 0 {
		return []string{}
	}
	return normalizeTags(stringSliceFromMap(items[0].Data, "tags", "case_tags", "caseTags"))
}

func (h *Handler) upsertCaseMetaTags(ctx context.Context, tenantID, caseID uuid.UUID, actorID *uuid.UUID, tags []string) ([]string, error) {
	if h.catalog == nil {
		return normalizeTags(tags), nil
	}
	normalizedTags := normalizeTags(tags)
	if len(normalizedTags) == 0 {
		return []string{}, nil
	}
	items, err := h.catalog.List(ctx, repository.CatalogListParams{
		Kind:     "case_meta",
		TenantID: &tenantID,
		RefID:    &caseID,
		Limit:    1,
	})
	if err != nil {
		return normalizedTags, err
	}
	data := map[string]any{
		"case_id":   caseID.String(),
		"tenant_id": tenantID.String(),
		"tags":      normalizedTags,
		"case_tags": normalizedTags,
	}
	if len(items) > 0 {
		_, err = h.catalog.Update(ctx, "case_meta", items[0].ID, &tenantID, repository.CatalogUpdateParams{
			Data: data,
		})
		return normalizedTags, err
	}
	ownerID := actorID
	createdBy := actorID
	if ownerID != nil && *ownerID == uuid.Nil {
		ownerID = nil
	}
	if createdBy != nil && *createdBy == uuid.Nil {
		createdBy = nil
	}
	_, err = h.catalog.Create(ctx, repository.CatalogCreateParams{
		TenantID:  &tenantID,
		Kind:      "case_meta",
		OwnerID:   ownerID,
		RefID:     &caseID,
		Data:      data,
		CreatedBy: createdBy,
	})
	return normalizedTags, err
}

func (h *Handler) autoFillCaseMetaMITREIfEmpty(ctx context.Context, tenantID, caseID uuid.UUID, actorID *uuid.UUID, miter ai.MITREMapping) (bool, error) {
	if h.catalog == nil {
		return false, nil
	}
	if len(miter.Tactics) == 0 && len(miter.Techniques) == 0 {
		return false, nil
	}
	items, err := h.catalog.List(ctx, repository.CatalogListParams{
		Kind:     "case_meta",
		TenantID: &tenantID,
		RefID:    &caseID,
		Limit:    1,
	})
	if err != nil {
		return false, err
	}
	if len(items) > 0 {
		existingTactics := normalizeCaseMetaMITREValues(stringSliceFromMap(items[0].Data, "tactics"))
		existingTechniques := normalizeCaseMetaMITREValues(stringSliceFromMap(items[0].Data, "techniques"))
		if len(existingTactics) > 0 || len(existingTechniques) > 0 {
			return false, nil
		}
		_, err = h.catalog.Update(ctx, "case_meta", items[0].ID, &tenantID, repository.CatalogUpdateParams{
			Data: map[string]any{
				"case_id":    caseID.String(),
				"tenant_id":  tenantID.String(),
				"tactics":    append([]string{}, miter.Tactics...),
				"techniques": append([]string{}, miter.Techniques...),
			},
		})
		return err == nil, err
	}
	ownerID := actorID
	createdBy := actorID
	if ownerID != nil && *ownerID == uuid.Nil {
		ownerID = nil
	}
	if createdBy != nil && *createdBy == uuid.Nil {
		createdBy = nil
	}
	_, err = h.catalog.Create(ctx, repository.CatalogCreateParams{
		TenantID: &tenantID,
		Kind:     "case_meta",
		OwnerID:  ownerID,
		RefID:    &caseID,
		Data: map[string]any{
			"case_id":    caseID.String(),
			"tenant_id":  tenantID.String(),
			"tactics":    append([]string{}, miter.Tactics...),
			"techniques": append([]string{}, miter.Techniques...),
		},
		CreatedBy: createdBy,
	})
	return err == nil, err
}

func normalizeCaseMetaMITREValues(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.ToUpper(strings.TrimSpace(value))
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func resolveAIAgentInvestigationStages(agent aiAgentDefinition) []aiAgentInvestigationStage {
	if len(agent.InvestigationPlan) > 0 {
		out := make([]aiAgentInvestigationStage, 0, len(agent.InvestigationPlan))
		for _, stage := range agent.InvestigationPlan {
			name := strings.TrimSpace(stage.Name)
			prompt := strings.TrimSpace(stage.Prompt)
			if name == "" && prompt == "" && len(stage.EnrichmentConnectorIDs) == 0 && len(stage.CaseTags) == 0 {
				continue
			}
			out = append(out, aiAgentInvestigationStage{
				ID:                     strings.TrimSpace(stage.ID),
				Name:                   name,
				Description:            strings.TrimSpace(stage.Description),
				Prompt:                 prompt,
				EnrichmentConnectorIDs: normalizeAIAgentConnectorIDs(stage.EnrichmentConnectorIDs),
				CaseTags:               normalizeTags(stage.CaseTags),
			})
		}
		if len(out) > 0 {
			return out
		}
	}
	if strings.TrimSpace(agent.Prompt) == "" && len(agent.EnrichmentConnectorIDs) == 0 {
		return nil
	}
	return []aiAgentInvestigationStage{
		{
			ID:                     "default",
			Name:                   "Default",
			Prompt:                 strings.TrimSpace(agent.Prompt),
			EnrichmentConnectorIDs: append([]string{}, agent.EnrichmentConnectorIDs...),
		},
	}
}

func parseAIAgentInvestigationPlan(data map[string]any) []aiAgentInvestigationStage {
	if len(data) == 0 {
		return nil
	}
	rawPlan := firstMapValue(data, "investigation_plan", "investigationPlan", "plan", "stages")
	if rawPlan == nil {
		return nil
	}
	stages := make([]aiAgentInvestigationStage, 0)
	items, ok := rawPlan.([]any)
	if !ok {
		if parsed := normalizeMap(rawPlan); len(parsed) > 0 {
			items = []any{parsed}
		}
	}
	for idx, raw := range items {
		stageMap := normalizeMap(raw)
		if len(stageMap) == 0 {
			continue
		}
		stage := aiAgentInvestigationStage{
			ID:                     strings.TrimSpace(stringFromMap(stageMap, "id", "key", "stage_id", "stageId")),
			Name:                   strings.TrimSpace(stringFromMap(stageMap, "name", "title")),
			Description:            strings.TrimSpace(stringFromMap(stageMap, "description", "about")),
			Prompt:                 strings.TrimSpace(stringFromMap(stageMap, "prompt", "instruction", "instructions", "system_prompt", "systemPrompt")),
			EnrichmentConnectorIDs: normalizeAIAgentConnectorIDs(stringSliceFromMap(stageMap, "enrichment_connector_ids", "enrichmentConnectorIds", "connector_ids", "connectorIds")),
			CaseTags:               normalizeTags(stringSliceFromMap(stageMap, "case_tags", "caseTags", "add_tags", "addTags")),
		}
		if stage.ID == "" {
			stage.ID = fmt.Sprintf("stage-%d", idx+1)
		}
		if stage.Name == "" {
			stage.Name = fmt.Sprintf("Stage %d", idx+1)
		}
		if stage.Prompt == "" && len(stage.EnrichmentConnectorIDs) == 0 && len(stage.CaseTags) == 0 {
			continue
		}
		stages = append(stages, stage)
	}
	if len(stages) == 0 {
		return nil
	}
	return stages
}

func normalizeAIAgentExecutionPolicy(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "exclusive":
		return string(models.AIAgentExecutionPolicyExclusive)
	case "first_match", "firstmatch", "first":
		return string(models.AIAgentExecutionPolicyFirstMatch)
	case "fallback_chain", "fallbackchain", "fallback":
		return string(models.AIAgentExecutionPolicyFallbackChain)
	default:
		return string(models.AIAgentExecutionPolicyAllMatching)
	}
}

func normalizeAIAgentExecutionPriority(raw int) int {
	if raw > 1000 {
		return 1000
	}
	if raw < -1000 {
		return -1000
	}
	return raw
}

func normalizeAIAgentTargetTypes(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		normalized := strings.ToLower(strings.TrimSpace(raw))
		switch normalized {
		case "case", "cases":
			normalized = "case"
		case "alert", "alerts":
			normalized = "alert"
		default:
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func firstMapValue(payload map[string]any, keys ...string) any {
	for _, key := range keys {
		value, ok := payload[key]
		if ok {
			return value
		}
	}
	return nil
}

func normalizeMap(input any) map[string]any {
	if input == nil {
		return nil
	}
	if typed, ok := input.(map[string]any); ok {
		out := make(map[string]any, len(typed))
		for key, value := range typed {
			out[key] = value
		}
		return out
	}
	raw, err := json.Marshal(input)
	if err != nil || len(raw) == 0 {
		return nil
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}

func parseAIAgentDefinition(item models.CatalogItem) aiAgentDefinition {
	data := item.Data
	if data == nil {
		data = map[string]any{}
	}
	name := strings.TrimSpace(stringFromMap(data, "name", "title"))
	if name == "" {
		name = "AI Agent"
	}
	description := strings.TrimSpace(stringFromMap(data, "description", "about"))
	prompt := strings.TrimSpace(stringFromMap(data, "prompt", "instructions", "system_prompt", "systemPrompt"))
	model := strings.TrimSpace(stringFromMap(data, "model"))
	provider := strings.TrimSpace(stringFromMap(data, "provider", "ai_provider", "aiProvider"))
	endpoint := strings.TrimSpace(stringFromMap(data, "endpoint", "base_url", "baseURL", "api_base", "apiBase"))
	language := normalizeAILanguage(stringFromMap(data, "language"))
	enabled := true
	if parsed, ok := boolFromMap(data, "enabled", "is_enabled", "isEnabled"); ok {
		enabled = parsed
	}
	autoCreateTasks := true
	if parsed, ok := boolFromMap(data, "auto_create_tasks", "autoCreateTasks"); ok {
		autoCreateTasks = parsed
	}
	autoComment := true
	if parsed, ok := boolFromMap(data, "auto_comment", "autoComment"); ok {
		autoComment = parsed
	}
	maxCasesPerRun := 10
	if parsed, ok := intFromMap(data, "max_cases_per_run", "maxCasesPerRun", "max_cases", "maxCases"); ok {
		maxCasesPerRun = parsed
	}
	if maxCasesPerRun <= 0 {
		maxCasesPerRun = 10
	}
	if maxCasesPerRun > 100 {
		maxCasesPerRun = 100
	}
	caseTags := normalizeTags(stringSliceFromMap(data, "case_tags", "caseTags", "tags"))
	targetTypes := normalizeAIAgentTargetTypes(stringSliceFromMap(data, "target_types", "targetTypes", "targets"))
	if len(targetTypes) == 0 {
		targetTypes = []string{"case"}
	}
	alertSources := normalizeTags(stringSliceFromMap(data, "alert_sources", "alertSources", "sources"))
	autoCaseTags := normalizeTags(stringSliceFromMap(data, "auto_case_tags", "autoCaseTags"))
	investigationPlan := parseAIAgentInvestigationPlan(data)
	connectorIDs := normalizeAIAgentConnectorIDs(stringSliceFromMap(data, "enrichment_connector_ids", "enrichmentConnectorIds", "connector_ids", "connectorIds"))
	notificationConnectorIDs := normalizeAIAgentConnectorIDs(stringSliceFromMap(data, "notification_connector_ids", "notificationConnectorIds", "notify_connector_ids", "notifyConnectorIds"))
	autoCloseCase := false
	if parsed, ok := boolFromMap(data, "auto_close_case", "autoCloseCase", "auto_close"); ok {
		autoCloseCase = parsed
	}
	autoCloseVerdicts := normalizeTags(stringSliceFromMap(data, "auto_close_verdicts", "autoCloseVerdicts"))
	autoCreateCaseFromAlert := false
	if parsed, ok := boolFromMap(data, "auto_create_case_from_alert", "autoCreateCaseFromAlert"); ok {
		autoCreateCaseFromAlert = parsed
	}
	taskAssigneeID := strings.TrimSpace(stringFromMap(data, "task_assignee_id", "taskAssigneeId", "assignee_user_id", "assigneeUserId"))
	triadEnabled := false
	if parsed, ok := boolFromMap(data, "triad_enabled", "triadEnabled", "three_agent_mode", "threeAgentMode"); ok {
		triadEnabled = parsed
	}
	triadCriticalOnly := true
	if parsed, ok := boolFromMap(data, "triad_critical_only", "triadCriticalOnly", "critical_only_triad", "criticalOnlyTriad"); ok {
		triadCriticalOnly = parsed
	}
	investigatorPrompt := strings.TrimSpace(stringFromMap(data, "investigator_prompt", "investigatorPrompt"))
	reviewerPrompt := strings.TrimSpace(stringFromMap(data, "reviewer_prompt", "reviewerPrompt"))
	arbiterPrompt := strings.TrimSpace(stringFromMap(data, "arbiter_prompt", "arbiterPrompt"))
	requireReviewerConsensus := true
	if parsed, ok := boolFromMap(data, "require_reviewer_consensus", "requireReviewerConsensus", "reviewer_consensus_required", "reviewerConsensusRequired"); ok {
		requireReviewerConsensus = parsed
	}
	autoActionMinConfidence := 85.0
	if parsed, ok := floatFromMap(data, "auto_action_min_confidence", "autoActionMinConfidence", "auto_action_confidence", "autoActionConfidence"); ok {
		autoActionMinConfidence = parsed
	}
	if autoActionMinConfidence < 0 {
		autoActionMinConfidence = 0
	}
	if autoActionMinConfidence > 100 {
		autoActionMinConfidence = 100
	}
	executionPolicy := normalizeAIAgentExecutionPolicy(stringFromMap(data, "execution_policy", "executionPolicy", "queue_execution_policy", "queueExecutionPolicy"))
	executionPriority := 0
	if parsed, ok := intFromMap(data, "execution_priority", "executionPriority", "queue_execution_priority", "queueExecutionPriority"); ok {
		executionPriority = normalizeAIAgentExecutionPriority(parsed)
	}

	return aiAgentDefinition{
		ID:                       item.ID,
		Name:                     name,
		Description:              description,
		Prompt:                   prompt,
		Model:                    model,
		Provider:                 provider,
		Endpoint:                 endpoint,
		Language:                 language,
		Enabled:                  enabled,
		TargetTypes:              targetTypes,
		CaseTags:                 caseTags,
		AlertSources:             alertSources,
		AutoCaseTags:             autoCaseTags,
		InvestigationPlan:        investigationPlan,
		EnrichmentConnectorIDs:   connectorIDs,
		NotificationConnectorIDs: notificationConnectorIDs,
		MaxCasesPerRun:           maxCasesPerRun,
		AutoCreateTasks:          autoCreateTasks,
		AutoComment:              autoComment,
		AutoCloseCase:            autoCloseCase,
		AutoCloseVerdicts:        autoCloseVerdicts,
		AutoCreateCaseFromAlert:  autoCreateCaseFromAlert,
		TaskAssigneeID:           taskAssigneeID,
		TriadEnabled:             triadEnabled,
		TriadCriticalOnly:        triadCriticalOnly,
		InvestigatorPrompt:       investigatorPrompt,
		ReviewerPrompt:           reviewerPrompt,
		ArbiterPrompt:            arbiterPrompt,
		RequireReviewerConsensus: requireReviewerConsensus,
		AutoActionMinConfidence:  autoActionMinConfidence,
		ExecutionPolicy:          executionPolicy,
		ExecutionPriority:        executionPriority,
	}
}

func normalizeAIAgentConnectorIDs(input []string) []string {
	if len(input) == 0 {
		return nil
	}
	out := make([]string, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for _, raw := range input {
		parsed, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			continue
		}
		id := parsed.String()
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (h *Handler) resolveAIAgentService(agent aiAgentDefinition) (*ai.Service, error) {
	if h.ai == nil {
		return nil, fmt.Errorf("ai service is not configured")
	}

	modelOverride := strings.TrimSpace(agent.Model)
	providerOverride := strings.TrimSpace(agent.Provider)
	endpointOverride := strings.TrimSpace(agent.Endpoint)
	if modelOverride == "" && providerOverride == "" && endpointOverride == "" {
		return h.ai, nil
	}

	cfg := h.cfg.AI
	if modelOverride != "" {
		cfg.Model = modelOverride
	}
	if endpointOverride != "" {
		cfg.Endpoint = endpointOverride
		cfg.OpenAIBaseURL = endpointOverride
	}
	if providerOverride != "" {
		provider, ok := normalizeAIAgentProvider(providerOverride)
		if !ok {
			return nil, fmt.Errorf("unsupported ai provider: %s", providerOverride)
		}
		cfg.Provider = provider
	}
	cfg.Enabled = true

	service := ai.NewService(cfg, h.search)
	if service == nil {
		return nil, fmt.Errorf("failed to configure ai service")
	}
	service.SetTenantContextProvider(newHandlerTenantContextProvider(h))
	return service, nil
}

func normalizeAIAgentProvider(raw string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "auto":
		return "", true
	case "openai", "openai-compatible", "openai_compatible", "openai compatible", "vllm", "lm-studio", "lmstudio", "openrouter", "localai":
		return "openai", true
	case "ollama":
		return "ollama", true
	default:
		return "", false
	}
}

func buildAIAgentComment(agent aiAgentDefinition, result ai.CaseAnalysisResult, enrichment []map[string]any) string {
	builder := strings.Builder{}
	builder.WriteString("AI Agent ")
	builder.WriteString(agent.Name)
	builder.WriteString(" completed automated case review.\n\n")
	if strings.TrimSpace(result.Summary) != "" {
		builder.WriteString("Summary: ")
		builder.WriteString(strings.TrimSpace(result.Summary))
		builder.WriteString("\n\n")
	}
	if len(result.Recommendations) > 0 {
		builder.WriteString("Recommendations:\n")
		for _, item := range result.Recommendations {
			text := strings.TrimSpace(item)
			if text == "" {
				continue
			}
			builder.WriteString("- ")
			builder.WriteString(text)
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}
	if len(result.MITER.Tactics) > 0 || len(result.MITER.Techniques) > 0 {
		builder.WriteString("MITER ATT&CK:\n")
		if len(result.MITER.Tactics) > 0 {
			builder.WriteString("- Tactics: ")
			builder.WriteString(strings.Join(result.MITER.Tactics, ", "))
			builder.WriteString("\n")
		}
		if len(result.MITER.Techniques) > 0 {
			builder.WriteString("- Techniques: ")
			builder.WriteString(strings.Join(result.MITER.Techniques, ", "))
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}
	if len(enrichment) > 0 {
		builder.WriteString("Enrichment responses:\n")
		for _, item := range enrichment {
			name := strings.TrimSpace(stringFromMap(item, "name"))
			if name == "" {
				name = strings.TrimSpace(stringFromMap(item, "connector_id"))
			}
			reply := strings.TrimSpace(stringFromMap(item, "reply"))
			errText := strings.TrimSpace(stringFromMap(item, "error"))
			if reply != "" {
				builder.WriteString("- ")
				builder.WriteString(name)
				builder.WriteString(": ")
				builder.WriteString(reply)
				builder.WriteString("\n")
			} else if errText != "" {
				builder.WriteString("- ")
				builder.WriteString(name)
				builder.WriteString(": error: ")
				builder.WriteString(errText)
				builder.WriteString("\n")
			}
		}
	}
	return truncateAgentText(strings.TrimSpace(builder.String()), 4000)
}

func buildAIAgentTriadComment(
	baseComment string,
	triadMeta map[string]any,
	triadWarnings []string,
	autoActionsAllowed bool,
	actionBlockers []string,
) string {
	builder := strings.Builder{}
	base := strings.TrimSpace(baseComment)
	if base != "" {
		builder.WriteString(base)
		builder.WriteString("\n\n")
	}
	builder.WriteString("Triad review:")
	builder.WriteString("\n")

	for _, role := range []string{"investigator", "reviewer", "arbiter"} {
		stage := normalizeMap(firstMapValue(triadMeta, role))
		if len(stage) == 0 {
			continue
		}
		builder.WriteString("- ")
		roleTitle := strings.ToUpper(role[:1]) + role[1:]
		builder.WriteString(roleTitle)
		if errText := strings.TrimSpace(stringFromMap(stage, "error")); errText != "" {
			builder.WriteString(": error: ")
			builder.WriteString(errText)
			builder.WriteString("\n")
			continue
		}
		if skipped := strings.TrimSpace(stringFromMap(stage, "skipped")); skipped != "" {
			builder.WriteString(": skipped: ")
			builder.WriteString(skipped)
			builder.WriteString("\n")
			continue
		}
		builder.WriteString(": verdict=")
		builder.WriteString(firstNonEmptyString(strings.TrimSpace(stringFromMap(stage, "verdict")), "unknown"))
		if confidence, ok := floatFromMap(stage, "confidence"); ok {
			builder.WriteString(", confidence=")
			_, _ = fmt.Fprintf(&builder, "%.1f", confidence)
		}
		summary := strings.TrimSpace(stringFromMap(stage, "summary"))
		if summary != "" {
			builder.WriteString(", summary=")
			builder.WriteString(truncateAgentText(summary, 400))
		}
		builder.WriteString("\n")
	}

	if consensus, ok := boolFromMap(triadMeta, "reviewer_consensus", "reviewerConsensus"); ok {
		builder.WriteString("- Reviewer consensus: ")
		if consensus {
			builder.WriteString("yes")
		} else {
			builder.WriteString("no")
		}
		builder.WriteString("\n")
	}
	if humanReview, ok := boolFromMap(triadMeta, "requires_human_review", "requiresHumanReview"); ok {
		builder.WriteString("- Human review required: ")
		if humanReview {
			builder.WriteString("yes")
		} else {
			builder.WriteString("no")
		}
		builder.WriteString("\n")
	}

	if autoActionsAllowed {
		builder.WriteString("- Auto actions: allowed\n")
	} else {
		builder.WriteString("- Auto actions: blocked\n")
	}
	for _, blocker := range dedupeNonEmptyStrings(actionBlockers) {
		builder.WriteString("  - ")
		builder.WriteString(blocker)
		builder.WriteString("\n")
	}

	for _, warning := range dedupeNonEmptyStrings(triadWarnings) {
		builder.WriteString("- Warning: ")
		builder.WriteString(warning)
		builder.WriteString("\n")
	}

	return truncateAgentText(strings.TrimSpace(builder.String()), 4000)
}

func buildAIAgentEnrichmentPrompt(caseItem models.Case, observables []models.Observable, stagePrompt string) string {
	builder := strings.Builder{}
	builder.WriteString("Provide enrichment for incident case.\n")
	builder.WriteString("Case ID: ")
	builder.WriteString(caseItem.ID.String())
	builder.WriteString("\n")
	if strings.TrimSpace(caseItem.CaseNumber) != "" {
		builder.WriteString("Case Number: ")
		builder.WriteString(strings.TrimSpace(caseItem.CaseNumber))
		builder.WriteString("\n")
	}
	builder.WriteString("Title: ")
	builder.WriteString(strings.TrimSpace(caseItem.Title))
	builder.WriteString("\n")
	builder.WriteString("Severity: ")
	builder.WriteString(strings.TrimSpace(caseItem.Severity))
	builder.WriteString("\n")
	builder.WriteString("Priority: ")
	builder.WriteString(strings.TrimSpace(caseItem.Priority))
	builder.WriteString("\n")
	if strings.TrimSpace(caseItem.Description) != "" {
		builder.WriteString("Description: ")
		builder.WriteString(truncateAgentText(strings.TrimSpace(caseItem.Description), 1000))
		builder.WriteString("\n")
	}
	if len(observables) > 0 {
		builder.WriteString("Observables:\n")
		limit := 8
		if len(observables) < limit {
			limit = len(observables)
		}
		for i := 0; i < limit; i++ {
			value := strings.TrimSpace(observables[i].Value)
			if value == "" {
				continue
			}
			builder.WriteString("- ")
			builder.WriteString(value)
			builder.WriteString("\n")
		}
	}
	if strings.TrimSpace(stagePrompt) != "" {
		builder.WriteString("Stage instruction: ")
		builder.WriteString(truncateAgentText(strings.TrimSpace(stagePrompt), 800))
		builder.WriteString("\n")
	}
	builder.WriteString("Return short actionable enrichment with confidence and next checks.")
	return truncateAgentText(builder.String(), 2500)
}

func aiAgentTaskTitle(recommendation string) string {
	trimmed := strings.TrimSpace(recommendation)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) > 120 {
		trimmed = trimmed[:120]
	}
	trimmed = strings.TrimSpace(trimmed)
	if trimmed == "" {
		return ""
	}
	return "AI: " + trimmed
}

func truncateAgentText(value string, maxLen int) string {
	text := strings.TrimSpace(value)
	if maxLen <= 0 || len(text) <= maxLen {
		return text
	}
	return strings.TrimSpace(text[:maxLen]) + "..."
}
