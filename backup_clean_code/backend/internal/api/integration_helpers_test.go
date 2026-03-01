package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"incidenthub/backend/internal/ai"
	"incidenthub/backend/internal/auth"
	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"
	"incidenthub/backend/internal/search"
	"incidenthub/backend/internal/security"
	"incidenthub/backend/internal/storage"
	"incidenthub/backend/internal/testutil"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v5"
)

type apiSearchStub struct {
	enabled   bool
	searches  []apiSearchCall
	indexes   []apiIndexCall
	deletes   []apiDeleteCall
	hits      []search.Hit
	searchErr error
	indexErr  error
	deleteErr error
}

type apiSearchCall struct {
	TenantID string
	Query    string
	Kinds    []string
	Size     int
}

type apiIndexCall struct {
	Kind string
	ID   string
	Doc  any
}

type apiDeleteCall struct {
	Kind string
	ID   string
}

func (s *apiSearchStub) IndexDocument(_ context.Context, kind, id string, doc any) error {
	if s.indexErr != nil {
		return s.indexErr
	}
	s.indexes = append(s.indexes, apiIndexCall{Kind: kind, ID: id, Doc: doc})
	return nil
}

func (s *apiSearchStub) DeleteDocument(_ context.Context, kind, id string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.deletes = append(s.deletes, apiDeleteCall{Kind: kind, ID: id})
	return nil
}

func (s *apiSearchStub) Search(_ context.Context, tenantID, query string, kinds []string, size int) ([]search.Hit, error) {
	if s.searchErr != nil {
		return nil, s.searchErr
	}
	kindsCopy := make([]string, len(kinds))
	copy(kindsCopy, kinds)
	s.searches = append(s.searches, apiSearchCall{
		TenantID: tenantID,
		Query:    query,
		Kinds:    kindsCopy,
		Size:     size,
	})
	out := make([]search.Hit, len(s.hits))
	copy(out, s.hits)
	return out, nil
}

func (s *apiSearchStub) Ping(context.Context) error { return nil }
func (s *apiSearchStub) Enabled() bool              { return s.enabled }

type artifactStorageStub struct {
	uploads    map[string][]byte
	uploadErr  error
	presign    string
	presignErr error
	healthErr  error
}

type asyncOpsQueueStub struct {
	enabled    bool
	enqueueErr error
	operations []AsyncOperation
}

func (s *asyncOpsQueueStub) Enabled() bool {
	return s != nil && s.enabled
}

func (s *asyncOpsQueueStub) Enqueue(_ context.Context, operation AsyncOperation) error {
	if s.enqueueErr != nil {
		return s.enqueueErr
	}
	s.operations = append(s.operations, operation)
	return nil
}

type notificationQueueStub struct {
	enabled    bool
	enqueueErr error
	events     []NotificationDeliveryEvent
}

func (s *notificationQueueStub) Enabled() bool {
	return s != nil && s.enabled
}

func (s *notificationQueueStub) Enqueue(_ context.Context, event NotificationDeliveryEvent) error {
	if s.enqueueErr != nil {
		return s.enqueueErr
	}
	s.events = append(s.events, event)
	return nil
}

func (s *artifactStorageStub) Upload(_ context.Context, key string, body io.Reader, _ int64, _ string) error {
	if s.uploadErr != nil {
		return s.uploadErr
	}
	payload, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	if s.uploads == nil {
		s.uploads = make(map[string][]byte, 4)
	}
	s.uploads[key] = payload
	return nil
}

func (s *artifactStorageStub) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	if s.presignErr != nil {
		return "", s.presignErr
	}
	if strings.TrimSpace(s.presign) != "" {
		return s.presign + "?key=" + key, nil
	}
	return "https://example.local/download?key=" + key, nil
}

func (s *artifactStorageStub) Health(context.Context) error {
	return s.healthErr
}

type apiTestEnv struct {
	t *testing.T

	pool        *pgxpool.Pool
	cfg         config.App
	e           *echo.Echo
	handler     *Handler
	searchStub  *apiSearchStub
	storageStub *artifactStorageStub

	users                *repository.UserRepository
	tenants              *repository.TenantRepository
	memberships          *repository.MembershipRepository
	alerts               *repository.AlertRepository
	cases                *repository.CaseRepository
	tasks                *repository.TaskRepository
	observables          *repository.ObservableRepository
	caseEvents           *repository.CaseEventRepository
	casePages            *repository.CasePageRepository
	attachments          *repository.AttachmentRepository
	catalog              *repository.CatalogRepository
	notificationBots     *repository.TelegramNotificationBotRepository
	notificationSettings *repository.UserNotificationSettingsRepository
	aiChats              *repository.AIChatRepository
	caseAI               *repository.CaseAIAnalysisRepository
	aiAgentQueue         *repository.AIAgentQueueRepository
	aiAgentWorkloads     *repository.AIAgentWorkloadRepository

	tenantID      uuid.UUID
	secondTenant  uuid.UUID
	userID        uuid.UUID
	secondUserID  uuid.UUID
	identity      models.Identity
	platformAdmin models.Identity
}

