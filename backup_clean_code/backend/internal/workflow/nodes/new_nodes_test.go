package nodes

import (
	"context"
	"os/exec"
	"testing"

	"incidenthub/backend/internal/workflow"

	miniredis "github.com/alicebob/miniredis/v2"
)

func TestSwitchNodeRoutesToMatchingLabel(t *testing.T) {
	node := newSwitchNode()
	result, err := node.Execute(context.Background(), workflow.NodeExecuteRequest{
		Node: workflow.Node{
			ID:    "switch_1",
			Type:  "switch",
			Label: "Switch",
			Config: map[string]any{
				"sourcePath": "input.severity",
				"cases": []map[string]any{
					{"value": "critical", "label": "critical"},
					{"value": "high", "label": "high"},
				},
				"defaultLabel": "default",
			},
		},
		Input: map[string]any{
			"severity": "critical",
		},
		Payload: map[string]any{},
		Scope: map[string]any{
			"input": map[string]any{"severity": "critical"},
		},
	})
	if err != nil {
		t.Fatalf("switch node execute: %v", err)
	}
	if result.NextLabel != "critical" {
		t.Fatalf("expected next label critical, got %q", result.NextLabel)
	}
}

func TestAggregateNodeCalculatesSum(t *testing.T) {
	node := newAggregateNode()
	result, err := node.Execute(context.Background(), workflow.NodeExecuteRequest{
		Node: workflow.Node{
			ID:    "aggregate_1",
			Type:  "aggregate",
			Label: "Aggregate",
			Config: map[string]any{
				"sourcePath": "payload.items",
				"valuePath":  "score",
				"operation":  "sum",
				"targetKey":  "total_score",
			},
		},
		Payload: map[string]any{
			"items": []any{
				map[string]any{"score": 2},
				map[string]any{"score": 3.5},
			},
		},
		Scope: map[string]any{
			"payload": map[string]any{
				"items": []any{
					map[string]any{"score": 2},
					map[string]any{"score": 3.5},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("aggregate node execute: %v", err)
	}
	total, ok := result.Output["total_score"].(float64)
	if !ok {
		t.Fatalf("expected total_score float64, got %T", result.Output["total_score"])
	}
	if total != 5.5 {
		t.Fatalf("expected total_score=5.5, got %v", total)
	}
}

func TestRedisNodeSetAndGet(t *testing.T) {
	redisServer := miniredis.RunT(t)
	node := newRedisNode()

	_, err := node.Execute(context.Background(), workflow.NodeExecuteRequest{
		Node: workflow.Node{
			ID:    "redis_set",
			Type:  "redis",
			Label: "Redis SET",
			Config: map[string]any{
				"operation": "set",
				"addr":      redisServer.Addr(),
				"db":        0,
				"key":       "workflow:key:1",
				"value":     "value-1",
			},
		},
		Payload: map[string]any{},
		Scope:   map[string]any{},
	})
	if err != nil {
		t.Fatalf("redis set execute: %v", err)
	}

	getResult, err := node.Execute(context.Background(), workflow.NodeExecuteRequest{
		Node: workflow.Node{
			ID:    "redis_get",
			Type:  "redis",
			Label: "Redis GET",
			Config: map[string]any{
				"operation": "get",
				"addr":      redisServer.Addr(),
				"db":        0,
				"key":       "workflow:key:1",
			},
		},
		Payload: map[string]any{},
		Scope:   map[string]any{},
	})
	if err != nil {
		t.Fatalf("redis get execute: %v", err)
	}
	if getResult.NextLabel != "hit" {
		t.Fatalf("expected redis get hit, got %q", getResult.NextLabel)
	}
	if got := getResult.Output["redis_value"]; got != "value-1" {
		t.Fatalf("expected redis value-1, got %v", got)
	}
}

func TestPythonCodeNodeExecutesScript(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skipf("python3 is not available: %v", err)
	}
	node := newPythonCodeNode()
	result, err := node.Execute(context.Background(), workflow.NodeExecuteRequest{
		Node: workflow.Node{
			ID:    "py_1",
			Type:  "python_code",
			Label: "Python",
			Config: map[string]any{
				"code":      "result = {'ok': True, 'case': input.get('case_id')}\noutput['py_flag'] = True",
				"resultKey": "py_result",
			},
		},
		Input: map[string]any{
			"case_id": "CASE-100",
		},
		Payload: map[string]any{},
		Scope: map[string]any{
			"input": map[string]any{"case_id": "CASE-100"},
		},
	})
	if err != nil {
		t.Fatalf("python node execute: %v", err)
	}

	pyResult, ok := result.Output["py_result"].(map[string]any)
	if !ok {
		t.Fatalf("expected py_result object, got %T", result.Output["py_result"])
	}
	if got := pyResult["case"]; got != "CASE-100" {
		t.Fatalf("expected case CASE-100, got %v", got)
	}
	if flag, _ := result.Output["py_flag"].(bool); !flag {
		t.Fatalf("expected py_flag=true")
	}
}

func TestIOCExtractNodeFindsIndicators(t *testing.T) {
	node := newIOCExtractNode()
	result, err := node.Execute(context.Background(), workflow.NodeExecuteRequest{
		Node: workflow.Node{
			ID:   "ioc_1",
			Type: "ioc_extract",
			Config: map[string]any{
				"sourcePath": "payload.description",
			},
		},
		Payload: map[string]any{
			"description": "Suspicious download from https://evil.example.com by user@test.local on host 10.10.10.8 CVE-2024-3400",
		},
		Scope: map[string]any{
			"payload": map[string]any{
				"description": "Suspicious download from https://evil.example.com by user@test.local on host 10.10.10.8 CVE-2024-3400",
			},
		},
	})
	if err != nil {
		t.Fatalf("ioc_extract execute: %v", err)
	}
	if count := toInt(result.Output["ioc_count"], 0); count < 4 {
		t.Fatalf("expected at least 4 iocs, got %d", count)
	}
	items, ok := result.Output["iocs"].([]map[string]any)
	if !ok || len(items) == 0 {
		t.Fatalf("expected IOC list, got %T", result.Output["iocs"])
	}
}

func TestWatchlistMatchNodeReturnsHit(t *testing.T) {
	node := newWatchlistMatchNode()
	result, err := node.Execute(context.Background(), workflow.NodeExecuteRequest{
		Node: workflow.Node{
			ID:   "wl_1",
			Type: "watchlist_match",
			Config: map[string]any{
				"sourcePath": "payload.iocs",
				"watchlist":  `["evil.example.com","203.0.113.10"]`,
			},
		},
		Payload: map[string]any{
			"iocs": []any{
				map[string]any{"type": "domain", "value": "evil.example.com"},
				map[string]any{"type": "ip", "value": "198.51.100.2"},
			},
		},
		Scope: map[string]any{
			"payload": map[string]any{
				"iocs": []any{
					map[string]any{"type": "domain", "value": "evil.example.com"},
					map[string]any{"type": "ip", "value": "198.51.100.2"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("watchlist_match execute: %v", err)
	}
	if !toBool(result.Output["watchlist_hit"], false) {
		t.Fatalf("expected watchlist hit")
	}
	if result.NextLabel != "hit" {
		t.Fatalf("expected next label hit, got %q", result.NextLabel)
	}
}

func TestMITRERiskContainmentFlow(t *testing.T) {
	miter := newMITREMapNode()
	mitreRes, err := miter.Execute(context.Background(), workflow.NodeExecuteRequest{
		Node: workflow.Node{
			ID:   "miter_1",
			Type: "miter_map",
			Config: map[string]any{
				"sourcePath": "payload.description",
			},
		},
		Payload: map[string]any{
			"description": "PowerShell download observed with impossible travel and multiple failed login",
		},
		Scope: map[string]any{
			"payload": map[string]any{
				"description": "PowerShell download observed with impossible travel and multiple failed login",
			},
		},
	})
	if err != nil {
		t.Fatalf("miter_map execute: %v", err)
	}
	if toInt(mitreRes.Output["miter_match_count"], 0) == 0 {
		t.Fatalf("expected miter matches")
	}

	risk := newRiskScoreNode()
	riskRes, err := risk.Execute(context.Background(), workflow.NodeExecuteRequest{
		Node: workflow.Node{
			ID:   "risk_1",
			Type: "risk_score",
		},
		Payload: map[string]any{
			"severity":      "high",
			"confidence":    0.9,
			"ioc_count":     8,
			"watchlist_hit": true,
		},
		Scope: map[string]any{
			"payload": map[string]any{
				"severity":      "high",
				"confidence":    0.9,
				"ioc_count":     8,
				"watchlist_hit": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("risk_score execute: %v", err)
	}
	riskLevel := toString(riskRes.Output["risk_level"])
	if riskLevel == "" {
		t.Fatalf("expected risk level")
	}

	containment := newContainmentDecisionNode()
	containmentRes, err := containment.Execute(context.Background(), workflow.NodeExecuteRequest{
		Node: workflow.Node{
			ID:   "containment_1",
			Type: "containment_decision",
		},
		Payload: map[string]any{
			"risk_level":        riskLevel,
			"watchlist_hit":     true,
			"miter_matches":     mitreRes.Output["miter_matches"],
			"miter_primary":     mitreRes.Output["miter_primary"],
			"miter_match_count": mitreRes.Output["miter_match_count"],
		},
		Scope: map[string]any{
			"payload": map[string]any{
				"risk_level":    riskLevel,
				"watchlist_hit": true,
				"miter_matches": mitreRes.Output["miter_matches"],
			},
		},
	})
	if err != nil {
		t.Fatalf("containment_decision execute: %v", err)
	}
	if action := toString(containmentRes.Output["containment_action"]); action == "" {
		t.Fatalf("expected containment action")
	}
}
