package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"incidenthub/backend/internal/ai"
	"incidenthub/backend/internal/auth"
	"incidenthub/backend/internal/cache"
	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/connectors/inbound"
	"incidenthub/backend/internal/connectors/outbound"
	"incidenthub/backend/internal/forumproxy"
	"incidenthub/backend/internal/metrics"
	"incidenthub/backend/internal/middleware"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"
	"incidenthub/backend/internal/search"
	"incidenthub/backend/internal/storage"
	"incidenthub/backend/internal/workflow"
	workflowNodes "incidenthub/backend/internal/workflow/nodes"
	"net/http"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labstack/echo/v5"
)

type Handler struct {
	cfg                    config.App
	metrics                *metrics.Collector
	jwt                    *auth.Service
	ldap                   *auth.LDAPAuthenticator
	users                  *repository.UserRepository
	tenants                *repository.TenantRepository
	memberships            *repository.MembershipRepository
	refresh                refreshTokenStore
	alerts                 *repository.AlertRepository
	cases                  *repository.CaseRepository
	tasks                  *repository.TaskRepository
	observables            *repository.ObservableRepository
	caseEvents             *repository.CaseEventRepository
	casePages              *repository.CasePageRepository
	attachments            *repository.AttachmentRepository
	catalog                *repository.CatalogRepository
	system                 *repository.SystemRepository
	connectorIngest        *repository.ConnectorIngestStateRepository
	inboundRuns            *repository.InboundConnectorRunRepository
	workflowRuns           *repository.WorkflowRunRepository
	forumBindings          *repository.ForumExternalBindingRepository
	notificationBots       *repository.TelegramNotificationBotRepository
	notificationSettings   *repository.UserNotificationSettingsRepository
	apiTokens              *repository.APIAccessTokenRepository
	asyncOperations        *repository.AsyncOperationRepository
	aiChats                *repository.AIChatRepository
	caseAIAnalyses         *repository.CaseAIAnalysisRepository
	experience             *repository.ExperienceRepository
	aiAgentQueue           *repository.AIAgentQueueRepository
	aiAgentWorkloads       *repository.AIAgentWorkloadRepository
	connectorHubExecutions *repository.ConnectorHubExecutionRepository
	cache                  *cache.Client
	artifacts              storage.ArtifactStorage
	audits                 *repository.AuditRepository
	inboundWorker          *inbound.Worker
	forumProxy             *forumproxy.Service
	outbound               *outbound.Service
	ai                     *ai.Service
	asyncOps               AsyncOpsQueue
	notificationQueue      NotificationDeliveryQueue
	workflowRuntime        *workflow.Runtime
	aiTenantMCP            *tenantMCPServer
	search                 interface {
		IndexDocument(ctx context.Context, kind, id string, doc any) error
		DeleteDocument(ctx context.Context, kind, id string) error
		Search(ctx context.Context, tenantID, query string, kinds []string, size int) ([]search.Hit, error)
		Ping(ctx context.Context) error
		Enabled() bool
	}
	loginLimiter interface {
		Allow(key string) bool
	}
	startedAt         time.Time
	backgroundWorkers sync.Once
}

type refreshTokenStore interface {
	Create(ctx context.Context, token models.RefreshToken) error
	GetActiveByHash(ctx context.Context, tokenHash string) (*models.RefreshToken, error)
	RevokeByHash(ctx context.Context, tokenHash string) error
	RevokeAllByUser(ctx context.Context, userID uuid.UUID) error
	RevokeAllByUserExceptHash(ctx context.Context, userID uuid.UUID, keepTokenHash string) (int64, error)
	ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]models.RefreshToken, error)
}

