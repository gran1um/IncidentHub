package api

import (
	"context"
	"strings"
	"testing"
	"time"

	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/metrics"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"

	"github.com/google/uuid"
)

type serviceAlertCatalogStub struct {
	rules   []models.CatalogItem
	created []repository.CatalogCreateParams
}

func (s *serviceAlertCatalogStub) ListAcrossTenants(_ context.Context, _ string, _ int) ([]models.CatalogItem, error) {
	out := make([]models.CatalogItem, len(s.rules))
	copy(out, s.rules)
	return out, nil
}

func (s *serviceAlertCatalogStub) Create(_ context.Context, p repository.CatalogCreateParams) (*models.CatalogItem, error) {
	s.created = append(s.created, p)
	item := &models.CatalogItem{
		ID:       uuid.New(),
		TenantID: p.TenantID,
		Kind:     p.Kind,
		OwnerID:  p.OwnerID,
		Data:     p.Data,
	}
	return item, nil
}

type serviceAlertRecipientsStub struct {
	items map[uuid.UUID][]models.UserNotificationSettings
}

func (s *serviceAlertRecipientsStub) ListDeliveryEnabledByTenant(_ context.Context, tenantID uuid.UUID, _ int) ([]models.UserNotificationSettings, error) {
	list := s.items[tenantID]
	out := make([]models.UserNotificationSettings, len(list))
	copy(out, list)
	return out, nil
}

type serviceAlertCaseMetricsStub struct {
	totalByTenant             map[uuid.UUID]int
	inWorkByTenant            map[uuid.UUID]int
	openBySeverityByTenant    map[uuid.UUID]int
	openCreatedBeforeByTenant map[uuid.UUID]int
	openInactiveByTenant      map[uuid.UUID]int
}

func (s *serviceAlertCaseMetricsStub) CountByTenant(_ context.Context, tenantID uuid.UUID) (int, error) {
	return s.totalByTenant[tenantID], nil
}

func (s *serviceAlertCaseMetricsStub) CountInWorkByTenant(_ context.Context, tenantID uuid.UUID, _ []string) (int, error) {
	return s.inWorkByTenant[tenantID], nil
}

func (s *serviceAlertCaseMetricsStub) CountOpenBySeverity(_ context.Context, tenantID uuid.UUID, _ []string, _ []string) (int, error) {
	return s.openBySeverityByTenant[tenantID], nil
}

func (s *serviceAlertCaseMetricsStub) CountOpenCreatedBefore(_ context.Context, tenantID uuid.UUID, _ time.Time, _ []string) (int, error) {
	return s.openCreatedBeforeByTenant[tenantID], nil
}

func (s *serviceAlertCaseMetricsStub) CountOpenInactiveSince(_ context.Context, tenantID uuid.UUID, _ time.Time, _ []string) (int, error) {
	return s.openInactiveByTenant[tenantID], nil
}

type serviceAlertCaseEventsStub struct {
	countByTenant map[uuid.UUID]int
}

func (s *serviceAlertCaseEventsStub) CountByTenantAndTypesSince(_ context.Context, tenantID uuid.UUID, _ []string, _ time.Time) (int, error) {
	return s.countByTenant[tenantID], nil
}

type serviceAlertQueueStub struct {
	events []NotificationDeliveryEvent
}

func (s *serviceAlertQueueStub) Enabled() bool {
	return true
}

func (s *serviceAlertQueueStub) Enqueue(_ context.Context, event NotificationDeliveryEvent) error {
	s.events = append(s.events, event)
	return nil
}

