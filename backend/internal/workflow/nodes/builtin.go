package nodes

import (
	"incidenthub/backend/internal/connectors/outbound"
	"incidenthub/backend/internal/repository"
	"incidenthub/backend/internal/workflow"
)

type Dependencies struct {
	System   *repository.SystemRepository
	Catalog  *repository.CatalogRepository
	Outbound *outbound.Service
}

func NewBuiltinSet(deps Dependencies) []workflow.NodeExecutor {
	return []workflow.NodeExecutor{
		newTriggerNode(),
		newConditionNode(),
		newSwitchNode(),
		newDelayNode(),
		newSchedulerNode(),
		newTransformNode(),
		newAggregateNode(),
		newIOCExtractNode(),
		newWatchlistMatchNode(),
		newMITREMapNode(),
		newRiskScoreNode(),
		newContainmentDecisionNode(),
		newPythonCodeNode(),
		newConnectorActionNode("analyzer", deps),
		newConnectorActionNode("responder", deps),
		newKafkaTriggerNode(deps),
		newWebhookTriggerNode(deps),
		newPostgresQueryNode(deps),
		newRedisNode(),
		newCassandraQueryNode(),
		newS3ObjectNode(),
		newTelegramSendNode(deps),
	}
}
