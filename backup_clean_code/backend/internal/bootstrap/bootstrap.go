package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"
	"incidenthub/backend/internal/security"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Sentinel errors for bootstrap (err113).
var (
	errBootstrapDepsNotConfigured = errors.New("bootstrap dependencies are not configured")
)

type Dependencies struct {
	Users       *repository.UserRepository
	Tenants     *repository.TenantRepository
	Memberships *repository.MembershipRepository
	Cases       *repository.CaseRepository
	Alerts      *repository.AlertRepository
	Catalog     *repository.CatalogRepository
}

func EnsurePlatformAdmin(ctx context.Context, cfg config.App, deps Dependencies) error {
	if !cfg.Bootstrap.Enabled {
		return nil
	}
	if deps.Users == nil || deps.Tenants == nil || deps.Memberships == nil {
		return errBootstrapDepsNotConfigured
	}

	adminTenant, err := deps.Tenants.Ensure(ctx, repository.CreateTenantParams{
		Slug:        "admin",
		Name:        "Admin",
		Description: "System administration tenant",
	})
	if err != nil {
		return err
	}

	email := strings.TrimSpace(strings.ToLower(cfg.Bootstrap.PlatformAdminEmail))
	if email == "" {
		email = "admin@incidenthub.local"
	}
	username := strings.TrimSpace(cfg.Bootstrap.PlatformAdminUser)
	if username == "" {
		username = "platform-admin"
	}
	fullName := strings.TrimSpace(cfg.Bootstrap.PlatformAdminName)
	if fullName == "" {
		fullName = "Platform Administrator"
	}
	password := strings.TrimSpace(cfg.Bootstrap.PlatformAdminPass)
	if password == "" {
		password = "ChangeMeNow123!"
	}
	passwordHash, hashErr := security.HashPassword(password)
	if hashErr != nil {
		return hashErr
	}

	user, err := deps.Users.GetByEmail(ctx, email)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		byUsername, usernameErr := deps.Users.GetByUsername(ctx, username)
		if usernameErr == nil {
			user = byUsername
		} else if !errors.Is(usernameErr, pgx.ErrNoRows) {
			return usernameErr
		}
	}

	if user == nil {
		created, createErr := deps.Users.Create(ctx, repository.CreateUserParams{
			Username:        username,
			Email:           email,
			FullName:        fullName,
			PasswordHash:    passwordHash,
			IsPlatformAdmin: true,
			LDAPEnabled:     false,
		})
		if createErr != nil {
			return createErr
		}
		user = created
	}

	if !user.IsPlatformAdmin {
		if err := deps.Users.SetPlatformAdmin(ctx, user.ID, true); err != nil {
			return err
		}
	}

	if err := deps.Memberships.Upsert(ctx, adminTenant.ID, user.ID, models.TenantRoleAdmin); err != nil {
		return err
	}
	if cfg.Bootstrap.SeedDemoData {
		if deps.Cases != nil {
			if err := ensureDemoCases(ctx, deps.Cases, adminTenant.ID, user.ID); err != nil {
				return err
			}
		}
		if deps.Alerts != nil {
			if err := ensureDemoAlerts(ctx, deps.Alerts, adminTenant.ID, user.ID); err != nil {
				return err
			}
		}
	}
	if deps.Catalog != nil {
		if err := ensureDemoAchievements(ctx, deps.Catalog, adminTenant.ID, user.ID); err != nil {
			return err
		}
	}
	return nil
}

