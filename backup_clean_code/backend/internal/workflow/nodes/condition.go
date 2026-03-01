package nodes

import (
	"context"

	"incidenthub/backend/internal/workflow"
)

type conditionNode struct{}

func newConditionNode() workflow.NodeExecutor {
	return conditionNode{}
}

func (n conditionNode) Type() string {
	return "condition"
}

func (n conditionNode) Execute(_ context.Context, req workflow.NodeExecuteRequest) (workflow.NodeExecuteResult, error) {
	expression := toString(req.Node.Config["expression"])
	ok, err := evaluateConditionExpression(expression, req.Scope)
	if err != nil {
		return workflow.NodeExecuteResult{}, err
	}
	output := workflow.CopyMap(req.Payload)
	output["condition_expression"] = expression
	output["condition_result"] = ok
	nextLabel := "false"
	if ok {
		nextLabel = "true"
	}
	return workflow.NodeExecuteResult{
		Output:    output,
		NextLabel: nextLabel,
	}, nil
}
