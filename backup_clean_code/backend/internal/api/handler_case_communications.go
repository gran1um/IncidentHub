package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"incidenthub/backend/internal/connectors/outbound"
	"incidenthub/backend/internal/forumproxy"
	"incidenthub/backend/internal/middleware"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"
	"incidenthub/backend/internal/workflow"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

const (
	caseCommunicationThreadKind   = "case_communication_thread"
	caseCommunicationMessageKind  = "case_communication_message"
	caseCommunicationTemplateKind = "communication_templates"
)

type createCaseCommunicationRequest struct {
	Title       string         `json:"title"`
	Status      string         `json:"status"`
	ConnectorID string         `json:"connector_id"`
	Channel     string         `json:"channel"`
	Subject     string         `json:"subject"`
	Participant map[string]any `json:"participant"`
	Metadata    map[string]any `json:"metadata"`
}

type sendCaseCommunicationMessageRequest struct {
	ConnectorID    string         `json:"connector_id"`
	Content        string         `json:"content"`
	Author         string         `json:"author"`
	Metadata       map[string]any `json:"metadata"`
	TemplateID     string         `json:"template_id"`
	TemplateVars   map[string]any `json:"template_vars"`
	Subject        string         `json:"subject"`
	NotifyManagers *bool          `json:"notify_managers"`
}

type syncCaseCommunicationRequest struct {
	ConnectorID string `json:"connector_id"`
	Subject     string `json:"subject"`
}

func (h *Handler) ListCommunicationConnectors(c *echo.Context) error {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	items, err := h.listDirectionalConnectors(c, tenantID, "outbound")
	if err != nil {
		return err
	}

	payload := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if raw, exists := item.Data["enabled"]; exists {
			if enabled, ok := raw.(bool); ok && !enabled {
				continue
			}
		}
		if !connectorHasCapabilityItem(item, connectorCapabilityCaseCommunications) && !connectorHasCapabilityItem(item, connectorCapabilityForumThreads) {
			continue
		}
		channel := connectorChannelFromItem(item)
		communicationMode := connectorCommunicationModeFromItem(item)
		if channel == "" || communicationMode == "" {
			continue
		}
		row := catalogItemToPayload(item)
		row["channel"] = channel
		row["type"] = normalizeConnectorType(stringFromMap(item.Data, "type", "provider"))
		row["communication_mode"] = communicationMode
		row["capabilities"] = connectorCapabilitiesFromData(item.Data)
		payload = append(payload, row)
	}
	return c.JSON(http.StatusOK, payload)
}

func (h *Handler) ListCaseCommunications(c *echo.Context) error {
	tenantID, caseID, err := h.resolveCaseInTenant(c)
	if err != nil {
		return err
	}

	items, err := h.catalog.List(c.Request().Context(), repository.CatalogListParams{
		Kind:     caseCommunicationThreadKind,
		TenantID: &tenantID,
		RefID:    &caseID,
		Limit:    500,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list case communications")
	}
	return c.JSON(http.StatusOK, mapCatalogItems(items))
}

func (h *Handler) GetCaseCommunication(c *echo.Context) error {
	tenantID, caseID, err := h.resolveCaseInTenant(c)
	if err != nil {
		return err
	}
	threadID, err := uuid.Parse(strings.TrimSpace(c.Param("threadID")))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid thread id")
	}
	thread, err := h.resolveCaseCommunicationThread(c.Request().Context(), tenantID, caseID, threadID)
	if err != nil {
		return err
	}

	messages, listErr := h.listCaseCommunicationMessages(c.Request().Context(), tenantID, threadID)
	if listErr != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list communication messages")
	}

	out := catalogItemToPayload(*thread)
	out["messages"] = messages
	return c.JSON(http.StatusOK, out)
}

