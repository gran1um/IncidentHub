package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"incidenthub/backend/internal/connectors/outbound"
	"incidenthub/backend/internal/repository"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/google/uuid"
)

func TestKafkaNotificationDeliveryProcessEmailChannel(t *testing.T) {
	env := newAPITestEnv(t)
	ctx := context.Background()

	requests := 0
	lastSubject := ""
	lastRecipient := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		lastSubject = strings.TrimSpace(stringFromMap(payload, "subject"))
		toRaw, _ := payload["to"].([]any)
		if len(toRaw) > 0 {
			lastRecipient = strings.TrimSpace(fmt.Sprint(toRaw[0]))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"reply": "accepted"})
	}))
	defer server.Close()

	_, err := env.catalog.Create(ctx, repository.CatalogCreateParams{
		TenantID: &env.tenantID,
		Kind:     "outbound_connectors",
		OwnerID:  &env.userID,
		Data: map[string]any{
			"name":      "Email Delivery",
			"channel":   "email",
			"direction": "outbound",
			"enabled":   true,
			"config": map[string]any{
				"send_url": server.URL,
			},
		},
		CreatedBy: &env.userID,
	})
	if err != nil {
		t.Fatalf("create outbound email connector: %v", err)
	}

	_, err = env.notificationSettings.Upsert(ctx, repository.UpsertUserNotificationSettingsParams{
		TenantID:          env.tenantID,
		UserID:            env.userID,
		DeliveryEnabled:   true,
		DeliveryChannel:   "email",
		NotificationEmail: "analyst@example.com",
		UpdatedBy:         &env.userID,
	})
	if err != nil {
		t.Fatalf("upsert notification settings: %v", err)
	}

	queue := &KafkaNotificationDelivery{
		settings: env.notificationSettings,
		bots:     env.notificationBots,
		catalog:  env.catalog,
		outbound: outbound.NewService(env.cfg.Outbound),
	}
	if err := queue.processNotificationDelivery(NotificationDeliveryEvent{
		EventID:        uuid.NewString(),
		NotificationID: uuid.NewString(),
		TenantID:       env.tenantID.String(),
		UserID:         env.userID.String(),
		Title:          "Rule breached",
		Message:        "SLA threshold exceeded",
		Type:           "warning",
	}); err != nil {
		t.Fatalf("processNotificationDelivery(email): %v", err)
	}

	if requests != 1 {
		t.Fatalf("expected one email request, got %d", requests)
	}
	if lastRecipient != "analyst@example.com" {
		t.Fatalf("unexpected email recipient: %q", lastRecipient)
	}
	if !strings.Contains(lastSubject, "Rule breached") {
		t.Fatalf("expected subject to include title, got %q", lastSubject)
	}
}

func TestKafkaNotificationDeliveryProcessTimeChannel(t *testing.T) {
	env := newAPITestEnv(t)
	ctx := context.Background()

	requests := 0
	lastRecipient := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		lastRecipient = strings.TrimSpace(stringFromMap(payload, "recipient", "conversation_id"))
		_ = json.NewEncoder(w).Encode(map[string]any{"reply": "ok"})
	}))
	defer server.Close()

	_, err := env.catalog.Create(ctx, repository.CatalogCreateParams{
		TenantID: &env.tenantID,
		Kind:     "outbound_connectors",
		OwnerID:  &env.userID,
		Data: map[string]any{
			"name":      "Time Delivery",
			"channel":   "time",
			"direction": "outbound",
			"enabled":   true,
			"config": map[string]any{
				"send_url": server.URL,
			},
		},
		CreatedBy: &env.userID,
	})
	if err != nil {
		t.Fatalf("create outbound time connector: %v", err)
	}

	_, err = env.notificationSettings.Upsert(ctx, repository.UpsertUserNotificationSettingsParams{
		TenantID:        env.tenantID,
		UserID:          env.userID,
		DeliveryEnabled: true,
		DeliveryChannel: "time",
		TimeRecipient:   "soc-time-room",
		UpdatedBy:       &env.userID,
	})
	if err != nil {
		t.Fatalf("upsert notification settings: %v", err)
	}

	queue := &KafkaNotificationDelivery{
		settings: env.notificationSettings,
		bots:     env.notificationBots,
		catalog:  env.catalog,
		outbound: outbound.NewService(env.cfg.Outbound),
	}
	if err := queue.processNotificationDelivery(NotificationDeliveryEvent{
		EventID:        uuid.NewString(),
		NotificationID: uuid.NewString(),
		TenantID:       env.tenantID.String(),
		UserID:         env.userID.String(),
		Title:          "Escalation",
		Message:        "Case escalated",
		Type:           "critical",
	}); err != nil {
		t.Fatalf("processNotificationDelivery(time): %v", err)
	}

	if requests != 1 {
		t.Fatalf("expected one time request, got %d", requests)
	}
	if lastRecipient != "soc-time-room" {
		t.Fatalf("unexpected time recipient: %q", lastRecipient)
	}
}

func TestIsNotificationsKafkaTopicUnavailable(t *testing.T) {
	if !isNotificationsKafkaTopicUnavailable(kafka.NewError(kafka.ErrUnknownTopicOrPart, "missing topic", false)) {
		t.Fatal("expected unknown topic or partition to be treated as topic unavailable")
	}
	if !isNotificationsKafkaTopicUnavailable(kafka.NewError(kafka.ErrUnknownTopic, "unknown topic", false)) {
		t.Fatal("expected unknown topic to be treated as topic unavailable")
	}
	if isNotificationsKafkaTopicUnavailable(kafka.NewError(kafka.ErrTimedOut, "timeout", true)) {
		t.Fatal("did not expect timeout to be treated as topic unavailable")
	}
}
