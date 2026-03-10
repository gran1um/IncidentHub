package api

import "testing"

func TestNormalizeCaseStatusCode(t *testing.T) {
	cases := map[string]string{
		" Open ":            "open",
		"In Progress":       "in_progress",
		"False-Positive!!":  "false-positive",
		"major vuln":        "major_vuln",
		"***":               "",
		"  ":                "",
		"RESOLVED__STATUS ": "resolved__status",
	}

	for in, expected := range cases {
		got := normalizeCaseStatusCode(in)
		if got != expected {
			t.Fatalf("normalizeCaseStatusCode(%q) = %q, expected %q", in, got, expected)
		}
	}
}

func TestHumanizeCaseStatusCode(t *testing.T) {
	if got := humanizeCaseStatusCode("major_vuln"); got != "Major Vuln" {
		t.Fatalf("unexpected humanized code: %q", got)
	}
	if got := humanizeCaseStatusCode(""); got != "Status" {
		t.Fatalf("expected fallback label for empty code, got %q", got)
	}
}

func TestDefaultCaseStatusesHasOpenState(t *testing.T) {
	items := defaultCaseStatuses()
	if len(items) == 0 {
		t.Fatalf("default statuses must not be empty")
	}
	hasOpen := false
	for _, item := range items {
		if !item.IsClosed {
			hasOpen = true
			break
		}
	}
	if !hasOpen {
		t.Fatalf("default statuses must include at least one open status")
	}
}

func TestDefaultCaseStatusesIncludeSOARLifecycleStates(t *testing.T) {
	items := defaultCaseStatuses()
	required := map[string]bool{
		"analysis":                 false,
		"response":                 false,
		"post_incident":            false,
		"infrastructure_hardening": false,
		"review":                   false,
	}
	for _, item := range items {
		if _, ok := required[item.Code]; ok {
			required[item.Code] = true
		}
	}
	for code, present := range required {
		if !present {
			t.Fatalf("default statuses must include %q", code)
		}
	}
}