func (h *Handler) CreateCaseCommunication(c *echo.Context) error {
	identity, _ := middleware.GetIdentity(c)
	tenantID, caseID, err := h.resolveCaseInTenant(c)
	if err != nil {
		return err
	}

	var req createCaseCommunicationRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	connectorID, err := uuid.Parse(strings.TrimSpace(req.ConnectorID))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "connector_id is required")
	}
	connector, err := h.resolveOutboundConnectorForTenant(c.Request().Context(), tenantID, connectorID)
	if err != nil {
		return err
	}
	if capabilityErr := validateCommunicationConnectorCapability(*connector, connectorCapabilityCaseCommunications); capabilityErr != nil {
		return capabilityErr
	}

	channel := connectorChannelFromItem(*connector)
	if channel == "" {
		channel = strings.ToLower(strings.TrimSpace(req.Channel))
	}
	if channel == "" {
		channel = "custom"
	}
	communicationMode := connectorCommunicationModeFromItem(*connector)

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = strings.TrimSpace(stringFromMap(connector.Data, "name"))
	}
	if title == "" {
		title = "Communication"
	}

	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status == "" {
		status = "open"
	}

	participant := normalizeCommunicationParticipantForConnector(*connector, normalizeCommunicationMap(req.Participant))
	metadata := communicationMetadataForConnector(*connector, normalizeCommunicationMap(req.Metadata))
	data := map[string]any{
		"title":              title,
		"status":             status,
		"case_id":            caseID.String(),
		"tenant_id":          tenantID.String(),
		"channel":            channel,
		"communication_mode": communicationMode,
		"connector_id":       connectorID.String(),
		"participant":        participant,
		"metadata":           metadata,
	}
	subject := strings.TrimSpace(req.Subject)
	if subject == "" {
		subject = strings.TrimSpace(stringFromMap(metadata, "subject", "email_subject", "topic"))
	}
	if subject != "" {
		data["subject"] = subject
	}

	item, err := h.catalog.Create(c.Request().Context(), repository.CatalogCreateParams{
		TenantID:  &tenantID,
		Kind:      caseCommunicationThreadKind,
		OwnerID:   &identity.UserID,
		RefID:     &caseID,
		Data:      data,
		CreatedBy: &identity.UserID,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to create communication thread")
	}
	return c.JSON(http.StatusCreated, catalogItemToPayload(*item))
}

