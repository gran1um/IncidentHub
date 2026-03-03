package ai

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var jsonArrayRegex = regexp.MustCompile(`\[[\s\S]*\]`)
var trailingJSONCommaRegex = regexp.MustCompile(`,\s*([}\]])`)
var arrayOrdinalPrefixRegex = regexp.MustCompile(`([\[,]\s*)(\d+\s*[.\):]\s*)`)
var arrayBulletPrefixRegex = regexp.MustCompile(`([\[,]\s*)([-*]\s+)`)

// Autonomous SOC style prompt set adapted from:
// https://github.com/nicholasmagner/autonomous-soc-analyst
const autonomousThreatHunterSystemPrompt = `
You are a cybersecurity threat hunting AI that supports SOC analysts by identifying suspicious or malicious behavior.

Rules:
- Evidence first. Never invent evidence.
- Use only provided tenant context from PostgreSQL and Elasticsearch.
- Be concise, structured, and actionable.
- Map behavior to MITER ATT&CK when possible.
- If evidence is insufficient, explicitly state data gaps and lower confidence.

Output quality bar:
- Clear summary
- Specific findings with evidence
- Actionable recommendations
- Explicit confidence and rationale
`

const autonomousFormattingInstructions = `
Return findings in JSON.

Preferred object schema:
{
  "executive_summary": "Short summary of what matters most",
  "confidence": "Low|Medium|High",
  "recommended_action": "Investigate|Monitor|Escalate|Ignore",
  "findings": [
    {
      "title": "Short finding title",
      "description": "Why suspicious / relevant",
      "miter": {
        "tactic": "Execution",
        "technique": "T1059",
        "sub_technique": "T1059.001",
        "id": "T1059.001",
        "description": "Technique description"
      },
      "log_lines": ["Evidence line 1", "Evidence line 2"],
      "confidence": "Low|Medium|High",
      "recommendations": ["investigate", "monitor"],
      "indicators_of_compromise": ["hash/ip/domain/account"],
      "tags": ["persistence", "credential access"],
      "notes": "Optional assumptions or caveats"
    }
  ],
  "next_steps": ["Step 1", "Step 2", "Step 3"]
}

Alternative accepted schema:
- array of finding objects (same finding schema as above)

Return only valid JSON.
`

//nolint:gochecknoglobals // Static prompt catalog used by the autonomous SOC agent.
var autonomousHuntPrompts = map[string]string{
	"GeneralThreatHunter": `
You are a top-tier SOC threat hunting analyst.
Detect suspicious behavior, adversary tradecraft, and active risk in tenant telemetry.
Focus on:
- Lateral movement
- Privilege escalation
- Credential abuse
- Command and control
- Persistence
- Data exfiltration
`,
	"CaseInvestigation": `
Analyze case investigation context:
- case lifecycle, status drift, and unresolved critical work
- timeline consistency and evidence sufficiency
- owner/assignee/action gaps
- forum/comments signals that change incident confidence
`,
	"AlertTriage": `
Analyze tenant alerts for triage quality:
- severity/status consistency
- repeated patterns and correlated entities
- alerts that require immediate escalation
`,
	"IdentityAndAccess": `
Focus on identity abuse and authentication anomalies:
- unusual sign-ins, suspicious account behavior, privilege misuse
- token/session abuse patterns
- risky account and role activity
`,
	"NetworkAndC2": `
Focus on network behavior and command-and-control patterns:
- beaconing-like traffic, rare outbound destinations, suspicious ports
- potential staging/exfil paths
`,
	"PersistenceAndEvasion": `
Focus on persistence and defense evasion:
- suspicious startup/service/task/registry-like persistence patterns
- control bypass or stealth indicators in case/alert evidence
`,
	"IOCAndMalware": `
Focus on malware and IOC handling:
- malicious or suspicious observable verdicts
- IOC propagation across alerts/cases
- suspicious files/hashes/domains and blast radius
`,
}

type autonomousHuntRoute struct {
	Profile       string
	LookbackHours int
	Rationale     string
}

type autonomousHuntResponse struct {
	ExecutiveSummary string                  `json:"executive_summary"`
	Confidence       string                  `json:"confidence"`
	Recommended      string                  `json:"recommended_action"`
	Findings         []autonomousHuntFinding `json:"findings"`
	NextSteps        []string                `json:"next_steps"`
}

