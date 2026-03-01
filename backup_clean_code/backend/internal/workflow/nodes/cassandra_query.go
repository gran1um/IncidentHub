package nodes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"incidenthub/backend/internal/workflow"
)

type cassandraQueryNode struct{}

func newCassandraQueryNode() workflow.NodeExecutor {
	return cassandraQueryNode{}
}

func (n cassandraQueryNode) Type() string {
	return "cassandra_query"
}

func (n cassandraQueryNode) Execute(ctx context.Context, req workflow.NodeExecuteRequest) (workflow.NodeExecuteResult, error) {
	endpoint := strings.TrimSpace(workflow.RenderTemplate(toString(req.Node.Config["endpoint"]), req.Scope))
	if endpoint == "" {
		endpoint = strings.TrimSpace(workflow.RenderTemplate(toString(req.Node.Config["url"]), req.Scope))
	}
	if endpoint == "" {
		return workflow.NodeExecuteResult{}, fmt.Errorf("cassandra endpoint is required")
	}

	query := strings.TrimSpace(toString(req.Node.Config["query"]))
	if query == "" {
		query = strings.TrimSpace(toString(req.Node.Config["cql"]))
	}
	if query == "" {
		return workflow.NodeExecuteResult{}, fmt.Errorf("cassandra CQL query is required")
	}
	query = workflow.RenderTemplate(query, req.Scope)

	params := normalizeQueryParams(req.Node.Config["params"], req.Scope)
	payload := map[string]any{
		"query":  query,
		"values": params,
	}
	if keyspace := strings.TrimSpace(workflow.RenderTemplate(toString(req.Node.Config["keyspace"]), req.Scope)); keyspace != "" {
		payload["keyspace"] = keyspace
	}
	if consistency := strings.TrimSpace(workflow.RenderTemplate(toString(req.Node.Config["consistency"]), req.Scope)); consistency != "" {
		payload["consistency"] = consistency
	}
	if pageSize := toInt(req.Node.Config["pageSize"], 0); pageSize > 0 {
		payload["page_size"] = pageSize
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return workflow.NodeExecuteResult{}, fmt.Errorf("encode cassandra request: %w", err)
	}

	timeoutSeconds := toInt(req.Node.Config["timeoutSeconds"], 15)
	if timeoutSeconds <= 0 {
		timeoutSeconds = 15
	}
	if timeoutSeconds > 120 {
		timeoutSeconds = 120
	}
	httpClient := &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return workflow.NodeExecuteResult{}, fmt.Errorf("build cassandra request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	for key, value := range parseRawHeaders(req.Node.Config["headers"], req.Scope) {
		httpReq.Header.Set(key, value)
	}
	if token := strings.TrimSpace(workflow.RenderTemplate(toString(req.Node.Config["token"]), req.Scope)); token != "" &&
		httpReq.Header.Get("X-Cassandra-Token") == "" {
		httpReq.Header.Set("X-Cassandra-Token", token)
	}
	if username := strings.TrimSpace(workflow.RenderTemplate(toString(req.Node.Config["username"]), req.Scope)); username != "" {
		httpReq.SetBasicAuth(username, strings.TrimSpace(workflow.RenderTemplate(toString(req.Node.Config["password"]), req.Scope)))
	}

	response, err := httpClient.Do(httpReq)
	if err != nil {
		return workflow.NodeExecuteResult{}, fmt.Errorf("execute cassandra request: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	rawBody, err := io.ReadAll(io.LimitReader(response.Body, 4*1024*1024))
	if err != nil {
		return workflow.NodeExecuteResult{}, fmt.Errorf("read cassandra response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return workflow.NodeExecuteResult{}, fmt.Errorf("cassandra query failed with status %s: %s", response.Status, strings.TrimSpace(string(rawBody)))
	}

	parsed := map[string]any{}
	if len(rawBody) > 0 {
		if err := json.Unmarshal(rawBody, &parsed); err != nil {
			parsed["raw"] = strings.TrimSpace(string(rawBody))
		}
	}

	rows := extractCassandraRows(parsed)
	output := workflow.CopyMap(req.Payload)
	output["cassandra_response"] = parsed
	output["cassandra_rows"] = rows
	output["cassandra_row_count"] = len(rows)
	return workflow.NodeExecuteResult{
		Output: output,
	}, nil
}

func parseRawHeaders(raw any, scope map[string]any) map[string]string {
	out := map[string]string{}
	switch typed := raw.(type) {
	case map[string]any:
		for key, value := range typed {
			name := strings.TrimSpace(key)
			if name == "" {
				continue
			}
			out[name] = workflow.RenderTemplate(toString(value), scope)
		}
	case string:
		rendered := strings.TrimSpace(workflow.RenderTemplate(typed, scope))
		if rendered == "" {
			return out
		}
		decoded := map[string]any{}
		if err := json.Unmarshal([]byte(rendered), &decoded); err != nil {
			return out
		}
		for key, value := range decoded {
			name := strings.TrimSpace(key)
			if name == "" {
				continue
			}
			out[name] = toString(value)
		}
	}
	return out
}

func extractCassandraRows(payload map[string]any) []map[string]any {
	if payload == nil {
		return nil
	}
	candidates := []string{"rows", "data", "result"}
	for _, key := range candidates {
		value, exists := payload[key]
		if !exists {
			continue
		}
		rows := normalizeRows(value)
		if len(rows) > 0 {
			return rows
		}
	}
	return nil
}

func normalizeRows(raw any) []map[string]any {
	switch typed := raw.(type) {
	case []map[string]any:
		return typed
	case []any:
		out := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if row, ok := item.(map[string]any); ok {
				out = append(out, row)
			}
		}
		return out
	case map[string]any:
		if rows, ok := typed["rows"]; ok {
			return normalizeRows(rows)
		}
		return []map[string]any{typed}
	default:
		return nil
	}
}