func (h *Handler) SendCaseCommunicationMessage(c *echo.Context) error {
	if h.forumProxy == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "forum proxy service is not configured")
	}
	identity, _ := middleware.GetIdentity(c)
	tenantID, caseID, err := h.resolveCaseInTenant(c)
	if err != nil {
		return err
	}
	threadID, err := uuid.Parse(strings.TrimSpace(c.Param("threadID")))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid thread id")
	}
	thread, err := h.resolveCaseCommunicationThread(c.Request().Context(), tenantID, caseID, threadID)
	if err != nil {
		return err
	}

	var req sendCaseCommunicationMessageRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	req.Content = strings.TrimSpace(req.Content)
	req.Subject = strings.TrimSpace(req.Subject)
	req.TemplateID = strings.TrimSpace(req.TemplateID)
	req.TemplateVars = normalizeCommunicationMap(req.TemplateVars)

	connectorID, err := resolveCommunicationConnectorID(req.ConnectorID, *thread)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "connector_id is required")
	}
	connector, err := h.resolveOutboundConnectorForTenant(c.Request().Context(), tenantID, connectorID)
	if err != nil {
		return err
	}
	if capabilityErr := validateCommunicationConnectorCapability(*connector, connectorCapabilityCaseCommunications); capabilityErr != nil {
		return capabilityErr
	}
	communicationMode := connectorCommunicationModeFromItem(*connector)

	authorName := strings.TrimSpace(req.Author)
	if authorName == "" {
		authorName = identity.Username
	}

	sendMetadata := communicationMetadataForConnector(*connector, mergeCommunicationMaps(normalizeCommunicationMap(thread.Data["metadata"]), req.Metadata))
	sendMetadata = mergeCommunicationMaps(sendMetadata, map[string]any{
		"channel":            thread.Data["channel"],
		"communication_mode": firstNonEmptyString(stringFromMap(thread.Data, "communication_mode"), communicationMode),
		"case_id":            caseID.String(),
		"thread_id":          threadID.String(),
		"participant":        normalizeCommunicationParticipantForConnector(*connector, normalizeCommunicationMap(thread.Data["participant"])),
		"connector_id":       connectorID.String(),
	})

	if req.TemplateID != "" {
		templateID, parseErr := uuid.Parse(req.TemplateID)
		if parseErr != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid template_id")
		}
		template, resolveErr := h.resolveCommunicationTemplate(c.Request().Context(), tenantID, templateID)
		if resolveErr != nil {
			return resolveErr
		}
		templateName := strings.TrimSpace(stringFromMap(template.Data, "name", "title"))
		renderedContent, renderedSubject := renderCommunicationTemplate(*template, buildCommunicationTemplateScope(caseID, threadID, authorName, thread.Data, req.TemplateVars))
		if req.Content == "" {
			req.Content = strings.TrimSpace(renderedContent)
		}
		if req.Subject == "" {
			req.Subject = strings.TrimSpace(renderedSubject)
		}
		sendMetadata = mergeCommunicationMaps(sendMetadata, map[string]any{
			"template_id":   template.ID.String(),
			"template_name": templateName,
		})
	}
	if req.Content == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "content is required")
	}
	if req.Subject != "" {
		sendMetadata["subject"] = req.Subject
	}
	sendMetadata = ensureCommunicationEmailRecipient(sendMetadata, thread.Data)
	if communicationMode == "email" && shouldNotifyManagers(req.NotifyManagers, sendMetadata) {
		managerRecipients := communicationManagerRecipients(thread.Data, sendMetadata)
		if len(managerRecipients) > 0 {
			sendMetadata["cc"] = mergeRecipientTargets(sendMetadata["cc"], managerRecipients)
			sendMetadata["manager_recipients"] = managerRecipients
			sendMetadata["manager_notification"] = true
		}
	}

	now := time.Now().UTC()
	userMessage, err := h.catalog.Create(c.Request().Context(), repository.CatalogCreateParams{
		TenantID:  &tenantID,
		Kind:      caseCommunicationMessageKind,
		OwnerID:   &identity.UserID,
		RefID:     &threadID,
		CreatedBy: &identity.UserID,
		Data: map[string]any{
			"thread_id":       threadID.String(),
			"case_id":         caseID.String(),
			"tenant_id":       tenantID.String(),
			"connector_id":    connectorID.String(),
			"direction":       "outbound",
			"author_id":       identity.UserID.String(),
			"authorName":      authorName,
			"content":         req.Content,
			"subject":         req.Subject,
			"timestamp":       now.Format(time.RFC3339),
			"delivery_status": "pending",
			"metadata":        sendMetadata,
		},
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to create outbound message")
	}

	sendResult, err := h.forumProxy.Send(c.Request().Context(), forumproxy.SendParams{
		TenantID:    tenantID,
		ThreadID:    threadID,
		ConnectorID: connectorID,
		Author:      authorName,
		Message:     req.Content,
		Metadata:    sendMetadata,
	})
	if err != nil {
		_, _ = h.catalog.Update(c.Request().Context(), caseCommunicationMessageKind, userMessage.ID, &tenantID, repository.CatalogUpdateParams{
			Data: map[string]any{
				"delivery_status": "failed",
				"delivery_error":  err.Error(),
			},
		})
		return echo.NewHTTPError(http.StatusBadGateway, "failed to send message to external service")
	}
	messageMetadata := mergeCommunicationMaps(sendMetadata, sendResult.Metadata)

	_, _ = h.catalog.Update(c.Request().Context(), caseCommunicationMessageKind, userMessage.ID, &tenantID, repository.CatalogUpdateParams{
		Data: map[string]any{
			"delivery_status": "sent",
			"metadata":        messageMetadata,
		},
	})
	if updateErr := h.updateCaseCommunicationThreadMetadata(c.Request().Context(), tenantID, threadID, thread.Data, messageMetadata); updateErr != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update communication thread")
	}
	if updateErr := h.updateCaseCommunicationThreadState(c.Request().Context(), tenantID, threadID, now, req.Content); updateErr != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update communication thread")
	}
	h.rewardResponderInvocation(c.Request().Context(), tenantID, identity.UserID, "case_responder:"+userMessage.ID.String(), map[string]any{
		"case_id":      caseID.String(),
		"thread_id":    threadID.String(),
		"message_id":   userMessage.ID.String(),
		"connector_id": connectorID.String(),
		"triggered":    xpEventTypeResponderInvoked,
	})
	h.rewardFirstIncidentMessage(c.Request().Context(), tenantID, identity.UserID, caseID, map[string]any{
		"case_id":      caseID.String(),
		"thread_id":    threadID.String(),
		"message_id":   userMessage.ID.String(),
		"connector_id": connectorID.String(),
		"triggered":    xpEventTypeIncidentFirstMessage,
	})

	var externalReply map[string]any
	if strings.TrimSpace(sendResult.Reply) != "" {
		replyItem, createErr := h.catalog.Create(c.Request().Context(), repository.CatalogCreateParams{
			TenantID:  &tenantID,
			Kind:      caseCommunicationMessageKind,
			OwnerID:   &identity.UserID,
			RefID:     &threadID,
			CreatedBy: &identity.UserID,
			Data: map[string]any{
				"thread_id":       threadID.String(),
				"case_id":         caseID.String(),
				"tenant_id":       tenantID.String(),
				"connector_id":    connectorID.String(),
				"direction":       "inbound",
				"author_id":       "external:" + connectorID.String(),
				"authorName":      "External Service",
				"content":         strings.TrimSpace(sendResult.Reply),
				"subject":         req.Subject,
				"timestamp":       time.Now().UTC().Format(time.RFC3339),
				"delivery_status": "received",
				"metadata":        messageMetadata,
			},
		})
		if createErr == nil {
			externalReply = catalogItemToPayload(*replyItem)
			_ = h.updateCaseCommunicationThreadState(c.Request().Context(), tenantID, threadID, time.Now().UTC(), strings.TrimSpace(sendResult.Reply))
		}
	}

	updatedUserMessage, getErr := h.catalog.GetByID(c.Request().Context(), caseCommunicationMessageKind, userMessage.ID, &tenantID)
	if getErr != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch communication message")
	}

	return c.JSON(http.StatusOK, map[string]any{
		"thread_id":      threadID.String(),
		"connector_id":   connectorID.String(),
		"user_message":   catalogItemToPayload(*updatedUserMessage),
		"external_reply": externalReply,
	})
}

