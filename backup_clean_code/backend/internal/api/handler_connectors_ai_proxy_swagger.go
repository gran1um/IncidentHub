package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"incidenthub/backend/internal/forumproxy"
	"incidenthub/backend/internal/middleware"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

type forumProxySendRequest struct {
	ConnectorID string         `json:"connector_id"`
	ProfileID   string         `json:"profile_id"`
	BindingKey  string         `json:"binding_key"`
	Content     string         `json:"content"`
	Author      string         `json:"author"`
	Metadata    map[string]any `json:"metadata"`
}

type forumProxySyncRequest struct {
	ConnectorID string         `json:"connector_id"`
	ProfileID   string         `json:"profile_id"`
	BindingKey  string         `json:"binding_key"`
	Metadata    map[string]any `json:"metadata"`
}

type forumProxyProfileUpsertRequest struct {
	ID          string         `json:"id"`
	ConnectorID string         `json:"connector_id"`
	Name        string         `json:"name"`
	BindingKey  string         `json:"binding_key"`
	Metadata    map[string]any `json:"metadata"`
}

type forumProxyProfile struct {
	ID             string         `json:"id"`
	ConnectorID    string         `json:"connector_id"`
	Name           string         `json:"name"`
	BindingKey     string         `json:"binding_key"`
	Metadata       map[string]any `json:"metadata"`
	CreatedAt      string         `json:"created_at,omitempty"`
	UpdatedAt      string         `json:"updated_at,omitempty"`
	LastSyncedAt   string         `json:"last_synced_at,omitempty"`
	HasBinding     bool           `json:"has_binding,omitempty"`
	ConversationID string         `json:"conversation_id,omitempty"`
	CursorPresent  bool           `json:"cursor_present,omitempty"`
}

func (h *Handler) ListInboundConnectors(c *echo.Context) error {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	items, err := h.listDirectionalConnectors(c, tenantID, "inbound")
	if err != nil {
		return err
	}

	latestRunsByConnector := map[uuid.UUID]models.InboundConnectorRun{}
	if h.inboundRuns != nil {
		runs, runErr := h.inboundRuns.ListLatestByTenant(c.Request().Context(), tenantID)
		if runErr == nil {
			for _, run := range runs {
				latestRunsByConnector[run.ConnectorID] = run
			}
		}
	}

	payload := make([]map[string]any, 0, len(items))
	for _, item := range items {
		row := catalogItemToPayload(item)
		if run, exists := latestRunsByConnector[item.ID]; exists {
			row["last_run"] = inboundConnectorRunToPayload(run)
		}
		payload = append(payload, row)
	}
	return c.JSON(http.StatusOK, payload)
}

func (h *Handler) ListOutboundConnectors(c *echo.Context) error {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	items, err := h.listDirectionalConnectors(c, tenantID, "outbound")
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, mapCatalogItems(items))
}

func (h *Handler) ListInboundConnectorRuns(c *echo.Context) error {
	if h.inboundRuns == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "inbound connector runs repository is not configured")
	}
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}

	limit := 100
	if rawLimit := strings.TrimSpace(c.QueryParam("limit")); rawLimit != "" {
		if parsed, err := strconv.Atoi(rawLimit); err == nil {
			limit = parsed
		}
	}
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	var runs []models.InboundConnectorRun
	connectorRaw := strings.TrimSpace(c.QueryParam("connector_id"))
	if connectorRaw != "" {
		connectorID, err := uuid.Parse(connectorRaw)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid connector_id")
		}
		list, listErr := h.inboundRuns.ListByConnector(c.Request().Context(), tenantID, connectorID, limit)
		if listErr != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to list inbound connector runs")
		}
		runs = list
	} else {
		list, listErr := h.inboundRuns.ListByTenant(c.Request().Context(), tenantID, limit)
		if listErr != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to list inbound connector runs")
		}
		runs = list
	}

	payload := make([]map[string]any, 0, len(runs))
	for _, run := range runs {
		payload = append(payload, inboundConnectorRunToPayload(run))
	}
	return c.JSON(http.StatusOK, payload)
}

func (h *Handler) RunInboundConnectors(c *echo.Context) error {
	if h.inboundWorker == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "inbound worker is not configured")
	}
	identity, _ := middleware.GetIdentity(c)
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	stats, err := h.inboundWorker.RunNow(c.Request().Context(), &tenantID, nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to run inbound connectors")
	}
	h.rewardConnectorInvocation(c.Request().Context(), tenantID, identity.UserID, "inbound_connectors_run:"+uuid.NewString(), map[string]any{
		"tenant_id": tenantID.String(),
		"triggered": xpEventTypeConnectorInvoked,
		"scope":     "all",
	})
	return c.JSON(http.StatusOK, stats)
}

