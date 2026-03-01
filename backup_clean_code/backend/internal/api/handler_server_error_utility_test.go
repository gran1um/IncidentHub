package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/metrics"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

func TestCustomHTTPErrorHandler(t *testing.T) {
	e := echo.New()

	{
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		customHTTPErrorHandler(c, errors.New("boom"))
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("unexpected status for generic error: %d", rec.Code)
		}
	}

	{
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		customHTTPErrorHandler(c, echo.NewHTTPError(http.StatusBadRequest, "bad request payload"))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("unexpected status for http error: %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "bad request payload") {
			t.Fatalf("unexpected payload: %s", rec.Body.String())
		}
	}
}

func TestNewServerRegisterAndShutdown(t *testing.T) {
	e := newAPITestEnv(t)
	e.cfg.Env = "dev"
	e.cfg.HTTP.ExposeSwagger = true
	e.cfg.HTTP.Addr = "127.0.0.1:0"
	e.cfg.HTTP.ShutdownTimeout = time.Second

	m := metrics.New(e.cfg.Metrics.Namespace)
	deps := Dependencies{
		Config:               e.cfg,
		Metrics:              m,
		JWT:                  e.handler.jwt,
		Users:                e.users,
		Tenants:              e.tenants,
		Memberships:          e.memberships,
		RefreshTokens:        repository.NewRefreshTokenRepository(e.pool),
		Alerts:               e.alerts,
		Cases:                e.cases,
		Tasks:                e.tasks,
		Observables:          e.observables,
		CaseEvents:           e.caseEvents,
		CasePages:            e.casePages,
		Attachments:          e.attachments,
		Catalog:              e.catalog,
		System:               repository.NewSystemRepository(e.pool),
		Audits:               repository.NewAuditRepository(e.pool),
		ConnectorIngest:      repository.NewConnectorIngestStateRepository(e.pool),
		InboundConnectorRuns: repository.NewInboundConnectorRunRepository(e.pool),
		ForumBindings:        repository.NewForumExternalBindingRepository(e.pool),
		AIChats:              e.aiChats,
		CaseAIAnalyses:       e.caseAI,
		ArtifactStorage:      e.storageStub,
		AI:                   e.handler.ai,
	}

	srv := NewServer(deps)
	if srv == nil || srv.Echo() == nil {
		t.Fatalf("expected initialized server")
	}

	routes := srv.Echo().Router().Routes()
	if len(routes) == 0 {
		t.Fatalf("expected registered routes")
	}

	foundHealth := false
	foundSwagger := false
	for _, route := range routes {
		if route.Path == "/healthz" {
			foundHealth = true
		}
		if route.Path == "/dev/swagger" {
			foundSwagger = true
		}
	}
	if !foundHealth {
		t.Fatalf("health route not registered")
	}
	if !foundSwagger {
		t.Fatalf("swagger route not registered in dev")
	}

	runCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := srv.Run(runCtx); err != nil {
		t.Fatalf("run with canceled context should shutdown cleanly: %v", err)
	}
	if err := srv.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown server: %v", err)
	}
}

func TestNewServerRegistersSwaggerWhenExplicitlyEnabled(t *testing.T) {
	e := newAPITestEnv(t)
	e.cfg.Env = "prod"
	e.cfg.HTTP.ExposeSwagger = true

	srv := NewServer(Dependencies{Config: e.cfg})
	routes := srv.Echo().Router().Routes()

	foundSwagger := false
	for _, route := range routes {
		if route.Path == "/swagger" {
			foundSwagger = true
			break
		}
	}
	if !foundSwagger {
		t.Fatalf("swagger route not registered when HTTP_EXPOSE_SWAGGER=true")
	}
}