var attachmentNameSanitizer = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func NewHandler(deps Dependencies) *Handler {
	outboundService := outbound.NewService(deps.Config.Outbound)
	h := &Handler{
		cfg:                    deps.Config,
		metrics:                deps.Metrics,
		jwt:                    deps.JWT,
		ldap:                   deps.LDAP,
		users:                  deps.Users,
		tenants:                deps.Tenants,
		memberships:            deps.Memberships,
		refresh:                deps.RefreshTokens,
		alerts:                 deps.Alerts,
		cases:                  deps.Cases,
		tasks:                  deps.Tasks,
		observables:            deps.Observables,
		caseEvents:             deps.CaseEvents,
		casePages:              deps.CasePages,
		attachments:            deps.Attachments,
		catalog:                deps.Catalog,
		system:                 deps.System,
		connectorIngest:        deps.ConnectorIngest,
		inboundRuns:            deps.InboundConnectorRuns,
		workflowRuns:           deps.WorkflowRuns,
		forumBindings:          deps.ForumBindings,
		notificationBots:       deps.NotificationBots,
		notificationSettings:   deps.NotificationSettings,
		apiTokens:              deps.APITokens,
		asyncOperations:        deps.AsyncOperations,
		aiChats:                deps.AIChats,
		caseAIAnalyses:         deps.CaseAIAnalyses,
		experience:             deps.Experience,
		aiAgentQueue:           deps.AIAgentQueue,
		aiAgentWorkloads:       deps.AIAgentWorkloads,
		connectorHubExecutions: deps.ConnectorHubExecutions,
		cache:                  deps.Cache,
		artifacts:              deps.ArtifactStorage,
		audits:                 deps.Audits,
		inboundWorker:          deps.InboundWorker,
		forumProxy:             deps.ForumProxy,
		outbound:               outboundService,
		ai:                     deps.AI,
		asyncOps:               deps.AsyncOps,
		notificationQueue:      deps.NotificationQueue,
		search:                 deps.Search,
		loginLimiter:           deps.SecurityRateLimiter,
		startedAt:              time.Now().UTC(),
	}
	h.aiTenantMCP = newTenantMCPServer(h)
	h.workflowRuntime = workflow.NewRuntime(
		workflowNodes.NewBuiltinSet(workflowNodes.Dependencies{
			System:   deps.System,
			Catalog:  deps.Catalog,
			Outbound: outboundService,
		})...,
	)
	if h.ai != nil {
		h.ai.SetTenantContextProvider(newHandlerTenantContextProvider(h))
	}
	return h
}

