package api

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"incidenthub/backend/internal/repository"

	"github.com/labstack/echo/v5"
)

func TestUploadUserMediaUpdatesProfile(t *testing.T) {
	env := newAPITestEnv(t)

	payload := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(payload)
	part, err := writer.CreateFormFile("file", "avatar.png")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, writeErr := part.Write([]byte("avatar-image-content")); writeErr != nil {
		t.Fatalf("write file content: %v", writeErr)
	}
	if closeErr := writer.Close(); closeErr != nil {
		t.Fatalf("close multipart writer: %v", closeErr)
	}

	c, rec := env.multipartContext("/api/v1/users/"+env.userID.String()+"/media/avatar/upload", payload, writer.FormDataContentType())
	setPath(c, "/api/v1/users/:id/media/:kind/upload", []string{"id", "kind"}, []string{env.userID.String(), "avatar"})
	setIdentity(c, env.identity)

	err = env.handler.UploadUserMedia(c)
	mustStatusOK(t, err, rec, http.StatusCreated)

	response := decodeBody[map[string]any](t, rec)
	if strings.TrimSpace(response["url"].(string)) == "" {
		t.Fatalf("expected upload url in response")
	}
	if !strings.Contains(response["url"].(string), "artifact.local/presigned") {
		t.Fatalf("expected presigned url, got %q", response["url"].(string))
	}
	if strings.TrimSpace(response["storage_uri"].(string)) == "" {
		t.Fatalf("expected storage uri in response")
	}

	user, err := env.users.GetByID(context.Background(), env.userID)
	if err != nil {
		t.Fatalf("load updated user: %v", err)
	}
	if !strings.HasPrefix(user.AvatarURL, "s3://"+env.cfg.S3.Bucket+"/users/") {
		t.Fatalf("expected avatar storage uri persisted, got %q", user.AvatarURL)
	}
}

func TestUploadAchievementIconAndUseInCatalog(t *testing.T) {
	env := newAPITestEnv(t)

	payload := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(payload)
	part, err := writer.CreateFormFile("file", "achievement-icon.png")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, writeErr := part.Write(buildPNGIconPayload(t, 96, 96)); writeErr != nil {
		t.Fatalf("write icon payload: %v", writeErr)
	}
	if closeErr := writer.Close(); closeErr != nil {
		t.Fatalf("close multipart writer: %v", closeErr)
	}

	uploadCtx, uploadRec := env.multipartContext("/api/v1/catalog/achievements/icon/upload", payload, writer.FormDataContentType())
	setPath(uploadCtx, "/api/v1/catalog/achievements/icon/upload", nil, nil)
	setIdentity(uploadCtx, env.identity)
	setTenant(uploadCtx, env.tenantID)

	err = env.handler.UploadAchievementIcon(uploadCtx)
	mustStatusOK(t, err, uploadRec, http.StatusCreated)

	uploadResponse := decodeBody[map[string]any](t, uploadRec)
	iconStorageURI := strings.TrimSpace(stringFromAny(uploadResponse["storage_uri"]))
	if !strings.HasPrefix(iconStorageURI, "s3://"+env.cfg.S3.Bucket+"/catalog/achievements/icons/"+env.tenantID.String()+"/") {
		t.Fatalf("unexpected icon storage uri: %q", iconStorageURI)
	}
	if !strings.Contains(strings.TrimSpace(stringFromAny(uploadResponse["icon_url"])), "artifact.local/presigned") {
		t.Fatalf("expected presigned icon_url in upload response")
	}

	createCtx, createRec := env.jsonContext(http.MethodPost, "/api/v1/catalog/achievements", map[string]any{
		"data": map[string]any{
			"name":        "S3 icon achievement",
			"description": "achievement icon upload flow",
			"icon":        iconStorageURI,
			"rarity":      "Rare",
			"xp_reward":   220,
		},
	})
	setPath(createCtx, "/api/v1/catalog/:kind", []string{"kind"}, []string{"achievements"})
	setIdentity(createCtx, env.identity)
	setTenant(createCtx, env.tenantID)
	err = env.handler.CreateCatalogItem(createCtx)
	mustStatusOK(t, err, createRec, http.StatusCreated)

	createdAchievement := decodeBody[map[string]any](t, createRec)
	if strings.TrimSpace(stringFromAny(createdAchievement["icon_storage_uri"])) != iconStorageURI {
		t.Fatalf("expected icon_storage_uri in achievement create response")
	}
	createdIconURL := strings.TrimSpace(stringFromAny(createdAchievement["icon"]))
	if !strings.Contains(createdIconURL, "artifact.local/presigned") {
		t.Fatalf("expected presigned icon in create response, got %q", createdIconURL)
	}
}

