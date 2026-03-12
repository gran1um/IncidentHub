package api

import (
	"context"
	"fmt"
	"incidenthub/backend/internal/middleware"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

func (h *Handler) ListForumThreads(c *echo.Context) error {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}

	limit := 300
	if rawLimit := strings.TrimSpace(c.QueryParam("limit")); rawLimit != "" {
		if parsed, parseErr := strconv.Atoi(rawLimit); parseErr == nil {
			limit = parsed
		}
	}

	var caseRefID *uuid.UUID
	if rawCaseID := strings.TrimSpace(c.QueryParam("case_id")); rawCaseID != "" {
		parsedCaseID, parseErr := uuid.Parse(rawCaseID)
		if parseErr != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid case_id")
		}
		caseRefID = &parsedCaseID
	}

	statusFilter := strings.ToLower(strings.TrimSpace(c.QueryParam("status")))
	searchQuery := strings.TrimSpace(c.QueryParam("q"))

	items, err := h.catalog.List(c.Request().Context(), repository.CatalogListParams{
		Kind:     "forum_thread",
		TenantID: &tenantID,
		RefID:    caseRefID,
		Limit:    limit,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list forum threads")
	}

	filtered := make([]models.CatalogItem, 0, len(items))
	for _, item := range items {
		status := strings.ToLower(strings.TrimSpace(stringFromMap(item.Data, "status")))
		if statusFilter != "" && status != statusFilter {
			continue
		}
		if searchQuery != "" && !matchesAllSearchTerms(
			searchQuery,
			stringFromMap(item.Data, "title"),
			status,
			stringFromMap(item.Data, "case_id", "caseId"),
			item.ID.String(),
		) {
			continue
		}
		filtered = append(filtered, item)
	}

	threadIDs := make([]uuid.UUID, 0, len(filtered))
	for _, item := range filtered {
		threadIDs = append(threadIDs, item.ID)
	}
	statsByThread, err := h.catalog.ListForumThreadStats(c.Request().Context(), tenantID, threadIDs)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list forum thread stats")
	}

	result := make([]map[string]any, 0, len(filtered))
	for _, item := range filtered {
		payload := catalogItemToPayload(item)
		enrichForumThreadPayloadWithStats(payload, statsByThread[item.ID], item.UpdatedAt)
		result = append(result, payload)
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) GetForumThread(c *echo.Context) error {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	threadID, err := uuid.Parse(strings.TrimSpace(c.Param("threadID")))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid thread id")
	}
	thread, err := h.catalog.GetByID(c.Request().Context(), "forum_thread", threadID, &tenantID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "thread not found")
	}
	posts, err := h.catalog.List(c.Request().Context(), repository.CatalogListParams{
		Kind:     "forum_post",
		TenantID: &tenantID,
		RefID:    &threadID,
		Limit:    500,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list forum posts")
	}
	sortCatalogItemsChronologically(posts, "timestamp", "created_at")
	result := catalogItemToPayload(*thread)
	result["proxy_profiles"] = forumProxyProfilesToPayload(
		h.enrichForumProxyProfilesWithBindingState(c.Request().Context(), tenantID, threadID, forumProxyProfilesFromThread(thread)),
	)
	postPayloads := mapCatalogItems(posts)
	for _, post := range postPayloads {
		h.enrichForumPostPayload(c.Request().Context(), post)
	}
	result["posts"] = postPayloads
	if len(postPayloads) > 0 {
		lastPost := postPayloads[len(postPayloads)-1]
		lastPostAt := firstNonEmptyString(
			stringFromMap(lastPost, "timestamp", "created_at"),
			thread.UpdatedAt.UTC().Format(time.RFC3339),
		)
		result["posts_count"] = len(postPayloads)
		result["last_post_at"] = lastPostAt
		result["last_activity_at"] = lastPostAt
		result["last_post_id"] = stringFromMap(lastPost, "id")
		result["last_post_preview"] = truncateForumPostPreview(stringFromMap(lastPost, "content"), 280)
		lastAuthorID := stringFromMap(lastPost, "author_id", "authorId")
		lastAuthorName := firstNonEmptyString(
			stringFromMap(lastPost, "author_name", "authorName"),
			stringFromMap(lastPost, "author_id", "authorId"),
		)
		if lastAuthorID != "" {
			result["last_post_author_id"] = lastAuthorID
		}
		if lastAuthorName != "" {
			result["last_post_author_name"] = lastAuthorName
		}
	} else {
		result["posts_count"] = 0
		result["last_activity_at"] = thread.UpdatedAt.UTC().Format(time.RFC3339)
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) CreateForumThread(c *echo.Context) error {
	identity, _ := middleware.GetIdentity(c)
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	var raw map[string]any
	if err := c.Bind(&raw); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	title := stringFromMap(raw, "title")
	if title == "" {
		title = "Thread"
	}
	status := stringFromMap(raw, "status")
	if status == "" {
		status = "In Progress"
	}
	initialMessage := strings.TrimSpace(stringFromMap(raw, "initial_message", "initialMessage"))
	initialAuthorID := identity.UserID
	if rawInitialAuthorID := stringFromMap(raw, "author_id", "authorId", "initial_author_id", "initialAuthorId"); rawInitialAuthorID != "" {
		if parsedInitialAuthorID, parseErr := uuid.Parse(strings.TrimSpace(rawInitialAuthorID)); parseErr == nil {
			initialAuthorID = parsedInitialAuthorID
		}
	}
	initialAuthorName := strings.TrimSpace(stringFromMap(raw, "author_name", "authorName", "initial_author_name", "initialAuthorName"))
	if initialAuthorName == "" {
		initialAuthorName = strings.TrimSpace(identity.Username)
	}

	caseIDRaw := stringFromMap(raw, "case_id", "caseId")
	if caseIDRaw == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "case_id is required")
	}

	caseID, err := uuid.Parse(caseIDRaw)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid case_id")
	}
	refID := &caseID

	exists, err := h.cases.ExistsInTenant(c.Request().Context(), caseID, tenantID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to validate case")
	}
	if !exists {
		return echo.NewHTTPError(http.StatusNotFound, "case not found in tenant")
	}

	existing, err := h.catalog.List(c.Request().Context(), repository.CatalogListParams{
		Kind:     "forum_thread",
		TenantID: &tenantID,
		RefID:    refID,
		Limit:    1,
	})
	if err == nil && len(existing) > 0 {
		return h.respondWithExistingForumThread(c, tenantID, caseID, existing[0], identity.UserID)
	}

	data := map[string]any{
		"title":     title,
		"status":    status,
		"case_id":   caseIDRaw,
		"tenant_id": tenantID.String(),
	}
	item, err := h.catalog.Create(c.Request().Context(), repository.CatalogCreateParams{
		TenantID:  &tenantID,
		Kind:      "forum_thread",
		OwnerID:   &identity.UserID,
		RefID:     refID,
		Data:      data,
		CreatedBy: &identity.UserID,
	})
	if err != nil {
		if isForumThreadUniqueViolation(err) {
			existing, listErr := h.catalog.List(c.Request().Context(), repository.CatalogListParams{
				Kind:     "forum_thread",
				TenantID: &tenantID,
				RefID:    refID,
				Limit:    1,
			})
			if listErr == nil && len(existing) > 0 {
				return h.respondWithExistingForumThread(c, tenantID, caseID, existing[0], identity.UserID)
			}
		}
		return echo.NewHTTPError(http.StatusBadRequest, "failed to create thread")
	}
	_ = h.upsertCaseForumLink(c.Request().Context(), tenantID, caseID, item.ID, identity.UserID)

	var initialPostPayload map[string]any
	if initialMessage != "" {
		postItem, postErr := h.createForumPostRecord(c.Request().Context(), createForumPostRecordParams{
			TenantID:   tenantID,
			ThreadID:   item.ID,
			AuthorID:   initialAuthorID,
			AuthorName: initialAuthorName,
			Content:    initialMessage,
			CreatedBy:  identity.UserID,
		})
		if postErr != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "failed to create initial forum message")
		}
		initialPostPayload = catalogItemToPayload(*postItem)
		h.enrichForumPostPayload(c.Request().Context(), initialPostPayload)
	}

	h.indexForumThreadDocument(c.Request().Context(), *item, tenantID)
	payload := catalogItemToPayload(*item)
	if initialPostPayload != nil {
		payload["initial_post"] = initialPostPayload
	}
	statsByThread, statsErr := h.catalog.ListForumThreadStats(c.Request().Context(), tenantID, []uuid.UUID{item.ID})
	if statsErr == nil {
		enrichForumThreadPayloadWithStats(payload, statsByThread[item.ID], item.UpdatedAt)
	}
	return c.JSON(http.StatusCreated, payload)
}

