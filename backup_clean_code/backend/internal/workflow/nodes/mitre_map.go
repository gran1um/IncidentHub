package nodes

import (
	"context"
	"sort"
	"strings"

	"incidenthub/backend/internal/workflow"
)

type mitreSignature struct {
	TechniqueID string
	Technique   string
	Tactic      string
	Keywords    []string
}

//nolint:gochecknoglobals // Static ATT&CK keyword signatures used by workflow node.
var mitreSignatures = []mitreSignature{
	{TechniqueID: "T1059.001", Technique: "PowerShell", Tactic: "execution", Keywords: []string{"powershell", "pwsh", "script block"}},
	{TechniqueID: "T1105", Technique: "Ingress Tool Transfer", Tactic: "command-and-control", Keywords: []string{"download", "wget", "curl", "tool transfer"}},
	{TechniqueID: "T1566", Technique: "Phishing", Tactic: "initial-access", Keywords: []string{"phishing", "attachment", "malicious email"}},
	{TechniqueID: "T1078", Technique: "Valid Accounts", Tactic: "defense-evasion", Keywords: []string{"valid account", "credential reuse", "impossible travel"}},
	{TechniqueID: "T1110", Technique: "Brute Force", Tactic: "credential-access", Keywords: []string{"brute force", "password spray", "multiple failed login"}},
	{TechniqueID: "T1041", Technique: "Exfiltration Over C2 Channel", Tactic: "exfiltration", Keywords: []string{"exfiltration", "data leak", "large outbound"}},
	{TechniqueID: "T1486", Technique: "Data Encrypted for Impact", Tactic: "impact", Keywords: []string{"ransomware", "mass encryption", "encrypted for impact"}},
}

type mitreMapNode struct{}

func newMITREMapNode() workflow.NodeExecutor {
	return mitreMapNode{}
}

func (n mitreMapNode) Type() string {
	return "miter_map"
}

func (n mitreMapNode) Execute(_ context.Context, req workflow.NodeExecuteRequest) (workflow.NodeExecuteResult, error) {
	sourcePath := strings.TrimSpace(toString(req.Node.Config["sourcePath"]))
	if sourcePath == "" {
		sourcePath = "payload.description"
	}
	iocPath := strings.TrimSpace(toString(req.Node.Config["iocPath"]))
	if iocPath == "" {
		iocPath = "payload.iocs"
	}
	targetKey := strings.TrimSpace(toString(req.Node.Config["targetKey"]))
	if targetKey == "" {
		targetKey = "miter_matches"
	}

	text := strings.ToLower(normalizeIOCRawText(resolveNodePath(req.Scope, sourcePath)) + " " + strings.Join(extractListValues(resolveNodePath(req.Scope, iocPath), "value"), " "))

	matches := make([]map[string]any, 0)
	signatureSeen := map[string]struct{}{}
	for _, signature := range mitreSignatures {
		score := 0
		reasons := make([]string, 0)
		for _, keyword := range signature.Keywords {
			normalized := strings.TrimSpace(strings.ToLower(keyword))
			if normalized == "" {
				continue
			}
			if strings.Contains(text, normalized) {
				score++
				reasons = append(reasons, normalized)
			}
		}
		if score == 0 {
			continue
		}
		if _, exists := signatureSeen[signature.TechniqueID]; exists {
			continue
		}
		signatureSeen[signature.TechniqueID] = struct{}{}
		confidence := float64(score) / float64(maxInt(len(signature.Keywords), 1))
		if confidence > 1 {
			confidence = 1
		}
		matches = append(matches, map[string]any{
			"technique_id": signature.TechniqueID,
			"technique":    signature.Technique,
			"tactic":       signature.Tactic,
			"confidence":   confidence,
			"reasons":      reasons,
		})
	}
	sort.Slice(matches, func(i, j int) bool {
		return toString(matches[i]["technique_id"]) < toString(matches[j]["technique_id"])
	})

	output := workflow.CopyMap(req.Payload)
	output[targetKey] = matches
	output["miter_match_count"] = len(matches)
	if len(matches) > 0 {
		output["miter_primary"] = matches[0]
	}
	return workflow.NodeExecuteResult{
		Output:    output,
		NextLabel: ternaryLabel(len(matches) > 0, "matched", "no_match"),
	}, nil
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