func TestHandlerUtilityMethodsWithCatalog(t *testing.T) {
	env := newAPITestEnv(t)
	h := env.handler

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	h.setRefreshCookie(c, "token-value", time.Now().Add(time.Hour))
	if got := rec.Result().Cookies(); len(got) == 0 || got[0].Name != env.cfg.Auth.CookieName {
		t.Fatalf("expected refresh cookie to be set")
	}

	tenantID := env.tenantID
	caseItem, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          tenantID,
		CaseNumber:        "CASE-UTIL-1",
		Title:             "Utility case",
		Description:       "for utility testing",
		Source:            "manual",
		IncidentType:      "generic",
		Status:            "open",
		Priority:          "medium",
		Impact:            "system",
		Confidence:        20,
		Severity:          "medium",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.userID,
	})
	if err != nil {
		t.Fatalf("create utility case: %v", err)
	}

	forumID := uuid.New()
	if upsertErr := h.upsertCaseForumLink(context.Background(), tenantID, caseItem.ID, forumID, env.userID); upsertErr != nil {
		t.Fatalf("upsert case forum link create: %v", upsertErr)
	}
	if upsertErr := h.upsertCaseForumLink(context.Background(), tenantID, caseItem.ID, uuid.New(), env.userID); upsertErr != nil {
		t.Fatalf("upsert case forum link update: %v", upsertErr)
	}

	if got := firstNonEmptyString("", " ", "x", "y"); got != "x" {
		t.Fatalf("unexpected first non-empty: %q", got)
	}
	if got := sanitizeAttachmentName("../A Test*Name?.txt"); !strings.Contains(got, "A_Test_Name_.txt") {
		t.Fatalf("unexpected sanitized attachment name: %q", got)
	}
	if got := sanitizeAttachmentName(" "); got != "artifact.bin" {
		t.Fatalf("unexpected fallback attachment name: %q", got)
	}
	if got := h.maxAttachmentBytes(); got <= 0 {
		t.Fatalf("max attachment bytes must be positive")
	}

	catalogItems, err := env.catalog.List(context.Background(), repository.CatalogListParams{
		Kind:     "case_meta",
		TenantID: &tenantID,
		RefID:    &caseItem.ID,
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("list catalog items: %v", err)
	}
	if len(mapCatalogItems(catalogItems)) == 0 {
		t.Fatalf("expected mapped catalog payload")
	}
}

func TestResolveCaseInTenant(t *testing.T) {
	env := newAPITestEnv(t)
	caseItem, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-RESOLVE-1",
		Title:             "resolve tenant case",
		Description:       "resolve helper",
		Source:            "manual",
		IncidentType:      "generic",
		Status:            "open",
		Priority:          "medium",
		Impact:            "system",
		Confidence:        10,
		Severity:          "low",
		TLP:               "green",
		PAP:               "green",
		ResolutionSummary: "",
		CreatedBy:         env.userID,
	})
	if err != nil {
		t.Fatalf("create case: %v", err)
	}

	c, _ := env.jsonContext(http.MethodGet, "/api/v1/cases/"+caseItem.ID.String(), nil)
	setPath(c, "/api/v1/cases/:caseID", []string{"caseID"}, []string{caseItem.ID.String()})
	setTenant(c, env.tenantID)

	tenantID, gotCaseID, err := env.handler.resolveCaseInTenant(c)
	if err != nil {
		t.Fatalf("resolveCaseInTenant: %v", err)
	}
	if tenantID != env.tenantID || gotCaseID != caseItem.ID {
		t.Fatalf("resolveCaseInTenant returned unexpected ids")
	}
}

func TestNewHandlerUsesDependencies(t *testing.T) {
	env := newAPITestEnv(t)
	h := NewHandler(Dependencies{
		Config:               env.cfg,
		JWT:                  env.handler.jwt,
		Users:                env.users,
		Tenants:              env.tenants,
		Memberships:          env.memberships,
		RefreshTokens:        repository.NewRefreshTokenRepository(env.pool),
		Alerts:               env.alerts,
		Cases:                env.cases,
		Tasks:                env.tasks,
		Observables:          env.observables,
		CaseEvents:           env.caseEvents,
		CasePages:            env.casePages,
		Attachments:          env.attachments,
		Catalog:              env.catalog,
		System:               repository.NewSystemRepository(env.pool),
		Audits:               repository.NewAuditRepository(env.pool),
		ConnectorIngest:      repository.NewConnectorIngestStateRepository(env.pool),
		InboundConnectorRuns: repository.NewInboundConnectorRunRepository(env.pool),
		ForumBindings:        repository.NewForumExternalBindingRepository(env.pool),
		AIChats:              env.aiChats,
		CaseAIAnalyses:       env.caseAI,
		ArtifactStorage:      env.storageStub,
		AI:                   env.handler.ai,
	})
	if h == nil {
		t.Fatalf("expected handler instance")
	}
	h.search = env.searchStub
}

func TestParseRoleRejectsUnknown(t *testing.T) {
	if _, err := parseRole("unknown-role"); err == nil {
		t.Fatalf("expected parseRole to reject unknown role")
	}
}

func TestDefaultMaxAttachmentBytesFallback(t *testing.T) {
	h := &Handler{cfg: config.App{}}
	if got := h.maxAttachmentBytes(); got != 50*1024*1024 {
		t.Fatalf("unexpected fallback max attachment bytes: %d", got)
	}
}

func TestAuthAttemptAndReverseMessages(t *testing.T) {
	collector := metrics.New("incidenthub_test")
	h := &Handler{metrics: collector}
	h.authAttempt("success")
	h.authAttempt("failure")

	items := []models.AIChatMessage{
		{Role: "user", Content: "first"},
		{Role: "assistant", Content: "second"},
		{Role: "user", Content: "third"},
	}
	reverseMessages(items)
	if items[0].Content != "third" || items[2].Content != "first" {
		t.Fatalf("reverseMessages did not reverse list")
	}
}
