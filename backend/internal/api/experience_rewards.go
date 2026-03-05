package api

import (
	"context"
	"fmt"
	"strings"

	"incidenthub/backend/internal/logger"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"

	"github.com/google/uuid"
)

const (
	xpRuleCaseClosedLow             = "case_closed_low"
	xpRuleCaseClosedMedium          = "case_closed_medium"
	xpRuleCaseClosedHigh            = "case_closed_high"
	xpRuleCaseClosedCritical        = "case_closed_critical"
	xpRuleAchievementGranted        = "achievement_granted_default"
	xpRuleConnectorInvoked          = "connector_invoked"
	xpRuleAnalyzerInvoked           = "analyzer_invoked"
	xpRuleResponderInvoked          = "responder_invoked"
	xpRuleIncidentFirstMessage      = "incident_first_message"
	xpEventTypeCaseClosed           = "case_closed"
	xpEventTypeAchievementGranted   = "achievement_granted"
	xpEventTypeConnectorInvoked     = "connector_invoked"
	xpEventTypeAnalyzerInvoked      = "analyzer_invoked"
	xpEventTypeResponderInvoked     = "responder_invoked"
	xpEventTypeIncidentFirstMessage = "incident_first_message"
	xpEventTypeManualGrant          = "manual_grant"
)

func (h *Handler) rewardCaseClosure(ctx context.Context, tenantID uuid.UUID, userID uuid.UUID, caseID uuid.UUID, severity string) {
	ruleKey := xpRuleKeyForSeverity(severity)
	details := map[string]any{
		"case_id":   caseID.String(),
		"severity":  normalizeSeverityForXP(severity),
		"rule_key":  ruleKey,
		"triggered": xpEventTypeCaseClosed,
	}
	h.awardExperienceByRule(ctx, &tenantID, userID, ruleKey, xpEventTypeCaseClosed, "case_closed:"+caseID.String(), defaultExperienceDescription(xpEventTypeCaseClosed, details), details)
}

func (h *Handler) rewardConnectorInvocation(ctx context.Context, tenantID uuid.UUID, userID uuid.UUID, eventKey string, details map[string]any) {
	h.awardExperienceByRule(ctx, &tenantID, userID, xpRuleConnectorInvoked, xpEventTypeConnectorInvoked, eventKey, defaultExperienceDescription(xpEventTypeConnectorInvoked, details), details)
}

func (h *Handler) rewardAnalyzerInvocation(ctx context.Context, tenantID uuid.UUID, userID uuid.UUID, eventKey string, details map[string]any) {
	h.awardExperienceByRule(ctx, &tenantID, userID, xpRuleAnalyzerInvoked, xpEventTypeAnalyzerInvoked, eventKey, defaultExperienceDescription(xpEventTypeAnalyzerInvoked, details), details)
}

func (h *Handler) rewardResponderInvocation(ctx context.Context, tenantID uuid.UUID, userID uuid.UUID, eventKey string, details map[string]any) {
	h.awardExperienceByRule(ctx, &tenantID, userID, xpRuleResponderInvoked, xpEventTypeResponderInvoked, eventKey, defaultExperienceDescription(xpEventTypeResponderInvoked, details), details)
}

func (h *Handler) rewardFirstIncidentMessage(ctx context.Context, tenantID uuid.UUID, userID uuid.UUID, caseID uuid.UUID, details map[string]any) {
	eventKey := "first_incident_message:" + caseID.String()
	h.awardExperienceByRule(ctx, &tenantID, userID, xpRuleIncidentFirstMessage, xpEventTypeIncidentFirstMessage, eventKey, defaultExperienceDescription(xpEventTypeIncidentFirstMessage, details), details)
}