func (h *Handler) CreateForumPost(c *echo.Context) error {
	identity, _ := middleware.GetIdentity(c)
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	var raw map[string]any
	if err := c.Bind(&raw); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	threadIDRaw := stringFromMap(raw, "thread_id", "threadId")
	if threadIDRaw == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "thread_id is required")
	}
	threadID, err := uuid.Parse(threadIDRaw)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid thread_id")
	}
	content := stringFromMap(raw, "content")
	if content == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "content is required")
	}
	authorID := identity.UserID
	if authorRaw := stringFromMap(raw, "author_id", "authorId"); authorRaw != "" {
		if parsed, parseErr := uuid.Parse(authorRaw); parseErr == nil {
			authorID = parsed
		}
	}
	item, err := h.createForumPostRecord(c.Request().Context(), createForumPostRecordParams{
		TenantID:    tenantID,
		ThreadID:    threadID,
		AuthorID:    authorID,
		AuthorName:  strings.TrimSpace(identity.Username),
		Content:     content,
		Attachments: normalizeAttachmentPayload(raw["attachments"]),
		CreatedBy:   identity.UserID,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to create post")
	}
	payload := catalogItemToPayload(*item)
	h.enrichForumPostPayload(c.Request().Context(), payload)
	return c.JSON(http.StatusCreated, payload)
}

