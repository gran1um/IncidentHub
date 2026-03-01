package api

import (
	"strings"
	"testing"
	"time"

	"incidenthub/backend/internal/models"

	"github.com/google/uuid"
)

func TestParseUniqueAlertIDs(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()

	out, err := parseUniqueAlertIDs([]string{
		"  " + id1.String() + "  ",
		id2.String(),
		id1.String(),
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 unique ids, got %d", len(out))
	}
	if out[0] != id1 || out[1] != id2 {
		t.Fatalf("unexpected ids order: %v", out)
	}
}

func TestParseUniqueAlertIDsRejectsInvalidID(t *testing.T) {
	_, err := parseUniqueAlertIDs([]string{"not-a-uuid"})
	if err == nil {
		t.Fatalf("expected validation error for invalid uuid")
	}
}

func TestBuildAlertSummaryDescriptionOrdersByUpdatedAtDesc(t *testing.T) {
	idNewest := uuid.New()
	idOldest := uuid.New()
	now := time.Now().UTC()

	summary := buildAlertSummaryDescription([]models.Alert{
		{
			ID:        idOldest,
			Title:     "Old",
			Source:    "siem",
			UpdatedAt: now.Add(-2 * time.Hour),
		},
		{
			ID:        idNewest,
			Title:     "New",
			Source:    "edr",
			UpdatedAt: now,
		},
	})

	lines := strings.Split(summary, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines in summary, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "Created from 2 selected alert(s).") {
		t.Fatalf("unexpected summary header: %s", lines[0])
	}
	if !strings.Contains(lines[1], idNewest.String()) {
		t.Fatalf("expected newest alert to appear first, got line: %s", lines[1])
	}
}
