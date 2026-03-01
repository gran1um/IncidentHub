package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"incidenthub/backend/internal/middleware"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"
	"io"
	"math"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

//nolint:gochecknoglobals // Static allowlist used for request validation.
var listPageSizeOptions = map[int]struct{}{
	10:  {},
	30:  {},
	50:  {},
	100: {},
}

//nolint:gochecknoglobals // Static allowlist used for request validation.
var listCaseSortFieldOptions = map[string]struct{}{
	"updated_at":  {},
	"created_at":  {},
	"severity":    {},
	"status":      {},
	"title":       {},
	"case_number": {},
}

//nolint:gochecknoglobals // Static allowlist used for request validation.
var listAlertSortFieldOptions = map[string]struct{}{
	"updated_at": {},
	"created_at": {},
	"severity":   {},
	"status":     {},
	"title":      {},
	"source":     {},
}

var caseCustomSortFieldPattern = regexp.MustCompile(`^[a-z0-9_][a-z0-9_:\-]{0,63}$`)

const (
	caseClosureApprovalEventType      = "closure_approval"
	caseClosureApprovalsRequiredField = "closure_required_approvals"
	caseClosureApproverIDsField       = "closure_approver_ids"
)

func (h *Handler) ListAlerts(c *echo.Context) error {
	identity, hasIdentity := middleware.GetIdentity(c)
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	assigned, assignedTo, err := parseAssignedListFilter(c, identity, hasIdentity)
	if err != nil {
		return err
	}
	sortBy, sortOrder, err := parseListSortParams(c, listAlertSortFieldOptions, false)
	if err != nil {
		return err
	}
	search, err := parseListSearchParams(c)
	if err != nil {
		return err
	}
	search.CaseMetaTagsAny = h.caseAllowedTagsForIdentity(c.Request().Context(), tenantID, identity)
	page, pageSize, paged, err := parseListPageParams(c)
	if err != nil {
		return err
	}
	if paged {
		total, countErr := h.alerts.CountByTenantWithAssignedAndSearch(c.Request().Context(), tenantID, assigned, assignedTo, search)
		if countErr != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to count alerts")
		}
		offset := (page - 1) * pageSize
		items, listErr := h.alerts.ListByTenantWithAssignedSortedAndSearch(
			c.Request().Context(),
			tenantID,
			pageSize,
			offset,
			assigned,
			assignedTo,
			sortBy,
			sortOrder,
			search,
		)
		if listErr != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to list alerts")
		}
		return c.JSON(http.StatusOK, pagedListPayload(items, page, pageSize, total))
	}

	limit, offset, err := parseLegacyLimitOffset(c, 200, 200)
	if err != nil {
		return err
	}
	items, err := h.alerts.ListByTenantWithAssignedSortedAndSearch(
		c.Request().Context(),
		tenantID,
		limit,
		offset,
		assigned,
		assignedTo,
		sortBy,
		sortOrder,
		search,
	)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list alerts")
	}
	return c.JSON(http.StatusOK, items)
}

func (h *Handler) GetAlert(c *echo.Context) error {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	alertID, parseErr := uuid.Parse(strings.TrimSpace(c.Param("alertID")))
	if parseErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid alert id")
	}
	item, err := h.alerts.GetByID(c.Request().Context(), tenantID, alertID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "alert not found")
	}
	return c.JSON(http.StatusOK, item)
}

func (h *Handler) CreateAlert(c *echo.Context) error {
	identity, _ := middleware.GetIdentity(c)
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}

	var req createAlertRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Source = strings.TrimSpace(req.Source)
	req.Description = strings.TrimSpace(req.Description)
	if req.Title == "" || req.Source == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "title and source are required")
	}
	if req.Status == "" {
		req.Status = "new"
	}
	if req.Severity == "" {
		req.Severity = "medium"
	}
	if req.TLP == "" {
		req.TLP = "amber"
	}
	if req.PAP == "" {
		req.PAP = "amber"
	}
	if h.asyncOps != nil && h.asyncOps.Enabled() {
		alertID := uuid.New()
		payload, err := json.Marshal(asyncAlertCreatePayload{
			AlertID:     alertID.String(),
			Title:       req.Title,
			Description: req.Description,
			Source:      req.Source,
			Status:      req.Status,
			Severity:    req.Severity,
			TLP:         req.TLP,
			PAP:         req.PAP,
		})
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to queue alert creation")
		}
		operation := AsyncOperation{
			OperationID: uuid.NewString(),
			Type:        AsyncOperationAlertCreate,
			TenantID:    tenantID.String(),
			ActorID:     identity.UserID.String(),
			ResourceID:  alertID.String(),
			RequestedAt: time.Now().UTC().Format(time.RFC3339Nano),
			Payload:     payload,
		}
		if recordErr := h.recordQueuedAsyncOperation(c.Request().Context(), operation, "alert"); recordErr != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to register alert creation operation")
		}
		if enqueueErr := h.asyncOps.Enqueue(c.Request().Context(), operation); enqueueErr != nil {
			h.markAsyncOperationFailed(c.Request().Context(), operation, enqueueErr)
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to enqueue alert creation")
		}
		return c.JSON(http.StatusAccepted, queuedAsyncOperationResponse{
			OperationID:   operation.OperationID,
			Resource:      "alert",
			ResourceID:    alertID.String(),
			OperationType: string(operation.Type),
			Status:        "queued",
		})
	}

	item, err := h.alerts.Create(c.Request().Context(), repository.CreateAlertParams{
		TenantID:    tenantID,
		Title:       req.Title,
		Description: req.Description,
		Source:      req.Source,
		Status:      req.Status,
		Severity:    req.Severity,
		TLP:         req.TLP,
		PAP:         req.PAP,
		CreatedBy:   &identity.UserID,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to create alert")
	}
	if h.search != nil {
		_ = h.search.IndexDocument(c.Request().Context(), "alerts", item.ID.String(), item)
	}
	_ = h.audits.Log(c.Request().Context(), &tenantID, &identity.UserID, "alert_create", "alert", &item.ID, map[string]any{"title": item.Title})
	h.enqueueAIAgentQueueEvent(c.Request().Context(), tenantID, &identity.UserID, "alert", item.ID, aiAgentQueueSourceAPI)
	return c.JSON(http.StatusCreated, item)
}

func (h *Handler) UpdateAlert(c *echo.Context) error {
	identity, _ := middleware.GetIdentity(c)
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}

	alertID, parseErr := uuid.Parse(c.Param("alertID"))
	if parseErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid alert id")
	}

	var req updateAlertRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	var assignedTo *uuid.UUID
	assignedToRaw := ""
	if req.AssignedTo != nil && strings.TrimSpace(*req.AssignedTo) != "" {
		parsed, parseErr := uuid.Parse(strings.TrimSpace(*req.AssignedTo))
		if parseErr != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid assigned_to")
		}
		assignedTo = &parsed
		assignedToRaw = parsed.String()
	}

	if h.asyncOps != nil && h.asyncOps.Enabled() {
		if _, err := h.alerts.GetByID(c.Request().Context(), tenantID, alertID); err != nil {
			return echo.NewHTTPError(http.StatusNotFound, "alert not found")
		}
		payload, err := json.Marshal(asyncAlertUpdatePayload{
			AlertID:     alertID.String(),
			Title:       trimOptional(req.Title),
			Description: trimOptional(req.Description),
			Source:      trimOptional(req.Source),
			Status:      lowerOptional(req.Status),
			Severity:    lowerOptional(req.Severity),
			TLP:         lowerOptional(req.TLP),
			PAP:         lowerOptional(req.PAP),
			AssignedTo:  assignedToRaw,
		})
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to queue alert update")
		}
		operation := AsyncOperation{
			OperationID: uuid.NewString(),
			Type:        AsyncOperationAlertUpdate,
			TenantID:    tenantID.String(),
			ActorID:     identity.UserID.String(),
			ResourceID:  alertID.String(),
			RequestedAt: time.Now().UTC().Format(time.RFC3339Nano),
			Payload:     payload,
		}
		if recordErr := h.recordQueuedAsyncOperation(c.Request().Context(), operation, "alert"); recordErr != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to register alert update operation")
		}
		if enqueueErr := h.asyncOps.Enqueue(c.Request().Context(), operation); enqueueErr != nil {
			h.markAsyncOperationFailed(c.Request().Context(), operation, enqueueErr)
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to enqueue alert update")
		}
		return c.JSON(http.StatusAccepted, queuedAsyncOperationResponse{
			OperationID:   operation.OperationID,
			Resource:      "alert",
			ResourceID:    alertID.String(),
			OperationType: string(operation.Type),
			Status:        "queued",
		})
	}

	item, err := h.alerts.Update(c.Request().Context(), tenantID, alertID, repository.UpdateAlertParams{
		Title:       trimOptional(req.Title),
		Description: trimOptional(req.Description),
		Source:      trimOptional(req.Source),
		Status:      lowerOptional(req.Status),
		Severity:    lowerOptional(req.Severity),
		TLP:         lowerOptional(req.TLP),
		PAP:         lowerOptional(req.PAP),
		AssignedTo:  assignedTo,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to update alert")
	}
	if h.search != nil {
		_ = h.search.IndexDocument(c.Request().Context(), "alerts", item.ID.String(), item)
	}
	_ = h.audits.Log(c.Request().Context(), &tenantID, &identity.UserID, "alert_update", "alert", &item.ID, nil)
	return c.JSON(http.StatusOK, item)
}

