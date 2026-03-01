package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/logger"
	"incidenthub/backend/internal/search"
	"incidenthub/backend/internal/tracing"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const maxAIConfidence = 0.92

type Retriever interface {
	Search(ctx context.Context, tenantID, query string, kinds []string, size int) ([]search.Hit, error)
	Enabled() bool
}
type recentRetriever interface {
	Recent(ctx context.Context, tenantID string, kinds []string, size int) ([]search.Hit, error)
}
type TenantContext struct {
	Prompt  string
	Sources []Source
}
type TenantContextProvider interface {
	BuildContext(ctx context.Context, tenantID, question string) (TenantContext, error)
}
type LLMClient interface {
	Chat(ctx context.Context, model string, messages []ChatMessage) (string, error)
	Ping(ctx context.Context) error
}
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type Service struct {
	cfg             config.AIConfig
	retriever       Retriever
	llm             LLMClient
	contextProvider TenantContextProvider
}
type Source struct {
	Kind  string `json:"kind"`
	ID    string `json:"id"`
	Title string `json:"title"`
}
type Answer struct {
	Model           string   `json:"model"`
	Question        string   `json:"question"`
	Answer          string   `json:"answer"`
	Confidence      float64  `json:"confidence"`
	Recommendations []string `json:"recommendations"`
	Sources         []Source `json:"sources"`
}
type CaseAnalysisTask struct {
	Title     string `json:"title"`
	Status    string `json:"status"`
	Assignee  string `json:"assignee"`
	Mandatory bool   `json:"mandatory"`
}
type CaseAnalysisObservable struct {
	Type    string `json:"type"`
	Value   string `json:"value"`
	Verdict string `json:"verdict"`
}
type CaseAnalysisEvent struct {
	EventType string `json:"event_type"`
	Title     string `json:"title"`
	Body      string `json:"body"`
}
type CaseAnalysisPage struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}
type CaseAnalysisInput struct {
	CaseID            string                   `json:"case_id"`
	CaseNumber        string                   `json:"case_number"`
	Title             string                   `json:"title"`
	Description       string                   `json:"description"`
	Source            string                   `json:"source"`
	IncidentType      string                   `json:"incident_type"`
	Status            string                   `json:"status"`
	Priority          string                   `json:"priority"`
	Impact            string                   `json:"impact"`
	Severity          string                   `json:"severity"`
	TLP               string                   `json:"tlp"`
	PAP               string                   `json:"pap"`
	Confidence        int                      `json:"confidence"`
	ResolutionSummary string                   `json:"resolution_summary"`
	Tasks             []CaseAnalysisTask       `json:"tasks"`
	Observables       []CaseAnalysisObservable `json:"observables"`
	Events            []CaseAnalysisEvent      `json:"events"`
	Pages             []CaseAnalysisPage       `json:"pages"`
	Comments          []string                 `json:"comments"`
}
type CaseAnalysisResult struct {
	Model           string       `json:"model"`
	Verdict         string       `json:"verdict"`
	Confidence      float64      `json:"confidence"`
	Summary         string       `json:"summary"`
	Recommendations []string     `json:"recommendations"`
	Findings        []string     `json:"findings"`
	MITER           MITREMapping `json:"miter"`
	Sources         []Source     `json:"sources"`
}
type caseAnalysisLLMResponse struct {
	Verdict         string       `json:"verdict"`
	Confidence      float64      `json:"confidence"`
	Summary         string       `json:"summary"`
	Recommendations []string     `json:"recommendations"`
	Findings        []string     `json:"findings"`
	MITER           MITREMapping `json:"miter"`
}

var jsonObjectRegex = regexp.MustCompile(`\{[\s\S]*\}`)
var thinkBlockRegex = regexp.MustCompile(`(?is)<think>[\s\S]*?</think>`)

