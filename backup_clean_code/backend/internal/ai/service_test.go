package ai

import (
	"context"
	"errors"
	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/metrics"
	"incidenthub/backend/internal/search"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

type fakeRetriever struct {
	enabled bool
	hits    []search.Hit
	err     error
}

func (f fakeRetriever) Search(context.Context, string, string, []string, int) ([]search.Hit, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.hits, nil
}

func (f fakeRetriever) Enabled() bool {
	return f.enabled
}

type contextualFakeRetriever struct {
	enabled        bool
	searchHits     []search.Hit
	recentHits     []search.Hit
	searchCalls    int
	recentCalls    int
	lastSearchQ    string
	lastRecentSize int
}

func (f *contextualFakeRetriever) Search(_ context.Context, _ string, query string, _ []string, _ int) ([]search.Hit, error) {
	f.searchCalls++
	f.lastSearchQ = query
	out := make([]search.Hit, len(f.searchHits))
	copy(out, f.searchHits)
	return out, nil
}

func (f *contextualFakeRetriever) Recent(_ context.Context, _ string, _ []string, size int) ([]search.Hit, error) {
	f.recentCalls++
	f.lastRecentSize = size
	out := make([]search.Hit, len(f.recentHits))
	copy(out, f.recentHits)
	return out, nil
}

func (f *contextualFakeRetriever) Enabled() bool {
	return f != nil && f.enabled
}

type fakeLLM struct {
	response string
	err      error
}

func (f fakeLLM) Chat(context.Context, string, []ChatMessage) (string, error) {
	return f.response, f.err
}

func (f fakeLLM) Ping(context.Context) error {
	return f.err
}

type delayedLLM struct {
	response  string
	pingDelay time.Duration
	chatDelay time.Duration
	err       error
}

func (f delayedLLM) Chat(context.Context, string, []ChatMessage) (string, error) {
	if f.chatDelay > 0 {
		time.Sleep(f.chatDelay)
	}
	return f.response, f.err
}

func (f delayedLLM) Ping(context.Context) error {
	if f.pingDelay > 0 {
		time.Sleep(f.pingDelay)
	}
	return f.err
}

type sequencedLLMCall struct {
	response string
	err      error
}

type sequencedLLM struct {
	calls []sequencedLLMCall
	index int
}

func (s *sequencedLLM) Chat(context.Context, string, []ChatMessage) (string, error) {
	if s == nil || len(s.calls) == 0 {
		return "", nil
	}
	idx := s.index
	if idx >= len(s.calls) {
		idx = len(s.calls) - 1
	}
	item := s.calls[idx]
	s.index++
	return item.response, item.err
}

func (s *sequencedLLM) Ping(context.Context) error {
	return nil
}

type fakeTenantContextProvider struct {
	ctx TenantContext
	err error
}

func (f fakeTenantContextProvider) BuildContext(context.Context, string, string) (TenantContext, error) {
	return f.ctx, f.err
}

func TestAskReturnsSources(t *testing.T) {
	svc := NewService(config.AIConfig{Enabled: true, Model: "sec-mini-rag", TopK: 5}, fakeRetriever{
		enabled: true,
		hits: []search.Hit{
			{
				Kind: "cases",
				ID:   "case-1",
				Source: map[string]any{
					"title": "Suspicious OAuth grant",
				},
			},
		},
	})
	svc.llm = fakeLLM{response: "Model-guided answer"}

	answer, err := svc.Ask(context.Background(), "tenant-1", "oauth phishing")
	if err != nil {
		t.Fatalf("ask failed: %v", err)
	}
	if len(answer.Sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(answer.Sources))
	}
	if answer.Confidence <= 0 {
		t.Fatalf("expected positive confidence, got %.2f", answer.Confidence)
	}
	if len(answer.Recommendations) == 0 {
		t.Fatal("expected recommendations")
	}
}

