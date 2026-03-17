package bootstrap

import (
	"context"
	"fmt"
	"testing"

	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"
	"incidenthub/backend/internal/testutil"
)

func TestEnsurePlatformAdminDisabled(t *testing.T) {
	cfg := config.App{
		Bootstrap: config.BootstrapConfig{
			Enabled: false,
		},
	}
	if err := EnsurePlatformAdmin(context.Background(), cfg, Dependencies{}); err != nil {
		t.Fatalf("disabled bootstrap should not fail: %v", err)
	}
}

func TestEnsurePlatformAdminCreatesUserMembershipAndDemoCases(t *testing.T) {
	pool := testutil.OpenTestPool(t)
	testutil.ResetPublicTables(t, pool)
	ctx := context.Background()

	users := repository.NewUserRepository(pool)
	tenants := repository.NewTenantRepository(pool)
	memberships := repository.NewMembershipRepository(pool)
	cases := repository.NewCaseRepository(pool)
	alerts := repository.NewAlertRepository(pool)

	cfg := config.App{
		Bootstrap: config.BootstrapConfig{
			Enabled:            true,
			SeedDemoData:       true,
			PlatformAdminEmail: "bootstrap-admin@example.com",
			PlatformAdminUser:  "bootstrap-admin",
			PlatformAdminPass:  "Password123!",
			PlatformAdminName:  "Bootstrap Admin",
		},
	}

	deps := Dependencies{
		Users:       users,
		Tenants:     tenants,
		Memberships: memberships,
		Cases:       cases,
		Alerts:      alerts,
	}

	if ensureErr := EnsurePlatformAdmin(ctx, cfg, deps); ensureErr != nil {
		t.Fatalf("ensure platform admin first run: %v", ensureErr)
	}
	if ensureErr := EnsurePlatformAdmin(ctx, cfg, deps); ensureErr != nil {
		t.Fatalf("ensure platform admin second run: %v", ensureErr)
	}

	adminTenant, err := tenants.GetBySlug(ctx, "admin")
	if err != nil {
		t.Fatalf("load admin tenant: %v", err)
	}
	adminUser, err := users.GetByEmail(ctx, "bootstrap-admin@example.com")
	if err != nil {
		t.Fatalf("load admin user: %v", err)
	}
	if !adminUser.IsPlatformAdmin {
		t.Fatalf("bootstrap admin should be platform admin")
	}

	membership, err := memberships.Get(ctx, adminTenant.ID, adminUser.ID)
	if err != nil {
		t.Fatalf("load bootstrap membership: %v", err)
	}
	if membership.Role != models.TenantRoleAdmin {
		t.Fatalf("unexpected bootstrap membership role: %s", membership.Role)
	}

	seedCases, err := cases.ListByTenant(ctx, adminTenant.ID, 100, 0)
	if err != nil {
		t.Fatalf("list seeded demo cases: %v", err)
	}
	if len(seedCases) != 3 {
		t.Fatalf("expected 3 demo cases after idempotent bootstrap, got %d", len(seedCases))
	}

	seedAlerts, err := alerts.ListByTenant(ctx, adminTenant.ID, 200, 0)
	if err != nil {
		t.Fatalf("list seeded demo alerts: %v", err)
	}
	if len(seedAlerts) != 5 {
		t.Fatalf("expected 5 demo alerts after idempotent bootstrap, got %d", len(seedAlerts))
	}
}

func TestEnsurePlatformAdminDoesNotOverrideExistingProfileMedia(t *testing.T) {
	pool := testutil.OpenTestPool(t)
	testutil.ResetPublicTables(t, pool)
	ctx := context.Background()

	users := repository.NewUserRepository(pool)
	tenants := repository.NewTenantRepository(pool)
	memberships := repository.NewMembershipRepository(pool)
	cases := repository.NewCaseRepository(pool)
	alerts := repository.NewAlertRepository(pool)

	created, err := users.Create(ctx, repository.CreateUserParams{
		Username:        "platform-admin",
		Email:           "admin@incidenthub.local",
		FullName:        "Platform Admin",
		PasswordHash:    "$2a$10$wXo5/1Qn42gcJ68dSMcrp.OAze48FDM9qokxv3M4fYpJXKo0xqV8e",
		IsPlatformAdmin: false,
		LDAPEnabled:     false,
	})
	if err != nil {
		t.Fatalf("create existing admin user: %v", err)
	}
	avatar := "http://localhost:9000/incidenthub-artifacts/avatar/admin.png"
	cover := "http://localhost:9000/incidenthub-artifacts/cover/admin.png"
	if _, updateErr := users.UpdateProfile(ctx, created.ID, nil, nil, nil, &avatar, &cover, nil); updateErr != nil {
		t.Fatalf("set media fields: %v", updateErr)
	}

	cfg := config.App{
		Bootstrap: config.BootstrapConfig{
			Enabled:            true,
			PlatformAdminEmail: "admin@incidenthub.local",
			PlatformAdminUser:  "platform-admin",
			PlatformAdminPass:  "ChangeMeNow123!",
			PlatformAdminName:  "Platform Administrator",
		},
	}
	deps := Dependencies{
		Users:       users,
		Tenants:     tenants,
		Memberships: memberships,
		Cases:       cases,
		Alerts:      alerts,
	}
	if ensureErr := EnsurePlatformAdmin(ctx, cfg, deps); ensureErr != nil {
		t.Fatalf("ensure platform admin: %v", ensureErr)
	}

	after, err := users.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("load updated user: %v", err)
	}
	if after.AvatarURL != avatar {
		t.Fatalf("avatar url should be preserved, got %q", after.AvatarURL)
	}
	if after.CoverImageURL != cover {
		t.Fatalf("cover image url should be preserved, got %q", after.CoverImageURL)
	}
	if !after.IsPlatformAdmin {
		t.Fatal("existing bootstrap user should be elevated to platform admin")
	}
}

