package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // register JPEG decoder
	_ "image/png"  // register PNG decoder
	"incidenthub/backend/internal/middleware"
	"incidenthub/backend/internal/storage"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

const (
	achievementIconUploadMaxBytes int64 = 2 * 1024 * 1024
	achievementIconMinDimensionPx       = 64
	achievementIconMaxDimensionPx       = 512
)

//nolint:gochecknoglobals // Static allowlist for achievement icon upload content-types.
var achievementIconAllowedContentTypes = []string{
	"image/png",
	"image/jpeg",
	"image/svg+xml",
}

func (h *Handler) UploadAchievementIcon(c *echo.Context) error {
	identity, ok := middleware.GetIdentity(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	if h.artifacts == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "artifact storage unavailable")
	}

	maxBytes := h.maxAchievementIconBytes()
	req := c.Request()
	req.Body = http.MaxBytesReader(c.Response(), req.Body, maxBytes+1024)
	if err := req.ParseMultipartForm(maxBytes); err != nil {
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
		return echo.NewHTTPError(http.StatusRequestEntityTooLarge, "achievement icon exceeds size limit")
	}

	fileName := sanitizeAttachmentName(fileHeader.Filename)
	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(fileName)))
	contentType := strings.ToLower(strings.TrimSpace(fileHeader.Header.Get("Content-Type")))
	contentType = strings.TrimSpace(strings.Split(contentType, ";")[0])
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = strings.ToLower(strings.TrimSpace(mime.TypeByExtension(ext)))
	}
	if !slices.Contains(achievementIconAllowedContentTypes, contentType) {
		return echo.NewHTTPError(http.StatusBadRequest, "unsupported image format: use PNG, JPEG, or SVG")
	}

	rawFile, err := fileHeader.Open()
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to open uploaded file")
	}
	defer func() { _ = rawFile.Close() }()

	if contentType == "image/svg+xml" {
		sniff := make([]byte, 1024)
		readN, readErr := rawFile.Read(sniff)
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return echo.NewHTTPError(http.StatusBadRequest, "failed to inspect svg payload")
		}
		if !strings.Contains(strings.ToLower(string(sniff[:readN])), "<svg") {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid svg payload")
		}
		if _, err := rawFile.Seek(0, io.SeekStart); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "failed to read uploaded file")
		}
	} else {
		cfg, _, decodeErr := image.DecodeConfig(rawFile)
		if decodeErr != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid image payload")
		}
		if cfg.Width < achievementIconMinDimensionPx || cfg.Height < achievementIconMinDimensionPx {
			return echo.NewHTTPError(http.StatusBadRequest, "achievement icon must be at least 64x64")
		}
		if cfg.Width > achievementIconMaxDimensionPx || cfg.Height > achievementIconMaxDimensionPx {
			return echo.NewHTTPError(http.StatusBadRequest, "achievement icon must be at most 512x512")
		}
		if cfg.Width != cfg.Height {
			return echo.NewHTTPError(http.StatusBadRequest, "achievement icon must be square")
		}
		if _, err := rawFile.Seek(0, io.SeekStart); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "failed to read uploaded file")
		}
	}

	iconID := uuid.New()
	objectKey := fmt.Sprintf(
		"catalog/achievements/icons/%s/%s%s",
		tenantID.String(),
		iconID.String(),
		normalizeAchievementIconExtension(ext, contentType),
	)
	hash := sha256.New()
	uploadErr := h.artifacts.Upload(c.Request().Context(), objectKey, io.TeeReader(rawFile, hash), fileHeader.Size, contentType)
	if uploadErr != nil {
		if errors.Is(uploadErr, storage.ErrStorageDisabled) {
			return echo.NewHTTPError(http.StatusServiceUnavailable, "artifact storage disabled")
		}
		return echo.NewHTTPError(http.StatusBadGateway, "failed to upload achievement icon")
	}

	storageURI := h.storageURIForKey(objectKey)
	iconURL := h.resolveStorageURL(c.Request().Context(), storageURI)
	_ = h.audits.Log(c.Request().Context(), &tenantID, &identity.UserID, "achievement_icon_upload", "achievement", nil, map[string]any{
		"file_name":       fileName,
		"content_type":    contentType,
		"file_size_bytes": fileHeader.Size,
	})

	return c.JSON(http.StatusCreated, map[string]any{
		"icon":            storageURI,
		"icon_url":        iconURL,
		"url":             iconURL,
		"storage_key":     objectKey,
		"storage_uri":     storageURI,
		"checksum_sha256": hex.EncodeToString(hash.Sum(nil)),
		"content_type":    contentType,
		"file_size_bytes": fileHeader.Size,
		"criteria": map[string]any{
			"allowed_content_types": achievementIconAllowedContentTypes,
			"max_bytes":             maxBytes,
			"min_dimension_px":      achievementIconMinDimensionPx,
			"max_dimension_px":      achievementIconMaxDimensionPx,
			"square_required":       true,
		},
	})
}

func (h *Handler) enrichAchievementPayload(ctx context.Context, payload map[string]any) {
	if payload == nil {
		return
	}
	rawIcon := strings.TrimSpace(stringFromMap(payload, "icon"))
	if rawIcon == "" {
		return
	}
	if _, ok := h.storageKeyFromURI(rawIcon); ok {
		payload["icon_storage_uri"] = rawIcon
		iconURL := h.resolveStorageURL(ctx, rawIcon)
		if strings.TrimSpace(iconURL) != "" {
			payload["icon"] = iconURL
			payload["icon_url"] = iconURL
		}
		return
	}
	if isAchievementImageLikeValue(rawIcon) {
		payload["icon_url"] = rawIcon
	}
}

func (h *Handler) maxAchievementIconBytes() int64 {
	maxBytes := achievementIconUploadMaxBytes
	if genericMax := h.maxAttachmentBytes(); genericMax > 0 && genericMax < maxBytes {
		return genericMax
	}
	return maxBytes
}

func normalizeAchievementIconExtension(ext, contentType string) string {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/svg+xml":
		return ".svg"
	}
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case ".png", ".jpg", ".jpeg", ".svg":
		if ext == ".jpeg" {
			return ".jpg"
		}
		return ext
	default:
		return ".png"
	}
}

func isAchievementImageLikeValue(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return false
	}
	return strings.HasPrefix(normalized, "http://") ||
		strings.HasPrefix(normalized, "https://") ||
		strings.HasPrefix(normalized, "/") ||
		strings.HasPrefix(normalized, "data:image/") ||
		strings.HasPrefix(normalized, "blob:")
}