func TestServiceRecordsPingAndAskAsSeparateOperations(t *testing.T) {
	collector := metrics.New("incidenthub_test")
	metrics.SetGlobal(collector)
	defer metrics.SetGlobal(nil)

	svc := NewService(config.AIConfig{Enabled: true, Model: "sec-mini-rag", TopK: 5}, fakeRetriever{})
	svc.llm = delayedLLM{
		response:  "Model-guided answer",
		pingDelay: 5 * time.Millisecond,
		chatDelay: 25 * time.Millisecond,
	}

	if err := svc.Ping(context.Background()); err != nil {
		t.Fatalf("ping failed: %v", err)
	}
	if _, err := svc.Ask(context.Background(), "tenant-1", "oauth phishing"); err != nil {
		t.Fatalf("ask failed: %v", err)
	}

	if got := testutil.ToFloat64(collector.ModuleOperations.WithLabelValues("ai", "ping", "ok")); got != 1 {
		t.Fatalf("expected ai ping counter = 1, got %v", got)
	}
	if got := testutil.ToFloat64(collector.ModuleOperations.WithLabelValues("ai", "ask", "ok")); got != 1 {
		t.Fatalf("expected ai ask counter = 1, got %v", got)
	}

	snapshot := collector.ModuleOperationSnapshot("ai", 60*time.Second)
	if snapshot.Operations != 2 {
		t.Fatalf("expected 2 ai operations in snapshot, got %d", snapshot.Operations)
	}
	if snapshot.AvgLatencyMs < 10 {
		t.Fatalf("expected snapshot to include measured ping+ask latency, got %.2f", snapshot.AvgLatencyMs)
	}
}

func TestAskUsesLLMResponseWhenAvailable(t *testing.T) {
	svc := NewService(config.AIConfig{
		Enabled:         true,
		Model:           "qwen2.5:0.5b",
		Provider:        "ollama",
		Endpoint:        "http://localhost:11434",
		HistoryMessages: 10,
		TopK:            3,
	}, fakeRetriever{enabled: true})
	svc.llm = fakeLLM{response: "Model-guided answer"}

	answer, err := svc.AskWithHistory(context.Background(), "tenant-1", "what happened?", []ChatMessage{
		{Role: "user", Content: "previous question"},
		{Role: "assistant", Content: "previous answer"},
	})
	if err != nil {
		t.Fatalf("ask with history failed: %v", err)
	}
	if answer.Answer != "Model-guided answer" {
		t.Fatalf("expected model answer, got %q", answer.Answer)
	}
}

func TestAskFallsBackToRecentTenantContextWhenSearchReturnsNoHits(t *testing.T) {
	retriever := &contextualFakeRetriever{
		enabled:    true,
		searchHits: []search.Hit{},
		recentHits: []search.Hit{
			{
				Kind: "cases",
				ID:   "case-99",
				Source: map[string]any{
					"title": "Recent suspicious case",
				},
			},
		},
	}
	svc := NewService(config.AIConfig{Enabled: true, Model: "sec-mini-rag", TopK: 3}, retriever)
	svc.llm = fakeLLM{response: "Model-guided answer"}

	answer, err := svc.Ask(context.Background(), "tenant-1", "hello")
	if err != nil {
		t.Fatalf("ask failed: %v", err)
	}
	if retriever.searchCalls != 1 {
		t.Fatalf("expected 1 search call, got %d", retriever.searchCalls)
	}
	if retriever.recentCalls != 1 {
		t.Fatalf("expected 1 recent fallback call, got %d", retriever.recentCalls)
	}
	if len(answer.Sources) == 0 {
		t.Fatalf("expected sources from recent tenant context fallback")
	}
	if answer.Sources[0].ID != "case-99" {
		t.Fatalf("expected fallback source case-99, got %s", answer.Sources[0].ID)
	}
}

func TestAskContinuesWhenKnowledgeBaseSearchFails(t *testing.T) {
	svc := NewService(config.AIConfig{Enabled: true, Model: "sec-mini-rag", TopK: 3}, fakeRetriever{
		enabled: true,
		err:     errors.New("elasticsearch search error: 404 Not Found"),
	})
	svc.llm = fakeLLM{response: "Model-guided answer"}

	answer, err := svc.Ask(context.Background(), "tenant-1", "hello")
	if err != nil {
		t.Fatalf("expected ask to continue without indexed context, got error: %v", err)
	}
	if answer.Answer != "Model-guided answer" {
		t.Fatalf("expected model answer, got %q", answer.Answer)
	}
	if len(answer.Sources) != 0 {
		t.Fatalf("expected no indexed sources on search failure, got %#v", answer.Sources)
	}
}

