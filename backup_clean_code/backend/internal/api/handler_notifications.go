package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"incidenthub/backend/internal/middleware"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v5"
)

// Sentinel errors for notification handlers (err113).
var (
	errTelegramTokenValidationFailed = errors.New("telegram token validation failed")
)

type createTelegramNotificationBotRequest struct {
	Name     string `json:"name"`
	BotToken string `json:"bot_token"`
	Enabled  *bool  `json:"enabled"`
}

type updateTelegramNotificationBotRequest struct {
	Name     *string `json:"name"`
	BotToken *string `json:"bot_token"`
	Enabled  *bool   `json:"enabled"`
}

type updateMyNotificationSettingsRequest struct {
	DeliveryEnabled   *bool   `json:"delivery_enabled"`
	DeliveryChannel   string  `json:"delivery_channel"`
	TelegramBotID     *string `json:"telegram_bot_id"`
	TelegramChatID    *string `json:"telegram_chat_id"`
	TelegramUsername  *string `json:"telegram_username"`
	NotificationEmail *string `json:"notification_email"`
	TimeRecipient     *string `json:"time_recipient"`
}

type telegramBotProfile struct {
	BotID                   int64
	Username                string
	FirstName               string
	CanJoinGroups           bool
	CanReadAllGroupMessages bool
	SupportsInlineQueries   bool
}

type telegramGetMeResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
	Result      struct {
		ID                      int64  `json:"id"`
		IsBot                   bool   `json:"is_bot"`
		FirstName               string `json:"first_name"`
		Username                string `json:"username"`
		CanJoinGroups           bool   `json:"can_join_groups"`
		CanReadAllGroupMessages bool   `json:"can_read_all_group_messages"`
		SupportsInlineQueries   bool   `json:"supports_inline_queries"`
	} `json:"result"`
}

func (h *Handler) listNotificationBots(c *echo.Context, adminOnly bool) error {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	if h.notificationBots == nil {
		return c.JSON(http.StatusOK, []map[string]any{})
	}

	items, err := h.notificationBots.ListByTenant(c.Request().Context(), tenantID, adminOnly, 300)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list notification bots")
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, notificationBotPayload(item))
	}
	return c.JSON(http.StatusOK, out)
}

func (h *Handler) ListAdminNotificationBots(c *echo.Context) error {
	return h.listNotificationBots(c, true)
}

func (h *Handler) CreateAdminNotificationBot(c *echo.Context) error {
	identity, _ := middleware.GetIdentity(c)
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	if h.notificationBots == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "notification bot storage is not configured")
	}

	var req createTelegramNotificationBotRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	name := strings.TrimSpace(req.Name)
	token := strings.TrimSpace(req.BotToken)
	if name == "" || token == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name and bot_token are required")
	}

	profile, err := h.fetchTelegramBotProfile(c.Request().Context(), token)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	item, err := h.notificationBots.Create(c.Request().Context(), repository.CreateTelegramNotificationBotParams{
		TenantID:                tenantID,
		Name:                    name,
		BotToken:                token,
		BotID:                   profile.BotID,
		BotUsername:             profile.Username,
		BotFirstName:            profile.FirstName,
		CanJoinGroups:           profile.CanJoinGroups,
		CanReadAllGroupMessages: profile.CanReadAllGroupMessages,
		SupportsInlineQueries:   profile.SupportsInlineQueries,
		Enabled:                 enabled,
		CreatedBy:               &identity.UserID,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to create notification bot")
	}
	return c.JSON(http.StatusCreated, notificationBotPayload(*item))
}

func (h *Handler) UpdateAdminNotificationBot(c *echo.Context) error {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	if h.notificationBots == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "notification bot storage is not configured")
	}

	botID, err := uuid.Parse(strings.TrimSpace(c.Param("botID")))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid bot id")
	}

	var req updateTelegramNotificationBotRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	updateParams := repository.UpdateTelegramNotificationBotParams{
		Name:    req.Name,
		Enabled: req.Enabled,
	}
	if req.BotToken != nil && strings.TrimSpace(*req.BotToken) != "" {
		token := strings.TrimSpace(*req.BotToken)
		profile, verifyErr := h.fetchTelegramBotProfile(c.Request().Context(), token)
		if verifyErr != nil {
			return echo.NewHTTPError(http.StatusBadRequest, verifyErr.Error())
		}
		updateParams.BotToken = &token
		updateParams.BotID = &profile.BotID
		updateParams.BotUsername = &profile.Username
		updateParams.BotFirstName = &profile.FirstName
		updateParams.CanJoinGroups = &profile.CanJoinGroups
		updateParams.CanReadAllGroupMessages = &profile.CanReadAllGroupMessages
		updateParams.SupportsInlineQueries = &profile.SupportsInlineQueries
	}

	item, err := h.notificationBots.Update(c.Request().Context(), tenantID, botID, updateParams)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return echo.NewHTTPError(http.StatusNotFound, "notification bot not found")
		}
		return echo.NewHTTPError(http.StatusBadRequest, "failed to update notification bot")
	}
	return c.JSON(http.StatusOK, notificationBotPayload(*item))
}