func (h *Handler) SyncCaseCommunication(c *echo.Context) error {
	if h.forumProxy == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "forum proxy service is not configured")
	}
	identity, _ := middleware.GetIdentity(c)
	tenantID, caseID, err := h.resolveCaseInTenant(c)
	if err != nil {
		return err
	}
	threadID, err := uuid.Parse(strings.TrimSpace(c.Param("threadID")))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid thread id")
	}
	thread, err := h.resolveCaseCommunicationThread(c.Request().Context(), tenantID, caseID, threadID)
	if err != nil {
		return err
	}

	var req syncCaseCommunicationRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	connectorID, err := resolveCommunicationConnectorID(req.ConnectorID, *thread)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "connector_id is required")
	}
	connector, err := h.resolveOutboundConnectorForTenant(c.Request().Context(), tenantID, connectorID)
	if err != nil {
		return err
	}
	if capabilityErr := validateCommunicationConnectorCapability(*connector, connectorCapabilitySyncMessages); capabilityErr != nil {
		return capabilityErr
	}
	communicationMode := connectorCommunicationModeFromItem(*connector)
	syncSubject := strings.TrimSpace(req.Subject)
	if syncSubject == "" {
		syncSubject = strings.TrimSpace(stringFromMap(thread.Data, "subject"))
	}
	if syncSubject == "" {
		syncSubject = strings.TrimSpace(stringFromMap(normalizeCommunicationMap(thread.Data["metadata"]), "subject", "email_subject", "topic"))
	}
	syncMetadata := map[string]any{}
	if syncSubject != "" {
		syncMetadata["subject"] = syncSubject
	}

	existingMessages, err := h.listCaseCommunicationMessageItems(c.Request().Context(), tenantID, threadID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to read communication history")
	}
	existingExternalIDs := make(map[string]struct{}, len(existingMessages))
	for _, item := range existingMessages {
		externalID := strings.TrimSpace(stringFromMap(item.Data, "external_id", "externalId"))
		if externalID != "" {
			existingExternalIDs[externalID] = struct{}{}
		}
	}

	syncResult, err := h.forumProxy.Sync(c.Request().Context(), forumproxy.SyncParams{
		TenantID:    tenantID,
		ThreadID:    threadID,
		ConnectorID: connectorID,
		Metadata:    syncMetadata,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, "failed to synchronize external messages")
	}

	created := make([]map[string]any, 0, len(syncResult.Messages))
	lastMessageContent := ""
	lastMessageTime := time.Time{}
	for _, message := range syncResult.Messages {
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		if communicationMode == "email" && syncSubject != "" && !messageMatchesSubject(syncSubject, message) {
			continue
		}
		externalID := strings.TrimSpace(message.ExternalID)
		if externalID != "" {
			if _, exists := existingExternalIDs[externalID]; exists {
				continue
			}
			existingExternalIDs[externalID] = struct{}{}
		}

		timestamp := parseCommunicationTimestamp(message.Timestamp, time.Now().UTC())
		messageMetadata := normalizeCommunicationMap(message.Metadata)
		messageSubject := strings.TrimSpace(stringFromMap(messageMetadata, "subject", "topic"))
		if messageSubject == "" && syncSubject != "" {
			messageSubject = syncSubject
			messageMetadata["subject"] = syncSubject
		}
		item, createErr := h.catalog.Create(c.Request().Context(), repository.CatalogCreateParams{
			TenantID:  &tenantID,
			Kind:      caseCommunicationMessageKind,
			OwnerID:   &identity.UserID,
			RefID:     &threadID,
			CreatedBy: &identity.UserID,
			Data: map[string]any{
				"thread_id":       threadID.String(),
				"case_id":         caseID.String(),
				"tenant_id":       tenantID.String(),
				"connector_id":    connectorID.String(),
				"direction":       "inbound",
				"author_id":       "external:" + connectorID.String(),
				"authorName":      strings.TrimSpace(message.Author),
				"content":         content,
				"subject":         messageSubject,
				"timestamp":       timestamp.Format(time.RFC3339),
				"external_id":     externalID,
				"delivery_status": "received",
				"metadata":        messageMetadata,
			},
		})
		if createErr != nil {
			continue
		}
		created = append(created, catalogItemToPayload(*item))
		lastMessageContent = content
		lastMessageTime = timestamp
	}

	syncedAt := time.Now().UTC()
	if h.forumBindings != nil {
		if binding, bindingErr := h.forumBindings.GetByThreadConnectorAndKey(c.Request().Context(), tenantID, threadID, connectorID, ""); bindingErr == nil && binding != nil && binding.LastSyncedAt != nil {
			syncedAt = binding.LastSyncedAt.UTC()
		}
	}
	if updateErr := h.updateCaseCommunicationThreadSyncState(c.Request().Context(), tenantID, threadID, syncedAt, len(created)); updateErr != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update communication thread")
	}
	if lastMessageContent != "" {
		if updateErr := h.updateCaseCommunicationThreadState(c.Request().Context(), tenantID, threadID, lastMessageTime, lastMessageContent); updateErr != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to update communication thread")
		}
	}

	return c.JSON(http.StatusOK, map[string]any{
		"thread_id":     threadID.String(),
		"connector_id":  connectorID.String(),
		"created_count": len(created),
		"created":       created,
		"synced_at":     syncedAt.Format(time.RFC3339),
	})
}

