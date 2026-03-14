package api

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"incidenthub/backend/internal/middleware"
	"incidenthub/backend/internal/storage"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v5"
)

func (h *Handler) UploadUserMedia(c *echo.Context) error {
	identity, ok := middleware.GetIdentity(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}
	if h.artifacts == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "artifact storage unavailable")
	}

	userID, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user id")
	}
	if userID != identity.UserID && !identity.IsPlatformAdmin {
		return echo.NewHTTPError(http.StatusForbidden, "forbidden")
	}

	profileField, objectPath, ok := normalizeUserMediaKind(c.Param("kind"))
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "unsupported media kind")
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

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "file field is required")
	}
	if fileHeader.Size <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "empty file is not allowed")
	}
	if fileHeader.Size > maxBytes {
		return echo.NewHTTPError(http.StatusRequestEntityTooLarge, "file exceeds configured size limit")
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

	mediaID := uuid.New()
	objectKey := fmt.Sprintf("users/%s/profile/%s/%s/%s", userID.String(), objectPath, mediaID.String(), fileName)

	hash := sha256.New()
	if uploadErr := h.artifacts.Upload(c.Request().Context(), objectKey, io.TeeReader(rawFile, hash), fileHeader.Size, contentType); uploadErr != nil {
		if errors.Is(uploadErr, storage.ErrStorageDisabled) {
			return echo.NewHTTPError(http.StatusServiceUnavailable, "artifact storage disabled")
		}
		return echo.NewHTTPError(http.StatusBadGateway, "failed to upload artifact")
	}

	storageURI := h.storageURIForKey(objectKey)
	var avatarURL, coverImageURL *string
	if profileField == "avatar" {
		avatarURL = &storageURI
	} else {
		coverImageURL = &storageURI
	}

	updated, err := h.users.UpdateProfile(c.Request().Context(), userID, nil, nil, nil, avatarURL, coverImageURL, nil)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, "user not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update user media")
	}
	h.enrichUserMediaURLs(c.Request().Context(), updated)

	_ = h.audits.Log(c.Request().Context(), nil, &identity.UserID, "user_media_upload", "user", &userID, map[string]any{
		"kind":      profileField,
		"file_name": fileName,
		"size":      fileHeader.Size,
	})

	return c.JSON(http.StatusCreated, map[string]any{
		"kind":            profileField,
		"storage_key":     objectKey,
		"storage_uri":     storageURI,
		"checksum_sha256": hex.EncodeToString(hash.Sum(nil)),
		"content_type":    contentType,
		"file_size_bytes": fileHeader.Size,
		"url":             h.resolveStorageURL(c.Request().Context(), storageURI),
		"user":            updated,
	})
}