func (h *Handler) ListCaseComments(c *echo.Context) error {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	caseIDRaw := strings.TrimSpace(c.QueryParam("case_id"))
	if caseIDRaw == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "case_id query param is required")
	}
	caseID, err := uuid.Parse(caseIDRaw)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid case_id")
	}

	items, err := h.catalog.List(c.Request().Context(), repository.CatalogListParams{
		Kind:     "case_comment",
		TenantID: &tenantID,
		RefID:    &caseID,
		Limit:    500,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list comments")
	}
	sortCatalogItemsChronologically(items, "created_at", "timestamp")
	return c.JSON(http.StatusOK, mapCatalogItems(items))
}

func (h *Handler) CreateCaseComment(c *echo.Context) error {
	identity, _ := middleware.GetIdentity(c)
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	var raw map[string]any
	if err := c.Bind(&raw); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	caseIDRaw := stringFromMap(raw, "case_id", "caseId")
	if caseIDRaw == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "case_id is required")
	}
	caseID, err := uuid.Parse(caseIDRaw)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid case_id")
	}
	content := stringFromMap(raw, "content")
	if content == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "content is required")
	}
	authorID := identity.UserID
	if rawAuthor := stringFromMap(raw, "author_id", "authorId"); rawAuthor != "" {
		if parsed, parseErr := uuid.Parse(rawAuthor); parseErr == nil {
			authorID = parsed
		}
	}

	data := map[string]any{
		"case_id":    caseID.String(),
		"author_id":  authorID.String(),
		"content":    content,
		"created_at": time.Now().UTC().Format(time.RFC3339),
		"tenant_id":  tenantID.String(),
	}
	item, err := h.catalog.Create(c.Request().Context(), repository.CatalogCreateParams{
		TenantID:  &tenantID,
		Kind:      "case_comment",
		OwnerID:   &authorID,
		RefID:     &caseID,
		Data:      data,
		CreatedBy: &identity.UserID,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to create comment")
	}
	payload := catalogItemToPayload(*item)
	return c.JSON(http.StatusCreated, payload)
}

