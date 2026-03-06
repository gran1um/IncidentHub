package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"incidenthub/backend/internal/ai"
	"incidenthub/backend/internal/logger"
	"incidenthub/backend/internal/middleware"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v5"
)

type aiAskRequest struct {
	Question  string `json:"question"`
	SessionID string `json:"session_id"`
	Language  string `json:"language"`
}

type createAISessionRequest struct {
	Title string `json:"title"`
}

type analyzeCaseAIRequest struct {
	Language string `json:"language"`
}

type reorderAISessionsRequest struct {
	SessionIDs []string `json:"session_ids"`
}

type listAIMessagesResponse struct {
	SessionID string `json:"session_id"`
	Messages  []any  `json:"messages"`
}

func (h *Handler) ListAISessions(c *echo.Context) error {
	if h.aiChats == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "ai chat repository is not configured")
	}
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	identity, _ := middleware.GetIdentity(c)

	limit := 30
	if rawLimit := strings.TrimSpace(c.QueryParam("limit")); rawLimit != "" {
		if parsed, parseErr := strconv.Atoi(rawLimit); parseErr == nil {
			limit = parsed
		}
	}
	if limit <= 0 || limit > 200 {
		limit = 30
	}

	items, err := h.aiChats.ListSessions(c.Request().Context(), tenantID, identity.UserID, limit)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list ai sessions")
	}
	return c.JSON(http.StatusOK, items)
}

func (h *Handler) ReorderAISessions(c *echo.Context) error {
	if h.aiChats == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "ai chat repository is not configured")
	}
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	identity, _ := middleware.GetIdentity(c)

	var req reorderAISessionsRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	ordered := make([]uuid.UUID, 0, len(req.SessionIDs))
	for _, raw := range req.SessionIDs {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		sessionID, parseErr := uuid.Parse(trimmed)
		if parseErr != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid session id in session_ids")
		}
		ordered = append(ordered, sessionID)
	}

	if err := h.aiChats.ReorderSessions(c.Request().Context(), tenantID, identity.UserID, ordered); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			return echo.NewHTTPError(http.StatusNotFound, "ai session not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to reorder ai sessions")
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) CreateAISession(c *echo.Context) error {
	if h.aiChats == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "ai chat repository is not configured")
	}
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	identity, _ := middleware.GetIdentity(c)

	var req createAISessionRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	item, err := h.aiChats.CreateSession(c.Request().Context(), repository.CreateAIChatSessionParams{
		TenantID: tenantID,
		UserID:   identity.UserID,
		Title:    req.Title,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create ai session")
	}
	return c.JSON(http.StatusCreated, item)
}

func (h *Handler) DeleteAISession(c *echo.Context) error {
	if h.aiChats == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "ai chat repository is not configured")
	}
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	identity, _ := middleware.GetIdentity(c)

	sessionID, err := uuid.Parse(strings.TrimSpace(c.Param("sessionID")))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid sessionID path param")
	}

	deleted, err := h.aiChats.DeleteSession(c.Request().Context(), tenantID, identity.UserID, sessionID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete ai session")
	}
	if !deleted {
		return echo.NewHTTPError(http.StatusNotFound, "ai session not found")
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) GetAISession(c *echo.Context) error {
	if h.aiChats == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "ai chat repository is not configured")
	}
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	identity, _ := middleware.GetIdentity(c)

	session, err := h.aiChats.GetFirstSession(c.Request().Context(), tenantID, identity.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || strings.Contains(strings.ToLower(err.Error()), "no rows") {
			created, createErr := h.aiChats.CreateSession(c.Request().Context(), repository.CreateAIChatSessionParams{
				TenantID: tenantID,
				UserID:   identity.UserID,
				Title:    "SOC Assistant",
			})
			if createErr != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to create ai session")
			}
			return c.JSON(http.StatusOK, created)
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load ai session")
	}
	return c.JSON(http.StatusOK, session)
}

