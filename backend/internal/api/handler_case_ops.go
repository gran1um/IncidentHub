package api

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"incidenthub/backend/internal/middleware"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"
	"incidenthub/backend/internal/storage"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v5"
)

func (h *Handler) ListTasks(c *echo.Context) error {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}

	caseFilter := strings.TrimSpace(c.QueryParam("case_id"))
	var (
		items []models.Task
		err   error
	)
	if caseFilter != "" {
		caseID, parseErr := uuid.Parse(caseFilter)
		if parseErr != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid case_id")
		}
		items, err = h.tasks.ListByCase(c.Request().Context(), tenantID, caseID, 200, 0)
	} else {
		items, err = h.tasks.ListByTenant(c.Request().Context(), tenantID, 200, 0)
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list tasks")
	}
	return c.JSON(http.StatusOK, items)
}

func (h *Handler) CreateTask(c *echo.Context) error {
	var req createTaskRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Description = strings.TrimSpace(req.Description)
	if req.Title == "" || req.CaseID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "case_id and title are required")
	}

	caseID, err := uuid.Parse(req.CaseID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid case_id")
	}
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}

	var assigneeID *uuid.UUID
	if strings.TrimSpace(req.AssigneeID) != "" {
		parsed, parseErr := uuid.Parse(strings.TrimSpace(req.AssigneeID))
		if parseErr != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid assignee_id")
		}
		assigneeID = &parsed
	}
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = "new"
	}
	dueDate, dueDateSet, err := parseTaskDueDatePatch(req.DueDate)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid due_date timestamp")
	}
	if !dueDateSet {
		dueDate = nil
	}

	exists, err := h.cases.ExistsInTenant(c.Request().Context(), caseID, tenantID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to validate case")
	}
	if !exists {
		return echo.NewHTTPError(http.StatusNotFound, "case not found in tenant")
	}

	item, err := h.tasks.Create(c.Request().Context(), repository.CreateTaskParams{
		CaseID:      caseID,
		TenantID:    tenantID,
		Title:       req.Title,
		Description: req.Description,
		Status:      status,
		AssigneeID:  assigneeID,
		DueDate:     dueDate,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to create task")
	}
	return c.JSON(http.StatusCreated, item)
}

func (h *Handler) UpdateTask(c *echo.Context) error {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	taskID, err := uuid.Parse(c.Param("taskID"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid task id")
	}

	var req updateTaskRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	var assigneeID *uuid.UUID
	if req.AssigneeID != nil && strings.TrimSpace(*req.AssigneeID) != "" {
		parsed, parseErr := uuid.Parse(strings.TrimSpace(*req.AssigneeID))
		if parseErr != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid assignee_id")
		}
		assigneeID = &parsed
	}
	dueDate, dueDateSet, err := parseTaskDueDatePatch(req.DueDate)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid due_date timestamp")
	}

	item, err := h.tasks.Update(c.Request().Context(), tenantID, taskID, repository.UpdateTaskParams{
		Title:       trimOptional(req.Title),
		Description: trimOptional(req.Description),
		Status:      lowerOptional(req.Status),
		AssigneeID:  assigneeID,
		DueDate:     dueDate,
		DueDateSet:  dueDateSet,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to update task")
	}
	return c.JSON(http.StatusOK, item)
}

func parseTaskDueDatePatch(input *string) (*time.Time, bool, error) {
	if input == nil {
		return nil, false, nil
	}
	trimmed := strings.TrimSpace(*input)
	if trimmed == "" {
		return nil, true, nil
	}
	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return nil, false, err
	}
	normalized := parsed.UTC()
	return &normalized, true, nil
}

func (h *Handler) DeleteTask(c *echo.Context) error {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	taskID, err := uuid.Parse(c.Param("taskID"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid task id")
	}
	if err := h.tasks.Delete(c.Request().Context(), tenantID, taskID); err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "task not found")
	}
	return c.JSON(http.StatusOK, map[string]any{"success": true})
}

