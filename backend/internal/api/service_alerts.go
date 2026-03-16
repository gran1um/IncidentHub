package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/logger"
	"incidenthub/backend/internal/metrics"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"
	"incidenthub/backend/internal/tracing"

	"github.com/google/uuid"
)

const serviceAlertRuleCatalogKind = "service_alert_rules"

const (
	serviceAlertMetricLowRPS      = "low_rps"
	serviceAlertMetricHighLatency = "high_latency"
	serviceAlertMetricSLA         = "sla"
	serviceAlertMetricThreshold   = "threshold"
	serviceAlertMetricCriticality = "criticality"
	serviceAlertMetricEscalation  = "escalation"
	serviceAlertMetricPing        = "ping"
)

var (
	errServiceAlertCasesRepositoryRequiredForSLARules  = errors.New("cases repository is required for sla rules")
	errServiceAlertCasesRepositoryRequiredForPingRules = errors.New("cases repository is required for ping rules")
)

type serviceAlertCatalogStore interface {
	ListAcrossTenants(ctx context.Context, kind string, limit int) ([]models.CatalogItem, error)
	Create(ctx context.Context, p repository.CatalogCreateParams) (*models.CatalogItem, error)
}

type serviceAlertRecipientsStore interface {
	ListDeliveryEnabledByTenant(ctx context.Context, tenantID uuid.UUID, limit int) ([]models.UserNotificationSettings, error)
}

type serviceAlertCaseMetricsStore interface {
	CountByTenant(ctx context.Context, tenantID uuid.UUID) (int, error)
	CountInWorkByTenant(ctx context.Context, tenantID uuid.UUID, openStatuses []string) (int, error)
	CountOpenBySeverity(ctx context.Context, tenantID uuid.UUID, severities []string, openStatuses []string) (int, error)
	CountOpenCreatedBefore(ctx context.Context, tenantID uuid.UUID, before time.Time, openStatuses []string) (int, error)
	CountOpenInactiveSince(ctx context.Context, tenantID uuid.UUID, since time.Time, openStatuses []string) (int, error)
}

type serviceAlertCaseEventsStore interface {
	CountByTenantAndTypesSince(ctx context.Context, tenantID uuid.UUID, eventTypes []string, since time.Time) (int, error)
}

type ServiceAlertEvaluatorDependencies struct {
	Catalog              serviceAlertCatalogStore
	NotificationSettings serviceAlertRecipientsStore
	Cases                serviceAlertCaseMetricsStore
	CaseEvents           serviceAlertCaseEventsStore
	Metrics              *metrics.Collector
	NotificationQueue    NotificationDeliveryQueue
}

type ServiceAlertEvaluator struct {
	cfg                  config.ServiceAlertingConfig
	catalog              serviceAlertCatalogStore
	notificationSettings serviceAlertRecipientsStore
	cases                serviceAlertCaseMetricsStore
	caseEvents           serviceAlertCaseEventsStore
	metrics              *metrics.Collector
	notificationQueue    NotificationDeliveryQueue

	startOnce sync.Once
	now       func() time.Time

	stateMu      sync.Mutex
	lastAlertMap map[string]time.Time
}

type serviceAlertRule struct {
	ID                   uuid.UUID
	TenantID             uuid.UUID
	Name                 string
	Description          string
	Enabled              bool
	MetricType           string
	Module               string
	MinRPS               float64
	MaxLatencyMs         float64
	Window               time.Duration
	Cooldown             time.Duration
	Severity             string
	WindowSeconds        int64
	CooldownSeconds      int64
	OpenStatuses         []string
	SLASeconds           int64
	CaseThreshold        int
	ThresholdMode        string
	CriticalSeverities   []string
	EscalationEventTypes []string
	PingInactivitySec    int64
}

type serviceAlertEvaluationResult struct {
	Breached bool
	Title    string
	Message  string
	Metadata map[string]any
}

