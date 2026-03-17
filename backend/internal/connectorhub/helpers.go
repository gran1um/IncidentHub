package connectorhub

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"incidenthub/backend/internal/connectors/outbound"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/workflow"
)

type ScopeInput struct {
	Input   map[string]any
	CaseID  string
	AlertID string
}

var actionKeyPattern = regexp.MustCompile(`[^a-z0-9]+`)

var errConnectorHubExecutionFailed = errors.New("connector hub execution failed")

func IsExecutionTerminalStatus(status models.ConnectorHubExecutionStatus) bool {
	switch status {
	case models.ConnectorHubExecutionStatusCompleted,
		models.ConnectorHubExecutionStatusDryRun,
		models.ConnectorHubExecutionStatusFailed,
		models.ConnectorHubExecutionStatusCancelled,
		models.ConnectorHubExecutionStatusDeadLetter:
		return true
	default:
		return false
	}
}

func ExecutionTerminalError(execution *models.ConnectorHubExecution) error {
	if execution == nil {
		return fmt.Errorf("connector hub execution is unavailable")
	}
	switch execution.Status {
	case models.ConnectorHubExecutionStatusCompleted, models.ConnectorHubExecutionStatusDryRun:
		return nil
	case models.ConnectorHubExecutionStatusCancelled:
		return fmt.Errorf("connector hub execution was canceled")
	default:
		message := firstNonEmptyString(strings.TrimSpace(execution.Error), "connector hub execution ended with status "+string(execution.Status))
		return fmt.Errorf("%w: %s", errConnectorHubExecutionFailed, message)
	}
}

func BuildSendResponsePayload(reply outbound.SendResponse) map[string]any {
	metadata := normalizeMap(reply.Metadata)
	payload := map[string]any{
		"reply":           strings.TrimSpace(reply.Reply),
		"conversation_id": strings.TrimSpace(reply.ConversationID),
		"cursor":          strings.TrimSpace(reply.Cursor),
		"metadata":        metadata,
	}
	externalID := strings.TrimSpace(stringFromMap(metadata, "external_id", "message_id", "id"))
	if externalID != "" {
		payload["external_id"] = externalID
	}
	providerStatus := strings.TrimSpace(stringFromMap(metadata, "provider_status", "status"))
	if providerStatus != "" {
		payload["provider_status"] = providerStatus
	}
	correlationID := strings.TrimSpace(stringFromMap(metadata, "correlation_id", "conversation_id"))
	if correlationID != "" {
		payload["correlation_id"] = correlationID
	}
	return payload
}

func RequestFromPayload(payload map[string]any) outbound.SendRequest {
	metadata := normalizeMap(firstMapValue(payload, "metadata"))
	if metadata == nil {
		metadata = map[string]any{}
	}
	return outbound.SendRequest{
		ThreadID:       strings.TrimSpace(stringFromMap(payload, "thread_id", "threadId")),
		ConversationID: strings.TrimSpace(stringFromMap(payload, "conversation_id", "conversationId")),
		Author:         strings.TrimSpace(stringFromMap(payload, "author")),
		Message:        strings.TrimSpace(stringFromMap(payload, "message")),
		Metadata:       metadata,
	}
}

func NormalizeActionKey(raw string) string {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" {
		return ""
	}
	normalized = actionKeyPattern.ReplaceAllString(normalized, "_")
	normalized = strings.Trim(normalized, "_")
	return normalized
}

func BuildScope(input ScopeInput, actionKey, methodName string) map[string]any {
	scope := map[string]any{
		"action":      actionKey,
		"method_name": methodName,
		"input":       normalizeMap(input.Input),
	}
	for key, value := range normalizeMap(input.Input) {
		scope[key] = value
	}
	if strings.TrimSpace(input.CaseID) != "" {
		scope["case_id"] = strings.TrimSpace(input.CaseID)
	}
	if strings.TrimSpace(input.AlertID) != "" {
		scope["alert_id"] = strings.TrimSpace(input.AlertID)
	}
	return scope
}

func RenderTemplateValue(value any, scope map[string]any) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			switch nested := item.(type) {
			case string:
				out[key] = workflow.RenderTemplate(nested, scope)
			case map[string]any:
				out[key] = RenderTemplateValue(nested, scope)
			case []any:
				out[key] = RenderSliceTemplates(nested, scope)
			default:
				out[key] = nested
			}
		}
		return out
	default:
		return map[string]any{}
	}
}

func RenderSliceTemplates(items []any, scope map[string]any) []any {
	if len(items) == 0 {
		return []any{}
	}
	out := make([]any, 0, len(items))
	for _, item := range items {
		switch typed := item.(type) {
		case string:
			out = append(out, workflow.RenderTemplate(typed, scope))
		case map[string]any:
			out = append(out, RenderTemplateValue(typed, scope))
		case []any:
			out = append(out, RenderSliceTemplates(typed, scope))
		default:
			out = append(out, typed)
		}
	}
	return out
}

func MergeMetadata(base map[string]any, extra map[string]any) map[string]any {
	out := normalizeMap(base)
	if out == nil {
		out = map[string]any{}
	}
	for key, value := range normalizeMap(extra) {
		out[key] = value
	}
	return out
}

func normalizeMap(input any) map[string]any {
	if input == nil {
		return nil
	}
	switch typed := input.(type) {
	case map[string]any:
		return typed
	case map[string]string:
		out := make(map[string]any, len(typed))
		for key, value := range typed {
			out[key] = value
		}
		return out
	default:
		return nil
	}
}

func firstMapValue(payload map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := payload[key]; ok {
			return value
		}
	}
	return nil
}

func stringFromMap(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := payload[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			return typed
		case fmt.Stringer:
			return typed.String()
		default:
			return fmt.Sprintf("%v", typed)
		}
	}
	return ""
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
