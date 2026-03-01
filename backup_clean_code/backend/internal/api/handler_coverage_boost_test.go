package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"incidenthub/backend/internal/ai"
	"incidenthub/backend/internal/repository"

	"github.com/google/uuid"
)

func TestParseSessionListLimitBranches(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want int
	}{
		{name: "empty", raw: "", want: 25},
		{name: "spaces", raw: "   ", want: 25},
		{name: "invalid", raw: "abc", want: 25},
		{name: "non-positive", raw: "0", want: 25},
		{name: "capped", raw: "201", want: 200},
		{name: "valid", raw: "150", want: 150},
	}

	for _, tc := range tests {
		if got := parseSessionListLimit(tc.raw); got != tc.want {
			t.Fatalf("%s: expected %d, got %d", tc.name, tc.want, got)
		}
	}
}

func TestNormalizeOptionalRFC3339PtrBranches(t *testing.T) {
	normalizedNil, err := normalizeOptionalRFC3339Ptr(nil)
	if err != nil {
		t.Fatalf("nil pointer should not fail: %v", err)
	}
	if normalizedNil != nil {
		t.Fatalf("nil pointer should stay nil")
	}

	raw := "2026-02-16T18:30:00+03:00"
	normalized, err := normalizeOptionalRFC3339Ptr(&raw)
	if err != nil {
		t.Fatalf("valid pointer should normalize: %v", err)
	}
	if normalized == nil || *normalized != "2026-02-16T15:30:00Z" {
		t.Fatalf("unexpected normalized value: %v", normalized)
	}

	invalid := "not-a-time"
	if _, err := normalizeOptionalRFC3339Ptr(&invalid); err == nil {
		t.Fatalf("invalid pointer value should fail")
	}
}

func TestCollectModulesHealthDegradedBranch(t *testing.T) {
	env := newAPITestEnv(t)
	env.handler.cfg.Elastic.Enabled = true
	env.handler.cfg.S3.Enabled = true
	env.handler.cfg.AI.Enabled = true
	env.handler.cache = nil
	env.handler.search = nil
	env.handler.artifacts = nil
	env.handler.ai = nil

	overall, modules := env.handler.collectModulesHealth(context.Background())
	if overall != "degraded" {
		t.Fatalf("expected degraded overall health, got %s", overall)
	}
	if modules["api"].Status != "ok" {
		t.Fatalf("api module should be ok, got %s", modules["api"].Status)
	}
	for _, key := range []string{"redis", "elasticsearch", "s3", "ai_model"} {
		module := modules[key]
		if module.Status != "error" {
			t.Fatalf("%s expected error status, got %s", key, module.Status)
		}
		if strings.TrimSpace(module.Message) == "" {
			t.Fatalf("%s expected non-empty error message", key)
		}
	}
}

func TestResolveCaseStatusBranches(t *testing.T) {
	env := newAPITestEnv(t)

	resolvedDefault, err := env.handler.resolveCaseStatus(context.Background(), env.tenantID, "")
	if err != nil {
		t.Fatalf("resolve default status: %v", err)
	}
	if resolvedDefault != "new" {
		t.Fatalf("expected default status new, got %s", resolvedDefault)
	}

	if _, statusErr := env.handler.resolveCaseStatus(context.Background(), env.tenantID, "unknown status"); statusErr == nil {
		t.Fatalf("expected unknown status validation error")
	}

	_, err = env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID: &env.tenantID,
		Kind:     "case_statuses",
		Data: map[string]any{
			"code":      "resolved_only",
			"label":     "Resolved Only",
			"order":     10,
			"is_closed": true,
		},
		CreatedBy: &env.userID,
	})
	if err != nil {
		t.Fatalf("create closed status one: %v", err)
	}
	_, err = env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID: &env.tenantID,
		Kind:     "case_statuses",
		Data: map[string]any{
			"code":      "closed_only",
			"label":     "Closed Only",
			"order":     20,
			"is_closed": true,
		},
		CreatedBy: &env.userID,
	})
	if err != nil {
		t.Fatalf("create closed status two: %v", err)
	}

	resolvedClosedFallback, err := env.handler.resolveCaseStatus(context.Background(), env.tenantID, " ")
	if err != nil {
		t.Fatalf("resolve closed fallback status: %v", err)
	}
	if resolvedClosedFallback != "resolved_only" {
		t.Fatalf("expected first closed tenant status, got %s", resolvedClosedFallback)
	}
}

