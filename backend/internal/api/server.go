package api

import (
	"context"
	"errors"
	"incidenthub/backend/internal/ai"
	"incidenthub/backend/internal/auth"
	"incidenthub/backend/internal/cache"
	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/connectors/inbound"
	"incidenthub/backend/internal/forumproxy"
	"incidenthub/backend/internal/metrics"
	appmw "incidenthub/backend/internal/middleware"
	"incidenthub/backend/internal/repository"
	"incidenthub/backend/internal/search"
	"incidenthub/backend/internal/storage"
	"net/http"

	"github.com/labstack/echo/v5"
	echomw "github.com/labstack/echo/v5/middleware"
)

type Dependencies struct {
	Config                 config.App
	Metrics                *metrics.Collector
	JWT                    *auth.Service
	LDAP                   *auth.LDAPAuthenticator
	Cache                  *cache.Client
	Search                 *search.Client
	Users                  *repository.UserRepository
	Tenants                *repository.TenantRepository
	Memberships            *repository.MembershipRepository
	RefreshTokens          *repository.RefreshTokenRepository
	Alerts                 *repository.AlertRepository
	Cases                  *repository.CaseRepository
	Tasks                  *repository.TaskRepository
	Observables            *repository.ObservableRepository
	CaseEvents             *repository.CaseEventRepository
	CasePages              *repository.CasePageRepository
	Attachments            *repository.AttachmentRepository
	Catalog                *repository.CatalogRepository
	System                 *repository.SystemRepository
	ArtifactStorage        storage.ArtifactStorage
	Audits                 *repository.AuditRepository
	ConnectorIngest        *repository.ConnectorIngestStateRepository
	InboundConnectorRuns   *repository.InboundConnectorRunRepository
	WorkflowRuns           *repository.WorkflowRunRepository
	ForumBindings          *repository.ForumExternalBindingRepository
	NotificationBots       *repository.TelegramNotificationBotRepository
	NotificationSettings   *repository.UserNotificationSettingsRepository
	APITokens              *repository.APIAccessTokenRepository
	AsyncOperations        *repository.AsyncOperationRepository
	AIChats                *repository.AIChatRepository
	CaseAIAnalyses         *repository.CaseAIAnalysisRepository
	Experience             *repository.ExperienceRepository
	AIAgentQueue           *repository.AIAgentQueueRepository
	AIAgentWorkloads       *repository.AIAgentWorkloadRepository
	ConnectorHubExecutions *repository.ConnectorHubExecutionRepository
	InboundWorker          *inbound.Worker
	ForumProxy             *forumproxy.Service
	AI                     *ai.Service
	AsyncOps               AsyncOpsQueue
	NotificationQueue      NotificationDeliveryQueue
	SecurityRateLimiter    interface {
		Allow(key string) bool
	}
}

type Server struct {
	e       *echo.Echo
	cfg     config.App
	http    *http.Server
	handler *Handler
}

func NewServer(deps Dependencies) *Server {
	e := echo.New()
	e.HTTPErrorHandler = customHTTPErrorHandler
	e.Use(echomw.Recover())
	e.Use(echomw.RequestID())
	e.Use(echomw.CORSWithConfig(echomw.CORSConfig{
		AllowOrigins:     deps.Config.HTTP.CORSOrigins(),
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowHeaders:     []string{"Authorization", "Content-Type", config.TenantHeader},
		AllowCredentials: true,
	}))
	e.Use(appmw.SecurityHeaders())
	e.Use(appmw.RequestObservability())
	if deps.Metrics != nil {
		e.Use(deps.Metrics.Middleware())
	}

	h := NewHandler(deps)
	h.Register(e)

	httpServer := &http.Server{
		Addr:         deps.Config.HTTP.Addr,
		ReadTimeout:  deps.Config.HTTP.ReadTimeout,
		WriteTimeout: deps.Config.HTTP.WriteTimeout,
		Handler:      e,
	}

	return &Server{e: e, cfg: deps.Config, http: httpServer, handler: h}
}

func (s *Server) Run(ctx context.Context) error {
	if s.handler != nil {
		s.handler.StartBackgroundWorkers(ctx)
	}
	errCh := make(chan error, 1)
	go func() {
		if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		return s.Shutdown(context.Background())
	case err := <-errCh:
		return err
	}
}

func (s *Server) Echo() *echo.Echo { return s.e }

func (s *Server) Shutdown(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	shutdownCtx, cancel := context.WithTimeout(ctx, s.cfg.HTTP.ShutdownTimeout)
	defer cancel()
	return s.http.Shutdown(shutdownCtx)
}
