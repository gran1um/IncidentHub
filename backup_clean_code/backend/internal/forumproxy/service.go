package forumproxy

import (
	"context"
	"errors"
	"fmt"
	"incidenthub/backend/internal/connectors/outbound"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"
	"incidenthub/backend/internal/tracing"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Service struct {
	catalog          *repository.CatalogRepository
	bindings         *repository.ForumExternalBindingRepository
	recipientAliases *repository.ConnectorRecipientAliasRepository
	outbound         *outbound.Service
}

type SendParams struct {
	TenantID    uuid.UUID
	ThreadID    uuid.UUID
	ConnectorID uuid.UUID
	BindingKey  string
	Author      string
	Message     string
	Metadata    map[string]any
}

type SendResult struct {
	ConnectorID uuid.UUID      `json:"connector_id"`
	Reply       string         `json:"reply"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type SyncParams struct {
	TenantID    uuid.UUID
	ThreadID    uuid.UUID
	ConnectorID uuid.UUID
	BindingKey  string
	Metadata    map[string]any
}

type SyncResult struct {
	ConnectorID uuid.UUID              `json:"connector_id"`
	Messages    []outbound.PollMessage `json:"messages"`
}

func NewService(
	catalog *repository.CatalogRepository,
	bindings *repository.ForumExternalBindingRepository,
	recipientAliases *repository.ConnectorRecipientAliasRepository,
	outboundSvc *outbound.Service,
) *Service {
	return &Service{
		catalog:          catalog,
		bindings:         bindings,
		recipientAliases: recipientAliases,
		outbound:         outboundSvc,
	}
}

func (s *Service) Send(ctx context.Context, p SendParams) (SendResult, error) {
	ctx, span, startedAt := tracing.StartModuleOperation(ctx, "forumproxy", "send")
	var err error
	defer func() {
		tracing.FinishModuleOperation(span, startedAt, "forumproxy", "send", err)
	}()

	if strings.TrimSpace(p.Message) == "" {
		err = fmt.Errorf("message is required")
		return SendResult{}, err
	}
	connector, err := s.resolveOutboundConnector(ctx, p.TenantID, p.ConnectorID)
	if err != nil {
		return SendResult{}, err
	}
	metadata, err := s.resolveSendMetadataRecipients(ctx, p.TenantID, p.ConnectorID, *connector, normalizeMetadataMap(p.Metadata))
	if err != nil {
		return SendResult{}, err
	}
	bindingKey := normalizeForumProxyBindingKey(p.BindingKey)
	if bindingKey != "" {
		metadata["binding_key"] = bindingKey
	}

	binding, _ := s.bindings.GetByThreadConnectorAndKey(ctx, p.TenantID, p.ThreadID, p.ConnectorID, bindingKey)
	conversationID := ""
	cursor := ""
	if binding != nil {
		conversationID = binding.ConversationID
		cursor = binding.Cursor
	}

	result, err := s.outbound.Send(ctx, *connector, outbound.SendRequest{
		ThreadID:       p.ThreadID.String(),
		ConversationID: conversationID,
		Author:         p.Author,
		Message:        p.Message,
		Metadata:       metadata,
	})
	if err != nil {
		return SendResult{}, err
	}

	_, upsertErr := s.bindings.Upsert(ctx, p.TenantID, p.ThreadID, p.ConnectorID, repository.ForumExternalBindingUpsertParams{
		BindingKey:     bindingKey,
		ConversationID: result.ConversationID,
		Cursor:         result.Cursor,
		Metadata:       result.Metadata,
	})
	if upsertErr != nil {
		err = upsertErr
		return SendResult{}, err
	}

	_ = cursor // reserved for future advanced sync logic.

	return SendResult{
		ConnectorID: p.ConnectorID,
		Reply:       result.Reply,
		Metadata:    result.Metadata,
	}, nil
}

func (s *Service) Sync(ctx context.Context, p SyncParams) (SyncResult, error) {
	ctx, span, startedAt := tracing.StartModuleOperation(ctx, "forumproxy", "sync")
	var err error
	defer func() {
		tracing.FinishModuleOperation(span, startedAt, "forumproxy", "sync", err)
	}()

	connector, err := s.resolveOutboundConnector(ctx, p.TenantID, p.ConnectorID)
	if err != nil {
		return SyncResult{}, err
	}
	bindingKey := normalizeForumProxyBindingKey(p.BindingKey)
	pollMetadata := normalizeMetadataMap(p.Metadata)
	if bindingKey != "" {
		pollMetadata["binding_key"] = bindingKey
	}

	binding, err := s.bindings.GetByThreadConnectorAndKey(ctx, p.TenantID, p.ThreadID, p.ConnectorID, bindingKey)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return SyncResult{}, err
	}

	conversationID := ""
	cursor := ""
	if binding != nil {
		conversationID = binding.ConversationID
		cursor = binding.Cursor
	}

	result, err := s.outbound.Poll(ctx, *connector, outbound.PollRequest{
		ThreadID:       p.ThreadID.String(),
		ConversationID: conversationID,
		Cursor:         cursor,
		Metadata:       pollMetadata,
	})
	if err != nil {
		return SyncResult{}, err
	}
	if captureErr := s.captureRecipientAliases(ctx, p.TenantID, p.ConnectorID, *connector, result.Messages); captureErr != nil {
		err = captureErr
		return SyncResult{}, err
	}

	_, err = s.bindings.Upsert(ctx, p.TenantID, p.ThreadID, p.ConnectorID, repository.ForumExternalBindingUpsertParams{
		BindingKey:     bindingKey,
		ConversationID: result.ConversationID,
		Cursor:         result.NextCursor,
		Metadata:       result.Metadata,
		LastSyncedAt:   true,
	})
	if err != nil {
		return SyncResult{}, err
	}

	return SyncResult{
		ConnectorID: p.ConnectorID,
		Messages:    result.Messages,
	}, nil
}

func (s *Service) resolveOutboundConnector(ctx context.Context, tenantID, connectorID uuid.UUID) (*models.CatalogItem, error) {
	if s.catalog == nil {
		return nil, fmt.Errorf("catalog repository is not configured")
	}
	if s.outbound == nil {
		return nil, fmt.Errorf("outbound connector service is not configured")
	}
	connector, err := s.catalog.GetByID(ctx, "outbound_connectors", connectorID, &tenantID)
	if err != nil {
		legacy, legacyErr := s.catalog.GetByID(ctx, "connectors", connectorID, &tenantID)
		if legacyErr != nil {
			return nil, fmt.Errorf("load outbound connector: %w", err)
		}
		connector = legacy
	}

	direction := strings.ToLower(strings.TrimSpace(fmt.Sprint(connector.Data["direction"])))
	if direction == "" {
		if strings.EqualFold(strings.TrimSpace(connector.Kind), "outbound_connectors") {
			direction = "outbound"
		}
	}
	if direction == "" {
		direction = "outbound"
	}
	if direction != "outbound" {
		return nil, fmt.Errorf("connector %s is not outbound", connectorID.String())
	}
	enabled := true
	if raw, exists := connector.Data["enabled"]; exists {
		if flag, ok := raw.(bool); ok {
			enabled = flag
		}
	}
	if !enabled {
		return nil, fmt.Errorf("connector %s is disabled", connectorID.String())
	}

	return connector, nil
}

func (s *Service) resolveSendMetadataRecipients(ctx context.Context, tenantID, connectorID uuid.UUID, connector models.CatalogItem, metadata map[string]any) (map[string]any, error) {
	channel := connectorChannel(connector)
	if channel != "telegram" {
		return metadata, nil
	}

	target := communicationRecipientTarget(metadata)
	if strings.TrimSpace(target) == "" {
		return metadata, nil
	}
	if isTelegramChatID(target) {
		return withResolvedTelegramRecipient(metadata, strings.TrimSpace(target)), nil
	}
	if s.recipientAliases == nil {
		return metadata, nil
	}

	recipientID, err := s.recipientAliases.ResolveRecipientID(ctx, tenantID, connectorID, channel, telegramRecipientLookupCandidates(target, metadata))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return metadata, nil
		}
		return nil, err
	}
	if strings.TrimSpace(recipientID) == "" {
		return metadata, nil
	}
	return withResolvedTelegramRecipient(metadata, recipientID), nil
}

func (s *Service) captureRecipientAliases(ctx context.Context, tenantID, connectorID uuid.UUID, connector models.CatalogItem, messages []outbound.PollMessage) error {
	if s.recipientAliases == nil || len(messages) == 0 {
		return nil
	}
	channel := connectorChannel(connector)
	if channel != "telegram" {
		return nil
	}

	for _, message := range messages {
		recipientID := strings.TrimSpace(firstStringFromMap(message.Metadata, "chat_id", "chatId", "recipient_id", "recipientId"))
		if recipientID == "" {
			continue
		}
		aliases := map[string]string{
			"chat_id": recipientID,
		}
		if username := strings.TrimSpace(firstStringFromMap(message.Metadata, "username", "telegram_username")); username != "" {
			aliases["username"] = username
		}
		if externalUserID := strings.TrimSpace(firstStringFromMap(message.Metadata, "external_user_id", "externalUserId", "user_id", "userId")); externalUserID != "" {
			aliases["external_user_id"] = externalUserID
		}
		if target := strings.TrimSpace(firstStringFromMap(message.Metadata, "target", "recipient")); target != "" {
			aliases["target"] = target
		}

		displayName := strings.TrimSpace(firstStringFromMap(message.Metadata, "display_name", "displayName"))
		if displayName == "" {
			displayName = strings.TrimSpace(message.Author)
		}

		if err := s.recipientAliases.UpsertAliases(ctx, tenantID, connectorID, repository.ConnectorRecipientAliasUpsertParams{
			Channel:     channel,
			RecipientID: recipientID,
			DisplayName: displayName,
			Metadata:    normalizeMetadataMap(message.Metadata),
			Aliases:     aliases,
		}); err != nil {
			return err
		}
	}
	return nil
}

func connectorChannel(connector models.CatalogItem) string {
	return strings.ToLower(strings.TrimSpace(firstStringFromMap(connector.Data, "channel", "type", "provider")))
}

func communicationRecipientTarget(metadata map[string]any) string {
	target := strings.TrimSpace(firstStringFromMap(metadata,
		"chat_id",
		"chatId",
		"recipient_chat_id",
		"recipientChatId",
		"recipient",
		"target",
		"external_user_id",
		"externalUserId",
		"username",
	))
	if target != "" {
		return target
	}
	participant := metadata["participant"]
	participantMap, ok := participant.(map[string]any)
	if !ok {
		return ""
	}
	return strings.TrimSpace(firstStringFromMap(participantMap,
		"chat_id",
		"chatId",
		"recipient_chat_id",
		"recipientChatId",
		"recipient",
		"target",
		"external_user_id",
		"externalUserId",
		"username",
	))
}

func telegramRecipientLookupCandidates(target string, metadata map[string]any) []string {
	candidates := []string{target}
	if stripped := strings.TrimSpace(strings.TrimPrefix(target, "@")); stripped != "" && stripped != target {
		candidates = append(candidates, stripped)
	}
	candidates = append(candidates,
		firstStringFromMap(metadata, "username", "external_user_id", "externalUserId", "chat_id", "chatId", "target", "recipient"),
	)
	if participant, ok := metadata["participant"].(map[string]any); ok {
		candidates = append(candidates,
			firstStringFromMap(participant, "username", "external_user_id", "externalUserId", "chat_id", "chatId", "target", "recipient"),
		)
	}
	out := make([]string, 0, len(candidates))
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		normalized := normalizeAliasCandidate(candidate)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

func withResolvedTelegramRecipient(metadata map[string]any, recipientID string) map[string]any {
	out := normalizeMetadataMap(metadata)
	normalizedRecipient := strings.TrimSpace(recipientID)
	if normalizedRecipient == "" {
		return out
	}
	out["chat_id"] = normalizedRecipient
	out["recipient_chat_id"] = normalizedRecipient
	participant := normalizeMetadataMap(out["participant"])
	participant["chat_id"] = normalizedRecipient
	out["participant"] = participant
	return out
}

func isTelegramChatID(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false
	}
	trimmed = strings.TrimPrefix(trimmed, "-")
	if trimmed == "" {
		return false
	}
	for _, ch := range trimmed {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func normalizeMetadataMap(input any) map[string]any {
	if input == nil {
		return map[string]any{}
	}
	typed, ok := input.(map[string]any)
	if !ok {
		return map[string]any{}
	}
	out := make(map[string]any, len(typed))
	for key, value := range typed {
		out[key] = value
	}
	return out
}

func firstStringFromMap(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := payload[key]; ok {
			trimmed := strings.TrimSpace(fmt.Sprint(value))
			if trimmed != "" && trimmed != "<nil>" {
				return trimmed
			}
		}
	}
	return ""
}

func normalizeAliasCandidate(raw string) string {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	normalized = strings.TrimPrefix(normalized, "@")
	return normalized
}

func normalizeForumProxyBindingKey(raw string) string {
	return strings.TrimSpace(raw)
}
