package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"

	"github.com/google/uuid"
)

func TestCreateEntitiesEnqueueAIAgentQueueIntegration(t *testing.T) {
	env := newAPITestEnv(t)

	alertCtx, alertRec := env.jsonContext(http.MethodPost, "/api/v1/alerts", map[string]any{
		"title":       "Queue alert",
		"description": "Queue alert description",
		"source":      "siem",
	})
	setIdentity(alertCtx, env.identity)
	setTenant(alertCtx, env.tenantID)
	err := env.handler.CreateAlert(alertCtx)
	mustStatusOK(t, err, alertRec, http.StatusCreated)
	alert := decodeBody[map[string]any](t, alertRec)
	alertID, err := uuid.Parse(strings.TrimSpace(alert["id"].(string)))
	if err != nil {
		t.Fatalf("parse alert id: %v", err)
	}

	caseCtx, caseRec := env.jsonContext(http.MethodPost, "/api/v1/cases", map[string]any{
		"title":         "Queue case",
		"description":   "Queue case description",
		"incident_type": "malware",
		"severity":      "high",
	})
	setIdentity(caseCtx, env.identity)
	setTenant(caseCtx, env.tenantID)
	err = env.handler.CreateCase(caseCtx)
	mustStatusOK(t, err, caseRec, http.StatusCreated)
	createdCase := decodeBody[map[string]any](t, caseRec)
	caseID, err := uuid.Parse(strings.TrimSpace(createdCase["id"].(string)))
	if err != nil {
		t.Fatalf("parse case id: %v", err)
	}

	copyCtx, copyRec := env.jsonContext(http.MethodPost, "/api/v1/cases/"+caseID.String()+"/copy", map[string]any{})
	setPath(copyCtx, "/api/v1/cases/:caseID/copy", []string{"caseID"}, []string{caseID.String()})
	setIdentity(copyCtx, env.identity)
	setTenant(copyCtx, env.tenantID)
	err = env.handler.CopyCase(copyCtx)
	mustStatusOK(t, err, copyRec, http.StatusCreated)
	copiedCase := decodeBody[map[string]any](t, copyRec)
	copiedCaseID, err := uuid.Parse(strings.TrimSpace(copiedCase["id"].(string)))
	if err != nil {
		t.Fatalf("parse copied case id: %v", err)
	}

	caseFromAlertsCtx, caseFromAlertsRec := env.jsonContext(http.MethodPost, "/api/v1/alerts/bulk/create-case", map[string]any{
		"alert_ids": []string{alertID.String()},
		"case": map[string]any{
			"title": "Case from alert queue",
		},
	})
	setIdentity(caseFromAlertsCtx, env.identity)
	setTenant(caseFromAlertsCtx, env.tenantID)
	err = env.handler.CreateCaseFromAlerts(caseFromAlertsCtx)
	mustStatusOK(t, err, caseFromAlertsRec, http.StatusCreated)
	caseFromAlertsPayload := decodeBody[map[string]any](t, caseFromAlertsRec)
	caseFromAlertsRaw, ok := caseFromAlertsPayload["case"].(map[string]any)
	if !ok {
		t.Fatalf("expected case payload in create case from alerts response")
	}
	caseFromAlertsID, err := uuid.Parse(strings.TrimSpace(caseFromAlertsRaw["id"].(string)))
	if err != nil {
		t.Fatalf("parse case-from-alerts id: %v", err)
	}

	queued, err := env.aiAgentQueue.ListByStatus(context.Background(), "queued", 50)
	if err != nil {
		t.Fatalf("list queued ai agent events: %v", err)
	}
	if len(queued) < 4 {
		t.Fatalf("expected at least 4 queued events, got %d", len(queued))
	}

	seen := map[string]bool{}
	for _, item := range queued {
		seen[item.EntityType+":"+item.EntityID.String()] = true
	}
	for _, expected := range []string{
		"alert:" + alertID.String(),
		"case:" + caseID.String(),
		"case:" + copiedCaseID.String(),
		"case:" + caseFromAlertsID.String(),
	} {
		if !seen[expected] {
			t.Fatalf("expected queued event %s", expected)
		}
	}
}

func TestAIAgentQueueWorkerProcessesCasePlanIntegration(t *testing.T) {
	env := newAPITestEnv(t)

	caseItem, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-QUEUE-PLAN-1",
		Title:             "Queue case plan",
		Description:       "Plan execution test",
		Source:            "manual",
		IncidentType:      "phishing",
		Status:            "open",
		Priority:          "high",
		Impact:            "user",
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
	if _, metaErr := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID: &env.tenantID,
		Kind:     "case_meta",
		OwnerID:  &env.identity.UserID,
		RefID:    &caseItem.ID,
		Data: map[string]any{
			"case_id":   caseItem.ID.String(),
			"tenant_id": env.tenantID.String(),
			"tags":      []string{"phishing"},
		},
		CreatedBy: &env.identity.UserID,
	}); metaErr != nil {
		t.Fatalf("create case meta: %v", metaErr)
	}

	connector, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID: &env.tenantID,
		Kind:     "outbound_connectors",
		OwnerID:  &env.identity.UserID,
		Data: map[string]any{
			"name":      "Mock enrichment",
			"channel":   "mock",
			"enabled":   true,
			"direction": "outbound",
			"config":    map[string]any{},
		},
		CreatedBy: &env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create outbound connector: %v", err)
	}

	agentItem, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID: &env.tenantID,
		Kind:     "ai_agents",
		OwnerID:  &env.identity.UserID,
		Data: map[string]any{
			"name":              "Queue plan agent",
			"enabled":           true,
			"target_types":      []string{"case"},
			"case_tags":         []string{"phishing"},
			"auto_create_tasks": false,
			"auto_comment":      false,
			"auto_case_tags":    []string{"ai-reviewed"},
			"investigation_plan": []map[string]any{
				{
					"name":                     "Threat intel",
					"prompt":                   "check IOC reputation",
					"enrichment_connector_ids": []string{connector.ID.String()},
					"case_tags":                []string{"stage-triaged"},
				},
			},
		},
		CreatedBy: &env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create ai agent: %v", err)
	}

	env.handler.enqueueAIAgentQueueEvent(context.Background(), env.tenantID, &env.identity.UserID, "case", caseItem.ID, aiAgentQueueSourceAPI)
	processed := env.handler.processNextAIAgentQueueBatch(context.Background(), 10)
	if processed != 1 {
		t.Fatalf("expected one processed queue event, got %d", processed)
	}

	doneEvents, err := env.aiAgentQueue.ListByStatus(context.Background(), "done", 10)
	if err != nil {
		t.Fatalf("list done queue events: %v", err)
	}
	if len(doneEvents) != 1 {
		t.Fatalf("expected one done queue event, got %d", len(doneEvents))
	}
	if doneEvents[0].MatchedAgents != 1 || doneEvents[0].ProcessedAgents != 1 {
		t.Fatalf("unexpected queue counters matched=%d processed=%d", doneEvents[0].MatchedAgents, doneEvents[0].ProcessedAgents)
	}

	runs, err := env.catalog.List(context.Background(), repository.CatalogListParams{
		Kind:     "ai_agent_runs",
		TenantID: &env.tenantID,
		RefID:    &agentItem.ID,
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("list ai agent runs: %v", err)
	}
	if len(runs) == 0 {
		t.Fatalf("expected ai agent run persisted by queue worker")
	}
	resultsAny, ok := runs[0].Data["results"].([]any)
	if !ok || len(resultsAny) == 0 {
		t.Fatalf("expected results array in ai agent run payload")
	}
	resultMap, ok := resultsAny[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first result object, got %T", resultsAny[0])
	}
	enrichmentAny, ok := resultMap["enrichment"].([]any)
	if !ok || len(enrichmentAny) == 0 {
		t.Fatalf("expected enrichment results in ai agent run")
	}
	enrichmentItem, ok := enrichmentAny[0].(map[string]any)
	if !ok {
		t.Fatalf("expected enrichment item object, got %T", enrichmentAny[0])
	}
	reply := strings.ToLower(strings.TrimSpace(stringFromMap(enrichmentItem, "reply")))
	if !strings.Contains(reply, "stage instruction") || !strings.Contains(reply, "check ioc reputation") {
		t.Fatalf("expected stage prompt to be included in enrichment payload, got %q", reply)
	}
	stageTimelineAny, ok := resultMap["stage_timeline"].([]any)
	if !ok || len(stageTimelineAny) < 2 {
		t.Fatalf("expected stage_timeline with enrichment and analysis stages, got %#v", resultMap["stage_timeline"])
	}
	firstStage, ok := stageTimelineAny[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first stage_timeline item object, got %T", stageTimelineAny[0])
	}
	if strings.TrimSpace(stringFromMap(firstStage, "name")) != "Threat intel" {
		t.Fatalf("expected first stage name Threat intel, got %#v", firstStage["name"])
	}
	if strings.TrimSpace(stringFromMap(firstStage, "status")) == "" {
		t.Fatalf("expected stage status in stage_timeline, got %#v", firstStage)
	}
	connectorTimelineAny, ok := resultMap["connector_timeline"].([]any)
	if !ok || len(connectorTimelineAny) == 0 {
		t.Fatalf("expected connector_timeline in ai agent run payload, got %#v", resultMap["connector_timeline"])
	}
	connectorTimelineItem, ok := connectorTimelineAny[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first connector_timeline item object, got %T", connectorTimelineAny[0])
	}
	if strings.TrimSpace(stringFromMap(connectorTimelineItem, "status")) != "completed" {
		t.Fatalf("expected completed connector timeline status, got %#v", connectorTimelineItem["status"])
	}
	if strings.TrimSpace(stringFromMap(connectorTimelineItem, "stage_name")) != "Threat intel" {
		t.Fatalf("expected connector timeline stage_name Threat intel, got %#v", connectorTimelineItem["stage_name"])
	}

	tags := env.handler.loadCaseMetaTags(context.Background(), env.tenantID, caseItem.ID)
	if !containsAnyCaseTag(tags, []string{"ai-reviewed"}) {
		t.Fatalf("expected ai-reviewed tag to be applied, got %v", tags)
	}
	if !containsAnyCaseTag(tags, []string{"stage-triaged"}) {
		t.Fatalf("expected stage tag to be applied, got %v", tags)
	}
}

