package nodes

import (
	"context"
	"strings"

	"incidenthub/backend/internal/workflow"
)

type riskScoreNode struct{}

func newRiskScoreNode() workflow.NodeExecutor {
	return riskScoreNode{}
}

func (n riskScoreNode) Type() string {
	return "risk_score"
}

func (n riskScoreNode) Execute(_ context.Context, req workflow.NodeExecuteRequest) (workflow.NodeExecuteResult, error) {
	severityPath := strings.TrimSpace(toString(req.Node.Config["severityPath"]))
	if severityPath == "" {
		severityPath = "payload.severity"
	}
	confidencePath := strings.TrimSpace(toString(req.Node.Config["confidencePath"]))
	if confidencePath == "" {
		confidencePath = "payload.confidence"
	}
	iocCountPath := strings.TrimSpace(toString(req.Node.Config["iocCountPath"]))
	if iocCountPath == "" {
		iocCountPath = "payload.ioc_count"
	}
	watchlistPath := strings.TrimSpace(toString(req.Node.Config["watchlistHitPath"]))
	if watchlistPath == "" {
		watchlistPath = "payload.watchlist_hit"
	}

	severity := normalizeLabel(toString(resolveNodePath(req.Scope, severityPath)))
	confidenceValue, _ := toFloat(resolveNodePath(req.Scope, confidencePath))
	if confidenceValue > 1 {
		confidenceValue /= 100
	}
	if confidenceValue < 0 {
		confidenceValue = 0
	}
	if confidenceValue > 1 {
		confidenceValue = 1
	}
	iocCount := toInt(resolveNodePath(req.Scope, iocCountPath), 0)
	watchlistHit := toBool(resolveNodePath(req.Scope, watchlistPath), false)

	baseWeight := map[string]float64{
		"critical": 55,
		"high":     40,
		"medium":   25,
		"low":      10,
	}
	score := baseWeight[severity]
	score += confidenceValue * 30
	score += minFloat(float64(iocCount), 20) * 1.2
	if watchlistHit {
		score += 12
	}
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	riskLevel := "low"
	switch {
	case score >= 85:
		riskLevel = "critical"
	case score >= 65:
		riskLevel = "high"
	case score >= 40:
		riskLevel = "medium"
	}

	recommendedPriority := "p4"
	switch riskLevel {
	case "critical":
		recommendedPriority = "p1"
	case "high":
		recommendedPriority = "p2"
	case "medium":
		recommendedPriority = "p3"
	}

	output := workflow.CopyMap(req.Payload)
	output["risk_score"] = score
	output["risk_level"] = riskLevel
	output["risk_priority"] = recommendedPriority
	output["risk_inputs"] = map[string]any{
		"severity":      severity,
		"confidence":    confidenceValue,
		"ioc_count":     iocCount,
		"watchlist_hit": watchlistHit,
	}
	return workflow.NodeExecuteResult{
		Output:    output,
		NextLabel: riskLevel,
	}, nil
}

func minFloat(left, right float64) float64 {
	if left < right {
		return left
	}
	return right
}
