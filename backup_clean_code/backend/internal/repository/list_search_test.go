package repository

import (
	"strings"
	"testing"
)

func TestNormalizeListSearchParamsKeepsFulltextMode(t *testing.T) {
	normalized := normalizeListSearchParams(ListSearchParams{
		Query: "credential",
		Mode:  "fulltext",
		Logic: "any",
	})
	if normalized.Mode != "fulltext" {
		t.Fatalf("expected fulltext mode to be preserved, got %q", normalized.Mode)
	}
	if normalized.Logic != "any" {
		t.Fatalf("expected any logic, got %q", normalized.Logic)
	}
}

func TestBuildListSearchWhereClauseFulltext(t *testing.T) {
	clause, args := buildListSearchWhereClause(
		[]string{"title", "description"},
		ListSearchParams{
			Query: "credential OR dns",
			Mode:  "fulltext",
			Logic: "all",
		},
		3,
	)
	if !strings.Contains(clause, "websearch_to_tsquery('simple', $3)") {
		t.Fatalf("expected fulltext tsquery placeholder in clause, got %q", clause)
	}
	if !strings.Contains(clause, "to_tsvector('simple'") {
		t.Fatalf("expected tsvector expression in clause, got %q", clause)
	}
	if len(args) != 1 || args[0] != "credential OR dns" {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestBuildListSearchWhereClauseFulltextAnyLogic(t *testing.T) {
	clause, args := buildListSearchWhereClause(
		[]string{"title"},
		ListSearchParams{
			Query: "credential dns",
			Mode:  "fulltext",
			Logic: "any",
		},
		1,
	)
	if !strings.Contains(clause, "websearch_to_tsquery('simple', $1)") {
		t.Fatalf("expected tsquery placeholder, got %q", clause)
	}
	if len(args) != 1 {
		t.Fatalf("expected one arg, got %d", len(args))
	}
	if value := args[0]; value != "credential OR dns" {
		t.Fatalf("expected OR-expanded fulltext query, got %#v", value)
	}
}
