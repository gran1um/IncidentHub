package nodes

import (
	"context"
	"fmt"
	"strings"
	"time"

	"incidenthub/backend/internal/workflow"
)

type schedulerNode struct{}

func newSchedulerNode() workflow.NodeExecutor {
	return schedulerNode{}
}

func (n schedulerNode) Type() string {
	return "scheduler"
}

func (n schedulerNode) Execute(ctx context.Context, req workflow.NodeExecuteRequest) (workflow.NodeExecuteResult, error) {
	mode := normalizeLabel(toString(req.Node.Config["mode"]))
	if mode == "" {
		mode = "delay"
	}
	maxWaitSeconds := toInt(req.Node.Config["maxWaitSeconds"], 300)
	if maxWaitSeconds < 0 {
		maxWaitSeconds = 0
	}
	if maxWaitSeconds > 3600 {
		maxWaitSeconds = 3600
	}

	requestedWait := 0
	runAtText := ""

	switch mode {
	case "delay":
		requestedWait = toInt(req.Node.Config["seconds"], 0)
		if requestedWait == 0 {
			requestedWait = toInt(req.Node.Config["delaySeconds"], 0)
		}
	case "at":
		runAtText = strings.TrimSpace(toString(req.Node.Config["runAt"]))
		if runAtText == "" {
			return workflow.NodeExecuteResult{}, fmt.Errorf("runAt is required for scheduler mode=at")
		}
		runAt := workflow.RenderTemplate(runAtText, req.Scope)
		parsed, err := parseSchedulerTime(runAt)
		if err != nil {
			return workflow.NodeExecuteResult{}, err
		}
		diff := time.Until(parsed)
		if diff > 0 {
			requestedWait = int(diff.Seconds())
		}
	default:
		return workflow.NodeExecuteResult{}, fmt.Errorf("unsupported scheduler mode %q", mode)
	}

	if requestedWait < 0 {
		requestedWait = 0
	}

	appliedWait := requestedWait
	skipped := false
	if maxWaitSeconds > 0 && appliedWait > maxWaitSeconds {
		appliedWait = maxWaitSeconds
		skipped = true
	}

	if appliedWait > 0 {
		timer := time.NewTimer(time.Duration(appliedWait) * time.Second)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return workflow.NodeExecuteResult{}, ctx.Err()
		case <-timer.C:
		}
	}

	output := workflow.CopyMap(req.Payload)
	output["scheduler_mode"] = mode
	output["scheduler_requested_wait_seconds"] = requestedWait
	output["scheduler_applied_wait_seconds"] = appliedWait
	output["scheduler_wait_capped"] = skipped
	if runAtText != "" {
		output["scheduler_run_at"] = workflow.RenderTemplate(runAtText, req.Scope)
	}

	nextLabel := "immediate"
	if appliedWait > 0 {
		nextLabel = "waited"
	}
	if skipped {
		nextLabel = "capped"
	}
	return workflow.NodeExecuteResult{
		Output:    output,
		NextLabel: nextLabel,
	}, nil
}

func parseSchedulerTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("runAt is required")
	}
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}
	for _, format := range formats {
		if parsed, err := time.Parse(format, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid scheduler runAt time, expected RFC3339")
}