func NewService(cfg config.AIConfig, retriever Retriever) *Service {
	if cfg.TopK <= 0 {
		cfg.TopK = 6
	}
	if cfg.Model == "" {
		cfg.Model = "llama3.2:3b"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 20 * time.Second
	}
	if cfg.HistoryMessages <= 0 {
		cfg.HistoryMessages = 20
	}
	if cfg.MaxContextChars <= 0 {
		cfg.MaxContextChars = 5000
	}
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 2
	}
	if cfg.MaxConcurrent > 128 {
		cfg.MaxConcurrent = 128
	}
	var llm LLMClient
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if provider == "" {
		if endpointLooksOpenAICompatible(endpoint) {
			provider = "openai"
		} else {
			provider = "ollama"
		}
	}
	if provider == "ollama" && endpointLooksOpenAICompatible(endpoint) {
		provider = "openai"
	}
	switch provider {
	case "ollama":
		if endpoint == "" {
			endpoint = "http://localhost:11434"
		}
		llm = NewOllamaClient(endpoint, cfg.Timeout)
	case "openai", "openai-compatible", "openai_compatible":
		if endpoint == "" || strings.EqualFold(endpoint, "http://localhost:11434") {
			endpoint = strings.TrimSpace(cfg.OpenAIBaseURL)
		}
		if endpoint == "" {
			endpoint = "https://api.openai.com/v1"
		}
		apiKey := strings.TrimSpace(cfg.APIKey)
		if apiKey == "" {
			apiKey = strings.TrimSpace(cfg.OpenAIAPIKey)
		}
		llm = NewOpenAIClient(endpoint, apiKey, cfg.Timeout)
	}
	return &Service{cfg: cfg, retriever: retriever, llm: llm}
}
func endpointLooksOpenAICompatible(endpoint string) bool {
	normalized := strings.ToLower(strings.TrimSpace(endpoint))
	if normalized == "" {
		return false
	}
	if strings.Contains(normalized, "/chat/completions") || strings.Contains(normalized, "/v1") {
		return true
	}
	if strings.Contains(normalized, ":11434") {
		return false
	}
	if strings.Contains(normalized, "openai") {
		return true
	}
	return false
}
func (s *Service) SetTenantContextProvider(provider TenantContextProvider) {
	if s == nil {
		return
	}
	s.contextProvider = provider
}
func (s *Service) Ask(ctx context.Context, tenantID, question string) (Answer, error) {
	return s.AskWithHistoryAndLanguage(ctx, tenantID, question, "", nil)
}
func (s *Service) Ping(ctx context.Context) error {
	ctx, span, startedAt := tracing.StartModuleOperation(ctx, "ai", "ping")
	var err error
	defer func() {
		tracing.FinishModuleOperation(span, startedAt, "ai", "ping", err)
	}()
	if s == nil || !s.cfg.Enabled {
		return nil
	}
	if s.llm == nil {
		err = fmt.Errorf("llm client is not configured")
		return err
	}
	err = s.llm.Ping(ctx)
	return err
}
func (s *Service) AskWithHistory(ctx context.Context, tenantID, question string, history []ChatMessage) (Answer, error) {
	return s.AskWithHistoryAndLanguage(ctx, tenantID, question, "", history)
}
func (s *Service) AskWithHistoryAndLanguage(ctx context.Context, tenantID, question, language string, history []ChatMessage) (Answer, error) {
	ctx, span, startedAt := tracing.StartModuleOperation(ctx, "ai", "ask")
	var err error
	defer func() {
		tracing.FinishModuleOperation(span, startedAt, "ai", "ask", err)
	}()
	question = strings.TrimSpace(question)
	language = resolveMessageLanguage(question, language)
	if question == "" {
		err = fmt.Errorf("question is required")
		return Answer{}, err
	}
	if s == nil {
		err = fmt.Errorf("ai service is not configured")
		return Answer{}, err
	}
	externalCtx := TenantContext{}
	if s != nil && s.contextProvider != nil {
		if tenantContext, ctxErr := s.contextProvider.BuildContext(ctx, tenantID, question); ctxErr == nil {
			externalCtx = tenantContext
		}
	}
	hits, sources, err := s.retrieveSources(ctx, tenantID, question, []string{"alerts", "cases", "observables", "forum_threads"})
	if err != nil {
		return Answer{}, err
	}
	combinedSources := mergeSources(sources, externalCtx.Sources)
	route := selectAutonomousHuntRoute(question, combinedSources)
	responseText := ""
	recommendations := baseRecommendations(question)
	confidence := 0.30
	answerModel := s.cfg.Model
	if len(combinedSources) > 0 {
		confidence = minFloat(0.35+float64(len(combinedSources))*0.08, maxAIConfidence)
	}
	if !s.canUseLLM() {
		if s == nil {
			err = fmt.Errorf("ai service is not configured")
			return Answer{}, err
		}
		if !s.cfg.Enabled {
			err = fmt.Errorf("ai module is disabled")
			return Answer{}, err
		}
		if s.llm == nil {
			err = fmt.Errorf("llm client is not configured")
			return Answer{}, err
		}
		err = fmt.Errorf("ai model is unavailable or misconfigured")
		return Answer{}, err
	}
	contextText := s.searchContextBlock(hits)
	if shouldUseConversationalPrompt(question) {
		messages := make([]ChatMessage, 0, len(history)+2)
		messages = append(messages, ChatMessage{
			Role: "system",
			Content: firstNonEmpty(
				strings.TrimSpace(s.cfg.SystemPrompt),
				autonomousThreatHunterSystemPrompt,
			),
		})
		history = trimHistory(history, s.cfg.HistoryMessages)
		messages = append(messages, history...)
		messages = append(messages, ChatMessage{
			Role: "user",
			Content: buildConversationalAskPrompt(
				question,
				contextText,
				externalCtx.Prompt,
				language,
			),
		})
		modelAnswer, modelErr := s.llmChat(ctx, messages)
		if modelErr == nil {
			trimmed := sanitizeModelOutput(modelAnswer)
			if trimmed != "" {
				responseText = trimmed
				return Answer{
					Model:           answerModel,
					Question:        question,
					Answer:          responseText,
					Confidence:      minFloat(confidence+0.08, maxAIConfidence),
					Recommendations: recommendations,
					Sources:         combinedSources,
				}, nil
			}
		}
	}
	messages := make([]ChatMessage, 0, len(history)+2)
	messages = append(messages, ChatMessage{
		Role: "system",
		Content: firstNonEmpty(
			strings.TrimSpace(s.cfg.SystemPrompt),
			autonomousThreatHunterSystemPrompt,
		),
	})
	history = trimHistory(history, s.cfg.HistoryMessages)
	messages = append(messages, history...)
	messages = append(messages, ChatMessage{
		Role:    "user",
		Content: buildAutonomousAskPrompt(question, contextText, externalCtx.Prompt, route, combinedSources, language),
	})
	modelAnswer, modelErr := s.llmChat(ctx, messages)
	if modelErr != nil {
		// Retry once with compact prompt and no history to reduce payload pressure on smaller local models.
		retryContextLimit := minInt(maxInt(s.cfg.MaxContextChars/2, 1400), s.cfg.MaxContextChars)
		retryMessages := []ChatMessage{
			messages[0],
			{
				Role: "user",
				Content: buildAutonomousAskPrompt(
					question,
					truncate(contextText, retryContextLimit),
					truncate(externalCtx.Prompt, retryContextLimit),
					route,
					combinedSources,
					language,
				),
			},
		}
		modelAnswer, modelErr = s.llmChat(ctx, retryMessages)
		if modelErr != nil {
			err = fmt.Errorf("llm chat request failed: %w", modelErr)
			return Answer{}, err
		}
	}
	if modelErr != nil {
		err = fmt.Errorf("llm chat request failed: %w", modelErr)
		return Answer{}, err
	}
	trimmed := sanitizeModelOutput(modelAnswer)
	if trimmed == "" {
		err = fmt.Errorf("empty answer")
		return Answer{}, err
	}
	if parsed, ok := parseAutonomousHuntModelOutput(trimmed); ok {
		if autonomousOutputIsLowQuality(question, parsed) {
			err = fmt.Errorf("low quality answer")
			return Answer{}, err
		}
		responseText = formatAutonomousHuntAnswer(parsed, language)
		recommendations = recommendationsFromAutonomousResponse(parsed, recommendations)
		confidence = confidenceScoreFromLabel(parsed.Confidence, len(combinedSources))
	} else {
		responseText = trimmed
	}
	return Answer{
		Model:           answerModel,
		Question:        question,
		Answer:          responseText,
		Confidence:      confidence,
		Recommendations: recommendations,
		Sources:         combinedSources,
	}, nil
}
func (s *Service) AnalyzeCase(ctx context.Context, tenantID string, input CaseAnalysisInput) (CaseAnalysisResult, error) {
	return s.AnalyzeCaseWithLanguage(ctx, tenantID, input, "")
}
func (s *Service) AnalyzeCaseWithLanguage(ctx context.Context, tenantID string, input CaseAnalysisInput, language string) (CaseAnalysisResult, error) {
	ctx, span, startedAt := tracing.StartModuleOperation(ctx, "ai", "analyze_case")
	var err error
	defer func() {
		tracing.FinishModuleOperation(span, startedAt, "ai", "analyze_case", err)
	}()
	language = normalizeResponseLanguage(language)
	analysisQuery := strings.TrimSpace(strings.Join([]string{
		input.CaseNumber,
		input.Title,
		input.Description,
		strings.Join(observableValues(input.Observables, 6), " "),
	}, " "))
	if analysisQuery == "" {
		analysisQuery = "incident case analysis"
	}
	hits, sources, err := s.retrieveSources(ctx, tenantID, analysisQuery, []string{"cases", "alerts", "observables", "forum_threads"})
	if err != nil {
		return CaseAnalysisResult{}, err
	}
	fallback := fallbackCaseAnalysis(input, sources, language)
	if !s.canUseLLM() {
		if s != nil && s.cfg.Enabled {
			err = fmt.Errorf("ai model is enabled but llm client is not configured")
			return CaseAnalysisResult{}, err
		}
		return fallback, nil
	}
	contextPayload := s.caseContextPayload(input, hits)
	prompt := caseAnalysisAutonomousPrompt(input, language)
	messages := []ChatMessage{
		{
			Role: "system",
			Content: firstNonEmpty(
				strings.TrimSpace(s.cfg.SystemPrompt),
				autonomousThreatHunterSystemPrompt,
			),
		},
		{
			Role:    "user",
			Content: prompt + "\n\nCase Context:\n" + contextPayload,
		},
	}
	rawAnswer, modelErr := s.llmChat(ctx, messages)
	if modelErr != nil {
		retryContextLimit := minInt(maxInt(s.cfg.MaxContextChars/2, 1400), s.cfg.MaxContextChars)
		retryMessages := []ChatMessage{
			messages[0],
			{
				Role:    "user",
				Content: prompt + "\n\nCase Context:\n" + truncate(contextPayload, retryContextLimit),
			},
		}
		rawAnswer, modelErr = s.llmChat(ctx, retryMessages)
		if modelErr != nil {
			err = fmt.Errorf("llm chat request failed: %w", modelErr)
			return CaseAnalysisResult{}, err
		}
	}
	parsed, parseErr := parseCaseAnalysisModelOutput(sanitizeModelOutput(rawAnswer))
	if parseErr != nil {
		err = fmt.Errorf("parse llm case analysis output: %w", parseErr)
		return CaseAnalysisResult{}, err
	}
	result := CaseAnalysisResult{
		Model:           s.cfg.Model,
		Verdict:         normalizeVerdict(parsed.Verdict),
		Confidence:      clampFloat(parsed.Confidence, 0, 100),
		Summary:         strings.TrimSpace(parsed.Summary),
		Recommendations: trimStringSlice(parsed.Recommendations, 8),
		Findings:        trimStringSlice(parsed.Findings, 10),
		MITER:           parsed.MITER,
		Sources:         sources,
	}
	if result.Summary == "" {
		err = fmt.Errorf("llm case analysis output missing summary")
		return CaseAnalysisResult{}, err
	}
	if len(result.Recommendations) == 0 {
		result.Recommendations = fallback.Recommendations
	}
	if len(result.Findings) == 0 {
		result.Findings = fallback.Findings
	}
	return result, nil
}
func (s *Service) canUseLLM() bool {
	if s == nil || !s.cfg.Enabled {
		return false
	}
	return s.llm != nil && strings.TrimSpace(s.cfg.Model) != ""
}
func (s *Service) llmChat(ctx context.Context, messages []ChatMessage) (string, error) {
	if s == nil || s.llm == nil {
		return "", fmt.Errorf("llm client is not configured")
	}
	release, err := acquireLLMSlot(ctx, s.cfg.MaxConcurrent)
	if err != nil {
		return "", err
	}
	defer release()
	return s.llm.Chat(ctx, s.cfg.Model, messages)
}
func (s *Service) retrieveSources(ctx context.Context, tenantID, query string, kinds []string) ([]search.Hit, []Source, error) {
	if s == nil || !s.cfg.Enabled || s.retriever == nil || !s.retriever.Enabled() {
		return []search.Hit{}, []Source{}, nil
	}
	hits, err := s.retriever.Search(ctx, tenantID, query, kinds, s.cfg.TopK)
	if err != nil {
		logger.Warnf("ai knowledge base search failed, continuing without indexed context: tenant=%s err=%v", tenantID, err)
		return []search.Hit{}, []Source{}, nil
	}
	if len(hits) == 0 {
		if contextualRetriever, ok := s.retriever.(recentRetriever); ok {
			recentHits, recentErr := contextualRetriever.Recent(ctx, tenantID, kinds, s.cfg.TopK)
			if recentErr != nil {
				logger.Warnf("ai knowledge base fallback search failed, continuing without indexed context: tenant=%s err=%v", tenantID, recentErr)
				recentHits = []search.Hit{}
			}
			hits = recentHits
		}
	}
	sources := make([]Source, 0, len(hits))
	for _, hit := range hits {
		title := firstNonEmpty(
			fmt.Sprint(hit.Source["title"]),
			fmt.Sprint(hit.Source["name"]),
			fmt.Sprintf("%s %s", strings.ToUpper(hit.Kind), hit.ID),
		)
		sources = append(sources, Source{
			Kind:  hit.Kind,
			ID:    hit.ID,
			Title: title,
		})
	}
	return hits, sources, nil
}
func (s *Service) searchContextBlock(hits []search.Hit) string {
	if len(hits) == 0 {
		return "По запросу не найден релевантный контекст в индексах тенанта."
	}
	lines := make([]string, 0, len(hits)*2)
	for i, hit := range hits {
		if i >= s.cfg.TopK {
			break
		}
		title := firstNonEmpty(fmt.Sprint(hit.Source["title"]), fmt.Sprint(hit.Source["name"]), hit.ID)
		lines = append(lines, fmt.Sprintf("- [%s] %s (id=%s, score=%.2f)", strings.ToUpper(hit.Kind), title, hit.ID, hit.Score))
		if evidence := hitEvidenceSummary(hit.Source); evidence != "" {
			lines = append(lines, "  evidence: "+evidence)
		}
	}
	return truncate(strings.Join(lines, "\n"), s.cfg.MaxContextChars)
}
func hitEvidenceSummary(source map[string]any) string {
	if len(source) == 0 {
		return ""
	}
	parts := make([]string, 0, 10)
	appendField := func(label string, keys ...string) {
		value := searchSourceField(source, keys...)
		if value == "" {
			return
		}
		parts = append(parts, fmt.Sprintf("%s=%s", label, value))
	}
	appendSnippet := func(label string, maxChars int, keys ...string) {
		value := compactWhitespace(searchSourceField(source, keys...))
		if value == "" {
			return
		}
		parts = append(parts, fmt.Sprintf("%s=%q", label, truncate(value, maxChars)))
	}
	appendField("case_number", "case_number")
	appendField("status", "status")
	appendField("severity", "severity")
	appendField("priority", "priority")
	appendField("incident_type", "incident_type")
	appendField("source", "source")
	appendField("updated_at", "updated_at", "created_at", "timestamp")
	appendSnippet("description", 220, "description", "summary")
	appendSnippet("value", 180, "value", "name")
	appendSnippet("body", 220, "body", "content")
	return strings.Join(parts, "; ")
}
func searchSourceField(source map[string]any, keys ...string) string {
	for _, key := range keys {
		raw, ok := source[key]
		if !ok || raw == nil {
			continue
		}
		value := strings.TrimSpace(fmt.Sprint(raw))
		if value == "" || value == "<nil>" || value == "[]" || value == "map[]" {
			continue
		}
		return value
	}
	return ""
}
func compactWhitespace(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return strings.Join(strings.Fields(value), " ")
}
func (s *Service) caseContextPayload(input CaseAnalysisInput, hits []search.Hit) string {
	type payload struct {
		CaseNumber   string                   `json:"case_number"`
		Title        string                   `json:"title"`
		Description  string                   `json:"description"`
		Severity     string                   `json:"severity"`
		Status       string                   `json:"status"`
		Priority     string                   `json:"priority"`
		IncidentType string                   `json:"incident_type"`
		TLP          string                   `json:"tlp"`
		PAP          string                   `json:"pap"`
		Tasks        []CaseAnalysisTask       `json:"tasks"`
		Observables  []CaseAnalysisObservable `json:"observables"`
		Events       []CaseAnalysisEvent      `json:"events"`
		Pages        []CaseAnalysisPage       `json:"pages"`
		Comments     []string                 `json:"comments"`
		Sources      []map[string]any         `json:"related_sources"`
	}
	related := make([]map[string]any, 0, minInt(len(hits), s.cfg.TopK))
	for i := 0; i < len(hits) && i < s.cfg.TopK; i++ {
		related = append(related, map[string]any{
			"kind":  hits[i].Kind,
			"id":    hits[i].ID,
			"score": math.Round(hits[i].Score*100) / 100,
			"title": firstNonEmpty(fmt.Sprint(hits[i].Source["title"]), fmt.Sprint(hits[i].Source["name"])),
		})
	}
	body := payload{
		CaseNumber:   input.CaseNumber,
		Title:        input.Title,
		Description:  input.Description,
		Severity:     input.Severity,
		Status:       input.Status,
		Priority:     input.Priority,
		IncidentType: input.IncidentType,
		TLP:          input.TLP,
		PAP:          input.PAP,
		Tasks:        trimTasks(input.Tasks, 20),
		Observables:  trimObservables(input.Observables, 30),
		Events:       trimEvents(input.Events, 20),
		Pages:        trimPages(input.Pages, 10),
		Comments:     trimStringSlice(input.Comments, 20),
		Sources:      related,
	}
	raw, _ := json.Marshal(body)
	return truncate(string(raw), s.cfg.MaxContextChars)
}
func parseCaseAnalysisModelOutput(input string) (caseAnalysisLLMResponse, error) {
	return parseCaseAnalysisModelOutputFlexible(input)
}
func fallbackCaseAnalysis(input CaseAnalysisInput, sources []Source, language string) CaseAnalysisResult {
	language = normalizeResponseLanguage(language)
	verdict := "unknown"
	summary := "Insufficient data for an unambiguous classification."
	if language == "ru" {
		summary = "Недостаточно данных для однозначной классификации."
	}
	confidence := 35.0
	findings := []string{}
	severity := strings.ToLower(strings.TrimSpace(input.Severity))
	if severity == "critical" || severity == "high" {
		verdict = "suspicious"
		confidence = 68
		if language == "ru" {
			summary = "Критичность и контекст кейса указывают на повышенный риск инцидента и требуют немедленного триажа."
		} else {
			summary = "Case severity and context indicate elevated incident risk and require immediate triage."
		}
	}
	maliciousObs := 0
	for _, obs := range input.Observables {
		if strings.EqualFold(strings.TrimSpace(obs.Verdict), "malicious") {
			maliciousObs++
		}
	}
	if maliciousObs > 0 {
		verdict = "malicious"
		confidence = clampFloat(72+float64(maliciousObs*4), 0, 95)
		if language == "ru" {
			summary = "В контексте кейса обнаружены наблюдаемые с вредоносным вердиктом."
			findings = append(findings, fmt.Sprintf("Обнаружено наблюдаемых с вердиктом malicious: %d.", maliciousObs))
		} else {
			summary = "Malicious observables were found in case context."
			findings = append(findings, fmt.Sprintf("Malicious observables detected: %d.", maliciousObs))
		}
	}
	if len(input.Tasks) > 0 {
		openMandatory := 0
		for _, task := range input.Tasks {
			if task.Mandatory && !strings.EqualFold(strings.TrimSpace(task.Status), "done") {
				openMandatory++
			}
		}
		if openMandatory > 0 {
			if language == "ru" {
				findings = append(findings, fmt.Sprintf("Открытых обязательных задач: %d.", openMandatory))
			} else {
				findings = append(findings, fmt.Sprintf("Open mandatory tasks: %d.", openMandatory))
			}
		}
	}
	if len(sources) > 0 {
		if language == "ru" {
			findings = append(findings, fmt.Sprintf("В индексе тенанта найдено связанных записей: %d.", len(sources)))
		} else {
			findings = append(findings, fmt.Sprintf("Related tenant index records found: %d.", len(sources)))
		}
		confidence = clampFloat(confidence+float64(minInt(len(sources), 4))*3, 0, 96)
	}
	recommendations := []string{
		"Refine impact scope using observables and correlated alerts.",
		"Complete mandatory containment and evidence tasks first.",
		"Before closure, document actions and residual risk on case pages.",
	}
	if language == "ru" {
		recommendations = []string{
			"Уточните зону воздействия по наблюдаемым и коррелирующим алертам.",
			"Сначала завершите обязательные задачи по сдерживанию и сбору артефактов.",
			"Перед закрытием зафиксируйте меры и остаточный риск в страницах кейса.",
		}
	}
	if strings.Contains(strings.ToLower(input.Title), "phish") || strings.Contains(strings.ToLower(input.IncidentType), "phish") {
		if language == "ru" {
			recommendations = []string{
				"Отзовите подозрительные сессии и сбросьте затронутые учетные данные.",
				"Проверьте связанные индикаторы отправителя/домена по системам тенанта.",
				"Проведите ревизию правил почтовых ящиков и OAuth-доступов для затронутых учетных записей.",
			}
		} else {
			recommendations = []string{
				"Revoke suspicious sessions and reset affected credentials.",
				"Check sender/domain indicators across tenant systems.",
				"Audit mailbox rules and OAuth grants for affected accounts.",
			}
		}
	}
	return CaseAnalysisResult{
		Model:           "sec-fallback-rag",
		Verdict:         verdict,
		Confidence:      confidence,
		Summary:         summary,
		Recommendations: recommendations,
		Findings:        trimStringSlice(findings, 10),
		Sources:         sources,
	}
}
func resolveMessageLanguage(question, fallback string) string {
	detected := detectLanguageFromText(question)
	if detected != "" {
		return detected
	}
	return normalizeResponseLanguage(fallback)
}
func detectLanguageFromText(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	for _, r := range trimmed {
		if r >= 0x0400 && r <= 0x04FF {
			return "ru"
		}
	}
	return "en"
}
func sanitizeModelOutput(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	clean := strings.TrimSpace(thinkBlockRegex.ReplaceAllString(trimmed, ""))
	if clean == "" {
		return ""
	}
	// Unwrap a single fenced block to keep parser stable.
	if strings.HasPrefix(clean, "```") && strings.HasSuffix(clean, "```") {
		lines := strings.Split(clean, "\n")
		if len(lines) >= 2 {
			clean = strings.Join(lines[1:len(lines)-1], "\n")
			clean = strings.TrimSpace(clean)
		}
	}
	return clean
}
func fallbackErrorText(question, reason, language string) string {
	language = normalizeResponseLanguage(language)
	questionLabel := truncate(strings.TrimSpace(question), 160)
	if questionLabel == "" {
		if language == "ru" {
			questionLabel = "без уточненного вопроса"
		} else {
			questionLabel = "without a specific question"
		}
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = localizedReasonInvalidModelOutput(language)
	}
	if language == "ru" {
		return fmt.Sprintf(
			"Ошибка AI по запросу %q: %s. Проверьте подключение модели и повторите запрос.",
			questionLabel,
			reason,
		)
	}
	return fmt.Sprintf(
		"AI error for request %q: %s. Check model connectivity and retry.",
		questionLabel,
		reason,
	)
}
func buildAnswerText(question string, sources []Source, tenantContextPrompt, language string) (answer string, recommendations []string) {
	language = normalizeResponseLanguage(language)
	questionLabel := truncate(strings.TrimSpace(question), 180)
	if questionLabel == "" {
		if language == "ru" {
			questionLabel = "без уточненного вопроса"
		} else {
			questionLabel = "without a specific question"
		}
	}
	opContext := extractOperationalContext(tenantContextPrompt)
	if questionWantsCasesInWork(questionLabel) {
		if count, ok := opContext.casesInWork(); ok {
			if language == "ru" {
				return fmt.Sprintf(
					"Сейчас в работе %d %s (по операционному контексту тенанта).",
					count,
					russianCasesLabel(count),
				), []string{}
			}
			return fmt.Sprintf(
				"%d active %s in progress (from tenant operational context).",
				count,
				englishCasesLabel(count),
			), []string{}
		}
		if count, ok := casesInWorkFromSourceTitles(sources); ok {
			if language == "ru" {
				return fmt.Sprintf(
					"Сейчас в работе %d %s (по последнему доступному контексту источников).",
					count,
					russianCasesLabel(count),
				), []string{}
			}
			return fmt.Sprintf(
				"%d active %s in progress (from latest indexed sources).",
				count,
				englishCasesLabel(count),
			), []string{}
		}
	}
	if len(sources) == 0 {
		if language == "ru" {
			return fmt.Sprintf(
				"По вопросу %q нет прямых совпадений в данных тенанта. Уточните IOC, номер кейса, источник алерта или диапазон времени.",
				questionLabel,
			), baseRecommendations(question)
		}
		return fmt.Sprintf(
			"No direct matches were found for %q in tenant data. Refine IOC, case number, alert source, or time range.",
			questionLabel,
		), baseRecommendations(question)
	}
	preview := make([]string, 0, minInt(len(sources), 3))
	for i := 0; i < len(sources) && i < 3; i++ {
		title := strings.TrimSpace(sources[i].Title)
		if title == "" {
			title = strings.TrimSpace(sources[i].ID)
		}
		preview = append(preview, fmt.Sprintf("%s: %s", strings.ToUpper(strings.TrimSpace(sources[i].Kind)), title))
	}
	recommendations = baseRecommendations(question)
	nextSteps := make([]string, 0, 2)
	for i := 0; i < len(recommendations) && i < 2; i++ {
		nextSteps = append(nextSteps, fmt.Sprintf("%d) %s", i+1, recommendations[i]))
	}
	if language == "ru" {
		return fmt.Sprintf(
			"По вопросу %q найдено %d связанных записей (%s). Ключевые совпадения: %s. Рекомендуемые шаги: %s",
			questionLabel,
			len(sources),
			summarizeSourceKinds(sources),
			strings.Join(preview, "; "),
			strings.Join(nextSteps, " "),
		), recommendations
	}
	return fmt.Sprintf(
		"For %q found %d related records (%s). Key matches: %s. Recommended steps: %s",
		questionLabel,
		len(sources),
		summarizeSourceKinds(sources),
		strings.Join(preview, "; "),
		strings.Join(nextSteps, " "),
	), recommendations
}
func operationalContextAnswer(question string, sources []Source, tenantContextPrompt, language string) (answer string, recommendations []string, ok bool) {
	if !questionWantsCasesInWork(question) {
		return "", nil, false
	}
	answer, recommendations = buildAnswerText(question, sources, tenantContextPrompt, language)
	return answer, recommendations, strings.TrimSpace(answer) != ""
}
func shouldUseConversationalPrompt(question string) bool {
	q := strings.ToLower(strings.TrimSpace(question))
	if q == "" {
		return false
	}
	if questionWantsCasesInWork(q) {
		return false
	}
	if containsAny(q, "привет", "hello", "hi", "hey", "thanks", "спасибо", "кто ты", "who are you", "help", "помощ") {
		return true
	}
	if containsAny(
		q,
		"miter", "ioc", "alert", "case", "incident", "investigate", "investigation", "triage",
		"login", "credential", "compromise", "endpoint", "threat", "suspicious", "malware", "ransom", "edr", "siem",
		"инцид", "кейс", "расслед", "триаж", "логин", "компром", "угроз", "подозр", "малвар", "фиш",
	) {
		return false
	}
	return len(strings.Fields(q)) <= 8
}
func buildConversationalAskPrompt(question, elasticContextText, postgresContextText, language string) string {
	language = normalizeResponseLanguage(language)
	builder := strings.Builder{}
	builder.WriteString("User request:\n")
	builder.WriteString(strings.TrimSpace(question))
	builder.WriteString("\n\n")
	if language == "ru" {
		builder.WriteString("Ответь только на русском языке. Пиши естественно и по делу, без JSON и без повторяющихся шаблонов.\n\n")
	} else {
		builder.WriteString("Respond only in English. Keep the answer natural, concise, and avoid repetitive templates. Do not return JSON.\n\n")
	}
	builder.WriteString("Available tenant context (Elasticsearch):\n")
	builder.WriteString(strings.TrimSpace(elasticContextText))
	builder.WriteString("\n\n")
	if strings.TrimSpace(postgresContextText) != "" {
		builder.WriteString("Available tenant context (PostgreSQL):\n")
		builder.WriteString(strings.TrimSpace(postgresContextText))
		builder.WriteString("\n\n")
	}
	builder.WriteString("If context is missing, explicitly say what data is missing.")
	return builder.String()
}
func localizedReasonModelUnavailable(language string) string {
	if normalizeResponseLanguage(language) == "ru" {
		return "модель недоступна или не настроена"
	}
	return "model is unavailable or not configured"
}
func localizedReasonAIModuleDisabled(language string) string {
	if normalizeResponseLanguage(language) == "ru" {
		return "AI модуль отключен в конфигурации"
	}
	return "AI module is disabled in configuration"
}
func localizedReasonModelRequestFailed(language string) string {
	if normalizeResponseLanguage(language) == "ru" {
		return "не удалось получить ответ от модели"
	}
	return "failed to get response from model"
}
func localizedReasonEmptyModelAnswer(language string) string {
	if normalizeResponseLanguage(language) == "ru" {
		return "модель вернула пустой ответ"
	}
	return "model returned an empty response"
}
func localizedReasonLowQualityAnswer(language string) string {
	if normalizeResponseLanguage(language) == "ru" {
		return "модель вернула шаблонный/низкокачественный ответ"
	}
	return "model returned a templated or low-quality response"
}
func localizedReasonInvalidModelOutput(language string) string {
	if normalizeResponseLanguage(language) == "ru" {
		return "модель не вернула валидный ответ"
	}
	return "model did not return a valid answer"
}