func (h *Handler) RunInboundConnector(c *echo.Context) error {
	if h.inboundWorker == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "inbound worker is not configured")
	}
	identity, _ := middleware.GetIdentity(c)
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	connectorID, err := uuid.Parse(strings.TrimSpace(c.Param("connectorID")))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid connectorID")
	}
	stats, err := h.inboundWorker.RunNow(c.Request().Context(), &tenantID, &connectorID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to run inbound connector")
	}
	h.rewardConnectorInvocation(c.Request().Context(), tenantID, identity.UserID, "inbound_connector_run:"+connectorID.String()+":"+uuid.NewString(), map[string]any{
		"tenant_id":    tenantID.String(),
		"connector_id": connectorID.String(),
		"triggered":    xpEventTypeConnectorInvoked,
		"scope":        "single",
	})
	return c.JSON(http.StatusOK, stats)
}

func (h *Handler) ListForumProxyProfiles(c *echo.Context) error {
	tenantID, thread, err := h.resolveForumThreadForProxy(c)
	if err != nil {
		return err
	}
	profiles := h.enrichForumProxyProfilesWithBindingState(c.Request().Context(), tenantID, thread.ID, forumProxyProfilesFromThread(thread))
	return c.JSON(http.StatusOK, map[string]any{
		"thread_id": thread.ID.String(),
		"tenant_id": tenantID.String(),
		"profiles":  forumProxyProfilesToPayload(profiles),
	})
}

func (h *Handler) UpsertForumProxyProfile(c *echo.Context) error {
	tenantID, thread, err := h.resolveForumThreadForProxy(c)
	if err != nil {
		return err
	}

	var req forumProxyProfileUpsertRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	profiles := forumProxyProfilesFromThread(thread)
	now := time.Now().UTC().Format(time.RFC3339)
	requestedID := normalizeForumProxyProfileID(req.ID)
	requestedBindingKey := normalizeForumProxyBindingKey(req.BindingKey)
	requestedName := strings.TrimSpace(req.Name)
	requestedMetadata := normalizeCommunicationMap(req.Metadata)
	requestedConnectorIDRaw := strings.TrimSpace(req.ConnectorID)

	updateIndex := -1
	for idx, profile := range profiles {
		if profile.ID == requestedID && requestedID != "" {
			updateIndex = idx
			break
		}
	}

	var connector *models.CatalogItem
	resolveConnector := func(connectorIDRaw string) (*models.CatalogItem, error) {
		connectorID, parseErr := uuid.Parse(strings.TrimSpace(connectorIDRaw))
		if parseErr != nil {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "connector_id is required")
		}
		return h.resolveOutboundConnectorForTenant(c.Request().Context(), tenantID, connectorID)
	}

	var upserted forumProxyProfile
	if updateIndex >= 0 {
		current := profiles[updateIndex]
		nextConnectorID := current.ConnectorID
		if requestedConnectorIDRaw != "" && requestedConnectorIDRaw != current.ConnectorID {
			connector, err = resolveConnector(requestedConnectorIDRaw)
			if err != nil {
				return err
			}
			nextConnectorID = connector.ID.String()
		}

		upserted = current
		upserted.ConnectorID = nextConnectorID
		if requestedName != "" {
			upserted.Name = requestedName
		}
		if len(requestedMetadata) > 0 {
			upserted.Metadata = mergeCommunicationMaps(upserted.Metadata, requestedMetadata)
		}
		if requestedBindingKey != "" {
			upserted.BindingKey = requestedBindingKey
		}
		if strings.TrimSpace(upserted.BindingKey) == "" {
			upserted.BindingKey = upserted.ID
		}
		upserted.UpdatedAt = now
		profiles[updateIndex] = upserted
	} else {
		if requestedConnectorIDRaw == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "connector_id is required")
		}
		connector, err = resolveConnector(requestedConnectorIDRaw)
		if err != nil {
			return err
		}
		profileID := requestedID
		if profileID == "" {
			profileID = uuid.NewString()
		}
		bindingKey := requestedBindingKey
		if bindingKey == "" {
			bindingKey = profileID
		}
		derivedName := requestedName
		if derivedName == "" {
			derivedName = strings.TrimSpace(stringFromMap(connector.Data, "name", "title"))
		}
		if derivedName == "" {
			derivedName = "Connector profile"
		}
		upserted = forumProxyProfile{
			ID:          profileID,
			ConnectorID: connector.ID.String(),
			Name:        derivedName,
			BindingKey:  bindingKey,
			Metadata:    requestedMetadata,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		profiles = append(profiles, upserted)
	}

	updatedThread, err := h.catalog.Update(c.Request().Context(), "forum_thread", thread.ID, &tenantID, repository.CatalogUpdateParams{
		Data: map[string]any{
			"proxy_profiles": forumProxyProfilesToPayload(profiles),
		},
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update forum proxy profiles")
	}
	h.indexForumThreadDocument(c.Request().Context(), *updatedThread, tenantID)

	enrichedProfiles := h.enrichForumProxyProfilesWithBindingState(c.Request().Context(), tenantID, thread.ID, profiles)
	var enrichedUpserted map[string]any
	for _, profile := range enrichedProfiles {
		if profile.ID == upserted.ID {
			enrichedUpserted = profile.toPayload()
			break
		}
	}
	if enrichedUpserted == nil {
		enrichedUpserted = upserted.toPayload()
	}
	return c.JSON(http.StatusOK, map[string]any{
		"thread_id": thread.ID.String(),
		"profile":   enrichedUpserted,
		"profiles":  forumProxyProfilesToPayload(enrichedProfiles),
	})
}

