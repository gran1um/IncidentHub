package repository

import (
	"context"
	"testing"
	"time"

	"incidenthub/backend/internal/models"

	"github.com/google/uuid"
)

func TestTenantUserMembershipRepositories(t *testing.T) {
	env := newRepositoryTestEnv(t)
	ctx := context.Background()

	tenantByID, err := env.tenants.GetByID(ctx, env.tenantID)
	if err != nil {
		t.Fatalf("get tenant by id: %v", err)
	}
	if tenantByID.Slug != "repo-tenant-main" {
		t.Fatalf("unexpected tenant slug: %s", tenantByID.Slug)
	}

	tenantBySlug, err := env.tenants.GetBySlug(ctx, "repo-tenant-main")
	if err != nil {
		t.Fatalf("get tenant by slug: %v", err)
	}
	if tenantBySlug.ID != env.tenantID {
		t.Fatalf("unexpected tenant by slug id")
	}

	tenantList, err := env.tenants.List(ctx, 10, 0)
	if err != nil {
		t.Fatalf("list tenants: %v", err)
	}
	if len(tenantList) < 2 {
		t.Fatalf("expected at least 2 tenants, got %d", len(tenantList))
	}

	ensuredExisting, err := env.tenants.Ensure(ctx, CreateTenantParams{
		Slug: "repo-tenant-main",
		Name: "Ignored",
	})
	if err != nil {
		t.Fatalf("ensure existing tenant: %v", err)
	}
	if ensuredExisting.ID != env.tenantID {
		t.Fatalf("ensure existing should return same tenant id")
	}

	ensuredNew, err := env.tenants.Ensure(ctx, CreateTenantParams{
		Slug:        "repo-tenant-third",
		Name:        "Repository Tenant Third",
		Description: "third tenant",
		MaxUsers:    50,
		IsActive:    true,
	})
	if err != nil {
		t.Fatalf("ensure new tenant: %v", err)
	}
	if ensuredNew.ID == uuid.Nil {
		t.Fatalf("ensure new should create tenant")
	}

	updatedTenant, err := env.tenants.Update(ctx, env.tenantID, UpdateTenantParams{
		Name:     strPtr("Repository Tenant Main Updated"),
		MaxUsers: intPtr(120),
	})
	if err != nil {
		t.Fatalf("update tenant: %v", err)
	}
	if updatedTenant.Name != "Repository Tenant Main Updated" || updatedTenant.MaxUsers != 120 {
		t.Fatalf("tenant update not persisted: %#v", updatedTenant)
	}

	userByEmail, err := env.users.GetByEmail(ctx, "repo-user-main@example.com")
	if err != nil {
		t.Fatalf("get user by email: %v", err)
	}
	if userByEmail.ID != env.userID {
		t.Fatalf("unexpected user by email id")
	}

	userByID, err := env.users.GetByID(ctx, env.secondUserID)
	if err != nil {
		t.Fatalf("get user by id: %v", err)
	}
	if userByID.Username != "repo-user-second" {
		t.Fatalf("unexpected second user username: %s", userByID.Username)
	}

	userList, err := env.users.List(ctx, 50, 0)
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	if len(userList) < 2 {
		t.Fatalf("expected at least 2 users, got %d", len(userList))
	}

	usersByTenant, err := env.users.ListByTenant(ctx, env.tenantID, 50, 0)
	if err != nil {
		t.Fatalf("list users by tenant: %v", err)
	}
	if len(usersByTenant) < 2 {
		t.Fatalf("expected at least 2 users in tenant, got %d", len(usersByTenant))
	}

	tenantUsers, err := env.users.ListByTenantWithRole(ctx, env.tenantID, 50, 0)
	if err != nil {
		t.Fatalf("list tenant users with role: %v", err)
	}
	if len(tenantUsers) < 2 {
		t.Fatalf("expected at least 2 tenant users with role, got %d", len(tenantUsers))
	}

	if touchErr := env.users.TouchLogin(ctx, env.userID); touchErr != nil {
		t.Fatalf("touch login: %v", touchErr)
	}

	updatedProfile, err := env.users.UpdateProfile(
		ctx,
		env.userID,
		strPtr("Main User Updated"),
		strPtr("repo-user-main-updated@example.com"),
		strPtr("SOC"),
		strPtr("https://example.local/avatar.png"),
		strPtr("https://example.local/cover.png"),
		strPtr("https://example.local/profile"),
	)
	if err != nil {
		t.Fatalf("update profile: %v", err)
	}
	if updatedProfile.FullName != "Main User Updated" || updatedProfile.Team != "SOC" {
		t.Fatalf("unexpected updated profile: %#v", updatedProfile)
	}

	if updateErr := env.users.UpdatePassword(ctx, env.userID, env.passwordHash); updateErr != nil {
		t.Fatalf("update password: %v", updateErr)
	}

	if upsertErr := env.memberships.Upsert(ctx, env.secondTenant, env.secondUserID, models.TenantRoleViewer); upsertErr != nil {
		t.Fatalf("upsert second membership: %v", upsertErr)
	}
	membership, err := env.memberships.Get(ctx, env.tenantID, env.userID)
	if err != nil {
		t.Fatalf("get membership: %v", err)
	}
	if membership.Role != models.TenantRoleAdmin {
		t.Fatalf("unexpected membership role: %s", membership.Role)
	}

	membershipList, err := env.memberships.ListByUser(ctx, env.secondUserID)
	if err != nil {
		t.Fatalf("list memberships by user: %v", err)
	}
	if len(membershipList) < 2 {
		t.Fatalf("expected at least 2 memberships for second user, got %d", len(membershipList))
	}
}

