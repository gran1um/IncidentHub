package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"incidenthub/backend/internal/workflow"
)

type postgresQueryNode struct {
	deps Dependencies
}

func newPostgresQueryNode(deps Dependencies) workflow.NodeExecutor {
	return postgresQueryNode{deps: deps}
}

func (n postgresQueryNode) Type() string {
	return "postgres_query"
}

func (n postgresQueryNode) Execute(ctx context.Context, req workflow.NodeExecuteRequest) (workflow.NodeExecuteResult, error) {
	if n.deps.System == nil {
		return workflow.NodeExecuteResult{}, fmt.Errorf("postgres repository is not configured")
	}
	sqlText := strings.TrimSpace(toString(req.Node.Config["query"]))
	if sqlText == "" {
		sqlText = strings.TrimSpace(toString(req.Node.Config["sql"]))
	}
	if sqlText == "" {
		return workflow.NodeExecuteResult{}, fmt.Errorf("sql query is required")
	}
	sqlText = workflow.RenderTemplate(sqlText, req.Scope)

	params := normalizeQueryParams(req.Node.Config["params"], req.Scope)
	limit := toInt(req.Node.Config["limit"], 100)
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	rows, err := n.deps.System.QueryReadOnly(ctx, sqlText, params, limit)
	if err != nil {
		return workflow.NodeExecuteResult{}, err
	}

	output := workflow.CopyMap(req.Payload)
	output["postgres_rows"] = rows
	output["postgres_row_count"] = len(rows)
	return workflow.NodeExecuteResult{
		Output: output,
	}, nil
}

func normalizeQueryParams(value any, scope map[string]any) []any {
	if value == nil {
		return nil
	}
	if text, ok := value.(string); ok {
		rendered := strings.TrimSpace(workflow.RenderTemplate(text, scope))
		if rendered == "" {
			return nil
		}
		raw := []any{}
		if err := json.Unmarshal([]byte(rendered), &raw); err != nil {
			return []any{rendered}
		}
		value = raw
	}
	raw := []any{}
	switch typed := value.(type) {
	case []any:
		raw = typed
	default:
		encoded, _ := json.Marshal(typed)
		_ = json.Unmarshal(encoded, &raw)
	}
	if len(raw) == 0 {
		return nil
	}
	out := make([]any, 0, len(raw))
	for _, item := range raw {
		if text, ok := item.(string); ok {
			text = workflow.RenderTemplate(text, scope)
			out = append(out, text)
			continue
		}
		out = append(out, item)
	}
	return out
}
