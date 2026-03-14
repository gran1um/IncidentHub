package api

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"incidenthub/backend/internal/middleware"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"
	"incidenthub/backend/internal/workflow"
	"incidenthub/backend/internal/workflowvault"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

//nolint:gosec // Not a credential: catalog kind identifier.
const workflowVaultSecretKind = "workflow_vault_secrets"

type workflowVaultCreateSecretRequest struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Field    string `json:"field"`
	NodeType string `json:"node_type"`
}

func (h *Handler) ListWorkflowVaultSecrets(c *echo.Context) error {
	if h.catalog == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "catalog repository is not configured")
	}
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	items, err := h.catalog.List(c.Request().Context(), repository.CatalogListParams{
		Kind:     workflowVaultSecretKind,
		TenantID: &tenantID,
		Limit:    500,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list workflow vault secrets")
	}
	fieldFilter := strings.TrimSpace(c.QueryParam("field"))
	nodeTypeFilter := strings.TrimSpace(strings.ToLower(c.QueryParam("node_type")))
	payload := make([]map[string]any, 0, len(items))
	for _, item := range items {
		fieldID := strings.TrimSpace(stringFromMap(item.Data, "field"))
		nodeType := strings.TrimSpace(strings.ToLower(stringFromMap(item.Data, "node_type")))
		if fieldFilter != "" && fieldID != fieldFilter {
			continue
		}
		if nodeTypeFilter != "" && nodeType != nodeTypeFilter {
			continue
		}
		payload = append(payload, workflowVaultSecretItemToPayload(item))
	}
	return c.JSON(http.StatusOK, payload)
}

func (h *Handler) CreateWorkflowVaultSecret(c *echo.Context) error {
	if h.catalog == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "catalog repository is not configured")
	}
	identity, _ := middleware.GetIdentity(c)
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	var req workflowVaultCreateSecretRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	item, err := h.createWorkflowVaultSecretRecord(c.Request().Context(), tenantID, &identity.UserID, req.Name, req.Value, req.Field, req.NodeType)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return c.JSON(http.StatusCreated, workflowVaultSecretItemToPayload(*item))
}

func (h *Handler) deleteCatalogItemByID(
	c *echo.Context,
	kind string,
	paramName string,
	invalidIDMsg string,
	notFoundMsg string,
	deleteFailedMsg string,
) error {
	if h.catalog == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "catalog repository is not configured")
	}
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	itemID, err := uuid.Parse(strings.TrimSpace(c.Param(paramName)))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, invalidIDMsg)
	}
	if _, err := h.catalog.GetByID(c.Request().Context(), kind, itemID, &tenantID); err != nil {
		return echo.NewHTTPError(http.StatusNotFound, notFoundMsg)
	}
	if err := h.catalog.Delete(c.Request().Context(), kind, itemID, &tenantID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, deleteFailedMsg)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) DeleteWorkflowVaultSecret(c *echo.Context) error {
	return h.deleteCatalogItemByID(
		c,
		workflowVaultSecretKind,
		"secretID",
		"invalid secret id",
		"workflow vault secret not found",
		"failed to delete workflow vault secret",
	)
}

func workflowVaultSecretItemToPayload(item models.CatalogItem) map[string]any {
	payload := map[string]any{
		"id":         item.ID.String(),
		"kind":       item.Kind,
		"name":       strings.TrimSpace(stringFromMap(item.Data, "name")),
		"field":      strings.TrimSpace(stringFromMap(item.Data, "field")),
		"node_type":  strings.TrimSpace(stringFromMap(item.Data, "node_type")),
		"created_at": item.CreatedAt,
		"updated_at": item.UpdatedAt,
	}
	if item.TenantID != nil {
		payload["tenant_id"] = item.TenantID.String()
	}
	return payload
}