func (h *Handler) DeleteForumProxyProfile(c *echo.Context) error {
	tenantID, thread, err := h.resolveForumThreadForProxy(c)
	if err != nil {
		return err
	}
	profileID := normalizeForumProxyProfileID(c.Param("profileID"))
	if profileID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "profile_id is required")
	}

	profiles := forumProxyProfilesFromThread(thread)
	nextProfiles := make([]forumProxyProfile, 0, len(profiles))
	removed := false
	for _, profile := range profiles {
		if profile.ID == profileID {
			removed = true
			continue
		}
		nextProfiles = append(nextProfiles, profile)
	}
	if !removed {
		return echo.NewHTTPError(http.StatusNotFound, "profile not found")
	}

	updatedThread, err := h.catalog.Update(c.Request().Context(), "forum_thread", thread.ID, &tenantID, repository.CatalogUpdateParams{
		Data: map[string]any{
			"proxy_profiles": forumProxyProfilesToPayload(nextProfiles),
		},
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete forum proxy profile")
	}
	h.indexForumThreadDocument(c.Request().Context(), *updatedThread, tenantID)

	enrichedProfiles := h.enrichForumProxyProfilesWithBindingState(c.Request().Context(), tenantID, thread.ID, nextProfiles)
	return c.JSON(http.StatusOK, map[string]any{
		"thread_id":  thread.ID.String(),
		"profile_id": profileID,
		"profiles":   forumProxyProfilesToPayload(enrichedProfiles),
	})
}

