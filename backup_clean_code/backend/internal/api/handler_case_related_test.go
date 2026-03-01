package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

func TestListRelatedCasesSplitsActiveRecentAndAllTime(t *testing.T) {
	env := newAPITestEnv(t)
	ctx := context.Background()

	sourceCase := createCaseFixture(t, env, env.tenantID, env.userID, "CASE-REL-SOURCE", "Source case", "open")
	createObservableFixture(t, env, env.tenantID, sourceCase.ID, env.userID, "ip", "1.1.1.1")
	createObservableFixture(t, env, env.tenantID, sourceCase.ID, env.userID, "domain", "malicious.example")

	activeCase := createCaseFixture(t, env, env.tenantID, env.userID, "CASE-REL-ACTIVE", "Active related case", "open")
	createObservableFixture(t, env, env.tenantID, activeCase.ID, env.userID, "ip", "1.1.1.1")

	historicalCase := createCaseFixture(t, env, env.tenantID, env.userID, "CASE-REL-HISTORY", "Historical related case", "closed")
	createObservableFixture(t, env, env.tenantID, historicalCase.ID, env.userID, "domain", "malicious.example")
	if _, err := env.pool.Exec(ctx, `UPDATE cases SET updated_at = NOW() - INTERVAL '90 days' WHERE id = $1`, historicalCase.ID); err != nil {
		t.Fatalf("set historical case updated_at: %v", err)
	}

	nonRelatedCase := createCaseFixture(t, env, env.tenantID, env.userID, "CASE-REL-NONE", "Not related", "open")
	createObservableFixture(t, env, env.tenantID, nonRelatedCase.ID, env.userID, "ip", "8.8.8.8")

	c, rec := env.jsonContext(http.MethodGet, "/api/v1/cases/"+sourceCase.ID.String()+"/related", nil)
	setPath(c, "/api/v1/cases/:caseID/related", []string{"caseID"}, []string{sourceCase.ID.String()})
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)
	err := env.handler.ListRelatedCases(c)
	mustStatusOK(t, err, rec, http.StatusOK)

	payload := decodeBody[map[string]any](t, rec)
	activeRecentRows := anySlice(payload["active_recent"])
	allTimeRows := anySlice(payload["all_time"])

	if len(activeRecentRows) != 1 {
		t.Fatalf("expected 1 active/recent related case, got %d", len(activeRecentRows))
	}
	if !containsRelatedCaseID(activeRecentRows, activeCase.ID.String()) {
		t.Fatalf("expected active case %s in active_recent", activeCase.ID)
	}
	if containsRelatedCaseID(activeRecentRows, historicalCase.ID.String()) {
		t.Fatalf("historical case %s must not be present in active_recent", historicalCase.ID)
	}

	if len(allTimeRows) != 2 {
		t.Fatalf("expected 2 related cases in all_time, got %d", len(allTimeRows))
	}
	if !containsRelatedCaseID(allTimeRows, activeCase.ID.String()) {
		t.Fatalf("expected active case %s in all_time", activeCase.ID)
	}
	if !containsRelatedCaseID(allTimeRows, historicalCase.ID.String()) {
		t.Fatalf("expected historical case %s in all_time", historicalCase.ID)
	}
	if containsRelatedCaseID(allTimeRows, nonRelatedCase.ID.String()) {
		t.Fatalf("non-related case %s must not be present in all_time", nonRelatedCase.ID)
	}

	activeEntry := relatedCaseByID(activeRecentRows, activeCase.ID.String())
	if matchCount := toInt(activeEntry["match_count"]); matchCount <= 0 {
		t.Fatalf("expected positive match_count for active case, got %d", matchCount)
	}
}