type autonomousHuntFinding struct {
	Title           string              `json:"title"`
	Description     string              `json:"description"`
	MITER           autonomousHuntMITRE `json:"miter"`
	LogLines        []string            `json:"log_lines"`
	Confidence      string              `json:"confidence"`
	Recommendations []string            `json:"recommendations"`
	IOCs            []string            `json:"indicators_of_compromise"`
	Tags            []string            `json:"tags"`
	Notes           string              `json:"notes"`
}

type autonomousHuntMITRE struct {
	Tactic       string `json:"tactic"`
	Technique    string `json:"technique"`
	SubTechnique string `json:"sub_technique"`
	ID           string `json:"id"`
	Description  string `json:"description"`
}

func selectAutonomousHuntRoute(question string, sources []Source) autonomousHuntRoute {
	q := strings.ToLower(strings.TrimSpace(question))
	kindCounts := make(map[string]int)
	for _, src := range sources {
		kind := strings.ToLower(strings.TrimSpace(src.Kind))
		if kind == "" {
			continue
		}
		kindCounts[kind]++
	}

	switch {
	case containsAny(q, "login", "signin", "auth", "password", "session", "credential", "аккаунт", "аутенти", "логин"):
		return autonomousHuntRoute{Profile: "IdentityAndAccess", LookbackHours: 72, Rationale: "identity_and_access_signals"}
	case containsAny(q, "network", "dns", "ip ", "domain", "c2", "beacon", "exfil", "lateral", "rdp", "psexec", "wmic", "сеть", "эксфиль", "латерал"):
		return autonomousHuntRoute{Profile: "NetworkAndC2", LookbackHours: 48, Rationale: "network_c2_signals"}
	case containsAny(q, "registry", "persistence", "startup", "service", "scheduled task", "schtask", "персист", "реестр"):
		return autonomousHuntRoute{Profile: "PersistenceAndEvasion", LookbackHours: 168, Rationale: "persistence_evasion_signals"}
	case containsAny(q, "ioc", "hash", "malware", "ransom", "file", "файл", "хеш", "малвар", "вымог"):
		return autonomousHuntRoute{Profile: "IOCAndMalware", LookbackHours: 168, Rationale: "ioc_malware_signals"}
	case containsAny(q, "alert", "triage", "алерт", "триаж"):
		return autonomousHuntRoute{Profile: "AlertTriage", LookbackHours: 24, Rationale: "alert_triage_signals"}
	case containsAny(q, "case", "incident", "timeline", "task", "comment", "forum", "кейс", "инцид", "таймлайн", "задач", "коммент"):
		return autonomousHuntRoute{Profile: "CaseInvestigation", LookbackHours: 168, Rationale: "case_investigation_signals"}
	}

	if kindCounts["cases"]+kindCounts["forum_threads"] > 0 {
		return autonomousHuntRoute{Profile: "CaseInvestigation", LookbackHours: 72, Rationale: "source_kinds_case_forum"}
	}
	if kindCounts["alerts"] > 0 {
		return autonomousHuntRoute{Profile: "AlertTriage", LookbackHours: 24, Rationale: "source_kinds_alerts"}
	}
	if kindCounts["observables"] > 0 {
		return autonomousHuntRoute{Profile: "IOCAndMalware", LookbackHours: 96, Rationale: "source_kinds_observables"}
	}
	return autonomousHuntRoute{Profile: "GeneralThreatHunter", LookbackHours: 24, Rationale: "default_general_hunt"}
}

