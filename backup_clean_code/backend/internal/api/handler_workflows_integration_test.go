package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"incidenthub/backend/internal/repository"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestWorkflowRedisNodeIntegration(t *testing.T) {
	env := newAPITestEnv(t)
	redisServer := miniredis.RunT(t)

	workflowItem, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID:  &env.tenantID,
		Kind:      "workflows",
		OwnerID:   &env.userID,
		CreatedBy: &env.userID,
		Data: map[string]any{
			"name":        "Redis integration workflow",
			"description": "Workflow runtime integration with Redis node",
			"enabled":     true,
			"status":      "published",
			"definition": map[string]any{
				"entryNodeId": "trigger_1",
				"nodes": []map[string]any{
					{
						"id":    "trigger_1",
						"type":  "trigger",
						"label": "Start",
						"config": map[string]any{
							"event": "manual",
						},
					},
					{
						"id":    "redis_set_1",
						"type":  "redis",
						"label": "Redis SET",
						"config": map[string]any{
							"operation": "set",
							"addr":      redisServer.Addr(),
							"db":        0,
							"key":       "workflow:int:{{input.case_id}}",
							"value":     "{{input.message}}",
						},
					},
					{
						"id":    "redis_get_1",
						"type":  "redis",
						"label": "Redis GET",
						"config": map[string]any{
							"operation": "get",
							"addr":      redisServer.Addr(),
							"db":        0,
							"key":       "workflow:int:{{input.case_id}}",
						},
					},
				},
				"edges": []map[string]any{
					{"source": "trigger_1", "target": "redis_set_1", "label": "next"},
					{"source": "redis_set_1", "target": "redis_get_1", "label": "next"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create workflow catalog item: %v", err)
	}

	runInput := map[string]any{
		"case_id": "CASE-REDIS-INT-1",
		"message": "workflow redis integration message",
	}

	c, rec := env.jsonContext(http.MethodPost, "/api/v1/workflows/"+workflowItem.ID.String()+"/run", map[string]any{
		"input": runInput,
	})
	setPath(c, "/api/v1/workflows/:workflowID/run", []string{"workflowID"}, []string{workflowItem.ID.String()})
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)

	callErr := env.handler.RunWorkflow(c)
	mustStatusOK(t, callErr, rec, http.StatusOK)

	payload := decodeBody[map[string]any](t, rec)
	ok, _ := payload["ok"].(bool)
	if !ok {
		t.Fatalf("expected successful workflow run, got payload=%v", payload)
	}

	runPayload, _ := payload["run"].(map[string]any)
	if runPayload == nil {
		t.Fatalf("expected run payload in workflow response")
	}
	if got, _ := runPayload["status"].(string); got != "success" {
		t.Fatalf("expected workflow run status=success, got %q", got)
	}

	resultPayload, _ := runPayload["result"].(map[string]any)
	finalOutput, _ := resultPayload["final_output"].(map[string]any)
	if got := strings.TrimSpace(fmt.Sprint(finalOutput["redis_value"])); got != "workflow redis integration message" {
		t.Fatalf("unexpected redis_value in final output: %q", got)
	}

	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	defer func() { _ = redisClient.Close() }()

	stored, err := redisClient.Get(context.Background(), "workflow:int:CASE-REDIS-INT-1").Result()
	if err != nil {
		t.Fatalf("read value from integration redis server: %v", err)
	}
	if stored != "workflow redis integration message" {
		t.Fatalf("unexpected redis stored value: %q", stored)
	}

	{
		c, rec := env.jsonContext(http.MethodGet, "/api/v1/workflows/"+workflowItem.ID.String()+"/runs?limit=10", nil)
		setPath(c, "/api/v1/workflows/:workflowID/runs", []string{"workflowID"}, []string{workflowItem.ID.String()})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.ListWorkflowRuns(c)
		mustStatusOK(t, err, rec, http.StatusOK)

		historyPayload := decodeBody[map[string]any](t, rec)
		runs, _ := historyPayload["runs"].([]any)
		if len(runs) != 1 {
			t.Fatalf("expected 1 workflow run in history, got %d", len(runs))
		}
	}
}
