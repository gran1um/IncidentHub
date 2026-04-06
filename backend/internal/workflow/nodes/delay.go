package nodes

import (
	"context"
	"time"

	"incidenthub/backend/internal/workflow"
)

type delayNode struct{}

func newDelayNode() workflow.NodeExecutor {
	return delayNode{}
}

func (n delayNode) Type() string {
	return "delay"
}

func (n delayNode) Execute(ctx context.Context, req workflow.NodeExecuteRequest) (workflow.NodeExecuteResult, error) {
	seconds := toInt(req.Node.Config["seconds"], 0)
	if seconds < 0 {
		seconds = 0
	}
	if seconds > 30 {
		seconds = 30
	}
	if seconds > 0 {
		timer := time.NewTimer(time.Duration(seconds) * time.Second)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return workflow.NodeExecuteResult{}, ctx.Err()
		case <-timer.C:
		}
	}
	output := workflow.CopyMap(req.Payload)
	output["delay_seconds"] = seconds
	return workflow.NodeExecuteResult{
		Output: output,
	}, nil
}