func (h *Handler) resolveCaseCommunicationThread(ctx context.Context, tenantID, caseID, threadID uuid.UUID) (*models.CatalogItem, error) {
	thread, err := h.catalog.GetByID(ctx, caseCommunicationThreadKind, threadID, &tenantID)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "communication thread not found")
	}
	if thread.RefID == nil || *thread.RefID != caseID {
		return nil, echo.NewHTTPError(http.StatusNotFound, "communication thread not found in case")
	}
	return thread, nil
}

func (h *Handler) resolveOutboundConnectorForTenant(ctx context.Context, tenantID, connectorID uuid.UUID) (*models.CatalogItem, error) {
	connector, err := h.catalog.GetByID(ctx, "outbound_connectors", connectorID, &tenantID)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "connector not found")
	}
	if connectorDirectionForItem(*connector) != "outbound" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "connector is not outbound")
	}
	if raw, exists := connector.Data["enabled"]; exists {
		if enabled, ok := raw.(bool); ok && !enabled {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "connector is disabled")
		}
	}

	// Resolve vault:// secrets in connector config so outbound drivers receive
	// real credentials instead of opaque references.
	if cfgRaw, exists := connector.Data["config"]; exists && cfgRaw != nil {
		resolved, resolveErr := h.resolveWorkflowVaultRefsValue(ctx, tenantID, cfgRaw)
		if resolveErr != nil {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "failed to resolve connector credentials")
		}
		if resolvedMap := normalizeMap(resolved); resolvedMap != nil {
			connector.Data["config"] = resolvedMap
		}
	}

	return connector, nil
}

func validateCommunicationConnectorCapability(connector models.CatalogItem, capability string) error {
	if connectorHasCapabilityItem(connector, capability) {
		return nil
	}
	return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("connector does not support %s", capability))
}

