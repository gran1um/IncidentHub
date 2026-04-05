package workflow

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

var templateTokenPattern = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_.-]+)\s*\}\}`)

func copyMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = copyValue(value)
	}
	return out
}

func copyValue(input any) any {
	switch typed := input.(type) {
	case map[string]any:
		return copyMap(typed)
	case []any:
		out := make([]any, 0, len(typed))
		for _, value := range typed {
			out = append(out, copyValue(value))
		}
		return out
	default:
		return typed
	}
}

func stringFromMap(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		if payload == nil {
			break
		}
		if value, ok := payload[key]; ok {
			text := strings.TrimSpace(fmt.Sprint(value))
			if text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}

func renderTemplate(input string, scope map[string]any) string {
	if strings.TrimSpace(input) == "" || len(scope) == 0 {
		return input
	}
	return templateTokenPattern.ReplaceAllStringFunc(input, func(raw string) string {
		matches := templateTokenPattern.FindStringSubmatch(raw)
		if len(matches) != 2 {
			return raw
		}
		path := strings.TrimSpace(matches[1])
		if path == "" {
			return raw
		}
		value, ok := lookupPath(scope, path)
		if !ok || value == nil {
			return ""
		}
		switch typed := value.(type) {
		case string:
			return typed
		default:
			return fmt.Sprint(value)
		}
	})
}

func lookupPath(scope map[string]any, path string) (any, bool) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, false
	}
	parts := strings.Split(path, ".")
	var current any = scope
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, false
		}
		switch typed := current.(type) {
		case map[string]any:
			value, ok := typed[part]
			if !ok {
				return nil, false
			}
			current = value
		case []any:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(typed) {
				return nil, false
			}
			current = typed[index]
		default:
			value := reflect.ValueOf(current)
			if value.Kind() == reflect.Pointer {
				if value.IsNil() {
					return nil, false
				}
				value = value.Elem()
			}
			if value.Kind() != reflect.Struct {
				return nil, false
			}
			field := value.FieldByName(part)
			if !field.IsValid() || !field.CanInterface() {
				return nil, false
			}
			current = field.Interface()
		}
	}
	return current, true
}

func intFromAny(value any, fallback int) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func boolFromAny(value any, fallback bool) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		normalized := strings.TrimSpace(strings.ToLower(typed))
		switch normalized {
		case "1", "true", "yes", "y", "on":
			return true
		case "0", "false", "no", "n", "off":
			return false
		}
	}
	return fallback
}

func CopyMap(input map[string]any) map[string]any {
	return copyMap(input)
}

func RenderTemplate(input string, scope map[string]any) string {
	return renderTemplate(input, scope)
}

func LookupPath(scope map[string]any, path string) (any, bool) {
	return lookupPath(scope, path)
}

func IntFromAny(value any, fallback int) int {
	return intFromAny(value, fallback)
}

func BoolFromAny(value any, fallback bool) bool {
	return boolFromAny(value, fallback)
}