func TestAnalyzeCaseFallback(t *testing.T) {
	svc := NewService(config.AIConfig{Enabled: false, Model: "sec-mini-rag"}, fakeRetriever{
		enabled: true,
	})
	result, err := svc.AnalyzeCase(context.Background(), "tenant-1", CaseAnalysisInput{
		Title:    "Critical phishing in finance",
		Severity: "critical",
		Observables: []CaseAnalysisObservable{
			{Type: "email", Value: "attacker@example.com", Verdict: "malicious"},
		},
	})
	if err != nil {
		t.Fatalf("analyze case failed: %v", err)
	}
	if result.Verdict != "malicious" {
		t.Fatalf("expected malicious verdict, got %q", result.Verdict)
	}
	if result.Summary == "" {
		t.Fatal("expected non-empty summary")
	}
}

func TestAnalyzeCaseParsesLLMJSON(t *testing.T) {
	svc := NewService(config.AIConfig{
		Enabled:  true,
		Model:    "qwen2.5:0.5b",
		Provider: "ollama",
		Endpoint: "http://localhost:11434",
	}, fakeRetriever{enabled: true})
	svc.llm = fakeLLM{
		response: `{
			"verdict": "suspicious",
			"confidence": 77.5,
			"summary": "Likely credential abuse.",
			"recommendations": ["Reset sessions"],
			"findings": ["Suspicious OAuth scope"]
		}`,
	}

	result, err := svc.AnalyzeCase(context.Background(), "tenant-1", CaseAnalysisInput{
		Title:       "OAuth abuse",
		Description: "Suspicious application consent observed.",
		Severity:    "high",
	})
	if err != nil {
		t.Fatalf("analyze case failed: %v", err)
	}
	if result.Verdict != "suspicious" {
		t.Fatalf("expected suspicious verdict, got %q", result.Verdict)
	}
	if len(result.Recommendations) == 0 {
		t.Fatal("expected recommendations")
	}
}

func TestAnalyzeCaseParsesMITREMapping(t *testing.T) {
	svc := NewService(config.AIConfig{
		Enabled:  true,
		Model:    "qwen2.5:0.5b",
		Provider: "ollama",
		Endpoint: "http://localhost:11434",
	}, fakeRetriever{enabled: true})
	svc.llm = fakeLLM{
		response: `{
			"verdict": "suspicious",
			"confidence": 77.5,
			"summary": "Likely credential abuse.",
			"recommendations": ["Reset sessions"],
			"findings": ["Suspicious OAuth scope (MITER T1078)"],
			"miter": {
				"tactics": ["initial access"],
				"techniques": ["Valid Accounts (T1078)"]
			}
		}`,
	}

	result, err := svc.AnalyzeCase(context.Background(), "tenant-1", CaseAnalysisInput{
		Title:       "OAuth abuse",
		Description: "Suspicious application consent observed.",
		Severity:    "high",
	})
	if err != nil {
		t.Fatalf("analyze case failed: %v", err)
	}
	if len(result.MITER.Tactics) != 1 || result.MITER.Tactics[0] != "TA0001" {
		t.Fatalf("expected MITER tactic TA0001, got %#v", result.MITER.Tactics)
	}
	if len(result.MITER.Techniques) != 1 || result.MITER.Techniques[0] != "T1078" {
		t.Fatalf("expected MITER technique T1078, got %#v", result.MITER.Techniques)
	}
}

func TestAskIncludesTenantContextProviderSources(t *testing.T) {
	svc := NewService(config.AIConfig{Enabled: true, Model: "sec-mini-rag", TopK: 3}, fakeRetriever{enabled: true})
	svc.llm = fakeLLM{response: "Model-guided answer"}
	svc.SetTenantContextProvider(fakeTenantContextProvider{
		ctx: TenantContext{
			Prompt:  "postgres context",
			Sources: []Source{{Kind: "postgres", ID: "tenant:1", Title: "Tenant operational context"}},
		},
	})

	answer, err := svc.Ask(context.Background(), "tenant-1", "How many active cases?")
	if err != nil {
		t.Fatalf("ask failed: %v", err)
	}
	if strings.TrimSpace(answer.Answer) == "" {
		t.Fatalf("expected non-empty model answer")
	}
	if len(answer.Sources) == 0 {
		t.Fatalf("expected sources to include tenant context source")
	}
	found := false
	for _, src := range answer.Sources {
		if src.Kind == "postgres" && src.ID == "tenant:1" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected tenant context provider source, got %#v", answer.Sources)
	}
}