func TestEnsurePlatformAdminUsesSafeDefaultsWhenConfigEmpty(t *testing.T) {
	pool := testutil.OpenTestPool(t)
	testutil.ResetPublicTables(t, pool)
	ctx := context.Background()

	users := repository.NewUserRepository(pool)
	tenants := repository.NewTenantRepository(pool)
	memberships := repository.NewMembershipRepository(pool)

	cfg := config.App{
		Bootstrap: config.BootstrapConfig{
			Enabled: true,
		},
	}
	if err := EnsurePlatformAdmin(ctx, cfg, Dependencies{
		Users:       users,
		Tenants:     tenants,
		Memberships: memberships,
	}); err != nil {
		t.Fatalf("ensure platform admin with defaults: %v", err)
	}

	admin, err := users.GetByEmail(ctx, "admin@incidenthub.local")
	if err != nil {
		t.Fatalf("load default bootstrap admin: %v", err)
	}
	if admin.Username != "platform-admin" {
		t.Fatalf("expected default username platform-admin, got %s", admin.Username)
	}
}

func TestEnsurePlatformAdminSkipsDuplicateDemoCasesOutsideListWindow(t *testing.T) {
	pool := testutil.OpenTestPool(t)
	testutil.ResetPublicTables(t, pool)
	ctx := context.Background()

	users := repository.NewUserRepository(pool)
	tenants := repository.NewTenantRepository(pool)
	memberships := repository.NewMembershipRepository(pool)
	cases := repository.NewCaseRepository(pool)
	alerts := repository.NewAlertRepository(pool)

	cfg := config.App{
		Bootstrap: config.BootstrapConfig{
			Enabled:            true,
			SeedDemoData:       true,
			PlatformAdminEmail: "bootstrap-window@example.com",
			PlatformAdminUser:  "bootstrap-window",
			PlatformAdminPass:  "Password123!",
			PlatformAdminName:  "Bootstrap Window",
		},
	}
	deps := Dependencies{
		Users:       users,
		Tenants:     tenants,
		Memberships: memberships,
		Cases:       cases,
		Alerts:      alerts,
	}

	if ensureErr := EnsurePlatformAdmin(ctx, cfg, deps); ensureErr != nil {
		t.Fatalf("ensure platform admin initial run: %v", ensureErr)
	}

	adminTenant, err := tenants.GetBySlug(ctx, "admin")
	if err != nil {
		t.Fatalf("load admin tenant: %v", err)
	}
	adminUser, err := users.GetByEmail(ctx, "bootstrap-window@example.com")
	if err != nil {
		t.Fatalf("load bootstrap user: %v", err)
	}

	for i := 0; i < 230; i++ {
		_, createErr := cases.Create(ctx, repository.CreateCaseParams{
			TenantID:     adminTenant.ID,
			CaseNumber:   fmt.Sprintf("LOAD-%04d", i),
			Title:        fmt.Sprintf("Load Case %d", i),
			Description:  "bootstrap idempotency load fixture",
			Source:       "loadtest",
			IncidentType: "load_fixture",
			Status:       "new",
			Priority:     "low",
			Impact:       "none",
			Confidence:   10,
			Severity:     "low",
			TLP:          "green",
			PAP:          "green",
			CreatedBy:    adminUser.ID,
			AssignedTo:   &adminUser.ID,
		})
		if createErr != nil {
			t.Fatalf("create load case %d: %v", i, createErr)
		}
	}

	if ensureErr := EnsurePlatformAdmin(ctx, cfg, deps); ensureErr != nil {
		t.Fatalf("ensure platform admin second run with >200 cases: %v", err)
	}

	for _, demoCaseNumber := range []string{"DEMO-0001", "DEMO-0002", "DEMO-0003"} {
		var count int
		if err := pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM cases WHERE tenant_id = $1 AND case_number = $2`,
			adminTenant.ID,
			demoCaseNumber,
		).Scan(&count); err != nil {
			t.Fatalf("count demo case %s: %v", demoCaseNumber, err)
		}
		if count != 1 {
			t.Fatalf("expected exactly one %s case, got %d", demoCaseNumber, count)
		}
	}
}