func newAPITestEnv(t *testing.T) *apiTestEnv {
	t.Helper()

	pool := testutil.OpenTestPool(t)
	testutil.ResetPublicTables(t, pool)

	cfg := config.App{
		Env: "test",
		HTTP: config.HTTPConfig{
			Addr:            "127.0.0.1:0",
			ReadTimeout:     2 * time.Second,
			WriteTimeout:    5 * time.Second,
			ShutdownTimeout: 2 * time.Second,
			SecureCookies:   false,
		},
		Auth: config.AuthConfig{
			JWTSecret:  "integration-test-secret",
			Issuer:     "incidenthub-test",
			AccessTTL:  15 * time.Minute,
			RefreshTTL: 12 * time.Hour,
			CookieName: "ih_refresh_token",
		},
		Security: config.SecurityConfig{
			PasswordMinLength: 8,
		},
		Artifacts: config.ArtifactsConfig{
			MaxUploadMB: 4,
			PresignTTL:  3 * time.Minute,
		},
		Elastic: config.ElasticConfig{
			Enabled: false,
		},
		S3: config.S3Config{
			Enabled: false,
			Bucket:  "incidenthub-artifacts",
		},
		AI: config.AIConfig{
			Enabled:         false,
			Model:           "sec-fallback-rag",
			TopK:            6,
			Timeout:         5 * time.Second,
			HistoryMessages: 20,
			MaxContextChars: 5000,
			MCPEnabled:      true,
			MCPTools:        "open_case_statuses,cases_in_work,recent_cases,recent_alerts,dashboard_stats",
			MCPPerToolLimit: 5,
		},
	}

	users := repository.NewUserRepository(pool)
	tenants := repository.NewTenantRepository(pool)
	memberships := repository.NewMembershipRepository(pool)
	refresh := repository.NewRefreshTokenRepository(pool)
	alerts := repository.NewAlertRepository(pool)
	cases := repository.NewCaseRepository(pool)
	tasks := repository.NewTaskRepository(pool)
	observables := repository.NewObservableRepository(pool)
	caseEvents := repository.NewCaseEventRepository(pool)
	casePages := repository.NewCasePageRepository(pool)
	attachments := repository.NewAttachmentRepository(pool)
	catalog := repository.NewCatalogRepository(pool)
	system := repository.NewSystemRepository(pool)
	audits := repository.NewAuditRepository(pool)
	connectorIngest := repository.NewConnectorIngestStateRepository(pool)
	inboundRuns := repository.NewInboundConnectorRunRepository(pool)
	workflowRuns := repository.NewWorkflowRunRepository(pool)
	forumBindings := repository.NewForumExternalBindingRepository(pool)
	notificationBots := repository.NewTelegramNotificationBotRepository(pool)
	notificationSettings := repository.NewUserNotificationSettingsRepository(pool)
	apiTokens := repository.NewAPIAccessTokenRepository(pool)
	asyncOperations := repository.NewAsyncOperationRepository(pool)
	aiChats := repository.NewAIChatRepository(pool)
	caseAI := repository.NewCaseAIAnalysisRepository(pool)
	experience := repository.NewExperienceRepository(pool)
	aiAgentQueue := repository.NewAIAgentQueueRepository(pool)
	aiAgentWorkloads := repository.NewAIAgentWorkloadRepository(pool)
	connectorHubExecutions := repository.NewConnectorHubExecutionRepository(pool)

	searchStub := &apiSearchStub{enabled: true}
	storageStub := &artifactStorageStub{presign: "https://artifact.local/presigned"}
	jwtService := auth.New(cfg.Auth)
	aiService := ai.NewService(cfg.AI, searchStub)

	tenant, err := tenants.Create(context.Background(), repository.CreateTenantParams{
		Slug:        "tenant-main",
		Name:        "Tenant Main",
		Description: "main tenant",
		MaxUsers:    100,
		IsActive:    true,
	})
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	secondTenant, err := tenants.Create(context.Background(), repository.CreateTenantParams{
		Slug:        "tenant-second",
		Name:        "Tenant Second",
		Description: "second tenant",
		MaxUsers:    100,
		IsActive:    true,
	})
	if err != nil {
		t.Fatalf("create second tenant: %v", err)
	}

	passwordHash, err := security.HashPassword("Password123!")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	user, err := users.Create(context.Background(), repository.CreateUserParams{
		Username:        "analyst-main",
		Email:           "analyst-main@example.com",
		FullName:        "Main Analyst",
		PasswordHash:    passwordHash,
		IsPlatformAdmin: false,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	secondUser, err := users.Create(context.Background(), repository.CreateUserParams{
		Username:        "analyst-second",
		Email:           "analyst-second@example.com",
		FullName:        "Second Analyst",
		PasswordHash:    passwordHash,
		IsPlatformAdmin: false,
	})
	if err != nil {
		t.Fatalf("create second user: %v", err)
	}
	platform, err := users.Create(context.Background(), repository.CreateUserParams{
		Username:        "platform-admin",
		Email:           "platform-admin@example.com",
		FullName:        "Platform Admin",
		PasswordHash:    passwordHash,
		IsPlatformAdmin: true,
	})
	if err != nil {
		t.Fatalf("create platform admin: %v", err)
	}

	if err := memberships.Upsert(context.Background(), tenant.ID, user.ID, models.TenantRoleAdmin); err != nil {
		t.Fatalf("upsert membership main user: %v", err)
	}
	if err := memberships.Upsert(context.Background(), tenant.ID, secondUser.ID, models.TenantRoleAnalyst); err != nil {
		t.Fatalf("upsert membership second user: %v", err)
	}
	if err := memberships.Upsert(context.Background(), secondTenant.ID, platform.ID, models.TenantRoleAdmin); err != nil {
		t.Fatalf("upsert membership platform user: %v", err)
	}

	handler := NewHandler(Dependencies{
		Config:                 cfg,
		JWT:                    jwtService,
		Users:                  users,
		Tenants:                tenants,
		Memberships:            memberships,
		RefreshTokens:          refresh,
		Alerts:                 alerts,
		Cases:                  cases,
		Tasks:                  tasks,
		Observables:            observables,
		CaseEvents:             caseEvents,
		CasePages:              casePages,
		Attachments:            attachments,
		Catalog:                catalog,
		System:                 system,
		Audits:                 audits,
		ConnectorIngest:        connectorIngest,
		InboundConnectorRuns:   inboundRuns,
		WorkflowRuns:           workflowRuns,
		ForumBindings:          forumBindings,
		NotificationBots:       notificationBots,
		NotificationSettings:   notificationSettings,
		APITokens:              apiTokens,
		AsyncOperations:        asyncOperations,
		AIChats:                aiChats,
		CaseAIAnalyses:         caseAI,
		Experience:             experience,
		AIAgentQueue:           aiAgentQueue,
		AIAgentWorkloads:       aiAgentWorkloads,
		ConnectorHubExecutions: connectorHubExecutions,
		ArtifactStorage:        storageStub,
		AI:                     aiService,
	})
	handler.search = searchStub

	return &apiTestEnv{
		t:                    t,
		pool:                 pool,
		cfg:                  cfg,
		e:                    echo.New(),
		handler:              handler,
		searchStub:           searchStub,
		storageStub:          storageStub,
		users:                users,
		tenants:              tenants,
		memberships:          memberships,
		alerts:               alerts,
		cases:                cases,
		tasks:                tasks,
		observables:          observables,
		caseEvents:           caseEvents,
		casePages:            casePages,
		attachments:          attachments,
		catalog:              catalog,
		notificationBots:     notificationBots,
		notificationSettings: notificationSettings,
		aiChats:              aiChats,
		caseAI:               caseAI,
		aiAgentQueue:         aiAgentQueue,
		aiAgentWorkloads:     aiAgentWorkloads,
		tenantID:             tenant.ID,
		secondTenant:         secondTenant.ID,
		userID:               user.ID,
		secondUserID:         secondUser.ID,
		identity: models.Identity{
			UserID:     user.ID,
			Username:   user.Username,
			TenantID:   &tenant.ID,
			TenantRole: models.TenantRoleAdmin,
		},
		platformAdmin: models.Identity{
			UserID:          platform.ID,
			Username:        platform.Username,
			IsPlatformAdmin: true,
		},
	}
}

func (env *apiTestEnv) jsonContext(method, target string, payload any) (*echo.Context, *httptest.ResponseRecorder) {
	env.t.Helper()

	var body io.Reader = http.NoBody
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			env.t.Fatalf("marshal payload: %v", err)
		}
		body = bytes.NewReader(raw)
	}

	req := httptest.NewRequest(method, target, body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	return env.e.NewContext(req, rec), rec
}

func (env *apiTestEnv) multipartContext(target string, body io.Reader, contentType string) (*echo.Context, *httptest.ResponseRecorder) {
	env.t.Helper()
	req := httptest.NewRequest(http.MethodPost, target, body)
	req.Header.Set(echo.HeaderContentType, contentType)
	rec := httptest.NewRecorder()
	return env.e.NewContext(req, rec), rec
}

func setIdentity(c *echo.Context, identity models.Identity) {
	c.Set("identity", identity)
}

func setTenant(c *echo.Context, tenantID uuid.UUID) {
	c.Set("tenant_id", tenantID)
}

func setPath(c *echo.Context, path string, names []string, values []string) {
	c.SetPath(path)
	params := make(echo.PathValues, 0, len(names))
	for i := range names {
		if i >= len(values) {
			break
		}
		params = append(params, echo.PathValue{Name: names[i], Value: values[i]})
		c.Request().SetPathValue(names[i], values[i])
	}
	if len(params) > 0 {
		c.SetPathValues(params)
	}
}

func decodeBody[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode body: %v\nbody=%s", err, rec.Body.String())
	}
	return out
}

func mustStatusOK(t *testing.T, err error, rec *httptest.ResponseRecorder, expected int) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if rec.Code != expected {
		t.Fatalf("unexpected status: got=%d want=%d body=%s", rec.Code, expected, rec.Body.String())
	}
}

var _ storage.ArtifactStorage = (*artifactStorageStub)(nil)