func (h *Handler) ListCaseObservables(c *echo.Context) error {
	tenantID, caseID, err := h.resolveCaseInTenant(c)
	if err != nil {
		return err
	}
	items, err := h.observables.ListByCase(c.Request().Context(), tenantID, caseID, 300, 0)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list observables")
	}
	return c.JSON(http.StatusOK, items)
}

func (h *Handler) CreateCaseObservable(c *echo.Context) error {
	identity, _ := middleware.GetIdentity(c)
	tenantID, caseID, err := h.resolveCaseInTenant(c)
	if err != nil {
		return err
	}

	var req createObservableRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	req.Type = strings.TrimSpace(strings.ToLower(req.Type))
	req.Value = strings.TrimSpace(req.Value)
	req.Verdict = strings.TrimSpace(strings.ToLower(req.Verdict))
	req.Source = strings.TrimSpace(req.Source)
	if req.Type == "" || req.Value == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "type and value are required")
	}
	if req.Verdict == "" {
		req.Verdict = "unknown"
	}

	item, err := h.observables.Create(c.Request().Context(), repository.CreateObservableParams{
		TenantID:  tenantID,
		CaseID:    caseID,
		Type:      req.Type,
		Value:     req.Value,
		Verdict:   req.Verdict,
		Source:    req.Source,
		Tags:      normalizeTags(req.Tags),
		CreatedBy: identity.UserID,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to create observable")
	}

	if h.search != nil {
		_ = h.search.IndexDocument(c.Request().Context(), "observables", item.ID.String(), item)
	}
	_ = h.audits.Log(c.Request().Context(), &tenantID, &identity.UserID, "observable_create", "observable", &item.ID, map[string]any{"case_id": caseID.String(), "type": item.Type})
	return c.JSON(http.StatusCreated, item)
}

func (h *Handler) UpdateCaseObservable(c *echo.Context) error {
	tenantID, caseID, err := h.resolveCaseInTenant(c)
	if err != nil {
		return err
	}
	observableID, err := uuid.Parse(c.Param("observableID"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid observable id")
	}

	var req updateObservableRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	item, err := h.observables.Update(c.Request().Context(), tenantID, caseID, observableID, repository.UpdateObservableParams{
		Type:    lowerOptional(req.Type),
		Value:   trimOptional(req.Value),
		Verdict: lowerOptional(req.Verdict),
		Source:  trimOptional(req.Source),
		Tags:    req.Tags,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to update observable")
	}
	if h.search != nil {
		_ = h.search.IndexDocument(c.Request().Context(), "observables", item.ID.String(), item)
	}
	return c.JSON(http.StatusOK, item)
}

func (h *Handler) DeleteCaseObservable(c *echo.Context) error {
	tenantID, caseID, err := h.resolveCaseInTenant(c)
	if err != nil {
		return err
	}
	observableID, err := uuid.Parse(c.Param("observableID"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid observable id")
	}
	if err := h.observables.Delete(c.Request().Context(), tenantID, caseID, observableID); err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "observable not found")
	}
	if h.search != nil {
		_ = h.search.DeleteDocument(c.Request().Context(), "observables", observableID.String())
	}
	return c.JSON(http.StatusOK, map[string]any{"success": true})
}

func (h *Handler) ListCaseEvents(c *echo.Context) error {
	tenantID, caseID, err := h.resolveCaseInTenant(c)
	if err != nil {
		return err
	}
	items, err := h.caseEvents.ListByCase(c.Request().Context(), tenantID, caseID, 300, 0)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list case events")
	}
	return c.JSON(http.StatusOK, items)
}

