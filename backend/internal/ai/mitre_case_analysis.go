package ai

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type MITREMapping struct {
	Tactics    []string `json:"tactics"`
	Techniques []string `json:"techniques"`
}

var (
	mitreTacticIDRegex    = regexp.MustCompile(`(?i)\bTA\d{4}\b`)
	mitreTechniqueIDRegex = regexp.MustCompile(`(?i)\bT\d{4}(?:\.\d{3})?\b`)
)

//nolint:gochecknoglobals // Static ATT&CK mapping table.
var mitreTacticAliases = map[string]string{
	"initial access":       "TA0001",
	"initial-access":       "TA0001",
	"execution":            "TA0002",
	"persistence":          "TA0003",
	"privilege escalation": "TA0004",
	"privilege-escalation": "TA0004",
	"defense evasion":      "TA0005",
	"defense-evasion":      "TA0005",
	"credential access":    "TA0006",
	"credential-access":    "TA0006",
	"discovery":            "TA0007",
	"lateral movement":     "TA0008",
	"lateral-movement":     "TA0008",
	"collection":           "TA0009",
	"exfiltration":         "TA0010",
	"command and control":  "TA0011",
	"command-and-control":  "TA0011",
	"impact":               "TA0040",
	"resource development": "TA0042",
	"resource-development": "TA0042",
	"reconnaissance":       "TA0043",
}

//nolint:gochecknoglobals // Static ATT&CK mapping table.
var mitreTechniqueToTactic = map[string]string{
	"T1566":     "TA0001",
	"T1190":     "TA0001",
	"T1133":     "TA0001",
	"T1078":     "TA0001",
	"T1195":     "TA0001",
	"T1059":     "TA0002",
	"T1059.001": "TA0002",
	"T1204":     "TA0002",
	"T1053":     "TA0002",
	"T1203":     "TA0002",
	"T1547":     "TA0003",
	"T1543":     "TA0003",
	"T1546":     "TA0003",
	"T1548":     "TA0004",
	"T1134":     "TA0004",
	"T1068":     "TA0004",
	"T1070":     "TA0005",
	"T1036":     "TA0005",
	"T1027":     "TA0005",
	"T1562":     "TA0005",
	"T1110":     "TA0006",
	"T1003":     "TA0006",
	"T1555":     "TA0006",
	"T1087":     "TA0007",
	"T1083":     "TA0007",
	"T1057":     "TA0007",
	"T1021":     "TA0008",
	"T1570":     "TA0008",
	"T1080":     "TA0008",
	"T1005":     "TA0009",
	"T1114":     "TA0009",
	"T1074":     "TA0009",
	"T1041":     "TA0010",
	"T1048":     "TA0010",
	"T1567":     "TA0010",
	"T1071":     "TA0011",
	"T1105":     "TA0011",
	"T1572":     "TA0011",
	"T1486":     "TA0040",
	"T1489":     "TA0040",
	"T1490":     "TA0040",
}

func normalizeCaseAnalysisMITRE(payload map[string]any, findings []string, summary string) MITREMapping {
	mitrePayload := firstMITREMap(payload, "miter", "miter_attack", "mitreAttack")
	rawTactics := make([]string, 0, 4)
	rawTechniques := make([]string, 0, 6)

	rawTactics = append(rawTactics,
		mitreStringsFromValue(firstMITREValue(mitrePayload, "tactics", "tactic_ids", "tacticIds", "ta_ids", "taIds"))...,
	)
	rawTechniques = append(rawTechniques,
		mitreStringsFromValue(firstMITREValue(mitrePayload, "techniques", "technique_ids", "techniqueIds", "ids"))...,
	)

	if value := firstMITREString(mitrePayload, "tactic", "tactic_id", "tacticId"); value != "" {
		rawTactics = append(rawTactics, value)
	}
	if value := firstMITREString(mitrePayload, "technique", "technique_id", "techniqueId", "id", "sub_technique", "subTechnique"); value != "" {
		rawTechniques = append(rawTechniques, value)
	}
	if value := firstMITREString(payload, "miter_tactic", "mitreTactic", "tactic"); value != "" {
		rawTactics = append(rawTactics, value)
	}
	if value := firstMITREString(payload, "miter_technique", "mitreTechnique", "technique", "sub_technique", "subTechnique"); value != "" {
		rawTechniques = append(rawTechniques, value)
	}
	rawTactics = append(rawTactics, mitreStringsFromValue(firstMITREValue(payload, "miter_tactics", "mitreTactics"))...)
	rawTechniques = append(rawTechniques, mitreStringsFromValue(firstMITREValue(payload, "miter_techniques", "mitreTechniques"))...)

	techniques := normalizeMITRETechniques(rawTechniques)
	if len(techniques) == 0 {
		texts := append([]string{}, findings...)
		texts = append(texts, summary)
		techniques = extractMITRETechniquesFromText(texts...)
	}
	tactics := normalizeMITRETactics(rawTactics, techniques)
	return MITREMapping{Tactics: tactics, Techniques: techniques}
}

