package nodes

import (
	"context"
	"strings"

	"incidenthub/backend/internal/workflow"
)

type containmentDecisionNode struct{}

func newContainmentDecisionNode() workflow.NodeExecutor {
	return containmentDecisionNode{}
}

func (n containmentDecisionNode) Type() string {
	return "containment_decision"
}

func (n containmentDecisionNode) Execute(_ context.Context, req workflow.NodeExecuteRequest) (workflow.NodeExecuteResult, error) {
	riskLevelPath := strings.TrimSpace(toString(req.Node.Config["riskLevelPath"]))
	if riskLevelPath == "" {
		riskLevelPath = "payload.risk_level"
	}
	watchlistPath := strings.TrimSpace(toString(req.Node.Config["watchlistHitPath"]))
	if watchlistPath == "" {
		watchlistPath = "payload.watchlist_hit"
	}
	mitrePath := strings.TrimSpace(toString(req.Node.Config["mitrePath"]))
	if mitrePath == "" {
		mitrePath = "payload.mitre_matches"
	}

	riskLevel := normalizeLabel(toString(resolveNodePath(req.Scope, riskLevelPath)))
	watchlistHit := toBool(resolveNodePath(req.Scope, watchlistPath), false)
	mitreIDs := extractMITRETechniqueIDs(resolveNodePath(req.Scope, mitrePath))

	action := "monitor"
	reason := "No high-confidence containment signal found."
	playbook := []string{
		"Continue monitoring and collect additional telemetry",
		"Notify duty analyst for manual verification",
	}

	switch {
	case riskLevel == "critical" && watchlistHit:
		action = "isolate_host"
		reason = "Critical risk with watchlist hit."
		playbook = []string{
			"Isolate affected endpoint from network",
			"Block observed IOC at perimeter controls",
			"Open P1 incident bridge",
		}
	case riskLevel == "high" && containsAny(mitreIDs, "T1110", "T1078"):
		action = "force_password_reset"
		reason = "Credential abuse technique detected."
		playbook = []string{
			"Force reset impacted user credentials",
			"Invalidate active sessions and refresh tokens",
			"Enable MFA challenge for suspicious accounts",
		}
	case containsAny(mitreIDs, "T1566"):
		action = "quarantine_email"
		reason = "Phishing-related behavior detected."
		playbook = []string{
			"Quarantine related emails/messages",
			"Block sender/domain/IP indicators",
			"Notify affected recipients",
		}
	case containsAny(mitreIDs, "T1486"):
		action = "isolate_host"
		reason = "Ransomware impact indicators detected."
		playbook = []string{
			"Isolate potentially impacted systems",
			"Disable lateral movement channels",
			"Start backup recovery validation",
		}
	}

	output := workflow.CopyMap(req.Payload)
	output["containment_action"] = action
	output["containment_reason"] = reason
	output["containment_playbook"] = playbook
	return workflow.NodeExecuteResult{
		Output:    output,
		NextLabel: action,
	}, nil
}

func extractMITRETechniqueIDs(raw any) []string {
	values := make([]string, 0)
	switch typed := raw.(type) {
	case []any:
		for _, item := range typed {
			if entry, ok := item.(map[string]any); ok {
				id := strings.ToUpper(strings.TrimSpace(toString(entry["technique_id"])))
				if id != "" {
					values = append(values, id)
				}
			}
		}
	case []map[string]any:
		for _, entry := range typed {
			id := strings.ToUpper(strings.TrimSpace(toString(entry["technique_id"])))
			if id != "" {
				values = append(values, id)
			}
		}
	}
	return dedupeStrings(values)
}

func containsAny(values []string, targets ...string) bool {
	if len(values) == 0 || len(targets) == 0 {
		return false
	}
	set := map[string]struct{}{}
	for _, value := range values {
		normalized := strings.ToUpper(strings.TrimSpace(value))
		if normalized != "" {
			set[normalized] = struct{}{}
		}
	}
	for _, target := range targets {
		if _, ok := set[strings.ToUpper(strings.TrimSpace(target))]; ok {
			return true
		}
	}
	return false
}