func NewServiceAlertEvaluator(cfg config.ServiceAlertingConfig, deps ServiceAlertEvaluatorDependencies) *ServiceAlertEvaluator {
	if !cfg.Enabled {
		return nil
	}
	if deps.Catalog == nil || deps.NotificationSettings == nil || deps.Metrics == nil {
		return nil
	}
	if cfg.EvaluationInterval <= 0 {
		cfg.EvaluationInterval = 30 * time.Second
	}
	if cfg.DefaultWindow <= 0 {
		cfg.DefaultWindow = 5 * time.Minute
	}
	if cfg.DefaultCooldown <= 0 {
		cfg.DefaultCooldown = 15 * time.Minute
	}
	if cfg.RulesLimit <= 0 {
		cfg.RulesLimit = 500
	}
	if cfg.RecipientsLimit <= 0 {
		cfg.RecipientsLimit = 300
	}
	return &ServiceAlertEvaluator{
		cfg:                  cfg,
		catalog:              deps.Catalog,
		notificationSettings: deps.NotificationSettings,
		cases:                deps.Cases,
		caseEvents:           deps.CaseEvents,
		metrics:              deps.Metrics,
		notificationQueue:    deps.NotificationQueue,
		now:                  func() time.Time { return time.Now().UTC() },
		lastAlertMap:         map[string]time.Time{},
	}
}

func (e *ServiceAlertEvaluator) Start(ctx context.Context) {
	if e == nil {
		return
	}
	e.startOnce.Do(func() {
		go e.runLoop(ctx)
	})
}

func (e *ServiceAlertEvaluator) runLoop(ctx context.Context) {
	e.runEvaluation(ctx)
	ticker := time.NewTicker(e.cfg.EvaluationInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.runEvaluation(ctx)
		}
	}
}

