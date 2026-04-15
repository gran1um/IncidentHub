package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func prepareFixtures(ctx context.Context, client *apiClient, session authSession, _ config) (fixtureData, error) {
	fixture := fixtureData{
		SearchProbeTerm: fmt.Sprintf("loadtest-probe-%d", time.Now().UTC().Unix()),
	}

	openStatus, err := fetchOpenCaseStatus(ctx, client, session)
	if err != nil {
		return fixtureData{}, err
	}
	fixture.OpenCaseStatus = openStatus

	caseID, err := createCaseFixture(ctx, client, session, openStatus)
	if err != nil {
		return fixtureData{}, err
	}
	fixture.CaseID = caseID
	return fixture, nil
}

func fetchOpenCaseStatus(ctx context.Context, client *apiClient, session authSession) (string, error) {
	resp, err := client.request(ctx, http.MethodGet, "/api/v1/case-statuses", nil, "", session.Token, session.TenantID, true)
	if err != nil {
		return "", fmt.Errorf("request case statuses: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("case statuses request failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed struct {
		Statuses []struct {
			Code     string `json:"code"`
			IsClosed bool   `json:"is_closed"`
		} `json:"statuses"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("decode case statuses: %w", err)
	}
	for _, item := range parsed.Statuses {
		if !item.IsClosed && strings.TrimSpace(item.Code) != "" {
			return item.Code, nil
		}
	}
	if len(parsed.Statuses) > 0 && strings.TrimSpace(parsed.Statuses[0].Code) != "" {
		return parsed.Statuses[0].Code, nil
	}
	return "new", nil
}

func createCaseFixture(ctx context.Context, client *apiClient, session authSession, status string) (string, error) {
	payload := map[string]any{
		"title":    fmt.Sprintf("Load Test Fixture Case %d", time.Now().UTC().UnixNano()),
		"severity": "medium",
		"source":   "loadtest",
		"status":   status,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal fixture case payload: %w", err)
	}

	resp, err := client.request(ctx, http.MethodPost, "/api/v1/cases", raw, "application/json", session.Token, session.TenantID, true)
	if err != nil {
		return "", fmt.Errorf("create fixture case request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("create fixture case failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	// Async mode returns 202 with operation envelope; sync mode returns case payload with id.
	if resp.StatusCode == http.StatusAccepted {
		var queued struct {
			OperationID string `json:"operation_id"`
			ResourceID  string `json:"resource_id"`
			Status      string `json:"status"`
		}
		if err := json.Unmarshal(body, &queued); err != nil {
			return "", fmt.Errorf("decode queued fixture case: %w", err)
		}
		resourceID := strings.TrimSpace(queued.ResourceID)
		if resourceID == "" {
			return "", fmt.Errorf("queued fixture case response missing resource_id")
		}
		operationID := strings.TrimSpace(queued.OperationID)
		if operationID != "" {
			if err := waitForAsyncOperationDone(ctx, client, session, operationID, 45*time.Second); err != nil {
				return "", err
			}
		}
		return resourceID, nil
	}

	var parsed struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("decode fixture case: %w", err)
	}
	if strings.TrimSpace(parsed.ID) == "" {
		return "", fmt.Errorf("fixture case response missing id")
	}
	return parsed.ID, nil
}

func waitForAsyncOperationDone(
	ctx context.Context,
	client *apiClient,
	session authSession,
	operationID string,
	timeout time.Duration,
) error {
	if timeout <= 0 {
		timeout = 45 * time.Second
	}
	pollCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	delay := 250 * time.Millisecond
	for {
		select {
		case <-pollCtx.Done():
			return fmt.Errorf("wait fixture async operation %s: %w", operationID, pollCtx.Err())
		default:
		}

		resp, err := client.request(
			pollCtx,
			http.MethodGet,
			"/api/v1/operations/"+operationID,
			nil,
			"",
			session.Token,
			session.TenantID,
			true,
		)
		if err != nil {
			return fmt.Errorf("request async operation %s: %w", operationID, err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return fmt.Errorf("read async operation %s failed: status=%d body=%s", operationID, resp.StatusCode, strings.TrimSpace(string(body)))
		}

		var statusPayload struct {
			Status string `json:"status"`
			Error  string `json:"error"`
		}
		if err := json.Unmarshal(body, &statusPayload); err != nil {
			return fmt.Errorf("decode async operation %s status: %w", operationID, err)
		}
		status := strings.ToLower(strings.TrimSpace(statusPayload.Status))
		switch status {
		case "done":
			return nil
		case "failed":
			lastError := strings.TrimSpace(statusPayload.Error)
			if lastError == "" {
				lastError = "operation marked as failed"
			}
			return fmt.Errorf("async operation %s failed: %s", operationID, lastError)
		case "queued", "processing":
			time.Sleep(delay)
			if delay < 1500*time.Millisecond {
				delay = time.Duration(float64(delay) * 1.45)
				if delay > 1500*time.Millisecond {
					delay = 1500 * time.Millisecond
				}
			}
		default:
			return fmt.Errorf("async operation %s returned unexpected status %q", operationID, status)
		}
	}
}

func buildScenarios(cfg config) []scenario {
	readScenarios := []scenario{
		{
			Name:          "Read Profile",
			Category:      "read",
			Method:        http.MethodGet,
			PathTemplate:  "/api/v1/me",
			RequireTenant: false,
			Build: func(_ uint64, _ fixtureData) (string, []byte, string) {
				return "/api/v1/me", nil, ""
			},
		},
		{
			Name:          "Read Alerts List",
			Category:      "read",
			Method:        http.MethodGet,
			PathTemplate:  "/api/v1/alerts",
			RequireTenant: true,
			Build: func(_ uint64, _ fixtureData) (string, []byte, string) {
				return "/api/v1/alerts", nil, ""
			},
		},
		{
			Name:          "Read Cases List",
			Category:      "read",
			Method:        http.MethodGet,
			PathTemplate:  "/api/v1/cases",
			RequireTenant: true,
			Build: func(_ uint64, _ fixtureData) (string, []byte, string) {
				return "/api/v1/cases", nil, ""
			},
		},
		{
			Name:          "Read Case Detail",
			Category:      "read",
			Method:        http.MethodGet,
			PathTemplate:  "/api/v1/cases/{fixture_case_id}",
			RequireTenant: true,
			Build: func(_ uint64, fixtures fixtureData) (string, []byte, string) {
				return "/api/v1/cases/" + fixtures.CaseID, nil, ""
			},
		},
		{
			Name:          "Read Search",
			Category:      "read",
			Method:        http.MethodGet,
			PathTemplate:  "/api/v1/search?q=loadtest",
			RequireTenant: true,
			Build: func(_ uint64, fixtures fixtureData) (string, []byte, string) {
				return "/api/v1/search?q=" + fixtures.SearchProbeTerm, nil, ""
			},
		},
	}

	writeScenarios := []scenario{
		{
			Name:          "Write Alert",
			Category:      "write",
			Method:        http.MethodPost,
			PathTemplate:  "/api/v1/alerts",
			RequireTenant: true,
			Build: func(seq uint64, _ fixtureData) (string, []byte, string) {
				payload := fmt.Sprintf(
					`{"title":"LoadTest Alert %d","description":"synthetic alert","source":"loadtest","severity":"high"}`,
					seq,
				)
				return "/api/v1/alerts", []byte(payload), "application/json"
			},
		},
	}
	if cfg.IncludeCaseWrites {
		writeScenarios = append(writeScenarios, scenario{
			Name:          "Write Case",
			Category:      "write",
			Method:        http.MethodPost,
			PathTemplate:  "/api/v1/cases",
			RequireTenant: true,
			Build: func(seq uint64, fixtures fixtureData) (string, []byte, string) {
				payload := fmt.Sprintf(
					`{"title":"LoadTest Case %d","description":"synthetic case","source":"loadtest","severity":"medium","status":%q}`,
					seq,
					fixtures.OpenCaseStatus,
				)
				return "/api/v1/cases", []byte(payload), "application/json"
			},
		})
	}

	out := make([]scenario, 0, len(readScenarios)+len(writeScenarios))
	out = append(out, readScenarios...)
	out = append(out, writeScenarios...)
	return out
}