func ensureDemoCases(ctx context.Context, cases *repository.CaseRepository, tenantID, userID uuid.UUID) error {
	existing, err := cases.ListByTenant(ctx, tenantID, 200, 0)
	if err != nil {
		return err
	}

	byNumber := map[string]struct{}{}
	for _, item := range existing {
		if strings.TrimSpace(item.CaseNumber) == "" {
			continue
		}
		byNumber[item.CaseNumber] = struct{}{}
	}

	seeds := []repository.CreateCaseParams{
		{
			TenantID:     tenantID,
			CaseNumber:   "DEMO-0001",
			Title:        "Suspicious OAuth Application Consent",
			Description:  "User granted consent to an untrusted OAuth application with high-privilege scopes.",
			Source:       "siem",
			IncidentType: "identity_compromise",
			Status:       "investigating",
			Priority:     "high",
			Impact:       "email_access",
			Confidence:   82,
			Severity:     "high",
			TLP:          "amber",
			PAP:          "amber",
			CreatedBy:    userID,
			AssignedTo:   &userID,
		},
		{
			TenantID:          tenantID,
			CaseNumber:        "DEMO-0002",
			Title:             "Phishing Campaign with Credential Harvesting",
			Description:       "Multiple users reported a phishing email that redirects to a fake SSO page.",
			Source:            "mail-gateway",
			IncidentType:      "phishing",
			Status:            "contained",
			Priority:          "high",
			Impact:            "account_takeover",
			Confidence:        89,
			Severity:          "critical",
			TLP:               "amber",
			PAP:               "amber",
			ResolutionSummary: "IOC set blocked on secure mail gateway.",
			CreatedBy:         userID,
			AssignedTo:        &userID,
		},
		{
			TenantID:     tenantID,
			CaseNumber:   "DEMO-0003",
			Title:        "Endpoint Malware Detection",
			Description:  "EDR detected suspicious PowerShell execution with encoded command line.",
			Source:       "edr",
			IncidentType: "malware",
			Status:       "new",
			Priority:     "medium",
			Impact:       "endpoint",
			Confidence:   64,
			Severity:     "medium",
			TLP:          "green",
			PAP:          "amber",
			CreatedBy:    userID,
			AssignedTo:   &userID,
		},
	}

	for _, seed := range seeds {
		if _, exists := byNumber[seed.CaseNumber]; exists {
			continue
		}
		if _, createErr := cases.Create(ctx, seed); createErr != nil {
			if isCaseNumberDuplicate(createErr) {
				continue
			}
			return createErr
		}
	}
	return nil
}

func isCaseNumberDuplicate(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23505" && pgErr.ConstraintName == "uq_cases_tenant_case_number"
}

func ensureDemoAlerts(ctx context.Context, alerts *repository.AlertRepository, tenantID, userID uuid.UUID) error {
	existing, err := alerts.ListByTenant(ctx, tenantID, 500, 0)
	if err != nil {
		return err
	}

	keys := make(map[string]struct{}, len(existing))
	for _, item := range existing {
		key := strings.TrimSpace(strings.ToLower(item.Source)) + "::" + strings.TrimSpace(strings.ToLower(item.Title))
		if key != "::" {
			keys[key] = struct{}{}
		}
	}

	seeds := []repository.CreateAlertParams{
		{
			TenantID:    tenantID,
			Title:       "Potential MFA Fatigue Attack",
			Description: "Identity provider recorded repeated push MFA prompts for a privileged account.",
			Source:      "identity-provider",
			Status:      "new",
			Severity:    "high",
			TLP:         "amber",
			PAP:         "amber",
			CreatedBy:   &userID,
			AssignedTo:  &userID,
		},
		{
			TenantID:    tenantID,
			Title:       "Suspicious PowerShell Download Cradle",
			Description: "EDR observed encoded PowerShell command downloading payload from unknown host.",
			Source:      "edr",
			Status:      "new",
			Severity:    "critical",
			TLP:         "amber",
			PAP:         "amber",
			CreatedBy:   &userID,
			AssignedTo:  &userID,
		},
		{
			TenantID:    tenantID,
			Title:       "Impossible Travel Sign-in",
			Description: "Authentication activity shows impossible travel between countries in short interval.",
			Source:      "siem",
			Status:      "triaged",
			Severity:    "medium",
			TLP:         "green",
			PAP:         "amber",
			CreatedBy:   &userID,
			AssignedTo:  &userID,
		},
		{
			TenantID:    tenantID,
			Title:       "High-Risk Attachment Blocked",
			Description: "Mail gateway blocked executable attachment delivered to finance group.",
			Source:      "mail-gateway",
			Status:      "new",
			Severity:    "high",
			TLP:         "amber",
			PAP:         "amber",
			CreatedBy:   &userID,
			AssignedTo:  &userID,
		},
		{
			TenantID:    tenantID,
			Title:       "Repeated Access to Disabled Account",
			Description: "Multiple authentication attempts detected for disabled user account.",
			Source:      "iam",
			Status:      "new",
			Severity:    "low",
			TLP:         "green",
			PAP:         "green",
			CreatedBy:   &userID,
			AssignedTo:  &userID,
		},
	}

	for _, seed := range seeds {
		key := strings.TrimSpace(strings.ToLower(seed.Source)) + "::" + strings.TrimSpace(strings.ToLower(seed.Title))
		if _, exists := keys[key]; exists {
			continue
		}
		if _, createErr := alerts.Create(ctx, seed); createErr != nil {
			return createErr
		}
	}

	return nil
}