func (h *Handler) DeleteAlert(c *echo.Context) error {
	identity, _ := middleware.GetIdentity(c)
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	alertID, parseErr := uuid.Parse(strings.TrimSpace(c.Param("alertID")))
	if parseErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid alert id")
	}
	if _, err := h.alerts.GetByID(c.Request().Context(), tenantID, alertID); err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "alert not found")
	}

	if h.asyncOps != nil && h.asyncOps.Enabled() {
		resp, queueErr := h.queueAsyncDeleteOperation(
			c.Request().Context(),
			tenantID,
			identity.UserID,
			"alert",
			alertID,
			AsyncOperationAlertDelete,
		)
		if queueErr != nil {
			return queueErr
		}
		return c.JSON(http.StatusAccepted, resp)
	}

	deleted, err := h.alerts.Delete(c.Request().Context(), tenantID, alertID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete alert")
	}
	if !deleted {
		return echo.NewHTTPError(http.StatusNotFound, "alert not found")
	}
	if h.search != nil {
		_ = h.search.DeleteDocument(c.Request().Context(), "alerts", alertID.String())
	}
	_ = h.audits.Log(c.Request().Context(), &tenantID, &identity.UserID, "alert_delete", "alert", &alertID, nil)
	return c.JSON(http.StatusOK, map[string]any{"success": true})
}

func (h *Handler) BindAlertsToCase(c *echo.Context) error {
	identity, _ := middleware.GetIdentity(c)
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	var req bulkBindAlertsToCaseRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	caseID, err := uuid.Parse(strings.TrimSpace(req.CaseID))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid case_id")
	}
	alertIDs, err := parseUniqueAlertIDs(req.AlertIDs)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	exists, err := h.cases.ExistsInTenant(c.Request().Context(), caseID, tenantID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to validate case")
	}
	if !exists {
		return echo.NewHTTPError(http.StatusNotFound, "case not found in tenant")
	}

	existingAlerts, err := h.alerts.ListByIDs(c.Request().Context(), tenantID, alertIDs)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load alerts")
	}
	if len(existingAlerts) != len(alertIDs) {
		return echo.NewHTTPError(http.StatusNotFound, "some alerts not found in tenant")
	}

	updatedAlerts, err := h.alerts.BindToCase(c.Request().Context(), tenantID, alertIDs, &caseID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to bind alerts to case")
	}
	if len(updatedAlerts) != len(alertIDs) {
		return echo.NewHTTPError(http.StatusConflict, "failed to bind all selected alerts")
	}

	if h.caseEvents != nil {
		_, _ = h.caseEvents.Create(c.Request().Context(), repository.CreateCaseEventParams{
			TenantID:  tenantID,
			CaseID:    caseID,
			EventType: "alert_import",
			Title:     "Alert imported",
			Body:      fmt.Sprintf("Linked %d alert(s) to this case", len(updatedAlerts)),
			ActorID:   &identity.UserID,
			Metadata: map[string]any{
				"alert_ids": uuidListToStrings(alertIDs),
			},
		})
	}

	if h.search != nil {
		for idx := range updatedAlerts {
			item := updatedAlerts[idx]
			_ = h.search.IndexDocument(c.Request().Context(), "alerts", item.ID.String(), item)
		}
	}

	_ = h.audits.Log(c.Request().Context(), &tenantID, &identity.UserID, "alert_bind_case", "alert", nil, map[string]any{
		"case_id": caseID.String(),
		"count":   len(updatedAlerts),
	})
	return c.JSON(http.StatusOK, map[string]any{
		"case_id":       caseID.String(),
		"updated_count": len(updatedAlerts),
		"alerts":        updatedAlerts,
	})
}

func (h *Handler) CreateCaseFromAlerts(c *echo.Context) error {
	identity, _ := middleware.GetIdentity(c)
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	var req createCaseFromAlertsRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	alertIDs, err := parseUniqueAlertIDs(req.AlertIDs)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	alerts, err := h.alerts.ListByIDs(c.Request().Context(), tenantID, alertIDs)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load alerts")
	}
	if len(alerts) != len(alertIDs) {
		return echo.NewHTTPError(http.StatusNotFound, "some alerts not found in tenant")
	}

	title := strings.TrimSpace(req.Case.Title)
	if title == "" {
		title = fmt.Sprintf("Case from %d alerts", len(alerts))
	}
	description := strings.TrimSpace(req.Case.Description)
	if description == "" {
		description = buildAlertSummaryDescription(alerts)
	}
	source := strings.TrimSpace(req.Case.Source)
	if source == "" {
		source = deriveCaseSourceFromAlerts(alerts)
	}
	incidentType := strings.TrimSpace(req.Case.IncidentType)
	priority := strings.TrimSpace(strings.ToLower(req.Case.Priority))
	if priority == "" {
		priority = "medium"
	}
	impact := strings.TrimSpace(req.Case.Impact)
	severity := strings.TrimSpace(strings.ToLower(req.Case.Severity))
	if severity == "" {
		severity = maxSeverityFromAlerts(alerts)
	}
	status, err := h.resolveCaseStatus(c.Request().Context(), tenantID, req.Case.Status)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid case status for tenant")
	}
	tlp := strings.TrimSpace(strings.ToLower(req.Case.TLP))
	if tlp == "" {
		tlp = "amber"
	}
	pap := strings.TrimSpace(strings.ToLower(req.Case.PAP))
	if pap == "" {
		pap = "amber"
	}
	caseNumber := strings.TrimSpace(req.Case.CaseNumber)
	if caseNumber == "" {
		caseNumber = generateCaseNumber()
	}
	confidence := 0
	if req.Case.Confidence != nil {
		confidence = *req.Case.Confidence
	}
	if confidence < 0 || confidence > 100 {
		return echo.NewHTTPError(http.StatusBadRequest, "confidence must be in range 0..100")
	}
	detectedAt, err := normalizeOptionalRFC3339(req.Case.DetectedAt)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid detected_at timestamp")
	}
	occurredAt, err := normalizeOptionalRFC3339(req.Case.OccurredAt)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid occurred_at timestamp")
	}
	closedAt, err := normalizeOptionalRFC3339(req.Case.ClosedAt)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid closed_at timestamp")
	}
	resolutionSummary := strings.TrimSpace(req.Case.ResolutionSummary)
	var assignedTo *uuid.UUID
	if strings.TrimSpace(req.Case.AssignedTo) != "" {
		parsedAssignee, parseErr := uuid.Parse(strings.TrimSpace(req.Case.AssignedTo))
		if parseErr != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid assigned_to")
		}
		assignedTo = &parsedAssignee
	}
	if assignedTo == nil {
		if autoAssignee, autoErr := h.resolveCaseAutoAssignee(c.Request().Context(), tenantID, nil, source, incidentType); autoErr == nil {
			assignedTo = autoAssignee
		}
	}
	if limitErr := h.enforceAssigneeCaseWorkloadLimit(
		c.Request().Context(),
		tenantID,
		identity,
		assignedTo,
		nil,
		status,
	); limitErr != nil {
		return limitErr
	}

	createdCase, updatedAlerts, err := h.cases.CreateWithLinkedAlerts(c.Request().Context(), repository.CreateCaseParams{
		TenantID:          tenantID,
		CaseNumber:        caseNumber,
		Title:             title,
		Description:       description,
		Source:            source,
		IncidentType:      incidentType,
		Status:            status,
		Priority:          priority,
		Impact:            impact,
		Confidence:        confidence,
		Severity:          severity,
		TLP:               tlp,
		PAP:               pap,
		DetectedAt:        detectedAt,
		OccurredAt:        occurredAt,
		ClosedAt:          closedAt,
		ResolutionSummary: resolutionSummary,
		CreatedBy:         identity.UserID,
		AssignedTo:        assignedTo,
	}, alertIDs)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to create case from alerts")
	}

	if h.caseEvents != nil {
		_, _ = h.caseEvents.Create(c.Request().Context(), repository.CreateCaseEventParams{
			TenantID:  tenantID,
			CaseID:    createdCase.ID,
			EventType: "alert_import",
			Title:     "Alert imported",
			Body:      fmt.Sprintf("Created case from %d alert(s)", len(updatedAlerts)),
			ActorID:   &identity.UserID,
			Metadata: map[string]any{
				"alert_ids": uuidListToStrings(alertIDs),
			},
		})
	}

	if h.search != nil {
		_ = h.search.IndexDocument(c.Request().Context(), "cases", createdCase.ID.String(), createdCase)
		for idx := range updatedAlerts {
			item := updatedAlerts[idx]
			_ = h.search.IndexDocument(c.Request().Context(), "alerts", item.ID.String(), item)
		}
	}

	_ = h.audits.Log(c.Request().Context(), &tenantID, &identity.UserID, "case_create_from_alerts", "case", &createdCase.ID, map[string]any{
		"alert_count": len(updatedAlerts),
	})
	h.enqueueAIAgentQueueEvent(c.Request().Context(), tenantID, &identity.UserID, "case", createdCase.ID, aiAgentQueueSourceCaseFromAlerts)

	return c.JSON(http.StatusCreated, map[string]any{
		"case":          createdCase,
		"alerts":        updatedAlerts,
		"updated_count": len(updatedAlerts),
	})
}