func (h *Handler) ListCatalogItems(c *echo.Context) error {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	kind := normalizeCatalogKind(c.Param("kind"))
	if kind == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "kind is required")
	}
	if isDeprecatedCatalogKind(kind) {
		return echo.NewHTTPError(http.StatusGone, "catalog kind is deprecated and unavailable")
	}
	includeGlobal := strings.EqualFold(c.QueryParam("include_global"), "true")
	var ownerID *uuid.UUID
	if ownerRaw := strings.TrimSpace(c.QueryParam("owner_id")); ownerRaw != "" {
		parsed, err := uuid.Parse(ownerRaw)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid owner_id")
		}
		ownerID = &parsed
	}
	var refID *uuid.UUID
	if refRaw := strings.TrimSpace(c.QueryParam("ref_id")); refRaw != "" {
		parsed, err := uuid.Parse(refRaw)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid ref_id")
		}
		refID = &parsed
	}
	limit := 300
	if rawLimit := strings.TrimSpace(c.QueryParam("limit")); rawLimit != "" {
		if parsed, err := strconv.Atoi(rawLimit); err == nil {
			limit = parsed
		}
	}

	var items []models.CatalogItem
	if kind == "inbound_connectors" {
		return echo.NewHTTPError(http.StatusGone, "inbound connectors are removed from v1")
	}
	if kind == "connectors" {
		seen := map[uuid.UUID]struct{}{}
		kinds := []string{"outbound_connectors", "connectors"}
		items = make([]models.CatalogItem, 0, limit*2)
		for _, catalogKind := range kinds {
			list, listErr := h.catalog.List(c.Request().Context(), repository.CatalogListParams{
				Kind:          catalogKind,
				TenantID:      &tenantID,
				IncludeGlobal: includeGlobal,
				OwnerID:       ownerID,
				RefID:         refID,
				Limit:         limit,
			})
			if listErr != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to list catalog items")
			}
			for _, item := range list {
				if _, exists := seen[item.ID]; exists {
					continue
				}
				seen[item.ID] = struct{}{}
				items = append(items, item)
			}
		}
	} else {
		list, listErr := h.catalog.List(c.Request().Context(), repository.CatalogListParams{
			Kind:          kind,
			TenantID:      &tenantID,
			IncludeGlobal: includeGlobal,
			OwnerID:       ownerID,
			RefID:         refID,
			Limit:         limit,
		})
		if listErr != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to list catalog items")
		}
		items = list
	}

	if kind == "shifts" {
		month := strings.TrimSpace(c.QueryParam("month"))
		year := strings.TrimSpace(c.QueryParam("year"))
		if month != "" || year != "" {
			filtered := make([]models.CatalogItem, 0, len(items))
			for _, item := range items {
				if month != "" && fmt.Sprint(item.Data["month"]) != month {
					continue
				}
				if year != "" && fmt.Sprint(item.Data["year"]) != year {
					continue
				}
				filtered = append(filtered, item)
			}
			items = filtered
		}
	}

	payloads := mapCatalogItems(items)
	if kind == "achievements" {
		for idx := range payloads {
			h.enrichAchievementPayload(c.Request().Context(), payloads[idx])
		}
	}
	return c.JSON(http.StatusOK, payloads)
}