func (h *Handler) ProxyForumSend(c *echo.Context) error {
	if h.forumProxy == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "forum proxy service is not configured")
	}
	identity, _ := middleware.GetIdentity(c)
	tenantID, thread, err := h.resolveForumThreadForProxy(c)
	if err != nil {
		return err
	}
	threadID := thread.ID

	var req forumProxySendRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	req.Content = strings.TrimSpace(req.Content)
	if req.Content == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "content is required")
	}

	connectorID, profile, sendMetadata, bindingKey, err := h.resolveForumProxyExecution(c.Request().Context(), tenantID, *thread, forumProxyExecutionParams{
		ConnectorID:        req.ConnectorID,
		ProfileID:          req.ProfileID,
		BindingKey:         req.BindingKey,
		Metadata:           req.Metadata,
		RequiredCapability: connectorCapabilityForumThreads,
	})
	if err != nil {
		return err
	}

	authorName := strings.TrimSpace(req.Author)
	if authorName == "" {
		authorName = identity.Username
	}
	postMetadata := mergeCommunicationMaps(sendMetadata, map[string]any{
		"connector_id": connectorID.String(),
	})
	if profile != nil {
		postMetadata["profile_id"] = profile.ID
	}
	if bindingKey != "" {
		postMetadata["binding_key"] = bindingKey
	}
	userPost, err := h.catalog.Create(c.Request().Context(), repository.CatalogCreateParams{
		TenantID: &tenantID,
		Kind:     "forum_post",
		OwnerID:  &identity.UserID,
		RefID:    &threadID,
		Data: map[string]any{
			"thread_id":  threadID.String(),
			"author_id":  identity.UserID.String(),
			"authorName": authorName,
			"content":    req.Content,
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
			"tenant_id":  tenantID.String(),
			"metadata":   postMetadata,
		},
		CreatedBy: &identity.UserID,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to create forum post")
	}

	sendResult, err := h.forumProxy.Send(c.Request().Context(), forumproxy.SendParams{
		TenantID:    tenantID,
		ThreadID:    threadID,
		ConnectorID: connectorID,
		BindingKey:  bindingKey,
		Author:      authorName,
		Message:     req.Content,
		Metadata:    sendMetadata,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, "failed to send message to external service")
	}

	resolvedProfile, persistErr := h.persistForumProxyProfileForExecution(
		c.Request().Context(),
		tenantID,
		*thread,
		connectorID,
		profile,
		bindingKey,
		mergeCommunicationMaps(sendMetadata, sendResult.Metadata),
	)
	if persistErr != nil {
		return persistErr
	}
	if resolvedProfile != nil {
		postMetadata["profile_id"] = resolvedProfile.ID
		bindingKey = firstNonEmptyString(bindingKey, resolvedProfile.BindingKey)
	}
	postMetadata = mergeCommunicationMaps(postMetadata, sendResult.Metadata)
	if updatedUserPost, updateErr := h.catalog.Update(c.Request().Context(), "forum_post", userPost.ID, &tenantID, repository.CatalogUpdateParams{
		Data: map[string]any{
			"metadata": postMetadata,
		},
	}); updateErr == nil {
		userPost = updatedUserPost
	}

	var replyPayload map[string]any
	if strings.TrimSpace(sendResult.Reply) != "" {
		replyItem, createErr := h.catalog.Create(c.Request().Context(), repository.CatalogCreateParams{
			TenantID: &tenantID,
			Kind:     "forum_post",
			OwnerID:  &identity.UserID,
			RefID:    &threadID,
			Data: map[string]any{
				"thread_id":  threadID.String(),
				"author_id":  "external:" + connectorID.String(),
				"authorName": "External Service",
				"content":    strings.TrimSpace(sendResult.Reply),
				"timestamp":  time.Now().UTC().Format(time.RFC3339),
				"tenant_id":  tenantID.String(),
				"metadata":   postMetadata,
			},
			CreatedBy: &identity.UserID,
		})
		if createErr == nil {
			replyPayload = catalogItemToPayload(*replyItem)
		}
	}

	return c.JSON(http.StatusOK, map[string]any{
		"user_post":      catalogItemToPayload(*userPost),
		"external_reply": replyPayload,
		"connector_id":   sendResult.ConnectorID.String(),
		"profile_id":     firstNonEmptyString(profileIDOrEmpty(resolvedProfile), req.ProfileID, profileIDOrEmpty(profile)),
		"binding_key":    bindingKey,
		"profile": func() map[string]any {
			if resolvedProfile == nil {
				return nil
			}
			enriched := h.enrichForumProxyProfilesWithBindingState(c.Request().Context(), tenantID, threadID, []forumProxyProfile{*resolvedProfile})
			if len(enriched) == 0 {
				return resolvedProfile.toPayload()
			}
			return enriched[0].toPayload()
		}(),
	})
}

func (h *Handler) ProxyForumSync(c *echo.Context) error {
	if h.forumProxy == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "forum proxy service is not configured")
	}
	identity, _ := middleware.GetIdentity(c)
	tenantID, thread, err := h.resolveForumThreadForProxy(c)
	if err != nil {
		return err
	}
	threadID := thread.ID

	var req forumProxySyncRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	connectorID, profile, syncMetadata, bindingKey, err := h.resolveForumProxyExecution(c.Request().Context(), tenantID, *thread, forumProxyExecutionParams{
		ConnectorID:        req.ConnectorID,
		ProfileID:          req.ProfileID,
		BindingKey:         req.BindingKey,
		Metadata:           req.Metadata,
		RequiredCapability: connectorCapabilitySyncMessages,
	})
	if err != nil {
		return err
	}

	syncResult, err := h.forumProxy.Sync(c.Request().Context(), forumproxy.SyncParams{
		TenantID:    tenantID,
		ThreadID:    threadID,
		ConnectorID: connectorID,
		BindingKey:  bindingKey,
		Metadata:    syncMetadata,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, "failed to synchronize external messages")
	}

	created := make([]map[string]any, 0, len(syncResult.Messages))
	for _, message := range syncResult.Messages {
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		externalID := strings.TrimSpace(message.ExternalID)
		if externalID != "" {
			exists, existsErr := h.catalog.ForumPostExistsByExternalID(c.Request().Context(), tenantID, threadID, externalID)
			if existsErr != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to check synchronized message duplicates")
			}
			if exists {
				continue
			}
		}
		timestamp := strings.TrimSpace(message.Timestamp)
		if timestamp == "" {
			timestamp = time.Now().UTC().Format(time.RFC3339)
		}
		authorName := strings.TrimSpace(message.Author)
		if authorName == "" {
			authorName = "External Service"
		}
		item, createErr := h.catalog.Create(c.Request().Context(), repository.CatalogCreateParams{
			TenantID: &tenantID,
			Kind:     "forum_post",
			OwnerID:  &identity.UserID,
			RefID:    &threadID,
			Data: map[string]any{
				"thread_id":   threadID.String(),
				"author_id":   "external:" + connectorID.String(),
				"authorName":  authorName,
				"content":     content,
				"timestamp":   timestamp,
				"tenant_id":   tenantID.String(),
				"external_id": externalID,
				"metadata": mergeCommunicationMaps(message.Metadata, map[string]any{
					"connector_id": connectorID.String(),
					"profile_id":   profileIDOrEmpty(profile),
					"binding_key":  bindingKey,
				}),
			},
			CreatedBy: &identity.UserID,
		})
		if createErr != nil {
			continue
		}
		created = append(created, catalogItemToPayload(*item))
	}

	syncedAt := time.Now().UTC().Format(time.RFC3339)
	if h.forumBindings != nil {
		binding, bindingErr := h.forumBindings.GetByThreadConnectorAndKey(
			c.Request().Context(),
			tenantID,
			threadID,
			connectorID,
			bindingKey,
		)
		if bindingErr == nil && binding != nil && binding.LastSyncedAt != nil {
			syncedAt = binding.LastSyncedAt.UTC().Format(time.RFC3339)
		}
	}

	return c.JSON(http.StatusOK, map[string]any{
		"connector_id":  connectorID.String(),
		"profile_id":    firstNonEmptyString(req.ProfileID, profileIDOrEmpty(profile)),
		"binding_key":   bindingKey,
		"created":       created,
		"created_count": len(created),
		"synced_at":     syncedAt,
	})
}