func (h *Handler) createWorkflowVaultSecretRecord(
	ctx context.Context,
	tenantID uuid.UUID,
	actorID *uuid.UUID,
	name string,
	value string,
	fieldID string,
	nodeType string,
) (*models.CatalogItem, error) {
	if h.catalog == nil {
		return nil, fmt.Errorf("catalog repository is not configured")
	}
	trimmedValue := strings.TrimSpace(value)
	if trimmedValue == "" {
		return nil, fmt.Errorf("secret value is required")
	}
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		trimmedName = firstNonEmptyString(strings.TrimSpace(fieldID), "workflow secret")
	}
	ciphertext, nonce, err := h.encryptWorkflowVaultSecret(trimmedValue)
	if err != nil {
		return nil, fmt.Errorf("encrypt workflow secret: %w", err)
	}
	item, err := h.catalog.Create(ctx, repository.CatalogCreateParams{
		TenantID: &tenantID,
		Kind:     workflowVaultSecretKind,
		OwnerID:  actorID,
		Data: map[string]any{
			"name":       trimmedName,
			"field":      strings.TrimSpace(fieldID),
			"node_type":  strings.TrimSpace(strings.ToLower(nodeType)),
			"ciphertext": ciphertext,
			"nonce":      nonce,
			"version":    1,
		},
		CreatedBy: actorID,
	})
	if err != nil {
		return nil, fmt.Errorf("create workflow vault secret: %w", err)
	}
	return item, nil
}

func (h *Handler) workflowVaultKey() []byte {
	seed := strings.TrimSpace(h.cfg.Workflow.VaultKey)
	if seed == "" {
		seed = strings.TrimSpace(h.cfg.Auth.JWTSecret) + ":workflow-vault"
	}
	sum := sha256.Sum256([]byte(seed))
	key := make([]byte, len(sum))
	copy(key, sum[:])
	return key
}

func (h *Handler) encryptWorkflowVaultSecret(value string) (ciphertextEncoded string, nonceEncoded string, err error) {
	block, err := aes.NewCipher(h.workflowVaultKey())
	if err != nil {
		return "", "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", "", err
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(value), nil)
	return base64.RawStdEncoding.EncodeToString(ciphertext), base64.RawStdEncoding.EncodeToString(nonce), nil
}