func TestAlertCaseTaskObservableAndCaseOpsRepositories(t *testing.T) {
	env := newRepositoryTestEnv(t)
	ctx := context.Background()

	alertOne, err := env.alerts.Create(ctx, CreateAlertParams{
		TenantID:    env.tenantID,
		Title:       "Alert one",
		Description: "first alert",
		Source:      "siem",
		Status:      "new",
		Severity:    "high",
		TLP:         "amber",
		PAP:         "amber",
		CreatedBy:   &env.userID,
	})
	if err != nil {
		t.Fatalf("create alert one: %v", err)
	}
	alertTwo, err := env.alerts.Create(ctx, CreateAlertParams{
		TenantID:    env.tenantID,
		Title:       "Alert two",
		Description: "second alert",
		Source:      "edr",
		Status:      "new",
		Severity:    "medium",
		TLP:         "green",
		PAP:         "green",
		CreatedBy:   &env.userID,
	})
	if err != nil {
		t.Fatalf("create alert two: %v", err)
	}

	alertsList, err := env.alerts.ListByTenant(ctx, env.tenantID, 50, 0)
	if err != nil {
		t.Fatalf("list alerts: %v", err)
	}
	if len(alertsList) < 2 {
		t.Fatalf("expected at least 2 alerts, got %d", len(alertsList))
	}

	gotAlert, err := env.alerts.GetByID(ctx, env.tenantID, alertOne.ID)
	if err != nil {
		t.Fatalf("get alert by id: %v", err)
	}
	if gotAlert.ID != alertOne.ID {
		t.Fatalf("unexpected alert id")
	}

	alertsByIDs, err := env.alerts.ListByIDs(ctx, env.tenantID, []uuid.UUID{alertOne.ID, alertTwo.ID})
	if err != nil {
		t.Fatalf("list alerts by ids: %v", err)
	}
	if len(alertsByIDs) != 2 {
		t.Fatalf("expected 2 alerts by ids, got %d", len(alertsByIDs))
	}

	mainCase := env.createCase("CASE-REPO-001")
	boundAlerts, err := env.alerts.BindToCase(ctx, env.tenantID, []uuid.UUID{alertOne.ID}, &mainCase.ID)
	if err != nil {
		t.Fatalf("bind alert to case: %v", err)
	}
	if len(boundAlerts) != 1 || boundAlerts[0].CaseID == nil || *boundAlerts[0].CaseID != mainCase.ID {
		t.Fatalf("alert bind to case not persisted: %#v", boundAlerts)
	}

	updatedAlert, err := env.alerts.Update(ctx, env.tenantID, alertOne.ID, UpdateAlertParams{
		Status:   strPtr("closed"),
		Severity: strPtr("low"),
	})
	if err != nil {
		t.Fatalf("update alert: %v", err)
	}
	if updatedAlert.Status != "closed" || updatedAlert.Severity != "low" {
		t.Fatalf("unexpected updated alert: %#v", updatedAlert)
	}

	caseWithLinkedAlerts, linked, err := env.cases.CreateWithLinkedAlerts(ctx, CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-REPO-002",
		Title:             "Case with linked alerts",
		Description:       "link alerts",
		Source:            "manual",
		IncidentType:      "generic",
		Status:            "open",
		Priority:          "high",
		Impact:            "system",
		Confidence:        70,
		Severity:          "high",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.userID,
	}, []uuid.UUID{alertTwo.ID})
	if err != nil {
		t.Fatalf("create case with linked alerts: %v", err)
	}
	if caseWithLinkedAlerts.ID == uuid.Nil || len(linked) != 1 {
		t.Fatalf("expected linked alert in case creation")
	}

	caseList, err := env.cases.ListByTenant(ctx, env.tenantID, 100, 0)
	if err != nil {
		t.Fatalf("list cases by tenant: %v", err)
	}
	if len(caseList) < 2 {
		t.Fatalf("expected at least 2 cases, got %d", len(caseList))
	}

	exists, err := env.cases.ExistsInTenant(ctx, mainCase.ID, env.tenantID)
	if err != nil {
		t.Fatalf("exists in tenant: %v", err)
	}
	if !exists {
		t.Fatalf("expected case to exist in tenant")
	}

	gotCase, err := env.cases.GetByID(ctx, env.tenantID, mainCase.ID)
	if err != nil {
		t.Fatalf("get case by id: %v", err)
	}
	if gotCase.ID != mainCase.ID {
		t.Fatalf("unexpected case id")
	}

	openCount, err := env.cases.CountInWorkByTenant(ctx, env.tenantID, []string{"open", "new"})
	if err != nil {
		t.Fatalf("count in-work cases by open statuses: %v", err)
	}
	if openCount < 2 {
		t.Fatalf("expected at least 2 open cases before resolution, got %d", openCount)
	}

	updatedCase, err := env.cases.Update(ctx, env.tenantID, mainCase.ID, UpdateCaseParams{
		Title:       strPtr("Repository Case Updated"),
		Status:      strPtr("resolved"),
		Confidence:  intPtr(90),
		AssignedTo:  &env.userID,
		Description: strPtr("Updated description"),
	})
	if err != nil {
		t.Fatalf("update case: %v", err)
	}
	if updatedCase.Title != "Repository Case Updated" || updatedCase.Status != "resolved" || updatedCase.Confidence != 90 {
		t.Fatalf("unexpected updated case: %#v", updatedCase)
	}

	openAfterResolve, err := env.cases.CountInWorkByTenant(ctx, env.tenantID, []string{"open", "new"})
	if err != nil {
		t.Fatalf("count in-work after resolve: %v", err)
	}
	if openAfterResolve < 1 {
		t.Fatalf("expected at least 1 open case after resolving one, got %d", openAfterResolve)
	}

	fallbackOpenCount, err := env.cases.CountInWorkByTenant(ctx, env.tenantID, nil)
	if err != nil {
		t.Fatalf("count in-work by fallback statuses: %v", err)
	}
	if fallbackOpenCount < 1 {
		t.Fatalf("expected fallback open count to be positive, got %d", fallbackOpenCount)
	}

	_, err = env.cases.Create(ctx, CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-REPO-ASSIGN-001",
		Title:             "Assignment workload one",
		Description:       "workload",
		Source:            "siem",
		IncidentType:      "phishing",
		Status:            "analysis",
		Priority:          "medium",
		Impact:            "medium",
		Confidence:        60,
		Severity:          "medium",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.userID,
		AssignedTo:        &env.secondUserID,
	})
	if err != nil {
		t.Fatalf("seed assignment case one: %v", err)
	}
	_, err = env.cases.Create(ctx, CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-REPO-ASSIGN-002",
		Title:             "Assignment workload two",
		Description:       "workload",
		Source:            "siem",
		IncidentType:      "phishing",
		Status:            "resolved",
		Priority:          "medium",
		Impact:            "medium",
		Confidence:        60,
		Severity:          "medium",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.userID,
		AssignedTo:        &env.secondUserID,
	})
	if err != nil {
		t.Fatalf("seed assignment case two: %v", err)
	}
	workloadByAssignee, err := env.cases.CountOpenByAssignees(ctx, env.tenantID, []uuid.UUID{env.userID, env.secondUserID}, []string{"open", "analysis"})
	if err != nil {
		t.Fatalf("count open by assignees: %v", err)
	}
	if workloadByAssignee[env.secondUserID] < 1 {
		t.Fatalf("expected second user to have open workload, got %d", workloadByAssignee[env.secondUserID])
	}
	if workloadByAssignee[env.userID] != 0 {
		t.Fatalf("expected main user open workload 0, got %d", workloadByAssignee[env.userID])
	}
	similarAssignee, err := env.cases.FindOpenSimilarAssignee(ctx, env.tenantID, []uuid.UUID{env.userID, env.secondUserID}, "siem", "phishing", []string{"open", "analysis"})
	if err != nil {
		t.Fatalf("find similar open assignee: %v", err)
	}
	if similarAssignee == nil || *similarAssignee != env.secondUserID {
		t.Fatalf("expected similar assignee %s, got %#v", env.secondUserID, similarAssignee)
	}

	dueAt := time.Now().UTC().Add(90 * time.Minute)
	task, err := env.tasks.Create(ctx, CreateTaskParams{
		TenantID:    env.tenantID,
		CaseID:      mainCase.ID,
		Title:       "Collect logs",
		Description: "Collect endpoint logs",
		Status:      "todo",
		AssigneeID:  &env.secondUserID,
		DueDate:     &dueAt,
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if task.DueDate == nil || task.DueDate.IsZero() {
		t.Fatalf("expected task due date to be persisted")
	}

	tasksByTenant, err := env.tasks.ListByTenant(ctx, env.tenantID, 100, 0)
	if err != nil {
		t.Fatalf("list tasks by tenant: %v", err)
	}
	if len(tasksByTenant) == 0 {
		t.Fatalf("expected tasks in tenant")
	}

	tasksByCase, err := env.tasks.ListByCase(ctx, env.tenantID, mainCase.ID, 100, 0)
	if err != nil {
		t.Fatalf("list tasks by case: %v", err)
	}
	if len(tasksByCase) == 0 {
		t.Fatalf("expected tasks in case")
	}

	updatedTask, err := env.tasks.Update(ctx, env.tenantID, task.ID, UpdateTaskParams{
		Status:     strPtr("done"),
		AssigneeID: &env.userID,
		DueDateSet: true,
		DueDate:    nil,
	})
	if err != nil {
		t.Fatalf("update task: %v", err)
	}
	if updatedTask.Status != "done" {
		t.Fatalf("unexpected updated task: %#v", updatedTask)
	}
	if updatedTask.DueDate != nil {
		t.Fatalf("expected updated task due date to be cleared")
	}

	if deleteErr := env.tasks.Delete(ctx, env.tenantID, task.ID); deleteErr != nil {
		t.Fatalf("delete task: %v", deleteErr)
	}

	observable, err := env.observables.Create(ctx, CreateObservableParams{
		TenantID:  env.tenantID,
		CaseID:    mainCase.ID,
		Type:      "ip",
		Value:     "10.20.30.40",
		Verdict:   "unknown",
		Source:    "manual",
		Tags:      []string{"ioc"},
		CreatedBy: env.userID,
	})
	if err != nil {
		t.Fatalf("create observable: %v", err)
	}

	observablesList, err := env.observables.ListByCase(ctx, env.tenantID, mainCase.ID, 100, 0)
	if err != nil {
		t.Fatalf("list observables: %v", err)
	}
	if len(observablesList) == 0 {
		t.Fatalf("expected observables in case")
	}

	updatedObservable, err := env.observables.Update(ctx, env.tenantID, mainCase.ID, observable.ID, UpdateObservableParams{
		Verdict: strPtr("malicious"),
		Tags:    &[]string{"ioc", "critical"},
	})
	if err != nil {
		t.Fatalf("update observable: %v", err)
	}
	if updatedObservable.Verdict != "malicious" {
		t.Fatalf("unexpected updated observable: %#v", updatedObservable)
	}

	if deleteErr := env.observables.Delete(ctx, env.tenantID, mainCase.ID, observable.ID); deleteErr != nil {
		t.Fatalf("delete observable: %v", deleteErr)
	}

	event, err := env.caseEvents.Create(ctx, CreateCaseEventParams{
		TenantID:  env.tenantID,
		CaseID:    mainCase.ID,
		EventType: "note",
		Title:     "Initial note",
		Body:      "Case started",
		ActorID:   &env.userID,
		Metadata:  map[string]any{"phase": "triage"},
	})
	if err != nil {
		t.Fatalf("create case event: %v", err)
	}
	if event.ID == uuid.Nil {
		t.Fatalf("created event id must not be nil")
	}

	eventsList, err := env.caseEvents.ListByCase(ctx, env.tenantID, mainCase.ID, 100, 0)
	if err != nil {
		t.Fatalf("list case events: %v", err)
	}
	if len(eventsList) == 0 {
		t.Fatalf("expected case events")
	}

	page, err := env.casePages.Create(ctx, CreateCasePageParams{
		TenantID:  env.tenantID,
		CaseID:    mainCase.ID,
		Title:     "Executive Summary",
		Body:      "Summary body",
		CreatedBy: env.userID,
	})
	if err != nil {
		t.Fatalf("create case page: %v", err)
	}
	if page.ID == uuid.Nil {
		t.Fatalf("created page id must not be nil")
	}

	pagesList, err := env.casePages.ListByCase(ctx, env.tenantID, mainCase.ID, 100, 0)
	if err != nil {
		t.Fatalf("list case pages: %v", err)
	}
	if len(pagesList) == 0 {
		t.Fatalf("expected case pages")
	}

	attachment, err := env.attachments.Create(ctx, CreateAttachmentParams{
		TenantID:      env.tenantID,
		CaseID:        mainCase.ID,
		FileName:      "artifact.txt",
		ContentType:   "text/plain",
		FileSizeBytes: 128,
		StorageKey:    "tenant/repo/case/artifact.txt",
		UploadedBy:    env.userID,
	})
	if err != nil {
		t.Fatalf("create attachment: %v", err)
	}
	if attachment.ID == uuid.Nil {
		t.Fatalf("created attachment id must not be nil")
	}

	attachmentsList, err := env.attachments.ListByCase(ctx, env.tenantID, mainCase.ID, 100, 0)
	if err != nil {
		t.Fatalf("list attachments: %v", err)
	}
	if len(attachmentsList) == 0 {
		t.Fatalf("expected attachments in case")
	}

	gotAttachment, err := env.attachments.GetByID(ctx, env.tenantID, mainCase.ID, attachment.ID)
	if err != nil {
		t.Fatalf("get attachment by id: %v", err)
	}
	if gotAttachment.ID != attachment.ID {
		t.Fatalf("unexpected attachment id")
	}
}

func TestCatalogAuditAndSystemRepositories(t *testing.T) {
	env := newRepositoryTestEnv(t)
	ctx := context.Background()
	mainCase := env.createCase("CASE-REPO-003")

	globalRateLimit, err := env.catalog.Create(ctx, CatalogCreateParams{
		Kind: "rate_limits",
		Data: map[string]any{"name": "global-rate-limit"},
	})
	if err != nil {
		t.Fatalf("create global catalog item: %v", err)
	}
	_, err = env.catalog.Create(ctx, CatalogCreateParams{
		TenantID:  &env.tenantID,
		Kind:      "rate_limits",
		OwnerID:   &env.userID,
		CreatedBy: &env.userID,
		Data:      map[string]any{"name": "tenant-rate-limit"},
	})
	if err != nil {
		t.Fatalf("create tenant rate limit item: %v", err)
	}
	tenantForumThread, err := env.catalog.Create(ctx, CatalogCreateParams{
		TenantID:  &env.tenantID,
		Kind:      "forum_thread",
		OwnerID:   &env.userID,
		RefID:     &mainCase.ID,
		CreatedBy: &env.userID,
		Data:      map[string]any{"title": "thread"},
	})
	if err != nil {
		t.Fatalf("create tenant forum thread item: %v", err)
	}
	_, err = env.catalog.Create(ctx, CatalogCreateParams{
		TenantID:  &env.tenantID,
		Kind:      "api_tokens",
		OwnerID:   &env.userID,
		CreatedBy: &env.userID,
		Data:      map[string]any{"name": "token-1"},
	})
	if err != nil {
		t.Fatalf("create api token item: %v", err)
	}

	itemsWithGlobal, err := env.catalog.List(ctx, CatalogListParams{
		Kind:          "rate_limits",
		TenantID:      &env.tenantID,
		IncludeGlobal: true,
		Limit:         50,
	})
	if err != nil {
		t.Fatalf("list catalog include global: %v", err)
	}
	if len(itemsWithGlobal) == 0 {
		t.Fatalf("expected include-global catalog items")
	}

	gotThread, err := env.catalog.GetByID(ctx, "forum_thread", tenantForumThread.ID, &env.tenantID)
	if err != nil {
		t.Fatalf("get catalog by id: %v", err)
	}
	if gotThread.ID != tenantForumThread.ID {
		t.Fatalf("unexpected catalog item id")
	}

	updatedGlobal, err := env.catalog.Update(ctx, "rate_limits", globalRateLimit.ID, nil, CatalogUpdateParams{
		Data: map[string]any{"name": "global-rate-limit-updated"},
	})
	if err != nil {
		t.Fatalf("update global catalog item: %v", err)
	}
	if updatedGlobal.Data["name"] != "global-rate-limit-updated" {
		t.Fatalf("unexpected updated catalog data: %#v", updatedGlobal.Data)
	}

	allRateLimits, err := env.catalog.ListByKindAllTenants(ctx, "rate_limits", 100, 0)
	if err != nil {
		t.Fatalf("list catalog kind all tenants: %v", err)
	}
	if len(allRateLimits) == 0 {
		t.Fatalf("expected catalog items in all-tenant list")
	}

	if deleteErr := env.catalog.Delete(ctx, "rate_limits", globalRateLimit.ID, nil); deleteErr != nil {
		t.Fatalf("delete global catalog item: %v", deleteErr)
	}

	if logErr := env.audits.Log(ctx, &env.tenantID, &env.userID, "case_view", "case", &mainCase.ID, map[string]any{"path": "/cases"}); logErr != nil {
		t.Fatalf("write audit log: %v", logErr)
	}

	auditItems, err := env.audits.ListByTenant(ctx, env.tenantID, 100, 0)
	if err != nil {
		t.Fatalf("list audit logs: %v", err)
	}
	if len(auditItems) == 0 {
		t.Fatalf("expected audit logs")
	}

	if pingErr := env.system.Ping(ctx); pingErr != nil {
		t.Fatalf("system ping: %v", pingErr)
	}
	stats := env.system.PoolStats()
	if stats.MaxConns < 0 || stats.TotalConns < 0 {
		t.Fatalf("unexpected pool stats: %#v", stats)
	}

	dashboardStats, err := env.system.TenantDashboardStats(ctx, env.tenantID)
	if err != nil {
		t.Fatalf("tenant dashboard stats: %v", err)
	}
	_ = dashboardStats

	advancedMetrics, err := env.system.TenantDashboardMetrics(ctx, env.tenantID, []string{"open", "new"}, []string{"resolved", "closed"}, 24*time.Hour)
	if err != nil {
		t.Fatalf("tenant advanced dashboard metrics: %v", err)
	}
	if advancedMetrics.OpenCases < 1 {
		t.Fatalf("expected open cases in advanced metrics, got %d", advancedMetrics.OpenCases)
	}

	customMetricValue, err := env.system.EvaluateDashboardCustomMetric(ctx, env.tenantID, []string{"open", "new"}, []string{"resolved", "closed"}, DashboardCustomMetricDefinition{
		Source:   "cases",
		Measure:  "count",
		Statuses: []string{"open"},
	})
	if err != nil {
		t.Fatalf("evaluate dashboard custom metric: %v", err)
	}
	if customMetricValue < 1 {
		t.Fatalf("expected custom metric value >= 1, got %f", customMetricValue)
	}

	resourceStats, err := env.system.TenantResourceStats(ctx, env.tenantID)
	if err != nil {
		t.Fatalf("tenant resource stats: %v", err)
	}
	if resourceStats.ActiveUsers == 0 {
		t.Fatalf("expected active users > 0 in resource stats")
	}
}