func (h *Handler) ListCases(c *echo.Context) error {
	identity, hasIdentity := middleware.GetIdentity(c)
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	assigned, assignedTo, err := parseAssignedListFilter(c, identity, hasIdentity)
	if err != nil {
		return err
	}
	sortBy, sortOrder, err := parseListSortParams(c, listCaseSortFieldOptions, true)
	if err != nil {
		return err
	}
	search, err := parseListSearchParams(c)
	if err != nil {
		return err
	}
	page, pageSize, paged, err := parseListPageParams(c)
	if err != nil {
		return err
	}
	if paged {
		total, countErr := h.cases.CountByTenantWithAssignedAndSearch(c.Request().Context(), tenantID, assigned, assignedTo, search)
		if countErr != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to count cases")
		}
		offset := (page - 1) * pageSize
		items, listErr := h.cases.ListByTenantWithAssignedSortedAndSearch(
			c.Request().Context(),
			tenantID,
			pageSize,
			offset,
			assigned,
			assignedTo,
			sortBy,
			sortOrder,
			search,
		)
		if listErr != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to list cases")
		}
		filteredItems, filterErr := h.filterCasesByTagPolicy(c.Request().Context(), tenantID, identity, items)
		if filterErr != nil {
			return filterErr
		}
		return c.JSON(http.StatusOK, pagedListPayload(filteredItems, page, pageSize, total))
	}

	limit, offset, err := parseLegacyLimitOffset(c, 200, 200)
	if err != nil {
		return err
	}
	items, err := h.cases.ListByTenantWithAssignedSortedAndSearch(
		c.Request().Context(),
		tenantID,
		limit,
		offset,
		assigned,
		assignedTo,
		sortBy,
		sortOrder,
		search,
	)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list cases")
	}
	filteredItems, filterErr := h.filterCasesByTagPolicy(c.Request().Context(), tenantID, identity, items)
	if filterErr != nil {
		return filterErr
	}
	return c.JSON(http.StatusOK, filteredItems)
}

func parseListPageParams(c *echo.Context) (page int, pageSize int, ok bool, err error) {
	rawPage := strings.TrimSpace(c.QueryParam("page"))
	rawPageSize := strings.TrimSpace(c.QueryParam("page_size"))
	if rawPage == "" && rawPageSize == "" {
		return 0, 0, false, nil
	}

	page = 1
	if rawPage != "" {
		parsedPage, err := strconv.Atoi(rawPage)
		if err != nil || parsedPage <= 0 {
			return 0, 0, false, echo.NewHTTPError(http.StatusBadRequest, "invalid page")
		}
		page = parsedPage
	}

	pageSize = 30
	if rawPageSize != "" {
		parsedPageSize, err := strconv.Atoi(rawPageSize)
		if err != nil || parsedPageSize <= 0 {
			return 0, 0, false, echo.NewHTTPError(http.StatusBadRequest, "invalid page_size")
		}
		pageSize = parsedPageSize
	}
	if _, ok := listPageSizeOptions[pageSize]; !ok {
		return 0, 0, false, echo.NewHTTPError(http.StatusBadRequest, "page_size must be one of: 10,30,50,100")
	}
	return page, pageSize, true, nil
}

func parseListSortParams(c *echo.Context, allowed map[string]struct{}, allowCaseCustomField bool) (sortBy string, sortOrder string, err error) {
	sortBy = strings.ToLower(strings.TrimSpace(c.QueryParam("sort_by")))
	if sortBy == "" {
		sortBy = "updated_at"
	}
	if _, ok := allowed[sortBy]; !ok {
		if allowCaseCustomField {
			normalizedSortBy, customOK := normalizeCaseCustomSortBy(sortBy)
			if !customOK {
				return "", "", echo.NewHTTPError(http.StatusBadRequest, "invalid sort_by")
			}
			sortBy = normalizedSortBy
		} else {
			return "", "", echo.NewHTTPError(http.StatusBadRequest, "invalid sort_by")
		}
	}

	sortOrder = strings.ToLower(strings.TrimSpace(c.QueryParam("sort_order")))
	if sortOrder == "" {
		sortOrder = "desc"
	}
	switch sortOrder {
	case "asc", "desc":
	default:
		return "", "", echo.NewHTTPError(http.StatusBadRequest, "sort_order must be one of: asc, desc")
	}

	return sortBy, sortOrder, nil
}

func normalizeCaseCustomSortBy(raw string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if !strings.HasPrefix(normalized, "cf:") {
		return "", false
	}
	field := strings.TrimSpace(strings.TrimPrefix(normalized, "cf:"))
	if !caseCustomSortFieldPattern.MatchString(field) {
		return "", false
	}
	return "cf:" + field, true
}

func parseListSearchParams(c *echo.Context) (repository.ListSearchParams, error) {
	params := repository.ListSearchParams{
		Query:   strings.TrimSpace(c.QueryParam("q")),
		Exclude: strings.TrimSpace(c.QueryParam("q_not")),
		Mode:    strings.ToLower(strings.TrimSpace(c.QueryParam("search_mode"))),
		Logic:   strings.ToLower(strings.TrimSpace(c.QueryParam("search_logic"))),
	}
	if params.Mode == "" {
		params.Mode = "plain"
	}
	switch params.Mode {
	case "plain", "regex", "fulltext":
	default:
		return repository.ListSearchParams{}, echo.NewHTTPError(http.StatusBadRequest, "search_mode must be one of: plain, regex, fulltext")
	}
	if params.Logic == "" {
		params.Logic = "all"
	}
	switch params.Logic {
	case "all", "any":
	default:
		return repository.ListSearchParams{}, echo.NewHTTPError(http.StatusBadRequest, "search_logic must be one of: all, any")
	}
	if params.Mode == "regex" {
		if params.Query != "" {
			if _, err := regexp.Compile(params.Query); err != nil {
				return repository.ListSearchParams{}, echo.NewHTTPError(http.StatusBadRequest, "invalid regex in q")
			}
		}
		if params.Exclude != "" {
			if _, err := regexp.Compile(params.Exclude); err != nil {
				return repository.ListSearchParams{}, echo.NewHTTPError(http.StatusBadRequest, "invalid regex in q_not")
			}
		}
	}
	return params, nil
}

