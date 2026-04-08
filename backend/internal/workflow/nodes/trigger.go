package nodes

import (
	"context"
	"strings"

	"incidenthub/backend/internal/workflow"
)

type triggerNode struct{}

func newTriggerNode() workflow.NodeExecutor {
	return triggerNode{}
}

func (n triggerNode) Type() string {
	return "trigger"
}

func (n triggerNode) Execute(_ context.Context, req workflow.NodeExecuteRequest) (workflow.NodeExecuteResult, error) {
	event := strings.TrimSpace(toString(req.Node.Config["event"]))
	if event == "" {
		event = "manual"
	}
	output := workflow.CopyMap(req.Payload)
	output["trigger_event"] = event
	return workflow.NodeExecuteResult{
		Output: output,
	}, nil
}