func (h *Handler) DeleteAdminNotificationBot(c *echo.Context) error {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	if h.notificationBots == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "notification bot storage is not configured")
	}
	botID, err := uuid.Parse(strings.TrimSpace(c.Param("botID")))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid bot id")
	}
	if err := h.notificationBots.Delete(c.Request().Context(), tenantID, botID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, "notification bot not found")
		}
		return echo.NewHTTPError(http.StatusBadRequest, "failed to delete notification bot")
	}
	return c.JSON(http.StatusOK, map[string]any{"success": true})
}

func (h *Handler) ListNotificationBots(c *echo.Context) error {
	return h.listNotificationBots(c, false)
}

func (h *Handler) ListAdminNotificationSettings(c *echo.Context) error {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	if h.users == nil || h.notificationSettings == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "notification settings storage is not configured")
	}

	tenantUsers, err := h.users.ListByTenantWithRole(c.Request().Context(), tenantID, 500, 0)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list tenant users")
	}
	settings, err := h.notificationSettings.ListByTenant(c.Request().Context(), tenantID, 1000, 0)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list notification settings")
	}

	settingsByUserID := make(map[uuid.UUID]models.UserNotificationSettings, len(settings))
	for _, item := range settings {
		settingsByUserID[item.UserID] = item
	}

	result := make([]map[string]any, 0, len(tenantUsers))
	for _, tenantUser := range tenantUsers {
		var userSettings *models.UserNotificationSettings
		if item, exists := settingsByUserID[tenantUser.ID]; exists {
			itemCopy := item
			userSettings = &itemCopy
		}
		result = append(result, adminNotificationSettingsPayload(tenantUser, userSettings))
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) GetMyNotificationSettings(c *echo.Context) error {
	identity, _ := middleware.GetIdentity(c)
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	if h.notificationSettings == nil {
		return c.JSON(http.StatusOK, defaultNotificationSettingsPayload(identity.UserID, tenantID))
	}

	item, err := h.notificationSettings.GetByUser(c.Request().Context(), tenantID, identity.UserID)
	if err != nil {
		if errorsIsNoRows(err) {
			return c.JSON(http.StatusOK, defaultNotificationSettingsPayload(identity.UserID, tenantID))
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load notification settings")
	}
	return c.JSON(http.StatusOK, notificationSettingsPayload(*item))
}

func (h *Handler) UpdateMyNotificationSettings(c *echo.Context) error {
	identity, _ := middleware.GetIdentity(c)
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	if h.notificationSettings == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "notification settings storage is not configured")
	}

	var req updateMyNotificationSettingsRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	item, err := h.upsertNotificationSettings(c.Request().Context(), tenantID, identity.UserID, identity.UserID, req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, notificationSettingsPayload(*item))
}

func (h *Handler) UpdateAdminNotificationSettings(c *echo.Context) error {
	identity, _ := middleware.GetIdentity(c)
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	if h.notificationSettings == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "notification settings storage is not configured")
	}
	targetUserID, err := uuid.Parse(strings.TrimSpace(c.Param("userID")))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user id")
	}
	targetMembership, err := h.memberships.Get(c.Request().Context(), tenantID, targetUserID)
	if err != nil || !targetMembership.IsActive {
		return echo.NewHTTPError(http.StatusNotFound, "user is not a member of tenant")
	}

	var req updateMyNotificationSettingsRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	item, err := h.upsertNotificationSettings(c.Request().Context(), tenantID, targetUserID, identity.UserID, req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, notificationSettingsPayload(*item))
}

