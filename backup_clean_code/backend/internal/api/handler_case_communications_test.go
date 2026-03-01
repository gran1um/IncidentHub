package api

import (
	"testing"
	"time"

	"incidenthub/backend/internal/models"

	"github.com/google/uuid"
)

func TestResolveCommunicationConnectorID(t *testing.T) {
	connectorID := uuid.New()
	thread := models.CatalogItem{
		Data: map[string]any{
			"connector_id": connectorID.String(),
		},
	}

	t.Run("uses request value", func(t *testing.T) {
		other := uuid.New()
		got, err := resolveCommunicationConnectorID(other.String(), thread)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != other {
			t.Fatalf("expected %s, got %s", other.String(), got.String())
		}
	})

	t.Run("falls back to thread connector", func(t *testing.T) {
		got, err := resolveCommunicationConnectorID("", thread)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != connectorID {
			t.Fatalf("expected %s, got %s", connectorID.String(), got.String())
		}
	})

	t.Run("fails without valid connector id", func(t *testing.T) {
		if _, err := resolveCommunicationConnectorID("", models.CatalogItem{}); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestMergeCommunicationMaps(t *testing.T) {
	merged := mergeCommunicationMaps(
		map[string]any{"a": 1, "b": "base"},
		map[string]any{"b": "patch", "c": true},
	)

	if merged["a"] != 1 {
		t.Fatalf("expected key a")
	}
	if merged["b"] != "patch" {
		t.Fatalf("expected patch override, got %v", merged["b"])
	}
	if merged["c"] != true {
		t.Fatalf("expected key c")
	}
}

func TestCommunicationPreview(t *testing.T) {
	input := "   one   two\nthree\tfour   "
	if got := communicationPreview(input); got != "one two three four" {
		t.Fatalf("unexpected normalized preview: %q", got)
	}

	longInput := ""
	for i := 0; i < 220; i++ {
		longInput += "a"
	}
	got := communicationPreview(longInput)
	if len([]rune(got)) != 183 {
		t.Fatalf("expected truncated preview length 183, got %d", len([]rune(got)))
	}
}

func TestParseCommunicationTimestamp(t *testing.T) {
	now := time.Now().UTC().Add(-time.Hour)
	parsed := parseCommunicationTimestamp("2026-01-02T03:04:05Z", now)
	if parsed.Format(time.RFC3339) != "2026-01-02T03:04:05Z" {
		t.Fatalf("unexpected parsed timestamp: %s", parsed.Format(time.RFC3339))
	}

	fallback := parseCommunicationTimestamp("invalid", now)
	if !fallback.Equal(now) {
		t.Fatalf("expected fallback %s, got %s", now.Format(time.RFC3339), fallback.Format(time.RFC3339))
	}
}
