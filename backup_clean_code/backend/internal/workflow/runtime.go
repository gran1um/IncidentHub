package workflow

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
)

type Node struct {
	ID     string
	Type   string
	Label  string
	Config map[string]any
}

type Edge struct {
	Source string
	Target string
	Label  string
}

type Definition struct {
	EntryNodeID string
	Nodes       []Node
	Edges       []Edge
}

type ExecuteMeta struct {
	TenantID   uuid.UUID
	WorkflowID uuid.UUID
	RunID      uuid.UUID
}

type NodeExecuteRequest struct {
	Meta       ExecuteMeta
	Node       Node
	Input      map[string]any
	Payload    map[string]any
	NodeOutput map[string]map[string]any
	Scope      map[string]any
}

type NodeExecuteResult struct {
	Output     map[string]any
	NextLabel  string
	NextNodeID string
	Stop       bool
}

type NodeExecutor interface {
	Type() string
	Execute(ctx context.Context, req NodeExecuteRequest) (NodeExecuteResult, error)
}

type Runtime struct {
	executors map[string]NodeExecutor
}

func NewRuntime(executors ...NodeExecutor) *Runtime {
	r := &Runtime{
		executors: make(map[string]NodeExecutor),
	}
	for _, executor := range executors {
		r.Register(executor)
	}
	return r
}

func (r *Runtime) Register(executor NodeExecutor) {
	if r == nil || executor == nil {
		return
	}
	nodeType := strings.TrimSpace(strings.ToLower(executor.Type()))
	if nodeType == "" {
		return
	}
	r.executors[nodeType] = executor
}

func (r *Runtime) Execute(ctx context.Context, definition *Definition, input map[string]any, meta ExecuteMeta) (map[string]any, error) {
	if r == nil {
		return nil, fmt.Errorf("workflow runtime is not configured")
	}
	if definition == nil {
		return nil, fmt.Errorf("workflow definition is required")
	}
	if len(definition.Nodes) == 0 {
		return nil, fmt.Errorf("workflow must contain at least one node")
	}

	nodeByID := make(map[string]Node, len(definition.Nodes))
	for _, node := range definition.Nodes {
		nodeByID[node.ID] = node
	}
	if _, ok := nodeByID[definition.EntryNodeID]; !ok {
		return nil, fmt.Errorf("workflow entry node is not found in node list")
	}

	outgoing := make(map[string][]Edge, len(definition.Nodes))
	for _, edge := range definition.Edges {
		outgoing[edge.Source] = append(outgoing[edge.Source], edge)
	}
	for source := range outgoing {
		sort.SliceStable(outgoing[source], func(i, j int) bool {
			left := outgoing[source][i]
			right := outgoing[source][j]
			leftPriority := edgePriority(left.Label)
			rightPriority := edgePriority(right.Label)
			if leftPriority != rightPriority {
				return leftPriority < rightPriority
			}
			if left.Label != right.Label {
				return left.Label < right.Label
			}
			return left.Target < right.Target
		})
	}

	initialInput := copyMap(input)
	currentPayload := copyMap(initialInput)
	nodeOutputs := make(map[string]map[string]any, len(definition.Nodes))
	visitCount := map[string]int{}
	trace := make([]map[string]any, 0, len(definition.Nodes))
	currentNodeID := definition.EntryNodeID

	maxSteps := len(definition.Nodes) * 10
	if maxSteps < 20 {
		maxSteps = 20
	}

	for step := 0; step < maxSteps; step++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		node, ok := nodeByID[currentNodeID]
		if !ok {
			return nil, fmt.Errorf("workflow references unknown node %q", currentNodeID)
		}
		executor, ok := r.executors[strings.TrimSpace(strings.ToLower(node.Type))]
		if !ok {
			return nil, fmt.Errorf("node type %q is not registered", node.Type)
		}

		scope := map[string]any{
			"tenant_id":    meta.TenantID.String(),
			"workflow_id":  meta.WorkflowID.String(),
			"run_id":       meta.RunID.String(),
			"input":        initialInput,
			"payload":      currentPayload,
			"node_outputs": nodeOutputs,
		}
		for key, value := range currentPayload {
			if _, exists := scope[key]; !exists {
				scope[key] = value
			}
		}

		result, err := executor.Execute(ctx, NodeExecuteRequest{
			Meta:       meta,
			Node:       node,
			Input:      initialInput,
			Payload:    currentPayload,
			NodeOutput: nodeOutputs,
			Scope:      scope,
		})
		if err != nil {
			return nil, fmt.Errorf("execute node %q (%s): %w", node.Label, node.Type, err)
		}

		output := copyMap(result.Output)
		nodeOutputs[node.ID] = output
		currentPayload = output
		visitCount[node.ID]++

		trace = append(trace, map[string]any{
			"step":       step + 1,
			"node_id":    node.ID,
			"node_type":  node.Type,
			"node_label": node.Label,
			"output":     output,
		})

		if result.Stop {
			break
		}

		nextEdge := pickNextEdge(outgoing[node.ID], result.NextNodeID, result.NextLabel)
		if nextEdge == nil {
			break
		}

		targetVisitCount := visitCount[nextEdge.Target]
		if targetVisitCount >= 12 {
			return nil, fmt.Errorf("workflow cycle limit reached near node %q", nextEdge.Target)
		}
		currentNodeID = nextEdge.Target
		if step == maxSteps-1 {
			return nil, fmt.Errorf("workflow traversal limit reached")
		}
	}

	result := map[string]any{
		"summary":      fmt.Sprintf("Executed %d workflow steps", len(trace)),
		"steps":        len(trace),
		"nodes_total":  len(definition.Nodes),
		"edges_total":  len(definition.Edges),
		"trace":        trace,
		"node_outputs": nodeOutputs,
		"final_output": currentPayload,
	}
	if len(initialInput) > 0 {
		result["input"] = initialInput
	}
	return result, nil
}

func pickNextEdge(edges []Edge, nextNodeID, nextLabel string) *Edge {
	if len(edges) == 0 {
		return nil
	}
	normalizedNodeID := strings.TrimSpace(nextNodeID)
	if normalizedNodeID != "" {
		for i := range edges {
			if edges[i].Target == normalizedNodeID {
				return &edges[i]
			}
		}
	}

	normalizedLabel := strings.TrimSpace(strings.ToLower(nextLabel))
	if normalizedLabel != "" {
		for i := range edges {
			if strings.TrimSpace(strings.ToLower(edges[i].Label)) == normalizedLabel {
				return &edges[i]
			}
		}
	}
	return &edges[0]
}

func edgePriority(label string) int {
	switch strings.TrimSpace(strings.ToLower(label)) {
	case "next":
		return 0
	case "success":
		return 1
	case "true":
		return 2
	case "":
		return 3
	case "failure":
		return 4
	case "false":
		return 5
	default:
		return 6
	}
}
