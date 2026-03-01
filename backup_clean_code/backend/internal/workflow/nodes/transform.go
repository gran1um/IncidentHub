package nodes

import (
	"context"
	"encoding/json"
	"strings"

	"incidenthub/backend/internal/workflow"
)

type transformNode struct{}

func newTransformNode() workflow.NodeExecutor {
	return transformNode{}
}

func (n transformNode) Type() string {
	return "transform"
}

func (n transformNode) Execute(_ context.Context, req workflow.NodeExecuteRequest) (workflow.NodeExecuteResult, error) {
	mapping := toString(req.Node.Config["mapping"])
	mapping = workflow.RenderTemplate(mapping, req.Scope)
	output := workflow.CopyMap(req.Payload)
	if strings.TrimSpace(mapping) == "" {
		return workflow.NodeExecuteResult{Output: output}, nil
	}
	var parsed any
	if err := json.Unmarshal([]byte(mapping), &parsed); err != nil {
		output["transform_raw"] = mapping
		return workflow.NodeExecuteResult{Output: output}, nil
	}
	output["transform"] = parsed
	if typed, ok := parsed.(map[string]any); ok {
		for key, value := range typed {
			output[key] = value
		}
	}
	return workflow.NodeExecuteResult{
		Output: output,
	}, nil
}
