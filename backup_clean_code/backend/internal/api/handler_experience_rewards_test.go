package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"incidenthub/backend/internal/repository"

	"github.com/labstack/echo/v5"
)

func TestCreateCatalogUserAchievementAwardsExperience(t *testing.T) {
	env := newAPITestEnv(t)

	achievementXP := 333
	achievement, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID: &env.tenantID,
		Kind:     "achievements",
		Data: map[string]any{
			"name":        "XP Test Achievement",
			"description": "Should grant XP",
			"icon":        "🏅",
			"rarity":      "Rare",
			"xp_reward":   achievementXP,
		},
		CreatedBy: &env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create achievement seed: %v", err)
	}

	c, rec := env.jsonContext(http.MethodPost, "/api/v1/catalog/user_achievements", map[string]any{
		"owner_id": env.secondUserID.String(),
		"data": map[string]any{
			"achievement_id": achievement.ID.String(),
			"granted_at":     time.Now().UTC().Format(time.RFC3339),
		},
	})
	setPath(c, "/api/v1/catalog/:kind", []string{"kind"}, []string{"user_achievements"})
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)
	err = env.handler.CreateCatalogItem(c)
	mustStatusOK(t, err, rec, http.StatusCreated)

	user, err := env.users.GetByID(context.Background(), env.secondUserID)
	if err != nil {
		t.Fatalf("load awarded user: %v", err)
	}
	if user.ExperiencePoints != int64(achievementXP) {
		t.Fatalf("expected user xp=%d, got %d", achievementXP, user.ExperiencePoints)
	}

	c, rec = env.jsonContext(http.MethodGet, "/api/v1/users/"+env.secondUserID.String()+"/experience-events?limit=10", nil)
	setPath(c, "/api/v1/users/:id/experience-events", []string{"id"}, []string{env.secondUserID.String()})
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)
	err = env.handler.ListUserExperienceEvents(c)
	mustStatusOK(t, err, rec, http.StatusOK)

	events := decodeBody[[]map[string]any](t, rec)
	if len(events) == 0 {
		t.Fatalf("expected experience history to contain achievement event")
	}
	if gotDescription := events[0]["description"]; gotDescription == nil || gotDescription == "" {
		t.Fatalf("expected non-empty experience event description")
	}
}

func TestCaseClosureAwardsExperienceOnlyFirstTime(t *testing.T) {
	env := newAPITestEnv(t)

	caseItem, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-XP-CLOSE-1",
		Title:             "XP close test",
		Description:       "test case",
		Source:            "manual",
		IncidentType:      "investigation",
		Status:            "open",
		Priority:          "high",
		Impact:            "high",
		Confidence:        70,
		Severity:          "high",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create case seed: %v", err)
	}

	closeCase := func(status string) {
		c, rec := env.jsonContext(http.MethodPatch, "/api/v1/cases/"+caseItem.ID.String(), map[string]any{
			"status": status,
		})
		setPath(c, "/api/v1/cases/:caseID", []string{"caseID"}, []string{caseItem.ID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		updateErr := env.handler.UpdateCase(c)
		mustStatusOK(t, updateErr, rec, http.StatusOK)
	}

	closeCase("closed")

	userAfterFirstClose, err := env.users.GetByID(context.Background(), env.identity.UserID)
	if err != nil {
		t.Fatalf("load user after first close: %v", err)
	}
	const expectedHighCloseReward = 350
	if userAfterFirstClose.ExperiencePoints != expectedHighCloseReward {
		t.Fatalf("expected xp=%d after first close, got %d", expectedHighCloseReward, userAfterFirstClose.ExperiencePoints)
	}

	closeCase("open")
	closeCase("closed")

	userAfterSecondClose, err := env.users.GetByID(context.Background(), env.identity.UserID)
	if err != nil {
		t.Fatalf("load user after second close: %v", err)
	}
	if userAfterSecondClose.ExperiencePoints != expectedHighCloseReward {
		t.Fatalf("expected xp to stay at %d after reopen/reclose, got %d", expectedHighCloseReward, userAfterSecondClose.ExperiencePoints)
	}

	c, rec := env.jsonContext(http.MethodGet, "/api/v1/users/"+env.identity.UserID.String()+"/experience-events?limit=10", nil)
	setPath(c, "/api/v1/users/:id/experience-events", []string{"id"}, []string{env.identity.UserID.String()})
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)
	err = env.handler.ListUserExperienceEvents(c)
	mustStatusOK(t, err, rec, http.StatusOK)
	events := decodeBody[[]map[string]any](t, rec)
	if len(events) == 0 {
		t.Fatalf("expected case closure experience event in history")
	}
	if events[0]["description"] == nil || events[0]["description"] == "" {
		t.Fatalf("expected case closure event to have description")
	}
}