func (h *Handler) decryptWorkflowVaultSecret(ciphertextEncoded string, nonceEncoded string) (string, error) {
	ciphertext, err := base64.RawStdEncoding.DecodeString(strings.TrimSpace(ciphertextEncoded))
	if err != nil {
		return "", err
	}
	nonce, err := base64.RawStdEncoding.DecodeString(strings.TrimSpace(nonceEncoded))
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(h.workflowVaultKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func (h *Handler) loadWorkflowVaultSecretValue(ctx context.Context, tenantID uuid.UUID, secretID uuid.UUID) (string, error) {
	item, err := h.catalog.GetByID(ctx, workflowVaultSecretKind, secretID, &tenantID)
	if err != nil {
		return "", err
	}
	return h.decryptWorkflowVaultSecret(
		strings.TrimSpace(stringFromMap(item.Data, "ciphertext")),
		strings.TrimSpace(stringFromMap(item.Data, "nonce")),
	)
}

func (h *Handler) resolveWorkflowDefinitionVaultRefs(ctx context.Context, tenantID uuid.UUID, definition *workflow.Definition) (*workflow.Definition, error) {
	if definition == nil {
		return nil, nil
	}
	resolved := &workflow.Definition{
		EntryNodeID: definition.EntryNodeID,
		Nodes:       make([]workflow.Node, 0, len(definition.Nodes)),
		Edges:       append([]workflow.Edge{}, definition.Edges...),
	}
	for _, node := range definition.Nodes {
		resolvedConfig, err := h.resolveWorkflowVaultRefsValue(ctx, tenantID, node.Config)
		if err != nil {
			return nil, err
		}
		configMap, _ := resolvedConfig.(map[string]any)
		if configMap == nil {
			configMap = map[string]any{}
		}
		resolved.Nodes = append(resolved.Nodes, workflow.Node{
			ID:     node.ID,
			Type:   node.Type,
			Label:  node.Label,
			Config: configMap,
		})
	}
	return resolved, nil
}

func (h *Handler) resolveWorkflowVaultRefsValue(ctx context.Context, tenantID uuid.UUID, raw any) (any, error) {
	switch typed := raw.(type) {
	case string:
		secretID, ok := workflowvault.ParseRef(typed)
		if !ok {
			return typed, nil
		}
		return h.loadWorkflowVaultSecretValue(ctx, tenantID, secretID)
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			resolved, err := h.resolveWorkflowVaultRefsValue(ctx, tenantID, item)
			if err != nil {
				return nil, err
			}
			out = append(out, resolved)
		}
		return out, nil
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, value := range typed {
			resolved, err := h.resolveWorkflowVaultRefsValue(ctx, tenantID, value)
			if err != nil {
				return nil, err
			}
			out[key] = resolved
		}
		return out, nil
	default:
		return raw, nil
	}
}

func (h *Handler) vaultizeWorkflowDefinitionPayload(ctx context.Context, tenantID uuid.UUID, actorID uuid.UUID, workflowName string, raw any) (any, error) {
	if raw == nil {
		return nil, nil
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal workflow definition: %w", err)
	}
	payload := map[string]any{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		return nil, fmt.Errorf("decode workflow definition: %w", err)
	}
	nodesRaw, _ := payload["nodes"].([]any)
	if len(nodesRaw) == 0 {
		return payload, nil
	}
	for idx, rawNode := range nodesRaw {
		nodeMap, ok := rawNode.(map[string]any)
		if !ok {
			continue
		}
		nodeType := strings.TrimSpace(strings.ToLower(stringFromMap(nodeMap, "type")))
		sensitiveFields := workflowvault.SensitiveFields(nodeType)
		if len(sensitiveFields) == 0 {
			continue
		}
		configMap, ok := nodeMap["config"].(map[string]any)
		if !ok {
			configMap = map[string]any{}
		}
		nodeLabel := firstNonEmptyString(strings.TrimSpace(stringFromMap(nodeMap, "label", "name")), nodeType, fmt.Sprintf("node_%d", idx+1))
		for fieldID := range sensitiveFields {
			currentValue := strings.TrimSpace(toWorkflowVaultString(configMap[fieldID]))
			if currentValue == "" || workflowvault.IsRef(currentValue) {
				continue
			}
			secretName := fmt.Sprintf("%s / %s / %s", firstNonEmptyString(strings.TrimSpace(workflowName), "Workflow"), nodeLabel, fieldID)
			secretItem, err := h.createWorkflowVaultSecretRecord(ctx, tenantID, &actorID, secretName, currentValue, fieldID, nodeType)
			if err != nil {
				return nil, err
			}
			configMap[fieldID] = workflowvault.RefPrefix + secretItem.ID.String()
		}
		nodeMap["config"] = configMap
		nodesRaw[idx] = nodeMap
	}
	payload["nodes"] = nodesRaw
	return payload, nil
}

func toWorkflowVaultString(raw any) string {
	switch typed := raw.(type) {
	case string:
		return typed
	case map[string]any, []any:
		encoded, err := json.Marshal(typed)
		if err == nil {
			return string(encoded)
		}
	case fmt.Stringer:
		return typed.String()
	}
	text := strings.TrimSpace(fmt.Sprint(raw))
	if text == "<nil>" {
		return ""
	}
	return text
}

//nolint:gosec // Not a credential: catalog kind identifier.
const workflowCredentialKind = "workflow_credential"

type workflowCredentialPayload struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	NodeType    string            `json:"node_type"`
	Fields      map[string]string `json:"fields"`
	CreatedAt   any               `json:"created_at"`
	UpdatedAt   any               `json:"updated_at"`
}