func (h *Handler) Register(e *echo.Echo) {
	e.GET("/healthz", h.Health)
	if h.metrics != nil {
		e.GET("/metrics", echo.WrapHandler(h.metrics.Handler()))
	}
	if strings.EqualFold(strings.TrimSpace(h.cfg.Env), "dev") || h.cfg.HTTP.ExposeSwagger {
		e.GET("/dev/swagger/openapi.json", h.SwaggerSpec)
		e.GET("/dev/swagger", h.SwaggerUI)
		e.GET("/swagger/openapi.json", h.SwaggerSpec)
		e.GET("/swagger", h.SwaggerUI)
	}

	api := e.Group("/api/v1")
	api.POST("/auth/login", h.Login)
	api.POST("/auth/refresh", h.Refresh)
	api.POST("/auth/logout", h.Logout, middleware.JWTAuth(h.jwt, h.apiTokens, h.metrics), middleware.APITokenAccessControl())

	authed := api.Group("", middleware.JWTAuth(h.jwt, h.apiTokens, h.metrics))
	authed.Use(middleware.APITokenAccessControl())
	authed.Use(middleware.APICacheInvalidation(h.cache, h.cfg.APICache))
	authed.Use(middleware.APICache(h.cache, h.cfg.APICache))
	authed.GET("/me", h.Me)
	authed.GET("/auth/sessions", h.ListAuthSessions)
	authed.POST("/auth/sessions/revoke-others", h.RevokeOtherAuthSessions)
	authed.GET("/tenants", h.ListTenants)
	authed.GET("/users/:id", h.GetUser)
	authed.GET("/users/:id/experience-events", h.ListUserExperienceEvents)
	authed.GET("/users/:id/performance", h.GetUserCasePerformance)
	authed.PATCH("/users/:id", h.UpdateUser)
	authed.DELETE("/users/:id", h.DeleteUser)
	authed.POST("/users/:id/experience-awards", h.AwardUserExperience, middleware.ResolveTenant(h.cfg, h.memberships, h.tenants), middleware.RequireTenantAdminOrPlatformAdmin())
	authed.POST("/users/:id/media/:kind/upload", h.UploadUserMedia)
	authed.GET("/users", h.ListUsers)
	authed.POST("/tenants", h.CreateTenant, middleware.RequirePlatformAdmin())
	authed.PATCH("/tenants/:tenantID", h.UpdateTenant, middleware.RequirePlatformAdmin())
	authed.POST("/users", h.CreateUser, middleware.ResolveTenant(h.cfg, h.memberships, h.tenants), middleware.RequireTenantAdminOrPlatformAdmin())

	tenantScoped := authed.Group("", middleware.ResolveTenant(h.cfg, h.memberships, h.tenants))
	tenantScoped.GET("/notification-bots", h.ListNotificationBots, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/me/notification-settings", h.GetMyNotificationSettings, middleware.RequireAnalystOrHigher())
	tenantScoped.PUT("/me/notification-settings", h.UpdateMyNotificationSettings, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/admin/notification-bots", h.ListAdminNotificationBots, middleware.RequireTenantAdminOrPlatformAdmin())
	tenantScoped.POST("/admin/notification-bots", h.CreateAdminNotificationBot, middleware.RequireTenantAdminOrPlatformAdmin())
	tenantScoped.PATCH("/admin/notification-bots/:botID", h.UpdateAdminNotificationBot, middleware.RequireTenantAdminOrPlatformAdmin())
	tenantScoped.DELETE("/admin/notification-bots/:botID", h.DeleteAdminNotificationBot, middleware.RequireTenantAdminOrPlatformAdmin())
	tenantScoped.GET("/admin/notification-settings", h.ListAdminNotificationSettings, middleware.RequireTenantAdminOrPlatformAdmin())
	tenantScoped.PUT("/admin/notification-settings/:userID", h.UpdateAdminNotificationSettings, middleware.RequireTenantAdminOrPlatformAdmin())
	tenantScoped.GET("/admin/api-tokens", h.ListAPIAccessTokens, middleware.RequireTenantAdminOrPlatformAdmin())
	tenantScoped.POST("/admin/api-tokens", h.CreateAPIAccessToken, middleware.RequireTenantAdminOrPlatformAdmin())
	tenantScoped.DELETE("/admin/api-tokens/:tokenID", h.RevokeAPIAccessToken, middleware.RequireTenantAdminOrPlatformAdmin())
	tenantScoped.POST("/admin/factory-reset-local", h.FactoryResetLocal, middleware.RequirePlatformAdmin())
	tenantScoped.GET("/operations/:operationID", h.GetAsyncOperation, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/tenant-users", h.ListTenantUsers, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/alerts", h.ListAlerts, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/alerts/:alertID", h.GetAlert, middleware.RequireAnalystOrHigher())
	tenantScoped.POST("/alerts", h.CreateAlert, middleware.RequireAnalystOrHigher())
	tenantScoped.PATCH("/alerts/:alertID", h.UpdateAlert, middleware.RequireAnalystOrHigher())
	tenantScoped.DELETE("/alerts/:alertID", h.DeleteAlert, middleware.RequireAnalystOrHigher())
	tenantScoped.POST("/alerts/bulk/link-case", h.BindAlertsToCase, middleware.RequireAnalystOrHigher())
	tenantScoped.POST("/alerts/bulk/create-case", h.CreateCaseFromAlerts, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/dashboard/stats", h.GetDashboardStats, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/dashboard/metrics", h.GetDashboardMetrics, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/dashboard/metrics/custom", h.ListDashboardCustomMetrics, middleware.RequireAnalystOrHigher())
	tenantScoped.POST("/dashboard/metrics/custom", h.CreateDashboardCustomMetric, middleware.RequireAnalystOrHigher())
	tenantScoped.PATCH("/dashboard/metrics/custom/:metricID", h.UpdateDashboardCustomMetric, middleware.RequireAnalystOrHigher())
	tenantScoped.DELETE("/dashboard/metrics/custom/:metricID", h.DeleteDashboardCustomMetric, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/activity/livestream", h.GetActivityLivestream, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/duty/overview", h.GetDutyOverview, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/system/health", h.SystemHealth, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/system/resources", h.SystemResources, middleware.RequireTenantAdminOrPlatformAdmin())
	tenantScoped.GET("/case-statuses", h.ListCaseStatuses, middleware.RequireAnalystOrHigher())
	tenantScoped.PUT("/case-statuses", h.UpsertCaseStatuses, middleware.RequireTenantAdminOrPlatformAdmin())
	tenantScoped.GET("/cases", h.ListCases, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/cases/summary", h.ListCaseListSummaries, middleware.RequireAnalystOrHigher())
	tenantScoped.POST("/cases", h.CreateCase, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/cases/:caseID", h.GetCase, middleware.RequireAnalystOrHigher())
	tenantScoped.POST("/cases/:caseID/copy", h.CopyCase, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/cases/:caseID/related", h.ListRelatedCases, middleware.RequireAnalystOrHigher())
	tenantScoped.PATCH("/cases/:caseID", h.UpdateCase, middleware.RequireAnalystOrHigher())
	tenantScoped.DELETE("/cases/:caseID", h.DeleteCase, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/cases/:caseID/shares", h.ListCaseShares, middleware.RequireAnalystOrHigher())
	tenantScoped.POST("/cases/:caseID/share", h.ShareCaseAcrossTenants, middleware.RequireTenantAdminOrPlatformAdmin())
	tenantScoped.POST("/cases/:caseID/escalate", h.EscalateCase, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/tasks", h.ListTasks, middleware.RequireAnalystOrHigher())
	tenantScoped.POST("/tasks", h.CreateTask, middleware.RequireAnalystOrHigher())
	tenantScoped.PATCH("/tasks/:taskID", h.UpdateTask, middleware.RequireAnalystOrHigher())
	tenantScoped.DELETE("/tasks/:taskID", h.DeleteTask, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/cases/:caseID/observables", h.ListCaseObservables, middleware.RequireAnalystOrHigher())
	tenantScoped.POST("/cases/:caseID/observables", h.CreateCaseObservable, middleware.RequireAnalystOrHigher())
	tenantScoped.PATCH("/cases/:caseID/observables/:observableID", h.UpdateCaseObservable, middleware.RequireAnalystOrHigher())
	tenantScoped.DELETE("/cases/:caseID/observables/:observableID", h.DeleteCaseObservable, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/cases/:caseID/events", h.ListCaseEvents, middleware.RequireAnalystOrHigher())
	tenantScoped.POST("/cases/:caseID/events", h.CreateCaseEvent, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/cases/:caseID/pages", h.ListCasePages, middleware.RequireAnalystOrHigher())
	tenantScoped.POST("/cases/:caseID/pages", h.CreateCasePage, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/cases/:caseID/attachments", h.ListCaseAttachments, middleware.RequireAnalystOrHigher())
	tenantScoped.POST("/cases/:caseID/attachments", h.CreateCaseAttachment, middleware.RequireAnalystOrHigher())
	tenantScoped.POST("/cases/:caseID/attachments/upload", h.UploadCaseAttachment, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/cases/:caseID/attachments/:attachmentID/download", h.GetCaseAttachmentDownloadURL, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/cases/:caseID/communications", h.ListCaseCommunications, middleware.RequireAnalystOrHigher())
	tenantScoped.POST("/cases/:caseID/communications", h.CreateCaseCommunication, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/cases/:caseID/communications/:threadID", h.GetCaseCommunication, middleware.RequireAnalystOrHigher())
	tenantScoped.POST("/cases/:caseID/communications/:threadID/messages", h.SendCaseCommunicationMessage, middleware.RequireAnalystOrHigher())
	tenantScoped.POST("/cases/:caseID/communications/:threadID/sync", h.SyncCaseCommunication, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/connectors/outbound", h.ListOutboundConnectors, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/connectors/hub/executions", h.ListConnectorHubExecutions, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/connectors/hub/executions/:executionID", h.GetConnectorHubExecution, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/connectors/hub/executions/:executionID/events", h.ListConnectorHubExecutionEvents, middleware.RequireAnalystOrHigher())
	tenantScoped.POST("/connectors/hub/executions/:executionID/retry", h.RetryConnectorHubExecution, middleware.RequireAnalystOrHigher())
	tenantScoped.POST("/connectors/hub/executions/:executionID/cancel", h.CancelConnectorHubExecution, middleware.RequireAnalystOrHigher())
	tenantScoped.POST("/connectors/hub/executions/:executionID/restart", h.RestartConnectorHubExecution, middleware.RequireAnalystOrHigher())
	tenantScoped.POST("/connectors/hub/execute", h.ExecuteConnectorHub, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/workflows/vault/secrets", h.ListWorkflowVaultSecrets, middleware.RequireAnalystOrHigher())
	tenantScoped.POST("/workflows/vault/secrets", h.CreateWorkflowVaultSecret, middleware.RequireAnalystOrHigher())
	tenantScoped.DELETE("/workflows/vault/secrets/:secretID", h.DeleteWorkflowVaultSecret, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/workflows/credentials", h.ListWorkflowCredentials, middleware.RequireAnalystOrHigher())
	tenantScoped.POST("/workflows/credentials", h.CreateWorkflowCredential, middleware.RequireAnalystOrHigher())
	tenantScoped.PUT("/workflows/credentials/:credentialID", h.UpdateWorkflowCredential, middleware.RequireAnalystOrHigher())
	tenantScoped.DELETE("/workflows/credentials/:credentialID", h.DeleteWorkflowCredential, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/communications/connectors", h.ListCommunicationConnectors, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/forum/threads", h.ListForumThreads, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/forum/threads/:threadID", h.GetForumThread, middleware.RequireAnalystOrHigher())
	tenantScoped.POST("/forum/threads", h.CreateForumThread, middleware.RequireAnalystOrHigher())
	tenantScoped.POST("/forum/threads/:threadID/posts/upload", h.CreateForumPostWithAttachments, middleware.RequireAnalystOrHigher())
	tenantScoped.POST("/forum/posts", h.CreateForumPost, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/forum/threads/:threadID/proxy/profiles", h.ListForumProxyProfiles, middleware.RequireAnalystOrHigher())
	tenantScoped.POST("/forum/threads/:threadID/proxy/profiles", h.UpsertForumProxyProfile, middleware.RequireAnalystOrHigher())
	tenantScoped.DELETE("/forum/threads/:threadID/proxy/profiles/:profileID", h.DeleteForumProxyProfile, middleware.RequireAnalystOrHigher())
	tenantScoped.POST("/forum/threads/:threadID/proxy/send", h.ProxyForumSend, middleware.RequireAnalystOrHigher())
	tenantScoped.POST("/forum/threads/:threadID/proxy/sync", h.ProxyForumSync, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/case-comments", h.ListCaseComments, middleware.RequireAnalystOrHigher())
	tenantScoped.POST("/case-comments", h.CreateCaseComment, middleware.RequireAnalystOrHigher())
	tenantScoped.POST("/workflows/:workflowID/run", h.RunWorkflow, middleware.RequireAnalystOrHigher())
	tenantScoped.POST("/workflows/:workflowID/test-run", h.TestRunWorkflow, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/workflows/:workflowID/runs", h.ListWorkflowRuns, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/ai/sessions", h.ListAISessions, middleware.RequireAnalystOrHigher())
	tenantScoped.POST("/ai/sessions", h.CreateAISession, middleware.RequireAnalystOrHigher())
	tenantScoped.DELETE("/ai/sessions/:sessionID", h.DeleteAISession, middleware.RequireAnalystOrHigher())
	tenantScoped.PATCH("/ai/sessions/reorder", h.ReorderAISessions, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/ai/session", h.GetAISession, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/ai/messages", h.ListAIMessages, middleware.RequireAnalystOrHigher())
	tenantScoped.POST("/ai/ask", h.AskAI, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/ai/agents/entities/:entityType/:entityID", h.GetAIAgentEntityTrace, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/ai/agents/ops/overview", h.GetAIAgentOperationsOverview, middleware.RequireTenantAdminOrPlatformAdmin())
	tenantScoped.POST("/ai/agents/ops/events/:eventID/restart", h.RestartAIAgentQueueEvent, middleware.RequireTenantAdminOrPlatformAdmin())
	tenantScoped.POST("/ai/agents/ops/events/:eventID/close", h.CloseAIAgentQueueEvent, middleware.RequireTenantAdminOrPlatformAdmin())
	tenantScoped.POST("/ai/agents/ops/workloads/:workloadID/restart", h.RestartAIAgentWorkload, middleware.RequireTenantAdminOrPlatformAdmin())
	tenantScoped.POST("/ai/agents/ops/workloads/:workloadID/close", h.CloseAIAgentWorkload, middleware.RequireTenantAdminOrPlatformAdmin())
	tenantScoped.POST("/ai/agents/:agentID/run", h.RunAIAgent, middleware.RequireTenantAdminOrPlatformAdmin())
	tenantScoped.GET("/ai/agents/:agentID/runs", h.ListAIAgentRuns, middleware.RequireTenantAdminOrPlatformAdmin())
	tenantScoped.GET("/cases/:caseID/ai/analyses", h.ListCaseAIAnalyses, middleware.RequireAnalystOrHigher())
	tenantScoped.POST("/cases/:caseID/ai/analyze", h.AnalyzeCaseWithAI, middleware.RequireAnalystOrHigher())
	tenantScoped.POST("/catalog/achievements/icon/upload", h.UploadAchievementIcon, middleware.RequireTenantAdminOrPlatformAdmin())
	tenantScoped.GET("/catalog/:kind", h.ListCatalogItems, middleware.RequireAnalystOrHigher())
	tenantScoped.POST("/catalog/:kind", h.CreateCatalogItem, middleware.RequireAnalystOrHigher())
	tenantScoped.PATCH("/catalog/:kind/:itemID", h.UpdateCatalogItem, middleware.RequireAnalystOrHigher())
	tenantScoped.DELETE("/catalog/:kind/:itemID", h.DeleteCatalogItem, middleware.RequireAnalystOrHigher())
	tenantScoped.GET("/search", h.Search, middleware.RequireAnalystOrHigher())
}

func (h *Handler) resolveCaseInTenant(c *echo.Context) (tenantID uuid.UUID, caseID uuid.UUID, err error) {
	requestedTenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return uuid.Nil, uuid.Nil, echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	identity, _ := middleware.GetIdentity(c)
	parsedCaseID, err := uuid.Parse(strings.TrimSpace(c.Param("caseID")))
	if err != nil {
		return uuid.Nil, uuid.Nil, echo.NewHTTPError(http.StatusBadRequest, "invalid caseID path param")
	}
	resolvedTenantID, exists, err := h.cases.ResolveAccessibleTenant(c.Request().Context(), requestedTenantID, parsedCaseID)
	if err != nil {
		return uuid.Nil, uuid.Nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to validate case")
	}
	if !exists {
		return uuid.Nil, uuid.Nil, echo.NewHTTPError(http.StatusNotFound, "case not found in tenant")
	}
	if err := h.enforceCaseTagAccessPolicy(c.Request().Context(), requestedTenantID, resolvedTenantID, parsedCaseID, identity); err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	return resolvedTenantID, parsedCaseID, nil
}

// Sentinel errors for health checks and validation (err113).
var (
	errSystemRepositoryUnavailable     = errors.New("system repository unavailable")
	errRedisClientUnavailable          = errors.New("redis client unavailable")
	errElasticsearchClientUnavailable  = errors.New("elasticsearch client unavailable")
	errArtifactStorageUnavailable      = errors.New("artifact storage unavailable")
	errAIServiceUnavailable            = errors.New("ai service unavailable")
	errAsyncOperationsQueueUnavailable = errors.New("async operations queue unavailable")
	errWorkflowRuntimeUnavailable      = errors.New("internal workflow runtime unavailable")
	errInvalidRole                     = errors.New("invalid role")
)

// moduleHealth reports module reachability and health-check latency, not business-operation latency.
type moduleHealth struct {
	Status     string                 `json:"status"`
	ResponseMs float64                `json:"responseMs"`
	Operation  *moduleOperationHealth `json:"operation,omitempty"`
	Message    string                 `json:"message,omitempty"`
}

type moduleOperationHealth struct {
	AvgLatencyMs      float64 `json:"avgLatencyMs"`
	RequestsPerSecond float64 `json:"requestsPerSecond"`
	Operations        int64   `json:"operations"`
	Errors            int64   `json:"errors"`
	WindowSeconds     int64   `json:"windowSeconds"`
}

type moduleOperationSource struct {
	metricModule string
	operations   []string
}

var moduleOperationSources = map[string]moduleOperationSource{
	"api":           {metricModule: "api", operations: []string{"request"}},
	"redis":         {metricModule: "cache", operations: []string{"get", "set", "incr", "incr_with_window", "del"}},
	"elasticsearch": {metricModule: "search", operations: []string{"search", "query", "index_document", "delete_document", "recent"}},
	"s3":            {metricModule: "storage", operations: []string{"upload", "presign_get"}},
	"ai_model":      {metricModule: "ai", operations: []string{"ask", "analyze_case"}},
	"async_ops":     {metricModule: "async_ops", operations: []string{"enqueue", "parse_operation", "consumer_poll"}},
	"forum_proxy":   {metricModule: "forumproxy", operations: []string{"send", "sync"}},
}

const moduleOperationWindow = 5 * time.Minute

func (h *Handler) collectModulesHealth(ctx context.Context) (overallStatus string, moduleStatuses map[string]moduleHealth) {
	modules := map[string]moduleHealth{}

	modules["api"] = h.checkModule(ctx, false, func(context.Context) error { return nil })

	modules["postgres"] = h.checkModule(ctx, false, func(runCtx context.Context) error {
		if h.system == nil {
			return errSystemRepositoryUnavailable
		}
		return h.system.Ping(runCtx)
	})

	modules["redis"] = h.checkModule(ctx, false, func(runCtx context.Context) error {
		if h.cache == nil {
			return errRedisClientUnavailable
		}
		return h.cache.Ping(runCtx)
	})

	modules["elasticsearch"] = h.checkModule(ctx, !h.cfg.Elastic.Enabled, func(runCtx context.Context) error {
		if h.search == nil {
			return errElasticsearchClientUnavailable
		}
		return h.search.Ping(runCtx)
	})

	modules["s3"] = h.checkModule(ctx, !h.cfg.S3.Enabled, func(runCtx context.Context) error {
		if h.artifacts == nil {
			return errArtifactStorageUnavailable
		}
		return h.artifacts.Health(runCtx)
	})

	modules["ai_model"] = h.checkModule(ctx, !h.cfg.AI.Enabled, func(runCtx context.Context) error {
		if h.ai == nil {
			return errAIServiceUnavailable
		}
		return h.ai.Ping(runCtx)
	})

	modules["async_ops"] = h.checkModule(ctx, !h.cfg.AsyncOps.Enabled, func(context.Context) error {
		if h.asyncOps == nil || !h.asyncOps.Enabled() {
			return errAsyncOperationsQueueUnavailable
		}
		return nil
	})

	modules["workflow_engine"] = h.checkModule(ctx, false, func(_ context.Context) error {
		if h.workflowRuntime == nil {
			return errWorkflowRuntimeUnavailable
		}
		return nil
	})

	modules["forum_proxy"] = h.checkModule(ctx, h.forumProxy == nil, func(context.Context) error { return nil })

	overall := "ok"
	for _, module := range modules {
		if module.Status == "error" {
			overall = "degraded"
			break
		}
	}
	if h.metrics != nil {
		for moduleName, module := range modules {
			h.metrics.SetModuleHealth(moduleName, module.Status, module.ResponseMs)
		}
		h.attachModuleOperationHealth(modules)
	}

	return overall, modules
}

func (h *Handler) checkModule(ctx context.Context, disabled bool, fn func(context.Context) error) moduleHealth {
	if disabled {
		return moduleHealth{
			Status:     "disabled",
			ResponseMs: 0,
		}
	}

	started := time.Now()
	moduleCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := fn(moduleCtx); err != nil {
		return moduleHealth{
			Status:     "error",
			ResponseMs: durationMilliseconds(time.Since(started)),
			Message:    err.Error(),
		}
	}

	return moduleHealth{
		Status:     "ok",
		ResponseMs: durationMilliseconds(time.Since(started)),
	}
}

func durationMilliseconds(duration time.Duration) float64 {
	if duration <= 0 {
		return 0
	}
	ms := float64(duration) / float64(time.Millisecond)
	if ms < 0.01 {
		return 0.01
	}
	return roundTo(ms)
}

func (h *Handler) attachModuleOperationHealth(modules map[string]moduleHealth) {
	if h == nil || h.metrics == nil || len(modules) == 0 {
		return
	}
	for moduleName, source := range moduleOperationSources {
		module, ok := modules[moduleName]
		if !ok {
			continue
		}
		if strings.EqualFold(module.Status, "disabled") {
			continue
		}
		snapshot := h.collectModuleOperationHealth(source)
		if snapshot == nil {
			continue
		}
		module.Operation = snapshot
		modules[moduleName] = module
	}
}

func (h *Handler) collectModuleOperationHealth(source moduleOperationSource) *moduleOperationHealth {
	if h == nil || h.metrics == nil {
		return nil
	}
	if len(source.operations) == 0 {
		snapshot := h.metrics.ModuleOperationSnapshot(source.metricModule, moduleOperationWindow)
		if snapshot.Operations == 0 {
			return nil
		}
		return moduleOperationHealthFromSnapshot(snapshot)
	}

	combined := metrics.ModuleOperationSnapshot{
		Module:        source.metricModule,
		WindowSeconds: int64(moduleOperationWindow / time.Second),
	}
	var totalDurationMs float64
	for _, operationName := range source.operations {
		snapshot := h.metrics.ModuleOperationSnapshotByOperation(source.metricModule, operationName, moduleOperationWindow)
		if snapshot.Operations <= 0 {
			continue
		}
		combined.Operations += snapshot.Operations
		combined.Errors += snapshot.Errors
		combined.RequestsPerSecond += snapshot.RequestsPerSecond
		totalDurationMs += snapshot.AvgLatencyMs * float64(snapshot.Operations)
	}
	if combined.Operations == 0 {
		return nil
	}
	combined.AvgLatencyMs = totalDurationMs / float64(combined.Operations)
	return moduleOperationHealthFromSnapshot(combined)
}

func moduleOperationHealthFromSnapshot(snapshot metrics.ModuleOperationSnapshot) *moduleOperationHealth {
	if snapshot.Operations <= 0 {
		return nil
	}
	return &moduleOperationHealth{
		AvgLatencyMs:      roundTo(snapshot.AvgLatencyMs),
		RequestsPerSecond: roundTo(snapshot.RequestsPerSecond),
		Operations:        snapshot.Operations,
		Errors:            snapshot.Errors,
		WindowSeconds:     snapshot.WindowSeconds,
	}
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func (h *Handler) setRefreshCookie(c *echo.Context, token string, expires time.Time) {
	cookie := &http.Cookie{
		Name:     h.cfg.Auth.CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cfg.HTTP.SecureCookies,
		SameSite: http.SameSiteLaxMode,
		Expires:  expires,
	}
	if token == "" {
		cookie.MaxAge = -1
	}
	c.SetCookie(cookie)
}

func (h *Handler) authAttempt(result string) {
	if h.metrics != nil {
		h.metrics.AuthAttemptsTotal.WithLabelValues(result).Inc()
	}
}

func parseRole(input string) (models.TenantRole, error) {
	role := models.TenantRole(strings.TrimSpace(input))
	if role == "" {
		return models.TenantRoleAnalyst, nil
	}
	switch role {
	case models.TenantRoleAdmin, models.TenantRoleAnalyst, models.TenantRoleViewer:
		return role, nil
	default:
		return "", errInvalidRole
	}
}

func trimOptional(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func lowerOptional(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(strings.ToLower(*value))
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func normalizeTags(input []string) []string {
	if len(input) == 0 {
		return []string{}
	}
	uniq := make(map[string]struct{}, len(input))
	result := make([]string, 0, len(input))
	for _, raw := range input {
		tag := strings.TrimSpace(strings.ToLower(raw))
		if tag == "" {
			continue
		}
		if _, exists := uniq[tag]; exists {
			continue
		}
		uniq[tag] = struct{}{}
		result = append(result, tag)
	}
	return result
}

func normalizeCatalogKind(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}

func isDeprecatedCatalogKind(kind string) bool {
	switch normalizeCatalogKind(kind) {
	case
		"action_modules",
		"analyzer_workflows",
		"responder_workflows",
		"connectors",
		"case_connectors",
		"workflow_vault_secrets":
		return true
	default:
		return false
	}
}

func stringFromMap(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := payload[key]; ok {
			raw := strings.TrimSpace(fmt.Sprint(value))
			if raw != "" && raw != "<nil>" {
				return raw
			}
		}
	}
	return ""
}

func matchesAllSearchTerms(query string, fields ...string) bool {
	terms := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	if len(terms) == 0 {
		return false
	}
	if len(fields) == 0 {
		return false
	}
	joined := strings.ToLower(strings.Join(fields, " "))
	for _, term := range terms {
		if term == "" {
			continue
		}
		if !strings.Contains(joined, term) {
			return false
		}
	}
	return true
}

func mapCatalogItems(items []models.CatalogItem) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, catalogItemToPayload(item))
	}
	return result
}

func catalogItemToPayload(item models.CatalogItem) map[string]any {
	out := map[string]any{
		"id":         item.ID.String(),
		"kind":       item.Kind,
		"created_at": item.CreatedAt,
		"updated_at": item.UpdatedAt,
	}
	if item.TenantID != nil {
		out["tenant_id"] = item.TenantID.String()
	}
	if item.OwnerID != nil {
		out["owner_id"] = item.OwnerID.String()
	}
	if item.RefID != nil {
		out["ref_id"] = item.RefID.String()
	}
	for k, v := range item.Data {
		if isCatalogSensitiveDataKey(item.Kind, k) {
			continue
		}
		out[k] = normalizeJSONValue(v)
	}
	if isLegacyAPITokensKind(item.Kind) {
		out["auth_supported"] = false
	}
	return out
}

func isLegacyAPITokensKind(kind string) bool {
	return normalizeCatalogKind(kind) == "api_tokens"
}

func isCatalogSensitiveDataKey(kind, key string) bool {
	if !isLegacyAPITokensKind(kind) {
		return false
	}
	switch strings.TrimSpace(strings.ToLower(key)) {
	case "token", "token_hash", "secret", "secret_hash":
		return true
	default:
		return false
	}
}

func normalizeJSONValue(input any) any {
	switch value := input.(type) {
	case json.Number:
		if i, err := value.Int64(); err == nil {
			return i
		}
		if f, err := value.Float64(); err == nil {
			return f
		}
	case []any:
		out := make([]any, 0, len(value))
		for _, item := range value {
			out = append(out, normalizeJSONValue(item))
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(value))
		for k, item := range value {
			out[k] = normalizeJSONValue(item)
		}
		return out
	}
	return input
}

func (h *Handler) canMutateCatalog(identity models.Identity, kind string, creating bool) bool {
	if identity.IsPlatformAdmin || identity.TenantRole == models.TenantRoleAdmin {
		return true
	}
	analystAllowedKinds := []string{
		"case_meta",
		"alert_meta",
		"case_communication_thread",
		"case_communication_message",
		"workflows",
		"notifications",
		"forum_thread",
		"forum_post",
		"case_comment",
	}
	if slices.Contains(analystAllowedKinds, kind) {
		return true
	}
	return !creating && kind == "notifications"
}

func (h *Handler) upsertCaseForumLink(ctx context.Context, tenantID, caseID, forumID, actorID uuid.UUID) error {
	items, err := h.catalog.List(ctx, repository.CatalogListParams{
		Kind:     "case_meta",
		TenantID: &tenantID,
		RefID:    &caseID,
		Limit:    1,
	})
	if err != nil {
		return err
	}

	data := map[string]any{
		"forumId":   forumID.String(),
		"forum_id":  forumID.String(),
		"case_id":   caseID.String(),
		"tenant_id": tenantID.String(),
	}
	if len(items) > 0 {
		_, err = h.catalog.Update(ctx, "case_meta", items[0].ID, &tenantID, repository.CatalogUpdateParams{Data: data})
		return err
	}

	_, err = h.catalog.Create(ctx, repository.CatalogCreateParams{
		TenantID:  &tenantID,
		Kind:      "case_meta",
		OwnerID:   &actorID,
		RefID:     &caseID,
		Data:      data,
		CreatedBy: &actorID,
	})
	return err
}

func isForumThreadUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	if pgErr.Code != "23505" {
		return false
	}
	return strings.Contains(pgErr.ConstraintName, "uq_catalog_forum_thread_case")
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" && trimmed != "<nil>" {
			return trimmed
		}
	}
	return ""
}

func (h *Handler) maxAttachmentBytes() int64 {
	maxUploadMB := h.cfg.Artifacts.MaxUploadMB
	if maxUploadMB <= 0 {
		maxUploadMB = 50
	}
	return int64(maxUploadMB) * 1024 * 1024
}

func sanitizeAttachmentName(input string) string {
	name := strings.TrimSpace(filepath.Base(input))
	if name == "" || name == "." || name == "/" {
		return "artifact.bin"
	}
	name = attachmentNameSanitizer.ReplaceAllString(name, "_")
	if len(name) > 180 {
		name = name[:180]
	}
	name = strings.Trim(name, "._")
	if name == "" {
		return "artifact.bin"
	}
	return name
}