func TestAIAgentQueueWorkerAutoFillsCaseMITREWhenEmpty(t *testing.T) {
	env := newAPITestEnv(t)

	openAI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/chat/completions":
			writeQueueAgentChatCompletion(w, `{
			"verdict": "suspicious",
			"confidence": 89,
			"summary": "Credential phishing confirmed",
			"recommendations": ["Reset credentials"],
			"findings": ["Phishing delivery observed"],
			"miter": {
				"tactics": ["TA0001"],
				"techniques": ["T1566"]
			}
		}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer openAI.Close()

	caseItem, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-QUEUE-MITER-1",
		Title:             "Queue MITER autofill",
		Description:       "MITER should be filled by AI",
		Source:            "manual",
		IncidentType:      "phishing",
		Status:            "open",
		Priority:          "high",
		Impact:            "user",
		Confidence:        65,
		Severity:          "high",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create case seed: %v", err)
	}
	if _, metaErr := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID: &env.tenantID,
		Kind:     "case_meta",
		OwnerID:  &env.identity.UserID,
		RefID:    &caseItem.ID,
		Data: map[string]any{
			"case_id":   caseItem.ID.String(),
			"tenant_id": env.tenantID.String(),
			"tags":      []string{"phishing"},
		},
		CreatedBy: &env.identity.UserID,
	}); metaErr != nil {
		t.Fatalf("create case meta: %v", metaErr)
	}

	agentItem, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID: &env.tenantID,
		Kind:     "ai_agents",
		OwnerID:  &env.identity.UserID,
		Data: map[string]any{
			"name":              "MITER autofill agent",
			"enabled":           true,
			"target_types":      []string{"case"},
			"case_tags":         []string{"phishing"},
			"auto_create_tasks": false,
			"auto_comment":      false,
			"provider":          "openai",
			"endpoint":          openAI.URL + "/v1",
			"model":             "cyankiwi/Qwen3.5-27B-AWQ-4bit",
		},
		CreatedBy: &env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create ai agent: %v", err)
	}

	env.handler.enqueueAIAgentQueueEvent(context.Background(), env.tenantID, &env.identity.UserID, "case", caseItem.ID, aiAgentQueueSourceAPI)
	processed := env.handler.processNextAIAgentQueueBatch(context.Background(), 10)
	if processed != 1 {
		t.Fatalf("expected one processed queue event, got %d", processed)
	}

	metaItems, err := env.catalog.List(context.Background(), repository.CatalogListParams{
		Kind:     "case_meta",
		TenantID: &env.tenantID,
		RefID:    &caseItem.ID,
		Limit:    1,
	})
	if err != nil {
		t.Fatalf("list case meta after ai processing: %v", err)
	}
	if len(metaItems) != 1 {
		t.Fatalf("expected one case_meta record, got %d", len(metaItems))
	}
	if tactics := stringSliceFromMap(metaItems[0].Data, "tactics"); len(tactics) != 1 || strings.TrimSpace(tactics[0]) != "TA0001" {
		t.Fatalf("expected tactics [TA0001], got %#v", tactics)
	}
	if techniques := stringSliceFromMap(metaItems[0].Data, "techniques"); len(techniques) != 1 || strings.TrimSpace(techniques[0]) != "T1566" {
		t.Fatalf("expected techniques [T1566], got %#v", techniques)
	}

	runs, err := env.catalog.List(context.Background(), repository.CatalogListParams{
		Kind:     "ai_agent_runs",
		TenantID: &env.tenantID,
		RefID:    &agentItem.ID,
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("list ai agent runs: %v", err)
	}
	if len(runs) == 0 {
		t.Fatalf("expected persisted ai agent run")
	}
	resultsAny, ok := runs[0].Data["results"].([]any)
	if !ok || len(resultsAny) == 0 {
		t.Fatalf("expected results array in ai agent run payload")
	}
	resultMap, ok := resultsAny[0].(map[string]any)
	if !ok {
		t.Fatalf("expected result object, got %T", resultsAny[0])
	}
	if filled, ok := boolFromMap(resultMap, "miter_auto_filled", "mitreAutoFilled"); !ok || !filled {
		t.Fatalf("expected miter_auto_filled=true, got %#v", resultMap["miter_auto_filled"])
	}
}

func TestAIAgentQueueWorkerDoesNotOverwriteExistingCaseMITRE(t *testing.T) {
	env := newAPITestEnv(t)

	openAI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/chat/completions":
			writeQueueAgentChatCompletion(w, `{
			"verdict": "suspicious",
			"confidence": 89,
			"summary": "Credential phishing confirmed",
			"recommendations": ["Reset credentials"],
			"findings": ["Phishing delivery observed"],
			"miter": {
				"tactics": ["TA0001"],
				"techniques": ["T1566"]
			}
		}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer openAI.Close()

	caseItem, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-QUEUE-MITER-2",
		Title:             "Queue MITER preserve",
		Description:       "Existing MITER should stay untouched",
		Source:            "manual",
		IncidentType:      "malware",
		Status:            "open",
		Priority:          "high",
		Impact:            "system",
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
	if _, metaErr := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID: &env.tenantID,
		Kind:     "case_meta",
		OwnerID:  &env.identity.UserID,
		RefID:    &caseItem.ID,
		Data: map[string]any{
			"case_id":    caseItem.ID.String(),
			"tenant_id":  env.tenantID.String(),
			"tags":       []string{"malware"},
			"tactics":    []string{"TA0040"},
			"techniques": []string{"T1486"},
		},
		CreatedBy: &env.identity.UserID,
	}); metaErr != nil {
		t.Fatalf("create case meta: %v", metaErr)
	}

	agentItem, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID: &env.tenantID,
		Kind:     "ai_agents",
		OwnerID:  &env.identity.UserID,
		Data: map[string]any{
			"name":              "MITER preserve agent",
			"enabled":           true,
			"target_types":      []string{"case"},
			"case_tags":         []string{"malware"},
			"auto_create_tasks": false,
			"auto_comment":      false,
			"provider":          "openai",
			"endpoint":          openAI.URL + "/v1",
			"model":             "cyankiwi/Qwen3.5-27B-AWQ-4bit",
		},
		CreatedBy: &env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create ai agent: %v", err)
	}

	env.handler.enqueueAIAgentQueueEvent(context.Background(), env.tenantID, &env.identity.UserID, "case", caseItem.ID, aiAgentQueueSourceAPI)
	processed := env.handler.processNextAIAgentQueueBatch(context.Background(), 10)
	if processed != 1 {
		t.Fatalf("expected one processed queue event, got %d", processed)
	}

	metaItems, err := env.catalog.List(context.Background(), repository.CatalogListParams{
		Kind:     "case_meta",
		TenantID: &env.tenantID,
		RefID:    &caseItem.ID,
		Limit:    1,
	})
	if err != nil {
		t.Fatalf("list case meta after ai processing: %v", err)
	}
	if len(metaItems) != 1 {
		t.Fatalf("expected one case_meta record, got %d", len(metaItems))
	}
	if tactics := stringSliceFromMap(metaItems[0].Data, "tactics"); len(tactics) != 1 || strings.TrimSpace(tactics[0]) != "TA0040" {
		t.Fatalf("expected tactics to remain [TA0040], got %#v", tactics)
	}
	if techniques := stringSliceFromMap(metaItems[0].Data, "techniques"); len(techniques) != 1 || strings.TrimSpace(techniques[0]) != "T1486" {
		t.Fatalf("expected techniques to remain [T1486], got %#v", techniques)
	}

	runs, err := env.catalog.List(context.Background(), repository.CatalogListParams{
		Kind:     "ai_agent_runs",
		TenantID: &env.tenantID,
		RefID:    &agentItem.ID,
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("list ai agent runs: %v", err)
	}
	if len(runs) == 0 {
		t.Fatalf("expected persisted ai agent run")
	}
	resultsAny, ok := runs[0].Data["results"].([]any)
	if !ok || len(resultsAny) == 0 {
		t.Fatalf("expected results array in ai agent run payload")
	}
	resultMap, ok := resultsAny[0].(map[string]any)
	if !ok {
		t.Fatalf("expected result object, got %T", resultsAny[0])
	}
	if filled, ok := boolFromMap(resultMap, "miter_auto_filled", "mitreAutoFilled"); !ok || filled {
		t.Fatalf("expected miter_auto_filled=false, got %#v", resultMap["miter_auto_filled"])
	}
}

