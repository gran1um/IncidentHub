package api

import (
	"encoding/json"
	"testing"
	"time"

	"incidenthub/backend/internal/models"

	"github.com/google/uuid"
)

func TestBoolFromMapVariants(t *testing.T) {
	tests := []struct {
		name     string
		payload  map[string]any
		expected bool
		ok       bool
	}{
		{name: "bool true", payload: map[string]any{"is_closed": true}, expected: true, ok: true},
		{name: "string yes", payload: map[string]any{"is_closed": "yes"}, expected: true, ok: true},
		{name: "string zero", payload: map[string]any{"is_closed": "0"}, expected: false, ok: true},
		{name: "json number", payload: map[string]any{"is_closed": json.Number("1")}, expected: true, ok: true},
		{name: "float", payload: map[string]any{"is_closed": float64(0)}, expected: false, ok: true},
		{name: "missing", payload: map[string]any{}, expected: false, ok: false},
	}
	for _, tc := range tests {
		got, ok := boolFromMap(tc.payload, "is_closed")
		if got != tc.expected || ok != tc.ok {
			t.Fatalf("%s: got (%v,%v) expected (%v,%v)", tc.name, got, ok, tc.expected, tc.ok)
		}
	}
}

func TestIntFromMapVariants(t *testing.T) {
	tests := []struct {
		name     string
		payload  map[string]any
		expected int
		ok       bool
	}{
		{name: "int", payload: map[string]any{"order": 10}, expected: 10, ok: true},
		{name: "int64", payload: map[string]any{"order": int64(11)}, expected: 11, ok: true},
		{name: "float", payload: map[string]any{"order": float64(12)}, expected: 12, ok: true},
		{name: "json number", payload: map[string]any{"order": json.Number("13")}, expected: 13, ok: true},
		{name: "string", payload: map[string]any{"order": "14"}, expected: 14, ok: true},
		{name: "missing", payload: map[string]any{}, expected: 0, ok: false},
	}
	for _, tc := range tests {
		got, ok := intFromMap(tc.payload, "order")
		if got != tc.expected || ok != tc.ok {
			t.Fatalf("%s: got (%d,%v) expected (%d,%v)", tc.name, got, ok, tc.expected, tc.ok)
		}
	}
}

func TestNormalizeOptionalRFC3339(t *testing.T) {
	normalized, err := normalizeOptionalRFC3339("2026-02-15T18:00:00+03:00")
	if err != nil {
		t.Fatalf("normalizeOptionalRFC3339 valid: %v", err)
	}
	if normalized == nil || *normalized != "2026-02-15T15:00:00Z" {
		t.Fatalf("unexpected normalized time: %v", normalized)
	}

	if _, err := normalizeOptionalRFC3339("invalid"); err == nil {
		t.Fatalf("expected normalizeOptionalRFC3339 parse error")
	}
}

func TestVerdictToCaseMeta(t *testing.T) {
	if verdictToCaseMeta("malicious") != "Malicious" {
		t.Fatalf("unexpected verdict mapping")
	}
	if verdictToCaseMeta("SUSPICIOUS") != "Suspicious" {
		t.Fatalf("unexpected verdict mapping")
	}
	if verdictToCaseMeta("benign") != "Benign" {
		t.Fatalf("unexpected verdict mapping")
	}
	if verdictToCaseMeta("other") != "Unknown" {
		t.Fatalf("unexpected verdict mapping")
	}
}

func TestInboundConnectorRunToPayloadAndConnectorDirection(t *testing.T) {
	now := time.Now().UTC()
	finished := now.Add(2 * time.Minute)
	run := models.InboundConnectorRun{
		ID:                uuid.New(),
		TenantID:          uuid.New(),
		ConnectorID:       uuid.New(),
		Trigger:           "manual",
		StartedAt:         now,
		FinishedAt:        &finished,
		Status:            "success",
		RecordsSeen:       10,
		AlertsCreated:     3,
		DuplicatesSkipped: 1,
		Errors:            0,
		Message:           "ok",
		Logs:              "logs",
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	payload := inboundConnectorRunToPayload(run)
	if payload["id"] == "" || payload["status"] != "success" {
		t.Fatalf("unexpected inbound run payload: %#v", payload)
	}

	if got := connectorDirection(map[string]any{"direction": "inbound"}); got != "inbound" {
		t.Fatalf("unexpected connector direction: %s", got)
	}
	if got := connectorDirection(map[string]any{"direction": "unknown"}); got != "outbound" {
		t.Fatalf("unexpected fallback connector direction: %s", got)
	}
}

func TestMergeCatalogData(t *testing.T) {
	merged := mergeCatalogData(
		map[string]any{"a": 1, "b": "base"},
		map[string]any{"b": "override", "c": true},
	)
	if merged["a"] != 1 || merged["b"] != "override" || merged["c"] != true {
		t.Fatalf("unexpected merged catalog data: %#v", merged)
	}
}

func TestSortCatalogItemsChronologically(t *testing.T) {
	firstID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	secondID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	thirdID := uuid.MustParse("00000000-0000-0000-0000-000000000003")

	items := []models.CatalogItem{
		{
			ID:        thirdID,
			Data:      map[string]any{"timestamp": "2026-02-20T12:00:00Z"},
			CreatedAt: time.Date(2026, time.February, 20, 12, 0, 0, 0, time.UTC),
		},
		{
			ID:        secondID,
			Data:      map[string]any{"timestamp": "2026-02-20T10:00:00Z"},
			CreatedAt: time.Date(2026, time.February, 20, 10, 0, 0, 0, time.UTC),
		},
		{
			ID:        firstID,
			Data:      map[string]any{"timestamp": "2026-02-20T10:00:00Z"},
			CreatedAt: time.Date(2026, time.February, 20, 10, 0, 0, 0, time.UTC),
		},
	}

	sortCatalogItemsChronologically(items, "timestamp", "created_at")

	if items[0].ID != firstID || items[1].ID != secondID || items[2].ID != thirdID {
		t.Fatalf("unexpected chronological order: [%s %s %s]", items[0].ID, items[1].ID, items[2].ID)
	}
}

func TestCatalogItemChronologyTimeFallback(t *testing.T) {
	fallback := time.Date(2026, time.February, 20, 9, 30, 0, 0, time.UTC)
	item := models.CatalogItem{
		ID:        uuid.New(),
		Data:      map[string]any{"timestamp": "not-a-date"},
		CreatedAt: fallback,
	}

	got := catalogItemChronologyTime(item, "timestamp", "created_at")
	if !got.Equal(fallback) {
		t.Fatalf("expected fallback time %s, got %s", fallback.Format(time.RFC3339), got.Format(time.RFC3339))
	}
}