func (h *Handler) CreateCaseEvent(c *echo.Context) error {
	identity, _ := middleware.GetIdentity(c)
	tenantID, caseID, err := h.resolveCaseInTenant(c)
	if err != nil {
		return err
	}

	var req createCaseEventRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	req.EventType = strings.TrimSpace(strings.ToLower(req.EventType))
	req.Title = strings.TrimSpace(req.Title)
	req.Body = strings.TrimSpace(req.Body)
	if req.EventType == "" {
		req.EventType = "note"
	}
	if req.Title == "" {
		req.Title = "Note"
	}
	if req.Body == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "body is required")
	}

	item, err := h.caseEvents.Create(c.Request().Context(), repository.CreateCaseEventParams{
		TenantID:  tenantID,
		CaseID:    caseID,
		EventType: req.EventType,
		Title:     req.Title,
		Body:      req.Body,
		ActorID:   &identity.UserID,
		Metadata:  req.Metadata,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to create case event")
	}

	_ = h.audits.Log(c.Request().Context(), &tenantID, &identity.UserID, "case_event_create", "case_event", &item.ID, map[string]any{"case_id": caseID.String(), "event_type": item.EventType})
	return c.JSON(http.StatusCreated, item)
}

func (h *Handler) ListCasePages(c *echo.Context) error {
	tenantID, caseID, err := h.resolveCaseInTenant(c)
	if err != nil {
		return err
	}
	items, err := h.casePages.ListByCase(c.Request().Context(), tenantID, caseID, 200, 0)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list case pages")
	}
	return c.JSON(http.StatusOK, items)
}

func (h *Handler) CreateCasePage(c *echo.Context) error {
	identity, _ := middleware.GetIdentity(c)
	tenantID, caseID, err := h.resolveCaseInTenant(c)
	if err != nil {
		return err
	}

	var req createCasePageRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Body = strings.TrimSpace(req.Body)
	if req.Title == "" || req.Body == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "title and body are required")
	}

	item, err := h.casePages.Create(c.Request().Context(), repository.CreateCasePageParams{
		TenantID:  tenantID,
		CaseID:    caseID,
		Title:     req.Title,
		Body:      req.Body,
		CreatedBy: identity.UserID,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to create case page")
	}

	_ = h.audits.Log(c.Request().Context(), &tenantID, &identity.UserID, "case_page_create", "case_page", &item.ID, map[string]any{"case_id": caseID.String(), "title": item.Title})
	return c.JSON(http.StatusCreated, item)
}

func (h *Handler) ListCaseAttachments(c *echo.Context) error {
	tenantID, caseID, err := h.resolveCaseInTenant(c)
	if err != nil {
		return err
	}
	items, err := h.attachments.ListByCase(c.Request().Context(), tenantID, caseID, 200, 0)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list case attachments")
	}
	return c.JSON(http.StatusOK, items)
}

func (h *Handler) CreateCaseAttachment(c *echo.Context) error {
	identity, _ := middleware.GetIdentity(c)
	tenantID, caseID, err := h.resolveCaseInTenant(c)
	if err != nil {
		return err
	}

	var req createCaseAttachmentRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	req.FileName = strings.TrimSpace(req.FileName)
	req.ContentType = strings.TrimSpace(req.ContentType)
	req.StorageKey = strings.TrimSpace(req.StorageKey)
	req.ChecksumSHA256 = strings.TrimSpace(strings.ToLower(req.ChecksumSHA256))
	if req.FileName == "" || req.StorageKey == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "file_name and storage_key are required")
	}
	if req.ContentType == "" {
		req.ContentType = "application/octet-stream"
	}
	if req.FileSizeBytes < 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "file_size_bytes cannot be negative")
	}

	item, err := h.attachments.Create(c.Request().Context(), repository.CreateAttachmentParams{
		TenantID:       tenantID,
		CaseID:         caseID,
		FileName:       req.FileName,
		ContentType:    req.ContentType,
		FileSizeBytes:  req.FileSizeBytes,
		StorageKey:     req.StorageKey,
		ChecksumSHA256: req.ChecksumSHA256,
		UploadedBy:     identity.UserID,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to create case attachment")
	}

	_ = h.audits.Log(c.Request().Context(), &tenantID, &identity.UserID, "case_attachment_create", "case_attachment", &item.ID, map[string]any{"case_id": caseID.String(), "file_name": item.FileName})
	return c.JSON(http.StatusCreated, item)
}

