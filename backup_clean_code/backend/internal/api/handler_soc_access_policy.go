package api

import (
	"context"
	"strings"

	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

const socAccessPolicyKind = "soc_access_policies"

type socAccessPolicy struct {
	AllowedCaseTags []string
	MaxCasesInWork  int
}

func (h *Handler) effectiveSOCAccessPolicy(ctx context.Context, tenantID uuid.UUID, identity models.Identity) socAccessPolicy {
	if identity.IsPlatformAdmin || identity.TenantRole == models.TenantRoleAdmin {
		return socAccessPolicy{}
	}
	return h.loadSOCAccessPolicy(ctx, tenantID)
}

func (h *Handler) loadSOCAccessPolicy(ctx context.Context, tenantID uuid.UUID) socAccessPolicy {
	if h == nil || h.catalog == nil {
		return socAccessPolicy{}
	}
	items, err := h.catalog.List(ctx, repository.CatalogListParams{
		Kind:     socAccessPolicyKind,
		TenantID: &tenantID,
		Limit:    1,
	})
	if err != nil || len(items) == 0 {
		return socAccessPolicy{}
	}
	item := items[0]
	allowedCaseTags := normalizeTags(stringSliceFromMap(item.Data, "allowed_case_tags", "allowedCaseTags", "case_tags", "caseTags"))
	maxCasesInWork := 0
	if parsed, ok := intFromMap(item.Data, "max_cases_in_work", "maxCasesInWork", "in_work_limit", "inWorkLimit"); ok && parsed > 0 {
		maxCasesInWork = parsed
	}
	return socAccessPolicy{
		AllowedCaseTags: allowedCaseTags,
		MaxCasesInWork:  maxCasesInWork,
	}
}

func (h *Handler) enforceCaseTagAccessPolicy(
	ctx context.Context,
	requestedTenantID uuid.UUID,
	resolvedTenantID uuid.UUID,
	caseID uuid.UUID,
	identity models.Identity,
) error {
	policy := h.effectiveSOCAccessPolicy(ctx, requestedTenantID, identity)
	if len(policy.AllowedCaseTags) == 0 {
		return nil
	}
	if h == nil || h.cases == nil {
		return echo.NewHTTPError(500, "cases repository is not configured")
	}
	allowed, err := h.cases.HasAnyCaseMetaTag(ctx, resolvedTenantID, caseID, policy.AllowedCaseTags)
	if err != nil {
		return echo.NewHTTPError(500, "failed to enforce case tag policy")
	}
	if !allowed {
		return echo.NewHTTPError(404, "case not found in tenant")
	}
	return nil
}

func (h *Handler) caseAllowedTagsForIdentity(ctx context.Context, tenantID uuid.UUID, identity models.Identity) []string {
	policy := h.effectiveSOCAccessPolicy(ctx, tenantID, identity)
	if len(policy.AllowedCaseTags) == 0 {
		return nil
	}
	return append([]string{}, policy.AllowedCaseTags...)
}

func (h *Handler) filterCasesByTagPolicy(
	ctx context.Context,
	requestedTenantID uuid.UUID,
	identity models.Identity,
	items []models.Case,
) ([]models.Case, error) {
	allowedTags := h.caseAllowedTagsForIdentity(ctx, requestedTenantID, identity)
	if len(allowedTags) == 0 || len(items) == 0 {
		return items, nil
	}
	filtered := make([]models.Case, 0, len(items))
	for _, item := range items {
		allowed, err := h.cases.HasAnyCaseMetaTag(ctx, item.TenantID, item.ID, allowedTags)
		if err != nil {
			return nil, echo.NewHTTPError(500, "failed to enforce case tag policy")
		}
		if allowed {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
}

func (h *Handler) enforceAssigneeCaseWorkloadLimit(
	ctx context.Context,
	tenantID uuid.UUID,
	identity models.Identity,
	assigneeID *uuid.UUID,
	currentCase *models.Case,
	targetStatus string,
) error {
	if assigneeID == nil {
		return nil
	}
	policy := h.effectiveSOCAccessPolicy(ctx, tenantID, identity)
	if policy.MaxCasesInWork <= 0 {
		return nil
	}
	if h.isCaseStatusClosed(ctx, tenantID, targetStatus) {
		return nil
	}
	openStatuses := h.listOpenCaseStatusCodes(ctx, tenantID)
	workload, err := h.cases.CountOpenByAssignees(ctx, tenantID, []uuid.UUID{*assigneeID}, openStatuses)
	if err != nil {
		return echo.NewHTTPError(500, "failed to validate assignee workload")
	}
	openCount := workload[*assigneeID]
	if currentCase != nil && currentCase.AssignedTo != nil && *currentCase.AssignedTo == *assigneeID && !h.isCaseStatusClosed(ctx, tenantID, currentCase.Status) {
		if openCount > 0 {
			openCount--
		}
	}
	if openCount >= policy.MaxCasesInWork {
		return echo.NewHTTPError(409, "assignee reached max in-work cases limit")
	}
	return nil
}

func resolveTargetCaseStatus(currentCase *models.Case, requested *string) string {
	if requested != nil {
		return strings.TrimSpace(strings.ToLower(*requested))
	}
	if currentCase != nil {
		return strings.TrimSpace(strings.ToLower(currentCase.Status))
	}
	return ""
}
