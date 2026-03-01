package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"incidenthub/backend/internal/models"

	"github.com/google/uuid"
)

func TestRefreshAndConnectorOperationalRepositories(t *testing.T) {
	env := newRepositoryTestEnv(t)
	ctx := context.Background()

	tokenKeep := models.RefreshToken{
		UserID:    env.userID,
		TokenHash: "refresh-keep",
		ExpiresAt: time.Now().UTC().Add(2 * time.Hour),
		UserAgent: "ua-keep",
		IPAddress: "10.0.0.1",
	}
	tokenRevoke := models.RefreshToken{
		UserID:    env.userID,
		TokenHash: "refresh-revoke",
		ExpiresAt: time.Now().UTC().Add(2 * time.Hour),
		UserAgent: "ua-revoke",
		IPAddress: "10.0.0.2",
	}
	tokenExpired := models.RefreshToken{
		UserID:    env.userID,
		TokenHash: "refresh-expired",
		ExpiresAt: time.Now().UTC().Add(-2 * time.Hour),
		UserAgent: "ua-expired",
		IPAddress: "10.0.0.3",
	}

	for _, token := range []models.RefreshToken{tokenKeep, tokenRevoke, tokenExpired} {
		if err := env.refresh.Create(ctx, token); err != nil {
			t.Fatalf("create refresh token %s: %v", token.TokenHash, err)
		}
	}

	activeToken, err := env.refresh.GetActiveByHash(ctx, tokenKeep.TokenHash)
	if err != nil {
		t.Fatalf("get active refresh token: %v", err)
	}
	if activeToken.TokenHash != tokenKeep.TokenHash {
		t.Fatalf("unexpected active refresh token hash")
	}

	listedTokens, err := env.refresh.ListByUser(ctx, env.userID, 100)
	if err != nil {
		t.Fatalf("list refresh tokens: %v", err)
	}
	if len(listedTokens) < 3 {
		t.Fatalf("expected at least 3 refresh tokens, got %d", len(listedTokens))
	}

	if revokeErr := env.refresh.RevokeByHash(ctx, tokenKeep.TokenHash); revokeErr != nil {
		t.Fatalf("revoke refresh token by hash: %v", revokeErr)
	}
	if _, getErr := env.refresh.GetActiveByHash(ctx, tokenKeep.TokenHash); getErr == nil {
		t.Fatalf("revoked token should not be active")
	}

	revokedRows, err := env.refresh.RevokeAllByUserExceptHash(ctx, env.userID, tokenRevoke.TokenHash)
	if err != nil {
		t.Fatalf("revoke refresh tokens except hash: %v", err)
	}
	if revokedRows < 0 {
		t.Fatalf("unexpected revoked rows: %d", revokedRows)
	}

	if revokeAllErr := env.refresh.RevokeAllByUser(ctx, env.userID); revokeAllErr != nil {
		t.Fatalf("revoke all refresh tokens by user: %v", revokeAllErr)
	}

	connectorItem, err := env.catalog.Create(ctx, CatalogCreateParams{
		TenantID:  &env.tenantID,
		Kind:      "inbound_connectors",
		OwnerID:   &env.userID,
		CreatedBy: &env.userID,
		Data: map[string]any{
			"name":      "http-ingest",
			"direction": "inbound",
		},
	})
	if err != nil {
		t.Fatalf("create inbound connector catalog item: %v", err)
	}

	acquiredFirst, err := env.ingest.TryAcquire(ctx, env.tenantID, connectorItem.ID, "external-1", "hash-1")
	if err != nil {
		t.Fatalf("try acquire first ingest state: %v", err)
	}
	if !acquiredFirst {
		t.Fatalf("first ingest acquire should succeed")
	}

	acquiredSecond, err := env.ingest.TryAcquire(ctx, env.tenantID, connectorItem.ID, "external-1", "hash-1")
	if err != nil {
		t.Fatalf("try acquire second ingest state: %v", err)
	}
	if acquiredSecond {
		t.Fatalf("second ingest acquire should be deduplicated")
	}

	alert, err := env.alerts.Create(ctx, CreateAlertParams{
		TenantID:    env.tenantID,
		Title:       "Connector alert",
		Description: "alert from connector",
		Source:      "connector",
		Status:      "new",
		Severity:    "medium",
		TLP:         "green",
		PAP:         "green",
		CreatedBy:   &env.userID,
	})
	if err != nil {
		t.Fatalf("create alert for ingest binding: %v", err)
	}

	if bindErr := env.ingest.BindAlert(ctx, env.tenantID, connectorItem.ID, "external-1", alert.ID); bindErr != nil {
		t.Fatalf("bind ingest alert: %v", bindErr)
	}
	if releaseErr := env.ingest.ReleaseOnFailure(ctx, env.tenantID, connectorItem.ID, "external-failed"); releaseErr != nil {
		t.Fatalf("release ingest state on failure: %v", releaseErr)
	}

	run, err := env.inboundRuns.Create(ctx, CreateInboundConnectorRunParams{
		TenantID:    env.tenantID,
		ConnectorID: connectorItem.ID,
		Trigger:     "manual",
	})
	if err != nil {
		t.Fatalf("create inbound run: %v", err)
	}

	if finishErr := env.inboundRuns.Finish(ctx, run.ID, env.tenantID, FinishInboundConnectorRunParams{
		Status:            "success",
		RecordsSeen:       12,
		AlertsCreated:     4,
		DuplicatesSkipped: 2,
		Errors:            0,
		Message:           "ok",
		Logs:              "run complete",
	}); finishErr != nil {
		t.Fatalf("finish inbound run: %v", finishErr)
	}

	latestRun, err := env.inboundRuns.GetLatestByConnector(ctx, env.tenantID, connectorItem.ID)
	if err != nil {
		t.Fatalf("get latest inbound run by connector: %v", err)
	}
	if latestRun.ID != run.ID {
		t.Fatalf("unexpected latest inbound run id")
	}

	runsByConnector, err := env.inboundRuns.ListByConnector(ctx, env.tenantID, connectorItem.ID, 50)
	if err != nil {
		t.Fatalf("list inbound runs by connector: %v", err)
	}
	if len(runsByConnector) == 0 {
		t.Fatalf("expected inbound runs by connector")
	}

	runsByTenant, err := env.inboundRuns.ListByTenant(ctx, env.tenantID, 50)
	if err != nil {
		t.Fatalf("list inbound runs by tenant: %v", err)
	}
	if len(runsByTenant) == 0 {
		t.Fatalf("expected inbound runs by tenant")
	}

	latestByTenant, err := env.inboundRuns.ListLatestByTenant(ctx, env.tenantID)
	if err != nil {
		t.Fatalf("list latest inbound runs by tenant: %v", err)
	}
	if len(latestByTenant) == 0 {
		t.Fatalf("expected latest inbound runs by tenant")
	}

	apiTokens := NewAPIAccessTokenRepository(env.pool)
	plainToken := "ihat_repo_test_token"
	digest := sha256.Sum256([]byte(plainToken))
	tokenHash := hex.EncodeToString(digest[:])
	createdToken, err := apiTokens.Create(ctx, CreateAPIAccessTokenParams{
		TenantID:    env.tenantID,
		Name:        "repo-token",
		Description: "repository token",
		TokenHash:   tokenHash,
		TokenPrefix: "ihat_repo",
		Scopes:      []string{"alerts:write", "alerts:write", "cases:read"},
		FullAccess:  false,
		CreatedBy:   env.userID,
	})
	if err != nil {
		t.Fatalf("create api access token: %v", err)
	}
	if len(createdToken.Scopes) != 2 {
		t.Fatalf("expected normalized scopes, got %v", createdToken.Scopes)
	}
	gotToken, err := apiTokens.GetActiveByHash(ctx, tokenHash)
	if err != nil {
		t.Fatalf("get api access token by hash: %v", err)
	}
	if gotToken.ID != createdToken.ID {
		t.Fatalf("unexpected active token id")
	}
	if touchErr := apiTokens.TouchLastUsed(ctx, gotToken.ID); touchErr != nil {
		t.Fatalf("touch api access token: %v", touchErr)
	}
	list, err := apiTokens.ListByTenant(ctx, env.tenantID, false, 20)
	if err != nil {
		t.Fatalf("list api access tokens: %v", err)
	}
	if len(list) == 0 {
		t.Fatalf("expected api access tokens")
	}
	if revokeErr := apiTokens.Revoke(ctx, env.tenantID, createdToken.ID); revokeErr != nil {
		t.Fatalf("revoke api access token: %v", revokeErr)
	}
	if _, getErr := apiTokens.GetActiveByHash(ctx, tokenHash); getErr == nil {
		t.Fatalf("revoked token should not be active")
	}

	asyncOps := NewAsyncOperationRepository(env.pool)
	operationID := uuid.New()
	resourceID := uuid.New()
	if createErr := asyncOps.Create(ctx, CreateAsyncOperationParams{
		OperationID:   operationID,
		TenantID:      env.tenantID,
		ActorID:       env.userID,
		Resource:      "alert",
		ResourceID:    &resourceID,
		OperationType: "alert.create",
		Status:        models.AsyncOperationStatusQueued,
		MaxAttempts:   5,
		Payload:       json.RawMessage(`{"title":"queued"}`),
	}); createErr != nil {
		t.Fatalf("create async operation: %v", createErr)
	}
	queuedOp, err := asyncOps.GetByID(ctx, env.tenantID, operationID)
	if err != nil {
		t.Fatalf("get queued async operation: %v", err)
	}
	if queuedOp.Status != models.AsyncOperationStatusQueued {
		t.Fatalf("expected queued status, got %q", queuedOp.Status)
	}
	if processingErr := asyncOps.MarkProcessing(ctx, operationID, 1); processingErr != nil {
		t.Fatalf("mark async operation processing: %v", processingErr)
	}
	if retryErr := asyncOps.MarkRetryQueued(ctx, operationID, 1, "temporary failure"); retryErr != nil {
		t.Fatalf("mark async operation retry queued: %v", retryErr)
	}
	if doneErr := asyncOps.MarkDone(ctx, operationID); doneErr != nil {
		t.Fatalf("mark async operation done: %v", doneErr)
	}
	doneOp, err := asyncOps.GetByID(ctx, env.tenantID, operationID)
	if err != nil {
		t.Fatalf("get done async operation: %v", err)
	}
	if doneOp.Status != models.AsyncOperationStatusDone {
		t.Fatalf("expected done status, got %q", doneOp.Status)
	}

	failedOperationID := uuid.New()
	if createErr := asyncOps.Create(ctx, CreateAsyncOperationParams{
		OperationID:   failedOperationID,
		TenantID:      env.tenantID,
		ActorID:       env.userID,
		Resource:      "case",
		OperationType: "case.delete",
		Status:        models.AsyncOperationStatusQueued,
		MaxAttempts:   3,
		Payload:       json.RawMessage(`{"id":"123"}`),
	}); createErr != nil {
		t.Fatalf("create failing async operation: %v", createErr)
	}
	if failedErr := asyncOps.MarkFailed(ctx, failedOperationID, 3, "terminal failure"); failedErr != nil {
		t.Fatalf("mark async operation failed: %v", failedErr)
	}
	failedOp, err := asyncOps.GetByID(ctx, env.tenantID, failedOperationID)
	if err != nil {
		t.Fatalf("get failed async operation: %v", err)
	}
	if failedOp.Status != models.AsyncOperationStatusFailed {
		t.Fatalf("expected failed status, got %q", failedOp.Status)
	}
}