func TestAskReportsErrorEvenWithOperationalContextWhenLLMUnavailable(t *testing.T) {
	svc := NewService(config.AIConfig{Enabled: false, Model: "sec-mini-rag", TopK: 3}, fakeRetriever{enabled: true})
	svc.SetTenantContextProvider(fakeTenantContextProvider{
		ctx: TenantContext{
			Prompt:  `{"tenant_id":"tenant-1","cases_in_work":2,"dashboard_stats":{"active_cases":5,"alerts_24h":8}}`,
			Sources: []Source{{Kind: "postgres", ID: "tenant:1", Title: "Tenant operational context (cases_in_work=2, alerts_24h=8)"}},
		},
	})

	_, err := svc.AskWithHistoryAndLanguage(context.Background(), "tenant-1", "сколько кейсов сейчас в работе", "ru", nil)
	if err == nil {
		t.Fatal("expected error when AI is disabled")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "disabled") {
		t.Fatalf("expected disabled error marker, got %v", err)
	}
}

func TestAskWithHistoryFallsBackWhenLLMCallFails(t *testing.T) {
	svc := NewService(config.AIConfig{
		Enabled:  true,
		Model:    "gpt-4o-mini",
		Provider: "openai",
	}, fakeRetriever{enabled: true})
	svc.llm = fakeLLM{err: context.DeadlineExceeded}

	_, err := svc.AskWithHistory(context.Background(), "tenant-1", "why did alert spike happen?", nil)
	if err == nil {
		t.Fatal("expected error when llm call fails")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "llm chat request failed") {
		t.Fatalf("expected llm failure marker, got %v", err)
	}
}

func TestAnalyzeCaseReturnsErrorWhenLLMOutputIsInvalid(t *testing.T) {
	svc := NewService(config.AIConfig{
		Enabled:  true,
		Model:    "gpt-4o-mini",
		Provider: "openai",
	}, fakeRetriever{enabled: true})
	svc.llm = fakeLLM{response: "not valid json"}

	_, err := svc.AnalyzeCase(context.Background(), "tenant-1", CaseAnalysisInput{
		Title:       "Suspicious token activity",
		Description: "Multiple token grants from unknown app",
		Severity:    "high",
	})
	if err == nil {
		t.Fatal("expected parse error when llm output is invalid")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "parse llm case analysis output") {
		t.Fatalf("expected parse error marker, got %v", err)
	}
}

func TestAnalyzeCaseReturnsErrorWhenLLMOutputMissingSummary(t *testing.T) {
	svc := NewService(config.AIConfig{
		Enabled:  true,
		Model:    "gpt-4o-mini",
		Provider: "openai",
	}, fakeRetriever{enabled: true})
	svc.llm = fakeLLM{
		response: `{"verdict":"suspicious","confidence":70,"recommendations":["Collect endpoint timeline"],"findings":["Suspicious sign-in chain"]}`,
	}

	_, err := svc.AnalyzeCase(context.Background(), "tenant-1", CaseAnalysisInput{
		Title:       "Suspicious sign-in chain",
		Description: "Multiple sign-ins from impossible geos",
		Severity:    "high",
	})
	if err == nil {
		t.Fatal("expected error when llm case analysis summary is missing")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "missing summary") {
		t.Fatalf("expected missing summary error marker, got %v", err)
	}
}