func parseAssignedListFilter(
	c *echo.Context,
	identity models.Identity,
	hasIdentity bool,
) (string, *uuid.UUID, error) {
	assigned := strings.ToLower(strings.TrimSpace(c.QueryParam("assigned")))
	if assigned == "" {
		assigned = "all"
	}
	switch assigned {
	case "all", "assigned", "unassigned", "mine":
	default:
		return "", nil, echo.NewHTTPError(http.StatusBadRequest, "assigned must be one of: all, assigned, unassigned, mine")
	}

	rawAssignedTo := strings.TrimSpace(c.QueryParam("assigned_to"))
	var assignedTo *uuid.UUID
	if rawAssignedTo != "" {
		parsed, err := uuid.Parse(rawAssignedTo)
		if err != nil {
			return "", nil, echo.NewHTTPError(http.StatusBadRequest, "invalid assigned_to")
		}
		assignedTo = &parsed
	}

	if assigned == "mine" {
		if !hasIdentity || identity.UserID == uuid.Nil {
			return "", nil, echo.NewHTTPError(http.StatusUnauthorized, "identity required for mine filter")
		}
		if assignedTo != nil && *assignedTo != identity.UserID {
			return "", nil, echo.NewHTTPError(http.StatusBadRequest, "assigned_to must match current user for mine filter")
		}
		userID := identity.UserID
		assignedTo = &userID
		assigned = "all"
	}

	if assigned == "unassigned" && assignedTo != nil {
		return "", nil, echo.NewHTTPError(http.StatusBadRequest, "assigned filter conflicts with assigned_to")
	}

	return assigned, assignedTo, nil
}

func parseLegacyLimitOffset(c *echo.Context, defaultLimit, maxLimit int) (limit int, offset int, err error) {
	limit = defaultLimit
	offset = 0
	if rawLimit := strings.TrimSpace(c.QueryParam("limit")); rawLimit != "" {
		parsedLimit, err := strconv.Atoi(rawLimit)
		if err != nil || parsedLimit <= 0 {
			return 0, 0, echo.NewHTTPError(http.StatusBadRequest, "invalid limit")
		}
		if parsedLimit > maxLimit {
			parsedLimit = maxLimit
		}
		limit = parsedLimit
	}
	if rawOffset := strings.TrimSpace(c.QueryParam("offset")); rawOffset != "" {
		parsedOffset, err := strconv.Atoi(rawOffset)
		if err != nil || parsedOffset < 0 {
			return 0, 0, echo.NewHTTPError(http.StatusBadRequest, "invalid offset")
		}
		offset = parsedOffset
	}
	return limit, offset, nil
}

func pagedListPayload(items any, page, pageSize, total int) map[string]any {
	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))
	if totalPages <= 0 {
		totalPages = 1
	}
	return map[string]any{
		"items":       items,
		"page":        page,
		"page_size":   pageSize,
		"total":       total,
		"total_pages": totalPages,
	}
}

// Sentinel errors for alerts/cases handlers (err113).
var (
	errAlertIDsMustNotBeEmpty = errors.New("alert_ids must not be empty")
)

func parseUniqueAlertIDs(raw []string) ([]uuid.UUID, error) {
	if len(raw) == 0 {
		return nil, errAlertIDsMustNotBeEmpty
	}
	seen := make(map[uuid.UUID]struct{}, len(raw))
	result := make([]uuid.UUID, 0, len(raw))
	for _, item := range raw {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		id, err := uuid.Parse(trimmed)
		if err != nil {
			return nil, fmt.Errorf("invalid alert id: %s", trimmed)
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	if len(result) == 0 {
		return nil, errAlertIDsMustNotBeEmpty
	}
	return result, nil
}

func uuidListToStrings(ids []uuid.UUID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, id.String())
	}
	return out
}

func deriveCaseSourceFromAlerts(alerts []models.Alert) string {
	if len(alerts) == 0 {
		return "alerts-bulk"
	}
	seen := make(map[string]struct{}, len(alerts))
	unique := make([]string, 0, len(alerts))
	for _, item := range alerts {
		source := strings.TrimSpace(item.Source)
		if source == "" {
			continue
		}
		key := strings.ToLower(source)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, source)
	}
	switch len(unique) {
	case 0:
		return "alerts-bulk"
	case 1:
		return unique[0]
	default:
		return "multi-source-alerts"
	}
}

func maxSeverityFromAlerts(alerts []models.Alert) string {
	if len(alerts) == 0 {
		return "medium"
	}
	rank := map[string]int{
		"critical": 4,
		"high":     3,
		"medium":   2,
		"low":      1,
	}
	maxScore := -1
	selected := "medium"
	for _, item := range alerts {
		sev := strings.TrimSpace(strings.ToLower(item.Severity))
		score, ok := rank[sev]
		if !ok {
			score = 0
		}
		if score > maxScore {
			maxScore = score
			selected = sev
		}
	}
	if selected == "" {
		return "medium"
	}
	return selected
}

func buildAlertSummaryDescription(alerts []models.Alert) string {
	if len(alerts) == 0 {
		return ""
	}
	ordered := make([]models.Alert, 0, len(alerts))
	ordered = append(ordered, alerts...)
	slices.SortFunc(ordered, func(a, b models.Alert) int {
		if a.UpdatedAt.Before(b.UpdatedAt) {
			return 1
		}
		if a.UpdatedAt.After(b.UpdatedAt) {
			return -1
		}
		return strings.Compare(a.ID.String(), b.ID.String())
	})
	builder := strings.Builder{}
	_, _ = fmt.Fprintf(&builder, "Created from %d selected alert(s).\n", len(ordered))
	limit := 5
	if len(ordered) < limit {
		limit = len(ordered)
	}
	for i := 0; i < limit; i++ {
		item := ordered[i]
		_, _ = fmt.Fprintf(&builder, "- [%s] %s (%s)\n", item.ID.String(), item.Title, item.Source)
	}
	if len(ordered) > limit {
		_, _ = fmt.Fprintf(&builder, "- ...and %d more", len(ordered)-limit)
	}
	return strings.TrimSpace(builder.String())
}

func (h *Handler) resolveCaseAutoAssignee(
	ctx context.Context,
	tenantID uuid.UUID,
	explicitAssignee *uuid.UUID,
	source string,
	incidentType string,
) (*uuid.UUID, error) {
	if explicitAssignee != nil {
		return explicitAssignee, nil
	}
	if h.users == nil || h.cases == nil {
		return nil, nil
	}
	tenantUsers, err := h.users.ListByTenantWithRole(ctx, tenantID, 500, 0)
	if err != nil {
		return nil, err
	}
	candidateAssignees := caseAutoAssignmentCandidates(tenantUsers)
	if len(candidateAssignees) == 0 {
		return nil, nil
	}

	openStatuses := h.listOpenCaseStatusCodes(ctx, tenantID)
	similarAssignee, err := h.cases.FindOpenSimilarAssignee(ctx, tenantID, candidateAssignees, source, incidentType, openStatuses)
	if err != nil {
		return nil, err
	}
	if similarAssignee != nil {
		return similarAssignee, nil
	}

	workload, err := h.cases.CountOpenByAssignees(ctx, tenantID, candidateAssignees, openStatuses)
	if err != nil {
		return nil, err
	}
	selected := candidateAssignees[0]
	minOpen := workload[selected]
	for _, candidateID := range candidateAssignees[1:] {
		openCount := workload[candidateID]
		if openCount < minOpen {
			selected = candidateID
			minOpen = openCount
		}
	}
	return &selected, nil
}

func (h *Handler) listOpenCaseStatusCodes(ctx context.Context, tenantID uuid.UUID) []string {
	statuses, err := h.loadCaseStatuses(ctx, tenantID)
	if err != nil {
		return nil
	}
	open := make([]string, 0, len(statuses))
	for _, item := range statuses {
		if !item.IsClosed {
			open = append(open, item.Code)
		}
	}
	return open
}

func caseAutoAssignmentCandidates(users []models.TenantUser) []uuid.UUID {
	analysts := make([]uuid.UUID, 0, len(users))
	admins := make([]uuid.UUID, 0, len(users))
	for _, item := range users {
		if !item.IsActive {
			continue
		}
		switch item.Role {
		case models.TenantRoleAnalyst:
			analysts = append(analysts, item.ID)
		case models.TenantRoleAdmin:
			admins = append(admins, item.ID)
		case models.TenantRoleViewer:
			// viewers cannot be auto-assigned
		}
	}
	if len(analysts) == 0 {
		analysts = admins
	}
	sort.SliceStable(analysts, func(i, j int) bool {
		return analysts[i].String() < analysts[j].String()
	})
	return analysts
}