func TestAIAgentQueueWorkerAutoCloseCaseIntegration(t *testing.T) {
	env := newAPITestEnv(t)

	openAI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/chat/completions":
			w.Header().Set("Content-Type", "application/json")
			writeQueueAgentChatCompletion(w, `{
				"verdict": "benign",
				"confidence": 92,
				"summary": "No active compromise",
				"recommendations": ["Close case"],
				"findings": ["False positive alert chain"]
			}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer openAI.Close()

	caseItem, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-QUEUE-AUTOCLOSE-1",
		Title:             "Queue autoclose case",
		Description:       "Should be closed by AI",
		Source:            "manual",
		IncidentType:      "malware",
		Status:            "open",
		Priority:          "high",
		Impact:            "system",
		Confidence:        60,
		Severity:          "medium",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create case seed: %v", err)
	}

	agentItem, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID: &env.tenantID,
		Kind:     "ai_agents",
		OwnerID:  &env.identity.UserID,
		Data: map[string]any{
			"name":                     "Auto close queue agent",
			"enabled":                  true,
			"target_types":             []string{"case"},
			"auto_create_tasks":        false,
			"auto_comment":             false,
			"provider":                 "openai",
			"endpoint":                 openAI.URL + "/v1",
			"model":                    "cyankiwi/Qwen3.5-27B-AWQ-4bit",
			"auto_close_case":          true,
			"auto_close_verdicts":      []string{"benign"},
			"max_cases_per_run":        1,
			"enrichment_connector_ids": []string{},
		},
		CreatedBy: &env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create ai agent: %v", err)
	}

	env.handler.enqueueAIAgentQueueEvent(context.Background(), env.tenantID, &env.identity.UserID, "case", caseItem.ID, aiAgentQueueSourceAPI)
	processed := env.handler.processNextAIAgentQueueBatch(context.Background(), 10)
	if processed != 1 {
		storedEvent, loadErr := env.aiAgentQueue.ListByStatus(context.Background(), "queued", 20)
		t.Fatalf("expected one processed queue event, got %d (queued_events=%d load_err=%v)", processed, len(storedEvent), loadErr)
	}

	reloaded, err := env.cases.GetByID(context.Background(), env.tenantID, caseItem.ID)
	if err != nil {
		t.Fatalf("reload case after ai queue processing: %v", err)
	}
	if !env.handler.isCaseStatusClosed(context.Background(), env.tenantID, reloaded.Status) {
		t.Fatalf("expected case to be closed by ai agent, got status %q", reloaded.Status)
	}
	if !strings.Contains(strings.ToLower(strings.TrimSpace(reloaded.ResolutionSummary)), "auto-closed by ai agent") {
		t.Fatalf("expected resolution summary to include auto-close marker, got %q", reloaded.ResolutionSummary)
	}

	runs, err := env.catalog.List(context.Background(), repository.CatalogListParams{
		Kind:     "ai_agent_runs",
		TenantID: &env.tenantID,
		RefID:    &agentItem.ID,
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("list ai agent runs: %v", err)
	}
	if len(runs) == 0 {
		t.Fatalf("expected ai agent run for auto-close scenario")
	}
}

func TestAIAgentQueueWorkerTriadPolicyBlocksAutoActionsIntegration(t *testing.T) {
	env := newAPITestEnv(t)

	openAI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/chat/completions":
			body, _ := io.ReadAll(r.Body)
			raw := strings.ToLower(strings.TrimSpace(string(body)))
			response := `{
				"verdict": "benign",
				"confidence": 96,
				"summary": "Triad investigator verdict",
				"recommendations": ["Close case"],
				"findings": ["Initial malicious chain not confirmed"]
			}`
			switch {
			case strings.Contains(raw, "triad role: reviewer"):
				response = `{
					"verdict": "malicious",
					"confidence": 91,
					"summary": "Reviewer found escalation indicators",
					"recommendations": ["Escalate to IR lead"],
					"findings": ["Cross-system evidence mismatch"]
				}`
			case strings.Contains(raw, "triad role: arbiter"):
				response = `{
					"verdict": "benign",
					"confidence": 95,
					"summary": "Arbiter selected benign after weighing evidence",
					"recommendations": ["Close case"],
					"findings": ["Needs analyst sign-off due disagreement"]
				}`
			}
			writeQueueAgentChatCompletion(w, response)
		default:
			http.NotFound(w, r)
		}
	}))
	defer openAI.Close()

	caseItem, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-QUEUE-TRIAD-1",
		Title:             "Critical triad policy case",
		Description:       "Triad policy should block auto-close due disagreement",
		Source:            "manual",
		IncidentType:      "malware",
		Status:            "open",
		Priority:          "critical",
		Impact:            "system",
		Confidence:        75,
		Severity:          "critical",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create case seed: %v", err)
	}

	notificationConnector, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID: &env.tenantID,
		Kind:     "outbound_connectors",
		OwnerID:  &env.identity.UserID,
		Data: map[string]any{
			"name":      "Mock notify",
			"channel":   "mock",
			"enabled":   true,
			"direction": "outbound",
			"config":    map[string]any{},
		},
		CreatedBy: &env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create notification connector: %v", err)
	}

	agentItem, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID: &env.tenantID,
		Kind:     "ai_agents",
		OwnerID:  &env.identity.UserID,
		Data: map[string]any{
			"name":                       "Triad gate agent",
			"enabled":                    true,
			"target_types":               []string{"case"},
			"provider":                   "openai",
			"endpoint":                   openAI.URL + "/v1",
			"model":                      "cyankiwi/Qwen3.5-27B-AWQ-4bit",
			"triad_enabled":              true,
			"triad_critical_only":        true,
			"require_reviewer_consensus": true,
			"auto_action_min_confidence": 80,
			"auto_close_case":            true,
			"auto_close_verdicts":        []string{"benign"},
			"notification_connector_ids": []string{notificationConnector.ID.String()},
			"auto_create_tasks":          false,
			"auto_comment":               false,
			"max_cases_per_run":          1,
		},
		CreatedBy: &env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create ai agent: %v", err)
	}

	env.handler.enqueueAIAgentQueueEvent(context.Background(), env.tenantID, &env.identity.UserID, "case", caseItem.ID, aiAgentQueueSourceAPI)
	processed := env.handler.processNextAIAgentQueueBatch(context.Background(), 10)
	if processed != 1 {
		queued, _ := env.aiAgentQueue.ListByStatus(context.Background(), models.AIAgentQueueStatusQueued, 20)
		failed, _ := env.aiAgentQueue.ListByStatus(context.Background(), models.AIAgentQueueStatusFailed, 20)
		runs, _ := env.catalog.List(context.Background(), repository.CatalogListParams{
			Kind:     aiAgentRunsCatalogKind,
			TenantID: &env.tenantID,
			RefID:    &agentItem.ID,
			Limit:    10,
		})
		t.Fatalf("expected one processed queue event, got %d queued=%#v failed=%#v runs=%#v", processed, queued, failed, runs)
	}

	reloaded, err := env.cases.GetByID(context.Background(), env.tenantID, caseItem.ID)
	if err != nil {
		t.Fatalf("reload case after triad processing: %v", err)
	}
	if env.handler.isCaseStatusClosed(context.Background(), env.tenantID, reloaded.Status) {
		t.Fatalf("expected case to remain open when triad blocks auto actions")
	}

	runs, err := env.catalog.List(context.Background(), repository.CatalogListParams{
		Kind:     "ai_agent_runs",
		TenantID: &env.tenantID,
		RefID:    &agentItem.ID,
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("list ai agent runs: %v", err)
	}
	if len(runs) == 0 {
		t.Fatalf("expected triad ai agent run")
	}
	resultsAny, ok := runs[0].Data["results"].([]any)
	if !ok || len(resultsAny) == 0 {
		t.Fatalf("expected non-empty results in ai agent run")
	}
	resultMap, ok := resultsAny[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first result object, got %T", resultsAny[0])
	}
	if allowed, _ := boolFromMap(resultMap, "auto_actions_allowed", "autoActionsAllowed"); allowed {
		t.Fatalf("expected auto_actions_allowed=false for triad disagreement")
	}
	if consensus, _ := boolFromMap(resultMap, "reviewer_consensus", "reviewerConsensus"); consensus {
		t.Fatalf("expected reviewer_consensus=false for triad disagreement")
	}
	if required, _ := boolFromMap(resultMap, "requires_human_review", "requiresHumanReview"); !required {
		t.Fatalf("expected requires_human_review=true for triad disagreement: %#v", resultMap)
	}
	if skipped := strings.TrimSpace(stringFromMap(resultMap, "auto_close_skipped_reason")); skipped == "" {
		t.Fatalf("expected auto_close_skipped_reason to be populated")
	}
	if skipped := strings.TrimSpace(stringFromMap(resultMap, "notifications_skipped_reason")); skipped == "" {
		t.Fatalf("expected notifications_skipped_reason to be populated")
	}
}

func TestAIAgentQueueWorkerCreatesCaseFromAlertIntegration(t *testing.T) {
	env := newAPITestEnv(t)

	alert, err := env.alerts.Create(context.Background(), repository.CreateAlertParams{
		TenantID:    env.tenantID,
		Title:       "Queue alert auto case",
		Description: "Auto-create case from alert",
		Source:      "edr",
		Status:      "new",
		Severity:    "high",
		TLP:         "amber",
		PAP:         "amber",
		CreatedBy:   &env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create alert seed: %v", err)
	}

	agentItem, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID: &env.tenantID,
		Kind:     "ai_agents",
		OwnerID:  &env.identity.UserID,
		Data: map[string]any{
			"name":                        "Alert queue agent",
			"enabled":                     true,
			"target_types":                []string{"alert"},
			"auto_create_case_from_alert": true,
			"auto_create_tasks":           false,
			"auto_comment":                false,
			"max_cases_per_run":           1,
			"enrichment_connector_ids":    []string{},
		},
		CreatedBy: &env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create ai agent: %v", err)
	}

	env.handler.enqueueAIAgentQueueEvent(context.Background(), env.tenantID, &env.identity.UserID, "alert", alert.ID, aiAgentQueueSourceAPI)
	processed := env.handler.processNextAIAgentQueueBatch(context.Background(), 10)
	if processed != 1 {
		t.Fatalf("expected one processed queue event, got %d", processed)
	}

	reloadedAlert, err := env.alerts.GetByID(context.Background(), env.tenantID, alert.ID)
	if err != nil {
		t.Fatalf("reload alert: %v", err)
	}
	if reloadedAlert.CaseID == nil {
		t.Fatalf("expected alert to be linked to auto-created case")
	}
	if _, getErr := env.cases.GetByID(context.Background(), env.tenantID, *reloadedAlert.CaseID); getErr != nil {
		t.Fatalf("expected auto-created case to exist: %v", getErr)
	}

	runs, err := env.catalog.List(context.Background(), repository.CatalogListParams{
		Kind:     "ai_agent_runs",
		TenantID: &env.tenantID,
		RefID:    &agentItem.ID,
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("list ai agent runs: %v", err)
	}
	if len(runs) == 0 {
		t.Fatalf("expected ai agent run for alert auto-case scenario")
	}
}

func TestGetAIAgentOperationsOverviewIntegration(t *testing.T) {
	env := newAPITestEnv(t)

	caseOne, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-OPS-1",
		Title:             "Ops queued case",
		Description:       "Queued event should appear in overview",
		Source:            "manual",
		IncidentType:      "malware",
		Status:            "open",
		Priority:          "high",
		Impact:            "system",
		Confidence:        60,
		Severity:          "high",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create case one: %v", err)
	}
	caseTwo, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-OPS-2",
		Title:             "Ops processing case",
		Description:       "Processing event should appear in overview",
		Source:            "manual",
		IncidentType:      "phishing",
		Status:            "open",
		Priority:          "critical",
		Impact:            "user",
		Confidence:        70,
		Severity:          "critical",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create case two: %v", err)
	}

	queuedEvent, _, err := env.aiAgentQueue.Enqueue(context.Background(), repository.EnqueueAIAgentQueueEventParams{
		TenantID:   env.tenantID,
		ActorID:    &env.identity.UserID,
		EntityType: "case",
		EntityID:   caseOne.ID,
		Source:     aiAgentQueueSourceAPI,
	})
	if err != nil {
		t.Fatalf("enqueue queued event: %v", err)
	}
	processingEvent, _, err := env.aiAgentQueue.Enqueue(context.Background(), repository.EnqueueAIAgentQueueEventParams{
		TenantID:   env.tenantID,
		ActorID:    &env.identity.UserID,
		EntityType: "case",
		EntityID:   caseTwo.ID,
		Source:     aiAgentQueueSourceAPI,
	})
	if err != nil {
		t.Fatalf("enqueue processing event: %v", err)
	}
	if markErr := env.aiAgentQueue.MarkProcessing(context.Background(), processingEvent.ID, "wf-ops-overview-1"); markErr != nil {
		t.Fatalf("mark processing event: %v", markErr)
	}

	if _, createErr := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID: &env.tenantID,
		Kind:     "ai_agents",
		OwnerID:  &env.identity.UserID,
		Data: map[string]any{
			"name":              "Ops monitor agent",
			"enabled":           true,
			"target_types":      []string{"case"},
			"auto_create_tasks": false,
			"auto_comment":      false,
		},
		CreatedBy: &env.identity.UserID,
	}); createErr != nil {
		t.Fatalf("create ai agent for ops overview: %v", createErr)
	}

	c, rec := env.jsonContext(http.MethodGet, "/api/v1/ai/agents/ops/overview?limit=10", nil)
	c.Request().URL.RawQuery = "limit=10"
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)
	err = env.handler.GetAIAgentOperationsOverview(c)
	mustStatusOK(t, err, rec, http.StatusOK)
	payload := decodeBody[map[string]any](t, rec)

	queueMap, ok := payload["queue"].(map[string]any)
	if !ok {
		t.Fatalf("expected queue object in overview payload")
	}
	if int(queueMap["queued"].(float64)) != 1 {
		t.Fatalf("expected queued count 1, got %v", queueMap["queued"])
	}
	if int(queueMap["processing"].(float64)) != 1 {
		t.Fatalf("expected processing count 1, got %v", queueMap["processing"])
	}

	queuedEventsAny, ok := payload["queued_events"].([]any)
	if !ok || len(queuedEventsAny) == 0 {
		t.Fatalf("expected queued events in overview payload")
	}
	processingEventsAny, ok := payload["processing_events"].([]any)
	if !ok || len(processingEventsAny) == 0 {
		t.Fatalf("expected processing events in overview payload")
	}

	foundQueuedEvent := false
	for _, raw := range queuedEventsAny {
		item, isMap := raw.(map[string]any)
		if !isMap {
			continue
		}
		if strings.TrimSpace(stringFromMap(item, "id")) == queuedEvent.ID.String() {
			foundQueuedEvent = true
			break
		}
	}
	if !foundQueuedEvent {
		t.Fatalf("expected queued event %s in queued_events", queuedEvent.ID.String())
	}

	agentRuntimeAny, ok := payload["agent_runtime"].([]any)
	if !ok || len(agentRuntimeAny) == 0 {
		t.Fatalf("expected non-empty agent_runtime in overview payload")
	}
}

func TestAIAgentQueueWorkerRetriesUntilMaxAttemptsIntegration(t *testing.T) {
	env := newAPITestEnv(t)

	caseItem, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-QUEUE-RETRY-1",
		Title:             "Queue retry case",
		Description:       "Queue event should retry and then fail",
		Source:            "manual",
		IncidentType:      "malware",
		Status:            "open",
		Priority:          "high",
		Impact:            "system",
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

	if _, createErr := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID: &env.tenantID,
		Kind:     "ai_agents",
		OwnerID:  &env.identity.UserID,
		Data: map[string]any{
			"name":              "Retry until failed",
			"enabled":           true,
			"target_types":      []string{"case"},
			"provider":          "openai",
			"endpoint":          "http://127.0.0.1:1/v1",
			"model":             "cyankiwi/Qwen3.5-27B-AWQ-4bit",
			"auto_create_tasks": false,
			"auto_comment":      false,
			"max_cases_per_run": 1,
		},
		CreatedBy: &env.identity.UserID,
	}); createErr != nil {
		t.Fatalf("create ai agent: %v", createErr)
	}

	event, _, err := env.aiAgentQueue.Enqueue(context.Background(), repository.EnqueueAIAgentQueueEventParams{
		TenantID:    env.tenantID,
		ActorID:     &env.identity.UserID,
		EntityType:  "case",
		EntityID:    caseItem.ID,
		Source:      aiAgentQueueSourceAPI,
		MaxAttempts: 2,
	})
	if err != nil {
		t.Fatalf("enqueue retry event: %v", err)
	}

	processed := env.handler.processNextAIAgentQueueBatch(context.Background(), 10)
	if processed != 0 {
		t.Fatalf("expected no completed events on first failed attempt, got %d", processed)
	}
	stored, err := env.aiAgentQueue.GetByID(context.Background(), event.ID)
	if err != nil {
		t.Fatalf("get queue event after first attempt: %v", err)
	}
	if stored.Status != "queued" {
		t.Fatalf("expected queued status after first failed attempt, got %q", stored.Status)
	}
	if stored.AttemptCount != 1 {
		t.Fatalf("expected attempt_count=1 after first failed attempt, got %d", stored.AttemptCount)
	}
	if strings.TrimSpace(stored.LastError) == "" {
		t.Fatalf("expected last_error to be set after first failed attempt")
	}

	processed = env.handler.processNextAIAgentQueueBatch(context.Background(), 10)
	if processed != 0 {
		t.Fatalf("expected no completed events on terminal failed attempt, got %d", processed)
	}
	stored, err = env.aiAgentQueue.GetByID(context.Background(), event.ID)
	if err != nil {
		t.Fatalf("get queue event after second attempt: %v", err)
	}
	if stored.Status != "failed" {
		t.Fatalf("expected failed status after max attempts, got %q", stored.Status)
	}
	if stored.AttemptCount != 2 {
		t.Fatalf("expected attempt_count=2 after terminal failure, got %d", stored.AttemptCount)
	}
}

func TestAIAgentQueueEventManualRestartAndCloseIntegration(t *testing.T) {
	env := newAPITestEnv(t)

	caseItem, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-QUEUE-CONTROL-1",
		Title:             "Queue manual controls case",
		Description:       "Manual restart and close controls",
		Source:            "manual",
		IncidentType:      "malware",
		Status:            "open",
		Priority:          "medium",
		Impact:            "system",
		Confidence:        60,
		Severity:          "medium",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create case seed: %v", err)
	}

	event, _, err := env.aiAgentQueue.Enqueue(context.Background(), repository.EnqueueAIAgentQueueEventParams{
		TenantID:   env.tenantID,
		ActorID:    &env.identity.UserID,
		EntityType: "case",
		EntityID:   caseItem.ID,
		Source:     aiAgentQueueSourceAPI,
	})
	if err != nil {
		t.Fatalf("enqueue event: %v", err)
	}
	if markErr := env.aiAgentQueue.MarkProcessing(context.Background(), event.ID, "wf-control-1"); markErr != nil {
		t.Fatalf("mark processing event: %v", markErr)
	}
	if queueFailedErr := env.aiAgentQueue.MarkFailed(context.Background(), event.ID, repository.CompleteAIAgentQueueEventParams{
		MatchedAgents:   1,
		ProcessedAgents: 0,
		LastError:       "terminal error",
	}); queueFailedErr != nil {
		t.Fatalf("mark failed event: %v", queueFailedErr)
	}

	restartCtx, restartRec := env.jsonContext(http.MethodPost, "/api/v1/ai/agents/ops/events/"+event.ID.String()+"/restart", nil)
	setPath(restartCtx, "/api/v1/ai/agents/ops/events/:eventID/restart", []string{"eventID"}, []string{event.ID.String()})
	setIdentity(restartCtx, env.identity)
	setTenant(restartCtx, env.tenantID)
	err = env.handler.RestartAIAgentQueueEvent(restartCtx)
	mustStatusOK(t, err, restartRec, http.StatusOK)

	restarted := decodeBody[map[string]any](t, restartRec)
	restartedEvent, ok := restarted["event"].(map[string]any)
	if !ok {
		t.Fatalf("expected event payload on restart response")
	}
	if strings.TrimSpace(stringFromMap(restartedEvent, "status")) != "queued" {
		t.Fatalf("expected queued status after restart, got %q", strings.TrimSpace(stringFromMap(restartedEvent, "status")))
	}
	if int(numberOrZero(restartedEvent, "attempt_count")) != 0 {
		t.Fatalf("expected attempt_count=0 after restart, got %v", restartedEvent["attempt_count"])
	}

	closeCtx, closeRec := env.jsonContext(http.MethodPost, "/api/v1/ai/agents/ops/events/"+event.ID.String()+"/close", nil)
	setPath(closeCtx, "/api/v1/ai/agents/ops/events/:eventID/close", []string{"eventID"}, []string{event.ID.String()})
	setIdentity(closeCtx, env.identity)
	setTenant(closeCtx, env.tenantID)
	err = env.handler.CloseAIAgentQueueEvent(closeCtx)
	mustStatusOK(t, err, closeRec, http.StatusOK)

	closed := decodeBody[map[string]any](t, closeRec)
	closedEvent, ok := closed["event"].(map[string]any)
	if !ok {
		t.Fatalf("expected event payload on close response")
	}
	if strings.TrimSpace(stringFromMap(closedEvent, "status")) != "done" {
		t.Fatalf("expected done status after close, got %q", strings.TrimSpace(stringFromMap(closedEvent, "status")))
	}
	if !boolValueOrDefault(closedEvent, false, "closed_by_user", "closedByUser") {
		t.Fatalf("expected closed_by_user=true after close response")
	}
}

func TestAIAgentQueueWorkerDoesNotRerunSuccessfulWorkloadOnRetryIntegration(t *testing.T) {
	env := newAPITestEnv(t)

	openAI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/chat/completions":
			writeQueueAgentChatCompletion(w, `{
			"verdict": "suspicious",
			"confidence": 88,
			"summary": "Reliable agent completed the analysis",
			"recommendations": ["Rotate the credential"],
			"findings": ["Single sign-on anomaly confirmed"]
		}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer openAI.Close()

	caseItem, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-QUEUE-MULTI-RETRY-1",
		Title:             "Multi-agent retry isolation",
		Description:       "Successful workload must not be re-run when another workload retries",
		Source:            "manual",
		IncidentType:      "identity",
		Status:            "open",
		Priority:          "high",
		Impact:            "user",
		Confidence:        65,
		Severity:          "high",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create case seed: %v", err)
	}

	goodAgent, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID: &env.tenantID,
		Kind:     "ai_agents",
		OwnerID:  &env.identity.UserID,
		Data: map[string]any{
			"name":              "Reliable queue agent",
			"enabled":           true,
			"target_types":      []string{"case"},
			"provider":          "openai",
			"endpoint":          openAI.URL + "/v1",
			"model":             "cyankiwi/Qwen3.5-27B-AWQ-4bit",
			"auto_create_tasks": false,
			"auto_comment":      false,
			"max_cases_per_run": 1,
		},
		CreatedBy: &env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create reliable ai agent: %v", err)
	}
	badAgent, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID: &env.tenantID,
		Kind:     "ai_agents",
		OwnerID:  &env.identity.UserID,
		Data: map[string]any{
			"name":              "Failing queue agent",
			"enabled":           true,
			"target_types":      []string{"case"},
			"provider":          "openai",
			"endpoint":          "http://127.0.0.1:1/v1",
			"model":             "cyankiwi/Qwen3.5-27B-AWQ-4bit",
			"auto_create_tasks": false,
			"auto_comment":      false,
			"max_cases_per_run": 1,
		},
		CreatedBy: &env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create failing ai agent: %v", err)
	}

	event, _, err := env.aiAgentQueue.Enqueue(context.Background(), repository.EnqueueAIAgentQueueEventParams{
		TenantID:    env.tenantID,
		ActorID:     &env.identity.UserID,
		EntityType:  "case",
		EntityID:    caseItem.ID,
		Source:      aiAgentQueueSourceAPI,
		MaxAttempts: 2,
	})
	if err != nil {
		t.Fatalf("enqueue retry event: %v", err)
	}

	processed := env.handler.processNextAIAgentQueueBatch(context.Background(), 10)
	if processed != 0 {
		t.Fatalf("expected no completed events after first mixed attempt, got %d", processed)
	}

	workloads, err := env.aiAgentWorkloads.ListByQueueEvent(context.Background(), event.ID)
	if err != nil {
		t.Fatalf("list workloads after first attempt: %v", err)
	}
	if len(workloads) != 2 {
		t.Fatalf("expected 2 workloads, got %d", len(workloads))
	}

	var reliableWorkload models.AIAgentWorkload
	var failingWorkload models.AIAgentWorkload
	hasReliableWorkload := false
	hasFailingWorkload := false
	for i := range workloads {
		item := workloads[i]
		switch item.AgentID {
		case goodAgent.ID:
			reliableWorkload = item
			hasReliableWorkload = true
		case badAgent.ID:
			failingWorkload = item
			hasFailingWorkload = true
		}
	}
	if !hasReliableWorkload || !hasFailingWorkload {
		t.Fatalf("expected workloads for both agents: %#v", workloads)
	}
	if reliableWorkload.Status != models.AIAgentWorkloadStatusDone {
		t.Fatalf("expected reliable workload done after first attempt, got %q workload=%#v", reliableWorkload.Status, reliableWorkload)
	}
	if reliableWorkload.AttemptCount != 1 {
		t.Fatalf("expected reliable workload attempt_count=1, got %d", reliableWorkload.AttemptCount)
	}
	if failingWorkload.Status != models.AIAgentWorkloadStatusQueued {
		t.Fatalf("expected failing workload to be re-queued, got %q", failingWorkload.Status)
	}
	if failingWorkload.AttemptCount != 1 {
		t.Fatalf("expected failing workload attempt_count=1 after first attempt, got %d", failingWorkload.AttemptCount)
	}
	if strings.TrimSpace(failingWorkload.LastError) == "" {
		t.Fatalf("expected failing workload last_error to be populated")
	}

	processed = env.handler.processNextAIAgentQueueBatch(context.Background(), 10)
	if processed != 0 {
		t.Fatalf("expected terminal mixed run to keep queue event failed, got processed=%d", processed)
	}

	storedEvent, err := env.aiAgentQueue.GetByID(context.Background(), event.ID)
	if err != nil {
		t.Fatalf("reload queue event: %v", err)
	}
	if storedEvent.Status != models.AIAgentQueueStatusFailed {
		t.Fatalf("expected queue event failed after second attempt, got %q", storedEvent.Status)
	}

	workloads, err = env.aiAgentWorkloads.ListByQueueEvent(context.Background(), event.ID)
	if err != nil {
		t.Fatalf("list workloads after second attempt: %v", err)
	}
	for i := range workloads {
		item := workloads[i]
		switch item.AgentID {
		case goodAgent.ID:
			if item.Status != models.AIAgentWorkloadStatusDone {
				t.Fatalf("expected reliable workload to stay done, got %q", item.Status)
			}
			if item.AttemptCount != 1 {
				t.Fatalf("expected reliable workload to stay at one attempt, got %d", item.AttemptCount)
			}
		case badAgent.ID:
			if item.Status != models.AIAgentWorkloadStatusFailed {
				t.Fatalf("expected failing workload failed after retry exhaustion, got %q", item.Status)
			}
			if item.AttemptCount != 2 {
				t.Fatalf("expected failing workload attempt_count=2 after retry exhaustion, got %d", item.AttemptCount)
			}
		}
	}

	reliableRuns, err := env.catalog.List(context.Background(), repository.CatalogListParams{
		Kind:     aiAgentRunsCatalogKind,
		TenantID: &env.tenantID,
		RefID:    &goodAgent.ID,
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("list reliable runs: %v", err)
	}
	if len(reliableRuns) != 1 {
		t.Fatalf("expected reliable agent to run exactly once, got %d", len(reliableRuns))
	}

	failingRuns, err := env.catalog.List(context.Background(), repository.CatalogListParams{
		Kind:     aiAgentRunsCatalogKind,
		TenantID: &env.tenantID,
		RefID:    &badAgent.ID,
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("list failing runs: %v", err)
	}
	if len(failingRuns) != 2 {
		t.Fatalf("expected failing agent to run twice, got %d", len(failingRuns))
	}
}

func TestAIAgentQueueWorkerCreatesSingleCaseFromAlertForMultipleMatchingAgentsIntegration(t *testing.T) {
	env := newAPITestEnv(t)

	casesBefore, err := env.cases.CountByTenant(context.Background(), env.tenantID)
	if err != nil {
		t.Fatalf("count cases before: %v", err)
	}

	alert, err := env.alerts.Create(context.Background(), repository.CreateAlertParams{
		TenantID:    env.tenantID,
		Title:       "Multi-agent alert auto case",
		Description: "Only one case must be created even when two agents match the alert",
		Source:      "edr",
		Status:      "new",
		Severity:    "high",
		TLP:         "amber",
		PAP:         "amber",
		CreatedBy:   &env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create alert seed: %v", err)
	}

	createAgent := func(name string) uuid.UUID {
		agentItem, createErr := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
			TenantID: &env.tenantID,
			Kind:     "ai_agents",
			OwnerID:  &env.identity.UserID,
			Data: map[string]any{
				"name":                        name,
				"enabled":                     true,
				"target_types":                []string{"alert"},
				"auto_create_case_from_alert": true,
				"auto_create_tasks":           false,
				"auto_comment":                false,
				"max_cases_per_run":           1,
			},
			CreatedBy: &env.identity.UserID,
		})
		if createErr != nil {
			t.Fatalf("create ai agent %s: %v", name, createErr)
		}
		return agentItem.ID
	}
	firstAgentID := createAgent("Alert queue agent A")
	secondAgentID := createAgent("Alert queue agent B")

	event, _, err := env.aiAgentQueue.Enqueue(context.Background(), repository.EnqueueAIAgentQueueEventParams{
		TenantID:   env.tenantID,
		ActorID:    &env.identity.UserID,
		EntityType: "alert",
		EntityID:   alert.ID,
		Source:     aiAgentQueueSourceAPI,
	})
	if err != nil {
		t.Fatalf("enqueue alert event: %v", err)
	}

	processed := env.handler.processNextAIAgentQueueBatch(context.Background(), 10)
	if processed != 1 {
		t.Fatalf("expected one processed queue event, got %d", processed)
	}

	reloadedAlert, err := env.alerts.GetByID(context.Background(), env.tenantID, alert.ID)
	if err != nil {
		t.Fatalf("reload alert: %v", err)
	}
	if reloadedAlert.CaseID == nil {
		t.Fatalf("expected alert to be linked to an auto-created case")
	}

	casesAfter, err := env.cases.CountByTenant(context.Background(), env.tenantID)
	if err != nil {
		t.Fatalf("count cases after: %v", err)
	}
	if casesAfter != casesBefore+1 {
		t.Fatalf("expected exactly one new case, before=%d after=%d", casesBefore, casesAfter)
	}

	workloads, err := env.aiAgentWorkloads.ListByQueueEvent(context.Background(), event.ID)
	if err != nil {
		t.Fatalf("list workloads: %v", err)
	}
	if len(workloads) != 2 {
		t.Fatalf("expected 2 workloads for two matching agents, got %d", len(workloads))
	}
	for _, workload := range workloads {
		if workload.Status != models.AIAgentWorkloadStatusDone {
			t.Fatalf("expected workload %s done, got %q", workload.ID.String(), workload.Status)
		}
	}

	for _, agentID := range []uuid.UUID{firstAgentID, secondAgentID} {
		runs, listErr := env.catalog.List(context.Background(), repository.CatalogListParams{
			Kind:     aiAgentRunsCatalogKind,
			TenantID: &env.tenantID,
			RefID:    &agentID,
			Limit:    10,
		})
		if listErr != nil {
			t.Fatalf("list runs for agent %s: %v", agentID.String(), listErr)
		}
		if len(runs) != 1 {
			t.Fatalf("expected agent %s to run exactly once, got %d", agentID.String(), len(runs))
		}
		results := sliceFromAny(runs[0].Data["results"])
		if len(results) == 0 {
			t.Fatalf("expected result payload for agent %s", agentID.String())
		}
		first := normalizeMap(results[0])
		if got := strings.TrimSpace(stringFromMap(first, "case_id", "caseId")); got != reloadedAlert.CaseID.String() {
			t.Fatalf("expected agent %s to analyze shared case %s, got %q", agentID.String(), reloadedAlert.CaseID.String(), got)
		}
	}
}