func TestAnalyzeCaseRepairsLooseJSONOutput(t *testing.T) {
	svc := NewService(config.AIConfig{
		Enabled:  true,
		Model:    "gpt-4o-mini",
		Provider: "openai",
	}, fakeRetriever{enabled: true})
	svc.llm = fakeLLM{
		response: `{
  "verdict":"suspicious",
  "confidence":71,
  "summary":"Observed suspicious process tree	with outbound beaconing.",
  "recommendations":[1. "Isolate affected endpoint",2. "Reset impacted credentials"],
  "findings":[1. "PowerShell spawned from Office macro",2. "Repeated outbound requests to rare domain"]
}`,
	}

	result, err := svc.AnalyzeCase(context.Background(), "tenant-1", CaseAnalysisInput{
		Title:       "Endpoint beaconing pattern",
		Description: "Suspicious process + network chain",
		Severity:    "high",
	})
	if err != nil {
		t.Fatalf("expected parser repair success, got error: %v", err)
	}
	if result.Verdict != "suspicious" {
		t.Fatalf("expected suspicious verdict, got %q", result.Verdict)
	}
	if len(result.Recommendations) < 2 {
		t.Fatalf("expected repaired recommendations, got %v", result.Recommendations)
	}
	if len(result.Findings) < 2 {
		t.Fatalf("expected repaired findings, got %v", result.Findings)
	}
	if strings.TrimSpace(result.Summary) == "" {
		t.Fatalf("expected non-empty summary after repair")
	}
	if strings.Contains(result.Summary, "\t") {
		t.Fatalf("expected summary tabs normalized, got %q", result.Summary)
	}
}

func TestAnalyzeCaseRepairsArrayMissingCommaOutput(t *testing.T) {
	svc := NewService(config.AIConfig{
		Enabled:  true,
		Model:    "gpt-4o-mini",
		Provider: "openai",
	}, fakeRetriever{enabled: true})
	svc.llm = fakeLLM{
		response: `{
  "verdict":"suspicious",
  "confidence":73,
  "summary":"Potential malicious execution chain detected.",
  "recommendations":["Isolate endpoint" "Collect volatile memory"],
  "findings":["Unsigned binary execution" "Repeated callback traffic"]
}`,
	}

	result, err := svc.AnalyzeCase(context.Background(), "tenant-1", CaseAnalysisInput{
		Title:       "Suspicious execution chain",
		Description: "Endpoint telemetry indicates unusual binary execution and callbacks",
		Severity:    "high",
	})
	if err != nil {
		t.Fatalf("expected parser to repair missing array comma, got: %v", err)
	}
	if len(result.Recommendations) < 2 {
		t.Fatalf("expected repaired recommendations array, got %v", result.Recommendations)
	}
	if len(result.Findings) < 2 {
		t.Fatalf("expected repaired findings array, got %v", result.Findings)
	}
}

func TestAskWithHistoryRetriesAfterFirstLLMFailure(t *testing.T) {
	svc := NewService(config.AIConfig{
		Enabled:  true,
		Model:    "gpt-4o-mini",
		Provider: "openai",
	}, fakeRetriever{enabled: true})
	llm := &sequencedLLM{
		calls: []sequencedLLMCall{
			{err: context.DeadlineExceeded},
			{response: "Retry succeeded"},
		},
	}
	svc.llm = llm

	answer, err := svc.AskWithHistory(context.Background(), "tenant-1", "what happened?", nil)
	if err != nil {
		t.Fatalf("expected retry success, got error: %v", err)
	}
	if answer.Answer != "Retry succeeded" {
		t.Fatalf("expected retry response, got %q", answer.Answer)
	}
	if llm.index < 2 {
		t.Fatalf("expected at least two llm attempts, got %d", llm.index)
	}
}

func TestAnalyzeCaseRetriesAfterFirstLLMFailure(t *testing.T) {
	svc := NewService(config.AIConfig{
		Enabled:  true,
		Model:    "gpt-4o-mini",
		Provider: "openai",
	}, fakeRetriever{enabled: true})
	llm := &sequencedLLM{
		calls: []sequencedLLMCall{
			{err: context.DeadlineExceeded},
			{response: `{"verdict":"suspicious","confidence":66,"summary":"Retry summary","recommendations":["Collect host triage"],"findings":["Observed unusual sign-in sequence"]}`},
		},
	}
	svc.llm = llm

	result, err := svc.AnalyzeCase(context.Background(), "tenant-1", CaseAnalysisInput{
		Title:       "Suspicious access burst",
		Description: "Repeated high-risk sign-ins",
		Severity:    "high",
	})
	if err != nil {
		t.Fatalf("expected retry success, got error: %v", err)
	}
	if result.Verdict != "suspicious" {
		t.Fatalf("expected suspicious verdict, got %q", result.Verdict)
	}
	if llm.index < 2 {
		t.Fatalf("expected at least two llm attempts, got %d", llm.index)
	}
}