func (h *Handler) CreateCase(c *echo.Context) error {
	identity, _ := middleware.GetIdentity(c)
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	var req createCaseRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Description = strings.TrimSpace(req.Description)
	req.CaseNumber = strings.TrimSpace(req.CaseNumber)
	req.Source = strings.TrimSpace(req.Source)
	req.IncidentType = strings.TrimSpace(req.IncidentType)
	req.Priority = strings.TrimSpace(strings.ToLower(req.Priority))
	req.Impact = strings.TrimSpace(req.Impact)
	req.ResolutionSummary = strings.TrimSpace(req.ResolutionSummary)
	if req.Title == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "title is required")
	}
	if req.CaseNumber == "" {
		req.CaseNumber = generateCaseNumber()
	}
	resolvedStatus, statusErr := h.resolveCaseStatus(c.Request().Context(), tenantID, req.Status)
	if statusErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid case status for tenant")
	}
	if req.Severity == "" {
		req.Severity = "medium"
	}
	if req.TLP == "" {
		req.TLP = "amber"
	}
	if req.PAP == "" {
		req.PAP = "amber"
	}
	if req.Priority == "" {
		req.Priority = "medium"
	}
	if req.Source == "" {
		req.Source = "manual"
	}
	confidence := 0
	if req.Confidence != nil {
		confidence = *req.Confidence
	}
	if confidence < 0 || confidence > 100 {
		return echo.NewHTTPError(http.StatusBadRequest, "confidence must be in range 0..100")
	}

	detectedAt, err := normalizeOptionalRFC3339(req.DetectedAt)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid detected_at timestamp")
	}
	occurredAt, err := normalizeOptionalRFC3339(req.OccurredAt)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid occurred_at timestamp")
	}
	closedAt, err := normalizeOptionalRFC3339(req.ClosedAt)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid closed_at timestamp")
	}

	var assignedTo *uuid.UUID
	if strings.TrimSpace(req.AssignedTo) != "" {
		parsedAssignee, parseErr := uuid.Parse(strings.TrimSpace(req.AssignedTo))
		if parseErr != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid assigned_to")
		}
		assignedTo = &parsedAssignee
	}
	if assignedTo == nil {
		if autoAssignee, autoErr := h.resolveCaseAutoAssignee(c.Request().Context(), tenantID, nil, req.Source, req.IncidentType); autoErr == nil {
			assignedTo = autoAssignee
		}
	}
	if limitErr := h.enforceAssigneeCaseWorkloadLimit(
		c.Request().Context(),
		tenantID,
		identity,
		assignedTo,
		nil,
		resolvedStatus,
	); limitErr != nil {
		return limitErr
	}
	if h.asyncOps != nil && h.asyncOps.Enabled() {
		detectedAtRaw := ""
		if detectedAt != nil {
			detectedAtRaw = *detectedAt
		}
		occurredAtRaw := ""
		if occurredAt != nil {
			occurredAtRaw = *occurredAt
		}
		closedAtRaw := ""
		if closedAt != nil {
			closedAtRaw = *closedAt
		}
		assignedToRaw := ""
		if assignedTo != nil {
			assignedToRaw = assignedTo.String()
		}
		caseID := uuid.New()
		payload, marshalErr := json.Marshal(asyncCaseCreatePayload{
			CaseID:            caseID.String(),
			CaseNumber:        req.CaseNumber,
			Title:             req.Title,
			Description:       req.Description,
			Source:            req.Source,
			IncidentType:      req.IncidentType,
			Status:            resolvedStatus,
			Priority:          req.Priority,
			Impact:            req.Impact,
			Confidence:        confidence,
			Severity:          req.Severity,
			TLP:               req.TLP,
			PAP:               req.PAP,
			DetectedAt:        detectedAtRaw,
			OccurredAt:        occurredAtRaw,
			ClosedAt:          closedAtRaw,
			ResolutionSummary: req.ResolutionSummary,
			AssignedTo:        assignedToRaw,
		})
		if marshalErr != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to queue case creation")
		}
		operation := AsyncOperation{
			OperationID: uuid.NewString(),
			Type:        AsyncOperationCaseCreate,
			TenantID:    tenantID.String(),
			ActorID:     identity.UserID.String(),
			ResourceID:  caseID.String(),
			RequestedAt: time.Now().UTC().Format(time.RFC3339Nano),
			Payload:     payload,
		}
		if recordErr := h.recordQueuedAsyncOperation(c.Request().Context(), operation, "case"); recordErr != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to register case creation operation")
		}
		if enqueueErr := h.asyncOps.Enqueue(c.Request().Context(), operation); enqueueErr != nil {
			h.markAsyncOperationFailed(c.Request().Context(), operation, enqueueErr)
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to enqueue case creation")
		}
		return c.JSON(http.StatusAccepted, queuedAsyncOperationResponse{
			OperationID:   operation.OperationID,
			Resource:      "case",
			ResourceID:    caseID.String(),
			OperationType: string(operation.Type),
			Status:        "queued",
		})
	}

	item, err := h.cases.Create(c.Request().Context(), repository.CreateCaseParams{
		TenantID:          tenantID,
		CaseNumber:        req.CaseNumber,
		Title:             req.Title,
		Description:       req.Description,
		Source:            req.Source,
		IncidentType:      req.IncidentType,
		Status:            resolvedStatus,
		Priority:          req.Priority,
		Impact:            req.Impact,
		Confidence:        confidence,
		Severity:          req.Severity,
		TLP:               req.TLP,
		PAP:               req.PAP,
		DetectedAt:        detectedAt,
		OccurredAt:        occurredAt,
		ClosedAt:          closedAt,
		ResolutionSummary: req.ResolutionSummary,
		CreatedBy:         identity.UserID,
		AssignedTo:        assignedTo,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to create case")
	}
	if h.search != nil {
		_ = h.search.IndexDocument(c.Request().Context(), "cases", item.ID.String(), item)
	}
	_ = h.audits.Log(c.Request().Context(), &tenantID, &identity.UserID, "case_create", "case", &item.ID, map[string]any{"title": item.Title})
	h.enqueueAIAgentQueueEvent(c.Request().Context(), tenantID, &identity.UserID, "case", item.ID, aiAgentQueueSourceAPI)
	return c.JSON(http.StatusCreated, item)
}

func (h *Handler) GetCase(c *echo.Context) error {
	tenantID, caseID, err := h.resolveCaseInTenant(c)
	if err != nil {
		return err
	}
	item, err := h.cases.GetByID(c.Request().Context(), tenantID, caseID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "case not found")
	}
	return c.JSON(http.StatusOK, item)
}