func (h *Handler) rewardAchievementGrant(ctx context.Context, tenantID uuid.UUID, grant models.CatalogItem) {
	if h.experience == nil || h.catalog == nil {
		return
	}
	if grant.OwnerID == nil {
		return
	}

	points := 0
	achievementID := strings.TrimSpace(stringFromMap(grant.Data, "achievement_id", "achievementId", "id"))
	achievementName := ""
	if parsedAchievementID, err := uuid.Parse(achievementID); err == nil {
		if achievement, resolveErr := h.resolveAchievementByID(ctx, tenantID, parsedAchievementID); resolveErr == nil && achievement != nil {
			achievementName = strings.TrimSpace(stringFromMap(achievement.Data, "name", "title"))
			if parsedPoints, ok := intFromMap(achievement.Data, "xp_reward", "xpReward", "xp", "experience_points"); ok && parsedPoints > 0 {
				points = parsedPoints
			}
		}
	}
	if points <= 0 {
		if defaultPoints, ok := h.loadRewardRulePoints(ctx, xpRuleAchievementGranted); ok {
			points = defaultPoints
		}
	}
	if points <= 0 {
		return
	}

	details := map[string]any{
		"grant_id":         grant.ID.String(),
		"achievement_id":   achievementID,
		"achievement_name": achievementName,
		"triggered":        xpEventTypeAchievementGranted,
		"achievement_xp":   points,
	}
	description := defaultExperienceDescription(xpEventTypeAchievementGranted, details)
	if achievementName != "" {
		description = "Achievement granted: " + achievementName
	}
	h.awardExperienceByPoints(ctx, &tenantID, *grant.OwnerID, points, xpEventTypeAchievementGranted, "achievement_grant:"+grant.ID.String(), description, details)
}

func (h *Handler) resolveAchievementByID(ctx context.Context, tenantID uuid.UUID, achievementID uuid.UUID) (*models.CatalogItem, error) {
	if achievementID == uuid.Nil {
		return nil, fmt.Errorf("achievement id is required")
	}
	if local, err := h.catalog.GetByID(ctx, "achievements", achievementID, &tenantID); err == nil {
		return local, nil
	}
	items, err := h.catalog.List(ctx, repository.CatalogListParams{
		Kind:          "achievements",
		TenantID:      &tenantID,
		IncludeGlobal: true,
		Limit:         1000,
	})
	if err != nil {
		return nil, err
	}
	for idx := range items {
		if items[idx].ID == achievementID {
			return &items[idx], nil
		}
	}
	return nil, fmt.Errorf("achievement %s not found", achievementID.String())
}

func (h *Handler) awardExperienceByRule(ctx context.Context, tenantID *uuid.UUID, userID uuid.UUID, ruleKey, eventType, eventKey, description string, details map[string]any) {
	points, ok := h.loadRewardRulePoints(ctx, ruleKey)
	if !ok || points <= 0 {
		return
	}
	h.awardExperienceByPoints(ctx, tenantID, userID, points, eventType, eventKey, description, details)
}

func (h *Handler) loadRewardRulePoints(ctx context.Context, ruleKey string) (int, bool) {
	if h.experience == nil {
		fallback := fallbackRewardRulePoints(ruleKey)
		return fallback, fallback > 0
	}
	points, err := h.experience.RulePoints(ctx, strings.TrimSpace(ruleKey))
	if err != nil {
		fallback := fallbackRewardRulePoints(ruleKey)
		if fallback > 0 {
			logger.Warnf("experience reward rule load failed (key=%s), using fallback=%d: %v", ruleKey, fallback, err)
			return fallback, true
		}
		logger.Warnf("experience reward rule load failed (key=%s): %v", ruleKey, err)
		return 0, false
	}
	if points <= 0 {
		fallback := fallbackRewardRulePoints(ruleKey)
		if fallback > 0 {
			return fallback, true
		}
		return 0, false
	}
	return points, true
}