func TestAwardUserExperienceManualGrantWithDescription(t *testing.T) {
	env := newAPITestEnv(t)

	const points = 777
	const description = "Exceptional work during incident response"

	c, rec := env.jsonContext(http.MethodPost, "/api/v1/users/"+env.secondUserID.String()+"/experience-awards", map[string]any{
		"points":      points,
		"description": description,
	})
	setPath(c, "/api/v1/users/:id/experience-awards", []string{"id"}, []string{env.secondUserID.String()})
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)
	err := env.handler.AwardUserExperience(c)
	mustStatusOK(t, err, rec, http.StatusOK)

	user, err := env.users.GetByID(context.Background(), env.secondUserID)
	if err != nil {
		t.Fatalf("load user after manual xp award: %v", err)
	}
	if user.ExperiencePoints != points {
		t.Fatalf("expected user xp=%d after manual grant, got %d", points, user.ExperiencePoints)
	}

	historyReq, historyRec := env.jsonContext(http.MethodGet, "/api/v1/users/"+env.secondUserID.String()+"/experience-events?limit=10", nil)
	setPath(historyReq, "/api/v1/users/:id/experience-events", []string{"id"}, []string{env.secondUserID.String()})
	setIdentity(historyReq, env.identity)
	setTenant(historyReq, env.tenantID)
	err = env.handler.ListUserExperienceEvents(historyReq)
	mustStatusOK(t, err, historyRec, http.StatusOK)

	events := decodeBody[[]map[string]any](t, historyRec)
	if len(events) == 0 {
		t.Fatalf("expected manual grant event in history")
	}
	if gotType := events[0]["event_type"]; gotType != xpEventTypeManualGrant {
		t.Fatalf("expected event_type=%q, got %v", xpEventTypeManualGrant, gotType)
	}
	if gotDescription := events[0]["description"]; gotDescription != description {
		t.Fatalf("expected description %q, got %v", description, gotDescription)
	}
	if gotPoints := int(events[0]["points"].(float64)); gotPoints != points {
		t.Fatalf("expected points %d, got %d", points, gotPoints)
	}
}