func (h *Handler) CreateCatalogItem(c *echo.Context) error {
	identity, _ := middleware.GetIdentity(c)
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	kind := normalizeCatalogKind(c.Param("kind"))
	if kind == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "kind is required")
	}
	if isDeprecatedCatalogKind(kind) {
		return echo.NewHTTPError(http.StatusGone, "catalog kind is deprecated and unavailable")
	}
	if isLegacyAPITokensKind(kind) {
		return echo.NewHTTPError(http.StatusBadRequest, "api tokens are not supported for authentication; use jwt bearer tokens")
	}
	if !h.canMutateCatalog(identity, kind, true) {
		return echo.NewHTTPError(http.StatusForbidden, "insufficient role")
	}

	var req createCatalogItemRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.Data == nil {
		req.Data = map[string]any{}
	}
	req.Data["tenant_id"] = tenantID.String()
	if kind == "workflows" && req.Data["definition"] != nil {
		securedDefinition, secureErr := h.vaultizeWorkflowDefinitionPayload(c.Request().Context(), tenantID, identity.UserID, stringFromMap(req.Data, "name"), req.Data["definition"])
		if secureErr != nil {
			return echo.NewHTTPError(http.StatusBadRequest, secureErr.Error())
		}
		req.Data["definition"] = securedDefinition
	}
	if kind == "achievements" {
		xpReward := 150
		if parsedXP, ok := intFromMap(req.Data, "xp_reward", "xpReward", "xp", "experience_points"); ok && parsedXP >= 0 {
			xpReward = parsedXP
		} else if defaultXP, ok := h.loadRewardRulePoints(c.Request().Context(), xpRuleAchievementGranted); ok {
			xpReward = defaultXP
		}
		req.Data["xp_reward"] = xpReward
	}
	switch kind {
	case "connectors", "outbound_connectors":
		kind = "outbound_connectors"
		normalizeOutboundConnectorData(req.Data)
	case "inbound_connectors":
		return echo.NewHTTPError(http.StatusGone, "inbound connectors are removed from v1")
	}

	// For outbound connectors, vaultize sensitive config fields (API keys, passwords, etc.).
	if kind == "outbound_connectors" {
		connectorType := strings.TrimSpace(stringFromMap(req.Data, "type"))
		category := strings.TrimSpace(stringFromMap(req.Data, "category"))
		cfg, secureErr := h.vaultizeConnectorConfig(c.Request().Context(), tenantID, identity.UserID, stringFromMap(req.Data, "name"), connectorType, category, req.Data["config"])
		if secureErr != nil {
			return echo.NewHTTPError(http.StatusBadRequest, secureErr.Error())
		}
		req.Data["config"] = cfg
	}

	var ownerID *uuid.UUID
	if strings.TrimSpace(req.OwnerID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(req.OwnerID))
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid owner_id")
		}
		ownerID = &parsed
	}
	var refID *uuid.UUID
	if strings.TrimSpace(req.RefID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(req.RefID))
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid ref_id")
		}
		refID = &parsed
	}

	item, err := h.catalog.Create(c.Request().Context(), repository.CatalogCreateParams{
		TenantID:  &tenantID,
		Kind:      kind,
		OwnerID:   ownerID,
		RefID:     refID,
		Data:      req.Data,
		CreatedBy: &identity.UserID,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to create catalog item")
	}
	if kind == "notifications" {
		if err := h.enqueueNotificationDelivery(c, *item); err != nil {
			return echo.NewHTTPError(http.StatusBadGateway, "failed to enqueue notification delivery")
		}
	}
	if kind == "user_achievements" {
		h.rewardAchievementGrant(c.Request().Context(), tenantID, *item)
	}
	if kind == "case_connectors" {
		h.rewardConnectorInvocation(c.Request().Context(), tenantID, identity.UserID, "case_connector:"+item.ID.String(), map[string]any{
			"catalog_item_id": item.ID.String(),
			"case_id":         stringFromMap(item.Data, "case_id", "caseId"),
			"triggered":       xpEventTypeConnectorInvoked,
		})
	}
	if kind == "forum_thread" {
		h.indexForumThreadDocument(c.Request().Context(), *item, tenantID)
	}
	payload := catalogItemToPayload(*item)
	if kind == "achievements" {
		h.enrichAchievementPayload(c.Request().Context(), payload)
	}
	return c.JSON(http.StatusCreated, payload)
}

