package workflow

import (
	"encoding/json"
	"errors"
	"strings"
)

// Sentinel errors for workflow definition parsing (err113).
var (
	errWorkflowDefinitionRequired = errors.New("workflow definition is required")
	errWorkflowDefinitionInvalid  = errors.New("workflow definition is invalid")
	errWorkflowDefinitionNoNodes  = errors.New("workflow must contain at least one node")
	errWorkflowEntryNodeNotFound  = errors.New("workflow entry node is not found in node list")
)

func NormalizeDefinition(raw any) (*Definition, error) {
	if raw == nil {
		return nil, errWorkflowDefinitionRequired
	}

	payload := map[string]any{}
	switch typed := raw.(type) {
	case map[string]any:
		payload = typed
	default:
		encoded, err := json.Marshal(raw)
		if err != nil {
			return nil, errWorkflowDefinitionInvalid
		}
		if err := json.Unmarshal(encoded, &payload); err != nil {
			return nil, errWorkflowDefinitionInvalid
		}
	}

	nodesRaw, _ := payload["nodes"].([]any)
	nodes := make([]Node, 0, len(nodesRaw))
	nodeByID := make(map[string]struct{}, len(nodesRaw))
	for _, rawNode := range nodesRaw {
		nodeMap, _ := rawNode.(map[string]any)
		nodeID := strings.TrimSpace(stringFromMap(nodeMap, "id"))
		if nodeID == "" {
			continue
		}
		nodeType := strings.TrimSpace(strings.ToLower(stringFromMap(nodeMap, "type")))
		if nodeType == "" {
			nodeType = "step"
		}
		nodeLabel := strings.TrimSpace(stringFromMap(nodeMap, "label", "name"))
		if nodeLabel == "" {
			nodeLabel = nodeID
		}
		config := map[string]any{}
		if rawConfig, ok := nodeMap["config"].(map[string]any); ok {
			config = rawConfig
		} else if nodeMap["config"] != nil {
			encoded, _ := json.Marshal(nodeMap["config"])
			_ = json.Unmarshal(encoded, &config)
		}

		nodes = append(nodes, Node{
			ID:     nodeID,
			Type:   nodeType,
			Label:  nodeLabel,
			Config: config,
		})
		nodeByID[nodeID] = struct{}{}
	}
	if len(nodes) == 0 {
		return nil, errWorkflowDefinitionNoNodes
	}

	edgesRaw, _ := payload["edges"].([]any)
	edges := make([]Edge, 0, len(edgesRaw))
	for _, rawEdge := range edgesRaw {
		edgeMap, _ := rawEdge.(map[string]any)
		source := strings.TrimSpace(stringFromMap(edgeMap, "source", "from"))
		target := strings.TrimSpace(stringFromMap(edgeMap, "target", "to"))
		if source == "" || target == "" {
			continue
		}
		if _, ok := nodeByID[source]; !ok {
			continue
		}
		if _, ok := nodeByID[target]; !ok {
			continue
		}
		label := strings.TrimSpace(strings.ToLower(stringFromMap(edgeMap, "label", "condition")))
		edges = append(edges, Edge{
			Source: source,
			Target: target,
			Label:  label,
		})
	}

	entryNodeID := strings.TrimSpace(stringFromMap(payload, "entryNodeId", "entry_node_id"))
	if entryNodeID == "" {
		entryNodeID = nodes[0].ID
	}
	if _, ok := nodeByID[entryNodeID]; !ok {
		return nil, errWorkflowEntryNodeNotFound
	}

	return &Definition{
		EntryNodeID: entryNodeID,
		Nodes:       nodes,
		Edges:       edges,
	}, nil
}