func TestListRelatedCasesRespectsTenantAccess(t *testing.T) {
	env := newAPITestEnv(t)
	ctx := context.Background()

	if err := env.memberships.Upsert(ctx, env.secondTenant, env.secondUserID, models.TenantRoleAnalyst); err != nil {
		t.Fatalf("upsert second tenant membership: %v", err)
	}

	sourceCase := createCaseFixture(t, env, env.tenantID, env.userID, "CASE-REL-ACCESS-SRC", "Source shared case", "open")
	createObservableFixture(t, env, env.tenantID, sourceCase.ID, env.userID, "ip", "10.0.0.1")

	ownerOnlyRelatedCase := createCaseFixture(t, env, env.tenantID, env.userID, "CASE-REL-OWNER", "Owner-only related", "open")
	createObservableFixture(t, env, env.tenantID, ownerOnlyRelatedCase.ID, env.userID, "ip", "10.0.0.1")

	secondTenantRelatedCase := createCaseFixture(t, env, env.secondTenant, env.secondUserID, "CASE-REL-SECOND", "Second tenant related", "open")
	createObservableFixture(t, env, env.secondTenant, secondTenantRelatedCase.ID, env.secondUserID, "ip", "10.0.0.1")

	if err := env.cases.ShareWithTenant(ctx, env.tenantID, sourceCase.ID, env.secondTenant, &env.identity.UserID); err != nil {
		t.Fatalf("share source case with second tenant: %v", err)
	}

	secondTenantIdentity := models.Identity{
		UserID:     env.secondUserID,
		Username:   "analyst-second",
		TenantID:   &env.secondTenant,
		TenantRole: models.TenantRoleAnalyst,
	}

	c, rec := env.jsonContext(http.MethodGet, "/api/v1/cases/"+sourceCase.ID.String()+"/related", nil)
	setPath(c, "/api/v1/cases/:caseID/related", []string{"caseID"}, []string{sourceCase.ID.String()})
	setIdentity(c, secondTenantIdentity)
	setTenant(c, env.secondTenant)
	err := env.handler.ListRelatedCases(c)
	mustStatusOK(t, err, rec, http.StatusOK)

	payload := decodeBody[map[string]any](t, rec)
	allTimeRows := anySlice(payload["all_time"])
	if len(allTimeRows) != 1 {
		t.Fatalf("expected only one accessible related case for second tenant, got %d", len(allTimeRows))
	}
	if !containsRelatedCaseID(allTimeRows, secondTenantRelatedCase.ID.String()) {
		t.Fatalf("expected second-tenant related case %s in all_time", secondTenantRelatedCase.ID)
	}
	if containsRelatedCaseID(allTimeRows, ownerOnlyRelatedCase.ID.String()) {
		t.Fatalf("owner-only related case %s should not be visible for second tenant", ownerOnlyRelatedCase.ID)
	}
}

func TestListRelatedCasesByLinkedField(t *testing.T) {
	env := newAPITestEnv(t)

	sourceCase := createCaseFixture(t, env, env.tenantID, env.userID, "CASE-REL-FIELD-SRC", "Source by field", "open")
	updateCaseFieldFixture(t, env, sourceCase.ID, "incident_type", "credential_access")

	matchingCase := createCaseFixture(t, env, env.tenantID, env.userID, "CASE-REL-FIELD-MATCH", "Matching incident type", "open")
	updateCaseFieldFixture(t, env, matchingCase.ID, "incident_type", "credential_access")

	nonMatchingCase := createCaseFixture(t, env, env.tenantID, env.userID, "CASE-REL-FIELD-NOPE", "Different incident type", "open")
	updateCaseFieldFixture(t, env, nonMatchingCase.ID, "incident_type", "phishing")

	c, rec := env.jsonContext(http.MethodGet, "/api/v1/cases/"+sourceCase.ID.String()+"/related?link_by=incident_type", nil)
	c.Request().URL.RawQuery = "link_by=incident_type"
	setPath(c, "/api/v1/cases/:caseID/related", []string{"caseID"}, []string{sourceCase.ID.String()})
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)
	err := env.handler.ListRelatedCases(c)
	mustStatusOK(t, err, rec, http.StatusOK)

	payload := decodeBody[map[string]any](t, rec)
	if got := strings.TrimSpace(toString(payload["link_by"])); got != "incident_type" {
		t.Fatalf("expected link_by incident_type, got %q", got)
	}
	activeRecentRows := anySlice(payload["active_recent"])
	if len(activeRecentRows) != 1 {
		t.Fatalf("expected one related case by incident_type, got %d", len(activeRecentRows))
	}
	if !containsRelatedCaseID(activeRecentRows, matchingCase.ID.String()) {
		t.Fatalf("expected matching case %s in active_recent", matchingCase.ID)
	}
	if containsRelatedCaseID(activeRecentRows, nonMatchingCase.ID.String()) {
		t.Fatalf("non-matching case %s should not be in active_recent", nonMatchingCase.ID)
	}
	row := relatedCaseByID(activeRecentRows, matchingCase.ID.String())
	matchedFields := anySlice(row["matched_fields"])
	if len(matchedFields) != 1 {
		t.Fatalf("expected one matched field entry, got %d", len(matchedFields))
	}
	fieldEntry, _ := matchedFields[0].(map[string]any)
	if strings.TrimSpace(toString(fieldEntry["field"])) != "incident_type" {
		t.Fatalf("expected matched field incident_type, got %#v", fieldEntry["field"])
	}
	if strings.TrimSpace(toString(fieldEntry["value"])) != "credential_access" {
		t.Fatalf("expected matched value credential_access, got %#v", fieldEntry["value"])
	}
}