func (h *Handler) UpdateCatalogItem(c *echo.Context) error {
	identity, _ := middleware.GetIdentity(c)
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	kind := normalizeCatalogKind(c.Param("kind"))
	if kind == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "kind is required")
	}
	if isDeprecatedCatalogKind(kind) {
		return echo.NewHTTPError(http.StatusGone, "catalog kind is deprecated and unavailable")
	}
	if isLegacyAPITokensKind(kind) {
		return echo.NewHTTPError(http.StatusBadRequest, "api tokens are not supported for authentication; use jwt bearer tokens")
	}

	itemID, err := uuid.Parse(strings.TrimSpace(c.Param("itemID")))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid item id")
	}

	var req updateCatalogItemRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.Data == nil {
		req.Data = map[string]any{}
	}
	if kind == "inbound_connectors" {
		return echo.NewHTTPError(http.StatusGone, "inbound connectors are removed from v1")
	}
	if kind == "outbound_connectors" || kind == "connectors" {
		if kind == "connectors" {
			kind = "outbound_connectors"
		}
		normalizeOutboundConnectorData(req.Data)
		// Vaultize connector config on update as well.
		connectorType := strings.TrimSpace(stringFromMap(req.Data, "type"))
		category := strings.TrimSpace(stringFromMap(req.Data, "category"))
		cfg, secureErr := h.vaultizeConnectorConfig(c.Request().Context(), tenantID, identity.UserID, stringFromMap(req.Data, "name"), connectorType, category, req.Data["config"])
		if secureErr != nil {
			return echo.NewHTTPError(http.StatusBadRequest, secureErr.Error())
		}
		req.Data["config"] = cfg
	}

	current, err := h.catalog.GetByID(c.Request().Context(), kind, itemID, &tenantID)
	if err != nil {
		if kind != "connectors" {
			return echo.NewHTTPError(http.StatusNotFound, "catalog item not found")
		}
		current, err = h.catalog.GetByID(c.Request().Context(), "outbound_connectors", itemID, &tenantID)
		if err != nil {
			return echo.NewHTTPError(http.StatusNotFound, "catalog item not found")
		}
		kind = "outbound_connectors"
	}
	if !h.canMutateCatalog(identity, kind, false) &&
		(kind != "notifications" || current.OwnerID == nil || *current.OwnerID != identity.UserID) {
		return echo.NewHTTPError(http.StatusForbidden, "insufficient role")
	}
	if kind == "workflows" && req.Data["definition"] != nil {
		workflowName := firstNonEmptyString(stringFromMap(req.Data, "name"), stringFromMap(current.Data, "name"))
		securedDefinition, secureErr := h.vaultizeWorkflowDefinitionPayload(c.Request().Context(), tenantID, identity.UserID, workflowName, req.Data["definition"])
		if secureErr != nil {
			return echo.NewHTTPError(http.StatusBadRequest, secureErr.Error())
		}
		req.Data["definition"] = securedDefinition
	}

	item, err := h.catalog.Update(c.Request().Context(), kind, itemID, &tenantID, repository.CatalogUpdateParams{
		Data: req.Data,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to update catalog item")
	}
	if kind == "forum_thread" {
		h.indexForumThreadDocument(c.Request().Context(), *item, tenantID)
	}
	payload := catalogItemToPayload(*item)
	if kind == "achievements" {
		h.enrichAchievementPayload(c.Request().Context(), payload)
	}
	return c.JSON(http.StatusOK, payload)
}

func (h *Handler) DeleteCatalogItem(c *echo.Context) error {
	identity, _ := middleware.GetIdentity(c)
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	kind := normalizeCatalogKind(c.Param("kind"))
	if kind == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "kind is required")
	}
	if isDeprecatedCatalogKind(kind) {
		return echo.NewHTTPError(http.StatusGone, "catalog kind is deprecated and unavailable")
	}
	if !h.canMutateCatalog(identity, kind, false) {
		return echo.NewHTTPError(http.StatusForbidden, "insufficient role")
	}
	itemID, err := uuid.Parse(strings.TrimSpace(c.Param("itemID")))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid item id")
	}
	if kind == "inbound_connectors" {
		return echo.NewHTTPError(http.StatusGone, "inbound connectors are removed from v1")
	}
	if err := h.catalog.Delete(c.Request().Context(), kind, itemID, &tenantID); err != nil {
		if kind != "connectors" {
			return echo.NewHTTPError(http.StatusNotFound, "catalog item not found")
		}
		if deleteErr := h.catalog.Delete(c.Request().Context(), "outbound_connectors", itemID, &tenantID); deleteErr == nil {
			return c.JSON(http.StatusOK, map[string]any{"success": true})
		}
		return echo.NewHTTPError(http.StatusNotFound, "catalog item not found")
	}
	if kind == "forum_thread" && h.search != nil {
		_ = h.search.DeleteDocument(c.Request().Context(), "forum_threads", itemID.String())
	}
	return c.JSON(http.StatusOK, map[string]any{"success": true})
}

