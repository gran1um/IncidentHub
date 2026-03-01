package nodes

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"incidenthub/backend/internal/connectors/outbound"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/workflow"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type connectorActionNode struct {
	nodeType string
	deps     Dependencies
}

func newConnectorActionNode(nodeType string, deps Dependencies) workflow.NodeExecutor {
	return connectorActionNode{
		nodeType: normalizeLabel(nodeType),
		deps:     deps,
	}
}

func (n connectorActionNode) Type() string {
	return n.nodeType
}

func (n connectorActionNode) Execute(ctx context.Context, req workflow.NodeExecuteRequest) (workflow.NodeExecuteResult, error) {
	if n.deps.Catalog == nil {
		return workflow.NodeExecuteResult{}, fmt.Errorf("catalog repository is not configured")
	}
	if n.deps.Outbound == nil {
		return workflow.NodeExecuteResult{}, fmt.Errorf("outbound connector service is not configured")
	}

	connectorIDRaw := toString(req.Node.Config["connectorId"])
	if connectorIDRaw == "" {
		connectorIDRaw = toString(req.Node.Config["connector_id"])
	}
	if connectorIDRaw == "" {
		output := workflow.CopyMap(req.Payload)
		output["connector_skipped"] = "connectorId is not configured"
		return workflow.NodeExecuteResult{Output: output}, nil
	}
	connectorID, err := uuid.Parse(connectorIDRaw)
	if err != nil {
		return workflow.NodeExecuteResult{}, fmt.Errorf("invalid connectorId")
	}

	connector, err := loadOutboundConnector(ctx, n.deps, req.Meta.TenantID, connectorID)
	if err != nil {
		return workflow.NodeExecuteResult{}, err
	}

	messageTemplate := toString(req.Node.Config["messageTemplate"])
	if messageTemplate == "" {
		messageTemplate = toString(req.Node.Config["message"])
	}
	if messageTemplate == "" {
		messageTemplate = "Workflow node {{node_label}} executed"
	}
	scope := workflow.CopyMap(req.Scope)
	scope["node_label"] = req.Node.Label
	scope["node_type"] = req.Node.Type
	message := workflow.RenderTemplate(messageTemplate, scope)
	if strings.TrimSpace(message) == "" {
		return workflow.NodeExecuteResult{}, fmt.Errorf("message template resolves to empty text")
	}

	recipient := workflow.RenderTemplate(toString(req.Node.Config["recipient"]), scope)
	metadata := map[string]any{
		"workflow_node_id":   req.Node.ID,
		"workflow_node_type": req.Node.Type,
		"method_id":          toString(req.Node.Config["methodId"]),
	}
	if strings.TrimSpace(recipient) != "" {
		metadata["target"] = recipient
		metadata["recipient"] = recipient
		metadata["participant"] = map[string]any{"target": recipient}
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
	output["connector_reply"] = response.Reply
	output["connector_conversation_id"] = response.ConversationID
	output["connector_cursor"] = response.Cursor
	if len(response.Metadata) > 0 {
		output["connector_metadata"] = response.Metadata
	}
	return workflow.NodeExecuteResult{
		Output: output,
	}, nil
}

func loadOutboundConnector(ctx context.Context, deps Dependencies, tenantID uuid.UUID, connectorID uuid.UUID) (*models.CatalogItem, error) {
	connector, err := deps.Catalog.GetByID(ctx, "outbound_connectors", connectorID, &tenantID)
	if err != nil {
		legacy, legacyErr := deps.Catalog.GetByID(ctx, "connectors", connectorID, &tenantID)
		if legacyErr != nil {
			if errors.Is(err, pgx.ErrNoRows) || errors.Is(legacyErr, pgx.ErrNoRows) {
				return nil, fmt.Errorf("connector %s is not found", connectorID.String())
			}
			return nil, fmt.Errorf("load outbound connector: %w", err)
		}
		connector = legacy
	}

	direction := strings.TrimSpace(strings.ToLower(toString(connector.Data["direction"])))
	if direction == "" && strings.EqualFold(strings.TrimSpace(connector.Kind), "outbound_connectors") {
		direction = "outbound"
	}
	if direction == "" {
		direction = "outbound"
	}
	if direction != "outbound" {
		return nil, fmt.Errorf("connector %s is not outbound", connectorID.String())
	}

	enabled := workflow.BoolFromAny(connector.Data["enabled"], true)
	if !enabled {
		return nil, fmt.Errorf("connector %s is disabled", connectorID.String())
	}
	return connector, nil
}