func buildAutonomousAskPrompt(question, elasticContextText, postgresContextText string, route autonomousHuntRoute, sources []Source, language string) string {
	profilePrompt := firstNonEmpty(
		autonomousHuntPrompts[route.Profile],
		autonomousHuntPrompts["GeneralThreatHunter"],
	)
	language = normalizeResponseLanguage(language)
	builder := strings.Builder{}
	builder.WriteString("User request:\n")
	builder.WriteString(strings.TrimSpace(question))
	builder.WriteString("\n\n")
	builder.WriteString("Autonomous routing decision:\n")
	_, _ = fmt.Fprintf(&builder, "- profile: %s\n", route.Profile)
	_, _ = fmt.Fprintf(&builder, "- lookback_hours: %d\n", route.LookbackHours)
	_, _ = fmt.Fprintf(&builder, "- rationale: %s\n\n", route.Rationale)
	if sourceKinds := summarizeSourceKinds(sources); strings.TrimSpace(sourceKinds) != "" {
		builder.WriteString("Retrieved source kinds:\n")
		builder.WriteString("- " + sourceKinds + "\n\n")
	}
	builder.WriteString("Critical response constraints:\n")
	builder.WriteString("- Do not return boilerplate. Tailor conclusions to this specific question and tenant context.\n")
	builder.WriteString("- Reference concrete evidence in findings (id/title/status/severity/value) whenever available.\n")
	builder.WriteString("- If evidence is insufficient, list data gaps explicitly before recommendations.\n")
	builder.WriteString("- Avoid repeating previous recommendations unless they are directly justified by current evidence.\n\n")
	if language == "ru" {
		builder.WriteString("Response language:\n- Write the final answer in Russian.\n\n")
	} else {
		builder.WriteString("Response language:\n- Write the final answer in English.\n\n")
	}
	builder.WriteString("Threat hunt instructions:\n")
	builder.WriteString(strings.TrimSpace(profilePrompt))
	builder.WriteString("\n\nTenant context (Elasticsearch retrieval):\n")
	builder.WriteString(strings.TrimSpace(elasticContextText))
	builder.WriteString("\n\n")
	if strings.TrimSpace(postgresContextText) != "" {
		builder.WriteString("Tenant context (PostgreSQL operational context):\n")
		builder.WriteString(strings.TrimSpace(postgresContextText))
		builder.WriteString("\n\n")
	}
	builder.WriteString("Formatting instructions:\n")
	builder.WriteString(strings.TrimSpace(autonomousFormattingInstructions))
	return builder.String()
}

func parseAutonomousHuntModelOutput(input string) (autonomousHuntResponse, bool) {
	raw := sanitizeModelOutput(input)
	if raw == "" {
		return autonomousHuntResponse{}, false
	}

	// When payload starts with an array we must parse it first;
	// otherwise object regex would capture the first element and lose findings.
	if strings.HasPrefix(raw, "[") {
		if parsed, ok := parseAutonomousHuntArray(raw); ok {
			return parsed, true
		}
		if parsed, ok := parseAutonomousHuntObject(raw); ok {
			return parsed, true
		}
		return autonomousHuntResponse{}, false
	}

	if parsed, ok := parseAutonomousHuntObject(raw); ok {
		return parsed, true
	}
	if parsed, ok := parseAutonomousHuntArray(raw); ok {
		return parsed, true
	}

	return autonomousHuntResponse{}, false
}

func parseAutonomousHuntArray(raw string) (autonomousHuntResponse, bool) {
	matchArray := strings.TrimSpace(jsonArrayRegex.FindString(raw))
	if matchArray == "" {
		return autonomousHuntResponse{}, false
	}
	var findings []autonomousHuntFinding
	if err := json.Unmarshal([]byte(matchArray), &findings); err != nil {
		return autonomousHuntResponse{}, false
	}
	summary := ""
	if len(findings) > 0 {
		summary = firstNonEmpty(strings.TrimSpace(findings[0].Description), strings.TrimSpace(findings[0].Title))
	}
	resp := autonomousHuntResponse{
		ExecutiveSummary: summary,
		Confidence:       strongestConfidenceFromFindings(findings),
		Recommended:      "Investigate",
		Findings:         findings,
		NextSteps:        nextStepsFromFindings(findings),
	}
	return sanitizeAutonomousHuntResponse(resp), true
}

