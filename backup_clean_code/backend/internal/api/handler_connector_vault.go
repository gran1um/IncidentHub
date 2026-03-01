package api

import (
	"context"
	"fmt"
	"strings"

	"incidenthub/backend/internal/workflowvault"

	"github.com/google/uuid"
)

// shouldVaultizeConnectorType determines whether the given connector type/category
// should have its config secrets stored in the workflow vault. We focus on
// transports that typically contain credentials and are used by the new
// Connector Hub runtime and security integrations.
func shouldVaultizeConnectorType(connectorType string, category string) bool {
	t := strings.ToUpper(strings.TrimSpace(connectorType))
	_ = strings.ToLower(strings.TrimSpace(category))
	switch t {
	case "HTTP", "WEBHOOK", "SQL", "S3", "KAFKA", "REDIS", "SMTP", "TELEGRAM", "SLACK", "OUTLOOK", "TIME":
		return true
	default:
		return false
	}
}

// vaultizeConnectorConfig walks the connector config and moves sensitive values
// (passwords, API keys, access keys, etc.) into the workflow vault. The config
// map then stores only vault://<id> references, so secrets never live in the
// catalog in plaintext.
func (h *Handler) vaultizeConnectorConfig(
	ctx context.Context,
	tenantID uuid.UUID,
	actorID uuid.UUID,
	connectorName string,
	connectorType string,
	category string,
	rawConfig any,
) (map[string]any, error) {
	// If this connector type is not marked for vaultization, just normalize
	// the config into a map and return as-is.
	if !shouldVaultizeConnectorType(connectorType, category) {
		cfg := normalizeMap(rawConfig)
		if cfg == nil {
			cfg = map[string]any{}
		}
		return cfg, nil
	}

	cfg := normalizeMap(rawConfig)
	if cfg == nil {
		cfg = map[string]any{}
	}

	nodeType := "connector_" + strings.ToLower(strings.TrimSpace(connectorType))
	displayName := firstNonEmptyString(strings.TrimSpace(connectorName), strings.TrimSpace(connectorType), "Connector")

	// Define candidate sensitive fields per connector transport.
	sensitiveFields := map[string]struct{}{}
	switch strings.ToUpper(strings.TrimSpace(connectorType)) {
	case "HTTP", "WEBHOOK":
		for _, field := range []string{"apiKey", "api_key", "bearerToken", "token", "password"} {
			sensitiveFields[field] = struct{}{}
		}
	case "SQL":
		for _, field := range []string{"password", "dsn"} {
			sensitiveFields[field] = struct{}{}
		}
	case "S3":
		for _, field := range []string{
			"accessKey", "access_key", "accessKeyId", "access_key_id",
			"secretKey", "secret_key", "secretAccessKey", "secret_access_key",
		} {
			sensitiveFields[field] = struct{}{}
		}
	case "KAFKA":
		for _, field := range []string{"password", "saslPassword", "sasl_password"} {
			sensitiveFields[field] = struct{}{}
		}
	case "REDIS":
		for _, field := range []string{"password"} {
			sensitiveFields[field] = struct{}{}
		}
	case "SMTP":
		for _, field := range []string{"password", "apiKey", "api_key", "token", "bearerToken"} {
			sensitiveFields[field] = struct{}{}
		}
	case "TELEGRAM":
		for _, field := range []string{"botToken", "token", "accessToken"} {
			sensitiveFields[field] = struct{}{}
		}
	case "SLACK":
		for _, field := range []string{"botToken", "token", "accessToken"} {
			sensitiveFields[field] = struct{}{}
		}
	case "OUTLOOK":
		for _, field := range []string{"clientSecret", "client_secret", "secret", "token", "bearerToken"} {
			sensitiveFields[field] = struct{}{}
		}
	case "TIME":
		for _, field := range []string{"token", "apiKey", "api_key", "password"} {
			sensitiveFields[field] = struct{}{}
		}
	}

	out := make(map[string]any, len(cfg))
	for key, value := range cfg {
		out[key] = value
	}

	for fieldID := range sensitiveFields {
		raw, ok := cfg[fieldID]
		if !ok {
			continue
		}
		text := strings.TrimSpace(toWorkflowVaultString(raw))
		if text == "" || workflowvault.IsRef(text) {
			continue
		}
		secretName := fmt.Sprintf("%s / %s / %s", displayName, nodeType, fieldID)
		secretItem, err := h.createWorkflowVaultSecretRecord(ctx, tenantID, &actorID, secretName, text, fieldID, nodeType)
		if err != nil {
			return nil, fmt.Errorf("create connector vault secret for %s: %w", fieldID, err)
		}
		out[fieldID] = workflowvault.RefPrefix + secretItem.ID.String()
	}

	return out, nil
}