type workflowCredentialRequest struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	NodeType    string            `json:"node_type"`
	Fields      map[string]string `json:"fields"`
}

func (h *Handler) ListWorkflowCredentials(c *echo.Context) error {
	if h.catalog == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "catalog repository is not configured")
	}
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	nodeTypeFilter := strings.TrimSpace(strings.ToLower(c.QueryParam("node_type")))
	items, err := h.catalog.List(c.Request().Context(), repository.CatalogListParams{
		Kind:     workflowCredentialKind,
		TenantID: &tenantID,
		Limit:    500,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list workflow credentials")
	}
	payload := make([]workflowCredentialPayload, 0, len(items))
	for _, item := range items {
		data := normalizeMap(item.Data)
		nodeType := strings.TrimSpace(strings.ToLower(stringFromMap(data, "node_type")))
		if nodeTypeFilter != "" && nodeType != nodeTypeFilter {
			continue
		}
		fields := map[string]string{}
		for key, value := range normalizeMap(firstMapValue(data, "fields")) {
			fields[key] = strings.TrimSpace(fmt.Sprint(value))
		}
		payload = append(payload, workflowCredentialPayload{
			ID:          item.ID.String(),
			Name:        strings.TrimSpace(stringFromMap(data, "name")),
			Description: strings.TrimSpace(stringFromMap(data, "description")),
			NodeType:    nodeType,
			Fields:      fields,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
		})
	}
	return c.JSON(http.StatusOK, payload)
}

func (h *Handler) CreateWorkflowCredential(c *echo.Context) error {
	if h.catalog == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "catalog repository is not configured")
	}
	identity, _ := middleware.GetIdentity(c)
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	var req workflowCredentialRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	item, err := h.upsertWorkflowCredentialRecord(c.Request().Context(), tenantID, nil, &identity.UserID, req)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	data := normalizeMap(item.Data)
	fields := map[string]string{}
	for key, value := range normalizeMap(firstMapValue(data, "fields")) {
		fields[key] = strings.TrimSpace(fmt.Sprint(value))
	}
	return c.JSON(http.StatusCreated, workflowCredentialPayload{
		ID:          item.ID.String(),
		Name:        strings.TrimSpace(stringFromMap(data, "name")),
		Description: strings.TrimSpace(stringFromMap(data, "description")),
		NodeType:    strings.TrimSpace(strings.ToLower(stringFromMap(data, "node_type"))),
		Fields:      fields,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	})
}

func (h *Handler) UpdateWorkflowCredential(c *echo.Context) error {
	if h.catalog == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "catalog repository is not configured")
	}
	identity, _ := middleware.GetIdentity(c)
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	credentialID, err := uuid.Parse(strings.TrimSpace(c.Param("credentialID")))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid credential id")
	}
	if _, getErr := h.catalog.GetByID(c.Request().Context(), workflowCredentialKind, credentialID, &tenantID); getErr != nil {
		return echo.NewHTTPError(http.StatusNotFound, "workflow credential not found")
	}
	var req workflowCredentialRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	item, err := h.upsertWorkflowCredentialRecord(c.Request().Context(), tenantID, &credentialID, &identity.UserID, req)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	data := normalizeMap(item.Data)
	fields := map[string]string{}
	for key, value := range normalizeMap(firstMapValue(data, "fields")) {
		fields[key] = strings.TrimSpace(fmt.Sprint(value))
	}
	return c.JSON(http.StatusOK, workflowCredentialPayload{
		ID:          item.ID.String(),
		Name:        strings.TrimSpace(stringFromMap(data, "name")),
		Description: strings.TrimSpace(stringFromMap(data, "description")),
		NodeType:    strings.TrimSpace(strings.ToLower(stringFromMap(data, "node_type"))),
		Fields:      fields,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	})
}