func communicationMetadataForConnector(connector models.CatalogItem, metadata map[string]any) map[string]any {
	out := normalizeCommunicationMap(metadata)
	if out == nil {
		out = map[string]any{}
	}
	if channel := connectorChannelFromItem(connector); channel != "" {
		out["channel"] = channel
	}
	if mode := connectorCommunicationModeFromItem(connector); mode != "" {
		out["communication_mode"] = mode
	}
	if capabilities := connectorCapabilitiesFromData(connector.Data); len(capabilities) > 0 {
		out["capabilities"] = capabilities
	}
	return out
}

func normalizeCommunicationParticipantForConnector(connector models.CatalogItem, participant map[string]any) map[string]any {
	out := normalizeCommunicationMap(participant)
	if out == nil {
		out = map[string]any{}
	}
	mode := connectorCommunicationModeFromItem(connector)
	channel := connectorChannelFromItem(connector)
	target := strings.TrimSpace(firstNonEmptyString(
		stringFromMap(out, "target", "recipient", "chat_id", "chatId", "channel_id", "channelId", "email", "external_user_id", "externalUserId", "username"),
	))
	switch mode {
	case "email":
		if target == "" {
			target = strings.TrimSpace(firstNonEmptyString(stringFromMap(out, "email")))
		}
		if target != "" {
			out["target"] = target
			if strings.TrimSpace(stringFromMap(out, "email")) == "" {
				out["email"] = target
			}
		}
	case "chat":
		switch channel {
		case "telegram":
			if target == "" {
				target = strings.TrimSpace(firstNonEmptyString(stringFromMap(out, "chat_id", "chatId", "username", "external_user_id", "externalUserId")))
			}
			if target != "" {
				out["target"] = target
				if strings.TrimSpace(stringFromMap(out, "chat_id", "chatId")) == "" {
					out["chat_id"] = target
				}
			}
		case "slack":
			if target == "" {
				target = strings.TrimSpace(firstNonEmptyString(stringFromMap(out, "channel_id", "channelId", "chat_id", "chatId")))
			}
			if target != "" {
				out["target"] = target
				if strings.TrimSpace(stringFromMap(out, "channel_id", "channelId")) == "" {
					out["channel_id"] = target
				}
			}
		}
	}
	return out
}

func (h *Handler) updateCaseCommunicationThreadMetadata(ctx context.Context, tenantID, threadID uuid.UUID, currentData map[string]any, metadata map[string]any) error {
	mergedMetadata := mergeCommunicationMaps(normalizeCommunicationMap(currentData["metadata"]), metadata)
	updateData := map[string]any{
		"metadata":   mergedMetadata,
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	}
	subject := strings.TrimSpace(firstNonEmptyString(
		stringFromMap(mergedMetadata, "subject", "email_subject", "topic"),
		stringFromMap(currentData, "subject"),
	))
	if subject != "" {
		updateData["subject"] = subject
	}
	_, err := h.catalog.Update(ctx, caseCommunicationThreadKind, threadID, &tenantID, repository.CatalogUpdateParams{Data: updateData})
	return err
}

func (h *Handler) updateCaseCommunicationThreadSyncState(ctx context.Context, tenantID, threadID uuid.UUID, syncedAt time.Time, createdCount int) error {
	_, err := h.catalog.Update(ctx, caseCommunicationThreadKind, threadID, &tenantID, repository.CatalogUpdateParams{
		Data: map[string]any{
			"last_synced_at":          syncedAt.UTC().Format(time.RFC3339),
			"last_sync_created_count": createdCount,
			"updated_at":              time.Now().UTC().Format(time.RFC3339),
		},
	})
	return err
}

func (h *Handler) updateCaseCommunicationThreadState(ctx context.Context, tenantID, threadID uuid.UUID, at time.Time, preview string) error {
	_, err := h.catalog.Update(ctx, caseCommunicationThreadKind, threadID, &tenantID, repository.CatalogUpdateParams{
		Data: map[string]any{
			"last_message_at":      at.UTC().Format(time.RFC3339),
			"last_message_preview": communicationPreview(preview),
			"updated_at":           time.Now().UTC().Format(time.RFC3339),
		},
	})
	return err
}

func (h *Handler) listCaseCommunicationMessages(ctx context.Context, tenantID, threadID uuid.UUID) ([]map[string]any, error) {
	items, err := h.listCaseCommunicationMessageItems(ctx, tenantID, threadID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, catalogItemToPayload(item))
	}
	return out, nil
}