func parseAutonomousHuntObject(raw string) (autonomousHuntResponse, bool) {
	matchObject := strings.TrimSpace(jsonObjectRegex.FindString(raw))
	if matchObject == "" {
		return autonomousHuntResponse{}, false
	}

	var payload autonomousHuntResponse
	if err := json.Unmarshal([]byte(matchObject), &payload); err != nil {
		return autonomousHuntResponse{}, false
	}
	if strings.TrimSpace(payload.ExecutiveSummary) == "" &&
		strings.TrimSpace(payload.Confidence) == "" &&
		strings.TrimSpace(payload.Recommended) == "" &&
		len(payload.Findings) == 0 &&
		len(payload.NextSteps) == 0 {
		return autonomousHuntResponse{}, false
	}
	return sanitizeAutonomousHuntResponse(payload), true
}

func sanitizeAutonomousHuntResponse(in autonomousHuntResponse) autonomousHuntResponse {
	out := autonomousHuntResponse{
		ExecutiveSummary: strings.TrimSpace(in.ExecutiveSummary),
		Confidence:       normalizeConfidenceLabel(in.Confidence),
		Recommended:      normalizeRecommendedAction(in.Recommended),
		Findings:         make([]autonomousHuntFinding, 0, minInt(len(in.Findings), 8)),
		NextSteps:        trimStringSlice(in.NextSteps, 8),
	}

	for i := 0; i < len(in.Findings) && i < 8; i++ {
		item := in.Findings[i]
		out.Findings = append(out.Findings, autonomousHuntFinding{
			Title:           strings.TrimSpace(item.Title),
			Description:     strings.TrimSpace(item.Description),
			MITER:           sanitizeMITRE(item.MITER),
			LogLines:        trimStringSlice(item.LogLines, 6),
			Confidence:      normalizeConfidenceLabel(item.Confidence),
			Recommendations: trimStringSlice(item.Recommendations, 5),
			IOCs:            trimStringSlice(item.IOCs, 12),
			Tags:            trimStringSlice(item.Tags, 8),
			Notes:           strings.TrimSpace(item.Notes),
		})
	}

	if out.ExecutiveSummary == "" {
		if len(out.Findings) > 0 && strings.TrimSpace(out.Findings[0].Description) != "" {
			out.ExecutiveSummary = out.Findings[0].Description
		}
	}
	if out.Confidence == "" {
		out.Confidence = strongestConfidenceFromFindings(out.Findings)
	}
	if out.Recommended == "" {
		out.Recommended = "Investigate"
	}
	if len(out.NextSteps) == 0 {
		out.NextSteps = nextStepsFromFindings(out.Findings)
	}
	return out
}

func sanitizeMITRE(item autonomousHuntMITRE) autonomousHuntMITRE {
	return autonomousHuntMITRE{
		Tactic:       strings.TrimSpace(item.Tactic),
		Technique:    strings.TrimSpace(item.Technique),
		SubTechnique: strings.TrimSpace(item.SubTechnique),
		ID:           strings.TrimSpace(item.ID),
		Description:  strings.TrimSpace(item.Description),
	}
}

func strongestConfidenceFromFindings(findings []autonomousHuntFinding) string {
	level := ""
	for _, item := range findings {
		level = strongerConfidence(level, item.Confidence)
	}
	if level == "" {
		return "Medium"
	}
	return normalizeConfidenceLabel(level)
}

func strongerConfidence(left, right string) string {
	leftScore := confidenceLabelScore(left)
	rightScore := confidenceLabelScore(right)
	if rightScore > leftScore {
		return right
	}
	return left
}

func confidenceLabelScore(label string) int {
	switch normalizeConfidenceLabel(label) {
	case "High":
		return 3
	case "Medium":
		return 2
	case "Low":
		return 1
	default:
		return 0
	}
}

func nextStepsFromFindings(findings []autonomousHuntFinding) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, 6)
	appendUnique := func(value string) {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return
		}
		key := strings.ToLower(trimmed)
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}

	for _, finding := range findings {
		for _, rec := range finding.Recommendations {
			appendUnique(rec)
			if len(out) >= 6 {
				return out
			}
		}
	}
	return out
}

func normalizeConfidenceLabel(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "high", "h", "strong":
		return "High"
	case "medium", "med", "moderate", "mid":
		return "Medium"
	case "low", "l", "weak":
		return "Low"
	default:
		return ""
	}
}