func (h *Handler) DeleteWorkflowCredential(c *echo.Context) error {
	return h.deleteCatalogItemByID(
		c,
		workflowCredentialKind,
		"credentialID",
		"invalid credential id",
		"workflow credential not found",
		"failed to delete workflow credential",
	)
}

func (h *Handler) upsertWorkflowCredentialRecord(
	ctx context.Context,
	tenantID uuid.UUID,
	credentialID *uuid.UUID,
	actorID *uuid.UUID,
	req workflowCredentialRequest,
) (*models.CatalogItem, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	nodeType := strings.TrimSpace(strings.ToLower(req.NodeType))
	if nodeType == "" {
		return nil, fmt.Errorf("node_type is required")
	}
	sensitive := workflowvault.SensitiveFields(nodeType)
	fields := map[string]any{}
	for key, value := range req.Fields {
		fieldID := strings.TrimSpace(key)
		if fieldID == "" {
			continue
		}
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		// If field is marked sensitive for this node type and value is not already a vault ref,
		// create a workflow vault secret and store vault://<id> instead of raw value.
		if _, isSensitive := sensitive[fieldID]; isSensitive && !workflowvault.IsRef(trimmed) {
			secretName := fmt.Sprintf("%s / %s / %s", name, nodeType, fieldID)
			secret, err := h.createWorkflowVaultSecretRecord(ctx, tenantID, actorID, secretName, trimmed, fieldID, nodeType)
			if err != nil {
				return nil, fmt.Errorf("create workflow credential secret for %s: %w", fieldID, err)
			}
			fields[fieldID] = workflowvault.RefPrefix + secret.ID.String()
			continue
		}
		fields[fieldID] = trimmed
	}
	data := map[string]any{
		"name":        name,
		"description": strings.TrimSpace(req.Description),
		"node_type":   nodeType,
		"fields":      fields,
	}
	if credentialID == nil {
		item, err := h.catalog.Create(ctx, repository.CatalogCreateParams{
			TenantID:  &tenantID,
			Kind:      workflowCredentialKind,
			OwnerID:   actorID,
			Data:      data,
			CreatedBy: actorID,
		})
		if err != nil {
			return nil, fmt.Errorf("create workflow credential: %w", err)
		}
		return item, nil
	}
	item, err := h.catalog.Update(ctx, workflowCredentialKind, *credentialID, &tenantID, repository.CatalogUpdateParams{Data: data})
	if err != nil {
		return nil, fmt.Errorf("update workflow credential: %w", err)
	}
	return item, nil
}

func (h *Handler) applyWorkflowCredentials(ctx context.Context, tenantID uuid.UUID, definition *workflow.Definition) *workflow.Definition {
	if definition == nil {
		return nil
	}
	resolved := &workflow.Definition{
		EntryNodeID: definition.EntryNodeID,
		Nodes:       make([]workflow.Node, 0, len(definition.Nodes)),
		Edges:       append([]workflow.Edge{}, definition.Edges...),
	}
	for _, node := range definition.Nodes {
		config := map[string]any{}
		for key, value := range node.Config {
			config[key] = value
		}
		if rawID, ok := config["credential_id"]; ok {
			credID := strings.TrimSpace(fmt.Sprint(rawID))
			if credID != "" && h.catalog != nil {
				if uuidID, err := uuid.Parse(credID); err == nil {
					item, err := h.catalog.GetByID(ctx, workflowCredentialKind, uuidID, &tenantID)
					if err == nil {
						data := normalizeMap(item.Data)
						fields := normalizeMap(firstMapValue(data, "fields"))
						for key, value := range fields {
							config[key] = value
						}
					}
				}
			}
		}
		resolved.Nodes = append(resolved.Nodes, workflow.Node{
			ID:     node.ID,
			Type:   node.Type,
			Label:  node.Label,
			Config: config,
		})
	}
	return resolved
}
