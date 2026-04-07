package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"incidenthub/backend/internal/workflow"
)

type switchNode struct{}

func newSwitchNode() workflow.NodeExecutor {
	return switchNode{}
}

func (n switchNode) Type() string {
	return "switch"
}

func (n switchNode) Execute(_ context.Context, req workflow.NodeExecuteRequest) (workflow.NodeExecuteResult, error) {
	sourcePath := strings.TrimSpace(toString(req.Node.Config["sourcePath"]))
	if sourcePath == "" {
		sourcePath = strings.TrimSpace(toString(req.Node.Config["expression"]))
	}
	if sourcePath == "" {
		sourcePath = "payload"
	}

	cases, err := parseSwitchCases(req.Node.Config["cases"])
	if err != nil {
		return workflow.NodeExecuteResult{}, err
	}
	if len(cases) == 0 {
		return workflow.NodeExecuteResult{}, fmt.Errorf("switch cases are required")
	}

	sourceValue := resolveSwitchSourceValue(sourcePath, req.Scope)
	sourceValueText := toString(sourceValue)
	defaultLabel := strings.TrimSpace(toString(req.Node.Config["defaultLabel"]))
	if defaultLabel == "" {
		defaultLabel = "default"
	}

	nextLabel := defaultLabel
	for _, item := range cases {
		if strings.EqualFold(strings.TrimSpace(item.Value), sourceValueText) {
			nextLabel = item.Label
			break
		}
	}

	output := workflow.CopyMap(req.Payload)
	output["switch_source_path"] = sourcePath
	output["switch_source_value"] = sourceValue
	output["switch_next_label"] = nextLabel
	return workflow.NodeExecuteResult{
		Output:    output,
		NextLabel: nextLabel,
	}, nil
}

type switchCase struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

func parseSwitchCases(raw any) ([]switchCase, error) {
	if raw == nil {
		return nil, nil
	}
	cases := make([]switchCase, 0)
	switch typed := raw.(type) {
	case []any:
		for _, entry := range typed {
			item, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			value := strings.TrimSpace(toString(item["value"]))
			label := strings.TrimSpace(toString(item["label"]))
			if value == "" || label == "" {
				continue
			}
			cases = append(cases, switchCase{Value: value, Label: label})
		}
	case map[string]any:
		for key, value := range typed {
			label := strings.TrimSpace(toString(value))
			if strings.TrimSpace(key) == "" || label == "" {
				continue
			}
			cases = append(cases, switchCase{Value: key, Label: label})
		}
	case string:
		rendered := strings.TrimSpace(typed)
		if rendered == "" {
			return nil, nil
		}
		decoded := []map[string]any{}
		if err := json.Unmarshal([]byte(rendered), &decoded); err != nil {
			return nil, fmt.Errorf("decode switch cases: %w", err)
		}
		for _, item := range decoded {
			value := strings.TrimSpace(toString(item["value"]))
			label := strings.TrimSpace(toString(item["label"]))
			if value == "" || label == "" {
				continue
			}
			cases = append(cases, switchCase{Value: value, Label: label})
		}
	default:
		encoded, _ := json.Marshal(typed)
		decoded := []map[string]any{}
		if err := json.Unmarshal(encoded, &decoded); err != nil {
			return nil, nil
		}
		for _, item := range decoded {
			value := strings.TrimSpace(toString(item["value"]))
			label := strings.TrimSpace(toString(item["label"]))
			if value == "" || label == "" {
				continue
			}
			cases = append(cases, switchCase{Value: value, Label: label})
		}
	}
	return cases, nil
}

func resolveSwitchSourceValue(sourcePath string, scope map[string]any) any {
	sourcePath = strings.TrimSpace(sourcePath)
	if sourcePath == "" {
		return nil
	}
	if value, ok := workflow.LookupPath(scope, sourcePath); ok {
		return value
	}
	return workflow.RenderTemplate(sourcePath, scope)
}