func TestServiceAlertEvaluatorLowRPSTriggersNotification(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()

	collector := metrics.New("incidenthub_test")
	collector.ObserveModuleOperation("api", "request", "ok", 20*time.Millisecond)

	catalog := &serviceAlertCatalogStub{
		rules: []models.CatalogItem{
			{
				ID:       uuid.New(),
				TenantID: &tenantID,
				Kind:     serviceAlertRuleCatalogKind,
				Data: map[string]any{
					"name":             "API Throughput",
					"metric_type":      "low_rps",
					"module":           "api",
					"min_rps":          5,
					"window_seconds":   600,
					"cooldown_seconds": 120,
					"enabled":          true,
				},
			},
		},
	}
	recipients := &serviceAlertRecipientsStub{items: map[uuid.UUID][]models.UserNotificationSettings{
		tenantID: {
			{TenantID: tenantID, UserID: userID, DeliveryEnabled: true, DeliveryChannel: "telegram"},
		},
	}}
	queue := &serviceAlertQueueStub{}

	evaluator := NewServiceAlertEvaluator(config.ServiceAlertingConfig{Enabled: true}, ServiceAlertEvaluatorDependencies{
		Catalog:              catalog,
		NotificationSettings: recipients,
		Metrics:              collector,
		NotificationQueue:    queue,
	})
	if evaluator == nil {
		t.Fatal("expected evaluator instance")
	}

	if err := evaluator.evaluateOnce(context.Background()); err != nil {
		t.Fatalf("evaluateOnce failed: %v", err)
	}
	if len(catalog.created) != 1 {
		t.Fatalf("expected one created notification, got %d", len(catalog.created))
	}
	created := catalog.created[0]
	if created.Kind != "notifications" {
		t.Fatalf("expected notifications kind, got %q", created.Kind)
	}
	if created.OwnerID == nil || *created.OwnerID != userID {
		t.Fatalf("expected notification owner user %s, got %#v", userID, created.OwnerID)
	}
	if len(queue.events) != 1 {
		t.Fatalf("expected one enqueued delivery event, got %d", len(queue.events))
	}
	if !strings.Contains(strings.ToLower(queue.events[0].Message), "low throughput") {
		t.Fatalf("expected low throughput message, got %q", queue.events[0].Message)
	}
}

func TestServiceAlertEvaluatorRespectsCooldown(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	now := time.Now().UTC()

	collector := metrics.New("incidenthub_test")
	collector.ObserveModuleOperation("api", "request", "ok", 25*time.Millisecond)

	catalog := &serviceAlertCatalogStub{
		rules: []models.CatalogItem{
			{
				ID:       uuid.New(),
				TenantID: &tenantID,
				Kind:     serviceAlertRuleCatalogKind,
				Data: map[string]any{
					"name":             "API Throughput",
					"metric_type":      "low_rps",
					"module":           "api",
					"min_rps":          100,
					"window_seconds":   3600,
					"cooldown_seconds": 300,
					"enabled":          true,
				},
			},
		},
	}
	recipients := &serviceAlertRecipientsStub{items: map[uuid.UUID][]models.UserNotificationSettings{
		tenantID: {
			{TenantID: tenantID, UserID: userID, DeliveryEnabled: true, DeliveryChannel: "telegram"},
		},
	}}

	evaluator := NewServiceAlertEvaluator(config.ServiceAlertingConfig{Enabled: true}, ServiceAlertEvaluatorDependencies{
		Catalog:              catalog,
		NotificationSettings: recipients,
		Metrics:              collector,
		NotificationQueue:    &serviceAlertQueueStub{},
	})
	if evaluator == nil {
		t.Fatal("expected evaluator instance")
	}
	evaluator.now = func() time.Time { return now }

	if err := evaluator.evaluateOnce(context.Background()); err != nil {
		t.Fatalf("first evaluateOnce failed: %v", err)
	}
	if err := evaluator.evaluateOnce(context.Background()); err != nil {
		t.Fatalf("second evaluateOnce failed: %v", err)
	}
	if len(catalog.created) != 1 {
		t.Fatalf("expected cooldown to suppress second alert, created=%d", len(catalog.created))
	}

	now = now.Add(6 * time.Minute)
	if err := evaluator.evaluateOnce(context.Background()); err != nil {
		t.Fatalf("third evaluateOnce failed: %v", err)
	}
	if len(catalog.created) != 2 {
		t.Fatalf("expected alert after cooldown expiry, created=%d", len(catalog.created))
	}
}