func (h *Handler) CopyCase(c *echo.Context) error {
	identity, _ := middleware.GetIdentity(c)
	tenantID, sourceCaseID, err := h.resolveCaseInTenant(c)
	if err != nil {
		return err
	}
	sourceCase, err := h.cases.GetByID(c.Request().Context(), tenantID, sourceCaseID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "case not found")
	}

	var req copyCaseRequest
	if bindErr := c.Bind(&req); bindErr != nil && !errors.Is(bindErr, io.EOF) {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	req.Title = strings.TrimSpace(req.Title)
	req.CaseNumber = strings.TrimSpace(req.CaseNumber)

	targetTitle := req.Title
	if targetTitle == "" {
		baseTitle := strings.TrimSpace(sourceCase.Title)
		if baseTitle == "" {
			targetTitle = "Case copy"
		} else {
			targetTitle = fmt.Sprintf("Copy of %s", baseTitle)
		}
	}
	targetCaseNumber := req.CaseNumber
	if targetCaseNumber == "" {
		targetCaseNumber = generateCaseNumber()
	}

	targetAssignedTo := sourceCase.AssignedTo
	if req.AssignedTo != nil {
		trimmed := strings.TrimSpace(*req.AssignedTo)
		if trimmed == "" {
			targetAssignedTo = nil
		} else {
			parsed, parseErr := uuid.Parse(trimmed)
			if parseErr != nil {
				return echo.NewHTTPError(http.StatusBadRequest, "invalid assigned_to")
			}
			targetAssignedTo = &parsed
		}
	}

	includeObservables := true
	if req.IncludeObservables != nil {
		includeObservables = *req.IncludeObservables
	}

	if limitErr := h.enforceAssigneeCaseWorkloadLimit(
		c.Request().Context(),
		tenantID,
		identity,
		targetAssignedTo,
		nil,
		sourceCase.Status,
	); limitErr != nil {
		return limitErr
	}

	if h.asyncOps != nil && h.asyncOps.Enabled() {
		targetCaseID := uuid.New()
		payload, marshalErr := json.Marshal(asyncCaseCreatePayload{
			CaseID:             targetCaseID.String(),
			CaseNumber:         targetCaseNumber,
			Title:              targetTitle,
			Description:        sourceCase.Description,
			Source:             sourceCase.Source,
			IncidentType:       sourceCase.IncidentType,
			Status:             sourceCase.Status,
			Priority:           sourceCase.Priority,
			Impact:             sourceCase.Impact,
			Confidence:         sourceCase.Confidence,
			Severity:           sourceCase.Severity,
			TLP:                sourceCase.TLP,
			PAP:                sourceCase.PAP,
			DetectedAt:         timePtrToRFC3339(sourceCase.DetectedAt),
			OccurredAt:         timePtrToRFC3339(sourceCase.OccurredAt),
			ClosedAt:           timePtrToRFC3339(sourceCase.ClosedAt),
			ResolutionSummary:  sourceCase.ResolutionSummary,
			AssignedTo:         optionalUUIDString(targetAssignedTo),
			SourceCaseID:       sourceCaseID.String(),
			IncludeObservables: includeObservables,
		})
		if marshalErr != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to queue case copy")
		}
		operation := AsyncOperation{
			OperationID: uuid.NewString(),
			Type:        AsyncOperationCaseCreate,
			TenantID:    tenantID.String(),
			ActorID:     identity.UserID.String(),
			ResourceID:  targetCaseID.String(),
			RequestedAt: time.Now().UTC().Format(time.RFC3339Nano),
			Payload:     payload,
		}
		if recordErr := h.recordQueuedAsyncOperation(c.Request().Context(), operation, "case"); recordErr != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to register case copy operation")
		}
		if enqueueErr := h.asyncOps.Enqueue(c.Request().Context(), operation); enqueueErr != nil {
			h.markAsyncOperationFailed(c.Request().Context(), operation, enqueueErr)
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to enqueue case copy")
		}
		return c.JSON(http.StatusAccepted, queuedAsyncOperationResponse{
			OperationID:   operation.OperationID,
			Resource:      "case",
			ResourceID:    targetCaseID.String(),
			OperationType: string(operation.Type),
			Status:        "queued",
		})
	}

	item, err := h.cases.Create(c.Request().Context(), repository.CreateCaseParams{
		TenantID:          tenantID,
		CaseNumber:        targetCaseNumber,
		Title:             targetTitle,
		Description:       sourceCase.Description,
		Source:            sourceCase.Source,
		IncidentType:      sourceCase.IncidentType,
		Status:            sourceCase.Status,
		Priority:          sourceCase.Priority,
		Impact:            sourceCase.Impact,
		Confidence:        sourceCase.Confidence,
		Severity:          sourceCase.Severity,
		TLP:               sourceCase.TLP,
		PAP:               sourceCase.PAP,
		DetectedAt:        timePtrToRFC3339Ptr(sourceCase.DetectedAt),
		OccurredAt:        timePtrToRFC3339Ptr(sourceCase.OccurredAt),
		ClosedAt:          timePtrToRFC3339Ptr(sourceCase.ClosedAt),
		ResolutionSummary: sourceCase.ResolutionSummary,
		CreatedBy:         identity.UserID,
		AssignedTo:        targetAssignedTo,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to copy case")
	}
	copiedObservables := 0
	if includeObservables {
		copiedObservables = h.copyCaseObservables(c.Request().Context(), tenantID, sourceCaseID, item.ID, identity.UserID)
	}
	if h.search != nil {
		_ = h.search.IndexDocument(c.Request().Context(), "cases", item.ID.String(), item)
	}
	_ = h.audits.Log(c.Request().Context(), &tenantID, &identity.UserID, "case_copy", "case", &item.ID, map[string]any{
		"source_case_id":      sourceCaseID.String(),
		"include_observables": includeObservables,
		"copied_observables":  copiedObservables,
	})
	h.enqueueAIAgentQueueEvent(c.Request().Context(), tenantID, &identity.UserID, "case", item.ID, aiAgentQueueSourceCaseCopy)
	return c.JSON(http.StatusCreated, item)
}

func (h *Handler) copyCaseObservables(ctx context.Context, tenantID, sourceCaseID, targetCaseID, actorID uuid.UUID) int {
	if h == nil || h.observables == nil {
		return 0
	}
	observables, err := h.observables.ListByCase(ctx, tenantID, sourceCaseID, 2000, 0)
	if err != nil {
		return 0
	}
	copied := 0
	for idx := range observables {
		sourceObservable := observables[idx]
		createdObservable, createErr := h.observables.Create(ctx, repository.CreateObservableParams{
			TenantID:  tenantID,
			CaseID:    targetCaseID,
			Type:      sourceObservable.Type,
			Value:     sourceObservable.Value,
			Verdict:   sourceObservable.Verdict,
			Source:    sourceObservable.Source,
			Tags:      sourceObservable.Tags,
			CreatedBy: actorID,
		})
		if createErr != nil {
			continue
		}
		copied++
		if h.search != nil {
			_ = h.search.IndexDocument(ctx, "observables", createdObservable.ID.String(), createdObservable)
		}
	}
	return copied
}

