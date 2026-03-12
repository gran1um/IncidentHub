package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"incidenthub/backend/internal/middleware"
	"incidenthub/backend/internal/repository"
	"incidenthub/backend/internal/storage"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

func (h *Handler) CreateForumPostWithAttachments(c *echo.Context) error {
	identity, _ := middleware.GetIdentity(c)
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	if h.artifacts == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "artifact storage unavailable")
	}

	threadID, err := uuid.Parse(strings.TrimSpace(c.Param("threadID")))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid thread id")
	}
	if _, getErr := h.catalog.GetByID(c.Request().Context(), "forum_thread", threadID, &tenantID); getErr != nil {
		return echo.NewHTTPError(http.StatusNotFound, "thread not found")
	}

	maxBytes := h.maxAttachmentBytes()
	req := c.Request()
	req.Body = http.MaxBytesReader(c.Response(), req.Body, maxBytes+1024)
	if parseErr := req.ParseMultipartForm(maxBytes); parseErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid multipart form")
	}
	if req.MultipartForm != nil {
		defer func() { _ = req.MultipartForm.RemoveAll() }()
	}

	content := strings.TrimSpace(req.FormValue("content"))
	authorID := identity.UserID
	if authorRaw := strings.TrimSpace(req.FormValue("author_id")); authorRaw != "" {
		parsed, parseErr := uuid.Parse(authorRaw)
		if parseErr != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid author_id")
		}
		authorID = parsed
	}

	fileHeaders := req.MultipartForm.File["files"]
	if len(fileHeaders) == 0 {
		fileHeaders = req.MultipartForm.File["file"]
	}
	if len(fileHeaders) == 0 && content == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "content or files are required")
	}

	attachments := make([]map[string]any, 0, len(fileHeaders))
	for _, fileHeader := range fileHeaders {
		if fileHeader == nil {
			continue
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

		rawFile, openErr := fileHeader.Open()
		if openErr != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "failed to open uploaded file")
		}

		attachmentID := uuid.New()
		objectKey := fmt.Sprintf(
			"tenant/%s/forum/%s/attachments/%s/%s",
			tenantID.String(),
			threadID.String(),
			attachmentID.String(),
			fileName,
		)
		hash := sha256.New()
		uploadErr := h.artifacts.Upload(c.Request().Context(), objectKey, io.TeeReader(rawFile, hash), fileHeader.Size, contentType)
		closeErr := rawFile.Close()
		if uploadErr != nil {
			if errors.Is(uploadErr, storage.ErrStorageDisabled) {
				return echo.NewHTTPError(http.StatusServiceUnavailable, "artifact storage disabled")
			}
			return echo.NewHTTPError(http.StatusBadGateway, "failed to upload artifact")
		}
		if closeErr != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "failed to process uploaded file")
		}

		attachment := map[string]any{
			"id":              attachmentID.String(),
			"file_name":       fileName,
			"content_type":    contentType,
			"size_bytes":      fileHeader.Size,
			"checksum_sha256": hex.EncodeToString(hash.Sum(nil)),
			"storage_key":     objectKey,
			"storage_uri":     h.storageURIForKey(objectKey),
		}
		attachments = append(attachments, attachment)
	}

	if content == "" && len(attachments) > 0 {
		content = "Attachment upload"
	}

	data := map[string]any{
		"thread_id":  threadID.String(),
		"author_id":  authorID.String(),
		"content":    content,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"tenant_id":  tenantID.String(),
		"authorName": identity.Username,
	}
	if len(attachments) > 0 {
		data["attachments"] = attachments
	}

	item, err := h.catalog.Create(c.Request().Context(), repository.CatalogCreateParams{
		TenantID:  &tenantID,
		Kind:      "forum_post",
		OwnerID:   &authorID,
		RefID:     &threadID,
		Data:      data,
		CreatedBy: &identity.UserID,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to create post")
	}

	payload := catalogItemToPayload(*item)
	h.enrichForumPostPayload(c.Request().Context(), payload)
	return c.JSON(http.StatusCreated, payload)
}

func (h *Handler) enrichForumPostPayload(ctx context.Context, payload map[string]any) {
	if payload == nil {
		return
	}
	rawAttachments, ok := payload["attachments"]
	if !ok {
		return
	}

	attachments := normalizeAttachmentPayload(rawAttachments)
	if len(attachments) == 0 {
		return
	}

	for _, attachment := range attachments {
		if attachment == nil {
			continue
		}
		if h.artifacts == nil {
			continue
		}
		key := firstNonEmptyString(
			stringFromMap(attachment, "storage_key", "storageKey"),
			func() string {
				if uri := stringFromMap(attachment, "storage_uri", "storageUri"); uri != "" {
					if parsedKey, parsed := h.storageKeyFromURI(uri); parsed {
						return parsedKey
					}
				}
				return ""
			}(),
		)
		if key == "" {
			continue
		}
		url, err := h.artifacts.PresignGet(ctx, key, h.cfg.Artifacts.PresignTTL)
		if err != nil || strings.TrimSpace(url) == "" {
			continue
		}
		attachment["download_url"] = url
		attachment["url"] = url
		if forumAttachmentIsImage(attachment) {
			attachment["preview_url"] = url
		}
	}

	payload["attachments"] = attachments
}

func normalizeAttachmentPayload(raw any) []map[string]any {
	switch value := raw.(type) {
	case []map[string]any:
		out := make([]map[string]any, 0, len(value))
		for _, item := range value {
			if item == nil {
				continue
			}
			out = append(out, item)
		}
		return out
	case []any:
		out := make([]map[string]any, 0, len(value))
		for _, item := range value {
			typed, ok := item.(map[string]any)
			if !ok || typed == nil {
				continue
			}
			out = append(out, typed)
		}
		return out
	default:
		return nil
	}
}

func forumAttachmentIsImage(attachment map[string]any) bool {
	contentType := strings.TrimSpace(strings.ToLower(stringFromMap(attachment, "content_type", "contentType")))
	if strings.HasPrefix(contentType, "image/") {
		return true
	}
	name := strings.TrimSpace(strings.ToLower(stringFromMap(attachment, "file_name", "fileName")))
	switch filepath.Ext(name) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".svg":
		return true
	default:
		return false
	}
}