func TestAIAgentQueueWorkerBlocksAutoCloseWhenConfidenceBelowThresholdWithoutTriadIntegration(t *testing.T) {
	env := newAPITestEnv(t)

	openAI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/chat/completions":
			writeQueueAgentChatCompletion(w, `{
			"verdict": "benign",
			"confidence": 71,
			"summary": "Confidence is not high enough for automated closure",
			"recommendations": ["Escalate to analyst"],
			"findings": ["One IOC still needs human validation"]
		}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer openAI.Close()

	caseItem, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-QUEUE-CONFIDENCE-1",
		Title:             "Low-confidence queue case",
		Description:       "Auto-close must be blocked when confidence is below threshold even without triad",
		Source:            "manual",
		IncidentType:      "malware",
		Status:            "open",
		Priority:          "high",
		Impact:            "system",
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

	agentItem, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID: &env.tenantID,
		Kind:     "ai_agents",
		OwnerID:  &env.identity.UserID,
		Data: map[string]any{
			"name":                       "Confidence gate queue agent",
			"enabled":                    true,
			"target_types":               []string{"case"},
			"provider":                   "openai",
			"endpoint":                   openAI.URL + "/v1",
			"model":                      "cyankiwi/Qwen3.5-27B-AWQ-4bit",
			"auto_close_case":            true,
			"auto_close_verdicts":        []string{"benign"},
			"auto_action_min_confidence": 85,
			"auto_create_tasks":          false,
			"auto_comment":               false,
			"max_cases_per_run":          1,
		},
		CreatedBy: &env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create ai agent: %v", err)
	}

	env.handler.enqueueAIAgentQueueEvent(context.Background(), env.tenantID, &env.identity.UserID, "case", caseItem.ID, aiAgentQueueSourceAPI)
	processed := env.handler.processNextAIAgentQueueBatch(context.Background(), 10)
	if processed != 1 {
		t.Fatalf("expected one processed queue event, got %d", processed)
	}

	reloaded, err := env.cases.GetByID(context.Background(), env.tenantID, caseItem.ID)
	if err != nil {
		t.Fatalf("reload case: %v", err)
	}
	if env.handler.isCaseStatusClosed(context.Background(), env.tenantID, reloaded.Status) {
		t.Fatalf("expected case to remain open when confidence threshold blocks auto-close")
	}

	runs, err := env.catalog.List(context.Background(), repository.CatalogListParams{
		Kind:     aiAgentRunsCatalogKind,
		TenantID: &env.tenantID,
		RefID:    &agentItem.ID,
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("list ai agent runs: %v", err)
	}
	if len(runs) == 0 {
		t.Fatalf("expected ai agent run")
	}
	resultsAny := sliceFromAny(runs[0].Data["results"])
	if len(resultsAny) == 0 {
		t.Fatalf("expected result payload")
	}
	resultMap := normalizeMap(resultsAny[0])
	if allowed, _ := boolFromMap(resultMap, "auto_actions_allowed", "autoActionsAllowed"); allowed {
		t.Fatalf("expected auto actions to be blocked by confidence threshold")
	}
	if skipped := strings.TrimSpace(stringFromMap(resultMap, "auto_close_skipped_reason")); skipped == "" {
		t.Fatalf("expected auto_close_skipped_reason to be populated")
	}
	blockers := stringSliceFromMap(resultMap, "action_blockers", "actionBlockers")
	joined := strings.ToLower(strings.Join(blockers, " "))
	if !strings.Contains(joined, "confidence") {
		t.Fatalf("expected confidence blocker, got %v", blockers)
	}
}

func TestAIAgentWorkloadManualRestartAndCloseIntegration(t *testing.T) {
	env := newAPITestEnv(t)

	caseItem, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-QUEUE-WORKLOAD-CONTROL-1",
		Title:             "Queue workload controls case",
		Description:       "Manual workload restart and close controls",
		Source:            "manual",
		IncidentType:      "malware",
		Status:            "open",
		Priority:          "medium",
		Impact:            "system",
		Confidence:        60,
		Severity:          "medium",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create case seed: %v", err)
	}

	agentID := uuid.New()
	event, _, err := env.aiAgentQueue.Enqueue(context.Background(), repository.EnqueueAIAgentQueueEventParams{
		TenantID:   env.tenantID,
		ActorID:    &env.identity.UserID,
		EntityType: "case",
		EntityID:   caseItem.ID,
		Source:     aiAgentQueueSourceAPI,
	})
	if err != nil {
		t.Fatalf("enqueue event: %v", err)
	}
	if _, upsertErr := env.aiAgentWorkloads.Upsert(context.Background(), repository.UpsertAIAgentWorkloadParams{
		TenantID:     env.tenantID,
		QueueEventID: event.ID,
		AgentID:      agentID,
		AgentName:    "Manual workload agent",
		EntityType:   "case",
		EntityID:     caseItem.ID,
		MaxAttempts:  3,
	}); upsertErr != nil {
		t.Fatalf("upsert workload: %v", upsertErr)
	}
	workloads, err := env.aiAgentWorkloads.ListByQueueEvent(context.Background(), event.ID)
	if err != nil {
		t.Fatalf("list workloads: %v", err)
	}
	if len(workloads) != 1 {
		t.Fatalf("expected 1 workload, got %d", len(workloads))
	}
	workload := workloads[0]
	processing, err := env.aiAgentWorkloads.MarkProcessing(context.Background(), workload.ID, "wf-workload-control-1")
	if err != nil {
		t.Fatalf("mark workload processing: %v", err)
	}
	if _, workloadFailedErr := env.aiAgentWorkloads.MarkFailed(context.Background(), processing.ID, repository.CompleteAIAgentWorkloadParams{
		WorkflowID: processing.WorkflowID,
		LastStage:  "analysis",
		LastError:  "connector timeout",
	}); workloadFailedErr != nil {
		t.Fatalf("mark workload failed: %v", workloadFailedErr)
	}
	if queueFailedErr := env.aiAgentQueue.MarkFailed(context.Background(), event.ID, repository.CompleteAIAgentQueueEventParams{
		MatchedAgents:   1,
		ProcessedAgents: 0,
		LastError:       "connector timeout",
	}); queueFailedErr != nil {
		t.Fatalf("mark queue event failed: %v", queueFailedErr)
	}

	restartCtx, restartRec := env.jsonContext(http.MethodPost, "/api/v1/ai/agents/ops/workloads/"+workload.ID.String()+"/restart", nil)
	setPath(restartCtx, "/api/v1/ai/agents/ops/workloads/:workloadID/restart", []string{"workloadID"}, []string{workload.ID.String()})
	setIdentity(restartCtx, env.identity)
	setTenant(restartCtx, env.tenantID)
	err = env.handler.RestartAIAgentWorkload(restartCtx)
	mustStatusOK(t, err, restartRec, http.StatusOK)

	restarted := decodeBody[map[string]any](t, restartRec)
	restartedWorkload, ok := restarted["workload"].(map[string]any)
	if !ok {
		t.Fatalf("expected workload payload on restart response")
	}
	if strings.TrimSpace(stringFromMap(restartedWorkload, "status")) != "queued" {
		t.Fatalf("expected queued status after workload restart, got %q", strings.TrimSpace(stringFromMap(restartedWorkload, "status")))
	}
	if int(numberOrZero(restartedWorkload, "attempt_count")) != 0 {
		t.Fatalf("expected workload attempt_count=0 after restart, got %v", restartedWorkload["attempt_count"])
	}
	parentEvent, err := env.aiAgentQueue.GetByID(context.Background(), event.ID)
	if err != nil {
		t.Fatalf("reload parent queue event after restart: %v", err)
	}
	if parentEvent.Status != models.AIAgentQueueStatusQueued {
		t.Fatalf("expected parent queue event queued after workload restart, got %q", parentEvent.Status)
	}

	closeCtx, closeRec := env.jsonContext(http.MethodPost, "/api/v1/ai/agents/ops/workloads/"+workload.ID.String()+"/close", nil)
	setPath(closeCtx, "/api/v1/ai/agents/ops/workloads/:workloadID/close", []string{"workloadID"}, []string{workload.ID.String()})
	setIdentity(closeCtx, env.identity)
	setTenant(closeCtx, env.tenantID)
	err = env.handler.CloseAIAgentWorkload(closeCtx)
	mustStatusOK(t, err, closeRec, http.StatusOK)

	closed := decodeBody[map[string]any](t, closeRec)
	closedWorkload, ok := closed["workload"].(map[string]any)
	if !ok {
		t.Fatalf("expected workload payload on close response")
	}
	if strings.TrimSpace(stringFromMap(closedWorkload, "status")) != "canceled" {
		t.Fatalf("expected canceled status after workload close, got %q", strings.TrimSpace(stringFromMap(closedWorkload, "status")))
	}
	if !boolValueOrDefault(closedWorkload, false, "closed_by_user", "closedByUser") {
		t.Fatalf("expected closed_by_user=true on workload close response")
	}
}

func TestAIAgentQueueWorkerExecutionPolicyExclusiveIntegration(t *testing.T) {
	env := newAPITestEnv(t)
	caseItem := seedAIAgentExecutionPolicyCase(t, env, "Exclusive queue case", []string{"phishing"})

	hits := map[string]int{}
	openAI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		model := strings.TrimSpace(stringFromMap(payload, "model"))
		hits[model]++
		writeQueueAgentChatCompletion(w, `{
			"verdict": "suspicious",
			"confidence": 91,
			"summary": "Exclusive agent completed the review",
			"recommendations": ["Rotate credentials"],
			"findings": ["Single agent selected by exclusive policy"]
		}`)
	}))
	defer openAI.Close()

	highPriorityAgentID := createQueuePolicyAgent(t, env, map[string]any{
		"name":               "Exclusive Primary",
		"enabled":            true,
		"target_types":       []string{"case"},
		"case_tags":          []string{"phishing"},
		"execution_policy":   "exclusive",
		"execution_priority": 50,
		"provider":           "openai",
		"endpoint":           openAI.URL + "/v1",
		"model":              "exclusive-primary",
		"auto_create_tasks":  false,
		"auto_comment":       false,
	})
	createQueuePolicyAgent(t, env, map[string]any{
		"name":               "Exclusive Secondary",
		"enabled":            true,
		"target_types":       []string{"case"},
		"case_tags":          []string{"phishing"},
		"execution_policy":   "exclusive",
		"execution_priority": 10,
		"provider":           "openai",
		"endpoint":           openAI.URL + "/v1",
		"model":              "exclusive-secondary",
		"auto_create_tasks":  false,
		"auto_comment":       false,
	})

	event, _, err := env.aiAgentQueue.Enqueue(context.Background(), repository.EnqueueAIAgentQueueEventParams{
		TenantID:    env.tenantID,
		ActorID:     &env.identity.UserID,
		EntityType:  "case",
		EntityID:    caseItem.ID,
		Source:      aiAgentQueueSourceAPI,
		MaxAttempts: 1,
	})
	if err != nil {
		t.Fatalf("enqueue queue event: %v", err)
	}

	processed := env.handler.processNextAIAgentQueueBatch(context.Background(), 10)
	if processed != 1 {
		t.Fatalf("expected one processed queue event, got %d", processed)
	}

	workloads, err := env.aiAgentWorkloads.ListByQueueEvent(context.Background(), event.ID)
	if err != nil {
		t.Fatalf("list workloads: %v", err)
	}
	if len(workloads) != 1 {
		t.Fatalf("expected exactly one workload for exclusive policy, got %d", len(workloads))
	}
	if workloads[0].AgentID != highPriorityAgentID {
		t.Fatalf("expected high-priority exclusive agent to run, got %s", workloads[0].AgentID.String())
	}
	if workloads[0].ExecutionPolicy != models.AIAgentExecutionPolicyExclusive {
		t.Fatalf("expected exclusive workload policy, got %q", workloads[0].ExecutionPolicy)
	}
	if hits["exclusive-primary"] != 1 {
		t.Fatalf("expected primary exclusive model to run once, got %d", hits["exclusive-primary"])
	}
	if hits["exclusive-secondary"] != 0 {
		t.Fatalf("expected secondary exclusive model to be skipped, got %d", hits["exclusive-secondary"])
	}
}

func TestAIAgentQueueWorkerExecutionPolicyFirstMatchIntegration(t *testing.T) {
	env := newAPITestEnv(t)
	caseItem := seedAIAgentExecutionPolicyCase(t, env, "First-match queue case", []string{"phishing"})

	hits := map[string]int{}
	openAI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		model := strings.TrimSpace(stringFromMap(payload, "model"))
		hits[model]++
		writeQueueAgentChatCompletion(w, `{
			"verdict": "suspicious",
			"confidence": 88,
			"summary": "First-match policy selected the lead agent",
			"recommendations": ["Review authentication logs"],
			"findings": ["First-match winner executed"]
		}`)
	}))
	defer openAI.Close()

	winnerID := createQueuePolicyAgent(t, env, map[string]any{
		"name":               "First Match Winner",
		"enabled":            true,
		"target_types":       []string{"case"},
		"case_tags":          []string{"phishing"},
		"execution_policy":   "first_match",
		"execution_priority": 40,
		"provider":           "openai",
		"endpoint":           openAI.URL + "/v1",
		"model":              "first-match-winner",
		"auto_create_tasks":  false,
		"auto_comment":       false,
	})
	createQueuePolicyAgent(t, env, map[string]any{
		"name":               "First Match Backup",
		"enabled":            true,
		"target_types":       []string{"case"},
		"case_tags":          []string{"phishing"},
		"execution_policy":   "first_match",
		"execution_priority": 5,
		"provider":           "openai",
		"endpoint":           openAI.URL + "/v1",
		"model":              "first-match-backup",
		"auto_create_tasks":  false,
		"auto_comment":       false,
	})
	createQueuePolicyAgent(t, env, map[string]any{
		"name":               "Parallel Agent",
		"enabled":            true,
		"target_types":       []string{"case"},
		"case_tags":          []string{"phishing"},
		"execution_policy":   "all_matching",
		"execution_priority": 100,
		"provider":           "openai",
		"endpoint":           openAI.URL + "/v1",
		"model":              "all-matching-skipped",
		"auto_create_tasks":  false,
		"auto_comment":       false,
	})

	event, _, err := env.aiAgentQueue.Enqueue(context.Background(), repository.EnqueueAIAgentQueueEventParams{
		TenantID:    env.tenantID,
		ActorID:     &env.identity.UserID,
		EntityType:  "case",
		EntityID:    caseItem.ID,
		Source:      aiAgentQueueSourceAPI,
		MaxAttempts: 1,
	})
	if err != nil {
		t.Fatalf("enqueue queue event: %v", err)
	}

	processed := env.handler.processNextAIAgentQueueBatch(context.Background(), 10)
	if processed != 1 {
		t.Fatalf("expected one processed queue event, got %d", processed)
	}

	workloads, err := env.aiAgentWorkloads.ListByQueueEvent(context.Background(), event.ID)
	if err != nil {
		t.Fatalf("list workloads: %v", err)
	}
	if len(workloads) != 1 {
		t.Fatalf("expected exactly one workload for first_match policy, got %d", len(workloads))
	}
	if workloads[0].AgentID != winnerID {
		t.Fatalf("expected first-match winner to run, got %s", workloads[0].AgentID.String())
	}
	if workloads[0].ExecutionPolicy != models.AIAgentExecutionPolicyFirstMatch {
		t.Fatalf("expected first_match workload policy, got %q", workloads[0].ExecutionPolicy)
	}
	if hits["first-match-winner"] != 1 {
		t.Fatalf("expected winner model to run once, got %d", hits["first-match-winner"])
	}
	if hits["first-match-backup"] != 0 || hits["all-matching-skipped"] != 0 {
		t.Fatalf("expected only first-match winner to run, got hits=%v", hits)
	}
}

func TestAIAgentQueueWorkerExecutionPolicyFallbackChainIntegration(t *testing.T) {
	env := newAPITestEnv(t)
	caseItem := seedAIAgentExecutionPolicyCase(t, env, "Fallback queue case", []string{"phishing"})

	hits := map[string]int{}
	openAI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		model := strings.TrimSpace(stringFromMap(payload, "model"))
		hits[model]++
		switch model {
		case "fallback-fail":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"error":{"message":"upstream timeout"}}`)
		case "fallback-success":
			writeQueueAgentChatCompletion(w, `{
			"verdict": "benign",
			"confidence": 93,
			"summary": "Fallback agent completed the investigation",
			"recommendations": ["Close case"],
			"findings": ["Fallback chain reached a successful verdict"]
		}`)
		default:
			writeQueueAgentChatCompletion(w, `{
			"verdict": "suspicious",
			"confidence": 84,
			"summary": "Unexpected fallback agent executed",
			"recommendations": ["Investigate"],
			"findings": ["This agent should have been skipped"]
		}`)
		}
	}))
	defer openAI.Close()

	createQueuePolicyAgent(t, env, map[string]any{
		"name":               "Fallback Primary",
		"enabled":            true,
		"target_types":       []string{"case"},
		"case_tags":          []string{"phishing"},
		"execution_policy":   "fallback_chain",
		"execution_priority": 30,
		"provider":           "openai",
		"endpoint":           openAI.URL + "/v1",
		"model":              "fallback-fail",
		"auto_create_tasks":  false,
		"auto_comment":       false,
	})
	successID := createQueuePolicyAgent(t, env, map[string]any{
		"name":               "Fallback Secondary",
		"enabled":            true,
		"target_types":       []string{"case"},
		"case_tags":          []string{"phishing"},
		"execution_policy":   "fallback_chain",
		"execution_priority": 20,
		"provider":           "openai",
		"endpoint":           openAI.URL + "/v1",
		"model":              "fallback-success",
		"auto_create_tasks":  false,
		"auto_comment":       false,
	})
	createQueuePolicyAgent(t, env, map[string]any{
		"name":               "Fallback Tertiary",
		"enabled":            true,
		"target_types":       []string{"case"},
		"case_tags":          []string{"phishing"},
		"execution_policy":   "fallback_chain",
		"execution_priority": 10,
		"provider":           "openai",
		"endpoint":           openAI.URL + "/v1",
		"model":              "fallback-unused",
		"auto_create_tasks":  false,
		"auto_comment":       false,
	})

	event, _, err := env.aiAgentQueue.Enqueue(context.Background(), repository.EnqueueAIAgentQueueEventParams{
		TenantID:    env.tenantID,
		ActorID:     &env.identity.UserID,
		EntityType:  "case",
		EntityID:    caseItem.ID,
		Source:      aiAgentQueueSourceAPI,
		MaxAttempts: 1,
	})
	if err != nil {
		t.Fatalf("enqueue queue event: %v", err)
	}

	processed := env.handler.processNextAIAgentQueueBatch(context.Background(), 10)
	if processed != 1 {
		t.Fatalf("expected one processed queue event, got %d", processed)
	}

	workloads, err := env.aiAgentWorkloads.ListByQueueEvent(context.Background(), event.ID)
	if err != nil {
		t.Fatalf("list workloads: %v", err)
	}
	if len(workloads) != 3 {
		t.Fatalf("expected three fallback workloads, got %d", len(workloads))
	}
	if workloads[0].Status != models.AIAgentWorkloadStatusFailed {
		t.Fatalf("expected first fallback workload to fail, got %q", workloads[0].Status)
	}
	if workloads[1].AgentID != successID || workloads[1].Status != models.AIAgentWorkloadStatusDone {
		t.Fatalf("expected second fallback workload to succeed, got agent=%s status=%q", workloads[1].AgentID.String(), workloads[1].Status)
	}
	if workloads[2].Status != models.AIAgentWorkloadStatusCancelled {
		t.Fatalf("expected third fallback workload to be canceled after success, got %q", workloads[2].Status)
	}
	if !strings.Contains(workloads[2].LastError, "fallback chain completed") {
		t.Fatalf("expected cancellation reason to mention fallback completion, got %q", workloads[2].LastError)
	}
	if hits["fallback-fail"] < 1 || hits["fallback-success"] != 1 || hits["fallback-unused"] != 0 {
		t.Fatalf("unexpected fallback execution order: %v", hits)
	}
	queueEvent, err := env.aiAgentQueue.GetByID(context.Background(), event.ID)
	if err != nil {
		t.Fatalf("reload queue event: %v", err)
	}
	if queueEvent.Status != models.AIAgentQueueStatusDone {
		t.Fatalf("expected fallback queue event to finish as done, got %q", queueEvent.Status)
	}
}

