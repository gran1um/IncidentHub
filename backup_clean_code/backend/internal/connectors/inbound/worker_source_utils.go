package inbound

import (
	"encoding/json"
	"fmt"
	"strings"
)

func extractRecords(payload any, arrayPath string) []map[string]any {
	switch typed := payload.(type) {
	case []any:
		return normalizeRecords(typed)
	case map[string]any:
		if strings.TrimSpace(arrayPath) != "" {
			if nested, ok := resolvePath(typed, arrayPath); ok {
				if list, ok := nested.([]any); ok {
					return normalizeRecords(list)
				}
				if single, ok := nested.(map[string]any); ok {
					return []map[string]any{single}
				}
			}
		}
		if list, ok := typed["items"].([]any); ok {
			return normalizeRecords(list)
		}
		if list, ok := typed["records"].([]any); ok {
			return normalizeRecords(list)
		}
		return []map[string]any{typed}
	default:
		return []map[string]any{{"value": typed}}
	}
}

func normalizeRecords(input []any) []map[string]any {
	out := make([]map[string]any, 0, len(input))
	for _, raw := range input {
		switch typed := raw.(type) {
		case map[string]any:
			out = append(out, typed)
		default:
			out = append(out, map[string]any{"value": typed})
		}
	}
	return out
}

func resolvePath(payload map[string]any, path string) (any, bool) {
	current := any(payload)
	for _, segment := range strings.Split(path, ".") {
		key := strings.TrimSpace(segment)
		if key == "" {
			continue
		}
		asMap, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		next, exists := asMap[key]
		if !exists {
			return nil, false
		}
		current = next
	}
	return current, true
}

func parseHeadersMap(data map[string]any) map[string]string {
	out := map[string]string{}
	if raw, ok := data["headers"].(map[string]any); ok {
		for key, value := range raw {
			if strings.TrimSpace(key) == "" {
				continue
			}
			out[key] = fmt.Sprint(value)
		}
	}
	if len(out) == 0 {
		if raw, ok := data["defaultHeaders"].(map[string]any); ok {
			for key, value := range raw {
				if strings.TrimSpace(key) == "" {
					continue
				}
				out[key] = fmt.Sprint(value)
			}
		}
	}
	if len(out) == 0 {
		if rawString, ok := data["defaultHeaders"].(string); ok {
			parsed := map[string]string{}
			if json.Unmarshal([]byte(rawString), &parsed) == nil {
				out = parsed
			}
		}
	}
	return out
}

func extractString(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	raw := strings.TrimSpace(key)
	if raw == "" {
		return ""
	}
	if value, ok := payload[raw]; ok {
		text := strings.TrimSpace(fmt.Sprint(value))
		if text != "" && text != "<nil>" {
			return text
		}
	}
	return ""
}

func firstString(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		value := extractString(payload, key)
		if value != "" {
			return value
		}
	}
	return ""
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