func TestListRelatedCasesRejectsInvalidLinkBy(t *testing.T) {
	env := newAPITestEnv(t)

	sourceCase := createCaseFixture(t, env, env.tenantID, env.userID, "CASE-REL-BAD-LINK", "Source case", "open")
	c, rec := env.jsonContext(http.MethodGet, "/api/v1/cases/"+sourceCase.ID.String()+"/related?link_by=unknown", nil)
	c.Request().URL.RawQuery = "link_by=unknown"
	setPath(c, "/api/v1/cases/:caseID/related", []string{"caseID"}, []string{sourceCase.ID.String()})
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)

	err := env.handler.ListRelatedCases(c)
	httpErr, ok := err.(*echo.HTTPError)
	if !ok || httpErr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 invalid link_by, got err=%v code=%d body=%s", err, rec.Code, rec.Body.String())
	}
}

func updateCaseFieldFixture(t *testing.T, env *apiTestEnv, caseID uuid.UUID, field string, value string) {
	t.Helper()
	query := "UPDATE cases SET " + field + " = $2 WHERE id = $1"
	if _, err := env.pool.Exec(context.Background(), query, caseID, value); err != nil {
		t.Fatalf("update case %s field %s: %v", caseID, field, err)
	}
}

func createCaseFixture(
	t *testing.T,
	env *apiTestEnv,
	tenantID uuid.UUID,
	createdBy uuid.UUID,
	caseNumber string,
	title string,
	status string,
) *models.Case {
	t.Helper()
	item, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          tenantID,
		CaseNumber:        caseNumber,
		Title:             title,
		Description:       "fixture",
		Source:            "manual",
		IncidentType:      "generic",
		Status:            status,
		Priority:          "medium",
		Impact:            "medium",
		Confidence:        50,
		Severity:          "medium",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         createdBy,
	})
	if err != nil {
		t.Fatalf("create case fixture %s: %v", caseNumber, err)
	}
	return item
}

func createObservableFixture(
	t *testing.T,
	env *apiTestEnv,
	tenantID uuid.UUID,
	caseID uuid.UUID,
	createdBy uuid.UUID,
	obsType string,
	obsValue string,
) {
	t.Helper()
	_, err := env.observables.Create(context.Background(), repository.CreateObservableParams{
		TenantID:  tenantID,
		CaseID:    caseID,
		Type:      strings.TrimSpace(strings.ToLower(obsType)),
		Value:     strings.TrimSpace(obsValue),
		Verdict:   "unknown",
		Source:    "fixture",
		Tags:      []string{},
		CreatedBy: createdBy,
	})
	if err != nil {
		t.Fatalf("create observable fixture %s:%s: %v", obsType, obsValue, err)
	}
}

func anySlice(value any) []any {
	items, _ := value.([]any)
	if items == nil {
		return []any{}
	}
	return items
}

func relatedCaseByID(items []any, caseID string) map[string]any {
	for _, row := range items {
		item, _ := row.(map[string]any)
		if strings.TrimSpace(toString(item["id"])) == caseID {
			return item
		}
	}
	return map[string]any{}
}

func containsRelatedCaseID(items []any, caseID string) bool {
	for _, row := range items {
		item, _ := row.(map[string]any)
		if strings.TrimSpace(toString(item["id"])) == caseID {
			return true
		}
	}
	return false
}

func toString(value any) string {
	result, _ := value.(string)
	return result
}

func toInt(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	case int:
		return typed
	case int64:
		return int(typed)
	default:
		return 0
	}
}