func (h *Handler) ListAIMessages(c *echo.Context) error {
	if h.aiChats == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "ai chat repository is not configured")
	}
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	identity, _ := middleware.GetIdentity(c)

	session, err := h.resolveAISession(c, tenantID, identity.UserID, strings.TrimSpace(c.QueryParam("session_id")))
	if err != nil {
		return err
	}

	limit := 80
	if rawLimit := strings.TrimSpace(c.QueryParam("limit")); rawLimit != "" {
		if parsed, parseErr := strconv.Atoi(rawLimit); parseErr == nil {
			limit = parsed
		}
	}
	if limit <= 0 || limit > 500 {
		limit = 80
	}

	items, err := h.aiChats.ListMessages(c.Request().Context(), tenantID, identity.UserID, session.ID, limit)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list ai chat messages")
	}
	reverseMessages(items)
	payload := make([]any, 0, len(items))
	for _, item := range items {
		payload = append(payload, item)
	}
	return c.JSON(http.StatusOK, listAIMessagesResponse{
		SessionID: session.ID.String(),
		Messages:  payload,
	})
}

func (h *Handler) AskAI(c *echo.Context) error {
	if h.ai == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "ai service is not configured")
	}
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	identity, _ := middleware.GetIdentity(c)

	var req aiAskRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	req.Question = strings.TrimSpace(req.Question)
	if req.Question == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "question is required")
	}

	if h.aiChats == nil {
		aiCtx, cancel := h.aiRequestContext(c.Request().Context())
		defer cancel()
		answer, err := h.ai.AskWithHistoryAndLanguage(
			aiCtx,
			tenantID.String(),
			req.Question,
			resolveAskLanguage(req.Question, req.Language),
			nil,
		)
		if err != nil {
			h.logAIRequestFailure("chat", tenantID, "", err)
			return aiRequestHTTPError(err)
		}
		return c.JSON(http.StatusOK, answer)
	}

	session, err := h.resolveAISession(c, tenantID, identity.UserID, req.SessionID)
	if err != nil {
		return err
	}

	historyRecords, historyErr := h.aiChats.ListMessages(c.Request().Context(), tenantID, identity.UserID, session.ID, h.cfg.AI.HistoryMessages)
	if historyErr != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load ai history")
	}
	reverseMessages(historyRecords)
	history := make([]ai.ChatMessage, 0, len(historyRecords))
	for _, item := range historyRecords {
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(item.Role))
		if role != "assistant" && role != "user" && role != "system" {
			continue
		}
		history = append(history, ai.ChatMessage{Role: role, Content: content})
	}

	aiCtx, cancel := h.aiRequestContext(c.Request().Context())
	defer cancel()
	answer, err := h.ai.AskWithHistoryAndLanguage(
		aiCtx,
		tenantID.String(),
		req.Question,
		resolveAskLanguage(req.Question, req.Language),
		history,
	)
	if err != nil {
		h.logAIRequestFailure("chat", tenantID, session.ID.String(), err)
		return aiRequestHTTPError(err)
	}

	userMessage, err := h.aiChats.CreateMessage(c.Request().Context(), repository.CreateAIChatMessageParams{
		SessionID: session.ID,
		TenantID:  tenantID,
		UserID:    identity.UserID,
		Role:      "user",
		Content:   req.Question,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to store ai request message")
	}
	assistantMessage, err := h.aiChats.CreateMessage(c.Request().Context(), repository.CreateAIChatMessageParams{
		SessionID: session.ID,
		TenantID:  tenantID,
		UserID:    identity.UserID,
		Role:      "assistant",
		Content:   answer.Answer,
		Sources:   sourcesToGeneric(answer.Sources),
		Metadata: map[string]any{
			"model":           answer.Model,
			"confidence":      answer.Confidence,
			"recommendations": answer.Recommendations,
		},
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to store ai response message")
	}

	return c.JSON(http.StatusOK, map[string]any{
		"session_id":        session.ID.String(),
		"user_message_id":   userMessage.ID.String(),
		"assistant_message": assistantMessage,
		"answer":            answer.Answer,
		"confidence":        answer.Confidence,
		"model":             answer.Model,
		"sources":           answer.Sources,
		"recommendations":   answer.Recommendations,
	})
}

func (h *Handler) AnalyzeCaseWithAI(c *echo.Context) error {
	if h.ai == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "ai service is not configured")
	}
	if h.caseAIAnalyses == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "case ai analysis repository is not configured")
	}

	tenantID, caseID, err := h.resolveCaseInTenant(c)
	if err != nil {
		return err
	}
	var req analyzeCaseAIRequest
	if c.Request().ContentLength > 0 {
		if bindErr := c.Bind(&req); bindErr != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
		}
	}
	language := normalizeAILanguage(req.Language)
	identity, _ := middleware.GetIdentity(c)

	caseItem, err := h.cases.GetByID(c.Request().Context(), tenantID, caseID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "case not found")
	}
	tasks, _ := h.tasks.ListByCase(c.Request().Context(), tenantID, caseID, 300, 0)
	observables, _ := h.observables.ListByCase(c.Request().Context(), tenantID, caseID, 500, 0)
	events, _ := h.caseEvents.ListByCase(c.Request().Context(), tenantID, caseID, 300, 0)
	pages, _ := h.casePages.ListByCase(c.Request().Context(), tenantID, caseID, 100, 0)

	comments := make([]string, 0)
	if h.catalog != nil {
		commentItems, listErr := h.catalog.List(c.Request().Context(), repository.CatalogListParams{
			Kind:     "case_comment",
			TenantID: &tenantID,
			RefID:    &caseID,
			Limit:    300,
		})
		if listErr == nil {
			for _, item := range commentItems {
				content := strings.TrimSpace(stringFromMap(item.Data, "content"))
				if content == "" {
					continue
				}
				comments = append(comments, content)
			}
		}
	}

	input := ai.CaseAnalysisInput{
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
		Tasks:             mapTasksForAnalysis(tasks),
		Observables:       mapObservablesForAnalysis(observables),
		Events:            mapEventsForAnalysis(events),
		Pages:             mapPagesForAnalysis(pages),
		Comments:          comments,
	}

	aiCtx, cancel := h.aiRequestContext(c.Request().Context())
	defer cancel()
	result, err := h.ai.AnalyzeCaseWithLanguage(aiCtx, tenantID.String(), input, language)
	if err != nil {
		_, _ = h.caseAIAnalyses.Create(c.Request().Context(), repository.CreateCaseAIAnalysisParams{
			TenantID:     tenantID,
			CaseID:       caseID,
			RequestedBy:  &identity.UserID,
			Model:        h.cfg.AI.Model,
			Status:       "failed",
			Verdict:      "unknown",
			Confidence:   0,
			Summary:      "AI-анализ завершился ошибкой.",
			ErrorMessage: err.Error(),
		})
		return aiRequestHTTPError(err)
	}

	stored, err := h.caseAIAnalyses.Create(c.Request().Context(), repository.CreateCaseAIAnalysisParams{
		TenantID:        tenantID,
		CaseID:          caseID,
		RequestedBy:     &identity.UserID,
		Model:           result.Model,
		Status:          "completed",
		Verdict:         result.Verdict,
		Confidence:      result.Confidence,
		Summary:         result.Summary,
		Recommendations: result.Recommendations,
		Findings:        result.Findings,
		Sources:         sourcesToGeneric(result.Sources),
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to persist case ai analysis")
	}

	if h.catalog != nil {
		_ = h.upsertCaseAIVerdict(c.Request().Context(), tenantID, caseID, identity.UserID, result)
	}
	_ = h.audits.Log(c.Request().Context(), &tenantID, &identity.UserID, "case_ai_analysis", "case", &caseID, map[string]any{
		"analysis_id": stored.ID.String(),
		"verdict":     stored.Verdict,
		"confidence":  stored.Confidence,
	})
	h.rewardAnalyzerInvocation(c.Request().Context(), tenantID, identity.UserID, "case_analysis:"+stored.ID.String(), map[string]any{
		"analysis_id": stored.ID.String(),
		"case_id":     caseID.String(),
		"triggered":   xpEventTypeAnalyzerInvoked,
	})

	return c.JSON(http.StatusOK, stored)
}

