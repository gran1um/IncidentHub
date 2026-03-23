package forumproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/connectors/outbound"
	"incidenthub/backend/internal/repository"
	"incidenthub/backend/internal/security"
	"incidenthub/backend/internal/testutil"

	"github.com/google/uuid"
)

func TestServiceResolveOutboundConnectorValidation(t *testing.T) {
	svc := NewService(nil, nil, nil, nil)
	_, err := svc.resolveOutboundConnector(context.Background(), uuid.Nil, uuid.Nil)
	if err == nil || !strings.Contains(err.Error(), "catalog repository is not configured") {
		t.Fatalf("expected catalog repository validation error, got %v", err)
	}
}

func TestServiceSendAndSync(t *testing.T) {
	pool := testutil.OpenTestPool(t)
	testutil.ResetPublicTables(t, pool)
	ctx := context.Background()

	tenants := repository.NewTenantRepository(pool)
	users := repository.NewUserRepository(pool)
	catalog := repository.NewCatalogRepository(pool)
	bindings := repository.NewForumExternalBindingRepository(pool)
	recipientAliases := repository.NewConnectorRecipientAliasRepository(pool)

	tenant, err := tenants.Create(ctx, repository.CreateTenantParams{
		Slug:        "forumproxy-tenant",
		Name:        "Forum Proxy Tenant",
		Description: "forum proxy test tenant",
		MaxUsers:    10,
		IsActive:    true,
	})
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	passwordHash, err := security.HashPassword("Password123!")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	user, err := users.Create(ctx, repository.CreateUserParams{
		Username:        "forumproxy-user",
		Email:           "forumproxy-user@example.com",
		FullName:        "Forum Proxy User",
		PasswordHash:    passwordHash,
		IsPlatformAdmin: false,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	thread, err := catalog.Create(ctx, repository.CatalogCreateParams{
		TenantID:  &tenant.ID,
		Kind:      "forum_threads",
		OwnerID:   &user.ID,
		CreatedBy: &user.ID,
		Data: map[string]any{
			"title": "Thread",
		},
	})
	if err != nil {
		t.Fatalf("create thread: %v", err)
	}

	outboundConnector, err := catalog.Create(ctx, repository.CatalogCreateParams{
		TenantID:  &tenant.ID,
		Kind:      "outbound_connectors",
		OwnerID:   &user.ID,
		CreatedBy: &user.ID,
		Data: map[string]any{
			"name":      "mock connector",
			"direction": "outbound",
			"channel":   "mock",
			"enabled":   true,
		},
	})
	if err != nil {
		t.Fatalf("create outbound connector: %v", err)
	}

	disabledConnector, err := catalog.Create(ctx, repository.CatalogCreateParams{
		TenantID:  &tenant.ID,
		Kind:      "outbound_connectors",
		OwnerID:   &user.ID,
		CreatedBy: &user.ID,
		Data: map[string]any{
			"name":      "disabled connector",
			"direction": "outbound",
			"channel":   "mock",
			"enabled":   false,
		},
	})
	if err != nil {
		t.Fatalf("create disabled connector: %v", err)
	}

	inboundConnector, err := catalog.Create(ctx, repository.CatalogCreateParams{
		TenantID:  &tenant.ID,
		Kind:      "outbound_connectors",
		OwnerID:   &user.ID,
		CreatedBy: &user.ID,
		Data: map[string]any{
			"name":      "wrong direction",
			"direction": "inbound",
			"channel":   "mock",
			"enabled":   true,
		},
	})
	if err != nil {
		t.Fatalf("create inbound-direction connector: %v", err)
	}

	svc := NewService(catalog, bindings, recipientAliases, outbound.NewService(config.OutboundConnectorsConfig{}))

	if _, sendErr := svc.Send(ctx, SendParams{
		TenantID:    tenant.ID,
		ThreadID:    thread.ID,
		ConnectorID: outboundConnector.ID,
		Message:     "   ",
	}); sendErr == nil {
		t.Fatalf("expected validation error for empty message")
	}

	sendResult, err := svc.Send(ctx, SendParams{
		TenantID:    tenant.ID,
		ThreadID:    thread.ID,
		ConnectorID: outboundConnector.ID,
		Author:      "SOC Analyst",
		Message:     "Need confirmation",
		Metadata:    map[string]any{"topic": "phishing"},
	})
	if err != nil {
		t.Fatalf("send through forum proxy: %v", err)
	}
	if !strings.Contains(sendResult.Reply, "Mock reply") {
		t.Fatalf("unexpected send reply: %#v", sendResult)
	}

	syncResult, err := svc.Sync(ctx, SyncParams{
		TenantID:    tenant.ID,
		ThreadID:    thread.ID,
		ConnectorID: outboundConnector.ID,
	})
	if err != nil {
		t.Fatalf("sync through forum proxy: %v", err)
	}
	if len(syncResult.Messages) == 0 {
		t.Fatalf("expected synced messages")
	}

	if _, err := svc.resolveOutboundConnector(ctx, tenant.ID, disabledConnector.ID); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("expected disabled connector error, got %v", err)
	}
	if _, err := svc.resolveOutboundConnector(ctx, tenant.ID, inboundConnector.ID); err == nil || !strings.Contains(err.Error(), "is not outbound") {
		t.Fatalf("expected outbound direction error, got %v", err)
	}
}

func TestServiceTelegramRecipientAliasResolutionAndCapture(t *testing.T) {
	pool := testutil.OpenTestPool(t)
	testutil.ResetPublicTables(t, pool)
	ctx := context.Background()

	tenants := repository.NewTenantRepository(pool)
	users := repository.NewUserRepository(pool)
	catalog := repository.NewCatalogRepository(pool)
	bindings := repository.NewForumExternalBindingRepository(pool)
	recipientAliases := repository.NewConnectorRecipientAliasRepository(pool)

	tenant, err := tenants.Create(ctx, repository.CreateTenantParams{
		Slug:        "forumproxy-telegram-tenant",
		Name:        "Forum Proxy Telegram Tenant",
		Description: "forum proxy telegram test tenant",
		MaxUsers:    10,
		IsActive:    true,
	})
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	passwordHash, err := security.HashPassword("Password123!")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	user, err := users.Create(ctx, repository.CreateUserParams{
		Username:        "forumproxy-telegram-user",
		Email:           "forumproxy-telegram-user@example.com",
		FullName:        "Forum Proxy Telegram User",
		PasswordHash:    passwordHash,
		IsPlatformAdmin: false,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	thread, err := catalog.Create(ctx, repository.CatalogCreateParams{
		TenantID:  &tenant.ID,
		Kind:      "forum_threads",
		OwnerID:   &user.ID,
		CreatedBy: &user.ID,
		Data: map[string]any{
			"title": "Thread",
		},
	})
	if err != nil {
		t.Fatalf("create thread: %v", err)
	}

	lastSentChatID := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/sendMessage"):
			var payload map[string]any
			if decodeErr := json.NewDecoder(r.Body).Decode(&payload); decodeErr != nil {
				t.Fatalf("decode telegram send payload: %v", decodeErr)
			}
			lastSentChatID = strings.TrimSpace(fmt.Sprint(payload["chat_id"]))
			_, _ = w.Write([]byte(`{"ok":true}`))
		case strings.Contains(r.URL.Path, "/getUpdates"):
			_, _ = w.Write([]byte(`{
				"ok": true,
				"result": [
					{
						"update_id": 100,
						"message": {
							"message_id": 200,
							"date": 1700000000,
							"text": "reply",
							"from": {"id": 901, "username": "bob"},
							"chat": {"id": 777}
						}
					}
				]
			}`))
		default:
			t.Fatalf("unexpected telegram endpoint path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	connector, err := catalog.Create(ctx, repository.CatalogCreateParams{
		TenantID:  &tenant.ID,
		Kind:      "outbound_connectors",
		OwnerID:   &user.ID,
		CreatedBy: &user.ID,
		Data: map[string]any{
			"name":      "telegram connector",
			"direction": "outbound",
			"channel":   "telegram",
			"enabled":   true,
			"config": map[string]any{
				"botToken":   "token",
				"apiBaseURL": server.URL,
			},
		},
	})
	if err != nil {
		t.Fatalf("create telegram connector: %v", err)
	}

	if upsertErr := recipientAliases.UpsertAliases(ctx, tenant.ID, connector.ID, repository.ConnectorRecipientAliasUpsertParams{
		Channel:     "telegram",
		RecipientID: "777",
		DisplayName: "Alice",
		Aliases: map[string]string{
			"username": "@alice",
		},
	}); upsertErr != nil {
		t.Fatalf("seed telegram recipient alias: %v", upsertErr)
	}

	svc := NewService(catalog, bindings, recipientAliases, outbound.NewService(config.OutboundConnectorsConfig{}))

	if _, sendErr := svc.Send(ctx, SendParams{
		TenantID:    tenant.ID,
		ThreadID:    thread.ID,
		ConnectorID: connector.ID,
		Author:      "SOC Analyst",
		Message:     "Ping",
		Metadata: map[string]any{
			"participant": map[string]any{
				"target": "@alice",
			},
		},
	}); sendErr != nil {
		t.Fatalf("send telegram message: %v", sendErr)
	}
	if lastSentChatID != "777" {
		t.Fatalf("expected resolved chat id 777, got %q", lastSentChatID)
	}

	if _, syncErr := svc.Sync(ctx, SyncParams{
		TenantID:    tenant.ID,
		ThreadID:    thread.ID,
		ConnectorID: connector.ID,
	}); syncErr != nil {
		t.Fatalf("sync telegram messages: %v", syncErr)
	}

	resolvedID, err := recipientAliases.ResolveRecipientID(ctx, tenant.ID, connector.ID, "telegram", []string{"bob"})
	if err != nil {
		t.Fatalf("resolve synced recipient alias: %v", err)
	}
	if resolvedID != "777" {
		t.Fatalf("expected synced alias to resolve recipient 777, got %q", resolvedID)
	}
}