func TestServiceAlertEvaluatorHighLatencyRule(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()

	collector := metrics.New("incidenthub_test")
	collector.ObserveModuleOperation("search", "query", "ok", 800*time.Millisecond)
	collector.ObserveModuleOperation("search", "query", "ok", 600*time.Millisecond)

	catalog := &serviceAlertCatalogStub{
		rules: []models.CatalogItem{
			{
				ID:       uuid.New(),
				TenantID: &tenantID,
				Kind:     serviceAlertRuleCatalogKind,
				Data: map[string]any{
					"name":             "Search Latency",
					"metric_type":      "high_latency",
					"module":           "search",
					"max_latency_ms":   200,
					"window_seconds":   300,
					"cooldown_seconds": 60,
					"enabled":          true,
				},
			},
		},
	}
	recipients := &serviceAlertRecipientsStub{items: map[uuid.UUID][]models.UserNotificationSettings{
		tenantID: {
			{TenantID: tenantID, UserID: userID, DeliveryEnabled: true, DeliveryChannel: "telegram"},
		},
	}}
	queue := &serviceAlertQueueStub{}

	evaluator := NewServiceAlertEvaluator(config.ServiceAlertingConfig{Enabled: true}, ServiceAlertEvaluatorDependencies{
		Catalog:              catalog,
		NotificationSettings: recipients,
		Metrics:              collector,
		NotificationQueue:    queue,
	})
	if evaluator == nil {
		t.Fatal("expected evaluator instance")
	}
	if err := evaluator.evaluateOnce(context.Background()); err != nil {
		t.Fatalf("evaluateOnce failed: %v", err)
	}
	if len(queue.events) != 1 {
		t.Fatalf("expected one enqueued event, got %d", len(queue.events))
	}
	if !strings.Contains(strings.ToLower(queue.events[0].Message), "latency is high") {
		t.Fatalf("expected high latency message, got %q", queue.events[0].Message)
	}
}