func TestResolveCaseCommunicationThreadBranches(t *testing.T) {
	env := newAPITestEnv(t)

	caseOne, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-COMM-1",
		Title:             "Case one",
		Description:       "case one",
		Source:            "manual",
		IncidentType:      "generic",
		Status:            "open",
		Priority:          "medium",
		Impact:            "system",
		Confidence:        30,
		Severity:          "low",
		TLP:               "green",
		PAP:               "green",
		ResolutionSummary: "",
		CreatedBy:         env.userID,
	})
	if err != nil {
		t.Fatalf("create case one: %v", err)
	}
	caseTwo, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-COMM-2",
		Title:             "Case two",
		Description:       "case two",
		Source:            "manual",
		IncidentType:      "generic",
		Status:            "open",
		Priority:          "medium",
		Impact:            "system",
		Confidence:        30,
		Severity:          "low",
		TLP:               "green",
		PAP:               "green",
		ResolutionSummary: "",
		CreatedBy:         env.userID,
	})
	if err != nil {
		t.Fatalf("create case two: %v", err)
	}

	thread, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID:  &env.tenantID,
		Kind:      caseCommunicationThreadKind,
		OwnerID:   &env.userID,
		RefID:     &caseOne.ID,
		CreatedBy: &env.userID,
		Data: map[string]any{
			"title": "thread one",
		},
	})
	if err != nil {
		t.Fatalf("create communication thread: %v", err)
	}

	if _, err := env.handler.resolveCaseCommunicationThread(context.Background(), env.tenantID, caseOne.ID, thread.ID); err != nil {
		t.Fatalf("expected successful thread resolution: %v", err)
	}
	if _, err := env.handler.resolveCaseCommunicationThread(context.Background(), env.tenantID, caseOne.ID, uuid.New()); httpErrorCode(t, err) != http.StatusNotFound {
		t.Fatalf("expected not found for missing thread")
	}
	if _, err := env.handler.resolveCaseCommunicationThread(context.Background(), env.tenantID, caseTwo.ID, thread.ID); httpErrorCode(t, err) != http.StatusNotFound {
		t.Fatalf("expected not found for case mismatch")
	}
}

func TestUpsertCaseAIVerdictCreateAndUpdateBranches(t *testing.T) {
	env := newAPITestEnv(t)

	caseItem, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-AI-UPSERT-1",
		Title:             "Case ai upsert",
		Description:       "ai verdict upsert",
		Source:            "manual",
		IncidentType:      "generic",
		Status:            "open",
		Priority:          "medium",
		Impact:            "system",
		Confidence:        30,
		Severity:          "low",
		TLP:               "green",
		PAP:               "green",
		ResolutionSummary: "",
		CreatedBy:         env.userID,
	})
	if err != nil {
		t.Fatalf("create ai upsert case: %v", err)
	}

	first := ai.CaseAnalysisResult{
		Verdict:         "malicious",
		Recommendations: []string{"isolate host"},
	}
	if upsertErr := env.handler.upsertCaseAIVerdict(context.Background(), env.tenantID, caseItem.ID, env.userID, first); upsertErr != nil {
		t.Fatalf("create case ai verdict: %v", upsertErr)
	}

	second := ai.CaseAnalysisResult{
		Verdict:         "benign",
		Recommendations: []string{"close case"},
	}
	if upsertErr := env.handler.upsertCaseAIVerdict(context.Background(), env.tenantID, caseItem.ID, env.userID, second); upsertErr != nil {
		t.Fatalf("update case ai verdict: %v", upsertErr)
	}

	metaItems, err := env.catalog.List(context.Background(), repository.CatalogListParams{
		Kind:     "case_meta",
		TenantID: &env.tenantID,
		RefID:    &caseItem.ID,
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("list case meta: %v", err)
	}
	if len(metaItems) != 1 {
		t.Fatalf("expected one case_meta record after upsert, got %d", len(metaItems))
	}
	if got := strings.TrimSpace(stringFromMap(metaItems[0].Data, "verdict")); got != "Benign" {
		t.Fatalf("expected updated verdict Benign, got %s", got)
	}
}

