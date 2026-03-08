package api

import (
	"context"
	"net/http"
	"testing"

	"incidenthub/backend/internal/repository"
)

func TestListAlertsPagination(t *testing.T) {
	env := newAPITestEnv(t)

	for idx := 0; idx < 11; idx++ {
		_, err := env.alerts.Create(context.Background(), repository.CreateAlertParams{
			TenantID:    env.tenantID,
			Title:       "Alert page test",
			Description: "pagination",
			Source:      "test",
			Status:      "new",
			Severity:    "high",
			TLP:         "amber",
			PAP:         "amber",
			CreatedBy:   &env.userID,
		})
		if err != nil {
			t.Fatalf("seed alert: %v", err)
		}
	}
	_, err := env.alerts.Create(context.Background(), repository.CreateAlertParams{
		TenantID:    env.tenantID,
		Title:       "Alert assigned to current user",
		Description: "pagination-assigned",
		Source:      "test",
		Status:      "new",
		Severity:    "high",
		TLP:         "amber",
		PAP:         "amber",
		CreatedBy:   &env.userID,
		AssignedTo:  &env.userID,
	})
	if err != nil {
		t.Fatalf("seed assigned alert for current user: %v", err)
	}
	_, err = env.alerts.Create(context.Background(), repository.CreateAlertParams{
		TenantID:    env.tenantID,
		Title:       "Alert assigned to another user",
		Description: "pagination-assigned",
		Source:      "test",
		Status:      "new",
		Severity:    "high",
		TLP:         "amber",
		PAP:         "amber",
		CreatedBy:   &env.userID,
		AssignedTo:  &env.secondUserID,
	})
	if err != nil {
		t.Fatalf("seed assigned alert for another user: %v", err)
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/alerts?page=2&page_size=10", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListAlerts(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		payload := decodeBody[map[string]any](t, rec)
		if page, _ := payload["page"].(float64); int(page) != 2 {
			t.Fatalf("expected page=2, got %v", payload["page"])
		}
		if pageSize, _ := payload["page_size"].(float64); int(pageSize) != 10 {
			t.Fatalf("expected page_size=10, got %v", payload["page_size"])
		}
		if total, _ := payload["total"].(float64); int(total) != 13 {
			t.Fatalf("expected total=13, got %v", payload["total"])
		}
		items, _ := payload["items"].([]any)
		if len(items) != 3 {
			t.Fatalf("expected 3 items on second page, got %d", len(items))
		}
	}

	{
		c, _ := env.jsonContext(http.MethodGet, "/api/v1/alerts?page=1&page_size=25", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListAlerts(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("expected bad request for invalid page_size, got %d", code)
		}
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/alerts?page=1&page_size=30&assigned=assigned", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListAlerts(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		payload := decodeBody[map[string]any](t, rec)
		if total, _ := payload["total"].(float64); int(total) != 2 {
			t.Fatalf("expected assigned total=2, got %v", payload["total"])
		}
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/alerts?page=1&page_size=30&assigned=unassigned", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListAlerts(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		payload := decodeBody[map[string]any](t, rec)
		if total, _ := payload["total"].(float64); int(total) != 11 {
			t.Fatalf("expected unassigned total=11, got %v", payload["total"])
		}
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/alerts?page=1&page_size=30&assigned=mine", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListAlerts(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		payload := decodeBody[map[string]any](t, rec)
		if total, _ := payload["total"].(float64); int(total) != 1 {
			t.Fatalf("expected mine total=1, got %v", payload["total"])
		}
	}
}

func TestListCasesPagination(t *testing.T) {
	env := newAPITestEnv(t)

	for idx := 0; idx < 11; idx++ {
		_, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
			TenantID:     env.tenantID,
			CaseNumber:   generateCaseNumber(),
			Title:        "Case page test",
			Description:  "pagination",
			Source:       "manual",
			IncidentType: "ops",
			Status:       "open",
			Priority:     "high",
			Impact:       "medium",
			Confidence:   70,
			Severity:     "high",
			TLP:          "amber",
			PAP:          "amber",
			CreatedBy:    env.userID,
		})
		if err != nil {
			t.Fatalf("seed case: %v", err)
		}
	}
	_, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:     env.tenantID,
		CaseNumber:   generateCaseNumber(),
		Title:        "Case assigned to current user",
		Description:  "pagination-assigned",
		Source:       "manual",
		IncidentType: "ops",
		Status:       "open",
		Priority:     "high",
		Impact:       "medium",
		Confidence:   70,
		Severity:     "high",
		TLP:          "amber",
		PAP:          "amber",
		CreatedBy:    env.userID,
		AssignedTo:   &env.userID,
	})
	if err != nil {
		t.Fatalf("seed assigned case for current user: %v", err)
	}
	_, err = env.cases.Create(context.Background(), repository.CreateCaseParams{
		TenantID:     env.tenantID,
		CaseNumber:   generateCaseNumber(),
		Title:        "Case assigned to another user",
		Description:  "pagination-assigned",
		Source:       "manual",
		IncidentType: "ops",
		Status:       "open",
		Priority:     "high",
		Impact:       "medium",
		Confidence:   70,
		Severity:     "high",
		TLP:          "amber",
		PAP:          "amber",
		CreatedBy:    env.userID,
		AssignedTo:   &env.secondUserID,
	})
	if err != nil {
		t.Fatalf("seed assigned case for another user: %v", err)
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/cases?page=2&page_size=10", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListCases(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		payload := decodeBody[map[string]any](t, rec)
		if page, _ := payload["page"].(float64); int(page) != 2 {
			t.Fatalf("expected page=2, got %v", payload["page"])
		}
		if pageSize, _ := payload["page_size"].(float64); int(pageSize) != 10 {
			t.Fatalf("expected page_size=10, got %v", payload["page_size"])
		}
		if total, _ := payload["total"].(float64); int(total) != 13 {
			t.Fatalf("expected total=13, got %v", payload["total"])
		}
		items, _ := payload["items"].([]any)
		if len(items) != 3 {
			t.Fatalf("expected 3 items on second page, got %d", len(items))
		}
	}

	{
		c, _ := env.jsonContext(http.MethodGet, "/api/v1/cases?page=1&page_size=15", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListCases(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("expected bad request for invalid page_size, got %d", code)
		}
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/cases?page=1&page_size=30&assigned=assigned", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListCases(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		payload := decodeBody[map[string]any](t, rec)
		if total, _ := payload["total"].(float64); int(total) != 2 {
			t.Fatalf("expected assigned total=2, got %v", payload["total"])
		}
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/cases?page=1&page_size=30&assigned=unassigned", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListCases(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		payload := decodeBody[map[string]any](t, rec)
		if total, _ := payload["total"].(float64); int(total) != 11 {
			t.Fatalf("expected unassigned total=11, got %v", payload["total"])
		}
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/cases?page=1&page_size=30&assigned=mine", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListCases(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		payload := decodeBody[map[string]any](t, rec)
		if total, _ := payload["total"].(float64); int(total) != 1 {
			t.Fatalf("expected mine total=1, got %v", payload["total"])
		}
	}
}

