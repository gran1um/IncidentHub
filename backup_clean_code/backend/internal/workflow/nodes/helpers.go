package nodes

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"incidenthub/backend/internal/workflow"
)

var expressionPattern = regexp.MustCompile(`^\s*([a-zA-Z0-9_.-]+)\s*(==|!=|>=|<=|>|<|contains)\s*(.+?)\s*$`)

func toString(value any) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
}

func toInt(value any, fallback int) int {
	return workflow.IntFromAny(value, fallback)
}

func toBool(value any, fallback bool) bool {
	switch typed := value.(type) {
	case nil:
		return fallback
	case bool:
		return typed
	case string:
		normalized := strings.ToLower(strings.TrimSpace(typed))
		switch normalized {
		case "true", "1", "yes", "y", "on":
			return true
		case "false", "0", "no", "n", "off":
			return false
		default:
			return fallback
		}
	case int:
		return typed != 0
	case int64:
		return typed != 0
	case float64:
		return typed != 0
	case float32:
		return typed != 0
	default:
		return fallback
	}
}

func normalizeLabel(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}

func evaluateConditionExpression(expression string, scope map[string]any) (bool, error) {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return false, nil
	}

	match := expressionPattern.FindStringSubmatch(expression)
	if len(match) != 4 {
		value, ok := workflow.LookupPath(scope, expression)
		if !ok {
			return false, nil
		}
		return truthy(value), nil
	}

	leftPath := strings.TrimSpace(match[1])
	operator := strings.TrimSpace(match[2])
	rightRaw := strings.TrimSpace(match[3])

	leftValue, _ := workflow.LookupPath(scope, leftPath)
	rightValue := parseExpressionOperand(rightRaw, scope)

	switch operator {
	case "==":
		return compareLoose(leftValue, rightValue) == 0, nil
	case "!=":
		return compareLoose(leftValue, rightValue) != 0, nil
	case ">":
		return compareLoose(leftValue, rightValue) > 0, nil
	case ">=":
		return compareLoose(leftValue, rightValue) >= 0, nil
	case "<":
		return compareLoose(leftValue, rightValue) < 0, nil
	case "<=":
		return compareLoose(leftValue, rightValue) <= 0, nil
	case "contains":
		leftText := strings.ToLower(toString(leftValue))
		rightText := strings.ToLower(toString(rightValue))
		if rightText == "" {
			return false, nil
		}
		return strings.Contains(leftText, rightText), nil
	default:
		return false, fmt.Errorf("unsupported operator %q", operator)
	}
}

func parseExpressionOperand(raw string, scope map[string]any) any {
	if strings.HasPrefix(raw, "'") && strings.HasSuffix(raw, "'") && len(raw) >= 2 {
		return raw[1 : len(raw)-1]
	}
	if strings.HasPrefix(raw, "\"") && strings.HasSuffix(raw, "\"") && len(raw) >= 2 {
		return raw[1 : len(raw)-1]
	}
	if normalized := strings.ToLower(strings.TrimSpace(raw)); normalized == "true" {
		return true
	} else if normalized == "false" {
		return false
	} else if parsed, err := strconv.ParseFloat(normalized, 64); err == nil {
		return parsed
	}
	if value, ok := workflow.LookupPath(scope, raw); ok {
		return value
	}
	return raw
}

func compareLoose(left, right any) int {
	leftNum, leftNumOK := toFloat(left)
	rightNum, rightNumOK := toFloat(right)
	if leftNumOK && rightNumOK {
		if leftNum < rightNum {
			return -1
		}
		if leftNum > rightNum {
			return 1
		}
		return 0
	}

	leftText := strings.ToLower(toString(left))
	rightText := strings.ToLower(toString(right))
	if leftText < rightText {
		return -1
	}
	if leftText > rightText {
		return 1
	}
	return 0
}

func toFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case json.Number:
		f, err := typed.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func truthy(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case bool:
		return typed
	case int:
		return typed != 0
	case int64:
		return typed != 0
	case float64:
		return typed != 0
	case float32:
		return typed != 0
	case string:
		normalized := strings.TrimSpace(strings.ToLower(typed))
		return normalized != "" && normalized != "false" && normalized != "0" && normalized != "null"
	default:
		return toString(value) != ""
	}
}
