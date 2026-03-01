package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"incidenthub/backend/internal/middleware"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

//nolint:gochecknoglobals // Static allowlist used for request validation.
var caseEscalationHandoffTypes = map[string]struct{}{
	"employee_client":         {},
	"employee_multi_clients":  {},
	"employee_without_client": {},
	"multi_employee_client":   {},
	"custom":                  {},
}

func (h *Handler) EscalateCase(c *echo.Context) error {
	identity, _ := middleware.GetIdentity(c)
	sourceTenantID, sourceCaseID, err := h.resolveCaseInTenant(c)
	if err != nil {
		return err
	}

	var req escalateCaseRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	handoffType, err := normalizeCaseEscalationHandoffType(req.HandoffType)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	targetTenantID, err := h.resolveTargetTenantID(c.Request().Context(), req.TargetTenantID, req.TargetTenantSlug)
	if err != nil {
		return err
	}
	if targetTenantID == sourceTenantID {
		return echo.NewHTTPError(http.StatusBadRequest, "target tenant must differ from source tenant")
	}
	targetTenant, err := h.tenants.GetByID(c.Request().Context(), targetTenantID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "target tenant not found")
	}
	if !targetTenant.IsActive {
		return echo.NewHTTPError(http.StatusConflict, "target tenant is inactive")
	}

	if !identity.IsPlatformAdmin {
		membership, membershipErr := h.memberships.Get(c.Request().Context(), targetTenantID, identity.UserID)
		if membershipErr != nil || !membership.IsActive {
			return echo.NewHTTPError(http.StatusForbidden, "membership in target tenant is required for escalation")
		}
	}

	sourceCase, err := h.cases.GetByID(c.Request().Context(), sourceTenantID, sourceCaseID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "case not found")
	}

	targetAssignee, err := h.resolveCaseEscalationTargetAssignee(c.Request().Context(), targetTenantID, req.TargetAssigneeID, sourceCase.Source, sourceCase.IncidentType)
	if err != nil {
		return err
	}

	targetStatus, err := h.resolveCaseStatus(c.Request().Context(), targetTenantID, sourceCase.Status)
	if err != nil {
		targetStatus, _ = h.resolveCaseStatus(c.Request().Context(), targetTenantID, "")
	}

	targetCase, err := h.cases.Create(c.Request().Context(), repository.CreateCaseParams{
		TenantID:          targetTenantID,
		CaseNumber:        generateCaseNumber(),
		Title:             sourceCase.Title,
		Description:       buildEscalatedCaseDescription(sourceCase, sourceTenantID, handoffType, req.Summary),
		Source:            sourceCase.Source,
		IncidentType:      sourceCase.IncidentType,
		Status:            targetStatus,
		Priority:          sourceCase.Priority,
		Impact:            sourceCase.Impact,
		Confidence:        sourceCase.Confidence,
		Severity:          sourceCase.Severity,
		TLP:               sourceCase.TLP,
		PAP:               sourceCase.PAP,
		DetectedAt:        optionalCaseTimeRFC3339(sourceCase.DetectedAt),
		OccurredAt:        optionalCaseTimeRFC3339(sourceCase.OccurredAt),
		ClosedAt:          nil,
		ResolutionSummary: sourceCase.ResolutionSummary,
		CreatedBy:         identity.UserID,
		AssignedTo:        targetAssignee,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to escalate case")
	}

	includeObservables := true
	if req.IncludeObservables != nil {
		includeObservables = *req.IncludeObservables
	}
	copiedObservables := 0
	if includeObservables && h.observables != nil {
		observables, listErr := h.observables.ListByCase(c.Request().Context(), sourceTenantID, sourceCaseID, 2000, 0)
		if listErr == nil {
			for idx := range observables {
				sourceObservable := observables[idx]
				createdObservable, createErr := h.observables.Create(c.Request().Context(), repository.CreateObservableParams{
					TenantID:  targetTenantID,
					CaseID:    targetCase.ID,
					Type:      sourceObservable.Type,
					Value:     sourceObservable.Value,
					Verdict:   sourceObservable.Verdict,
					Source:    sourceObservable.Source,
					Tags:      sourceObservable.Tags,
					CreatedBy: identity.UserID,
				})
				if createErr != nil {
					continue
				}
				copiedObservables++
				if h.search != nil {
					_ = h.search.IndexDocument(c.Request().Context(), "observables", createdObservable.ID.String(), createdObservable)
				}
			}
		}
	}

	if h.caseEvents != nil {
		sourceEventMetadata := map[string]any{
			"target_tenant_id":   targetTenantID.String(),
			"target_case_id":     targetCase.ID.String(),
			"handoff_type":       handoffType,
			"summary":            strings.TrimSpace(req.Summary),
			"copied_observables": copiedObservables,
		}
		_, _ = h.caseEvents.Create(c.Request().Context(), repository.CreateCaseEventParams{
			TenantID:  sourceTenantID,
			CaseID:    sourceCaseID,
			EventType: "case_escalated",
			Title:     "Case escalated",
			Body:      fmt.Sprintf("Escalated to tenant %s as case %s", targetTenant.Slug, targetCase.CaseNumber),
			ActorID:   &identity.UserID,
			Metadata:  sourceEventMetadata,
		})

		targetEventMetadata := map[string]any{
			"source_tenant_id":   sourceTenantID.String(),
			"source_case_id":     sourceCaseID.String(),
			"handoff_type":       handoffType,
			"summary":            strings.TrimSpace(req.Summary),
			"copied_observables": copiedObservables,
		}
		_, _ = h.caseEvents.Create(c.Request().Context(), repository.CreateCaseEventParams{
			TenantID:  targetTenantID,
			CaseID:    targetCase.ID,
			EventType: "case_escalation_received",
			Title:     "Escalated case received",
			Body:      fmt.Sprintf("Received escalation from tenant %s", sourceTenantID.String()),
			ActorID:   &identity.UserID,
			Metadata:  targetEventMetadata,
		})
	}

	if h.search != nil {
		_ = h.search.IndexDocument(c.Request().Context(), "cases", targetCase.ID.String(), targetCase)
	}
	_ = h.audits.Log(c.Request().Context(), &sourceTenantID, &identity.UserID, "case_escalate", "case", &sourceCaseID, map[string]any{
		"target_tenant_id": targetTenantID.String(),
		"target_case_id":   targetCase.ID.String(),
		"handoff_type":     handoffType,
	})
	h.enqueueAIAgentQueueEvent(c.Request().Context(), targetTenantID, &identity.UserID, "case", targetCase.ID, aiAgentQueueSourceCaseEscalation)

	return c.JSON(http.StatusCreated, map[string]any{
		"source_case_id":      sourceCaseID.String(),
		"target_tenant_id":    targetTenantID.String(),
		"target_tenant_slug":  targetTenant.Slug,
		"target_case":         targetCase,
		"handoff_type":        handoffType,
		"copied_observables":  copiedObservables,
		"include_observables": includeObservables,
	})
}