func confidenceScoreFromLabel(label string, sourceCount int) float64 {
	base := 0.55
	switch normalizeConfidenceLabel(label) {
	case "High":
		base = 0.82
	case "Medium":
		base = 0.64
	case "Low":
		base = 0.42
	}
	if sourceCount > 0 {
		base += float64(minInt(sourceCount, 3)) * 0.03
	}
	return clampFloat(base, 0.20, 0.95)
}

func normalizeRecommendedAction(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "investigate", "investigation":
		return "Investigate"
	case "monitor":
		return "Monitor"
	case "escalate", "escalation":
		return "Escalate"
	case "ignore", "no action", "none":
		return "Ignore"
	default:
		return ""
	}
}

func recommendationsFromAutonomousResponse(resp autonomousHuntResponse, fallback []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, 8)

	appendUnique := func(value string) {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return
		}
		key := strings.ToLower(trimmed)
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}

	for _, step := range resp.NextSteps {
		appendUnique(step)
	}
	for _, finding := range resp.Findings {
		for _, rec := range finding.Recommendations {
			appendUnique(rec)
			if len(out) >= 8 {
				return out
			}
		}
	}
	for _, item := range fallback {
		appendUnique(item)
		if len(out) >= 8 {
			break
		}
	}
	return out
}

func formatAutonomousHuntAnswer(resp autonomousHuntResponse, language string) string {
	language = normalizeResponseLanguage(language)
	summaryLabel := "Summary"
	confidenceLabel := "Confidence"
	recommendedLabel := "Recommended action"
	findingsLabel := "Findings"
	nextStepsLabel := "Next steps"
	findingLabel := "Finding"
	noDetailsText := "No additional details."
	if language == "ru" {
		summaryLabel = "Сводка"
		confidenceLabel = "Уверенность"
		recommendedLabel = "Рекомендованное действие"
		findingsLabel = "Наблюдения"
		nextStepsLabel = "Следующие шаги"
		findingLabel = "Наблюдение"
		noDetailsText = "Дополнительных деталей нет."
	}

	lines := []string{
		summaryLabel + ": " + strings.TrimSpace(resp.ExecutiveSummary),
		confidenceLabel + ": " + normalizeConfidenceLabel(resp.Confidence),
		recommendedLabel + ": " + normalizeRecommendedAction(resp.Recommended),
	}

	if len(resp.Findings) > 0 {
		lines = append(lines, findingsLabel+":")
		for i := 0; i < len(resp.Findings) && i < 4; i++ {
			finding := resp.Findings[i]
			title := firstNonEmpty(finding.Title, fmt.Sprintf("%s %d", findingLabel, i+1))
			description := firstNonEmpty(finding.Description, noDetailsText)
			mitreID := firstNonEmpty(finding.MITER.ID, finding.MITER.Technique)
			if strings.TrimSpace(mitreID) != "" {
				lines = append(lines, fmt.Sprintf("%d) %s - %s (MITER %s)", i+1, title, description, mitreID))
			} else {
				lines = append(lines, fmt.Sprintf("%d) %s - %s", i+1, title, description))
			}
		}
	}

	steps := trimStringSlice(resp.NextSteps, 3)
	if len(steps) > 0 {
		lines = append(lines, nextStepsLabel+":")
		for i, step := range steps {
			lines = append(lines, fmt.Sprintf("%d) %s", i+1, step))
		}
	}

	// Keep deterministic stable output for tests and chat rendering.
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func normalizeResponseLanguage(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "ru", "ru-ru", "russian":
		return "ru"
	default:
		return "en"
	}
}

func caseAnalysisAutonomousPrompt(input CaseAnalysisInput, language string) string {
	language = normalizeResponseLanguage(language)
	sections := []string{
		"You are an incident response AI analyst. Perform an evidence-first SOC case assessment.",
		"Map key behaviors to MITER ATT&CK where possible and keep findings concise.",
		"Return only valid JSON with fields:",
		`{"verdict":"malicious|suspicious|benign|unknown","confidence":0..100,"summary":"...","recommendations":["..."],"findings":["..."],"miter":{"tactics":["TA0001"],"techniques":["T1566"]}}`,
		"If MITER is unknown, return empty arrays in mitre.tactics and mitre.techniques.",
		"Use findings to include concise evidence and optional MITER references.",
		"Never include <think> blocks, hidden reasoning, or extra markdown wrappers.",
	}
	if language == "ru" {
		sections = append(sections, "Return summary/recommendations/findings in Russian.")
	} else {
		sections = append(sections, "Return summary/recommendations/findings in English.")
	}

	if strings.TrimSpace(input.Severity) != "" || strings.TrimSpace(input.Status) != "" {
		sections = append(sections, fmt.Sprintf("Case meta: severity=%s status=%s priority=%s", strings.TrimSpace(input.Severity), strings.TrimSpace(input.Status), strings.TrimSpace(input.Priority)))
	}
	return strings.Join(sections, "\n")
}