func TestGetAIAgentEntityTraceIntegration(t *testing.T) {
	env := newAPITestEnv(t)
	caseItem := seedAIAgentExecutionPolicyCase(t, env, "Trace queue case", []string{"phishing"})

	openAI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		writeQueueAgentChatCompletion(w, `{
			"verdict": "suspicious",
			"confidence": 89,
			"summary": "Trace endpoint captured the queue run",
			"recommendations": ["Escalate case"],
			"findings": ["Trace snapshot contains workload details"]
		}`)
	}))
	defer openAI.Close()

	createQueuePolicyAgent(t, env, map[string]any{
		"name":               "Trace Agent",
		"enabled":            true,
		"target_types":       []string{"case"},
		"case_tags":          []string{"phishing"},
		"execution_policy":   "fallback_chain",
		"execution_priority": 12,
		"provider":           "openai",
		"endpoint":           openAI.URL + "/v1",
		"model":              "trace-agent",
		"auto_create_tasks":  false,
		"auto_comment":       false,
	})

	_, _, err := env.aiAgentQueue.Enqueue(context.Background(), repository.EnqueueAIAgentQueueEventParams{
		TenantID:    env.tenantID,
		ActorID:     &env.identity.UserID,
		EntityType:  "case",
		EntityID:    caseItem.ID,
		Source:      aiAgentQueueSourceAPI,
		MaxAttempts: 1,
	})
	if err != nil {
		t.Fatalf("enqueue queue event: %v", err)
	}
	processed := env.handler.processNextAIAgentQueueBatch(context.Background(), 10)
	if processed != 1 {
		t.Fatalf("expected one processed queue event, got %d", processed)
	}

	ctx, rec := env.jsonContext(http.MethodGet, "/api/v1/ai/agents/entities/case/"+caseItem.ID.String(), nil)
	setPath(ctx, "/api/v1/ai/agents/entities/:entityType/:entityID", []string{"entityType", "entityID"}, []string{"case", caseItem.ID.String()})
	setIdentity(ctx, env.identity)
	setTenant(ctx, env.tenantID)
	err = env.handler.GetAIAgentEntityTrace(ctx)
	mustStatusOK(t, err, rec, http.StatusOK)

	payload := decodeBody[map[string]any](t, rec)
	events, ok := payload["events"].([]any)
	if !ok || len(events) != 1 {
		t.Fatalf("expected one queue event in trace payload, got %#v", payload["events"])
	}
	event := normalizeMap(events[0])
	if strings.TrimSpace(stringFromMap(event, "execution_policy", "executionPolicy")) != "fallback_chain" {
		t.Fatalf("expected event execution_policy=fallback_chain, got %#v", event["execution_policy"])
	}
	workloads, ok := event["workloads"].([]any)
	if !ok || len(workloads) != 1 {
		t.Fatalf("expected one workload in trace payload, got %#v", event["workloads"])
	}
	workload := normalizeMap(workloads[0])
	if strings.TrimSpace(stringFromMap(workload, "execution_policy", "executionPolicy")) != "fallback_chain" {
		t.Fatalf("expected workload execution_policy=fallback_chain, got %#v", workload["execution_policy"])
	}
	if int(numberOrZero(workload, "execution_priority", "executionPriority")) != 12 {
		t.Fatalf("expected workload execution_priority=12, got %#v", workload["execution_priority"])
	}
	runPayload := normalizeMap(workload["run"])
	result := normalizeMap(runPayload["result"])
	if strings.TrimSpace(stringFromMap(result, "summary")) == "" {
		t.Fatalf("expected run result summary in trace payload, got %#v", runPayload)
	}
}