func (e *ServiceAlertEvaluator) runEvaluation(parent context.Context) {
	if e == nil {
		return
	}
	timeout := e.cfg.EvaluationInterval
	if timeout < 10*time.Second {
		timeout = 10 * time.Second
	}
	if timeout > 2*time.Minute {
		timeout = 2 * time.Minute
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	if err := e.evaluateOnce(ctx); err != nil {
		logger.Warnf("service alerts evaluation failed: %v", err)
	}
}

func (e *ServiceAlertEvaluator) evaluateOnce(ctx context.Context) error {
	ctx, span, startedAt := tracing.StartModuleOperation(ctx, "service_alerts", "evaluate")
	var runErr error
	defer func() {
		tracing.FinishModuleOperation(span, startedAt, "service_alerts", "evaluate", runErr)
	}()

	rules, err := e.catalog.ListAcrossTenants(ctx, serviceAlertRuleCatalogKind, e.cfg.RulesLimit)
	if err != nil {
		runErr = fmt.Errorf("list service alert rules: %w", err)
		return runErr
	}

	now := e.now().UTC()
	for _, item := range rules {
		rule, parseErr := e.parseRule(item)
		if parseErr != nil {
			logger.Warnf("skip invalid service alert rule %s: %v", item.ID.String(), parseErr)
			continue
		}
		if !rule.Enabled {
			continue
		}

		evaluation, evalErr := e.evaluateRule(ctx, rule, now)
		if evalErr != nil {
			logger.Warnf("service alert rule evaluation failed (rule=%s): %v", rule.ID.String(), evalErr)
			continue
		}
		if !evaluation.Breached {
			continue
		}
		if e.cooldownActive(rule, now) {
			continue
		}

		recipients, recipientsErr := e.notificationSettings.ListDeliveryEnabledByTenant(ctx, rule.TenantID, e.cfg.RecipientsLimit)
		if recipientsErr != nil {
			logger.Warnf("service alert %s recipients lookup failed: %v", rule.ID.String(), recipientsErr)
			continue
		}
		if len(recipients) == 0 {
			continue
		}

		sentAny := false
		for _, recipient := range recipients {
			if err := e.createServiceAlertNotification(ctx, rule, recipient.UserID, evaluation, now); err != nil {
				logger.Warnf("service alert notification create failed (rule=%s user=%s): %v", rule.ID.String(), recipient.UserID.String(), err)
				continue
			}
			sentAny = true
		}
		if sentAny {
			e.markAlertSent(rule, now)
		}
	}

	return nil
}

func (e *ServiceAlertEvaluator) parseRule(item models.CatalogItem) (serviceAlertRule, error) {
	if item.TenantID == nil {
		return serviceAlertRule{}, fmt.Errorf("rule is missing tenant_id")
	}
	data := item.Data
	if data == nil {
		data = map[string]any{}
	}

	rawMetricType := strings.ToLower(strings.TrimSpace(stringFromMap(data, "metric_type", "metricType", "type")))
	metricType := normalizeServiceAlertMetricType(rawMetricType)
	if metricType == "" {
		return serviceAlertRule{}, fmt.Errorf("metric_type is required")
	}

	rule := serviceAlertRule{
		ID:          item.ID,
		TenantID:    *item.TenantID,
		Name:        stringFromMap(data, "name", "title"),
		Description: stringFromMap(data, "description", "message"),
		Enabled:     true,
		MetricType:  metricType,
		Module:      strings.ToLower(strings.TrimSpace(stringFromMap(data, "module", "module_name", "moduleName"))),
		Severity:    normalizeServiceAlertSeverity(stringFromMap(data, "severity", "level")),
		OpenStatuses: normalizeUniqueStrings(
			stringSliceFromMap(data, "open_statuses", "openStatuses", "statuses"),
		),
	}
	if rule.Name == "" {
		rule.Name = "Service alert"
	}
	if enabled, ok := boolFromMap(data, "enabled", "is_enabled", "isEnabled"); ok {
		rule.Enabled = enabled
	}
	if rule.Module == "" {
		rule.Module = "api"
	}

	window := e.cfg.DefaultWindow
	if seconds, ok := intFromMap(data, "window_seconds", "windowSeconds", "evaluation_window_seconds", "evaluationWindowSeconds"); ok && seconds > 0 {
		window = time.Duration(seconds) * time.Second
	}
	rule.Window = window
	rule.WindowSeconds = int64(window / time.Second)
	if rule.WindowSeconds < 1 {
		rule.WindowSeconds = 1
		rule.Window = time.Second
	}

	cooldown := e.cfg.DefaultCooldown
	if seconds, ok := intFromMap(data, "cooldown_seconds", "cooldownSeconds"); ok && seconds > 0 {
		cooldown = time.Duration(seconds) * time.Second
	}
	rule.Cooldown = cooldown
	rule.CooldownSeconds = int64(cooldown / time.Second)
	if rule.CooldownSeconds < 1 {
		rule.CooldownSeconds = 1
		rule.Cooldown = time.Second
	}

	switch rule.MetricType {
	case serviceAlertMetricLowRPS:
		if err := parseServiceAlertRuleLowRPS(&rule, data); err != nil {
			return serviceAlertRule{}, err
		}
	case serviceAlertMetricHighLatency:
		if err := parseServiceAlertRuleHighLatency(&rule, data); err != nil {
			return serviceAlertRule{}, err
		}
	case serviceAlertMetricSLA:
		if err := parseServiceAlertRuleSLA(&rule, data); err != nil {
			return serviceAlertRule{}, err
		}
	case serviceAlertMetricThreshold:
		if err := parseServiceAlertRuleThreshold(&rule, data); err != nil {
			return serviceAlertRule{}, err
		}
	case serviceAlertMetricCriticality:
		if err := parseServiceAlertRuleCriticality(&rule, data); err != nil {
			return serviceAlertRule{}, err
		}
	case serviceAlertMetricEscalation:
		if err := parseServiceAlertRuleEscalation(&rule, data); err != nil {
			return serviceAlertRule{}, err
		}
	case serviceAlertMetricPing:
		if err := parseServiceAlertRulePing(&rule, data); err != nil {
			return serviceAlertRule{}, err
		}
	default:
		return serviceAlertRule{}, fmt.Errorf("unsupported metric_type %q", rawMetricType)
	}

	return rule, nil
}

func ensureServiceAlertRuleCasesModule(rule *serviceAlertRule) {
	if rule.Module == "api" {
		rule.Module = "cases"
	}
}

func parseServiceAlertRuleLowRPS(rule *serviceAlertRule, data map[string]any) error {
	threshold, ok := floatFromMap(data, "min_rps", "minRps", "rps_threshold", "rpsThreshold")
	if !ok || threshold < 0 {
		return fmt.Errorf("low_rps rule requires non-negative min_rps")
	}
	rule.MinRPS = threshold
	return nil
}

func parseServiceAlertRuleHighLatency(rule *serviceAlertRule, data map[string]any) error {
	threshold, ok := floatFromMap(data, "max_latency_ms", "maxLatencyMs", "latency_ms", "latencyMs", "p95_ms", "p95Ms")
	if !ok || threshold <= 0 {
		return fmt.Errorf("high_latency rule requires positive max_latency_ms")
	}
	rule.MaxLatencyMs = threshold
	return nil
}

func parseServiceAlertRuleSLA(rule *serviceAlertRule, data map[string]any) error {
	ensureServiceAlertRuleCasesModule(rule)
	seconds, ok := intFromMap(data, "sla_seconds", "slaSeconds")
	if !ok || seconds <= 0 {
		return fmt.Errorf("sla rule requires positive sla_seconds")
	}
	rule.SLASeconds = int64(seconds)

	rule.CaseThreshold = 1
	if threshold, ok := intFromMap(data, "cases_threshold", "case_threshold", "threshold", "sla_threshold", "slaThreshold"); ok {
		rule.CaseThreshold = threshold
	}
	if rule.CaseThreshold <= 0 {
		return fmt.Errorf("sla rule requires positive cases_threshold")
	}
	return nil
}

func parseServiceAlertRuleThreshold(rule *serviceAlertRule, data map[string]any) error {
	ensureServiceAlertRuleCasesModule(rule)
	rule.CaseThreshold = 0
	if threshold, ok := intFromMap(data, "cases_threshold", "case_threshold", "threshold", "max_cases"); ok {
		rule.CaseThreshold = threshold
	}
	if rule.CaseThreshold <= 0 {
		return fmt.Errorf("threshold rule requires positive cases_threshold")
	}
	rule.ThresholdMode = normalizeServiceAlertThresholdMode(stringFromMap(data, "threshold_mode", "thresholdMode", "scope"))
	return nil
}

func parseServiceAlertRuleCriticality(rule *serviceAlertRule, data map[string]any) error {
	ensureServiceAlertRuleCasesModule(rule)
	rule.CriticalSeverities = normalizeUniqueStrings(
		stringSliceFromMap(data, "criticality_levels", "criticalityLevels", "critical_severities", "criticalSeverities", "severity_levels", "severityLevels"),
	)
	if len(rule.CriticalSeverities) == 0 {
		rule.CriticalSeverities = []string{"critical"}
	}
	rule.CaseThreshold = 1
	if threshold, ok := intFromMap(data, "cases_threshold", "case_threshold", "threshold", "critical_cases_threshold", "criticalCasesThreshold"); ok {
		rule.CaseThreshold = threshold
	}
	if rule.CaseThreshold <= 0 {
		return fmt.Errorf("criticality rule requires positive cases_threshold")
	}
	return nil
}

func parseServiceAlertRuleEscalation(rule *serviceAlertRule, data map[string]any) error {
	ensureServiceAlertRuleCasesModule(rule)
	rule.CaseThreshold = 1
	if threshold, ok := intFromMap(
		data,
		"escalation_threshold",
		"escalationThreshold",
		"cases_threshold",
		"case_threshold",
		"threshold",
	); ok {
		rule.CaseThreshold = threshold
	}
	if rule.CaseThreshold <= 0 {
		return fmt.Errorf("escalation rule requires positive threshold")
	}
	rule.EscalationEventTypes = normalizeUniqueStrings(
		stringSliceFromMap(data, "escalation_event_types", "escalationEventTypes", "event_types", "eventTypes"),
	)
	if len(rule.EscalationEventTypes) == 0 {
		rule.EscalationEventTypes = []string{"case_escalated", "case_escalation_received"}
	}
	return nil
}

func parseServiceAlertRulePing(rule *serviceAlertRule, data map[string]any) error {
	ensureServiceAlertRuleCasesModule(rule)
	seconds, ok := intFromMap(data, "ping_inactivity_seconds", "pingInactivitySeconds")
	if !ok || seconds <= 0 {
		return fmt.Errorf("ping rule requires positive ping_inactivity_seconds")
	}
	rule.PingInactivitySec = int64(seconds)

	rule.CaseThreshold = 1
	if threshold, ok := intFromMap(
		data,
		"ping_threshold",
		"pingThreshold",
		"cases_threshold",
		"case_threshold",
		"threshold",
	); ok {
		rule.CaseThreshold = threshold
	}
	if rule.CaseThreshold <= 0 {
		return fmt.Errorf("ping rule requires positive ping_threshold")
	}
	return nil
}

func (e *ServiceAlertEvaluator) evaluateRule(ctx context.Context, rule serviceAlertRule, now time.Time) (serviceAlertEvaluationResult, error) {
	switch rule.MetricType {
	case serviceAlertMetricLowRPS:
		snapshot := e.metrics.ModuleOperationSnapshot(rule.Module, rule.Window)
		if snapshot.RequestsPerSecond >= rule.MinRPS {
			return serviceAlertEvaluationResult{}, nil
		}
		title := fmt.Sprintf("%s: low RPS detected", rule.Name)
		message := fmt.Sprintf("Module %s has low throughput: %.3f rps (threshold %.3f, window %ds).", snapshot.Module, snapshot.RequestsPerSecond, rule.MinRPS, rule.WindowSeconds)
		return serviceAlertEvaluationResult{
			Breached: true,
			Title:    title,
			Message:  message,
			Metadata: map[string]any{
				"module":              snapshot.Module,
				"observed_rps":        roundFloat(snapshot.RequestsPerSecond, 4),
				"observed_latency_ms": roundFloat(snapshot.AvgLatencyMs, 2),
				"observed_operations": snapshot.Operations,
				"threshold_min_rps":   rule.MinRPS,
				"window_seconds":      rule.WindowSeconds,
				"cooldown_seconds":    rule.CooldownSeconds,
				"threshold_mode":      "module",
				"rule_category":       "service_module",
			},
		}, nil
	case serviceAlertMetricHighLatency:
		snapshot := e.metrics.ModuleOperationSnapshot(rule.Module, rule.Window)
		if snapshot.Operations == 0 || snapshot.AvgLatencyMs <= rule.MaxLatencyMs {
			return serviceAlertEvaluationResult{}, nil
		}
		title := fmt.Sprintf("%s: high latency detected", rule.Name)
		message := fmt.Sprintf(
			"Module %s latency is high: avg %.1fms (threshold %.1fms, operations %d, window %ds).",
			snapshot.Module,
			snapshot.AvgLatencyMs,
			rule.MaxLatencyMs,
			snapshot.Operations,
			rule.WindowSeconds,
		)
		return serviceAlertEvaluationResult{
			Breached: true,
			Title:    title,
			Message:  message,
			Metadata: map[string]any{
				"module":                   snapshot.Module,
				"observed_rps":             roundFloat(snapshot.RequestsPerSecond, 4),
				"observed_latency_ms":      roundFloat(snapshot.AvgLatencyMs, 2),
				"observed_operations":      snapshot.Operations,
				"threshold_max_latency_ms": rule.MaxLatencyMs,
				"window_seconds":           rule.WindowSeconds,
				"cooldown_seconds":         rule.CooldownSeconds,
				"threshold_mode":           "module",
				"rule_category":            "service_module",
			},
		}, nil
	case serviceAlertMetricSLA:
		return e.evaluateCaseCutoffMetric(
			ctx,
			rule,
			now,
			rule.SLASeconds,
			func(ctx context.Context, tenantID uuid.UUID, cutoff time.Time, openStatuses []string) (int, error) {
				return e.cases.CountOpenCreatedBefore(ctx, tenantID, cutoff, openStatuses)
			},
			errServiceAlertCasesRepositoryRequiredForSLARules,
			"SLA breach",
			"Detected %d open case(s) older than %ds (threshold %d).",
			"sla_seconds",
			"case_sla",
		)
	case serviceAlertMetricThreshold:
		if e.cases == nil {
			return serviceAlertEvaluationResult{}, fmt.Errorf("cases repository is required for threshold rules")
		}
		count, err := e.evaluateCaseThreshold(ctx, rule)
		if err != nil {
			return serviceAlertEvaluationResult{}, err
		}
		if count < rule.CaseThreshold {
			return serviceAlertEvaluationResult{}, nil
		}
		title := fmt.Sprintf("%s: case threshold exceeded", rule.Name)
		message := fmt.Sprintf("Case count reached %d (threshold %d, scope %s).", count, rule.CaseThreshold, rule.ThresholdMode)
		return serviceAlertEvaluationResult{
			Breached: true,
			Title:    title,
			Message:  message,
			Metadata: map[string]any{
				"module":           "cases",
				"observed_cases":   count,
				"threshold_cases":  rule.CaseThreshold,
				"threshold_mode":   rule.ThresholdMode,
				"open_statuses":    rule.OpenStatuses,
				"window_seconds":   rule.WindowSeconds,
				"cooldown_seconds": rule.CooldownSeconds,
				"rule_category":    "case_threshold",
			},
		}, nil
	case serviceAlertMetricCriticality:
		if e.cases == nil {
			return serviceAlertEvaluationResult{}, fmt.Errorf("cases repository is required for criticality rules")
		}
		count, err := e.cases.CountOpenBySeverity(ctx, rule.TenantID, rule.CriticalSeverities, rule.OpenStatuses)
		if err != nil {
			return serviceAlertEvaluationResult{}, err
		}
		if count < rule.CaseThreshold {
			return serviceAlertEvaluationResult{}, nil
		}
		title := fmt.Sprintf("%s: criticality threshold exceeded", rule.Name)
		message := fmt.Sprintf("Detected %d open case(s) with severities [%s] (threshold %d).", count, strings.Join(rule.CriticalSeverities, ", "), rule.CaseThreshold)
		return serviceAlertEvaluationResult{
			Breached: true,
			Title:    title,
			Message:  message,
			Metadata: map[string]any{
				"module":             "cases",
				"observed_cases":     count,
				"threshold_cases":    rule.CaseThreshold,
				"criticality_levels": rule.CriticalSeverities,
				"open_statuses":      rule.OpenStatuses,
				"window_seconds":     rule.WindowSeconds,
				"cooldown_seconds":   rule.CooldownSeconds,
				"rule_category":      "case_criticality",
			},
		}, nil
	case serviceAlertMetricEscalation:
		if e.caseEvents == nil {
			return serviceAlertEvaluationResult{}, fmt.Errorf("case events repository is required for escalation rules")
		}
		count, err := e.caseEvents.CountByTenantAndTypesSince(ctx, rule.TenantID, rule.EscalationEventTypes, now.Add(-rule.Window))
		if err != nil {
			return serviceAlertEvaluationResult{}, err
		}
		if count < rule.CaseThreshold {
			return serviceAlertEvaluationResult{}, nil
		}
		title := fmt.Sprintf("%s: escalation activity detected", rule.Name)
		message := fmt.Sprintf("Detected %d escalation event(s) in the last %ds (threshold %d).", count, rule.WindowSeconds, rule.CaseThreshold)
		return serviceAlertEvaluationResult{
			Breached: true,
			Title:    title,
			Message:  message,
			Metadata: map[string]any{
				"module":           "cases",
				"observed_events":  count,
				"threshold_events": rule.CaseThreshold,
				"event_types":      rule.EscalationEventTypes,
				"window_seconds":   rule.WindowSeconds,
				"cooldown_seconds": rule.CooldownSeconds,
				"rule_category":    "case_escalation",
			},
		}, nil
	case serviceAlertMetricPing:
		return e.evaluateCaseCutoffMetric(
			ctx,
			rule,
			now,
			rule.PingInactivitySec,
			func(ctx context.Context, tenantID uuid.UUID, cutoff time.Time, openStatuses []string) (int, error) {
				return e.cases.CountOpenInactiveSince(ctx, tenantID, cutoff, openStatuses)
			},
			errServiceAlertCasesRepositoryRequiredForPingRules,
			"ping required",
			"Detected %d open case(s) without updates for %ds (threshold %d).",
			"ping_inactivity_seconds",
			"case_ping",
		)
	default:
		return serviceAlertEvaluationResult{}, nil
	}
}

type caseCountCutoffFn func(context.Context, uuid.UUID, time.Time, []string) (int, error)

func (e *ServiceAlertEvaluator) evaluateCaseCutoffMetric(
	ctx context.Context,
	rule serviceAlertRule,
	now time.Time,
	cutoffSeconds int64,
	countFn caseCountCutoffFn,
	repoRequiredError error,
	titleSuffix string,
	messageTemplate string,
	secondsKey string,
	ruleCategory string,
) (serviceAlertEvaluationResult, error) {
	if e.cases == nil {
		return serviceAlertEvaluationResult{}, repoRequiredError
	}
	cutoff := now.Add(-time.Duration(cutoffSeconds) * time.Second)
	count, err := countFn(ctx, rule.TenantID, cutoff, rule.OpenStatuses)
	if err != nil {
		return serviceAlertEvaluationResult{}, err
	}
	if count < rule.CaseThreshold {
		return serviceAlertEvaluationResult{}, nil
	}
	title := fmt.Sprintf("%s: %s", rule.Name, titleSuffix)
	message := fmt.Sprintf(messageTemplate, count, cutoffSeconds, rule.CaseThreshold)
	metadata := map[string]any{
		"module":           "cases",
		"observed_cases":   count,
		"threshold_cases":  rule.CaseThreshold,
		secondsKey:         cutoffSeconds,
		"open_statuses":    rule.OpenStatuses,
		"window_seconds":   rule.WindowSeconds,
		"cooldown_seconds": rule.CooldownSeconds,
		"rule_category":    ruleCategory,
	}
	return serviceAlertEvaluationResult{
		Breached: true,
		Title:    title,
		Message:  message,
		Metadata: metadata,
	}, nil
}

func (e *ServiceAlertEvaluator) evaluateCaseThreshold(ctx context.Context, rule serviceAlertRule) (int, error) {
	if rule.ThresholdMode == "total" {
		return e.cases.CountByTenant(ctx, rule.TenantID)
	}
	return e.cases.CountInWorkByTenant(ctx, rule.TenantID, rule.OpenStatuses)
}

func (e *ServiceAlertEvaluator) createServiceAlertNotification(ctx context.Context, rule serviceAlertRule, userID uuid.UUID, evaluation serviceAlertEvaluationResult, now time.Time) error {
	ownerID := userID
	tenantID := rule.TenantID
	metadata := map[string]any{
		"rule_id":          rule.ID.String(),
		"metric_type":      rule.MetricType,
		"module":           rule.Module,
		"severity":         rule.Severity,
		"window_seconds":   rule.WindowSeconds,
		"cooldown_seconds": rule.CooldownSeconds,
	}
	for key, value := range evaluation.Metadata {
		metadata[key] = value
	}
	if rule.Description != "" {
		metadata["description"] = rule.Description
	}

	notification, err := e.catalog.Create(ctx, repository.CatalogCreateParams{
		TenantID: &tenantID,
		Kind:     "notifications",
		OwnerID:  &ownerID,
		Data: map[string]any{
			"title":      evaluation.Title,
			"message":    evaluation.Message,
			"type":       rule.Severity,
			"source":     "service_alerting",
			"rule_name":  rule.Name,
			"rule_id":    rule.ID.String(),
			"metadata":   metadata,
			"created_at": now.Format(time.RFC3339Nano),
		},
		CreatedBy: nil,
	})
	if err != nil {
		return err
	}
	if e.notificationQueue == nil || !e.notificationQueue.Enabled() {
		return nil
	}
	return e.notificationQueue.Enqueue(ctx, NotificationDeliveryEvent{
		EventID:        uuid.NewString(),
		NotificationID: notification.ID.String(),
		TenantID:       tenantID.String(),
		UserID:         userID.String(),
		Title:          evaluation.Title,
		Message:        evaluation.Message,
		Type:           rule.Severity,
		CreatedAt:      now.Format(time.RFC3339Nano),
	})
}

func (e *ServiceAlertEvaluator) cooldownActive(rule serviceAlertRule, now time.Time) bool {
	if rule.Cooldown <= 0 {
		return false
	}
	key := serviceAlertRuleKey(rule.TenantID, rule.ID)
	e.stateMu.Lock()
	lastSent, ok := e.lastAlertMap[key]
	e.stateMu.Unlock()
	if !ok {
		return false
	}
	return now.Sub(lastSent) < rule.Cooldown
}

func (e *ServiceAlertEvaluator) markAlertSent(rule serviceAlertRule, now time.Time) {
	key := serviceAlertRuleKey(rule.TenantID, rule.ID)
	e.stateMu.Lock()
	e.lastAlertMap[key] = now
	e.stateMu.Unlock()
}

func serviceAlertRuleKey(tenantID, ruleID uuid.UUID) string {
	return tenantID.String() + ":" + ruleID.String()
}

func floatFromMap(payload map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		value, ok := payload[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case float64:
			return typed, true
		case float32:
			return float64(typed), true
		case int:
			return float64(typed), true
		case int64:
			return float64(typed), true
		case int32:
			return float64(typed), true
		case json.Number:
			if parsed, err := typed.Float64(); err == nil {
				return parsed, true
			}
		case string:
			if parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64); err == nil {
				return parsed, true
			}
		}
	}
	return 0, false
}