type operationalContext struct {
	CasesInWork *int64
	ActiveCases *int64
}

func (o operationalContext) casesInWork() (int64, bool) {
	if o.CasesInWork != nil {
		return *o.CasesInWork, true
	}
	if o.ActiveCases != nil {
		return *o.ActiveCases, true
	}
	return 0, false
}
func extractOperationalContext(prompt string) operationalContext {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return operationalContext{}
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(prompt), &payload); err != nil {
		return operationalContext{}
	}
	result := operationalContext{}
	if value, ok := parseInt64Any(payload["cases_in_work"]); ok {
		result.CasesInWork = &value
	}
	if dashboard, ok := payload["dashboard_stats"].(map[string]any); ok {
		if value, ok := parseInt64Any(dashboard["active_cases"]); ok {
			result.ActiveCases = &value
		}
	}
	return result
}

var sourceCasesInWorkPattern = regexp.MustCompile(`(?i)\bcases_in_work\s*=\s*(\d+)\b`)

func casesInWorkFromSourceTitles(sources []Source) (int64, bool) {
	for _, source := range sources {
		match := sourceCasesInWorkPattern.FindStringSubmatch(source.Title)
		if len(match) < 2 {
			continue
		}
		value, err := strconv.ParseInt(strings.TrimSpace(match[1]), 10, 64)
		if err != nil {
			continue
		}
		return value, true
	}
	return 0, false
}
func parseInt64Any(value any) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int8:
		return int64(typed), true
	case int16:
		return int64(typed), true
	case int32:
		return int64(typed), true
	case int64:
		return typed, true
	case uint:
		if uint64(typed) > uint64(math.MaxInt64) {
			return 0, false
		}
		return int64(typed), true //nolint:gosec // bounded above by MaxInt64 check
	case uint8:
		return int64(typed), true
	case uint16:
		return int64(typed), true
	case uint32:
		return int64(typed), true
	case uint64:
		if typed > uint64(math.MaxInt64) {
			return 0, false
		}
		return int64(typed), true
	case float32:
		return int64(typed), true
	case float64:
		return int64(typed), true
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}
func questionWantsCasesInWork(question string) bool {
	q := strings.ToLower(strings.TrimSpace(question))
	if q == "" {
		return false
	}
	caseMention := containsAny(q, "кейс", "инцидент", "case", "incident")
	countIntent := containsAny(q, "сколько", "количество", "how many", "number of")
	workIntent := containsAny(q, "в работе", "активн", "open", "in work", "active", "opened")
	return caseMention && (countIntent || workIntent)
}
func russianCasesLabel(count int64) string {
	if count%100 >= 11 && count%100 <= 14 {
		return "кейсов"
	}
	switch count % 10 {
	case 1:
		return "кейс"
	case 2, 3, 4:
		return "кейса"
	default:
		return "кейсов"
	}
}
func englishCasesLabel(count int64) string {
	if count == 1 {
		return "case"
	}
	return "cases"
}
func autonomousOutputIsLowQuality(question string, parsed autonomousHuntResponse) bool {
	summary := strings.ToLower(strings.TrimSpace(parsed.ExecutiveSummary))
	firstFinding := ""
	if len(parsed.Findings) > 0 {
		firstFinding = strings.ToLower(strings.TrimSpace(parsed.Findings[0].Description))
	}
	if summary == "" && firstFinding == "" {
		return true
	}
	if strings.Contains(summary, "no additional details") || strings.Contains(summary, "нет дополнительных деталей") {
		return true
	}
	if strings.Contains(firstFinding, "no additional details") || strings.Contains(firstFinding, "нет дополнительных деталей") {
		return true
	}
	if strings.Contains(summary, "нет четких признаков фишинга") || strings.Contains(summary, "no clear signs of phishing") {
		loweredQuestion := strings.ToLower(strings.TrimSpace(question))
		if !containsAny(loweredQuestion, "phish", "фиш", "mail", "почт", "oauth") {
			return true
		}
	}
	recommended := normalizeRecommendedAction(parsed.Recommended)
	confidenceLabel := normalizeConfidenceLabel(parsed.Confidence)
	if strings.EqualFold(recommended, "Ignore") && strings.EqualFold(confidenceLabel, "Low") && len(parsed.Findings) <= 1 {
		return true
	}
	return false
}
func summarizeSourceKinds(sources []Source) string {
	if len(sources) == 0 {
		return "нет источников"
	}
	counts := map[string]int{}
	for _, source := range sources {
		kind := strings.TrimSpace(strings.ToLower(source.Kind))
		if kind == "" {
			kind = "other"
		}
		counts[kind]++
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", key, counts[key]))
	}
	return strings.Join(parts, ", ")
}
func baseRecommendations(question string) []string {
	q := strings.ToLower(question)
	switch {
	case strings.Contains(q, "phish"),
		strings.Contains(q, "mail"),
		strings.Contains(q, "oauth"),
		strings.Contains(q, "фиш"),
		strings.Contains(q, "почт"),
		strings.Contains(q, "письм"):
		return []string{
			"Сбросьте или отзовите затронутые сессии и токены идентификации.",
			"Выполните поиск одинаковых индикаторов отправителя/домена/хеша по всему тенанту.",
			"Проверьте почтовые правила и OAuth-доступы для затронутых пользователей.",
		}
	case strings.Contains(q, "ransom"),
		strings.Contains(q, "encrypt"),
		strings.Contains(q, "extortion"),
		strings.Contains(q, "вымог"),
		strings.Contains(q, "шифров"):
		return []string{
			"Изолируйте затронутые хосты и перекройте пути латерального перемещения.",
			"Сохраните volatile-артефакты и соберите форензик-данные для триажа.",
			"Проверьте целостность резервных копий до любых восстановительных действий.",
		}
	default:
		return []string{
			"Скоррелируйте алерты и кейсы по общим наблюдаемым и таймлайну.",
			"Приоритизируйте кейсы высокой критичности с незавершенными задачами.",
			"Зафиксируйте шаги реагирования и артефакты на странице кейса.",
		}
	}
}
func trimHistory(history []ChatMessage, limit int) []ChatMessage {
	if limit <= 0 || len(history) <= limit {
		return history
	}
	return history[len(history)-limit:]
}
func trimStringSlice(items []string, limit int) []string {
	if len(items) == 0 {
		return []string{}
	}
	out := make([]string, 0, minInt(len(items), limit))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
		if len(out) >= limit {
			break
		}
	}
	return out
}
func trimTasks(items []CaseAnalysisTask, limit int) []CaseAnalysisTask {
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}
func trimObservables(items []CaseAnalysisObservable, limit int) []CaseAnalysisObservable {
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}
func trimEvents(items []CaseAnalysisEvent, limit int) []CaseAnalysisEvent {
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}
func trimPages(items []CaseAnalysisPage, limit int) []CaseAnalysisPage {
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}
func mergeSources(primary []Source, secondary []Source) []Source {
	if len(primary) == 0 && len(secondary) == 0 {
		return []Source{}
	}
	out := make([]Source, 0, len(primary)+len(secondary))
	seen := make(map[string]struct{}, len(primary)+len(secondary))
	appendUnique := func(items []Source) {
		for _, item := range items {
			key := strings.TrimSpace(item.Kind) + "|" + strings.TrimSpace(item.ID)
			if key == "|" {
				continue
			}
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, item)
		}
	}
	appendUnique(primary)
	appendUnique(secondary)
	return out
}
func observableValues(items []CaseAnalysisObservable, limit int) []string {
	if limit <= 0 {
		limit = 5
	}
	out := make([]string, 0, minInt(len(items), limit))
	for _, item := range items {
		v := strings.TrimSpace(item.Value)
		if v == "" {
			continue
		}
		out = append(out, v)
		if len(out) >= limit {
			break
		}
	}
	return out
}
func normalizeVerdict(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "malicious", "suspicious", "benign", "unknown":
		return normalized
	default:
		return "unknown"
	}
}
func clampFloat(value, minValue, maxValue float64) float64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
func truncate(value string, maxLen int) string {
	if maxLen <= 0 {
		return value
	}
	if len(value) <= maxLen {
		return value
	}
	return value[:maxLen]
}
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" && trimmed != "<nil>" {
			return trimmed
		}
	}
	return ""
}