type forumProxyExecutionParams struct {
	ConnectorID        string
	ProfileID          string
	BindingKey         string
	Metadata           map[string]any
	RequiredCapability string
}

func (h *Handler) resolveForumProxyExecution(
	ctx context.Context,
	tenantID uuid.UUID,
	thread models.CatalogItem,
	params forumProxyExecutionParams,
) (connectorUUID uuid.UUID, profile *forumProxyProfile, metadataPayload map[string]any, bindingKeyPayload string, err error) {
	requestedProfileID := normalizeForumProxyProfileID(params.ProfileID)
	requestMetadata := normalizeCommunicationMap(params.Metadata)
	profiles := forumProxyProfilesFromThread(&thread)

	var selectedProfile *forumProxyProfile
	if requestedProfileID != "" {
		for _, profile := range profiles {
			if profile.ID != requestedProfileID {
				continue
			}
			profileCopy := profile
			selectedProfile = &profileCopy
			break
		}
		if selectedProfile == nil {
			return uuid.Nil, nil, nil, "", echo.NewHTTPError(http.StatusNotFound, "profile not found")
		}
	}

	connectorIDRaw := strings.TrimSpace(params.ConnectorID)
	if connectorIDRaw == "" && selectedProfile != nil {
		connectorIDRaw = selectedProfile.ConnectorID
	}
	connectorID, err := uuid.Parse(connectorIDRaw)
	if err != nil {
		return uuid.Nil, nil, nil, "", echo.NewHTTPError(http.StatusBadRequest, "connector_id is required")
	}
	connector, err := h.resolveOutboundConnectorForTenant(ctx, tenantID, connectorID)
	if err != nil {
		return uuid.Nil, nil, nil, "", err
	}
	if strings.TrimSpace(params.RequiredCapability) != "" {
		if err := validateCommunicationConnectorCapability(*connector, params.RequiredCapability); err != nil {
			return uuid.Nil, nil, nil, "", err
		}
	}

	metadata := communicationMetadataForConnector(*connector, map[string]any{})
	if selectedProfile != nil {
		metadata = mergeCommunicationMaps(metadata, selectedProfile.Metadata)
	}
	metadata = mergeCommunicationMaps(metadata, requestMetadata)
	metadata["thread_id"] = thread.ID.String()
	metadata["connector_id"] = connectorID.String()
	if selectedProfile != nil {
		metadata["profile_id"] = selectedProfile.ID
	}

	bindingKey := normalizeForumProxyBindingKey(params.BindingKey)
	if bindingKey == "" && selectedProfile != nil {
		bindingKey = normalizeForumProxyBindingKey(selectedProfile.BindingKey)
	}
	if bindingKey == "" && selectedProfile != nil {
		bindingKey = selectedProfile.ID
	}
	if bindingKey == "" {
		bindingKey = deriveForumProxyBindingKey(metadata)
	}
	if bindingKey != "" {
		metadata["binding_key"] = bindingKey
	}

	return connectorID, selectedProfile, metadata, bindingKey, nil
}