func TestNewServiceSupportsOpenAIProviderWithAIAPIKey(t *testing.T) {
	svc := NewService(config.AIConfig{
		Enabled:       true,
		Provider:      "openai",
		Endpoint:      "https://api.openai.com/v1",
		APIKey:        "test-key",
		Model:         "gpt-4o-mini",
		OpenAIBaseURL: "https://api.openai.com/v1",
	}, fakeRetriever{enabled: true})
	if svc == nil || svc.llm == nil {
		t.Fatal("expected openai llm to be configured from AI_API_KEY")
	}
}

func TestNewServiceSupportsOpenAIProviderWithLegacyEnvKey(t *testing.T) {
	svc := NewService(config.AIConfig{
		Enabled:       true,
		Provider:      "openai",
		Endpoint:      "",
		OpenAIAPIKey:  "legacy-key",
		OpenAIBaseURL: "https://api.openai.com/v1",
		Model:         "gpt-4o-mini",
	}, fakeRetriever{enabled: true})
	if svc == nil || svc.llm == nil {
		t.Fatal("expected openai llm to be configured from OPENAI_API_KEY")
	}
}

func TestNewServiceAutoDetectsOpenAIProviderFromEndpoint(t *testing.T) {
	svc := NewService(config.AIConfig{
		Enabled:  true,
		Provider: "",
		Endpoint: "http://192.168.31.190:8001/v1/chat/completions",
		APIKey:   "",
		Model:    "qwen",
	}, fakeRetriever{enabled: true})
	if svc == nil || svc.llm == nil {
		t.Fatal("expected llm client to be configured")
	}
	if _, ok := svc.llm.(*OpenAIClient); !ok {
		t.Fatalf("expected openai-compatible llm for endpoint, got %T", svc.llm)
	}
}

func TestAskWithHistoryParsesAutonomousObjectResponse(t *testing.T) {
	svc := NewService(config.AIConfig{
		Enabled:         true,
		Model:           "gpt-4o-mini",
		Provider:        "openai",
		HistoryMessages: 10,
		TopK:            3,
	}, fakeRetriever{enabled: true})
	svc.llm = fakeLLM{
		response: `{
  "executive_summary":"Credential abuse is likely for this tenant user.",
  "confidence":"High",
  "recommended_action":"Escalate",
  "findings":[
    {
      "title":"Impossible travel sign-in pattern",
      "description":"Two successful sign-ins from distant geographies within 5 minutes.",
      "miter":{"id":"T1078"},
      "recommendations":["Reset active sessions"]
    }
  ],
  "next_steps":["Reset active sessions","Review risky sign-ins"]
}`,
	}

	answer, err := svc.AskWithHistory(context.Background(), "tenant-1", "Investigate suspicious login activity", nil)
	if err != nil {
		t.Fatalf("ask with history failed: %v", err)
	}
	if !strings.Contains(answer.Answer, "Summary: Credential abuse is likely for this tenant user.") {
		t.Fatalf("expected formatted summary, got %q", answer.Answer)
	}
	if !strings.Contains(answer.Answer, "MITER T1078") {
		t.Fatalf("expected MITER reference in answer, got %q", answer.Answer)
	}
	if answer.Confidence < 0.75 {
		t.Fatalf("expected high confidence score, got %.2f", answer.Confidence)
	}
	if len(answer.Recommendations) == 0 {
		t.Fatal("expected recommendations from autonomous payload")
	}
}

func TestAskWithHistoryParsesAutonomousArrayResponse(t *testing.T) {
	svc := NewService(config.AIConfig{
		Enabled:         true,
		Model:           "gpt-4o-mini",
		Provider:        "openai",
		HistoryMessages: 10,
		TopK:            3,
	}, fakeRetriever{enabled: true})
	svc.llm = fakeLLM{
		response: `[
  {
    "title":"Suspicious PowerShell execution",
    "description":"Encoded command execution detected on endpoint host-22.",
    "confidence":"Medium",
    "recommendations":["Acquire endpoint timeline", "Isolate host-22"]
  }
]`,
	}

	answer, err := svc.AskWithHistory(context.Background(), "tenant-1", "Is there endpoint compromise?", nil)
	if err != nil {
		t.Fatalf("ask with history failed: %v", err)
	}
	if !strings.Contains(answer.Answer, "Findings:") {
		t.Fatalf("expected formatted findings section, got %q", answer.Answer)
	}
	if len(answer.Recommendations) < 2 {
		t.Fatalf("expected recommendations from findings, got %v", answer.Recommendations)
	}
}