func (h *Handler) Search(c *echo.Context) error {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	query := strings.TrimSpace(c.QueryParam("q"))
	if len(query) < 2 {
		return c.JSON(http.StatusOK, map[string]any{
			"alerts":  []any{},
			"cases":   []any{},
			"threads": []any{},
		})
	}

	alerts := make([]map[string]any, 0)
	cases := make([]map[string]any, 0)
	threads := make([]map[string]any, 0)

	if h.search != nil {
		hits, err := h.search.Search(c.Request().Context(), tenantID.String(), query, []string{"alerts", "cases", "forum_threads"}, 80)
		if err == nil {
			for _, hit := range hits {
				switch hit.Kind {
				case "alerts":
					alerts = append(alerts, map[string]any{
						"id":       firstNonEmptyString(fmt.Sprint(hit.Source["id"]), hit.ID),
						"title":    firstNonEmptyString(fmt.Sprint(hit.Source["title"]), "Alert"),
						"status":   fmt.Sprint(hit.Source["status"]),
						"severity": fmt.Sprint(hit.Source["severity"]),
					})
				case "cases":
					cases = append(cases, map[string]any{
						"id":       firstNonEmptyString(fmt.Sprint(hit.Source["id"]), hit.ID),
						"title":    firstNonEmptyString(fmt.Sprint(hit.Source["title"]), "Case"),
						"status":   fmt.Sprint(hit.Source["status"]),
						"severity": fmt.Sprint(hit.Source["severity"]),
					})
				case "forum_threads":
					threads = append(threads, map[string]any{
						"id":    firstNonEmptyString(fmt.Sprint(hit.Source["id"]), hit.ID),
						"title": firstNonEmptyString(fmt.Sprint(hit.Source["title"]), "Thread"),
					})
				}
			}
		}
	}

	if len(alerts) == 0 {
		list, err := h.alerts.ListByTenant(c.Request().Context(), tenantID, 300, 0)
		if err == nil {
			for _, item := range list {
				if matchesAllSearchTerms(query, item.Title, item.Description, item.Source) {
					alerts = append(alerts, map[string]any{
						"id":       item.ID.String(),
						"title":    item.Title,
						"status":   item.Status,
						"severity": item.Severity,
					})
				}
			}
		}
	}
	if len(cases) == 0 {
		list, err := h.cases.ListByTenant(c.Request().Context(), tenantID, 300, 0)
		if err == nil {
			for _, item := range list {
				if matchesAllSearchTerms(query, item.Title, item.Description) {
					cases = append(cases, map[string]any{
						"id":       item.ID.String(),
						"title":    item.Title,
						"status":   item.Status,
						"severity": item.Severity,
					})
				}
			}
		}
	}
	if len(threads) == 0 {
		list, err := h.catalog.List(c.Request().Context(), repository.CatalogListParams{
			Kind:     "forum_thread",
			TenantID: &tenantID,
			Limit:    300,
		})
		if err == nil {
			for _, item := range list {
				if matchesAllSearchTerms(query, fmt.Sprint(item.Data["title"])) {
					threads = append(threads, map[string]any{
						"id":    item.ID.String(),
						"title": fmt.Sprint(item.Data["title"]),
					})
				}
			}
		}
	}
	return c.JSON(http.StatusOK, map[string]any{
		"alerts":  alerts,
		"cases":   cases,
		"threads": threads,
	})
}

func (h *Handler) indexForumThreadDocument(ctx context.Context, item models.CatalogItem, fallbackTenantID uuid.UUID) {
	if h == nil || h.search == nil {
		return
	}
	_ = h.search.IndexDocument(ctx, "forum_threads", item.ID.String(), buildForumThreadSearchDocument(item, fallbackTenantID))
}