func (h *Handler) upsertNotificationSettings(ctx context.Context, tenantID, userID, updatedBy uuid.UUID, req updateMyNotificationSettingsRequest) (*models.UserNotificationSettings, error) {
	current, currentErr := h.notificationSettings.GetByUser(ctx, tenantID, userID)
	if currentErr != nil && !errorsIsNoRows(currentErr) {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to load notification settings")
	}

	channel := "in_app"
	deliveryEnabled := false
	telegramBotID := (*uuid.UUID)(nil)
	telegramChatID := ""
	telegramUsername := ""
	notificationEmail := ""
	timeRecipient := ""
	if current != nil {
		channel = current.DeliveryChannel
		deliveryEnabled = current.DeliveryEnabled
		telegramBotID = current.TelegramBotID
		telegramChatID = current.TelegramChatID
		telegramUsername = current.TelegramUsername
		notificationEmail = current.NotificationEmail
		timeRecipient = current.TimeRecipient
	}
	if req.DeliveryChannel != "" {
		channel = strings.ToLower(strings.TrimSpace(req.DeliveryChannel))
	}
	if req.DeliveryEnabled != nil {
		deliveryEnabled = *req.DeliveryEnabled
	}
	if req.TelegramBotID != nil {
		rawBotID := strings.TrimSpace(*req.TelegramBotID)
		if rawBotID == "" {
			telegramBotID = nil
		} else {
			parsed, err := uuid.Parse(rawBotID)
			if err != nil {
				return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid telegram_bot_id")
			}
			telegramBotID = &parsed
		}
	}
	if req.TelegramChatID != nil {
		telegramChatID = strings.TrimSpace(*req.TelegramChatID)
	}
	if req.TelegramUsername != nil {
		telegramUsername = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(*req.TelegramUsername)), "@")
	}
	if req.NotificationEmail != nil {
		notificationEmail = strings.TrimSpace(strings.ToLower(*req.NotificationEmail))
	}
	if req.TimeRecipient != nil {
		timeRecipient = strings.TrimSpace(*req.TimeRecipient)
	}

	switch channel {
	case "in_app", "telegram", "email", "time":
	default:
		return nil, echo.NewHTTPError(http.StatusBadRequest, "unsupported delivery_channel")
	}
	if channel == "telegram" && deliveryEnabled {
		if telegramBotID == nil {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "telegram_bot_id is required for telegram delivery")
		}
		if strings.TrimSpace(telegramChatID) == "" && strings.TrimSpace(telegramUsername) == "" {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "telegram_chat_id or telegram_username is required")
		}
		if h.notificationBots == nil {
			return nil, echo.NewHTTPError(http.StatusServiceUnavailable, "notification bot storage is not configured")
		}
		bot, err := h.notificationBots.GetByID(ctx, tenantID, *telegramBotID)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "telegram bot is not available")
		}
		if !bot.Enabled {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "selected telegram bot is disabled")
		}
	}
	if channel == "email" && deliveryEnabled {
		if notificationEmail == "" {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "notification_email is required for email delivery")
		}
	}
	if channel == "time" && deliveryEnabled {
		if timeRecipient == "" {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "time_recipient is required for time delivery")
		}
	}

	item, err := h.notificationSettings.Upsert(ctx, repository.UpsertUserNotificationSettingsParams{
		TenantID:          tenantID,
		UserID:            userID,
		DeliveryEnabled:   deliveryEnabled,
		DeliveryChannel:   channel,
		TelegramBotID:     telegramBotID,
		TelegramChatID:    telegramChatID,
		TelegramUsername:  telegramUsername,
		NotificationEmail: notificationEmail,
		TimeRecipient:     timeRecipient,
		UpdatedBy:         &updatedBy,
	})
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "failed to save notification settings")
	}
	return item, nil
}

func (h *Handler) fetchTelegramBotProfile(ctx context.Context, token string) (telegramBotProfile, error) {
	trimmedToken := strings.TrimSpace(token)
	if trimmedToken == "" {
		return telegramBotProfile{}, fmt.Errorf("telegram bot token is required")
	}
	baseURL := strings.TrimSpace(h.cfg.Outbound.TelegramAPIBaseURL)
	if baseURL == "" {
		baseURL = "https://api.telegram.org"
	}
	requestURL := strings.TrimRight(baseURL, "/") + "/bot" + trimmedToken + "/getMe"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, http.NoBody)
	if err != nil {
		return telegramBotProfile{}, fmt.Errorf("build telegram getMe request: %w", err)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return telegramBotProfile{}, fmt.Errorf("request telegram getMe failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var payload telegramGetMeResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return telegramBotProfile{}, fmt.Errorf("decode telegram getMe response: %w", err)
	}
	if !payload.OK {
		description := strings.TrimSpace(payload.Description)
		if description == "" {
			return telegramBotProfile{}, errTelegramTokenValidationFailed
		}
		return telegramBotProfile{}, fmt.Errorf("%w: %s", errTelegramTokenValidationFailed, description)
	}
	if !payload.Result.IsBot || payload.Result.ID == 0 || strings.TrimSpace(payload.Result.Username) == "" {
		return telegramBotProfile{}, fmt.Errorf("telegram getMe response is invalid")
	}
	return telegramBotProfile{
		BotID:                   payload.Result.ID,
		Username:                strings.TrimSpace(strings.TrimPrefix(strings.ToLower(payload.Result.Username), "@")),
		FirstName:               strings.TrimSpace(payload.Result.FirstName),
		CanJoinGroups:           payload.Result.CanJoinGroups,
		CanReadAllGroupMessages: payload.Result.CanReadAllGroupMessages,
		SupportsInlineQueries:   payload.Result.SupportsInlineQueries,
	}, nil
}