func (h *Handler) aiRequestContext(parent context.Context) (context.Context, context.CancelFunc) {
	timeout := h.cfg.AI.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	if h.cfg.HTTP.WriteTimeout > 0 {
		if timeout > h.cfg.HTTP.WriteTimeout {
			timeout = h.cfg.HTTP.WriteTimeout
		}
	}
	return context.WithTimeout(parent, timeout)
}

func (h *Handler) ListCaseAIAnalyses(c *echo.Context) error {
	if h.caseAIAnalyses == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "case ai analysis repository is not configured")
	}
	tenantID, caseID, err := h.resolveCaseInTenant(c)
	if err != nil {
		return err
	}

	limit := 50
	if rawLimit := strings.TrimSpace(c.QueryParam("limit")); rawLimit != "" {
		if parsed, parseErr := strconv.Atoi(rawLimit); parseErr == nil {
			limit = parsed
		}
	}
	if limit <= 0 || limit > 500 {
		limit = 50
	}

	items, err := h.caseAIAnalyses.ListByCase(c.Request().Context(), tenantID, caseID, limit)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list case ai analyses")
	}
	return c.JSON(http.StatusOK, items)
}

func (h *Handler) resolveAISession(c *echo.Context, tenantID, userID uuid.UUID, sessionRaw string) (*models.AIChatSession, error) {
	if h.aiChats == nil {
		return nil, echo.NewHTTPError(http.StatusServiceUnavailable, "ai chat repository is not configured")
	}
	trimmed := strings.TrimSpace(sessionRaw)
	if trimmed == "" {
		firstSession, firstErr := h.aiChats.GetFirstSession(c.Request().Context(), tenantID, userID)
		if firstErr == nil {
			return firstSession, nil
		}
		if !errors.Is(firstErr, pgx.ErrNoRows) && !strings.Contains(strings.ToLower(firstErr.Error()), "no rows") {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to load ai session")
		}
		created, createErr := h.aiChats.CreateSession(c.Request().Context(), repository.CreateAIChatSessionParams{
			TenantID: tenantID,
			UserID:   userID,
			Title:    "SOC Assistant",
		})
		if createErr != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to create ai session")
		}
		return created, nil
	}

	sessionID, err := uuid.Parse(trimmed)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid session_id")
	}
	session, err := h.aiChats.GetSessionByID(c.Request().Context(), tenantID, userID, sessionID)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "ai session not found")
	}
	return session, nil
}

