package api

import (
	"context"
	"encoding/json"
	"testing"

	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"

	"github.com/google/uuid"
)

func TestKafkaAsyncOpsProcessCaseDeleteMaintainsSearchConsistency(t *testing.T) {
	env := newAPITestEnv(t)

	caseItem, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-ASYNC-CASCADE",
		Title:             "Async case delete with linked entities",
		Description:       "delete case and keep search index consistent",
		Source:            "manual",
		IncidentType:      "investigation",
		Status:            "open",
		Priority:          "medium",
		Impact:            "medium",
		Confidence:        55,
		Severity:          "medium",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create case seed: %v", err)
	}

	alert, err := env.alerts.Create(context.Background(), repository.CreateAlertParams{
		TenantID:    env.tenantID,
		CaseID:      &caseItem.ID,
		Title:       "Async linked alert",
		Description: "linked to deleting case",
		Source:      "siem",
		Status:      "new",
		Severity:    "critical",
		TLP:         "amber",
		PAP:         "amber",
		CreatedBy:   &env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create linked alert seed: %v", err)
	}

	observable, err := env.observables.Create(context.Background(), repository.CreateObservableParams{
		TenantID:  env.tenantID,
		CaseID:    caseItem.ID,
		Type:      "domain",
		Value:     "example.test",
		Verdict:   "unknown",
		Source:    "manual",
		Tags:      []string{"seed"},
		CreatedBy: env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create linked observable seed: %v", err)
	}

	payload, err := json.Marshal(asyncDeletePayload{ID: caseItem.ID.String()})
	if err != nil {
		t.Fatalf("marshal async delete payload: %v", err)
	}

	queue := &KafkaAsyncOps{
		enabled:     true,
		alerts:      env.alerts,
		cases:       env.cases,
		observables: env.observables,
		audits:      repository.NewAuditRepository(env.pool),
		search:      env.searchStub,
	}

	operation := AsyncOperation{
		OperationID: uuid.NewString(),
		Type:        AsyncOperationCaseDelete,
		TenantID:    env.tenantID.String(),
		ActorID:     env.identity.UserID.String(),
		ResourceID:  caseItem.ID.String(),
		Payload:     payload,
	}

	env.searchStub.indexes = nil
	env.searchStub.deletes = nil
	if processErr := queue.processCaseDelete(context.Background(), operation); processErr != nil {
		t.Fatalf("process async case delete: %v", processErr)
	}

	if _, getErr := env.cases.GetByID(context.Background(), env.tenantID, caseItem.ID); getErr == nil {
		t.Fatal("expected case to be deleted by async worker")
	}
	if observables, listErr := env.observables.ListByCase(context.Background(), env.tenantID, caseItem.ID, 10, 0); listErr != nil {
		t.Fatalf("list observables after async case delete: %v", listErr)
	} else if len(observables) != 0 {
		t.Fatalf("expected linked observables to be removed by cascade, got %d", len(observables))
	}

	reloadedAlert, err := env.alerts.GetByID(context.Background(), env.tenantID, alert.ID)
	if err != nil {
		t.Fatalf("load linked alert after async case delete: %v", err)
	}
	if reloadedAlert.CaseID != nil {
		t.Fatalf("expected linked alert case_id to be null after async case delete, got %s", reloadedAlert.CaseID.String())
	}

	hasCaseDelete := false
	hasObservableDelete := false
	for _, call := range env.searchStub.deletes {
		if call.Kind == "cases" && call.ID == caseItem.ID.String() {
			hasCaseDelete = true
		}
		if call.Kind == "observables" && call.ID == observable.ID.String() {
			hasObservableDelete = true
		}
	}
	if !hasCaseDelete {
		t.Fatalf("expected search delete for deleted case %s", caseItem.ID.String())
	}
	if !hasObservableDelete {
		t.Fatalf("expected search delete for linked observable %s", observable.ID.String())
	}

	alertReindexed := false
	for _, call := range env.searchStub.indexes {
		if call.Kind != "alerts" || call.ID != alert.ID.String() {
			continue
		}
		alertReindexed = true
		switch item := call.Doc.(type) {
		case models.Alert:
			if item.CaseID != nil {
				t.Fatalf("expected reindexed alert case_id to be nil, got %s", item.CaseID.String())
			}
		case *models.Alert:
			if item != nil && item.CaseID != nil {
				t.Fatalf("expected reindexed alert case_id to be nil, got %s", item.CaseID.String())
			}
		default:
			t.Fatalf("unexpected alert reindex payload type %T", call.Doc)
		}
	}
	if !alertReindexed {
		t.Fatalf("expected linked alert %s to be reindexed after async case delete", alert.ID.String())
	}
}

func TestKafkaAsyncOpsProcessAlertUpdateMaintainsSearchConsistency(t *testing.T) {
	env := newAPITestEnv(t)

	alert, err := env.alerts.Create(context.Background(), repository.CreateAlertParams{
		TenantID:    env.tenantID,
		Title:       "Async update alert",
		Description: "before update",
		Source:      "siem",
		Status:      "new",
		Severity:    "medium",
		TLP:         "amber",
		PAP:         "amber",
		CreatedBy:   &env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create alert seed: %v", err)
	}

	payload, err := json.Marshal(asyncAlertUpdatePayload{
		AlertID:  alert.ID.String(),
		Status:   strPtr("triaged"),
		Severity: strPtr("high"),
	})
	if err != nil {
		t.Fatalf("marshal async update payload: %v", err)
	}

	queue := &KafkaAsyncOps{
		enabled: true,
		alerts:  env.alerts,
		cases:   env.cases,
		audits:  repository.NewAuditRepository(env.pool),
		search:  env.searchStub,
	}
	operation := AsyncOperation{
		OperationID: uuid.NewString(),
		Type:        AsyncOperationAlertUpdate,
		TenantID:    env.tenantID.String(),
		ActorID:     env.identity.UserID.String(),
		ResourceID:  alert.ID.String(),
		Payload:     payload,
	}

	env.searchStub.indexes = nil
	if processErr := queue.processAlertUpdate(context.Background(), operation); processErr != nil {
		t.Fatalf("process async alert update: %v", processErr)
	}

	reloaded, err := env.alerts.GetByID(context.Background(), env.tenantID, alert.ID)
	if err != nil {
		t.Fatalf("reload alert after async update: %v", err)
	}
	if reloaded.Status != "triaged" {
		t.Fatalf("expected updated alert status triaged, got %q", reloaded.Status)
	}
	if reloaded.Severity != "high" {
		t.Fatalf("expected updated alert severity high, got %q", reloaded.Severity)
	}
	if !hasSearchIndexCall(env.searchStub, "alerts", alert.ID.String()) {
		t.Fatalf("expected alert %s to be reindexed", alert.ID.String())
	}
}

func TestKafkaAsyncOpsProcessCaseUpdateMaintainsSearchConsistencyAndRewards(t *testing.T) {
	env := newAPITestEnv(t)

	caseItem, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-ASYNC-UPDATE",
		Title:             "Async case update",
		Description:       "before update",
		Source:            "manual",
		IncidentType:      "investigation",
		Status:            "open",
		Priority:          "medium",
		Impact:            "medium",
		Confidence:        60,
		Severity:          "high",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create case seed: %v", err)
	}

	payload, err := json.Marshal(asyncCaseUpdatePayload{
		CaseID:                 caseItem.ID.String(),
		Status:                 strPtr("resolved"),
		Priority:               strPtr("high"),
		RewardCaseClosureToID:  env.identity.UserID.String(),
		RewardCaseClosureEvent: true,
	})
	if err != nil {
		t.Fatalf("marshal async case update payload: %v", err)
	}

	queue := &KafkaAsyncOps{
		enabled:     true,
		alerts:      env.alerts,
		cases:       env.cases,
		observables: env.observables,
		experience:  repository.NewExperienceRepository(env.pool),
		audits:      repository.NewAuditRepository(env.pool),
		search:      env.searchStub,
	}
	operation := AsyncOperation{
		OperationID: uuid.NewString(),
		Type:        AsyncOperationCaseUpdate,
		TenantID:    env.tenantID.String(),
		ActorID:     env.identity.UserID.String(),
		ResourceID:  caseItem.ID.String(),
		Payload:     payload,
	}

	env.searchStub.indexes = nil
	if processErr := queue.processCaseUpdate(context.Background(), operation); processErr != nil {
		t.Fatalf("process async case update: %v", processErr)
	}

	reloaded, err := env.cases.GetByID(context.Background(), env.tenantID, caseItem.ID)
	if err != nil {
		t.Fatalf("reload case after async update: %v", err)
	}
	if reloaded.Status != "resolved" {
		t.Fatalf("expected updated case status resolved, got %q", reloaded.Status)
	}
	if reloaded.Priority != "high" {
		t.Fatalf("expected updated case priority high, got %q", reloaded.Priority)
	}
	if !hasSearchIndexCall(env.searchStub, "cases", caseItem.ID.String()) {
		t.Fatalf("expected case %s to be reindexed", caseItem.ID.String())
	}

	events, err := repository.NewExperienceRepository(env.pool).ListByUser(context.Background(), env.identity.UserID, &env.tenantID, 200, 0)
	if err != nil {
		t.Fatalf("list xp events: %v", err)
	}
	foundClosureReward := false
	for _, event := range events {
		if event.EventType == xpEventTypeCaseClosed && event.EventKey == "case_closed:"+caseItem.ID.String() {
			foundClosureReward = true
			break
		}
	}
	if !foundClosureReward {
		t.Fatalf("expected case closure experience event for case %s", caseItem.ID.String())
	}
}

func TestKafkaAsyncOpsProcessCreateEnqueuesAIAgentQueueEvents(t *testing.T) {
	env := newAPITestEnv(t)

	alertID := uuid.New()
	alertPayload, err := json.Marshal(asyncAlertCreatePayload{
		AlertID:     alertID.String(),
		Title:       "Async queue alert",
		Description: "created by async worker",
		Source:      "siem",
		Status:      "new",
		Severity:    "high",
		TLP:         "amber",
		PAP:         "amber",
	})
	if err != nil {
		t.Fatalf("marshal async alert create payload: %v", err)
	}
	caseID := uuid.New()
	casePayload, err := json.Marshal(asyncCaseCreatePayload{
		CaseID:            caseID.String(),
		CaseNumber:        "CASE-ASYNC-QUEUE-1",
		Title:             "Async queue case",
		Description:       "created by async worker",
		Source:            "manual",
		IncidentType:      "investigation",
		Status:            "open",
		Priority:          "high",
		Impact:            "system",
		Confidence:        55,
		Severity:          "high",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		AssignedTo:        env.identity.UserID.String(),
	})
	if err != nil {
		t.Fatalf("marshal async case create payload: %v", err)
	}

	queue := &KafkaAsyncOps{
		enabled:      true,
		alerts:       env.alerts,
		cases:        env.cases,
		observables:  env.observables,
		audits:       repository.NewAuditRepository(env.pool),
		aiAgentQueue: env.aiAgentQueue,
		search:       env.searchStub,
	}

	alertOperation := AsyncOperation{
		OperationID: uuid.NewString(),
		Type:        AsyncOperationAlertCreate,
		TenantID:    env.tenantID.String(),
		ActorID:     env.identity.UserID.String(),
		ResourceID:  alertID.String(),
		Payload:     alertPayload,
	}
	if processErr := queue.processAlertCreate(context.Background(), alertOperation); processErr != nil {
		t.Fatalf("process async alert create: %v", processErr)
	}
	caseOperation := AsyncOperation{
		OperationID: uuid.NewString(),
		Type:        AsyncOperationCaseCreate,
		TenantID:    env.tenantID.String(),
		ActorID:     env.identity.UserID.String(),
		ResourceID:  caseID.String(),
		Payload:     casePayload,
	}
	if processErr := queue.processCaseCreate(context.Background(), caseOperation); processErr != nil {
		t.Fatalf("process async case create: %v", processErr)
	}

	queued, err := env.aiAgentQueue.ListByStatus(context.Background(), models.AIAgentQueueStatusQueued, 20)
	if err != nil {
		t.Fatalf("list queued ai agent events: %v", err)
	}
	seenAlert := false
	seenCase := false
	for _, item := range queued {
		switch item.EntityType + ":" + item.EntityID.String() {
		case "alert:" + alertID.String():
			seenAlert = item.Source == aiAgentQueueSourceAsyncKafka
		case "case:" + caseID.String():
			seenCase = item.Source == aiAgentQueueSourceAsyncKafka
		}
	}
	if !seenAlert {
		t.Fatalf("expected queued ai-agent alert event for async create")
	}
	if !seenCase {
		t.Fatalf("expected queued ai-agent case event for async create")
	}
}

func strPtr(value string) *string {
	return &value
}