func (h *Handler) UpdateCase(c *echo.Context) error {
	identity, _ := middleware.GetIdentity(c)
	tenantID, caseID, err := h.resolveCaseInTenant(c)
	if err != nil {
		return err
	}
	currentCase, currentCaseErr := h.cases.GetByID(c.Request().Context(), tenantID, caseID)
	wasClosed := false
	if currentCaseErr == nil && currentCase != nil {
		wasClosed = h.isCaseStatusClosed(c.Request().Context(), tenantID, currentCase.Status)
	}

	var req updateCaseRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	expectedUpdatedAt, err := parseOptionalRFC3339Nano(req.ExpectedUpdatedAt)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid expected_updated_at timestamp")
	}
	if currentCase != nil && expectedUpdatedAt != nil && !matchesCaseUpdateVersion(currentCase.UpdatedAt, *expectedUpdatedAt) {
		return echo.NewHTTPError(http.StatusConflict, "case was updated by another user; reload and retry")
	}

	var assignedTo *uuid.UUID
	assignedToRaw := ""
	clearAssignedTo := false
	if req.AssignedTo != nil && strings.TrimSpace(*req.AssignedTo) != "" {
		parsed, parseErr := uuid.Parse(strings.TrimSpace(*req.AssignedTo))
		if parseErr != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid assigned_to")
		}
		assignedTo = &parsed
		assignedToRaw = parsed.String()
	} else if req.AssignedTo != nil {
		clearAssignedTo = true
	}
	if req.Confidence != nil && (*req.Confidence < 0 || *req.Confidence > 100) {
		return echo.NewHTTPError(http.StatusBadRequest, "confidence must be in range 0..100")
	}
	detectedAt, err := normalizeOptionalRFC3339Ptr(req.DetectedAt)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid detected_at timestamp")
	}
	occurredAt, err := normalizeOptionalRFC3339Ptr(req.OccurredAt)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid occurred_at timestamp")
	}
	closedAt, err := normalizeOptionalRFC3339Ptr(req.ClosedAt)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid closed_at timestamp")
	}

	var resolvedUpdateStatus *string
	if req.Status != nil {
		normalizedStatus, statusErr := h.resolveCaseStatus(c.Request().Context(), tenantID, *req.Status)
		if statusErr != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid case status for tenant")
		}
		resolvedUpdateStatus = &normalizedStatus
	}
	if transitionErr := h.validateCaseReviewAndClosureTransition(
		c.Request().Context(),
		tenantID,
		caseID,
		identity,
		currentCase,
		resolvedUpdateStatus,
		assignedTo,
		clearAssignedTo,
	); transitionErr != nil {
		return transitionErr
	}
	targetAssignee := (*uuid.UUID)(nil)
	if currentCase != nil {
		targetAssignee = currentCase.AssignedTo
	}
	switch {
	case assignedTo != nil:
		targetAssignee = assignedTo
	case clearAssignedTo:
		targetAssignee = nil
	}
	targetStatus := resolveTargetCaseStatus(currentCase, resolvedUpdateStatus)
	if limitErr := h.enforceAssigneeCaseWorkloadLimit(
		c.Request().Context(),
		tenantID,
		identity,
		targetAssignee,
		currentCase,
		targetStatus,
	); limitErr != nil {
		return limitErr
	}

	rewardCaseClosureEvent := false
	if !wasClosed && resolvedUpdateStatus != nil {
		rewardCaseClosureEvent = h.isCaseStatusClosed(c.Request().Context(), tenantID, *resolvedUpdateStatus)
	}

	if h.asyncOps != nil && h.asyncOps.Enabled() {
		expectedUpdatedAtRaw := ""
		if expectedUpdatedAt != nil {
			expectedUpdatedAtRaw = expectedUpdatedAt.UTC().Format(time.RFC3339Nano)
		}
		payload, marshalErr := json.Marshal(asyncCaseUpdatePayload{
			CaseID:                 caseID.String(),
			CaseNumber:             trimOptional(req.CaseNumber),
			Title:                  trimOptional(req.Title),
			Description:            trimOptional(req.Description),
			Source:                 trimOptional(req.Source),
			IncidentType:           trimOptional(req.IncidentType),
			Status:                 resolvedUpdateStatus,
			Priority:               lowerOptional(req.Priority),
			Impact:                 trimOptional(req.Impact),
			Confidence:             req.Confidence,
			Severity:               lowerOptional(req.Severity),
			TLP:                    lowerOptional(req.TLP),
			PAP:                    lowerOptional(req.PAP),
			DetectedAt:             detectedAt,
			OccurredAt:             occurredAt,
			ClosedAt:               closedAt,
			ExpectedUpdatedAt:      expectedUpdatedAtRaw,
			ResolutionSummary:      trimOptional(req.ResolutionSummary),
			AssignedTo:             assignedToRaw,
			ClearAssignedTo:        clearAssignedTo,
			RewardCaseClosureToID:  identity.UserID.String(),
			RewardCaseClosureEvent: rewardCaseClosureEvent,
		})
		if marshalErr != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to queue case update")
		}
		operation := AsyncOperation{
			OperationID: uuid.NewString(),
			Type:        AsyncOperationCaseUpdate,
			TenantID:    tenantID.String(),
			ActorID:     identity.UserID.String(),
			ResourceID:  caseID.String(),
			RequestedAt: time.Now().UTC().Format(time.RFC3339Nano),
			Payload:     payload,
		}
		if recordErr := h.recordQueuedAsyncOperation(c.Request().Context(), operation, "case"); recordErr != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to register case update operation")
		}
		if enqueueErr := h.asyncOps.Enqueue(c.Request().Context(), operation); enqueueErr != nil {
			h.markAsyncOperationFailed(c.Request().Context(), operation, enqueueErr)
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to enqueue case update")
		}
		return c.JSON(http.StatusAccepted, queuedAsyncOperationResponse{
			OperationID:   operation.OperationID,
			Resource:      "case",
			ResourceID:    caseID.String(),
			OperationType: string(operation.Type),
			Status:        "queued",
		})
	}

	item, err := h.cases.Update(c.Request().Context(), tenantID, caseID, repository.UpdateCaseParams{
		CaseNumber:        trimOptional(req.CaseNumber),
		Title:             trimOptional(req.Title),
		Description:       trimOptional(req.Description),
		Source:            trimOptional(req.Source),
		IncidentType:      trimOptional(req.IncidentType),
		Status:            resolvedUpdateStatus,
		Priority:          lowerOptional(req.Priority),
		Impact:            trimOptional(req.Impact),
		Confidence:        req.Confidence,
		Severity:          lowerOptional(req.Severity),
		TLP:               lowerOptional(req.TLP),
		PAP:               lowerOptional(req.PAP),
		DetectedAt:        detectedAt,
		OccurredAt:        occurredAt,
		ClosedAt:          closedAt,
		ResolutionSummary: trimOptional(req.ResolutionSummary),
		AssignedTo:        assignedTo,
		ClearAssignedTo:   clearAssignedTo,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to update case")
	}
	if h.search != nil {
		_ = h.search.IndexDocument(c.Request().Context(), "cases", item.ID.String(), item)
	}
	isClosed := h.isCaseStatusClosed(c.Request().Context(), tenantID, item.Status)
	if !wasClosed && isClosed {
		h.rewardCaseClosure(c.Request().Context(), tenantID, identity.UserID, item.ID, item.Severity)
	}
	_ = h.audits.Log(c.Request().Context(), &tenantID, &identity.UserID, "case_update", "case", &item.ID, nil)
	return c.JSON(http.StatusOK, item)
}

func (h *Handler) validateCaseReviewAndClosureTransition(
	ctx context.Context,
	tenantID uuid.UUID,
	caseID uuid.UUID,
	identity models.Identity,
	currentCase *models.Case,
	nextStatus *string,
	nextAssignedTo *uuid.UUID,
	clearAssignedTo bool,
) error {
	if nextStatus == nil {
		return nil
	}
	requestedStatus := strings.TrimSpace(strings.ToLower(*nextStatus))
	if requestedStatus == "" {
		return nil
	}
	isSenior := identity.IsPlatformAdmin || identity.TenantRole == models.TenantRoleAdmin
	isReview := requestedStatus == "review"
	isFinalClose := requestedStatus == "closed"

	if isFinalClose {
		return h.validateFinalCaseClosureTransition(ctx, tenantID, caseID, isSenior)
	}

	if !isReview || isSenior {
		return nil
	}

	nextAssignee := (*uuid.UUID)(nil)
	switch {
	case nextAssignedTo != nil:
		nextAssignee = nextAssignedTo
	case clearAssignedTo:
		nextAssignee = nil
	case currentCase != nil:
		nextAssignee = currentCase.AssignedTo
	}
	if nextAssignee == nil {
		return echo.NewHTTPError(http.StatusBadRequest, "review requires assignment to another analyst")
	}
	if *nextAssignee == identity.UserID {
		return echo.NewHTTPError(http.StatusBadRequest, "review assignee must be different from current analyst")
	}
	if h.memberships != nil {
		membership, err := h.memberships.Get(ctx, tenantID, *nextAssignee)
		if err != nil || !membership.IsActive {
			return echo.NewHTTPError(http.StatusBadRequest, "review assignee must be an active tenant member")
		}
		switch membership.Role {
		case models.TenantRoleAdmin, models.TenantRoleAnalyst:
		default:
			return echo.NewHTTPError(http.StatusBadRequest, "review assignee must have analyst role")
		}
	}
	return nil
}

func (h *Handler) validateFinalCaseClosureTransition(ctx context.Context, tenantID uuid.UUID, caseID uuid.UUID, isSenior bool) error {
	if !isSenior {
		return echo.NewHTTPError(http.StatusForbidden, "final case closure requires tenant admin or platform admin")
	}
	policy, err := h.resolveCaseClosureApprovalPolicy(ctx, tenantID, caseID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load case closure policy")
	}
	if policy.RequiredApprovals <= 0 && len(policy.ApproverIDs) == 0 {
		return nil
	}
	approvalsByActor, approvalsErr := h.collectCaseClosureApprovals(ctx, tenantID, caseID)
	if approvalsErr != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load case closure approvals")
	}
	if len(policy.ApproverIDs) > 0 {
		missingApprovers := make([]string, 0)
		for approverID := range policy.ApproverIDs {
			if _, ok := approvalsByActor[approverID]; !ok {
				missingApprovers = append(missingApprovers, approverID.String())
			}
		}
		if len(missingApprovers) > 0 {
			sort.Strings(missingApprovers)
			return echo.NewHTTPError(http.StatusConflict, fmt.Sprintf("final case closure requires approval from all configured approvers, missing %d", len(missingApprovers)))
		}
	}
	if len(approvalsByActor) < policy.RequiredApprovals {
		return echo.NewHTTPError(
			http.StatusConflict,
			fmt.Sprintf("final case closure requires %d approvals, got %d", policy.RequiredApprovals, len(approvalsByActor)),
		)
	}
	return nil
}

type caseClosureApprovalPolicy struct {
	RequiredApprovals int
	ApproverIDs       map[uuid.UUID]struct{}
}

func (h *Handler) resolveCaseClosureApprovalPolicy(ctx context.Context, tenantID, caseID uuid.UUID) (caseClosureApprovalPolicy, error) {
	policy := caseClosureApprovalPolicy{
		RequiredApprovals: 0,
		ApproverIDs:       make(map[uuid.UUID]struct{}),
	}
	if h.catalog == nil {
		return policy, nil
	}
	items, err := h.catalog.List(ctx, repository.CatalogListParams{
		Kind:     "case_meta",
		TenantID: &tenantID,
		RefID:    &caseID,
		Limit:    1,
	})
	if err != nil {
		return policy, fmt.Errorf("list case_meta for closure policy: %w", err)
	}
	if len(items) == 0 {
		return policy, nil
	}

	applyClosurePolicyFromMap(&policy, items[0].Data)
	if nested, ok := asMapStringAny(items[0].Data["custom_fields"]); ok {
		applyClosurePolicyFromMap(&policy, nested)
	}
	if nested, ok := asMapStringAny(items[0].Data["customFields"]); ok {
		applyClosurePolicyFromMap(&policy, nested)
	}

	if len(policy.ApproverIDs) > policy.RequiredApprovals {
		policy.RequiredApprovals = len(policy.ApproverIDs)
	}
	return policy, nil
}