func normalizeAILanguage(raw string) string {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	switch normalized {
	case "ru", "ru-ru", "russian":
		return "ru"
	case "en", "en-us", "en-gb", "english":
		return "en"
	default:
		return "en"
	}
}

func resolveAskLanguage(question, fallback string) string {
	if detected := detectQuestionLanguage(question); detected != "" {
		return detected
	}
	return normalizeAILanguage(fallback)
}

func detectQuestionLanguage(question string) string {
	trimmed := strings.TrimSpace(question)
	if trimmed == "" {
		return ""
	}
	hasLatin := false
	hasCyrillic := false
	for _, r := range trimmed {
		switch {
		case unicode.In(r, unicode.Cyrillic):
			hasCyrillic = true
		case unicode.In(r, unicode.Latin):
			hasLatin = true
		}
	}
	if hasCyrillic {
		return "ru"
	}
	if hasLatin {
		return "en"
	}
	return ""
}

func mapTasksForAnalysis(items []models.Task) []ai.CaseAnalysisTask {
	out := make([]ai.CaseAnalysisTask, 0, len(items))
	for _, item := range items {
		assignee := ""
		if item.AssigneeID != nil {
			assignee = item.AssigneeID.String()
		}
		out = append(out, ai.CaseAnalysisTask{
			Title:    item.Title,
			Status:   item.Status,
			Assignee: assignee,
		})
	}
	return out
}

