package nodes

import (
	"context"
	"encoding/json"
	"regexp"
	"sort"
	"strings"

	"incidenthub/backend/internal/workflow"
)

var (
	iocURLRe    = regexp.MustCompile(`(?i)\bhttps?://[^\s"'<>]+`)
	iocEmailRe  = regexp.MustCompile(`(?i)\b[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}\b`)
	iocIPv4Re   = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	iocDomainRe = regexp.MustCompile(`(?i)\b(?:[a-z0-9](?:[a-z0-9\-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}\b`)
	iocHashRe   = regexp.MustCompile(`(?i)\b(?:[a-f0-9]{32}|[a-f0-9]{40}|[a-f0-9]{64})\b`)
	iocCVERe    = regexp.MustCompile(`(?i)\bCVE-\d{4}-\d{4,7}\b`)
)

type iocExtractNode struct{}

func newIOCExtractNode() workflow.NodeExecutor {
	return iocExtractNode{}
}

func (n iocExtractNode) Type() string {
	return "ioc_extract"
}

func (n iocExtractNode) Execute(_ context.Context, req workflow.NodeExecuteRequest) (workflow.NodeExecuteResult, error) {
	sourcePath := strings.TrimSpace(toString(req.Node.Config["sourcePath"]))
	if sourcePath == "" {
		sourcePath = "payload.description"
	}
	sourceText := n.resolveSourceText(req, sourcePath)
	targetKey := strings.TrimSpace(toString(req.Node.Config["targetKey"]))
	if targetKey == "" {
		targetKey = "iocs"
	}

	collect := map[string]map[string]struct{}{}
	if toBool(req.Node.Config["includeURL"], true) {
		appendMatches(collect, "url", iocURLRe.FindAllString(sourceText, -1))
	}
	if toBool(req.Node.Config["includeEmail"], true) {
		appendMatches(collect, "email", iocEmailRe.FindAllString(sourceText, -1))
	}
	if toBool(req.Node.Config["includeIP"], true) {
		appendMatches(collect, "ip", iocIPv4Re.FindAllString(sourceText, -1))
	}
	if toBool(req.Node.Config["includeDomain"], true) {
		appendMatches(collect, "domain", iocDomainRe.FindAllString(sourceText, -1))
	}
	if toBool(req.Node.Config["includeHash"], true) {
		appendMatches(collect, "hash", iocHashRe.FindAllString(sourceText, -1))
	}
	if toBool(req.Node.Config["includeCVE"], true) {
		appendMatches(collect, "cve", iocCVERe.FindAllString(sourceText, -1))
	}

	output := workflow.CopyMap(req.Payload)
	iocs := flattenIOCs(collect)
	output[targetKey] = iocs
	output["ioc_count"] = len(iocs)
	output["ioc_by_type"] = mapIOCByType(collect)
	output["ioc_extract_source"] = sourcePath
	return workflow.NodeExecuteResult{Output: output}, nil
}

func (n iocExtractNode) resolveSourceText(req workflow.NodeExecuteRequest, sourcePath string) string {
	if value, ok := workflow.LookupPath(req.Scope, sourcePath); ok {
		return normalizeIOCRawText(value)
	}
	if rawTemplate := strings.TrimSpace(toString(req.Node.Config["sourceText"])); rawTemplate != "" {
		return workflow.RenderTemplate(rawTemplate, req.Scope)
	}
	return normalizeIOCRawText(req.Payload)
}

func appendMatches(target map[string]map[string]struct{}, iocType string, values []string) {
	if target[iocType] == nil {
		target[iocType] = map[string]struct{}{}
	}
	for _, item := range values {
		normalized := strings.TrimSpace(strings.Trim(item, ".,;:!?)("))
		if normalized == "" {
			continue
		}
		target[iocType][normalized] = struct{}{}
	}
}

func flattenIOCs(collect map[string]map[string]struct{}) []map[string]any {
	types := make([]string, 0, len(collect))
	for iocType := range collect {
		types = append(types, iocType)
	}
	sort.Strings(types)
	items := make([]map[string]any, 0)
	for _, iocType := range types {
		values := make([]string, 0, len(collect[iocType]))
		for value := range collect[iocType] {
			values = append(values, value)
		}
		sort.Strings(values)
		for _, value := range values {
			items = append(items, map[string]any{
				"type":  iocType,
				"value": value,
			})
		}
	}
	return items
}

func mapIOCByType(collect map[string]map[string]struct{}) map[string][]string {
	result := make(map[string][]string, len(collect))
	for iocType, values := range collect {
		out := make([]string, 0, len(values))
		for value := range values {
			out = append(out, value)
		}
		sort.Strings(out)
		result[iocType] = out
	}
	return result
}

func normalizeIOCRawText(input any) string {
	switch typed := input.(type) {
	case nil:
		return ""
	case string:
		return typed
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return toString(input)
		}
		return string(encoded)
	}
}
