package nodes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"incidenthub/backend/internal/workflow"
)

type pythonCodeNode struct{}

func newPythonCodeNode() workflow.NodeExecutor {
	return pythonCodeNode{}
}

func (n pythonCodeNode) Type() string {
	return "python_code"
}

func (n pythonCodeNode) Execute(ctx context.Context, req workflow.NodeExecuteRequest) (workflow.NodeExecuteResult, error) {
	code := strings.TrimSpace(toString(req.Node.Config["code"]))
	if code == "" {
		return workflow.NodeExecuteResult{}, fmt.Errorf("python code is required")
	}
	code = workflow.RenderTemplate(code, req.Scope)

	timeoutSeconds := toInt(req.Node.Config["timeoutSeconds"], 10)
	if timeoutSeconds <= 0 {
		timeoutSeconds = 10
	}
	if timeoutSeconds > 120 {
		timeoutSeconds = 120
	}
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	inputPayload := map[string]any{
		"code":    code,
		"input":   req.Input,
		"payload": req.Payload,
		"scope":   req.Scope,
	}
	stdin, err := json.Marshal(inputPayload)
	if err != nil {
		return workflow.NodeExecuteResult{}, fmt.Errorf("encode python input: %w", err)
	}

	cmd := exec.CommandContext(runCtx, "python3", "-c", pythonNodeWrapperScript)
	cmd.Stdin = bytes.NewReader(stdin)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			return workflow.NodeExecuteResult{}, fmt.Errorf("python execution exceeded timeout")
		}
		details := strings.TrimSpace(stderr.String())
		if details == "" {
			details = strings.TrimSpace(err.Error())
		}
		return workflow.NodeExecuteResult{}, fmt.Errorf("python execution failed: %s", details)
	}

	type pythonNodeResult struct {
		OK     bool           `json:"ok"`
		Result any            `json:"result"`
		Output map[string]any `json:"output"`
		Stdout string         `json:"stdout"`
		Error  string         `json:"error"`
	}
	result := pythonNodeResult{}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		details := strings.TrimSpace(stderr.String())
		return workflow.NodeExecuteResult{}, fmt.Errorf("decode python result: %w (%s)", err, details)
	}
	if !result.OK {
		return workflow.NodeExecuteResult{}, fmt.Errorf("python runtime error: %s", strings.TrimSpace(result.Error))
	}

	output := workflow.CopyMap(req.Payload)
	for key, value := range result.Output {
		output[key] = value
	}
	resultKey := strings.TrimSpace(toString(req.Node.Config["resultKey"]))
	if resultKey == "" {
		resultKey = "python_result"
	}
	output[resultKey] = result.Result
	if strings.TrimSpace(result.Stdout) != "" {
		output["python_stdout"] = strings.TrimSpace(result.Stdout)
	}
	return workflow.NodeExecuteResult{
		Output: output,
	}, nil
}

const pythonNodeWrapperScript = `
import contextlib
import io
import json
import sys
import traceback

response = {"ok": False}
stdout_buffer = io.StringIO()

try:
    payload = json.load(sys.stdin)
    user_code = str(payload.get("code") or "")
    runtime_scope = {
        "input": payload.get("input", {}),
        "payload": payload.get("payload", {}),
        "scope": payload.get("scope", {}),
        "result": None,
        "output": {},
    }
    with contextlib.redirect_stdout(stdout_buffer):
        exec(user_code, {}, runtime_scope)
    output_value = runtime_scope.get("output", {})
    if not isinstance(output_value, dict):
        output_value = {"value": output_value}
    response = {
        "ok": True,
        "result": runtime_scope.get("result"),
        "output": output_value,
        "stdout": stdout_buffer.getvalue(),
    }
except Exception as exc:
    response = {
        "ok": False,
        "error": f"{exc.__class__.__name__}: {exc}",
        "stdout": stdout_buffer.getvalue(),
        "traceback": traceback.format_exc(),
    }

sys.stdout.write(json.dumps(response))
`
