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
	"github.com/labstack/echo/v5"
)

func TestSendCaseCommunicationMessageTelegramUsesParticipantChatID(t *testing.T) {
	env := newAPITestEnv(t)

	caseID := createCaseForTelegramCommunicationTest(t, env, "CASE-TG-COMMS-0001")

	var lastChatID string
	var lastText string
	telegramServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST request to telegram, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/sendMessage") {
			t.Fatalf("unexpected telegram endpoint path: %s", r.URL.Path)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode telegram payload: %v", err)
		}
		lastChatID = strings.TrimSpace(fmt.Sprint(payload["chat_id"]))
		lastText = strings.TrimSpace(fmt.Sprint(payload["text"]))
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer telegramServer.Close()

	connectorID := createTelegramOutboundConnectorForCommsTest(t, env, telegramServer.URL)
	env.handler.forumProxy = forumproxy.NewService(
		env.catalog,
		repository.NewForumExternalBindingRepository(env.pool),
		repository.NewConnectorRecipientAliasRepository(env.pool),
		outbound.NewService(config.OutboundConnectorsConfig{}),
	)

	threadID := createCaseCommunicationThreadForTelegramTest(t, env, caseID, connectorID, map[string]any{
		"chat_id": "355321741",
		"name":    "Target User",
	})

	c, rec := env.jsonContext(http.MethodPost, "/api/v1/cases/"+caseID.String()+"/communications/"+threadID.String()+"/messages", map[string]any{
		"content": "Please confirm mitigation details",
		"author":  "SOC Analyst",
	})
	setPath(
		c,
		"/api/v1/cases/:caseID/communications/:threadID/messages",
		[]string{"caseID", "threadID"},
		[]string{caseID.String(), threadID.String()},
	)
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)
	err := env.handler.SendCaseCommunicationMessage(c)
	mustStatusOK(t, err, rec, http.StatusOK)

	if lastChatID != "355321741" {
		t.Fatalf("expected telegram chat_id=355321741, got %q", lastChatID)
	}
	if lastText != "Please confirm mitigation details" {
		t.Fatalf("unexpected telegram text: %q", lastText)
	}

	payload := decodeBody[map[string]any](t, rec)
	if got := strings.TrimSpace(fmt.Sprint(payload["thread_id"])); got != threadID.String() {
		t.Fatalf("unexpected thread_id in response: %q", got)
	}
	if got := strings.TrimSpace(fmt.Sprint(payload["connector_id"])); got != connectorID.String() {
		t.Fatalf("unexpected connector_id in response: %q", got)
	}

	userMessage := asMap(payload["user_message"])
	if status := strings.TrimSpace(fmt.Sprint(userMessage["delivery_status"])); status != "sent" {
		t.Fatalf("expected user_message delivery_status=sent, got %q", status)
	}
	metadata := asMap(userMessage["metadata"])
	if chatID := strings.TrimSpace(fmt.Sprint(metadata["chat_id"])); chatID != "355321741" {
		t.Fatalf("expected user_message metadata.chat_id=355321741, got %q", chatID)
	}
}

func TestSendCaseCommunicationMessageTelegramWithParticipantIDOnlyReturnsBadGateway(t *testing.T) {
	env := newAPITestEnv(t)

	caseID := createCaseForTelegramCommunicationTest(t, env, "CASE-TG-COMMS-0002")

	telegramServer := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("telegram endpoint should not be called when recipient is unresolved")
	}))
	defer telegramServer.Close()

	connectorID := createTelegramOutboundConnectorForCommsTest(t, env, telegramServer.URL)
	env.handler.forumProxy = forumproxy.NewService(
		env.catalog,
		repository.NewForumExternalBindingRepository(env.pool),
		repository.NewConnectorRecipientAliasRepository(env.pool),
		outbound.NewService(config.OutboundConnectorsConfig{}),
	)

	threadID := createCaseCommunicationThreadForTelegramTest(t, env, caseID, connectorID, map[string]any{
		"id":   "355321741",
		"name": "Target User",
	})

	c, _ := env.jsonContext(http.MethodPost, "/api/v1/cases/"+caseID.String()+"/communications/"+threadID.String()+"/messages", map[string]any{
		"content": "Ping",
	})
	setPath(
		c,
		"/api/v1/cases/:caseID/communications/:threadID/messages",
		[]string{"caseID", "threadID"},
		[]string{caseID.String(), threadID.String()},
	)
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)

	err := env.handler.SendCaseCommunicationMessage(c)
	if err == nil {
		t.Fatal("expected send error when only participant.id is provided")
	}
	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected *echo.HTTPError, got %T", err)
	}
	if httpErr.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d", httpErr.Code)
	}
}

func createCaseForTelegramCommunicationTest(t *testing.T, env *apiTestEnv, caseNumber string) uuid.UUID {
	t.Helper()
	createdCase, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        caseNumber,
		Title:             "Case communication telegram test",
		Description:       "Validate case communications telegram delivery",
		Source:            "manual",
		IncidentType:      "communication",
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
	return createdCase.ID
}

func createTelegramOutboundConnectorForCommsTest(t *testing.T, env *apiTestEnv, apiBaseURL string) uuid.UUID {
	t.Helper()
	connector, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID: &env.tenantID,
		Kind:     "outbound_connectors",
		OwnerID:  &env.userID,
		Data: map[string]any{
			"name":      "telegram-comms",
			"channel":   "telegram",
			"direction": "outbound",
			"enabled":   true,
			"config": map[string]any{
				"botToken":   "token",
				"apiBaseURL": apiBaseURL,
			},
		},
		CreatedBy: &env.userID,
	})
	if err != nil {
		t.Fatalf("create telegram outbound connector: %v", err)
	}
	return connector.ID
}

func createCaseCommunicationThreadForTelegramTest(t *testing.T, env *apiTestEnv, caseID, connectorID uuid.UUID, participant map[string]any) uuid.UUID {
	t.Helper()
	c, rec := env.jsonContext(http.MethodPost, "/api/v1/cases/"+caseID.String()+"/communications", map[string]any{
		"title":        "External communication",
		"status":       "open",
		"connector_id": connectorID.String(),
		"channel":      "telegram",
		"participant":  participant,
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

func asMap(value any) map[string]any {
	typed, _ := value.(map[string]any)
	if typed == nil {
		return map[string]any{}
	}
	return typed
}
