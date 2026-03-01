package repository

import (
	"context"
	"testing"

	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/security"
	"incidenthub/backend/internal/testutil"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type repositoryTestEnv struct {
	t    *testing.T
	pool *pgxpool.Pool

	users       *UserRepository
	tenants     *TenantRepository
	memberships *MembershipRepository
	alerts      *AlertRepository
	cases       *CaseRepository
	tasks       *TaskRepository
	observables *ObservableRepository
	caseEvents  *CaseEventRepository
	casePages   *CasePageRepository
	attachments *AttachmentRepository
	catalog     *CatalogRepository
	audits      *AuditRepository
	refresh     *RefreshTokenRepository
	system      *SystemRepository
	ingest      *ConnectorIngestStateRepository
	inboundRuns *InboundConnectorRunRepository
	bindings    *ForumExternalBindingRepository
	aiChats     *AIChatRepository
	caseAI      *CaseAIAnalysisRepository

	tenantID     uuid.UUID
	secondTenant uuid.UUID
	userID       uuid.UUID
	secondUserID uuid.UUID
	passwordHash string
}

func newRepositoryTestEnv(t *testing.T) *repositoryTestEnv {
	t.Helper()

	pool := testutil.OpenTestPool(t)
	testutil.ResetPublicTables(t, pool)

	users := NewUserRepository(pool)
	tenants := NewTenantRepository(pool)
	memberships := NewMembershipRepository(pool)
	alerts := NewAlertRepository(pool)
	cases := NewCaseRepository(pool)
	tasks := NewTaskRepository(pool)
	observables := NewObservableRepository(pool)
	caseEvents := NewCaseEventRepository(pool)
	casePages := NewCasePageRepository(pool)
	attachments := NewAttachmentRepository(pool)
	catalog := NewCatalogRepository(pool)
	audits := NewAuditRepository(pool)
	refresh := NewRefreshTokenRepository(pool)
	system := NewSystemRepository(pool)
	ingest := NewConnectorIngestStateRepository(pool)
	inboundRuns := NewInboundConnectorRunRepository(pool)
	bindings := NewForumExternalBindingRepository(pool)
	aiChats := NewAIChatRepository(pool)
	caseAI := NewCaseAIAnalysisRepository(pool)

	passwordHash, err := security.HashPassword("Password123!")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	tenant, err := tenants.Create(context.Background(), CreateTenantParams{
		Slug:        "repo-tenant-main",
		Name:        "Repository Tenant Main",
		Description: "main tenant",
		MaxUsers:    100,
		IsActive:    true,
	})
	if err != nil {
		t.Fatalf("create tenant main: %v", err)
	}
	secondTenant, err := tenants.Create(context.Background(), CreateTenantParams{
		Slug:        "repo-tenant-second",
		Name:        "Repository Tenant Second",
		Description: "second tenant",
		MaxUsers:    100,
		IsActive:    true,
	})
	if err != nil {
		t.Fatalf("create tenant second: %v", err)
	}

	user, err := users.Create(context.Background(), CreateUserParams{
		Username:        "repo-user-main",
		Email:           "repo-user-main@example.com",
		FullName:        "Repo User Main",
		PasswordHash:    passwordHash,
		IsPlatformAdmin: false,
	})
	if err != nil {
		t.Fatalf("create user main: %v", err)
	}
	secondUser, err := users.Create(context.Background(), CreateUserParams{
		Username:        "repo-user-second",
		Email:           "repo-user-second@example.com",
		FullName:        "Repo User Second",
		PasswordHash:    passwordHash,
		IsPlatformAdmin: false,
	})
	if err != nil {
		t.Fatalf("create user second: %v", err)
	}

	if err := memberships.Upsert(context.Background(), tenant.ID, user.ID, models.TenantRoleAdmin); err != nil {
		t.Fatalf("upsert membership main: %v", err)
	}
	if err := memberships.Upsert(context.Background(), tenant.ID, secondUser.ID, models.TenantRoleAnalyst); err != nil {
		t.Fatalf("upsert membership second: %v", err)
	}

	return &repositoryTestEnv{
		t:            t,
		pool:         pool,
		users:        users,
		tenants:      tenants,
		memberships:  memberships,
		alerts:       alerts,
		cases:        cases,
		tasks:        tasks,
		observables:  observables,
		caseEvents:   caseEvents,
		casePages:    casePages,
		attachments:  attachments,
		catalog:      catalog,
		audits:       audits,
		refresh:      refresh,
		system:       system,
		ingest:       ingest,
		inboundRuns:  inboundRuns,
		bindings:     bindings,
		aiChats:      aiChats,
		caseAI:       caseAI,
		tenantID:     tenant.ID,
		secondTenant: secondTenant.ID,
		userID:       user.ID,
		secondUserID: secondUser.ID,
		passwordHash: passwordHash,
	}
}

func (env *repositoryTestEnv) createCase(caseNumber string) *models.Case {
	env.t.Helper()
	item, err := env.cases.Create(context.Background(), CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        caseNumber,
		Title:             "Repository case " + caseNumber,
		Description:       "Repository case description",
		Source:            "manual",
		IncidentType:      "generic",
		Status:            "open",
		Priority:          "medium",
		Impact:            "system",
		Confidence:        50,
		Severity:          "medium",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.userID,
		AssignedTo:        &env.secondUserID,
	})
	if err != nil {
		env.t.Fatalf("create case %s: %v", caseNumber, err)
	}
	return item
}

func strPtr(v string) *string { return &v }

func intPtr(v int) *int { return &v }