func TestListCasesSorting(t *testing.T) {
	env := newAPITestEnv(t)

	customFieldByTitle := map[string]string{
		"Zulu case":  "omega",
		"Alpha case": "beta",
		"Bravo case": "alpha",
	}
	for _, title := range []string{"Zulu case", "Alpha case", "Bravo case"} {
		createdCase, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
			TenantID:     env.tenantID,
			CaseNumber:   generateCaseNumber(),
			Title:        title,
			Description:  "sorting",
			Source:       "manual",
			IncidentType: "ops",
			Status:       "open",
			Priority:     "high",
			Impact:       "medium",
			Confidence:   70,
			Severity:     "high",
			TLP:          "amber",
			PAP:          "amber",
			CreatedBy:    env.userID,
		})
		if err != nil {
			t.Fatalf("seed case: %v", err)
		}
		_, err = env.catalog.Create(context.Background(), repository.CatalogCreateParams{
			TenantID:  &env.tenantID,
			Kind:      "case_meta",
			OwnerID:   &env.userID,
			RefID:     &createdCase.ID,
			CreatedBy: &env.userID,
			Data: map[string]any{
				"case_id": createdCase.ID.String(),
				"custom_fields": map[string]any{
					"client": customFieldByTitle[title],
				},
			},
		})
		if err != nil {
			t.Fatalf("seed case_meta: %v", err)
		}
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/cases?page=1&page_size=30&sort_by=title&sort_order=asc", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListCases(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		payload := decodeBody[map[string]any](t, rec)
		items, _ := payload["items"].([]any)
		if len(items) < 3 {
			t.Fatalf("expected >= 3 items, got %d", len(items))
		}
		first, _ := items[0].(map[string]any)
		second, _ := items[1].(map[string]any)
		third, _ := items[2].(map[string]any)
		if first["title"] != "Alpha case" || second["title"] != "Bravo case" || third["title"] != "Zulu case" {
			t.Fatalf("unexpected title order: %v, %v, %v", first["title"], second["title"], third["title"])
		}
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/cases?page=1&page_size=30&sort_by=cf:client&sort_order=asc", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListCases(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		payload := decodeBody[map[string]any](t, rec)
		items, _ := payload["items"].([]any)
		if len(items) < 3 {
			t.Fatalf("expected >= 3 items, got %d", len(items))
		}
		titleIndex := make(map[string]int, len(items))
		for idx, raw := range items {
			item, _ := raw.(map[string]any)
			title, _ := item["title"].(string)
			if title == "" {
				continue
			}
			titleIndex[title] = idx
		}
		if titleIndex["Bravo case"] >= titleIndex["Alpha case"] || titleIndex["Alpha case"] >= titleIndex["Zulu case"] {
			t.Fatalf("unexpected custom-field order by client: %+v", titleIndex)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodGet, "/api/v1/cases?page=1&page_size=30&sort_by=unknown", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListCases(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("expected bad request for invalid sort_by, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodGet, "/api/v1/cases?page=1&page_size=30&sort_by=cf:bad$field", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListCases(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("expected bad request for invalid custom sort_by, got %d", code)
		}
	}
}