func (h *Handler) resolveForumThreadForProxy(c *echo.Context) (uuid.UUID, *models.CatalogItem, error) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return uuid.Nil, nil, echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	threadID, err := uuid.Parse(strings.TrimSpace(c.Param("threadID")))
	if err != nil {
		return uuid.Nil, nil, echo.NewHTTPError(http.StatusBadRequest, "invalid thread id")
	}
	thread, err := h.catalog.GetByID(c.Request().Context(), "forum_thread", threadID, &tenantID)
	if err != nil {
		return uuid.Nil, nil, echo.NewHTTPError(http.StatusNotFound, "thread not found")
	}
	return tenantID, thread, nil
}

func forumProxyProfilesFromThread(thread *models.CatalogItem) []forumProxyProfile {
	if thread == nil {
		return []forumProxyProfile{}
	}
	return forumProxyProfilesFromData(thread.Data["proxy_profiles"])
}

func forumProxyProfilesFromData(raw any) []forumProxyProfile {
	encoded, err := json.Marshal(raw)
	if err != nil || len(encoded) == 0 {
		return []forumProxyProfile{}
	}
	parsed := make([]forumProxyProfile, 0)
	if err := json.Unmarshal(encoded, &parsed); err != nil {
		return []forumProxyProfile{}
	}
	out := make([]forumProxyProfile, 0, len(parsed))
	for _, profile := range parsed {
		profile.ID = normalizeForumProxyProfileID(profile.ID)
		if profile.ID == "" {
			continue
		}
		profile.ConnectorID = strings.TrimSpace(profile.ConnectorID)
		if profile.ConnectorID == "" {
			continue
		}
		profile.Name = strings.TrimSpace(profile.Name)
		if profile.Name == "" {
			profile.Name = "Connector profile"
		}
		profile.BindingKey = normalizeForumProxyBindingKey(profile.BindingKey)
		if profile.BindingKey == "" {
			profile.BindingKey = profile.ID
		}
		profile.Metadata = normalizeCommunicationMap(profile.Metadata)
		if profile.Metadata == nil {
			profile.Metadata = map[string]any{}
		}
		out = append(out, profile)
	}
	return out
}

func forumProxyProfilesToPayload(profiles []forumProxyProfile) []map[string]any {
	out := make([]map[string]any, 0, len(profiles))
	for _, profile := range profiles {
		out = append(out, profile.toPayload())
	}
	return out
}

func forumProxyProfileBindingMapKey(connectorID, bindingKey string) string {
	return strings.TrimSpace(connectorID) + "::" + normalizeForumProxyBindingKey(bindingKey)
}

func findForumProxyProfileByConnectorAndBindingKey(profiles []forumProxyProfile, connectorID, bindingKey string) (int, *forumProxyProfile) {
	normalizedConnectorID := strings.TrimSpace(connectorID)
	normalizedBindingKey := normalizeForumProxyBindingKey(bindingKey)
	for idx := range profiles {
		if strings.TrimSpace(profiles[idx].ConnectorID) != normalizedConnectorID {
			continue
		}
		if normalizeForumProxyBindingKey(profiles[idx].BindingKey) != normalizedBindingKey {
			continue
		}
		profile := profiles[idx]
		return idx, &profile
	}
	return -1, nil
}

func forumProxyProfileDisplayName(existing *forumProxyProfile, metadata map[string]any) string {
	if existing != nil {
		if current := strings.TrimSpace(existing.Name); current != "" {
			return current
		}
	}
	participant := normalizeCommunicationMap(metadata["participant"])
	candidates := []string{
		stringFromMap(metadata, "target_name", "label", "name", "subject", "target", "recipient", "to", "chat_id", "chatId", "channel_id", "channelId", "username"),
		stringFromMap(participant, "name", "target", "recipient", "email", "chat_id", "chatId", "channel_id", "channelId", "username"),
	}
	for _, candidate := range candidates {
		normalized := strings.TrimSpace(candidate)
		if normalized != "" {
			return normalized
		}
	}
	return "Linked route"
}