func (h *Handler) listCaseCommunicationMessageItems(ctx context.Context, tenantID, threadID uuid.UUID) ([]models.CatalogItem, error) {
	items, err := h.catalog.List(ctx, repository.CatalogListParams{
		Kind:     caseCommunicationMessageKind,
		TenantID: &tenantID,
		RefID:    &threadID,
		Limit:    1000,
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool {
		left := parseCommunicationTimestamp(stringFromMap(items[i].Data, "timestamp", "created_at"), items[i].CreatedAt)
		right := parseCommunicationTimestamp(stringFromMap(items[j].Data, "timestamp", "created_at"), items[j].CreatedAt)
		return left.Before(right)
	})
	return items, nil
}

func resolveCommunicationConnectorID(rawConnectorID string, thread models.CatalogItem) (uuid.UUID, error) {
	connectorIDRaw := strings.TrimSpace(rawConnectorID)
	if connectorIDRaw == "" {
		connectorIDRaw = strings.TrimSpace(stringFromMap(thread.Data, "connector_id", "connectorId"))
	}
	if connectorIDRaw == "" {
		return uuid.Nil, fmt.Errorf("connector_id is required")
	}
	connectorID, err := uuid.Parse(connectorIDRaw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("connector_id is required")
	}
	return connectorID, nil
}

func mergeCommunicationMaps(primary map[string]any, extra map[string]any) map[string]any {
	out := normalizeCommunicationMap(primary)
	for key, value := range normalizeCommunicationMap(extra) {
		out[key] = value
	}
	return out
}

func normalizeCommunicationMap(input any) map[string]any {
	if input == nil {
		return map[string]any{}
	}
	if typed, ok := input.(map[string]any); ok {
		out := make(map[string]any, len(typed))
		for key, value := range typed {
			out[key] = value
		}
		return out
	}

	raw, err := json.Marshal(input)
	if err != nil || len(raw) == 0 {
		return map[string]any{}
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func (h *Handler) resolveCommunicationTemplate(ctx context.Context, tenantID uuid.UUID, templateID uuid.UUID) (*models.CatalogItem, error) {
	template, err := h.catalog.GetByID(ctx, caseCommunicationTemplateKind, templateID, &tenantID)
	if err == nil {
		return template, nil
	}
	legacyTemplate, legacyErr := h.catalog.GetByID(ctx, "email_templates", templateID, &tenantID)
	if legacyErr == nil {
		return legacyTemplate, nil
	}
	return nil, echo.NewHTTPError(http.StatusNotFound, "communication template not found")
}

func renderCommunicationTemplate(template models.CatalogItem, scope map[string]any) (content string, subject string) {
	contentTemplate := strings.TrimSpace(stringFromMap(
		template.Data,
		"body_template",
		"message_template",
		"content_template",
		"body",
		"content",
		"text",
	))
	subjectTemplate := strings.TrimSpace(stringFromMap(template.Data, "subject_template", "subject"))
	return strings.TrimSpace(workflow.RenderTemplate(contentTemplate, scope)), strings.TrimSpace(workflow.RenderTemplate(subjectTemplate, scope))
}

func buildCommunicationTemplateScope(caseID, threadID uuid.UUID, authorName string, threadData map[string]any, vars map[string]any) map[string]any {
	scope := map[string]any{
		"case": map[string]any{
			"id": caseID.String(),
		},
		"thread": map[string]any{
			"id":      threadID.String(),
			"title":   stringFromMap(threadData, "title"),
			"channel": strings.ToLower(strings.TrimSpace(stringFromMap(threadData, "channel"))),
			"subject": stringFromMap(threadData, "subject"),
		},
		"author": authorName,
		"vars":   normalizeCommunicationMap(vars),
	}
	participant := normalizeCommunicationMap(threadData["participant"])
	if len(participant) > 0 {
		scope["participant"] = participant
	}
	for key, value := range normalizeCommunicationMap(vars) {
		scope[key] = value
	}
	return scope
}

func ensureCommunicationEmailRecipient(metadata map[string]any, threadData map[string]any) map[string]any {
	out := mergeCommunicationMaps(metadata, nil)
	mode := normalizeConnectorCommunicationMode(firstNonEmptyString(
		stringFromMap(out, "communication_mode", "communicationMode"),
		stringFromMap(threadData, "communication_mode", "communicationMode"),
		defaultConnectorCommunicationMode(strings.ToLower(strings.TrimSpace(stringFromMap(out, "channel")))),
		defaultConnectorCommunicationMode(strings.ToLower(strings.TrimSpace(stringFromMap(threadData, "channel")))),
	))
	if mode != "email" {
		return out
	}
	recipients := collectEmailTargets(out["to"])
	if len(recipients) == 0 {
		recipients = append(recipients, collectEmailTargets(out["recipient"])...)
	}
	if len(recipients) == 0 {
		recipients = append(recipients, collectEmailTargets(out["target"])...)
	}
	if len(recipients) == 0 {
		participant := normalizeCommunicationMap(threadData["participant"])
		recipients = append(recipients, collectEmailTargets(participant["email"])...)
		recipients = append(recipients, collectEmailTargets(participant["target"])...)
	}
	if len(recipients) > 0 {
		out["to"] = uniqueTargets(recipients)
	}
	return out
}

func shouldNotifyManagers(flag *bool, metadata map[string]any) bool {
	if flag != nil {
		return *flag
	}
	if enabled, ok := boolFromMap(metadata, "notify_managers", "notifyManagers", "notify_management", "notifyManagement"); ok {
		return enabled
	}
	action := strings.ToLower(strings.TrimSpace(stringFromMap(metadata, "action", "action_type", "event_type", "event", "operation")))
	if strings.Contains(action, "unblock") || strings.Contains(action, "unlock") {
		return false
	}
	return strings.Contains(action, "block")
}

func communicationManagerRecipients(threadData map[string]any, metadata map[string]any) []string {
	participant := normalizeCommunicationMap(threadData["participant"])
	collected := make([]string, 0, 8)
	collected = append(collected, collectEmailTargets(metadata["manager_recipients"])...)
	collected = append(collected, collectEmailTargets(metadata["manager_emails"])...)
	collected = append(collected, collectEmailTargets(metadata["manager_email"])...)
	collected = append(collected, collectEmailTargets(participant["manager_emails"])...)
	collected = append(collected, collectEmailTargets(participant["manager_email"])...)
	collected = append(collected, collectEmailTargets(participant["supervisor_email"])...)
	return uniqueTargets(collected)
}

func mergeRecipientTargets(current any, extra []string) []string {
	merged := append(collectEmailTargets(current), extra...)
	return uniqueTargets(merged)
}

func collectEmailTargets(input any) []string {
	out := make([]string, 0)
	appendToken := func(raw string) {
		token := strings.TrimSpace(raw)
		if token == "" {
			return
		}
		for _, part := range strings.FieldsFunc(token, func(r rune) bool {
			return r == ',' || r == ';'
		}) {
			trimmed := strings.TrimSpace(part)
			if trimmed == "" {
				continue
			}
			out = append(out, trimmed)
		}
	}
	switch typed := input.(type) {
	case nil:
		return out
	case string:
		appendToken(typed)
	case []string:
		for _, value := range typed {
			appendToken(value)
		}
	case []any:
		for _, value := range typed {
			appendToken(fmt.Sprint(value))
		}
	default:
		appendToken(fmt.Sprint(typed))
	}
	return uniqueTargets(out)
}

func uniqueTargets(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func messageMatchesSubject(subject string, message outbound.PollMessage) bool {
	needle := strings.ToLower(strings.TrimSpace(subject))
	if needle == "" {
		return true
	}
	haystack := strings.ToLower(strings.TrimSpace(stringFromMap(normalizeCommunicationMap(message.Metadata), "subject", "topic")))
	if haystack == "" {
		return false
	}
	return haystack == needle || strings.Contains(haystack, needle)
}

func communicationPreview(content string) string {
	normalized := strings.Join(strings.Fields(strings.TrimSpace(content)), " ")
	if normalized == "" {
		return ""
	}
	runes := []rune(normalized)
	if len(runes) <= 180 {
		return normalized
	}
	return strings.TrimSpace(string(runes[:180])) + "..."
}

func parseCommunicationTimestamp(raw string, fallback time.Time) time.Time {
	trimmed := strings.TrimSpace(raw)
	if trimmed != "" {
		if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
			return parsed.UTC()
		}
	}
	if fallback.IsZero() {
		return time.Now().UTC()
	}
	return fallback.UTC()
}
