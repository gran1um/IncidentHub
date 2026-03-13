package api

import (
	"context"
	"fmt"
	"incidenthub/backend/internal/models"
	"strings"
)

const s3StorageURIPrefix = "s3://"

func (h *Handler) storageURIForKey(key string) string {
	bucket := strings.TrimSpace(h.cfg.S3.Bucket)
	cleanKey := strings.TrimPrefix(strings.TrimSpace(key), "/")
	if bucket == "" || cleanKey == "" {
		return strings.TrimSpace(key)
	}
	return fmt.Sprintf("%s%s/%s", s3StorageURIPrefix, bucket, cleanKey)
}

func (h *Handler) storageKeyFromURI(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(trimmed, s3StorageURIPrefix) {
		return "", false
	}
	rest := strings.TrimPrefix(trimmed, s3StorageURIPrefix)
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 {
		return "", false
	}
	bucket := strings.TrimSpace(parts[0])
	key := strings.TrimPrefix(strings.TrimSpace(parts[1]), "/")
	if bucket == "" || key == "" {
		return "", false
	}
	if cfgBucket := strings.TrimSpace(h.cfg.S3.Bucket); cfgBucket != "" && !strings.EqualFold(cfgBucket, bucket) {
		return "", false
	}
	return key, true
}

func (h *Handler) resolveStorageURL(ctx context.Context, value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	key, ok := h.storageKeyFromURI(trimmed)
	if !ok || h.artifacts == nil {
		return trimmed
	}
	url, err := h.artifacts.PresignGet(ctx, key, h.cfg.Artifacts.PresignTTL)
	if err != nil || strings.TrimSpace(url) == "" {
		return trimmed
	}
	return url
}

func (h *Handler) enrichUserMediaURLs(ctx context.Context, user *models.User) {
	if user == nil {
		return
	}
	user.AvatarURL = h.resolveStorageURL(ctx, user.AvatarURL)
	user.CoverImageURL = h.resolveStorageURL(ctx, user.CoverImageURL)
}

func (h *Handler) enrichTenantUserMediaURLs(ctx context.Context, user *models.TenantUser) {
	if user == nil {
		return
	}
	user.AvatarURL = h.resolveStorageURL(ctx, user.AvatarURL)
	user.CoverImageURL = h.resolveStorageURL(ctx, user.CoverImageURL)
}

func normalizeUserMediaKind(raw string) (profileField string, objectPath string, ok bool) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	switch normalized {
	case "avatar", "profile", "photo":
		return "avatar", "avatar", true
	case "cover", "cover_image", "cover-image", "background", "banner":
		return "cover_image", "cover", true
	default:
		return "", "", false
	}
}