func mapObservablesForAnalysis(items []models.Observable) []ai.CaseAnalysisObservable {
	out := make([]ai.CaseAnalysisObservable, 0, len(items))
	for _, item := range items {
		out = append(out, ai.CaseAnalysisObservable{
			Type:    item.Type,
			Value:   item.Value,
			Verdict: item.Verdict,
		})
	}
	return out
}

func mapEventsForAnalysis(items []models.CaseTimelineEvent) []ai.CaseAnalysisEvent {
	out := make([]ai.CaseAnalysisEvent, 0, len(items))
	for _, item := range items {
		out = append(out, ai.CaseAnalysisEvent{
			EventType: item.EventType,
			Title:     item.Title,
			Body:      item.Body,
		})
	}
	return out
}

func mapPagesForAnalysis(items []models.CasePage) []ai.CaseAnalysisPage {
	out := make([]ai.CaseAnalysisPage, 0, len(items))
	for _, item := range items {
		out = append(out, ai.CaseAnalysisPage{
			Title: item.Title,
			Body:  item.Body,
		})
	}
	return out
}

func sourcesToGeneric(items []ai.Source) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{
			"kind":  item.Kind,
			"id":    item.ID,
			"title": item.Title,
		})
	}
	return out
}

func reverseMessages(items []models.AIChatMessage) {
	for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
		items[left], items[right] = items[right], items[left]
	}
}

func (h *Handler) upsertCaseAIVerdict(ctx context.Context, tenantID, caseID, actorID uuid.UUID, analysis ai.CaseAnalysisResult) error {
	items, err := h.catalog.List(ctx, repository.CatalogListParams{
		Kind:     "case_meta",
		TenantID: &tenantID,
		RefID:    &caseID,
		Limit:    1,
	})
	if err != nil {
		return err
	}

	data := map[string]any{
		"verdict":         verdictToCaseMeta(analysis.Verdict),
		"recommendations": analysis.Recommendations,
	}
	if len(items) > 0 {
		_, err = h.catalog.Update(ctx, "case_meta", items[0].ID, &tenantID, repository.CatalogUpdateParams{Data: data})
		return err
	}
	_, err = h.catalog.Create(ctx, repository.CatalogCreateParams{
		TenantID:  &tenantID,
		Kind:      "case_meta",
		OwnerID:   &actorID,
		RefID:     &caseID,
		Data:      data,
		CreatedBy: &actorID,
	})
	return err
}

func verdictToCaseMeta(input string) string {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "malicious":
		return "Malicious"
	case "suspicious":
		return "Suspicious"
	case "benign":
		return "Benign"
	default:
		return "Unknown"
	}
}

func aiRequestHTTPError(sourceErr error) *echo.HTTPError {
	message := "failed to process ai request"
	lowered := strings.ToLower(strings.TrimSpace(sourceErr.Error()))
	switch {
	case strings.Contains(lowered, "not configured"),
		strings.Contains(lowered, "llm chat request failed"),
		strings.Contains(lowered, "empty answer"):
		message = "ai model is unavailable or misconfigured; check AI_PROVIDER, AI_ENDPOINT and AI_MODEL settings"
	case strings.Contains(lowered, "parse llm case analysis output"),
		strings.Contains(lowered, "llm case analysis output missing summary"):
		message = "ai model returned invalid structured output; adjust AI model or prompt settings"
	}
	return echo.NewHTTPError(http.StatusBadGateway, message)
}

func (h *Handler) logAIRequestFailure(operation string, tenantID uuid.UUID, sessionID string, sourceErr error) {
	if h == nil || sourceErr == nil {
		return
	}
	logger.Warnf(
		"ai %s failed: tenant=%s session_id=%s provider=%s endpoint=%s model=%s err=%v",
		strings.TrimSpace(operation),
		tenantID.String(),
		strings.TrimSpace(sessionID),
		strings.TrimSpace(h.cfg.AI.Provider),
		strings.TrimSpace(h.cfg.AI.Endpoint),
		strings.TrimSpace(h.cfg.AI.Model),
		sourceErr,
	)
}