func firstMITREMap(payload map[string]any, keys ...string) map[string]any {
	for _, key := range keys {
		if payload == nil {
			return nil
		}
		value, ok := payload[key]
		if !ok || value == nil {
			continue
		}
		if item, ok := value.(map[string]any); ok {
			return item
		}
	}
	return nil
}

func firstMITREValue(payload map[string]any, keys ...string) any {
	for _, key := range keys {
		if payload == nil {
			return nil
		}
		if value, ok := payload[key]; ok {
			return value
		}
	}
	return nil
}

func firstMITREString(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		if payload == nil {
			return ""
		}
		value, ok := payload[key]
		if !ok || value == nil {
			continue
		}
		if typed, ok := value.(string); ok {
			trimmed := strings.TrimSpace(typed)
			if trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

func mitreStringsFromValue(value any) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string{}, typed...)
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			text := strings.TrimSpace(fmt.Sprint(item))
			if text != "" && text != "<nil>" {
				out = append(out, text)
			}
		}
		return out
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return nil
		}
		parts := strings.FieldsFunc(trimmed, func(r rune) bool {
			return r == ',' || r == ';' || r == '\n'
		})
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			text := strings.TrimSpace(part)
			if text != "" {
				out = append(out, text)
			}
		}
		if len(out) == 0 {
			return []string{trimmed}
		}
		return out
	default:
		return nil
	}
}

func normalizeMITRETactics(values []string, techniques []string) []string {
	seen := make(map[string]struct{}, len(values)+len(techniques))
	out := make([]string, 0, len(values)+len(techniques))
	appendValue := func(candidate string) {
		normalized := normalizeMITRETactic(candidate)
		if normalized == "" {
			return
		}
		if _, exists := seen[normalized]; exists {
			return
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	for _, value := range values {
		appendValue(value)
	}
	for _, technique := range techniques {
		appendValue(mitreTechniqueToTactic[normalizeMITRETechnique(technique)])
	}
	sort.Strings(out)
	return out
}

func normalizeMITRETactic(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if match := mitreTacticIDRegex.FindString(strings.ToUpper(trimmed)); match != "" {
		return strings.ToUpper(match)
	}
	normalized := strings.ToLower(trimmed)
	if mapped, ok := mitreTacticAliases[normalized]; ok {
		return mapped
	}
	return ""
}

func normalizeMITRETechniques(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized := normalizeMITRETechnique(value)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	sort.Strings(out)
	return out
}

func normalizeMITRETechnique(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if match := mitreTechniqueIDRegex.FindString(strings.ToUpper(trimmed)); match != "" {
		return strings.ToUpper(match)
	}
	return ""
}

func extractMITRETechniquesFromText(values ...string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		for _, match := range mitreTechniqueIDRegex.FindAllString(strings.ToUpper(value), -1) {
			normalized := normalizeMITRETechnique(match)
			if normalized == "" {
				continue
			}
			if _, exists := seen[normalized]; exists {
				continue
			}
			seen[normalized] = struct{}{}
			out = append(out, normalized)
		}
	}
	sort.Strings(out)
	return out
}
