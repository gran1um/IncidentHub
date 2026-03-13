package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"incidenthub/backend/internal/repository"

	"github.com/google/uuid"
)

func TestAdminTelegramNotificationBotCRUD(t *testing.T) {
	env := newAPITestEnv(t)

	telegramServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/botvalid-token/getMe") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"result": map[string]any{
					"id":                          int64(991122),
					"is_bot":                      true,
					"first_name":                  "IncidentHub",
					"username":                    "incidenthub_notify_bot",
					"can_join_groups":             true,
					"can_read_all_group_messages": false,
					"supports_inline_queries":     false,
				},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":          false,
			"description": "Unauthorized",
		})
	}))
	defer telegramServer.Close()
	env.handler.cfg.Outbound.TelegramAPIBaseURL = telegramServer.URL

	var botID string
	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/admin/notification-bots", map[string]any{
			"name":      "SOC notify",
			"bot_token": "valid-token",
			"enabled":   true,
		})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.CreateAdminNotificationBot(c)
		mustStatusOK(t, err, rec, http.StatusCreated)
		payload := decodeBody[map[string]any](t, rec)
		botID, _ = payload["id"].(string)
		if botID == "" {
			t.Fatalf("expected bot id in create response")
		}
		if got, _ := payload["bot_username"].(string); got != "incidenthub_notify_bot" {
			t.Fatalf("unexpected bot_username: %q", got)
		}
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/admin/notification-bots", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListAdminNotificationBots(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		items := decodeBody[[]map[string]any](t, rec)
		if len(items) != 1 {
			t.Fatalf("expected 1 admin bot, got %d", len(items))
		}
	}

	{
		c, rec := env.jsonContext(http.MethodPatch, "/api/v1/admin/notification-bots/"+botID, map[string]any{
			"enabled": false,
		})
		setPath(c, "/api/v1/admin/notification-bots/:botID", []string{"botID"}, []string{botID})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.UpdateAdminNotificationBot(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		payload := decodeBody[map[string]any](t, rec)
		if enabled, _ := payload["enabled"].(bool); enabled {
			t.Fatalf("expected bot to be disabled")
		}
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/notification-bots", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListNotificationBots(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		items := decodeBody[[]map[string]any](t, rec)
		if len(items) != 0 {
			t.Fatalf("expected disabled bot to be hidden in user list")
		}
	}

	{
		c, rec := env.jsonContext(http.MethodDelete, "/api/v1/admin/notification-bots/"+botID, nil)
		setPath(c, "/api/v1/admin/notification-bots/:botID", []string{"botID"}, []string{botID})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.DeleteAdminNotificationBot(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}
}

func TestMyNotificationSettingsSaveAndRead(t *testing.T) {
	env := newAPITestEnv(t)

	seedBot, err := env.notificationBots.Create(context.Background(), repository.CreateTelegramNotificationBotParams{
		TenantID:                env.tenantID,
		Name:                    "Tenant Telegram Bot",
		BotToken:                "test-token",
		BotID:                   778899,
		BotUsername:             "tenant_notify_bot",
		BotFirstName:            "Tenant Bot",
		CanJoinGroups:           true,
		CanReadAllGroupMessages: false,
		SupportsInlineQueries:   false,
		Enabled:                 true,
		CreatedBy:               &env.userID,
	})
	if err != nil {
		t.Fatalf("seed telegram bot: %v", err)
	}

	{
		c, rec := env.jsonContext(http.MethodPut, "/api/v1/me/notification-settings", map[string]any{
			"delivery_enabled":  true,
			"delivery_channel":  "telegram",
			"telegram_bot_id":   seedBot.ID.String(),
			"telegram_chat_id":  "777000001",
			"telegram_username": "soc_operator",
		})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.UpdateMyNotificationSettings(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		payload := decodeBody[map[string]any](t, rec)
		if got, _ := payload["delivery_channel"].(string); got != "telegram" {
			t.Fatalf("unexpected delivery_channel: %q", got)
		}
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/me/notification-settings", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.GetMyNotificationSettings(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		payload := decodeBody[map[string]any](t, rec)
		if enabled, _ := payload["delivery_enabled"].(bool); !enabled {
			t.Fatalf("expected delivery_enabled=true")
		}
		if botID, _ := payload["telegram_bot_id"].(string); botID != seedBot.ID.String() {
			t.Fatalf("unexpected telegram_bot_id: %q", botID)
		}
	}
}

func TestMyNotificationSettingsEmailAndTimeChannels(t *testing.T) {
	env := newAPITestEnv(t)

	{
		c, rec := env.jsonContext(http.MethodPut, "/api/v1/me/notification-settings", map[string]any{
			"delivery_enabled":   true,
			"delivery_channel":   "email",
			"notification_email": "analyst@example.com",
		})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.UpdateMyNotificationSettings(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		payload := decodeBody[map[string]any](t, rec)
		if got, _ := payload["delivery_channel"].(string); got != "email" {
			t.Fatalf("unexpected delivery_channel for email: %q", got)
		}
		if got, _ := payload["notification_email"].(string); got != "analyst@example.com" {
			t.Fatalf("unexpected notification_email: %q", got)
		}
	}

	{
		c, rec := env.jsonContext(http.MethodPut, "/api/v1/me/notification-settings", map[string]any{
			"delivery_enabled": true,
			"delivery_channel": "time",
			"time_recipient":   "soc-time-room",
		})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.UpdateMyNotificationSettings(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		payload := decodeBody[map[string]any](t, rec)
		if got, _ := payload["delivery_channel"].(string); got != "time" {
			t.Fatalf("unexpected delivery_channel for time: %q", got)
		}
		if got, _ := payload["time_recipient"].(string); got != "soc-time-room" {
			t.Fatalf("unexpected time_recipient: %q", got)
		}
	}
}

func TestAdminNotificationSettingsListAndUpdate(t *testing.T) {
	env := newAPITestEnv(t)

	seedBot, err := env.notificationBots.Create(context.Background(), repository.CreateTelegramNotificationBotParams{
		TenantID:                env.tenantID,
		Name:                    "Admin Tenant Telegram Bot",
		BotToken:                "admin-test-token",
		BotID:                   228899,
		BotUsername:             "admin_notify_bot",
		BotFirstName:            "Admin Bot",
		CanJoinGroups:           true,
		CanReadAllGroupMessages: false,
		SupportsInlineQueries:   false,
		Enabled:                 true,
		CreatedBy:               &env.userID,
	})
	if err != nil {
		t.Fatalf("seed telegram bot: %v", err)
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/admin/notification-settings", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListAdminNotificationSettings(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		items := decodeBody[[]map[string]any](t, rec)
		if len(items) != 2 {
			t.Fatalf("expected 2 tenant users in notification settings list, got %d", len(items))
		}
	}

	{
		c, rec := env.jsonContext(http.MethodPut, "/api/v1/admin/notification-settings/"+env.secondUserID.String(), map[string]any{
			"delivery_enabled":  true,
			"delivery_channel":  "telegram",
			"telegram_bot_id":   seedBot.ID.String(),
			"telegram_chat_id":  "777123456",
			"telegram_username": "tenant_analyst",
		})
		setPath(c, "/api/v1/admin/notification-settings/:userID", []string{"userID"}, []string{env.secondUserID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.UpdateAdminNotificationSettings(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		payload := decodeBody[map[string]any](t, rec)
		if got, _ := payload["user_id"].(string); got != env.secondUserID.String() {
			t.Fatalf("expected user_id %s, got %q", env.secondUserID.String(), got)
		}
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/admin/notification-settings", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListAdminNotificationSettings(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		items := decodeBody[[]map[string]any](t, rec)
		var target map[string]any
		for _, item := range items {
			if userID, _ := item["user_id"].(string); userID == env.secondUserID.String() {
				target = item
				break
			}
		}
		if target == nil {
			t.Fatalf("expected user settings for %s", env.secondUserID.String())
		}
		if enabled, _ := target["delivery_enabled"].(bool); !enabled {
			t.Fatalf("expected delivery_enabled=true for updated user")
		}
		if channel, _ := target["delivery_channel"].(string); channel != "telegram" {
			t.Fatalf("expected delivery_channel=telegram, got %q", channel)
		}
		if botID, _ := target["telegram_bot_id"].(string); botID != seedBot.ID.String() {
			t.Fatalf("expected telegram_bot_id=%s, got %q", seedBot.ID.String(), botID)
		}
	}
}

func TestCreateCatalogNotificationEnqueuesDeliveryEvent(t *testing.T) {
	env := newAPITestEnv(t)
	queue := &notificationQueueStub{enabled: true}
	env.handler.notificationQueue = queue

	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/catalog/notifications", map[string]any{
			"owner_id": env.userID.String(),
			"data": map[string]any{
				"title":   "Case updated",
				"message": "A new high severity alert was linked to your case",
				"type":    "warning",
			},
		})
		setPath(c, "/api/v1/catalog/:kind", []string{"kind"}, []string{"notifications"})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.CreateCatalogItem(c)
		mustStatusOK(t, err, rec, http.StatusCreated)
	}

	if len(queue.events) != 1 {
		t.Fatalf("expected one enqueued notification event, got %d", len(queue.events))
	}
	if queue.events[0].UserID != env.userID.String() {
		t.Fatalf("unexpected queued user id: %q", queue.events[0].UserID)
	}

	queue.enqueueErr = errFake("kafka down")
	{
		c, _ := env.jsonContext(http.MethodPost, "/api/v1/catalog/notifications", map[string]any{
			"owner_id": env.userID.String(),
			"data": map[string]any{
				"title":   "Queue failure test",
				"message": "should fail when kafka enqueue fails",
				"type":    "error",
			},
		})
		setPath(c, "/api/v1/catalog/:kind", []string{"kind"}, []string{"notifications"})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.CreateCatalogItem(c)
		if code := httpErrorCode(t, err); code != http.StatusBadGateway {
			t.Fatalf("expected bad gateway on enqueue failure, got %d", code)
		}
	}
}

type errFake string

func (e errFake) Error() string { return string(e) }

func TestUpdateMyNotificationSettingsRejectsUnknownBot(t *testing.T) {
	env := newAPITestEnv(t)

	c, _ := env.jsonContext(http.MethodPut, "/api/v1/me/notification-settings", map[string]any{
		"delivery_enabled": true,
		"delivery_channel": "telegram",
		"telegram_bot_id":  uuid.NewString(),
		"telegram_chat_id": "12345",
	})
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)
	err := env.handler.UpdateMyNotificationSettings(c)
	if code := httpErrorCode(t, err); code != http.StatusBadRequest {
		t.Fatalf("expected bad request for unknown bot, got %d", code)
	}
}

func TestUpdateMyNotificationSettingsChannelValidation(t *testing.T) {
	env := newAPITestEnv(t)

	{
		c, _ := env.jsonContext(http.MethodPut, "/api/v1/me/notification-settings", map[string]any{
			"delivery_enabled": true,
			"delivery_channel": "email",
		})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.UpdateMyNotificationSettings(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("expected bad request for email without address, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodPut, "/api/v1/me/notification-settings", map[string]any{
			"delivery_enabled": true,
			"delivery_channel": "time",
		})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.UpdateMyNotificationSettings(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("expected bad request for time without recipient, got %d", code)
		}
	}
}

func TestUpdateAdminNotificationSettingsRejectsNonTenantUser(t *testing.T) {
	env := newAPITestEnv(t)

	outsideUserID := uuid.NewString()
	c, _ := env.jsonContext(http.MethodPut, "/api/v1/admin/notification-settings/"+outsideUserID, map[string]any{
		"delivery_enabled": true,
		"delivery_channel": "in_app",
	})
	setPath(c, "/api/v1/admin/notification-settings/:userID", []string{"userID"}, []string{outsideUserID})
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)

	err := env.handler.UpdateAdminNotificationSettings(c)
	if code := httpErrorCode(t, err); code != http.StatusNotFound {
		t.Fatalf("expected not found for non-tenant user, got %d", code)
	}
}