func ensureDemoAchievements(ctx context.Context, catalog *repository.CatalogRepository, tenantID, userID uuid.UUID) error {
	achievementSeeds := []map[string]any{
		{
			"slug":        "diamond-sentinel",
			"name":        "Diamond Sentinel",
			"description": "Closed 100+ critical incidents with zero SLA breach.",
			"rarity":      "Diamond",
			"icon":        "💎",
			"xp_reward":   500,
		},
		{
			"slug":        "legendary-hunter",
			"name":        "Legendary Hunter",
			"description": "Detected and contained a multi-stage intrusion campaign.",
			"rarity":      "Legendary",
			"icon":        "🏆",
			"xp_reward":   300,
		},
		{
			"slug":        "epic-responder",
			"name":        "Epic Responder",
			"description": "Resolved 50 incidents under target response time.",
			"rarity":      "Epic",
			"icon":        "⚡",
			"xp_reward":   220,
		},
		{
			"slug":        "rare-connector-master",
			"name":        "Connector Master",
			"description": "Executed 500 successful connector actions.",
			"rarity":      "Rare",
			"icon":        "🔌",
			"xp_reward":   140,
		},
		{
			"slug":        "common-first-response",
			"name":        "First Response",
			"description": "Completed your first incident workflow.",
			"rarity":      "Common",
			"icon":        "🛡️",
			"xp_reward":   80,
		},
	}

	existingAchievements, err := catalog.List(ctx, repository.CatalogListParams{
		Kind:          "achievements",
		TenantID:      &tenantID,
		IncludeGlobal: true,
		Limit:         300,
	})
	if err != nil {
		return err
	}
	achievementIDBySlug := make(map[string]uuid.UUID, len(existingAchievements))
	for _, item := range existingAchievements {
		slug := strings.TrimSpace(strings.ToLower(fmtValue(item.Data["slug"])))
		if slug == "" {
			slug = strings.TrimSpace(strings.ToLower(fmtValue(item.Data["name"])))
		}
		if slug == "" {
			continue
		}
		achievementIDBySlug[slug] = item.ID
	}

	for _, seed := range achievementSeeds {
		slug := strings.TrimSpace(strings.ToLower(fmtValue(seed["slug"])))
		if slug == "" {
			continue
		}
		if _, exists := achievementIDBySlug[slug]; exists {
			continue
		}
		item, createErr := catalog.Create(ctx, repository.CatalogCreateParams{
			TenantID:  &tenantID,
			Kind:      "achievements",
			Data:      seed,
			CreatedBy: &userID,
		})
		if createErr != nil {
			return createErr
		}
		achievementIDBySlug[slug] = item.ID
	}

	existingGrants, err := catalog.List(ctx, repository.CatalogListParams{
		Kind:     "user_achievements",
		TenantID: &tenantID,
		OwnerID:  &userID,
		Limit:    300,
	})
	if err != nil {
		return err
	}
	grantedAchievementIDs := make(map[string]struct{}, len(existingGrants))
	for _, item := range existingGrants {
		achievementID := strings.TrimSpace(fmtValue(item.Data["achievement_id"]))
		if achievementID == "" {
			achievementID = strings.TrimSpace(fmtValue(item.Data["achievementId"]))
		}
		if achievementID == "" {
			continue
		}
		grantedAchievementIDs[achievementID] = struct{}{}
	}

	for _, seed := range achievementSeeds {
		slug := strings.TrimSpace(strings.ToLower(fmtValue(seed["slug"])))
		achievementID, exists := achievementIDBySlug[slug]
		if !exists {
			continue
		}
		achievementIDStr := achievementID.String()
		if _, granted := grantedAchievementIDs[achievementIDStr]; granted {
			continue
		}
		if _, createErr := catalog.Create(ctx, repository.CatalogCreateParams{
			TenantID:  &tenantID,
			Kind:      "user_achievements",
			OwnerID:   &userID,
			Data:      map[string]any{"achievement_id": achievementIDStr, "granted_at": time.Now().UTC().Format(time.RFC3339)},
			CreatedBy: &userID,
		}); createErr != nil {
			return createErr
		}
		grantedAchievementIDs[achievementIDStr] = struct{}{}
	}

	return nil
}

func fmtValue(value any) string {
	return strings.TrimSpace(strings.ToLower(fmt.Sprintf("%v", value)))
}
