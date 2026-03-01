package main

import (
	"context"
	"fmt"
	"incidenthub/backend/internal/ai"
	"incidenthub/backend/internal/api"
	"incidenthub/backend/internal/auth"
	"incidenthub/backend/internal/bootstrap"
	"incidenthub/backend/internal/cache"
	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/connectors/inbound"
	"incidenthub/backend/internal/connectors/outbound"
	"incidenthub/backend/internal/database"
	"incidenthub/backend/internal/forumproxy"
	"incidenthub/backend/internal/logger"
	"incidenthub/backend/internal/repository"
	"incidenthub/backend/internal/search"
	"incidenthub/backend/internal/security"
	"incidenthub/backend/internal/settings"
	"incidenthub/backend/internal/storage"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	rt, err := settings.Init(ctx, cfg)
	if err != nil {
		return fmt.Errorf("init runtime: %w", err)
	}
	defer rt.Close()

	pool, err := database.NewPool(ctx, cfg.Postgres)
	if err != nil {
		return fmt.Errorf("init postgres: %w", err)
	}
	defer pool.Close()

	if migrationErr := database.ApplyMigrations(ctx, pool, "migrations"); migrationErr != nil {
		return fmt.Errorf("apply migrations: %w", migrationErr)
	}

	redisClient, err := cache.New(ctx, cfg.Redis)
	if err != nil {
		return fmt.Errorf("init redis: %w", err)
	}
	defer func() { _ = redisClient.Close() }()

	searchClient, err := search.New(cfg.Elastic)
	if err != nil {
		return fmt.Errorf("init elastic: %w", err)
	}

	artifactStorage, err := storage.NewS3Storage(ctx, cfg.S3)
	if err != nil {
		return fmt.Errorf("init artifact storage: %w", err)
	}

	users := repository.NewUserRepository(pool)
	tenants := repository.NewTenantRepository(pool)
	memberships := repository.NewMembershipRepository(pool)
	refreshTokens := repository.NewRefreshTokenRepository(pool)
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
	inboundConnectorRuns := repository.NewInboundConnectorRunRepository(pool)
	workflowRuns := repository.NewWorkflowRunRepository(pool)
	forumBindings := repository.NewForumExternalBindingRepository(pool)
	recipientAliases := repository.NewConnectorRecipientAliasRepository(pool)
	notificationBots := repository.NewTelegramNotificationBotRepository(pool)
	notificationSettings := repository.NewUserNotificationSettingsRepository(pool)
	apiTokens := repository.NewAPIAccessTokenRepository(pool)
	asyncOperations := repository.NewAsyncOperationRepository(pool)
	aiChats := repository.NewAIChatRepository(pool)
	caseAIAnalyses := repository.NewCaseAIAnalysisRepository(pool)
	experience := repository.NewExperienceRepository(pool)
	aiAgentQueue := repository.NewAIAgentQueueRepository(pool)
	aiAgentWorkloads := repository.NewAIAgentWorkloadRepository(pool)
	connectorHubExecutions := repository.NewConnectorHubExecutionRepository(pool)

	outboundSvc := outbound.NewService(cfg.Outbound)
	forumProxySvc := forumproxy.NewService(catalog, forumBindings, recipientAliases, outboundSvc)
	aiSvc := ai.NewService(cfg.AI, searchClient)
	inboundWorker := inbound.NewWorker(cfg.Inbound, catalog, alerts, connectorIngest, inboundConnectorRuns, searchClient)
	inboundWorker.Start(ctx)
	asyncOps, err := api.NewKafkaAsyncOps(cfg.AsyncOps, api.KafkaAsyncOpsDependencies{
		Alerts:          alerts,
		Cases:           cases,
		Observables:     observables,
		Experience:      experience,
		Audits:          audits,
		AsyncOperations: asyncOperations,
		Search:          searchClient,
		AIAgentQueue:    aiAgentQueue,
	})
	if err != nil {
		return fmt.Errorf("init async ops kafka: %w", err)
	}
	if asyncOps != nil {
		asyncOps.Start(ctx)
		defer func() { _ = asyncOps.Close() }()
	}
	notificationQueue, err := api.NewKafkaNotificationDelivery(cfg.Notifications, cfg.Outbound, api.KafkaNotificationDeliveryDependencies{
		Settings: notificationSettings,
		Bots:     notificationBots,
		Catalog:  catalog,
		Outbound: outboundSvc,
	})
	if err != nil {
		return fmt.Errorf("init notification delivery kafka: %w", err)
	}
	if notificationQueue != nil {
		notificationQueue.Start(ctx)
		defer func() { _ = notificationQueue.Close() }()
	}
	serviceAlertEvaluator := api.NewServiceAlertEvaluator(cfg.ServiceAlerts, api.ServiceAlertEvaluatorDependencies{
		Catalog:              catalog,
		NotificationSettings: notificationSettings,
		Cases:                cases,
		CaseEvents:           caseEvents,
		Metrics:              rt.Metrics(),
		NotificationQueue:    notificationQueue,
	})
	if serviceAlertEvaluator != nil {
		serviceAlertEvaluator.Start(ctx)
	}

	if err := bootstrap.EnsurePlatformAdmin(ctx, cfg, bootstrap.Dependencies{
		Users:       users,
		Tenants:     tenants,
		Memberships: memberships,
		Cases:       cases,
		Alerts:      alerts,
		Catalog:     catalog,
	}); err != nil {
		return fmt.Errorf("bootstrap admin: %w", err)
	}

	jwtSvc := auth.New(cfg.Auth)
	ldapSvc := auth.NewLDAPAuthenticator(cfg.LDAP)
	limiter := security.NewRedisRateLimiter(redisClient, cfg.Security.LoginRateLimit, cfg.Security.LoginRateWindow)

	server := api.NewServer(api.Dependencies{
		Config:                 cfg,
		Metrics:                rt.Metrics(),
		JWT:                    jwtSvc,
		LDAP:                   ldapSvc,
		Cache:                  redisClient,
		Search:                 searchClient,
		Users:                  users,
		Tenants:                tenants,
		Memberships:            memberships,
		RefreshTokens:          refreshTokens,
		Alerts:                 alerts,
		Cases:                  cases,
		Tasks:                  tasks,
		Observables:            observables,
		CaseEvents:             caseEvents,
		CasePages:              casePages,
		Attachments:            attachments,
		Catalog:                catalog,
		System:                 system,
		ArtifactStorage:        artifactStorage,
		Audits:                 audits,
		ConnectorIngest:        connectorIngest,
		InboundConnectorRuns:   inboundConnectorRuns,
		WorkflowRuns:           workflowRuns,
		ForumBindings:          forumBindings,
		NotificationBots:       notificationBots,
		NotificationSettings:   notificationSettings,
		APITokens:              apiTokens,
		AsyncOperations:        asyncOperations,
		AIChats:                aiChats,
		CaseAIAnalyses:         caseAIAnalyses,
		Experience:             experience,
		AIAgentQueue:           aiAgentQueue,
		AIAgentWorkloads:       aiAgentWorkloads,
		ConnectorHubExecutions: connectorHubExecutions,
		InboundWorker:          inboundWorker,
		ForumProxy:             forumProxySvc,
		AI:                     aiSvc,
		AsyncOps:               asyncOps,
		NotificationQueue:      notificationQueue,
		SecurityRateLimiter:    limiter,
	})

	logger.Infof("starting api server on %s", cfg.HTTP.Addr)
	if err := server.Run(ctx); err != nil {
		return fmt.Errorf("run server: %w", err)
	}
	logger.Infof("api server stopped")
	return nil
}