func TestListUserExperienceEventsSupportsOffset(t *testing.T) {
	env := newAPITestEnv(t)

	award := func(points int, description string) {
		req, rec := env.jsonContext(http.MethodPost, "/api/v1/users/"+env.secondUserID.String()+"/experience-awards", map[string]any{
			"points":      points,
			"description": description,
		})
		setPath(req, "/api/v1/users/:id/experience-awards", []string{"id"}, []string{env.secondUserID.String()})
		setIdentity(req, env.identity)
		setTenant(req, env.tenantID)
		err := env.handler.AwardUserExperience(req)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	for i := 1; i <= 3; i++ {
		award(100*i, fmt.Sprintf("event-%d", i))
		time.Sleep(2 * time.Millisecond)
	}

	firstPageReq, firstPageRec := env.jsonContext(http.MethodGet, "/api/v1/users/"+env.secondUserID.String()+"/experience-events?limit=2&offset=0", nil)
	setPath(firstPageReq, "/api/v1/users/:id/experience-events", []string{"id"}, []string{env.secondUserID.String()})
	setIdentity(firstPageReq, env.identity)
	setTenant(firstPageReq, env.tenantID)
	err := env.handler.ListUserExperienceEvents(firstPageReq)
	mustStatusOK(t, err, firstPageRec, http.StatusOK)

	firstPage := decodeBody[[]map[string]any](t, firstPageRec)
	if len(firstPage) != 2 {
		t.Fatalf("expected first page with 2 events, got %d", len(firstPage))
	}
	if got := firstPage[0]["description"]; got != "event-3" {
		t.Fatalf("expected most recent event description to be event-3, got %v", got)
	}
	if got := firstPage[1]["description"]; got != "event-2" {
		t.Fatalf("expected second event description to be event-2, got %v", got)
	}

	secondPageReq, secondPageRec := env.jsonContext(http.MethodGet, "/api/v1/users/"+env.secondUserID.String()+"/experience-events?limit=2&offset=2", nil)
	setPath(secondPageReq, "/api/v1/users/:id/experience-events", []string{"id"}, []string{env.secondUserID.String()})
	setIdentity(secondPageReq, env.identity)
	setTenant(secondPageReq, env.tenantID)
	err = env.handler.ListUserExperienceEvents(secondPageReq)
	mustStatusOK(t, err, secondPageRec, http.StatusOK)

	secondPage := decodeBody[[]map[string]any](t, secondPageRec)
	if len(secondPage) != 1 {
		t.Fatalf("expected second page with 1 event, got %d", len(secondPage))
	}
	if got := secondPage[0]["description"]; got != "event-1" {
		t.Fatalf("expected third event description to be event-1, got %v", got)
	}

	invalidOffsetReq, _ := env.jsonContext(http.MethodGet, "/api/v1/users/"+env.secondUserID.String()+"/experience-events?limit=2&offset=-1", nil)
	setPath(invalidOffsetReq, "/api/v1/users/:id/experience-events", []string{"id"}, []string{env.secondUserID.String()})
	setIdentity(invalidOffsetReq, env.identity)
	setTenant(invalidOffsetReq, env.tenantID)
	err = env.handler.ListUserExperienceEvents(invalidOffsetReq)
	if err == nil {
		t.Fatalf("expected invalid offset request to fail")
	}
	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected *echo.HTTPError, got %T", err)
	}
	if httpErr.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: got=%d want=%d", httpErr.Code, http.StatusBadRequest)
	}
}

func TestGetUserCasePerformance(t *testing.T) {
	env := newAPITestEnv(t)

	caseItem, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-PERF-1",
		Title:             "Performance close test",
		Description:       "test case",
		Source:            "manual",
		IncidentType:      "investigation",
		Status:            "open",
		Priority:          "medium",
		Impact:            "medium",
		Confidence:        70,
		Severity:          "high",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create case seed: %v", err)
	}
	if _, execErr := env.pool.Exec(context.Background(), `UPDATE cases SET created_at = created_at - INTERVAL '2 hours' WHERE id = $1`, caseItem.ID); execErr != nil {
		t.Fatalf("backdate case created_at: %v", execErr)
	}

	closeReq, closeRec := env.jsonContext(http.MethodPatch, "/api/v1/cases/"+caseItem.ID.String(), map[string]any{
		"status": "closed",
	})
	setPath(closeReq, "/api/v1/cases/:caseID", []string{"caseID"}, []string{caseItem.ID.String()})
	setIdentity(closeReq, env.identity)
	setTenant(closeReq, env.tenantID)
	err = env.handler.UpdateCase(closeReq)
	mustStatusOK(t, err, closeRec, http.StatusOK)

	req, rec := env.jsonContext(http.MethodGet, "/api/v1/users/"+env.identity.UserID.String()+"/performance", nil)
	setPath(req, "/api/v1/users/:id/performance", []string{"id"}, []string{env.identity.UserID.String()})
	setIdentity(req, env.identity)
	setTenant(req, env.tenantID)
	err = env.handler.GetUserCasePerformance(req)
	mustStatusOK(t, err, rec, http.StatusOK)

	payload := decodeBody[map[string]any](t, rec)
	if int(payload["closed_cases_total"].(float64)) != 1 {
		t.Fatalf("expected closed_cases_total=1, got %v", payload["closed_cases_total"])
	}
	if int(payload["current_month_closed_cases"].(float64)) < 1 {
		t.Fatalf("expected current_month_closed_cases >= 1, got %v", payload["current_month_closed_cases"])
	}
	if payload["avg_investigation_minutes"] == nil {
		t.Fatalf("expected avg_investigation_minutes to be present")
	}
}
