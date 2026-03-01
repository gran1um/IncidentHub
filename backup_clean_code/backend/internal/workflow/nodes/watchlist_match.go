package nodes

import (
	"context"
	"encoding/json"
	"regexp"
	"sort"
	"strings"

	"incidenthub/backend/internal/workflow"
)

type watchlistMatchNode struct{}

func newWatchlistMatchNode() workflow.NodeExecutor {
	return watchlistMatchNode{}
}

func (n watchlistMatchNode) Type() string {
	return "watchlist_match"
}

func (n watchlistMatchNode) Execute(_ context.Context, req workflow.NodeExecuteRequest) (workflow.NodeExecuteResult, error) {
	sourcePath := strings.TrimSpace(toString(req.Node.Config["sourcePath"]))
	if sourcePath == "" {
		sourcePath = "payload.iocs"
	}
	valueKey := strings.TrimSpace(toString(req.Node.Config["valueKey"]))
	if valueKey == "" {
		valueKey = "value"
	}
	matchMode := normalizeLabel(toString(req.Node.Config["matchMode"]))
	if matchMode == "" {
		matchMode = "exact"
	}
	caseInsensitive := toBool(req.Node.Config["caseInsensitive"], true)
	targetKey := strings.TrimSpace(toString(req.Node.Config["targetKey"]))
	if targetKey == "" {
		targetKey = "watchlist_matches"
	}

	sourceValues := extractListValues(resolveNodePath(req.Scope, sourcePath), valueKey)
	watchlist := normalizeWatchlist(req.Node.Config["watchlist"], req.Scope)

	matches := make([]string, 0)
	matchSet := map[string]struct{}{}
	for _, value := range sourceValues {
		for _, watchValue := range watchlist {
			if !isWatchlistMatch(value, watchValue, matchMode, caseInsensitive) {
				continue
			}
			key := strings.TrimSpace(value)
			if key == "" {
				continue
			}
			if _, exists := matchSet[key]; exists {
				continue
			}
			matchSet[key] = struct{}{}
			matches = append(matches, key)
		}
	}
	sort.Strings(matches)

	output := workflow.CopyMap(req.Payload)
	output[targetKey] = matches
	output["watchlist_match_count"] = len(matches)
	output["watchlist_hit"] = len(matches) > 0
	output["watchlist_source_path"] = sourcePath
	return workflow.NodeExecuteResult{
		Output:    output,
		NextLabel: ternaryLabel(len(matches) > 0, "hit", "miss"),
	}, nil
}

func resolveNodePath(scope map[string]any, path string) any {
	if value, ok := workflow.LookupPath(scope, path); ok {
		return value
	}
	return nil
}

func normalizeWatchlist(raw any, scope map[string]any) []string {
	if raw == nil {
		return nil
	}
	switch typed := raw.(type) {
	case string:
		rendered := strings.TrimSpace(workflow.RenderTemplate(typed, scope))
		if rendered == "" {
			return nil
		}
		var arr []any
		if strings.HasPrefix(rendered, "[") && strings.HasSuffix(rendered, "]") {
			if err := json.Unmarshal([]byte(rendered), &arr); err == nil {
				return extractListValues(arr, "value")
			}
		}
		parts := strings.FieldsFunc(rendered, func(r rune) bool {
			return r == ',' || r == '\n' || r == ';'
		})
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			candidate := strings.TrimSpace(part)
			if candidate != "" {
				out = append(out, candidate)
			}
		}
		return out
	default:
		return extractListValues(raw, "value")
	}
}

func extractListValues(raw any, valueKey string) []string {
	result := make([]string, 0)
	appendString := func(value string) {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return
		}
		result = append(result, trimmed)
	}

	switch typed := raw.(type) {
	case []any:
		for _, item := range typed {
			switch entry := item.(type) {
			case string:
				appendString(entry)
			case map[string]any:
				appendString(toString(entry[valueKey]))
			default:
				appendString(toString(entry))
			}
		}
	case []string:
		for _, item := range typed {
			appendString(item)
		}
	case map[string]any:
		for _, value := range typed {
			appendString(toString(value))
		}
	case string:
		appendString(typed)
	default:
		appendString(toString(typed))
	}
	return dedupeStrings(result)
}

func isWatchlistMatch(left, right, mode string, caseInsensitive bool) bool {
	leftValue := strings.TrimSpace(left)
	rightValue := strings.TrimSpace(right)
	if leftValue == "" || rightValue == "" {
		return false
	}
	if caseInsensitive {
		leftValue = strings.ToLower(leftValue)
		rightValue = strings.ToLower(rightValue)
	}
	switch mode {
	case "contains":
		return strings.Contains(leftValue, rightValue)
	case "regex":
		re, err := regexp.Compile(rightValue)
		if err != nil {
			return false
		}
		return re.MatchString(leftValue)
	default:
		return leftValue == rightValue
	}
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

func ternaryLabel(condition bool, left, right string) string {
	if condition {
		return left
	}
	return right
}