func (h *Handler) resolveCaseEscalationTargetAssignee(
	ctx context.Context,
	targetTenantID uuid.UUID,
	targetAssigneeIDRaw string,
	source string,
	incidentType string,
) (*uuid.UUID, error) {
	targetAssigneeIDRaw = strings.TrimSpace(targetAssigneeIDRaw)
	if targetAssigneeIDRaw == "" {
		return h.resolveCaseAutoAssignee(ctx, targetTenantID, nil, source, incidentType)
	}
	targetAssigneeID, err := uuid.Parse(targetAssigneeIDRaw)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid target_assignee_id")
	}
	if h.memberships != nil {
		membership, membershipErr := h.memberships.Get(ctx, targetTenantID, targetAssigneeID)
		if membershipErr != nil || !membership.IsActive {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "target assignee must be active in target tenant")
		}
		switch membership.Role {
		case models.TenantRoleAnalyst, models.TenantRoleAdmin:
		default:
			return nil, echo.NewHTTPError(http.StatusBadRequest, "target assignee must be analyst or tenant admin")
		}
	}
	return &targetAssigneeID, nil
}

func normalizeCaseEscalationHandoffType(input string) (string, error) {
	normalized := strings.TrimSpace(strings.ToLower(input))
	if normalized == "" {
		normalized = "employee_client"
	}
	if _, ok := caseEscalationHandoffTypes[normalized]; !ok {
		return "", fmt.Errorf("handoff_type must be one of: employee_client, employee_multi_clients, employee_without_client, multi_employee_client, custom")
	}
	return normalized, nil
}

func optionalCaseTimeRFC3339(value *time.Time) *string {
	if value == nil || value.IsZero() {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339)
	return &formatted
}

func buildEscalatedCaseDescription(sourceCase *models.Case, sourceTenantID uuid.UUID, handoffType, summary string) string {
	base := ""
	if sourceCase != nil {
		base = strings.TrimSpace(sourceCase.Description)
	}
	meta := strings.TrimSpace(summary)
	if meta == "" {
		meta = "Escalation summary is not provided."
	}
	escalationBlock := fmt.Sprintf(
		"Escalated from tenant %s, source case %s.\nHandoff type: %s.\nSummary: %s",
		sourceTenantID.String(),
		sourceCase.CaseNumber,
		handoffType,
		meta,
	)
	if base == "" {
		return escalationBlock
	}
	return base + "\n\n---\n" + escalationBlock
}
