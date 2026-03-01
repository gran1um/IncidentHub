package api

import (
	"strings"

	"incidenthub/backend/internal/models"
)

const (
	connectorCapabilityHubExecute         = "hub_execute"
	connectorCapabilityCaseCommunications = "case_communications"
	connectorCapabilityForumThreads       = "forum_threads"
	connectorCapabilitySyncMessages       = "sync_messages"
)

func normalizeConnectorType(raw string) string {
	upper := strings.ToUpper(strings.TrimSpace(raw))
	switch upper {
	case "":
		return ""
	case "HTTP", "REST", "API":
		return "HTTP"
	case "SQL", "DATABASE", "DB":
		return "SQL"
	case "S3", "OBJECT_STORAGE", "OBJECT-STORE", "OBJECT_STORE":
		return "S3"
	case "KAFKA":
		return "Kafka"
	case "REDIS":
		return "Redis"
	case "SMTP", "EMAIL", "MAIL":
		return "SMTP"
	case "TELEGRAM":
		return "Telegram"
	case "SLACK":
		return "Slack"
	case "OUTLOOK", "OUTLOOK_MAIL", "OUTLOOK MAIL", "MICROSOFT_GRAPH", "MICROSOFT GRAPH", "GRAPH":
		return "Outlook"
	case "WEBHOOK":
		return "Webhook"
	case "TIME":
		return "Time"
	case "CUSTOM":
		return "Custom"
	default:
		return strings.TrimSpace(raw)
	}
}

func connectorTypeToChannel(connectorType string) string {
	switch strings.ToUpper(strings.TrimSpace(connectorType)) {
	case "HTTP":
		return "webhook"
	case "SQL":
		return "sql"
	case "S3":
		return "object_storage"
	case "KAFKA":
		return "kafka"
	case "REDIS":
		return "redis"
	case "SMTP":
		return "email"
	case "TELEGRAM":
		return "telegram"
	case "SLACK":
		return "slack"
	case "OUTLOOK":
		return "outlook"
	case "WEBHOOK":
		return "webhook"
	case "TIME":
		return "time"
	case "CUSTOM":
		return "custom"
	default:
		return ""
	}
}

func connectorChannelFromData(data map[string]any) string {
	channel := strings.ToLower(strings.TrimSpace(stringFromMap(data, "channel")))
	if channel != "" {
		return channel
	}
	return connectorTypeToChannel(normalizeConnectorType(stringFromMap(data, "type", "provider")))
}

func connectorChannelFromItem(item models.CatalogItem) string {
	return connectorChannelFromData(item.Data)
}

func normalizeConnectorCapability(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case connectorCapabilityHubExecute:
		return connectorCapabilityHubExecute
	case connectorCapabilityCaseCommunications:
		return connectorCapabilityCaseCommunications
	case connectorCapabilityForumThreads:
		return connectorCapabilityForumThreads
	case connectorCapabilitySyncMessages:
		return connectorCapabilitySyncMessages
	default:
		return ""
	}
}

func defaultConnectorCapabilities(channel string) []string {
	capabilities := []string{connectorCapabilityHubExecute}
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case "telegram", "slack", "outlook", "email", "mock":
		capabilities = append(capabilities, connectorCapabilityCaseCommunications, connectorCapabilityForumThreads, connectorCapabilitySyncMessages)
	case "time":
		capabilities = append(capabilities, connectorCapabilityCaseCommunications, connectorCapabilitySyncMessages)
	}
	return capabilities
}

func connectorCapabilitiesFromData(data map[string]any) []string {
	rawValues := stringSliceFromMap(data, "capabilities", "connector_capabilities", "connectorCapabilities")
	seen := map[string]struct{}{}
	out := make([]string, 0)
	appendOne := func(raw string) {
		normalized := normalizeConnectorCapability(raw)
		if normalized == "" {
			return
		}
		if _, exists := seen[normalized]; exists {
			return
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	for _, raw := range rawValues {
		appendOne(raw)
	}
	if len(out) == 0 {
		for _, capability := range defaultConnectorCapabilities(connectorChannelFromData(data)) {
			appendOne(capability)
		}
	}
	return out
}

func connectorHasCapabilityData(data map[string]any, capability string) bool {
	normalized := normalizeConnectorCapability(capability)
	if normalized == "" {
		return false
	}
	for _, item := range connectorCapabilitiesFromData(data) {
		if item == normalized {
			return true
		}
	}
	return false
}

func connectorHasCapabilityItem(item models.CatalogItem, capability string) bool {
	return connectorHasCapabilityData(item.Data, capability)
}

func normalizeConnectorCommunicationMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "chat":
		return "chat"
	case "email":
		return "email"
	default:
		return ""
	}
}

func defaultConnectorCommunicationMode(channel string) string {
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case "telegram", "slack", "time":
		return "chat"
	case "email", "outlook":
		return "email"
	default:
		return ""
	}
}

func connectorCommunicationModeFromData(data map[string]any) string {
	if mode := normalizeConnectorCommunicationMode(stringFromMap(data, "communication_mode", "communicationMode")); mode != "" {
		return mode
	}
	return defaultConnectorCommunicationMode(connectorChannelFromData(data))
}

func connectorCommunicationModeFromItem(item models.CatalogItem) string {
	return connectorCommunicationModeFromData(item.Data)
}

func normalizeOutboundConnectorData(data map[string]any) {
	if data == nil {
		return
	}
	data["direction"] = "outbound"

	rawType := strings.TrimSpace(stringFromMap(data, "type", "provider"))
	normalizedType := normalizeConnectorType(rawType)
	if normalizedType == "" && strings.TrimSpace(stringFromMap(data, "channel")) == "" {
		normalizedType = "HTTP"
	}
	if normalizedType != "" {
		data["type"] = normalizedType
	}

	channel := connectorChannelFromData(data)
	if channel != "" {
		data["channel"] = channel
	}

	if category := strings.TrimSpace(stringFromMap(data, "category")); category == "" {
		data["category"] = "standard"
	}

	capabilities := connectorCapabilitiesFromData(data)
	if len(capabilities) > 0 {
		data["capabilities"] = capabilities
	}

	if mode := connectorCommunicationModeFromData(data); mode != "" {
		data["communication_mode"] = mode
	}
}