func TestListAlertsSorting(t *testing.T) {
	env := newAPITestEnv(t)

	for _, title := range []string{"Zulu alert", "Alpha alert", "Bravo alert"} {
		_, err := env.alerts.Create(context.Background(), repository.CreateAlertParams{
			TenantID:    env.tenantID,
			Title:       title,
			Description: "sorting",
			Source:      "test",
			Status:      "new",
			Severity:    "high",
			TLP:         "amber",
			PAP:         "amber",
			CreatedBy:   &env.userID,
		})
		if err != nil {
			t.Fatalf("seed alert: %v", err)
		}
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/alerts?page=1&page_size=30&sort_by=title&sort_order=asc", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListAlerts(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		payload := decodeBody[map[string]any](t, rec)
		items, _ := payload["items"].([]any)
		if len(items) < 3 {
			t.Fatalf("expected >= 3 items, got %d", len(items))
		}
		first, _ := items[0].(map[string]any)
		second, _ := items[1].(map[string]any)
		third, _ := items[2].(map[string]any)
		if first["title"] != "Alpha alert" || second["title"] != "Bravo alert" || third["title"] != "Zulu alert" {
			t.Fatalf("unexpected title order: %v, %v, %v", first["title"], second["title"], third["title"])
		}
	}

	{
		c, _ := env.jsonContext(http.MethodGet, "/api/v1/alerts?page=1&page_size=30&sort_order=invalid", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListAlerts(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("expected bad request for invalid sort_order, got %d", code)
		}
	}
}

func TestListCasesSearch(t *testing.T) {
	env := newAPITestEnv(t)

	seed := []string{
		"Credential leak detected",
		"DNS anomaly",
		"Credential malware combo",
	}
	for _, title := range seed {
		_, err := env.cases.Create(context.Background(), repository.CreateCaseParams{
			TenantID:     env.tenantID,
			CaseNumber:   generateCaseNumber(),
			Title:        title,
			Description:  "search-check",
			Source:       "manual",
			IncidentType: "ops",
			Status:       "open",
			Priority:     "high",
			Impact:       "medium",
			Confidence:   70,
			Severity:     "high",
			TLP:          "amber",
			PAP:          "amber",
			CreatedBy:    env.userID,
		})
		if err != nil {
			t.Fatalf("seed case: %v", err)
		}
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/cases?page=1&page_size=30&q=credential", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListCases(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		payload := decodeBody[map[string]any](t, rec)
		if total, _ := payload["total"].(float64); int(total) != 2 {
			t.Fatalf("expected total=2 for credential search, got %v", payload["total"])
		}
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/cases?page=1&page_size=30&q=credential%20malware&search_logic=all", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListCases(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		payload := decodeBody[map[string]any](t, rec)
		if total, _ := payload["total"].(float64); int(total) != 1 {
			t.Fatalf("expected total=1 for all-logic search, got %v", payload["total"])
		}
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/cases?page=1&page_size=30&q=credential|dns&search_mode=regex", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListCases(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		payload := decodeBody[map[string]any](t, rec)
		if total, _ := payload["total"].(float64); int(total) != 3 {
			t.Fatalf("expected total=3 for regex search, got %v", payload["total"])
		}
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/cases?page=1&page_size=30&q=credential%20OR%20dns&search_mode=fulltext", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListCases(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		payload := decodeBody[map[string]any](t, rec)
		if total, _ := payload["total"].(float64); int(total) != 3 {
			t.Fatalf("expected total=3 for fulltext search, got %v", payload["total"])
		}
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/cases?page=1&page_size=30&q=credential&q_not=combo", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListCases(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		payload := decodeBody[map[string]any](t, rec)
		if total, _ := payload["total"].(float64); int(total) != 1 {
			t.Fatalf("expected total=1 for exclude search, got %v", payload["total"])
		}
	}

	{
		c, _ := env.jsonContext(http.MethodGet, "/api/v1/cases?page=1&page_size=30&search_mode=regex&q=(", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListCases(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("expected bad request for invalid regex, got %d", code)
		}
	}
}

func TestListAlertsSearch(t *testing.T) {
	env := newAPITestEnv(t)

	seed := []string{
		"Suspicious powershell",
		"DNS exfiltration",
		"Malware attachment",
	}
	for _, title := range seed {
		_, err := env.alerts.Create(context.Background(), repository.CreateAlertParams{
			TenantID:    env.tenantID,
			Title:       title,
			Description: "search-check",
			Source:      "test",
			Status:      "new",
			Severity:    "high",
			TLP:         "amber",
			PAP:         "amber",
			CreatedBy:   &env.userID,
		})
		if err != nil {
			t.Fatalf("seed alert: %v", err)
		}
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/alerts?page=1&page_size=30&q=dns", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListAlerts(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		payload := decodeBody[map[string]any](t, rec)
		if total, _ := payload["total"].(float64); int(total) != 1 {
			t.Fatalf("expected total=1 for dns search, got %v", payload["total"])
		}
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/alerts?page=1&page_size=30&q=powershell%20attachment&search_logic=any", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListAlerts(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		payload := decodeBody[map[string]any](t, rec)
		if total, _ := payload["total"].(float64); int(total) != 2 {
			t.Fatalf("expected total=2 for any-logic search, got %v", payload["total"])
		}
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/alerts?page=1&page_size=30&q=powershell|malware&search_mode=regex", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListAlerts(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		payload := decodeBody[map[string]any](t, rec)
		if total, _ := payload["total"].(float64); int(total) != 2 {
			t.Fatalf("expected total=2 for regex search, got %v", payload["total"])
		}
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/alerts?page=1&page_size=30&q=powershell%20OR%20malware&search_mode=fulltext", nil)
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListAlerts(c)
		mustStatusOK(t, err, rec, http.StatusOK)
		payload := decodeBody[map[string]any](t, rec)
		if total, _ := payload["total"].(float64); int(total) != 2 {
			t.Fatalf("expected total=2 for fulltext search, got %v", payload["total"])
		}
	}
}