func TestAskWithHistoryUsesDeterministicFallbackForLowQualityAutonomousOutput(t *testing.T) {
	svc := NewService(config.AIConfig{
		Enabled:         true,
		Model:           "gpt-4o-mini",
		Provider:        "openai",
		HistoryMessages: 10,
		TopK:            3,
	}, fakeRetriever{
		enabled: true,
		hits: []search.Hit{
			{
				Kind: "cases",
				ID:   "case-1",
				Source: map[string]any{
					"title": "Privilege escalation investigation",
				},
			},
		},
	})
	svc.llm = fakeLLM{
		response: `{
  "executive_summary":"Нет четких признаков фишинга в последних алертах",
  "confidence":"Low",
  "recommended_action":"Ignore",
  "findings":[{"title":"Finding 1","description":"No additional details."}],
  "next_steps":["Ignore"]
}`,
	}

	_, err := svc.AskWithHistory(context.Background(), "tenant-1", "Какие сейчас самые рискованные кейсы и почему?", nil)
	if err == nil {
		t.Fatal("expected error for low quality output")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "low quality") {
		t.Fatalf("expected low quality marker, got %v", err)
	}
}

func TestAnalyzeCaseParsesAutonomousAlternateSchema(t *testing.T) {
	svc := NewService(config.AIConfig{
		Enabled:  true,
		Model:    "gpt-4o-mini",
		Provider: "openai",
	}, fakeRetriever{enabled: true})
	svc.llm = fakeLLM{
		response: `{
  "classification":"suspicious",
  "confidence_level":"Medium",
  "assessment":"Lateral movement indicators present and require deeper containment checks.",
  "next_steps":["Disable suspicious credentials","Review east-west traffic pivots"],
  "findings":[
    {"title":"SMB pivoting", "description":"Unusual SMB access fan-out from one workstation"}
  ]
}`,
	}

	result, err := svc.AnalyzeCase(context.Background(), "tenant-1", CaseAnalysisInput{
		Title:       "Potential lateral movement",
		Description: "Multiple hosts accessed via SMB from single workstation.",
		Severity:    "high",
	})
	if err != nil {
		t.Fatalf("analyze case failed: %v", err)
	}
	if result.Verdict != "suspicious" {
		t.Fatalf("expected suspicious verdict, got %q", result.Verdict)
	}
	if result.Confidence <= 0 {
		t.Fatalf("expected mapped confidence value, got %.2f", result.Confidence)
	}
	if !strings.Contains(result.Summary, "Lateral movement indicators present") {
		t.Fatalf("expected assessment summary, got %q", result.Summary)
	}
	if len(result.Recommendations) == 0 {
		t.Fatal("expected recommendations from next_steps")
	}
	if len(result.Findings) == 0 {
		t.Fatal("expected findings from alternate schema")
	}
}

func TestSearchContextBlockIncludesEvidenceFields(t *testing.T) {
	svc := NewService(config.AIConfig{
		Enabled:         true,
		Model:           "gpt-4o-mini",
		Provider:        "openai",
		TopK:            3,
		MaxContextChars: 3000,
	}, fakeRetriever{enabled: true})

	block := svc.searchContextBlock([]search.Hit{
		{
			Kind:  "cases",
			ID:    "case-42",
			Score: 2.17,
			Source: map[string]any{
				"title":       "Possible credential abuse",
				"status":      "open",
				"severity":    "high",
				"priority":    "p1",
				"description": "Multiple impossible-travel sign-ins for admin account.",
			},
		},
	})

	if !strings.Contains(block, "status=open") {
		t.Fatalf("expected status evidence in context block, got %q", block)
	}
	if !strings.Contains(block, "severity=high") {
		t.Fatalf("expected severity evidence in context block, got %q", block)
	}
	if !strings.Contains(strings.ToLower(block), "description=") {
		t.Fatalf("expected description evidence in context block, got %q", block)
	}
}