func parseCaseAnalysisModelOutputFlexible(input string) (caseAnalysisLLMResponse, error) {
	raw := sanitizeModelOutput(input)
	if raw == "" {
		return caseAnalysisLLMResponse{}, fmt.Errorf("empty model output")
	}
	candidates := extractJSONObjectCandidates(raw)
	if len(candidates) == 0 {
		return caseAnalysisLLMResponse{}, fmt.Errorf("json object not found in model output")
	}

	var (
		payload map[string]any
		lastErr error
	)
	for _, candidate := range candidates {
		decoded, err := decodeCaseAnalysisPayload(candidate)
		if err != nil {
			lastErr = err
			continue
		}
		payload = decoded
		break
	}
	if payload == nil {
		if lastErr == nil {
			lastErr = fmt.Errorf("unable to decode model json payload")
		}
		return caseAnalysisLLMResponse{}, lastErr
	}

	out := caseAnalysisLLMResponse{
		Verdict: strings.TrimSpace(firstString(payload, "verdict", "classification", "result")),
		Summary: strings.TrimSpace(firstString(payload, "summary", "executive_summary", "assessment")),
		MITER:   normalizeCaseAnalysisMITRE(payload, normalizeFindings(payload["findings"]), strings.TrimSpace(firstString(payload, "summary", "executive_summary", "assessment"))),
	}
	out.Recommendations = trimStringSlice(anySliceToStrings(payload["recommendations"]), 10)
	if len(out.Recommendations) == 0 {
		out.Recommendations = trimStringSlice(anySliceToStrings(payload["next_steps"]), 10)
	}
	out.Findings = normalizeFindings(payload["findings"])

	if confidenceValue, ok := firstFloat(payload, "confidence"); ok {
		out.Confidence = confidenceValue
	} else {
		out.Confidence = confidencePercentFromLabel(firstString(payload, "confidence_level", "confidence"))
	}
	return out, nil
}

func decodeCaseAnalysisPayload(candidate string) (map[string]any, error) {
	var (
		payload map[string]any
		lastErr error
	)
	attempts := []string{
		strings.TrimSpace(candidate),
	}
	normalized := normalizeModelJSONCandidate(candidate)
	if normalized != attempts[0] {
		attempts = append(attempts, normalized)
	}
	for _, attempt := range attempts {
		if strings.TrimSpace(attempt) == "" {
			continue
		}
		if err := json.Unmarshal([]byte(attempt), &payload); err != nil {
			lastErr = err
			continue
		}
		return payload, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("empty json candidate")
	}
	return nil, lastErr
}

func normalizeModelJSONCandidate(candidate string) string {
	normalized := strings.TrimSpace(candidate)
	if normalized == "" {
		return ""
	}
	normalized = strings.TrimPrefix(normalized, "\uFEFF")
	normalized = strings.ReplaceAll(normalized, "\u00A0", " ")
	normalized = strings.ReplaceAll(normalized, "\t", " ")
	normalized = strings.ReplaceAll(normalized, "\r", "")
	normalized = strings.NewReplacer(
		"“", "\"",
		"”", "\"",
		"‘", "'",
		"’", "'",
	).Replace(normalized)
	normalized = trailingJSONCommaRegex.ReplaceAllString(normalized, "$1")
	normalized = arrayOrdinalPrefixRegex.ReplaceAllString(normalized, "$1")
	normalized = arrayBulletPrefixRegex.ReplaceAllString(normalized, "$1")
	normalized = insertMissingArrayCommas(normalized)
	return strings.TrimSpace(normalized)
}

