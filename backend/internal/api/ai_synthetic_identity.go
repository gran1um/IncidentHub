package api

import (
	"strings"

	"incidenthub/backend/internal/models"
)

const (
	aiSyntheticAuthorKind      = "ai_agent"
	aiSyntheticAuthorAvatarKey = "ai-agent"
)

func aiAgentDisplayName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "AI Agent"
	}
	if strings.HasPrefix(strings.ToLower(trimmed), "ai agent") {
		return trimmed
	}
	return "AI Agent " + trimmed
}

func aiAgentSyntheticAuthorID(agentID string) string {
	trimmed := strings.TrimSpace(agentID)
	if trimmed == "" {
		return "ai-agent"
	}
	return "ai-agent:" + trimmed
}

func aiAgentSyntheticActorFields(agentID, agentName string) map[string]any {
	displayName := aiAgentDisplayName(agentName)
	actor := map[string]any{
		"author_kind":       aiSyntheticAuthorKind,
		"author_id":         aiAgentSyntheticAuthorID(agentID),
		"author_name":       displayName,
		"author_avatar_key": aiSyntheticAuthorAvatarKey,
	}
	if trimmedID := strings.TrimSpace(agentID); trimmedID != "" {
		actor["agent_id"] = trimmedID
	}
	return actor
}

func aiAgentSyntheticActor(agent aiAgentDefinition) map[string]any {
	return aiAgentSyntheticActorFields(agent.ID.String(), agent.Name)
}

func aiSyntheticActorFromMetadata(metadata map[string]any) map[string]any {
	meta := normalizeMap(metadata)
	if len(meta) == 0 {
		return nil
	}
	if actor := normalizeMap(meta["actor"]); len(actor) > 0 {
		return aiAgentSyntheticActorFields(
			stringFromMap(actor, "agent_id", "agentId"),
			firstNonEmptyString(
				stringFromMap(actor, "author_name", "authorName"),
				stringFromMap(actor, "agent_name", "agentName"),
			),
		)
	}
	agentID := stringFromMap(meta, "agent_id", "agentId")
	agentName := stringFromMap(meta, "agent_name", "agentName")
	if strings.TrimSpace(agentID) == "" && strings.TrimSpace(agentName) == "" {
		return nil
	}
	return aiAgentSyntheticActorFields(agentID, agentName)
}

func syntheticActorFromConnectorExecution(execution models.ConnectorHubExecution) map[string]any {
	if strings.TrimSpace(strings.ToLower(execution.ExecutionMode)) != "ai_agent" {
		return nil
	}
	return aiSyntheticActorFromMetadata(execution.Metadata)
}
