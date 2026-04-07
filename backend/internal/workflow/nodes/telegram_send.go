package nodes

import (
	"context"
	"fmt"
	"strings"

	"incidenthub/backend/internal/connectors/outbound"
	"incidenthub/backend/internal/workflow"

	"github.com/google/uuid"
)

type telegramSendNode struct {
	deps Dependencies
}

func newTelegramSendNode(deps Dependencies) workflow.NodeExecutor {
	return telegramSendNode{deps: deps}
}

func (n telegramSendNode) Type() string {
	return "telegram_send"
}

func (n telegramSendNode) Execute(ctx context.Context, req workflow.NodeExecuteRequest) (workflow.NodeExecuteResult, error) {
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
	if channel != "telegram" {
		return workflow.NodeExecuteResult{}, fmt.Errorf("connector %s is not a telegram connector", connectorID.String())
	}

	scope := workflow.CopyMap(req.Scope)
	scope["node_label"] = req.Node.Label
	scope["node_type"] = req.Node.Type

	messageTemplate := strings.TrimSpace(toString(req.Node.Config["message"]))
	if messageTemplate == "" {
		messageTemplate = strings.TrimSpace(toString(req.Node.Config["messageTemplate"]))
	}
	if messageTemplate == "" {
		return workflow.NodeExecuteResult{}, fmt.Errorf("message template is required")
	}
	message := workflow.RenderTemplate(messageTemplate, scope)
	if strings.TrimSpace(message) == "" {
		return workflow.NodeExecuteResult{}, fmt.Errorf("message template resolves to empty text")
	}

	recipient := workflow.RenderTemplate(toString(req.Node.Config["recipient"]), scope)
	if strings.TrimSpace(recipient) == "" {
		recipient = workflow.RenderTemplate(toString(req.Node.Config["chatId"]), scope)
	}
	if strings.TrimSpace(recipient) == "" {
		return workflow.NodeExecuteResult{}, fmt.Errorf("recipient is required")
	}

	metadata := map[string]any{
		"target":             recipient,
		"recipient":          recipient,
		"chat_id":            recipient,
		"workflow_node_id":   req.Node.ID,
		"workflow_node_type": req.Node.Type,
		"participant": map[string]any{
			"target": recipient,
		},
	}

	response, err := n.deps.Outbound.Send(ctx, *connector, outbound.SendRequest{
		Author:   "workflow",
		Message:  message,
		Metadata: metadata,
	})
	if err != nil {
		return workflow.NodeExecuteResult{}, err
	}

	output := workflow.CopyMap(req.Payload)
	output["telegram_reply"] = response.Reply
	output["telegram_conversation_id"] = response.ConversationID
	output["telegram_cursor"] = response.Cursor
	output["telegram_recipient"] = recipient
	if len(response.Metadata) > 0 {
		output["telegram_metadata"] = response.Metadata
	}
	return workflow.NodeExecuteResult{
		Output: output,
	}, nil
}