func buildForumThreadSearchDocument(item models.CatalogItem, fallbackTenantID uuid.UUID) map[string]any {
	tenantID := fallbackTenantID
	if item.TenantID != nil && *item.TenantID != uuid.Nil {
		tenantID = *item.TenantID
	}

	caseID := stringFromMap(item.Data, "case_id", "caseId")
	if caseID == "" && item.RefID != nil && *item.RefID != uuid.Nil {
		caseID = item.RefID.String()
	}

	status := stringFromMap(item.Data, "status")
	if status == "" {
		status = "in_progress"
	}

	title := stringFromMap(item.Data, "title")
	if title == "" {
		title = "Thread"
	}

	return map[string]any{
		"id":         item.ID.String(),
		"tenant_id":  tenantID.String(),
		"title":      title,
		"status":     status,
		"case_id":    caseID,
		"kind":       "forum_thread",
		"created_at": item.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at": item.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

type createForumPostRecordParams struct {
	TenantID    uuid.UUID
	ThreadID    uuid.UUID
	AuthorID    uuid.UUID
	AuthorName  string
	Content     string
	Attachments []map[string]any
	CreatedBy   uuid.UUID
}

func (h *Handler) createForumPostRecord(ctx context.Context, params createForumPostRecordParams) (*models.CatalogItem, error) {
	data := map[string]any{
		"thread_id":  params.ThreadID.String(),
		"author_id":  params.AuthorID.String(),
		"content":    params.Content,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"tenant_id":  params.TenantID.String(),
		"authorName": strings.TrimSpace(params.AuthorName),
	}
	if len(params.Attachments) > 0 {
		data["attachments"] = params.Attachments
	}
	return h.catalog.Create(ctx, repository.CatalogCreateParams{
		TenantID:  &params.TenantID,
		Kind:      "forum_post",
		OwnerID:   &params.AuthorID,
		RefID:     &params.ThreadID,
		Data:      data,
		CreatedBy: &params.CreatedBy,
	})
}

func enrichForumThreadPayloadWithStats(payload map[string]any, stats repository.ForumThreadStats, fallbackUpdatedAt time.Time) {
	if payload == nil {
		return
	}

	if stats.ThreadID == uuid.Nil || stats.PostsCount <= 0 {
		payload["posts_count"] = 0
		payload["last_activity_at"] = fallbackUpdatedAt.UTC().Format(time.RFC3339)
		return
	}

	lastPostAt := stats.LastPostAt.UTC().Format(time.RFC3339)
	payload["posts_count"] = stats.PostsCount
	payload["last_post_id"] = stats.LastPostID.String()
	payload["last_post_at"] = lastPostAt
	payload["last_activity_at"] = lastPostAt
	payload["last_post_preview"] = truncateForumPostPreview(stats.LastPostPreview, 280)
	if stats.LastPostAuthorID != "" {
		payload["last_post_author_id"] = stats.LastPostAuthorID
	}
	if stats.LastPostAuthorName != "" {
		payload["last_post_author_name"] = stats.LastPostAuthorName
	}
}

func truncateForumPostPreview(value string, maxLen int) string {
	trimmed := strings.TrimSpace(value)
	if maxLen <= 0 || len(trimmed) <= maxLen {
		return trimmed
	}
	return strings.TrimSpace(trimmed[:maxLen]) + "…"
}

func mergeCatalogData(base map[string]any, patch map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range base {
		out[key] = value
	}
	for key, value := range patch {
		out[key] = value
	}
	return out
}

func sortCatalogItemsChronologically(items []models.CatalogItem, keys ...string) {
	sort.SliceStable(items, func(i, j int) bool {
		leftTime := catalogItemChronologyTime(items[i], keys...)
		rightTime := catalogItemChronologyTime(items[j], keys...)
		if leftTime.Equal(rightTime) {
			return items[i].ID.String() < items[j].ID.String()
		}
		return leftTime.Before(rightTime)
	})
}

func catalogItemChronologyTime(item models.CatalogItem, keys ...string) time.Time {
	raw := strings.TrimSpace(stringFromMap(item.Data, keys...))
	if raw != "" {
		if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
			return parsed
		}
	}
	return item.CreatedAt
}

func (h *Handler) respondWithExistingForumThread(
	c *echo.Context,
	tenantID uuid.UUID,
	caseID uuid.UUID,
	thread models.CatalogItem,
	actorID uuid.UUID,
) error {
	ctx := c.Request().Context()
	h.indexForumThreadDocument(ctx, thread, tenantID)
	_ = h.upsertCaseForumLink(ctx, tenantID, caseID, thread.ID, actorID)
	payload := catalogItemToPayload(thread)
	statsByThread, statsErr := h.catalog.ListForumThreadStats(ctx, tenantID, []uuid.UUID{thread.ID})
	if statsErr == nil {
		enrichForumThreadPayloadWithStats(payload, statsByThread[thread.ID], thread.UpdatedAt)
	}
	return c.JSON(http.StatusOK, payload)
}
