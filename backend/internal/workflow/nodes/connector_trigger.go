package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"incidenthub/backend/internal/connectors/outbound"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/workflow"

	"github.com/google/uuid"
)

type connectorTriggerNode struct {
	nodeType string
	channel  string
	deps     Dependencies
}

func newWebhookTriggerNode(deps Dependencies) workflow.NodeExecutor {
	return connectorTriggerNode{
		nodeType: "webhook_trigger",
		channel:  "webhook",
		deps:     deps,
	}
}

func newKafkaTriggerNode(deps Dependencies) workflow.NodeExecutor {
	return connectorTriggerNode{
		nodeType: "kafka_trigger",
		channel:  "kafka",
		deps:     deps,
	}
}

func (n connectorTriggerNode) Type() string {
	return n.nodeType
}

func (n connectorTriggerNode) Execute(ctx context.Context, req workflow.NodeExecuteRequest) (workflow.NodeExecuteResult, error) {
	if n.deps.Catalog == nil {
		return workflow.NodeExecuteResult{}, fmt.Errorf("catalog repository is not configured")
	}
	if n.deps.Outbound == nil {
		return workflow.NodeExecuteResult{}, fmt.Errorf("outbound connector service is not configured")
	}

	connectorIDRaw := strings.TrimSpace(toString(req.Node.Config["connectorId"]))
	if connectorIDRaw == "" {
		return workflow.NodeExecuteResult{}, fmt.Errorf("connectorId is required")
	}
	connectorID, err := uuid.Parse(connectorIDRaw)
	if err != nil {
		return workflow.NodeExecuteResult{}, fmt.Errorf("invalid connectorId")
	}
	connector, err := loadOutboundConnector(ctx, n.deps, req.Meta.TenantID, connectorID)
	if err != nil {
		return workflow.NodeExecuteResult{}, err
	}
	channel := normalizeLabel(toString(connector.Data["channel"]))
	if channel != n.channel {
		return workflow.NodeExecuteResult{}, fmt.Errorf("connector %s must use channel=%s", connectorID.String(), n.channel)
	}

	conversationID := strings.TrimSpace(workflow.RenderTemplate(toString(req.Node.Config["conversationId"]), req.Scope))
	cursor := strings.TrimSpace(workflow.RenderTemplate(toString(req.Node.Config["cursor"]), req.Scope))
	threadID := strings.TrimSpace(workflow.RenderTemplate(toString(req.Node.Config["threadId"]), req.Scope))
	maxMessages := toInt(req.Node.Config["maxMessages"], 20)
	if maxMessages <= 0 {
		maxMessages = 20
	}
	if maxMessages > 1000 {
		maxMessages = 1000
	}

	connectorToPoll := copyConnectorWithPollLimit(*connector, maxMessages)
	response, err := n.deps.Outbound.Poll(ctx, connectorToPoll, outbound.PollRequest{
		ThreadID:       threadID,
		ConversationID: conversationID,
		Cursor:         cursor,
	})
	if err != nil {
		return workflow.NodeExecuteResult{}, err
	}

	prefix := strings.TrimSuffix(n.nodeType, "_trigger")
	messages := make([]map[string]any, 0, len(response.Messages))
	for _, item := range response.Messages {
		messages = append(messages, map[string]any{
			"external_id": item.ExternalID,
			"author":      item.Author,
			"content":     item.Content,
			"timestamp":   item.Timestamp,
			"metadata":    item.Metadata,
		})
	}

	output := workflow.CopyMap(req.Payload)
	output[prefix+"_messages"] = messages
	output[prefix+"_message_count"] = len(messages)
	output[prefix+"_conversation_id"] = response.ConversationID
	output[prefix+"_next_cursor"] = response.NextCursor
	if len(response.Metadata) > 0 {
		output[prefix+"_metadata"] = response.Metadata
	}
	if len(messages) > 0 {
		output[prefix+"_message"] = messages[0]
		return workflow.NodeExecuteResult{
			Output:    output,
			NextLabel: "received",
		}, nil
	}
	return workflow.NodeExecuteResult{
		Output:    output,
		NextLabel: "empty",
	}, nil
}

func copyConnectorWithPollLimit(connector models.CatalogItem, maxMessages int) models.CatalogItem {
	out := connector
	out.Data = workflow.CopyMap(connector.Data)

	configRaw := out.Data["config"]
	configMap := map[string]any{}
	switch typed := configRaw.(type) {
	case map[string]any:
		configMap = workflow.CopyMap(typed)
	case string:
		if strings.TrimSpace(typed) != "" {
			_ = json.Unmarshal([]byte(typed), &configMap)
		}
	default:
		encoded, _ := json.Marshal(typed)
		_ = json.Unmarshal(encoded, &configMap)
	}
	configMap["poll_max_messages"] = maxMessages
	out.Data["config"] = configMap
	return out
}