func TestUploadAchievementIconRejectsNonSquareImage(t *testing.T) {
	env := newAPITestEnv(t)

	payload := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(payload)
	part, err := writer.CreateFormFile("file", "achievement-icon.png")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, writeErr := part.Write(buildPNGIconPayload(t, 120, 80)); writeErr != nil {
		t.Fatalf("write icon payload: %v", writeErr)
	}
	if closeErr := writer.Close(); closeErr != nil {
		t.Fatalf("close multipart writer: %v", closeErr)
	}

	uploadCtx, _ := env.multipartContext("/api/v1/catalog/achievements/icon/upload", payload, writer.FormDataContentType())
	setPath(uploadCtx, "/api/v1/catalog/achievements/icon/upload", nil, nil)
	setIdentity(uploadCtx, env.identity)
	setTenant(uploadCtx, env.tenantID)

	err = env.handler.UploadAchievementIcon(uploadCtx)
	if err == nil {
		t.Fatalf("expected bad request for non-square icon")
	}
	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected echo HTTP error, got %T", err)
	}
	if httpErr.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: got=%d want=%d", httpErr.Code, http.StatusBadRequest)
	}
}

func TestCreateForumPostWithAttachments(t *testing.T) {
	env := newAPITestEnv(t)

	createdCase, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-ATTACH-01",
		Title:             "Forum attachments case",
		Description:       "Case used for forum attachment upload flow.",
		Source:            "manual",
		IncidentType:      "phishing",
		Status:            "new",
		Priority:          "medium",
		Impact:            "low",
		Confidence:        65,
		Severity:          "medium",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.userID,
		AssignedTo:        &env.userID,
	})
	if err != nil {
		t.Fatalf("create case: %v", err)
	}

	createThreadCtx, createThreadRec := env.jsonContext(http.MethodPost, "/api/v1/forum/threads", map[string]any{
		"case_id": createdCase.ID.String(),
		"title":   "Case attachments discussion",
		"status":  "In Progress",
	})
	setIdentity(createThreadCtx, env.identity)
	setTenant(createThreadCtx, env.tenantID)
	threadErr := env.handler.CreateForumThread(createThreadCtx)
	mustStatusOK(t, threadErr, createThreadRec, http.StatusCreated)
	threadPayload := decodeBody[map[string]any](t, createThreadRec)
	threadID := threadPayload["id"].(string)

	payload := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(payload)
	if writeErr := writer.WriteField("content", "Uploaded forensic screenshot"); writeErr != nil {
		t.Fatalf("write content field: %v", writeErr)
	}
	filePart, err := writer.CreateFormFile("files", "screenshot.png")
	if err != nil {
		t.Fatalf("create files form field: %v", err)
	}
	if _, writeErr := filePart.Write([]byte("png-binary-payload")); writeErr != nil {
		t.Fatalf("write file payload: %v", writeErr)
	}
	if closeErr := writer.Close(); closeErr != nil {
		t.Fatalf("close multipart writer: %v", closeErr)
	}

	c, rec := env.multipartContext("/api/v1/forum/threads/"+threadID+"/posts/upload", payload, writer.FormDataContentType())
	setPath(c, "/api/v1/forum/threads/:threadID/posts/upload", []string{"threadID"}, []string{threadID})
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)

	err = env.handler.CreateForumPostWithAttachments(c)
	mustStatusOK(t, err, rec, http.StatusCreated)

	postPayload := decodeBody[map[string]any](t, rec)
	rawAttachments, ok := postPayload["attachments"].([]any)
	if !ok || len(rawAttachments) != 1 {
		t.Fatalf("expected one attachment in created post response, got %#v", postPayload["attachments"])
	}
	firstAttachment, ok := rawAttachments[0].(map[string]any)
	if !ok {
		t.Fatalf("expected attachment payload map, got %#v", rawAttachments[0])
	}
	if strings.TrimSpace(firstAttachment["download_url"].(string)) == "" {
		t.Fatalf("expected presigned download_url in attachment payload")
	}

	threadCtx, threadRec := env.jsonContext(http.MethodGet, "/api/v1/forum/threads/"+threadID, nil)
	setPath(threadCtx, "/api/v1/forum/threads/:threadID", []string{"threadID"}, []string{threadID})
	setIdentity(threadCtx, env.identity)
	setTenant(threadCtx, env.tenantID)
	err = env.handler.GetForumThread(threadCtx)
	mustStatusOK(t, err, threadRec, http.StatusOK)

	threadResponse := decodeBody[map[string]any](t, threadRec)
	rawPosts, ok := threadResponse["posts"].([]any)
	if !ok || len(rawPosts) == 0 {
		t.Fatalf("expected forum posts in thread response")
	}
	firstPost, ok := rawPosts[0].(map[string]any)
	if !ok {
		t.Fatalf("expected post map payload, got %#v", rawPosts[0])
	}
	postAttachments, ok := firstPost["attachments"].([]any)
	if !ok || len(postAttachments) == 0 {
		t.Fatalf("expected post attachments in thread response")
	}
	postAttachment, ok := postAttachments[0].(map[string]any)
	if !ok {
		t.Fatalf("expected post attachment map, got %#v", postAttachments[0])
	}
	if strings.TrimSpace(postAttachment["download_url"].(string)) == "" {
		t.Fatalf("expected download_url in thread post attachment")
	}
}

func buildPNGIconPayload(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 214, B: 0, A: 255})
		}
	}
	buf := bytes.NewBuffer(nil)
	if err := png.Encode(buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}