func TestServiceAlertEvaluatorCaseTriggers(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()

	catalog := &serviceAlertCatalogStub{
		rules: []models.CatalogItem{
			{
				ID:       uuid.New(),
				TenantID: &tenantID,
				Kind:     serviceAlertRuleCatalogKind,
				Data: map[string]any{
					"name":            "SLA Breach",
					"metric_type":     "sla",
					"sla_seconds":     600,
					"cases_threshold": 1,
					"enabled":         true,
				},
			},
			{
				ID:       uuid.New(),
				TenantID: &tenantID,
				Kind:     serviceAlertRuleCatalogKind,
				Data: map[string]any{
					"name":            "In-work threshold",
					"metric_type":     "threshold",
					"threshold_mode":  "in_work",
					"cases_threshold": 10,
					"enabled":         true,
				},
			},
			{
				ID:       uuid.New(),
				TenantID: &tenantID,
				Kind:     serviceAlertRuleCatalogKind,
				Data: map[string]any{
					"name":               "Critical queue",
					"metric_type":        "criticality",
					"criticality_levels": []any{"critical", "high"},
					"cases_threshold":    2,
					"enabled":            true,
				},
			},
			{
				ID:       uuid.New(),
				TenantID: &tenantID,
				Kind:     serviceAlertRuleCatalogKind,
				Data: map[string]any{
					"name":                 "Escalation spikes",
					"metric_type":          "escalation",
					"window_seconds":       900,
					"escalation_threshold": 1,
					"enabled":              true,
				},
			},
			{
				ID:       uuid.New(),
				TenantID: &tenantID,
				Kind:     serviceAlertRuleCatalogKind,
				Data: map[string]any{
					"name":                    "Case ping",
					"metric_type":             "ping",
					"ping_inactivity_seconds": 1800,
					"ping_threshold":          2,
					"enabled":                 true,
				},
			},
		},
	}
	recipients := &serviceAlertRecipientsStub{items: map[uuid.UUID][]models.UserNotificationSettings{
		tenantID: {
			{TenantID: tenantID, UserID: userID, DeliveryEnabled: true, DeliveryChannel: "telegram"},
		},
	}}
	queue := &serviceAlertQueueStub{}
	caseMetrics := &serviceAlertCaseMetricsStub{
		totalByTenant:             map[uuid.UUID]int{tenantID: 12},
		inWorkByTenant:            map[uuid.UUID]int{tenantID: 12},
		openBySeverityByTenant:    map[uuid.UUID]int{tenantID: 4},
		openCreatedBeforeByTenant: map[uuid.UUID]int{tenantID: 3},
		openInactiveByTenant:      map[uuid.UUID]int{tenantID: 5},
	}
	caseEvents := &serviceAlertCaseEventsStub{
		countByTenant: map[uuid.UUID]int{tenantID: 2},
	}

	evaluator := NewServiceAlertEvaluator(config.ServiceAlertingConfig{Enabled: true}, ServiceAlertEvaluatorDependencies{
		Catalog:              catalog,
		NotificationSettings: recipients,
		Cases:                caseMetrics,
		CaseEvents:           caseEvents,
		Metrics:              metrics.New("incidenthub_test"),
		NotificationQueue:    queue,
	})
	if evaluator == nil {
		t.Fatal("expected evaluator instance")
	}

	if err := evaluator.evaluateOnce(context.Background()); err != nil {
		t.Fatalf("evaluateOnce failed: %v", err)
	}
	if len(catalog.created) != 5 {
		t.Fatalf("expected 5 created notifications, got %d", len(catalog.created))
	}
	if len(queue.events) != 5 {
		t.Fatalf("expected 5 queued events, got %d", len(queue.events))
	}

	titles := make([]string, 0, len(queue.events))
	for _, event := range queue.events {
		titles = append(titles, strings.ToLower(event.Title))
	}
	mustContain := []string{"sla", "threshold", "criticality", "escalation", "ping"}
	for _, token := range mustContain {
		found := false
		for _, title := range titles {
			if strings.Contains(title, token) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected title containing %q, got %v", token, titles)
		}
	}
}

func TestServiceAlertEvaluatorSkipsCaseRuleWithoutRepositories(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()

	catalog := &serviceAlertCatalogStub{
		rules: []models.CatalogItem{
			{
				ID:       uuid.New(),
				TenantID: &tenantID,
				Kind:     serviceAlertRuleCatalogKind,
				Data: map[string]any{
					"name":            "SLA Breach",
					"metric_type":     "sla",
					"sla_seconds":     600,
					"cases_threshold": 1,
					"enabled":         true,
				},
			},
		},
	}
	recipients := &serviceAlertRecipientsStub{items: map[uuid.UUID][]models.UserNotificationSettings{
		tenantID: {
			{TenantID: tenantID, UserID: userID, DeliveryEnabled: true, DeliveryChannel: "telegram"},
		},
	}}
	queue := &serviceAlertQueueStub{}

	evaluator := NewServiceAlertEvaluator(config.ServiceAlertingConfig{Enabled: true}, ServiceAlertEvaluatorDependencies{
		Catalog:              catalog,
		NotificationSettings: recipients,
		Metrics:              metrics.New("incidenthub_test"),
		NotificationQueue:    queue,
	})
	if evaluator == nil {
		t.Fatal("expected evaluator instance")
	}

	if err := evaluator.evaluateOnce(context.Background()); err != nil {
		t.Fatalf("evaluateOnce failed: %v", err)
	}
	if len(catalog.created) != 0 {
		t.Fatalf("expected no notifications when repositories are missing, got %d", len(catalog.created))
	}
	if len(queue.events) != 0 {
		t.Fatalf("expected no queued events when repositories are missing, got %d", len(queue.events))
	}
}