func TestResolveAISessionRepositoryUnavailableBranch(t *testing.T) {
	env := newAPITestEnv(t)
	original := env.handler.aiChats
	env.handler.aiChats = nil
	defer func() { env.handler.aiChats = original }()

	c, _ := env.jsonContext(http.MethodGet, "/api/v1/ai/messages", nil)
	_, err := env.handler.resolveAISession(c, env.tenantID, env.userID, "")
	if code := httpErrorCode(t, err); code != http.StatusServiceUnavailable {
		t.Fatalf("expected ai chat repo unavailable, got %d", code)
	}
}

func TestDeleteCaseObservableErrorBranches(t *testing.T) {
	env := newAPITestEnv(t)

	caseItem, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-OBS-DEL-1",
		Title:             "Case observable delete",
		Description:       "observable delete branches",
		Source:            "manual",
		IncidentType:      "generic",
		Status:            "open",
		Priority:          "medium",
		Impact:            "system",
		Confidence:        30,
		Severity:          "low",
		TLP:               "green",
		PAP:               "green",
		ResolutionSummary: "",
		CreatedBy:         env.userID,
	})
	if err != nil {
		t.Fatalf("create case: %v", err)
	}

	{
		c, _ := env.jsonContext(http.MethodDelete, "/api/v1/cases/"+caseItem.ID.String()+"/observables/not-uuid", nil)
		setPath(c, "/api/v1/cases/:caseID/observables/:observableID", []string{"caseID", "observableID"}, []string{caseItem.ID.String(), "not-uuid"})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		deleteErr := env.handler.DeleteCaseObservable(c)
		if code := httpErrorCode(t, deleteErr); code != http.StatusBadRequest {
			t.Fatalf("expected invalid observable id error, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodDelete, "/api/v1/cases/"+caseItem.ID.String()+"/observables/"+uuid.NewString(), nil)
		missingID := uuid.NewString()
		setPath(c, "/api/v1/cases/:caseID/observables/:observableID", []string{"caseID", "observableID"}, []string{caseItem.ID.String(), missingID})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		deleteErr := env.handler.DeleteCaseObservable(c)
		if code := httpErrorCode(t, deleteErr); code != http.StatusNotFound {
			t.Fatalf("expected observable not found error, got %d", code)
		}
	}

	observable, err := env.observables.Create(context.Background(), repository.CreateObservableParams{
		TenantID:  env.tenantID,
		CaseID:    caseItem.ID,
		Type:      "ip",
		Value:     "10.10.10.10",
		Verdict:   "unknown",
		Source:    "test",
		Tags:      []string{"branch"},
		CreatedBy: env.userID,
	})
	if err != nil {
		t.Fatalf("create observable: %v", err)
	}

	{
		c, rec := env.jsonContext(http.MethodDelete, "/api/v1/cases/"+caseItem.ID.String()+"/observables/"+observable.ID.String(), nil)
		setPath(c, "/api/v1/cases/:caseID/observables/:observableID", []string{"caseID", "observableID"}, []string{caseItem.ID.String(), observable.ID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		deleteErr := env.handler.DeleteCaseObservable(c)
		mustStatusOK(t, deleteErr, rec, http.StatusOK)
	}
}
