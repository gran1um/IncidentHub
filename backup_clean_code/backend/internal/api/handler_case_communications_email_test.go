package api

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
	"incidenthub/backend/internal/forumproxy"
	"incidenthub/backend/internal/repository"

	"github.com/google/uuid"
)

func TestCaseCommunicationEmailTemplateAndSyncBySubject(t *testing.T) {
	env := newAPITestEnv(t)
	caseID := createCaseForTelegramCommunicationTest(t, env, "CASE-EMAIL-COMMS-0001")

	var capturedSendPayload map[string]any
	var capturedSyncSubject string
	emailServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/mail/send":
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST request to email send endpoint, got %s", r.Method)
			}
			if err := json.NewDecoder(r.Body).Decode(&capturedSendPayload); err != nil {
				t.Fatalf("decode email send payload: %v", err)
			}
			_, _ = w.Write([]byte(`{"reply":"email accepted","conversation_id":"mail-conv-1","cursor":"mail-cursor-1"}`))
		case "/mail/poll":
			if r.Method != http.MethodGet {
				t.Fatalf("expected GET request to email poll endpoint, got %s", r.Method)
			}
			capturedSyncSubject = strings.TrimSpace(r.URL.Query().Get("subject"))
			_, _ = w.Write([]byte(`{
				"messages": [
					{
						"id": "mail-1",
						"author": "employee@example.com",
						"subject": "Account blocked: CASE-EMAIL-COMMS-0001",
						"content": "Received, thanks",
						"timestamp": "2026-02-27T10:00:00Z"
					},
					{
						"id": "mail-2",
						"author": "employee@example.com",
						"subject": "Other thread",
						"content": "Should not be imported",
						"timestamp": "2026-02-27T10:01:00Z"
					}
				],
				"next_cursor": "mail-cursor-2"
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer emailServer.Close()

	connectorID, err := createEmailOutboundConnectorForCommsTest(t, env, emailServer.URL)
	if err != nil {
		t.Fatalf("create email outbound connector: %v", err)
	}
	env.handler.forumProxy = forumproxy.NewService(
		env.catalog,
		repository.NewForumExternalBindingRepository(env.pool),
		repository.NewConnectorRecipientAliasRepository(env.pool),
		outbound.NewService(config.OutboundConnectorsConfig{}),
	)

	threadID := createCaseCommunicationThreadForEmailTest(t, env, caseID, connectorID, map[string]any{
		"email":         "employee@example.com",
		"name":          "Employee",
		"manager_email": "manager@example.com",
	})

	template, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID:  &env.tenantID,
		Kind:      "communication_templates",
		OwnerID:   &env.userID,
		CreatedBy: &env.userID,
		Data: map[string]any{
			"name":             "account-block-template",
			"subject_template": "Account blocked: {{vars.case_ref}}",
			"body_template":    "{{vars.action}} for {{participant.email}}",
		},
	})
	if err != nil {
		t.Fatalf("create communication template: %v", err)
	}

	sendCtx, sendRec := env.jsonContext(http.MethodPost, "/api/v1/cases/"+caseID.String()+"/communications/"+threadID.String()+"/messages", map[string]any{
		"connector_id": connectorID.String(),
		"template_id":  template.ID.String(),
		"template_vars": map[string]any{
			"case_ref": "CASE-EMAIL-COMMS-0001",
			"action":   "Account blocked",
		},
		"metadata": map[string]any{
			"action_type": "block_account",
		},
	})
	setPath(
		sendCtx,
		"/api/v1/cases/:caseID/communications/:threadID/messages",
		[]string{"caseID", "threadID"},
		[]string{caseID.String(), threadID.String()},
	)
	setIdentity(sendCtx, env.identity)
	setTenant(sendCtx, env.tenantID)
	sendErr := env.handler.SendCaseCommunicationMessage(sendCtx)
	mustStatusOK(t, sendErr, sendRec, http.StatusOK)

	if capturedSendPayload == nil {
		t.Fatal("expected email send payload to be captured")
	}
	if got := strings.TrimSpace(firstStringFromPayload(capturedSendPayload, "subject")); got != "Account blocked: CASE-EMAIL-COMMS-0001" {
		t.Fatalf("unexpected email subject: %q", got)
	}
	if got := strings.TrimSpace(firstStringFromPayload(capturedSendPayload, "message")); got != "Account blocked for employee@example.com" {
		t.Fatalf("unexpected email message body: %q", got)
	}
	if !containsRecipient(capturedSendPayload["to"], "employee@example.com") {
		t.Fatalf("expected employee recipient in to list, payload=%v", capturedSendPayload["to"])
	}
	if !containsRecipient(capturedSendPayload["cc"], "manager@example.com") {
		t.Fatalf("expected manager recipient in cc list, payload=%v", capturedSendPayload["cc"])
	}

	syncCtx, syncRec := env.jsonContext(http.MethodPost, "/api/v1/cases/"+caseID.String()+"/communications/"+threadID.String()+"/sync", map[string]any{
		"connector_id": connectorID.String(),
		"subject":      "Account blocked: CASE-EMAIL-COMMS-0001",
	})
	setPath(
		syncCtx,
		"/api/v1/cases/:caseID/communications/:threadID/sync",
		[]string{"caseID", "threadID"},
		[]string{caseID.String(), threadID.String()},
	)
	setIdentity(syncCtx, env.identity)
	setTenant(syncCtx, env.tenantID)
	syncErr := env.handler.SyncCaseCommunication(syncCtx)
	mustStatusOK(t, syncErr, syncRec, http.StatusOK)
	syncPayload := decodeBody[map[string]any](t, syncRec)
	createdCount := int(syncPayload["created_count"].(float64))
	if createdCount != 1 {
		t.Fatalf("expected created_count=1 after subject filtering, got %d", createdCount)
	}
	if strings.TrimSpace(fmt.Sprint(syncPayload["synced_at"])) == "" {
		t.Fatal("expected synced_at in sync response")
	}
	if capturedSyncSubject != "Account blocked: CASE-EMAIL-COMMS-0001" {
		t.Fatalf("expected sync subject query filter to be propagated, got %q", capturedSyncSubject)
	}

	getCtx, getRec := env.jsonContext(http.MethodGet, "/api/v1/cases/"+caseID.String()+"/communications/"+threadID.String(), nil)
	setPath(
		getCtx,
		"/api/v1/cases/:caseID/communications/:threadID",
		[]string{"caseID", "threadID"},
		[]string{caseID.String(), threadID.String()},
	)
	setIdentity(getCtx, env.identity)
	setTenant(getCtx, env.tenantID)
	getErr := env.handler.GetCaseCommunication(getCtx)
	mustStatusOK(t, getErr, getRec, http.StatusOK)
	threadPayload := decodeBody[map[string]any](t, getRec)
	if strings.TrimSpace(fmt.Sprint(threadPayload["last_synced_at"])) == "" {
		t.Fatal("expected last_synced_at to be persisted on communication thread")
	}
	if got := int(threadPayload["last_sync_created_count"].(float64)); got != 1 {
		t.Fatalf("expected last_sync_created_count=1, got %d", got)
	}
	messages, _ := threadPayload["messages"].([]any)
	if len(messages) == 0 {
		t.Fatal("expected communication messages in thread")
	}
	containsSyncedMessage := false
	for _, raw := range messages {
		message, _ := raw.(map[string]any)
		content := strings.TrimSpace(fmt.Sprint(message["content"]))
		if content == "Received, thanks" {
			containsSyncedMessage = true
		}
		if content == "Should not be imported" {
			t.Fatalf("unexpected message imported despite subject filter: %q", content)
		}
	}
	if !containsSyncedMessage {
		t.Fatal("expected synced subject-matched email message in thread")
	}
}

func TestListCommunicationConnectorsIncludesSupportedChannels(t *testing.T) {
	env := newAPITestEnv(t)

	_, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID:  &env.tenantID,
		Kind:      "outbound_connectors",
		OwnerID:   &env.userID,
		CreatedBy: &env.userID,
		Data: map[string]any{
			"name":      "email connector",
			"channel":   "email",
			"direction": "outbound",
			"enabled":   true,
			"config": map[string]any{
				"send_url": "http://localhost:8181/email/send",
			},
		},
	})
	if err != nil {
		t.Fatalf("create email connector: %v", err)
	}
	_, err = env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID:  &env.tenantID,
		Kind:      "outbound_connectors",
		OwnerID:   &env.userID,
		CreatedBy: &env.userID,
		Data: map[string]any{
			"name":      "time connector",
			"channel":   "time",
			"direction": "outbound",
			"enabled":   true,
		},
	})
	if err != nil {
		t.Fatalf("create time connector: %v", err)
	}
	_, err = env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID:  &env.tenantID,
		Kind:      "outbound_connectors",
		OwnerID:   &env.userID,
		CreatedBy: &env.userID,
		Data: map[string]any{
			"name":      "mock connector",
			"channel":   "mock",
			"direction": "outbound",
			"enabled":   true,
		},
	})
	if err != nil {
		t.Fatalf("create mock connector: %v", err)
	}

	c, rec := env.jsonContext(http.MethodGet, "/api/v1/communications/connectors", nil)
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)
	handlerErr := env.handler.ListCommunicationConnectors(c)
	mustStatusOK(t, handlerErr, rec, http.StatusOK)

	payload := decodeBody[[]map[string]any](t, rec)
	channels := map[string]bool{}
	for _, item := range payload {
		channel := strings.ToLower(strings.TrimSpace(fmt.Sprint(item["channel"])))
		channels[channel] = true
	}
	if !channels["email"] {
		t.Fatalf("expected email connector in communications list, got channels=%v", channels)
	}
	if !channels["time"] {
		t.Fatalf("expected time connector in communications list, got channels=%v", channels)
	}
	if channels["mock"] {
		t.Fatalf("unexpected unsupported mock connector in communications list, channels=%v", channels)
	}
}

func createEmailOutboundConnectorForCommsTest(t *testing.T, env *apiTestEnv, baseURL string) (uuid.UUID, error) {
	t.Helper()
	connector, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID:  &env.tenantID,
		Kind:      "outbound_connectors",
		OwnerID:   &env.userID,
		CreatedBy: &env.userID,
		Data: map[string]any{
			"name":      "email-comms",
			"channel":   "email",
			"direction": "outbound",
			"enabled":   true,
			"config": map[string]any{
				"send_url": baseURL + "/mail/send",
				"poll_url": baseURL + "/mail/poll",
			},
		},
	})
	if err != nil {
		return uuid.Nil, err
	}
	return connector.ID, nil
}

func createCaseCommunicationThreadForEmailTest(t *testing.T, env *apiTestEnv, caseID, connectorID uuid.UUID, participant map[string]any) uuid.UUID {
	t.Helper()
	c, rec := env.jsonContext(http.MethodPost, "/api/v1/cases/"+caseID.String()+"/communications", map[string]any{
		"title":        "Email communication",
		"status":       "open",
		"connector_id": connectorID.String(),
		"channel":      "email",
		"participant":  participant,
		"subject":      "Account blocked: CASE-EMAIL-COMMS-0001",
	})
	setPath(c, "/api/v1/cases/:caseID/communications", []string{"caseID"}, []string{caseID.String()})
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)
	err := env.handler.CreateCaseCommunication(c)
	mustStatusOK(t, err, rec, http.StatusCreated)

	payload := decodeBody[map[string]any](t, rec)
	threadIDRaw := strings.TrimSpace(fmt.Sprint(payload["id"]))
	threadID, parseErr := uuid.Parse(threadIDRaw)
	if parseErr != nil {
		t.Fatalf("parse communication thread id: %v", parseErr)
	}
	return threadID
}

func containsRecipient(raw any, candidate string) bool {
	needle := strings.ToLower(strings.TrimSpace(candidate))
	switch typed := raw.(type) {
	case []any:
		for _, value := range typed {
			if strings.ToLower(strings.TrimSpace(fmt.Sprint(value))) == needle {
				return true
			}
		}
	case []string:
		for _, value := range typed {
			if strings.ToLower(strings.TrimSpace(value)) == needle {
				return true
			}
		}
	case string:
		for _, token := range strings.FieldsFunc(typed, func(r rune) bool { return r == ',' || r == ';' }) {
			if strings.ToLower(strings.TrimSpace(token)) == needle {
				return true
			}
		}
	}
	return false
}

func firstStringFromPayload(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(payload[key]))
}