func seedAIAgentExecutionPolicyCase(t *testing.T, env *apiTestEnv, title string, tags []string) *models.Case {
	t.Helper()
	caseItem, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:          env.tenantID,
		CaseNumber:        "CASE-POLICY-" + strings.ToUpper(uuid.NewString()[:8]),
		Title:             title,
		Description:       "Execution policy integration test case",
		Source:            "manual",
		IncidentType:      "phishing",
		Status:            "open",
		Priority:          "high",
		Impact:            "user",
		Confidence:        78,
		Severity:          "high",
		TLP:               "amber",
		PAP:               "amber",
		ResolutionSummary: "",
		CreatedBy:         env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create queue policy case: %v", err)
	}
	if len(tags) > 0 {
		if _, metaErr := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
			TenantID: &env.tenantID,
			Kind:     "case_meta",
			OwnerID:  &env.identity.UserID,
			RefID:    &caseItem.ID,
			Data: map[string]any{
				"case_id":   caseItem.ID.String(),
				"tenant_id": env.tenantID.String(),
				"tags":      tags,
			},
			CreatedBy: &env.identity.UserID,
		}); metaErr != nil {
			t.Fatalf("create case meta: %v", metaErr)
		}
	}
	return caseItem
}

func createQueuePolicyAgent(t *testing.T, env *apiTestEnv, data map[string]any) uuid.UUID {
	t.Helper()
	agentItem, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID:  &env.tenantID,
		Kind:      "ai_agents",
		OwnerID:   &env.identity.UserID,
		Data:      data,
		CreatedBy: &env.identity.UserID,
	})
	if err != nil {
		t.Fatalf("create queue policy agent: %v", err)
	}
	return agentItem.ID
}

func writeQueueAgentChatCompletion(w http.ResponseWriter, content string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"choices": []any{
			map[string]any{
				"message": map[string]any{
					"role":    "assistant",
					"content": content,
				},
			},
		},
	})
}
