package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/connectors/outbound"
	"incidenthub/backend/internal/forumproxy"
	"incidenthub/backend/internal/repository"

	"github.com/google/uuid"
)

func TestHandlerForumCatalogConnectorsAndCommunicationsIntegration(t *testing.T) {
	env := newAPITestEnv(t)

	createdCase, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-INT-0001",
		Title:             "Forum + communications case",
		Description:       "Case for catalog coverage",
		Source:            "manual",
		IncidentType:      "phishing",
		Status:            "open",
		Priority:          "medium",
		Impact:            "user",
		Confidence:        60,
		Severity:          "medium",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.userID,
	})
	if err != nil {
		t.Fatalf("create setup case: %v", err)
	}

	outboundConnector, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID: &env.tenantID,
		Kind:     "outbound_connectors",
		OwnerID:  &env.userID,
		Data: map[string]any{
			"name":               "telegram-proxy",
			"channel":            "mock",
			"direction":          "outbound",
			"enabled":            true,
			"communication_mode": "chat",
			"capabilities": []string{
				connectorCapabilityHubExecute,
				connectorCapabilityCaseCommunications,
				connectorCapabilityForumThreads,
				connectorCapabilitySyncMessages,
			},
		},
		CreatedBy: &env.userID,
	})
	if err != nil {
		t.Fatalf("create outbound connector: %v", err)
	}

	var threadID uuid.UUID
	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/forum/threads", map[string]any{
			"title":           "Case discussion",
			"status":          "In Progress",
			"case_id":         createdCase.ID.String(),
			"initial_message": "Initial forum message",
		})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.CreateForumThread(c)
		mustStatusOK(t, err, rec, http.StatusCreated)
		payload := decodeBody[map[string]any](t, rec)
		postsCount, _ := payload["posts_count"].(float64)
		if postsCount < 1 {
			t.Fatalf("expected initial forum message to be persisted, got posts_count=%v", payload["posts_count"])
		}
		if strings.TrimSpace(fmt.Sprint(payload["last_post_at"])) == "" {
			t.Fatalf("expected last_post_at in create thread response")
		}
		idRaw, _ := payload["id"].(string)
		parsed, parseErr := uuid.Parse(idRaw)
		if parseErr != nil {
			t.Fatalf("parse thread id: %v", parseErr)
		}
		threadID = parsed
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/forum/threads", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListForumThreads(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		payload := decodeBody[[]map[string]any](t, rec)
		if len(payload) == 0 {
			t.Fatalf("expected forum threads in list response")
		}
		first := payload[0]
		postsCount, _ := first["posts_count"].(float64)
		if postsCount < 1 {
			t.Fatalf("expected posts_count >= 1, got %v", first["posts_count"])
		}
		if strings.TrimSpace(fmt.Sprint(first["last_post_at"])) == "" {
			t.Fatalf("expected last_post_at in list response")
		}
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/forum/threads/"+threadID.String(), nil)
		setPath(c, "/api/v1/forum/threads/:threadID", []string{"threadID"}, []string{threadID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.GetForumThread(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/forum/posts", map[string]any{
			"thread_id": threadID.String(),
			"content":   "Additional forum message",
		})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.CreateForumPost(c)
		mustStatusOK(t, err, rec, http.StatusCreated)
	}

	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/case-comments", map[string]any{
			"case_id":  createdCase.ID.String(),
			"content":  "Case comment via API",
			"authorId": env.userID.String(),
		})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.CreateCaseComment(c)
		mustStatusOK(t, err, rec, http.StatusCreated)
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/case-comments?case_id="+createdCase.ID.String(), nil)
		c.Request().URL.RawQuery = "case_id=" + createdCase.ID.String()
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListCaseComments(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	var notificationID uuid.UUID
	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/catalog/notifications", map[string]any{
			"data": map[string]any{
				"name":    "Incident Notify",
				"enabled": true,
				"type":    "webhook",
			},
		})
		setPath(c, "/api/v1/catalog/:kind", []string{"kind"}, []string{"notifications"})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.CreateCatalogItem(c)
		mustStatusOK(t, err, rec, http.StatusCreated)
		payload := decodeBody[map[string]any](t, rec)
		idRaw, _ := payload["id"].(string)
		parsed, parseErr := uuid.Parse(idRaw)
		if parseErr != nil {
			t.Fatalf("parse catalog item id: %v", parseErr)
		}
		notificationID = parsed
	}

	{
		c, rec := env.jsonContext(http.MethodPatch, "/api/v1/catalog/notifications/"+notificationID.String(), map[string]any{
			"data": map[string]any{"enabled": false},
		})
		setPath(c, "/api/v1/catalog/:kind/:itemID", []string{"kind", "itemID"}, []string{"notifications", notificationID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.UpdateCatalogItem(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	{
		c, rec := env.jsonContext(http.MethodDelete, "/api/v1/catalog/notifications/"+notificationID.String(), nil)
		setPath(c, "/api/v1/catalog/:kind/:itemID", []string{"kind", "itemID"}, []string{"notifications", notificationID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.DeleteCatalogItem(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/search?q=discussion", nil)
		c.Request().URL.RawQuery = "q=discussion"
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.Search(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	var commThreadID uuid.UUID
	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/cases/"+createdCase.ID.String()+"/communications", map[string]any{
			"title":        "External communication",
			"status":       "open",
			"connector_id": outboundConnector.ID.String(),
			"channel":      "telegram",
			"participant":  map[string]any{"chat_id": "12345", "name": "Target User"},
		})
		setPath(c, "/api/v1/cases/:caseID/communications", []string{"caseID"}, []string{createdCase.ID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.CreateCaseCommunication(c)
		mustStatusOK(t, err, rec, http.StatusCreated)
		payload := decodeBody[map[string]any](t, rec)
		idRaw, _ := payload["id"].(string)
		parsed, parseErr := uuid.Parse(idRaw)
		if parseErr != nil {
			t.Fatalf("parse communication thread id: %v", parseErr)
		}
		commThreadID = parsed
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/cases/"+createdCase.ID.String()+"/communications", nil)
		setPath(c, "/api/v1/cases/:caseID/communications", []string{"caseID"}, []string{createdCase.ID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListCaseCommunications(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/cases/"+createdCase.ID.String()+"/communications/"+commThreadID.String(), nil)
		setPath(
			c,
			"/api/v1/cases/:caseID/communications/:threadID",
			[]string{"caseID", "threadID"},
			[]string{createdCase.ID.String(), commThreadID.String()},
		)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.GetCaseCommunication(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	env.handler.forumProxy = forumproxy.NewService(
		env.catalog,
		repository.NewForumExternalBindingRepository(env.pool),
		repository.NewConnectorRecipientAliasRepository(env.pool),
		outbound.NewService(config.OutboundConnectorsConfig{}),
	)

	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/cases/"+createdCase.ID.String()+"/communications/"+commThreadID.String()+"/messages", map[string]any{
			"connector_id": outboundConnector.ID.String(),
			"content":      "Please provide additional details",
			"author":       "SOC Analyst",
		})
		setPath(
			c,
			"/api/v1/cases/:caseID/communications/:threadID/messages",
			[]string{"caseID", "threadID"},
			[]string{createdCase.ID.String(), commThreadID.String()},
		)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.SendCaseCommunicationMessage(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/cases/"+createdCase.ID.String()+"/communications/"+commThreadID.String()+"/sync", map[string]any{
			"connector_id": outboundConnector.ID.String(),
		})
		setPath(
			c,
			"/api/v1/cases/:caseID/communications/:threadID/sync",
			[]string{"caseID", "threadID"},
			[]string{createdCase.ID.String(), commThreadID.String()},
		)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.SyncCaseCommunication(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/communications/connectors", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListCommunicationConnectors(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	var forumProfileID string
	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/forum/threads/"+threadID.String()+"/proxy/profiles", nil)
		setPath(c, "/api/v1/forum/threads/:threadID/proxy/profiles", []string{"threadID"}, []string{threadID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListForumProxyProfiles(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		payload := decodeBody[map[string]any](t, rec)
		profiles, _ := payload["profiles"].([]any)
		if len(profiles) != 0 {
			t.Fatalf("expected no proxy profiles before setup, got %d", len(profiles))
		}
	}

	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/forum/threads/"+threadID.String()+"/proxy/profiles", map[string]any{
			"connector_id": outboundConnector.ID.String(),
			"name":         "Main telegram target",
			"binding_key":  "tg-12345",
			"metadata": map[string]any{
				"chat_id": "12345",
				"target":  "@target_user",
			},
		})
		setPath(c, "/api/v1/forum/threads/:threadID/proxy/profiles", []string{"threadID"}, []string{threadID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.UpsertForumProxyProfile(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		payload := decodeBody[map[string]any](t, rec)
		profile, _ := payload["profile"].(map[string]any)
		forumProfileID = strings.TrimSpace(fmt.Sprint(profile["id"]))
		if forumProfileID == "" {
			t.Fatalf("expected forum proxy profile id in response")
		}
		if hasBinding, _ := profile["has_binding"].(bool); hasBinding {
			t.Fatal("expected new proxy profile to have no binding before first send")
		}
	}

	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/forum/threads/"+threadID.String()+"/proxy/send", map[string]any{
			"profile_id": forumProfileID,
			"content":    "Proxy send message",
			"author":     "SOC Analyst",
		})
		setPath(c, "/api/v1/forum/threads/:threadID/proxy/send", []string{"threadID"}, []string{threadID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ProxyForumSend(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		payload := decodeBody[map[string]any](t, rec)
		if got := strings.TrimSpace(fmt.Sprint(payload["profile_id"])); got != forumProfileID {
			t.Fatalf("expected send response profile_id=%s, got %q", forumProfileID, got)
		}
		if got := strings.TrimSpace(fmt.Sprint(payload["binding_key"])); got != "tg-12345" {
			t.Fatalf("expected send response binding_key=tg-12345, got %q", got)
		}
	}

	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/forum/threads/"+threadID.String()+"/proxy/sync", map[string]any{
			"profile_id": forumProfileID,
		})
		setPath(c, "/api/v1/forum/threads/:threadID/proxy/sync", []string{"threadID"}, []string{threadID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ProxyForumSync(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		payload := decodeBody[map[string]any](t, rec)
		if got := int(payload["created_count"].(float64)); got < 1 {
			t.Fatalf("expected sync to create forum posts, got %d", got)
		}
		if strings.TrimSpace(fmt.Sprint(payload["synced_at"])) == "" {
			t.Fatal("expected synced_at in forum proxy sync response")
		}
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/forum/threads/"+threadID.String()+"/proxy/profiles", nil)
		setPath(c, "/api/v1/forum/threads/:threadID/proxy/profiles", []string{"threadID"}, []string{threadID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListForumProxyProfiles(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		payload := decodeBody[map[string]any](t, rec)
		profiles, _ := payload["profiles"].([]any)
		if len(profiles) != 1 {
			t.Fatalf("expected one proxy profile after send+sync, got %d", len(profiles))
		}
		profile, _ := profiles[0].(map[string]any)
		if hasBinding, _ := profile["has_binding"].(bool); !hasBinding {
			t.Fatal("expected persisted proxy profile to report active binding")
		}
		if strings.TrimSpace(fmt.Sprint(profile["last_synced_at"])) == "" {
			t.Fatal("expected proxy profile last_synced_at after manual sync")
		}
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/forum/threads/"+threadID.String(), nil)
		setPath(c, "/api/v1/forum/threads/:threadID", []string{"threadID"}, []string{threadID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.GetForumThread(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		payload := decodeBody[map[string]any](t, rec)
		profiles, _ := payload["proxy_profiles"].([]any)
		if len(profiles) != 1 {
			t.Fatalf("expected forum thread payload to expose one proxy profile, got %d", len(profiles))
		}
		profile, _ := profiles[0].(map[string]any)
		if strings.TrimSpace(fmt.Sprint(profile["last_synced_at"])) == "" {
			t.Fatal("expected enriched proxy profile sync-state on forum thread payload")
		}
	}

	{
		c, rec := env.jsonContext(http.MethodDelete, "/api/v1/forum/threads/"+threadID.String()+"/proxy/profiles/"+forumProfileID, nil)
		setPath(
			c,
			"/api/v1/forum/threads/:threadID/proxy/profiles/:profileID",
			[]string{"threadID", "profileID"},
			[]string{threadID.String(), forumProfileID},
		)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.DeleteForumProxyProfile(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/dev/swagger/openapi.json", nil)
		err := env.handler.SwaggerSpec(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/dev/swagger", nil)
		err := env.handler.SwaggerUI(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}
}