func (h *Handler) enrichForumProxyProfilesWithBindingState(ctx context.Context, tenantID, threadID uuid.UUID, profiles []forumProxyProfile) []forumProxyProfile {
	if len(profiles) == 0 || h.forumBindings == nil {
		return profiles
	}
	bindings, err := h.forumBindings.ListByThread(ctx, tenantID, threadID)
	if err != nil {
		return profiles
	}
	bindingsByKey := make(map[string]models.ForumExternalBinding, len(bindings))
	for _, binding := range bindings {
		bindingsByKey[forumProxyProfileBindingMapKey(binding.ConnectorID.String(), binding.BindingKey)] = binding
	}
	out := make([]forumProxyProfile, 0, len(profiles))
	for _, profile := range profiles {
		enriched := profile
		binding, ok := bindingsByKey[forumProxyProfileBindingMapKey(profile.ConnectorID, profile.BindingKey)]
		if ok {
			enriched.HasBinding = true
			enriched.ConversationID = strings.TrimSpace(binding.ConversationID)
			enriched.CursorPresent = strings.TrimSpace(binding.Cursor) != ""
			if binding.LastSyncedAt != nil {
				enriched.LastSyncedAt = binding.LastSyncedAt.UTC().Format(time.RFC3339)
			}
			enriched.Metadata = mergeCommunicationMaps(enriched.Metadata, binding.Metadata)
		}
		out = append(out, enriched)
	}
	return out
}

func (h *Handler) persistForumProxyProfileForExecution(
	ctx context.Context,
	tenantID uuid.UUID,
	thread models.CatalogItem,
	connectorID uuid.UUID,
	profile *forumProxyProfile,
	bindingKey string,
	metadata map[string]any,
) (*forumProxyProfile, error) {
	profiles := forumProxyProfilesFromThread(&thread)
	normalizedBindingKey := normalizeForumProxyBindingKey(bindingKey)
	requestedProfileID := profileIDOrEmpty(profile)
	profileIndex := -1
	if requestedProfileID != "" {
		for idx := range profiles {
			if profiles[idx].ID == requestedProfileID {
				profileIndex = idx
				break
			}
		}
	}
	if profileIndex < 0 {
		profileIndex, _ = findForumProxyProfileByConnectorAndBindingKey(profiles, connectorID.String(), normalizedBindingKey)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	normalizedMetadata := normalizeCommunicationMap(metadata)
	if normalizedMetadata == nil {
		normalizedMetadata = map[string]any{}
	}
	var persisted forumProxyProfile
	if profileIndex >= 0 {
		persisted = profiles[profileIndex]
		persisted.Metadata = mergeCommunicationMaps(persisted.Metadata, normalizedMetadata)
		persisted.Name = forumProxyProfileDisplayName(&persisted, persisted.Metadata)
		persisted.BindingKey = firstNonEmptyString(normalizedBindingKey, normalizeForumProxyBindingKey(persisted.BindingKey), persisted.ID)
		persisted.UpdatedAt = now
		profiles[profileIndex] = persisted
	} else {
		profileID := requestedProfileID
		if profileID == "" {
			profileID = uuid.NewString()
		}
		persisted = forumProxyProfile{
			ID:          profileID,
			ConnectorID: connectorID.String(),
			Name:        forumProxyProfileDisplayName(nil, normalizedMetadata),
			BindingKey:  firstNonEmptyString(normalizedBindingKey, profileID),
			Metadata:    normalizedMetadata,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		profiles = append(profiles, persisted)
	}

	updatedThread, err := h.catalog.Update(ctx, "forum_thread", thread.ID, &tenantID, repository.CatalogUpdateParams{
		Data: map[string]any{
			"proxy_profiles": forumProxyProfilesToPayload(profiles),
		},
	})
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to update forum proxy profiles")
	}
	h.indexForumThreadDocument(ctx, *updatedThread, tenantID)
	for idx := range profiles {
		if profiles[idx].ID == persisted.ID {
			profileCopy := profiles[idx]
			return &profileCopy, nil
		}
	}
	return &persisted, nil
}

func (p forumProxyProfile) toPayload() map[string]any {
	payload := map[string]any{
		"id":           p.ID,
		"connector_id": p.ConnectorID,
		"name":         p.Name,
		"binding_key":  p.BindingKey,
		"metadata":     normalizeCommunicationMap(p.Metadata),
	}
	if strings.TrimSpace(p.CreatedAt) != "" {
		payload["created_at"] = p.CreatedAt
	}
	if strings.TrimSpace(p.UpdatedAt) != "" {
		payload["updated_at"] = p.UpdatedAt
	}
	if strings.TrimSpace(p.LastSyncedAt) != "" {
		payload["last_synced_at"] = p.LastSyncedAt
	}
	if p.HasBinding {
		payload["has_binding"] = true
	}
	if strings.TrimSpace(p.ConversationID) != "" {
		payload["conversation_id"] = p.ConversationID
	}
	if p.CursorPresent {
		payload["cursor_present"] = true
	}
	return payload
}

func normalizeForumProxyProfileID(raw string) string {
	return strings.TrimSpace(raw)
}

func normalizeForumProxyBindingKey(raw string) string {
	return strings.TrimSpace(raw)
}

func deriveForumProxyBindingKey(metadata map[string]any) string {
	target := strings.TrimSpace(stringFromMap(metadata,
		"binding_key",
		"chat_id",
		"chatId",
		"recipient",
		"target",
		"external_user_id",
		"externalUserId",
		"username",
	))
	if target != "" {
		return target
	}
	participant := normalizeCommunicationMap(metadata["participant"])
	return strings.TrimSpace(stringFromMap(participant,
		"binding_key",
		"chat_id",
		"chatId",
		"recipient",
		"target",
		"external_user_id",
		"externalUserId",
		"username",
	))
}

func profileIDOrEmpty(profile *forumProxyProfile) string {
	if profile == nil {
		return ""
	}
	return profile.ID
}

func (h *Handler) SwaggerSpec(c *echo.Context) error {
	return c.Blob(http.StatusOK, "application/json", openAPISpec)
}

func (h *Handler) SwaggerUI(c *echo.Context) error {
	ui := `<!doctype html>
<html>
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>IncidentHub API Docs</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui.css" />
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5.17.14/swagger-ui-bundle.js"></script>
  <script>
    const specUrl = window.location.pathname.replace(/\/+$/, '') + '/openapi.json';
    window.ui = SwaggerUIBundle({
      url: specUrl,
      dom_id: '#swagger-ui',
      deepLinking: true,
      displayRequestDuration: true
    });
  </script>
</body>
</html>`
	return c.HTML(http.StatusOK, ui)
}

func (h *Handler) listDirectionalConnectors(c *echo.Context, tenantID uuid.UUID, direction string) ([]models.CatalogItem, error) {
	primaryKind := "outbound_connectors"
	if direction == "inbound" {
		primaryKind = "inbound_connectors"
	}

	primaryItems, err := h.catalog.List(c.Request().Context(), repository.CatalogListParams{
		Kind:     primaryKind,
		TenantID: &tenantID,
		Limit:    500,
	})
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to list connectors")
	}

	legacyItems, legacyErr := h.catalog.List(c.Request().Context(), repository.CatalogListParams{
		Kind:     "connectors",
		TenantID: &tenantID,
		Limit:    500,
	})
	if legacyErr != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to list connectors")
	}

	out := make([]models.CatalogItem, 0, len(primaryItems)+len(legacyItems))
	seen := make(map[uuid.UUID]struct{}, len(primaryItems)+len(legacyItems))
	appendDirectional := func(items []models.CatalogItem) {
		for _, item := range items {
			if connectorDirectionForItem(item) != direction {
				continue
			}
			if _, exists := seen[item.ID]; exists {
				continue
			}
			seen[item.ID] = struct{}{}
			out = append(out, item)
		}
	}
	appendDirectional(primaryItems)
	appendDirectional(legacyItems)
	return out, nil
}