func TestForumBindingAndAIRepositories(t *testing.T) {
	env := newRepositoryTestEnv(t)
	ctx := context.Background()

	caseItem := env.createCase("CASE-REPO-AI-001")

	threadItem, err := env.catalog.Create(ctx, CatalogCreateParams{
		TenantID:  &env.tenantID,
		Kind:      "forum_thread",
		OwnerID:   &env.userID,
		RefID:     &caseItem.ID,
		CreatedBy: &env.userID,
		Data: map[string]any{
			"title": "Case thread",
		},
	})
	if err != nil {
		t.Fatalf("create forum thread catalog item: %v", err)
	}
	connectorItem, err := env.catalog.Create(ctx, CatalogCreateParams{
		TenantID:  &env.tenantID,
		Kind:      "outbound_connectors",
		OwnerID:   &env.userID,
		CreatedBy: &env.userID,
		Data: map[string]any{
			"name":      "mock connector",
			"direction": "outbound",
			"channel":   "mock",
		},
	})
	if err != nil {
		t.Fatalf("create outbound connector item: %v", err)
	}
	recipientAliases := NewConnectorRecipientAliasRepository(env.pool)
	if upsertErr := recipientAliases.UpsertAliases(ctx, env.tenantID, connectorItem.ID, ConnectorRecipientAliasUpsertParams{
		Channel:     "telegram",
		RecipientID: "777",
		DisplayName: "SOC Contact",
		Metadata:    map[string]any{"provider": "telegram"},
		Aliases: map[string]string{
			"username":         "@soc-contact",
			"external_user_id": "9001",
		},
	}); upsertErr != nil {
		t.Fatalf("upsert connector recipient aliases: %v", upsertErr)
	}
	resolvedRecipientID, err := recipientAliases.ResolveRecipientID(ctx, env.tenantID, connectorItem.ID, "telegram", []string{"soc-contact"})
	if err != nil {
		t.Fatalf("resolve connector recipient aliases: %v", err)
	}
	if resolvedRecipientID != "777" {
		t.Fatalf("unexpected resolved recipient id: %s", resolvedRecipientID)
	}

	upsertedBinding, err := env.bindings.Upsert(ctx, env.tenantID, threadItem.ID, connectorItem.ID, ForumExternalBindingUpsertParams{
		ConversationID: "conv-1",
		Cursor:         "cursor-1",
		Metadata:       map[string]any{"provider": "mock"},
		LastSyncedAt:   true,
	})
	if err != nil {
		t.Fatalf("upsert forum external binding: %v", err)
	}
	if upsertedBinding.ID == uuid.Nil {
		t.Fatalf("binding id must not be nil")
	}

	gotBinding, err := env.bindings.GetByThreadAndConnector(ctx, env.tenantID, threadItem.ID, connectorItem.ID)
	if err != nil {
		t.Fatalf("get forum external binding: %v", err)
	}
	if gotBinding.ConversationID != "conv-1" {
		t.Fatalf("unexpected binding conversation id: %s", gotBinding.ConversationID)
	}

	threadBindings, err := env.bindings.ListByThread(ctx, env.tenantID, threadItem.ID)
	if err != nil {
		t.Fatalf("list forum bindings by thread: %v", err)
	}
	if len(threadBindings) == 0 {
		t.Fatalf("expected forum bindings by thread")
	}

	_, err = env.catalog.Create(ctx, CatalogCreateParams{
		TenantID:  &env.tenantID,
		Kind:      "forum_post",
		OwnerID:   &env.userID,
		RefID:     &threadItem.ID,
		CreatedBy: &env.userID,
		Data: map[string]any{
			"thread_id":   threadItem.ID.String(),
			"author_id":   env.userID.String(),
			"authorName":  "SOC Analyst",
			"content":     "Synchronized external update",
			"external_id": "ext-001",
			"timestamp":   time.Now().UTC().Format(time.RFC3339),
		},
	})
	if err != nil {
		t.Fatalf("create forum post with external id: %v", err)
	}

	existsByExternalID, err := env.catalog.ForumPostExistsByExternalID(ctx, env.tenantID, threadItem.ID, "ext-001")
	if err != nil {
		t.Fatalf("check forum post external id existence: %v", err)
	}
	if !existsByExternalID {
		t.Fatalf("expected forum post with external id to exist")
	}

	missingExternalID, err := env.catalog.ForumPostExistsByExternalID(ctx, env.tenantID, threadItem.ID, "ext-404")
	if err != nil {
		t.Fatalf("check forum post missing external id: %v", err)
	}
	if missingExternalID {
		t.Fatalf("expected forum post with missing external id to be absent")
	}

	defaultSession, err := env.aiChats.GetOrCreateDefaultSession(ctx, env.tenantID, env.userID)
	if err != nil {
		t.Fatalf("get/create default ai session: %v", err)
	}
	if defaultSession.ID == uuid.Nil {
		t.Fatalf("default ai session id must not be nil")
	}

	defaultSessionSecondCall, err := env.aiChats.GetOrCreateDefaultSession(ctx, env.tenantID, env.userID)
	if err != nil {
		t.Fatalf("get/create default ai session second call: %v", err)
	}
	if defaultSessionSecondCall.ID != defaultSession.ID {
		t.Fatalf("default ai session should be reused")
	}

	customSession, err := env.aiChats.CreateSession(ctx, CreateAIChatSessionParams{
		TenantID: env.tenantID,
		UserID:   env.userID,
		Title:    "Custom AI Session",
	})
	if err != nil {
		t.Fatalf("create custom ai chat session: %v", err)
	}
	if customSession.ID == uuid.Nil {
		t.Fatalf("custom ai session id must not be nil")
	}
	if customSession.IsDefault {
		t.Fatalf("custom ai session must not be default")
	}

	sessions, err := env.aiChats.ListSessions(ctx, env.tenantID, env.userID, 20)
	if err != nil {
		t.Fatalf("list ai chat sessions: %v", err)
	}
	if len(sessions) < 2 {
		t.Fatalf("expected at least 2 sessions, got %d", len(sessions))
	}
	if sessions[0].ID != customSession.ID {
		t.Fatalf("most recently created session should be first by sort order")
	}

	gotSession, err := env.aiChats.GetSessionByID(ctx, env.tenantID, env.userID, defaultSession.ID)
	if err != nil {
		t.Fatalf("get ai session by id: %v", err)
	}
	if gotSession.ID != defaultSession.ID {
		t.Fatalf("unexpected ai session id")
	}

	createdMessage, err := env.aiChats.CreateMessage(ctx, CreateAIChatMessageParams{
		TenantID:  env.tenantID,
		SessionID: defaultSession.ID,
		UserID:    env.userID,
		Role:      "user",
		Content:   "What happened in this case?",
		Sources:   []map[string]any{{"kind": "case", "id": caseItem.ID.String()}},
		Metadata:  map[string]any{"scope": "case"},
	})
	if err != nil {
		t.Fatalf("create ai chat message: %v", err)
	}
	if createdMessage.ID == uuid.Nil {
		t.Fatalf("ai chat message id must not be nil")
	}

	messages, err := env.aiChats.ListMessages(ctx, env.tenantID, env.userID, defaultSession.ID, 20)
	if err != nil {
		t.Fatalf("list ai chat messages: %v", err)
	}
	if len(messages) == 0 {
		t.Fatalf("expected ai chat messages")
	}

	analysis, err := env.caseAI.Create(ctx, CreateCaseAIAnalysisParams{
		TenantID:        env.tenantID,
		CaseID:          caseItem.ID,
		RequestedBy:     &env.userID,
		Model:           "sec-model",
		Status:          "completed",
		Verdict:         "suspicious",
		Confidence:      0.72,
		Summary:         "Suspicious activity detected",
		Recommendations: []string{"Collect additional endpoint telemetry"},
		Findings:        []string{"Unusual process tree"},
		Sources:         []map[string]any{{"kind": "observable", "id": "obs-1"}},
	})
	if err != nil {
		t.Fatalf("create case ai analysis: %v", err)
	}
	if analysis.ID == uuid.Nil {
		t.Fatalf("case ai analysis id must not be nil")
	}

	analysisList, err := env.caseAI.ListByCase(ctx, env.tenantID, caseItem.ID, 20)
	if err != nil {
		t.Fatalf("list case ai analyses: %v", err)
	}
	if len(analysisList) == 0 {
		t.Fatalf("expected case ai analyses")
	}
}