func (h *Handler) awardExperienceByPoints(ctx context.Context, tenantID *uuid.UUID, userID uuid.UUID, points int, eventType, eventKey, description string, details map[string]any) {
	if h.experience == nil || points <= 0 {
		return
	}
	description = strings.TrimSpace(description)
	if description == "" {
		description = defaultExperienceDescription(eventType, details)
	}
	result, err := h.experience.Award(ctx, repository.AwardExperienceParams{
		TenantID:    tenantID,
		UserID:      userID,
		EventKey:    strings.TrimSpace(eventKey),
		EventType:   strings.TrimSpace(eventType),
		Points:      points,
		Description: description,
		Details:     details,
	})
	if err != nil {
		logger.Warnf("experience reward failed (user=%s event_key=%s): %v", userID.String(), eventKey, err)
		return
	}
	if result.Awarded {
		logger.Infof("experience rewarded (user=%s points=%d total=%d event_key=%s)", userID.String(), points, result.TotalPoints, eventKey)
	}
}

func xpRuleKeyForSeverity(severity string) string {
	switch normalizeSeverityForXP(severity) {
	case "critical":
		return xpRuleCaseClosedCritical
	case "high":
		return xpRuleCaseClosedHigh
	case "low":
		return xpRuleCaseClosedLow
	default:
		return xpRuleCaseClosedMedium
	}
}

func normalizeSeverityForXP(input string) string {
	switch strings.TrimSpace(strings.ToLower(input)) {
	case "critical":
		return "critical"
	case "high":
		return "high"
	case "low":
		return "low"
	default:
		return "medium"
	}
}

func (h *Handler) isCaseStatusClosed(ctx context.Context, tenantID uuid.UUID, status string) bool {
	statusCode := strings.TrimSpace(strings.ToLower(status))
	if statusCode == "" {
		return false
	}
	statuses, err := h.loadCaseStatuses(ctx, tenantID)
	if err != nil {
		return fallbackClosedCaseStatus(statusCode)
	}
	for _, item := range statuses {
		if item.Code == statusCode {
			return item.IsClosed
		}
	}
	return fallbackClosedCaseStatus(statusCode)
}

func fallbackClosedCaseStatus(statusCode string) bool {
	switch strings.TrimSpace(strings.ToLower(statusCode)) {
	case "closed", "resolved", "done":
		return true
	default:
		return false
	}
}

func fallbackRewardRulePoints(ruleKey string) int {
	switch strings.TrimSpace(ruleKey) {
	case xpRuleCaseClosedLow:
		return 120
	case xpRuleCaseClosedMedium:
		return 220
	case xpRuleCaseClosedHigh:
		return 350
	case xpRuleCaseClosedCritical:
		return 500
	case xpRuleAchievementGranted:
		return 150
	case xpRuleConnectorInvoked:
		return 60
	case xpRuleAnalyzerInvoked:
		return 80
	case xpRuleResponderInvoked:
		return 70
	case xpRuleIncidentFirstMessage:
		return 100
	default:
		return 0
	}
}

func defaultExperienceDescription(eventType string, details map[string]any) string {
	switch strings.TrimSpace(strings.ToLower(eventType)) {
	case xpEventTypeCaseClosed:
		severity := normalizeSeverityForXP(stringFromMap(details, "severity"))
		return fmt.Sprintf("Closed case (%s severity)", severity)
	case xpEventTypeAchievementGranted:
		if name := strings.TrimSpace(stringFromMap(details, "achievement_name", "achievementName")); name != "" {
			return "Achievement granted: " + name
		}
		return "Achievement granted"
	case xpEventTypeConnectorInvoked:
		return "Connector invoked"
	case xpEventTypeAnalyzerInvoked:
		return "Analyzer invoked"
	case xpEventTypeResponderInvoked:
		return "Responder invoked"
	case xpEventTypeIncidentFirstMessage:
		return "First incident message sent"
	case xpEventTypeManualGrant:
		return "Manual experience grant"
	default:
		return "Experience rewarded"
	}
}