func connectorDirectionForItem(item models.CatalogItem) string {
	kind := strings.ToLower(strings.TrimSpace(item.Kind))
	if kind == "inbound_connectors" {
		return "inbound"
	}
	if kind == "outbound_connectors" {
		return "outbound"
	}
	return connectorDirection(item.Data)
}

func inboundConnectorRunToPayload(run models.InboundConnectorRun) map[string]any {
	payload := map[string]any{
		"id":                 run.ID.String(),
		"tenant_id":          run.TenantID.String(),
		"connector_id":       run.ConnectorID.String(),
		"trigger":            run.Trigger,
		"started_at":         run.StartedAt.Format(time.RFC3339),
		"status":             run.Status,
		"records_seen":       run.RecordsSeen,
		"alerts_created":     run.AlertsCreated,
		"duplicates_skipped": run.DuplicatesSkipped,
		"errors":             run.Errors,
		"message":            run.Message,
		"logs":               run.Logs,
		"created_at":         run.CreatedAt.Format(time.RFC3339),
		"updated_at":         run.UpdatedAt.Format(time.RFC3339),
	}
	if run.ScheduledFor != nil {
		payload["scheduled_for"] = run.ScheduledFor.Format(time.RFC3339)
	}
	if run.FinishedAt != nil {
		payload["finished_at"] = run.FinishedAt.Format(time.RFC3339)
	}
	return payload
}

func connectorDirection(data map[string]any) string {
	raw := strings.ToLower(strings.TrimSpace(fmt.Sprint(data["direction"])))
	if raw == "" || raw == "<nil>" {
		return "outbound"
	}
	if raw != "inbound" && raw != "outbound" {
		return "outbound"
	}
	return raw
}