func TestAskWithHistoryAndLanguageFormatsRussianAnswer(t *testing.T) {
	svc := NewService(config.AIConfig{
		Enabled:  true,
		Model:    "gpt-4o-mini",
		Provider: "openai",
		TopK:     3,
	}, fakeRetriever{enabled: true})
	svc.llm = fakeLLM{
		response: `{
  "executive_summary":"Обнаружены признаки компрометации учетной записи.",
  "confidence":"High",
  "recommended_action":"Escalate",
  "findings":[
    {"title":"Impossible travel", "description":"Логины из географически удалённых точек за короткий период.", "miter":{"id":"T1078"}}
  ],
  "next_steps":["Сбросить активные сессии", "Проверить OAuth grants"]
}`,
	}

	answer, err := svc.AskWithHistoryAndLanguage(context.Background(), "tenant-1", "Проведи triage по логинам", "ru", nil)
	if err != nil {
		t.Fatalf("ask with language failed: %v", err)
	}
	if !strings.Contains(answer.Answer, "Сводка:") {
		t.Fatalf("expected russian summary label, got %q", answer.Answer)
	}
	if !strings.Contains(answer.Answer, "Следующие шаги:") {
		t.Fatalf("expected russian next-steps label, got %q", answer.Answer)
	}
}

func TestAskWithHistoryAndLanguageUsesOperationalContextForCaseCount(t *testing.T) {
	svc := NewService(config.AIConfig{
		Enabled:  true,
		Model:    "gpt-4o-mini",
		Provider: "openai",
		TopK:     3,
	}, fakeRetriever{enabled: true})
	svc.llm = fakeLLM{err: context.DeadlineExceeded}
	svc.SetTenantContextProvider(fakeTenantContextProvider{
		ctx: TenantContext{
			Prompt:  `{"cases_in_work":7}`,
			Sources: []Source{{Kind: "postgres", ID: "tenant:1", Title: "Tenant operational context"}},
		},
	})

	_, err := svc.AskWithHistoryAndLanguage(context.Background(), "tenant-1", "сколько кейсов сейчас в работе", "ru", nil)
	if err == nil {
		t.Fatal("expected llm error (no deterministic operational-context fallback)")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "llm chat request failed") {
		t.Fatalf("expected llm error marker, got %v", err)
	}
}

func TestAskWithHistoryAndLanguageOverridesUILanguageByQuestion(t *testing.T) {
	svc := NewService(config.AIConfig{
		Enabled:  true,
		Model:    "gpt-4o-mini",
		Provider: "openai",
		TopK:     3,
	}, fakeRetriever{enabled: true})
	svc.llm = fakeLLM{
		response: `{
  "executive_summary":"Обнаружена подозрительная активность входов.",
  "confidence":"High",
  "recommended_action":"Investigate",
  "findings":[{"title":"Login anomaly","description":"Короткий интервал между входами из разных стран."}],
  "next_steps":["Проверить сессии","Сбросить токены"]
}`,
	}

	answer, err := svc.AskWithHistoryAndLanguage(context.Background(), "tenant-1", "проверь подозрительные входы", "en", nil)
	if err != nil {
		t.Fatalf("ask with language failed: %v", err)
	}
	if !strings.Contains(answer.Answer, "Сводка:") {
		t.Fatalf("expected russian labels to override ui language, got %q", answer.Answer)
	}
}

func TestAskWithHistoryRemovesThinkBlock(t *testing.T) {
	svc := NewService(config.AIConfig{
		Enabled:  true,
		Model:    "gpt-4o-mini",
		Provider: "openai",
		TopK:     3,
	}, fakeRetriever{enabled: true})
	svc.llm = fakeLLM{
		response: `<think>hidden reasoning</think>
{
  "executive_summary":"Credential abuse is likely.",
  "confidence":"High",
  "recommended_action":"Escalate",
  "findings":[{"title":"Impossible travel sign-ins","description":"Two countries in short interval."}],
  "next_steps":["Reset sessions"]
}`,
	}

	answer, err := svc.AskWithHistory(context.Background(), "tenant-1", "Investigate login anomaly", nil)
	if err != nil {
		t.Fatalf("ask with think block failed: %v", err)
	}
	if strings.Contains(strings.ToLower(answer.Answer), "<think>") {
		t.Fatalf("expected think block to be removed, got %q", answer.Answer)
	}
	if !strings.Contains(answer.Answer, "Summary:") {
		t.Fatalf("expected parsed structured answer after think strip, got %q", answer.Answer)
	}
}