func normalizeServiceAlertSeverity(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "critical", "error", "high":
		return "critical"
	case "info", "low":
		return "info"
	case "warning", "warn", "medium":
		return "warning"
	default:
		return "warning"
	}
}

func normalizeServiceAlertMetricType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case serviceAlertMetricLowRPS:
		return serviceAlertMetricLowRPS
	case serviceAlertMetricHighLatency:
		return serviceAlertMetricHighLatency
	case serviceAlertMetricSLA, "case_sla":
		return serviceAlertMetricSLA
	case serviceAlertMetricThreshold, "case_threshold":
		return serviceAlertMetricThreshold
	case serviceAlertMetricCriticality, "case_criticality":
		return serviceAlertMetricCriticality
	case serviceAlertMetricEscalation, "case_escalation":
		return serviceAlertMetricEscalation
	case serviceAlertMetricPing, "case_ping":
		return serviceAlertMetricPing
	default:
		return ""
	}
}

func normalizeServiceAlertThresholdMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "total", "all":
		return "total"
	default:
		return "in_work"
	}
}

func normalizeUniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func stringSliceFromMap(payload map[string]any, keys ...string) []string {
	out := make([]string, 0)
	for _, key := range keys {
		value, ok := payload[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case []string:
			out = append(out, typed...)
		case []any:
			for _, item := range typed {
				text := strings.TrimSpace(fmt.Sprint(item))
				if text != "" && text != "<nil>" {
					out = append(out, text)
				}
			}
		case string:
			for _, token := range strings.FieldsFunc(typed, func(r rune) bool {
				return r == ',' || r == ';' || r == '\n'
			}) {
				text := strings.TrimSpace(token)
				if text != "" {
					out = append(out, text)
				}
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func roundFloat(value float64, digits int) float64 {
	if digits <= 0 {
		return value
	}
	factor := math.Pow(10, float64(digits))
	return math.Round(value*factor) / factor
}
