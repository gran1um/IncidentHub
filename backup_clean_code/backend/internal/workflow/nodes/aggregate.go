package nodes

import (
	"context"
	"math"
	"strings"

	"incidenthub/backend/internal/workflow"
)

type aggregateNode struct{}

func newAggregateNode() workflow.NodeExecutor {
	return aggregateNode{}
}

func (n aggregateNode) Type() string {
	return "aggregate"
}

func (n aggregateNode) Execute(_ context.Context, req workflow.NodeExecuteRequest) (workflow.NodeExecuteResult, error) {
	sourcePath := strings.TrimSpace(toString(req.Node.Config["sourcePath"]))
	if sourcePath == "" {
		sourcePath = strings.TrimSpace(toString(req.Node.Config["itemsPath"]))
	}
	if sourcePath == "" {
		sourcePath = "payload.items"
	}
	valuePath := strings.TrimSpace(toString(req.Node.Config["valuePath"]))

	rawItems := resolveAggregateSource(sourcePath, req.Scope)
	values := extractAggregateValues(rawItems, valuePath)
	operation := normalizeLabel(toString(req.Node.Config["operation"]))
	if operation == "" {
		operation = "count"
	}
	targetKey := strings.TrimSpace(toString(req.Node.Config["targetKey"]))
	if targetKey == "" {
		targetKey = "aggregate_result"
	}

	result := aggregateValues(operation, values, strings.TrimSpace(toString(req.Node.Config["separator"])))

	output := workflow.CopyMap(req.Payload)
	output[targetKey] = result
	output["aggregate_operation"] = operation
	output["aggregate_source_path"] = sourcePath
	output["aggregate_count"] = len(values)
	return workflow.NodeExecuteResult{
		Output: output,
	}, nil
}

func resolveAggregateSource(sourcePath string, scope map[string]any) any {
	if value, ok := workflow.LookupPath(scope, sourcePath); ok {
		return value
	}
	return nil
}

func extractAggregateValues(raw any, valuePath string) []any {
	out := make([]any, 0)
	appendValue := func(value any) {
		if value == nil {
			return
		}
		out = append(out, value)
	}

	switch typed := raw.(type) {
	case []any:
		for _, item := range typed {
			appendValue(resolveAggregateItemValue(item, valuePath))
		}
	case map[string]any:
		appendValue(resolveAggregateItemValue(typed, valuePath))
	default:
		appendValue(resolveAggregateItemValue(typed, valuePath))
	}
	return out
}

func resolveAggregateItemValue(item any, valuePath string) any {
	if strings.TrimSpace(valuePath) == "" {
		return item
	}
	scope := map[string]any{
		"item": item,
	}
	if value, ok := workflow.LookupPath(scope, "item."+valuePath); ok {
		return value
	}
	return nil
}

func aggregateValues(operation string, values []any, separator string) any {
	switch operation {
	case "sum":
		sum := 0.0
		for _, item := range values {
			if parsed, ok := toFloat(item); ok {
				sum += parsed
			}
		}
		return sum
	case "avg", "average":
		sum := 0.0
		count := 0
		for _, item := range values {
			if parsed, ok := toFloat(item); ok {
				sum += parsed
				count++
			}
		}
		if count == 0 {
			return 0.0
		}
		return sum / float64(count)
	case "min":
		best := math.MaxFloat64
		ok := false
		for _, item := range values {
			if parsed, parsedOK := toFloat(item); parsedOK {
				if !ok || parsed < best {
					best = parsed
				}
				ok = true
			}
		}
		if !ok {
			return 0.0
		}
		return best
	case "max":
		best := -math.MaxFloat64
		ok := false
		for _, item := range values {
			if parsed, parsedOK := toFloat(item); parsedOK {
				if !ok || parsed > best {
					best = parsed
				}
				ok = true
			}
		}
		if !ok {
			return 0.0
		}
		return best
	case "concat":
		if separator == "" {
			separator = ", "
		}
		parts := make([]string, 0, len(values))
		for _, item := range values {
			text := strings.TrimSpace(toString(item))
			if text == "" {
				continue
			}
			parts = append(parts, text)
		}
		return strings.Join(parts, separator)
	case "unique":
		seen := map[string]struct{}{}
		unique := make([]string, 0)
		for _, item := range values {
			text := strings.TrimSpace(toString(item))
			if text == "" {
				continue
			}
			if _, exists := seen[text]; exists {
				continue
			}
			seen[text] = struct{}{}
			unique = append(unique, text)
		}
		return unique
	default:
		if operation == "collect" || operation == "array" || operation == "list" {
			return values
		}
		return len(values)
	}
}