func (h *Handler) UploadCaseAttachment(c *echo.Context) error {
	identity, _ := middleware.GetIdentity(c)
	tenantID, caseID, err := h.resolveCaseInTenant(c)
	if err != nil {
		return err
	}
	if h.artifacts == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "artifact storage unavailable")
	}

	maxBytes := h.maxAttachmentBytes()
	req := c.Request()
	req.Body = http.MaxBytesReader(c.Response(), req.Body, maxBytes+1024)
	if multipartErr := req.ParseMultipartForm(maxBytes); multipartErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid multipart form")
	}
	if req.MultipartForm != nil {
		defer func() { _ = req.MultipartForm.RemoveAll() }()
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "file field is required")
	}
	if fileHeader.Size <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "empty file is not allowed")
	}
	if fileHeader.Size > maxBytes {
		return echo.NewHTTPError(http.StatusRequestEntityTooLarge, "attachment exceeds configured size limit")
	}

	fileName := sanitizeAttachmentName(fileHeader.Filename)
	contentType := strings.TrimSpace(fileHeader.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(fileName))
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	rawFile, err := fileHeader.Open()
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to open uploaded file")
	}
	defer func() { _ = rawFile.Close() }()

	attachmentID := uuid.New()
	objectKey := fmt.Sprintf("tenant/%s/case/%s/attachments/%s/%s", tenantID.String(), caseID.String(), attachmentID.String(), fileName)

	hash := sha256.New()
	if uploadErr := h.artifacts.Upload(c.Request().Context(), objectKey, io.TeeReader(rawFile, hash), fileHeader.Size, contentType); uploadErr != nil {
		if errors.Is(err, storage.ErrStorageDisabled) {
			return echo.NewHTTPError(http.StatusServiceUnavailable, "artifact storage disabled")
		}
		return echo.NewHTTPError(http.StatusBadGateway, "failed to upload artifact")
	}

	item, err := h.attachments.Create(c.Request().Context(), repository.CreateAttachmentParams{
		TenantID:       tenantID,
		CaseID:         caseID,
		FileName:       fileName,
		ContentType:    contentType,
		FileSizeBytes:  fileHeader.Size,
		StorageKey:     objectKey,
		ChecksumSHA256: hex.EncodeToString(hash.Sum(nil)),
		UploadedBy:     identity.UserID,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to persist attachment metadata")
	}

	downloadURL, _ := h.artifacts.PresignGet(c.Request().Context(), item.StorageKey, h.cfg.Artifacts.PresignTTL)
	_ = h.audits.Log(c.Request().Context(), &tenantID, &identity.UserID, "case_attachment_upload", "case_attachment", &item.ID, map[string]any{
		"case_id":   caseID.String(),
		"file_name": item.FileName,
		"size":      item.FileSizeBytes,
	})

	return c.JSON(http.StatusCreated, map[string]any{
		"attachment":   item,
		"download_url": downloadURL,
	})
}

func (h *Handler) GetCaseAttachmentDownloadURL(c *echo.Context) error {
	tenantID, caseID, err := h.resolveCaseInTenant(c)
	if err != nil {
		return err
	}
	if h.artifacts == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "artifact storage unavailable")
	}

	attachmentID, err := uuid.Parse(strings.TrimSpace(c.Param("attachmentID")))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid attachmentID path param")
	}

	item, err := h.attachments.GetByID(c.Request().Context(), tenantID, caseID, attachmentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, "attachment not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load attachment")
	}

	url, err := h.artifacts.PresignGet(c.Request().Context(), item.StorageKey, h.cfg.Artifacts.PresignTTL)
	if err != nil {
		if errors.Is(err, storage.ErrStorageDisabled) {
			return echo.NewHTTPError(http.StatusServiceUnavailable, "artifact storage disabled")
		}
		return echo.NewHTTPError(http.StatusBadGateway, "failed to generate download url")
	}

	return c.JSON(http.StatusOK, map[string]any{
		"attachment_id": item.ID,
		"url":           url,
		"expires_in":    int(h.cfg.Artifacts.PresignTTL.Seconds()),
	})
}