func insertMissingArrayCommas(input string) string {
	if strings.TrimSpace(input) == "" {
		return input
	}
	var builder strings.Builder
	builder.Grow(len(input) + 16)

	stack := make([]byte, 0, 8)
	inString := false
	escaped := false
	lastNonSpace := byte(0)

	for i := 0; i < len(input); i++ {
		ch := input[i]
		if inString {
			builder.WriteByte(ch)
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
				lastNonSpace = '"'
			}
			continue
		}

		if ch == '"' {
			top := byte(0)
			if len(stack) > 0 {
				top = stack[len(stack)-1]
			}
			if top == '[' && arrayValueTerminator(lastNonSpace) {
				builder.WriteByte(',')
			}
			inString = true
			builder.WriteByte(ch)
			continue
		}

		switch ch {
		case '[', '{':
			stack = append(stack, ch)
		case ']':
			if len(stack) > 0 && stack[len(stack)-1] == '[' {
				stack = stack[:len(stack)-1]
			}
		case '}':
			if len(stack) > 0 && stack[len(stack)-1] == '{' {
				stack = stack[:len(stack)-1]
			}
		}

		builder.WriteByte(ch)
		if !isJSONWhitespace(ch) {
			lastNonSpace = ch
		}
	}
	return builder.String()
}

func isJSONWhitespace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r'
}

func arrayValueTerminator(ch byte) bool {
	switch ch {
	case '"', '}', ']', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '.', '-', 'e', 'E', 'l', 'f', 't':
		return true
	default:
		return false
	}
}

func extractJSONObjectCandidates(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	candidates := make([]string, 0, 3)
	seen := make(map[string]struct{})

	start := -1
	depth := 0
	inString := false
	escaped := false

	appendCandidate := func(value string) {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return
		}
		if _, ok := seen[trimmed]; ok {
			return
		}
		seen[trimmed] = struct{}{}
		candidates = append(candidates, trimmed)
	}

	for i := 0; i < len(raw); i++ {
		ch := raw[i]
		if start < 0 {
			if ch == '{' {
				start = i
				depth = 1
				inString = false
				escaped = false
			}
			continue
		}

		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}

		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				appendCandidate(raw[start : i+1])
				start = -1
			}
		}
	}

	if len(candidates) == 0 {
		appendCandidate(jsonObjectRegex.FindString(raw))
	}
	return candidates
}

func firstString(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		value, exists := payload[key]
		if !exists || value == nil {
			continue
		}
		typed, ok := value.(string)
		if !ok {
			continue
		}
		if strings.TrimSpace(typed) == "" {
			continue
		}
		return typed
	}
	return ""
}

func firstFloat(payload map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		value, exists := payload[key]
		if !exists || value == nil {
			continue
		}
		switch typed := value.(type) {
		case float64:
			return typed, true
		case int:
			return float64(typed), true
		case int64:
			return float64(typed), true
		case string:
			trimmed := strings.TrimSpace(typed)
			if trimmed == "" {
				continue
			}
			var parsed float64
			if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
				return parsed, true
			}
		}
	}
	return 0, false
}

func anySliceToStrings(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return []string{}
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		typed, ok := item.(string)
		if !ok {
			continue
		}
		out = append(out, typed)
	}
	return out
}

func normalizeFindings(value any) []string {
	switch typed := value.(type) {
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			switch v := item.(type) {
			case string:
				out = append(out, strings.TrimSpace(v))
			case map[string]any:
				title := strings.TrimSpace(firstString(v, "title"))
				description := strings.TrimSpace(firstString(v, "description"))
				switch {
				case title != "" && description != "":
					out = append(out, title+": "+description)
				case description != "":
					out = append(out, description)
				case title != "":
					out = append(out, title)
				}
			}
		}
		return trimStringSlice(out, 12)
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return []string{}
		}
		return []string{trimmed}
	default:
		return []string{}
	}
}

func confidencePercentFromLabel(value string) float64 {
	switch normalizeConfidenceLabel(value) {
	case "High":
		return 82
	case "Medium":
		return 62
	case "Low":
		return 38
	default:
		return 55
	}
}

func containsAny(input string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(input, strings.ToLower(strings.TrimSpace(needle))) {
			return true
		}
	}
	return false
}
