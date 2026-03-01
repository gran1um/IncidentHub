package repository

import (
	"fmt"
	"strings"
)

type ListSearchParams struct {
	Query           string
	Exclude         string
	Mode            string
	Logic           string
	CaseMetaTagsAny []string
}

func normalizeListSearchParams(params ListSearchParams) ListSearchParams {
	mode := strings.ToLower(strings.TrimSpace(params.Mode))
	if mode != "regex" && mode != "fulltext" {
		mode = "plain"
	}
	logic := strings.ToLower(strings.TrimSpace(params.Logic))
	if logic != "any" {
		logic = "all"
	}
	return ListSearchParams{
		Query:           strings.TrimSpace(params.Query),
		Exclude:         strings.TrimSpace(params.Exclude),
		Mode:            mode,
		Logic:           logic,
		CaseMetaTagsAny: normalizeListSearchTags(params.CaseMetaTagsAny),
	}
}

func normalizeListSearchTags(input []string) []string {
	if len(input) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(input))
	out := make([]string, 0, len(input))
	for _, raw := range input {
		tag := strings.ToLower(strings.TrimSpace(raw))
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func buildListSearchWhereClause(fields []string, params ListSearchParams, startPos int) (clause string, clauseArgs []any) {
	normalized := normalizeListSearchParams(params)
	if len(fields) == 0 {
		return "", nil
	}
	nextPos := startPos
	clauses := make([]string, 0, 2)
	args := make([]any, 0, 4)

	inclusion, inclusionArgs := buildListSearchExpression(fields, normalized.Query, normalized.Mode, normalized.Logic, nextPos)
	if inclusion != "" {
		clauses = append(clauses, inclusion)
		args = append(args, inclusionArgs...)
		nextPos += len(inclusionArgs)
	}

	exclusion, exclusionArgs := buildListSearchExpression(fields, normalized.Exclude, normalized.Mode, normalized.Logic, nextPos)
	if exclusion != "" {
		clauses = append(clauses, "NOT "+exclusion)
		args = append(args, exclusionArgs...)
	}

	if len(clauses) == 0 {
		return "", nil
	}
	return " AND " + strings.Join(clauses, " AND "), args
}

func buildListSearchExpression(fields []string, raw, mode, logic string, startPos int) (expression string, expressionArgs []any) {
	query := strings.TrimSpace(raw)
	if query == "" {
		return "", nil
	}
	switch mode {
	case "regex":
		args := []any{query}
		parts := make([]string, 0, len(fields))
		for _, field := range fields {
			parts = append(parts, fmt.Sprintf("%s ~* $%d", field, startPos))
		}
		return "(" + strings.Join(parts, " OR ") + ")", args
	case "fulltext":
		return buildFulltextListSearchExpression(fields, query, logic, startPos)
	default:
		return buildPlainListSearchExpression(fields, query, logic, startPos)
	}
}

func buildFulltextListSearchExpression(fields []string, query, logic string, startPos int) (expression string, expressionArgs []any) {
	searchQuery := strings.TrimSpace(query)
	if searchQuery == "" {
		return "", nil
	}
	if logic == "any" {
		tokens := strings.Fields(searchQuery)
		if len(tokens) > 1 {
			searchQuery = strings.Join(tokens, " OR ")
		}
	}
	args := []any{searchQuery}
	return fmt.Sprintf("(%s @@ websearch_to_tsquery('simple', $%d))", buildFulltextVectorExpression(fields), startPos), args
}

func buildFulltextVectorExpression(fields []string) string {
	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		parts = append(parts, fmt.Sprintf("COALESCE((%s)::text, '')", field))
	}
	return fmt.Sprintf("to_tsvector('simple', concat_ws(' ', %s))", strings.Join(parts, ", "))
}

func buildPlainListSearchExpression(fields []string, query, logic string, startPos int) (expression string, expressionArgs []any) {
	tokens := strings.Fields(query)
	if len(tokens) == 0 {
		return "", nil
	}
	nextPos := startPos
	args := make([]any, 0, len(tokens))
	if logic == "any" {
		parts := make([]string, 0, len(tokens)*len(fields))
		for _, token := range tokens {
			pattern := "%" + token + "%"
			args = append(args, pattern)
			for _, field := range fields {
				parts = append(parts, fmt.Sprintf("%s ILIKE $%d", field, nextPos))
			}
			nextPos++
		}
		return "(" + strings.Join(parts, " OR ") + ")", args
	}

	tokenGroups := make([]string, 0, len(tokens))
	for _, token := range tokens {
		pattern := "%" + token + "%"
		args = append(args, pattern)
		fieldParts := make([]string, 0, len(fields))
		for _, field := range fields {
			fieldParts = append(fieldParts, fmt.Sprintf("%s ILIKE $%d", field, nextPos))
		}
		tokenGroups = append(tokenGroups, "("+strings.Join(fieldParts, " OR ")+")")
		nextPos++
	}
	return "(" + strings.Join(tokenGroups, " AND ") + ")", args
}