func applyClosurePolicyFromMap(policy *caseClosureApprovalPolicy, data map[string]any) {
	if policy == nil || len(data) == 0 {
		return
	}
	if rawValue, ok := data[caseClosureApprovalsRequiredField]; ok {
		if parsed, parseOK := asInt(rawValue); parseOK && parsed >= 0 {
			policy.RequiredApprovals = parsed
		}
	}
	if rawValue, ok := data[caseClosureApproverIDsField]; ok {
		for _, approverID := range parseUUIDList(rawValue) {
			policy.ApproverIDs[approverID] = struct{}{}
		}
	}
}

func (h *Handler) collectCaseClosureApprovals(ctx context.Context, tenantID, caseID uuid.UUID) (map[uuid.UUID]models.CaseTimelineEvent, error) {
	events, err := h.caseEvents.ListByCase(ctx, tenantID, caseID, 2000, 0)
	if err != nil {
		return nil, fmt.Errorf("list case timeline events: %w", err)
	}
	approvalsByActor := make(map[uuid.UUID]models.CaseTimelineEvent)
	for _, event := range events {
		if strings.TrimSpace(strings.ToLower(event.EventType)) != caseClosureApprovalEventType {
			continue
		}
		if event.ActorID == nil || *event.ActorID == uuid.Nil {
			continue
		}
		actorID := *event.ActorID
		existing, exists := approvalsByActor[actorID]
		if !exists || event.CreatedAt.After(existing.CreatedAt) {
			approvalsByActor[actorID] = event
		}
	}
	return approvalsByActor, nil
}

func asMapStringAny(value any) (map[string]any, bool) {
	typed, ok := value.(map[string]any)
	if !ok || typed == nil {
		return nil, false
	}
	return typed, true
}

func asInt(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case json.Number:
		if i, err := typed.Int64(); err == nil {
			return int(i), true
		}
		if f, err := typed.Float64(); err == nil {
			return int(f), true
		}
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return 0, false
		}
		parsed, err := strconv.Atoi(trimmed)
		if err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func parseUUIDList(value any) []uuid.UUID {
	out := make([]uuid.UUID, 0)
	seen := make(map[uuid.UUID]struct{})
	push := func(raw string) {
		parsed, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			return
		}
		if _, exists := seen[parsed]; exists {
			return
		}
		seen[parsed] = struct{}{}
		out = append(out, parsed)
	}

	switch typed := value.(type) {
	case []string:
		for _, entry := range typed {
			push(entry)
		}
	case []any:
		for _, entry := range typed {
			push(fmt.Sprint(entry))
		}
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return out
		}
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			var parsed []string
			if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
				for _, entry := range parsed {
					push(entry)
				}
				return out
			}
		}
		for _, chunk := range strings.Split(trimmed, ",") {
			push(chunk)
		}
	}
	return out
}

func (h *Handler) DeleteCase(c *echo.Context) error {
	identity, _ := middleware.GetIdentity(c)
	tenantID, caseID, err := h.resolveCaseInTenant(c)
	if err != nil {
		return err
	}

	if h.asyncOps != nil && h.asyncOps.Enabled() {
		resp, queueErr := h.queueAsyncDeleteOperation(
			c.Request().Context(),
			tenantID,
			identity.UserID,
			"case",
			caseID,
			AsyncOperationCaseDelete,
		)
		if queueErr != nil {
			return queueErr
		}
		return c.JSON(http.StatusAccepted, resp)
	}

	linkedObservableIDs := make([]uuid.UUID, 0)
	linkedAlertIDs := make([]uuid.UUID, 0)
	if h.search != nil {
		if h.observables != nil {
			if linkedObservables, listErr := h.observables.ListByCase(c.Request().Context(), tenantID, caseID, 2000, 0); listErr == nil {
				for idx := range linkedObservables {
					linkedObservableIDs = append(linkedObservableIDs, linkedObservables[idx].ID)
				}
			}
		}
		if h.alerts != nil {
			if linkedAlerts, listErr := h.alerts.ListByCase(c.Request().Context(), tenantID, caseID, 2000, 0); listErr == nil {
				for idx := range linkedAlerts {
					linkedAlertIDs = append(linkedAlertIDs, linkedAlerts[idx].ID)
				}
			}
		}
	}

	deleted, err := h.cases.Delete(c.Request().Context(), tenantID, caseID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete case")
	}
	if !deleted {
		return echo.NewHTTPError(http.StatusNotFound, "case not found")
	}
	if h.search != nil {
		_ = h.search.DeleteDocument(c.Request().Context(), "cases", caseID.String())
		for _, observableID := range linkedObservableIDs {
			_ = h.search.DeleteDocument(c.Request().Context(), "observables", observableID.String())
		}
		if len(linkedAlertIDs) > 0 {
			if affectedAlerts, listErr := h.alerts.ListByIDs(c.Request().Context(), tenantID, linkedAlertIDs); listErr == nil {
				for idx := range affectedAlerts {
					item := affectedAlerts[idx]
					_ = h.search.IndexDocument(c.Request().Context(), "alerts", item.ID.String(), item)
				}
			}
		}
	}
	_ = h.audits.Log(c.Request().Context(), &tenantID, &identity.UserID, "case_delete", "case", &caseID, nil)
	return c.JSON(http.StatusOK, map[string]any{"success": true})
}

func optionalUUIDString(value *uuid.UUID) string {
	if value == nil {
		return ""
	}
	return value.String()
}

func timePtrToRFC3339(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func timePtrToRFC3339Ptr(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339)
	return &formatted
}

func parseOptionalRFC3339Nano(value *string) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, trimmed)
	if err != nil {
		return nil, err
	}
	normalized := parsed.UTC()
	return &normalized, nil
}

func matchesCaseUpdateVersion(current time.Time, expected time.Time) bool {
	return current.UTC().Truncate(time.Millisecond).Equal(expected.UTC().Truncate(time.Millisecond))
}

func normalizeOptionalRFC3339(value string) (*string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return nil, err
	}
	normalized := parsed.UTC().Format(time.RFC3339)
	return &normalized, nil
}

func normalizeOptionalRFC3339Ptr(value *string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	return normalizeOptionalRFC3339(*value)
}

func generateCaseNumber() string {
	return fmt.Sprintf("CASE-%s-%s", time.Now().UTC().Format("20060102"), strings.ToUpper(uuid.NewString()[:6]))
}

func (h *Handler) queueAsyncDeleteOperation(
	ctx context.Context,
	tenantID uuid.UUID,
	actorID uuid.UUID,
	resource string,
	resourceID uuid.UUID,
	opType AsyncOperationType,
) (*queuedAsyncOperationResponse, error) {
	payload, err := json.Marshal(asyncDeletePayload{ID: resourceID.String()})
	if err != nil {
		return nil, echo.NewHTTPError(
			http.StatusInternalServerError,
			fmt.Sprintf("failed to queue %s deletion", resource),
		)
	}
	operation := AsyncOperation{
		OperationID: uuid.NewString(),
		Type:        opType,
		TenantID:    tenantID.String(),
		ActorID:     actorID.String(),
		ResourceID:  resourceID.String(),
		RequestedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Payload:     payload,
	}
	if recordErr := h.recordQueuedAsyncOperation(ctx, operation, resource); recordErr != nil {
		return nil, echo.NewHTTPError(
			http.StatusInternalServerError,
			fmt.Sprintf("failed to register %s deletion operation", resource),
		)
	}
	if enqueueErr := h.asyncOps.Enqueue(ctx, operation); enqueueErr != nil {
		h.markAsyncOperationFailed(ctx, operation, enqueueErr)
		return nil, echo.NewHTTPError(
			http.StatusInternalServerError,
			fmt.Sprintf("failed to enqueue %s deletion", resource),
		)
	}
	return &queuedAsyncOperationResponse{
		OperationID:   operation.OperationID,
		Resource:      resource,
		ResourceID:    resourceID.String(),
		OperationType: string(operation.Type),
		Status:        "queued",
	}, nil
}