func notificationBotPayload(item models.TelegramNotificationBot) map[string]any {
	return map[string]any{
		"id":                          item.ID.String(),
		"tenant_id":                   item.TenantID.String(),
		"name":                        item.Name,
		"bot_id":                      item.BotID,
		"bot_username":                item.BotUsername,
		"bot_first_name":              item.BotFirstName,
		"can_join_groups":             item.CanJoinGroups,
		"can_read_all_group_messages": item.CanReadAllGroupMessages,
		"supports_inline_queries":     item.SupportsInlineQueries,
		"enabled":                     item.Enabled,
		"created_by":                  uuidStringOrEmpty(item.CreatedBy),
		"created_at":                  item.CreatedAt,
		"updated_at":                  item.UpdatedAt,
	}
}

func notificationSettingsPayload(item models.UserNotificationSettings) map[string]any {
	return map[string]any{
		"id":                 item.ID.String(),
		"tenant_id":          item.TenantID.String(),
		"user_id":            item.UserID.String(),
		"delivery_enabled":   item.DeliveryEnabled,
		"delivery_channel":   item.DeliveryChannel,
		"telegram_bot_id":    uuidStringOrEmpty(item.TelegramBotID),
		"telegram_chat_id":   item.TelegramChatID,
		"telegram_username":  item.TelegramUsername,
		"notification_email": item.NotificationEmail,
		"time_recipient":     item.TimeRecipient,
		"updated_by":         uuidStringOrEmpty(item.UpdatedBy),
		"created_at":         item.CreatedAt,
		"updated_at":         item.UpdatedAt,
	}
}

func adminNotificationSettingsPayload(user models.TenantUser, item *models.UserNotificationSettings) map[string]any {
	name := strings.TrimSpace(user.FullName)
	if name == "" {
		name = strings.TrimSpace(user.Username)
	}
	payload := defaultNotificationSettingsPayload(user.ID, user.TenantID)
	if item != nil {
		payload = notificationSettingsPayload(*item)
	}
	payload["user_name"] = name
	payload["user_email"] = user.Email
	payload["user_role"] = user.Role
	payload["user_active"] = user.IsActive
	payload["is_platform_admin"] = user.IsPlatformAdmin
	return payload
}

func defaultNotificationSettingsPayload(userID, tenantID uuid.UUID) map[string]any {
	return map[string]any{
		"id":                 "",
		"tenant_id":          tenantID.String(),
		"user_id":            userID.String(),
		"delivery_enabled":   false,
		"delivery_channel":   "in_app",
		"telegram_bot_id":    "",
		"telegram_chat_id":   "",
		"telegram_username":  "",
		"notification_email": "",
		"time_recipient":     "",
	}
}

func uuidStringOrEmpty(value *uuid.UUID) string {
	if value == nil {
		return ""
	}
	return value.String()
}

func errorsIsNoRows(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, pgx.ErrNoRows)
}

func (h *Handler) enqueueNotificationDelivery(c *echo.Context, item models.CatalogItem) error {
	if h.notificationQueue == nil || !h.notificationQueue.Enabled() {
		return nil
	}
	if item.OwnerID == nil || item.TenantID == nil {
		return nil
	}
	title := strings.TrimSpace(stringFromMap(item.Data, "title", "name"))
	message := strings.TrimSpace(stringFromMap(item.Data, "message", "description"))
	if title == "" && message == "" {
		return nil
	}
	eventType := strings.TrimSpace(strings.ToLower(stringFromMap(item.Data, "type")))
	if eventType == "" {
		eventType = "info"
	}
	event := NotificationDeliveryEvent{
		EventID:        uuid.NewString(),
		NotificationID: item.ID.String(),
		TenantID:       item.TenantID.String(),
		UserID:         item.OwnerID.String(),
		Title:          title,
		Message:        message,
		Type:           eventType,
		CreatedAt:      time.Now().UTC().Format(time.RFC3339Nano),
	}
	return h.notificationQueue.Enqueue(c.Request().Context(), event)
}
