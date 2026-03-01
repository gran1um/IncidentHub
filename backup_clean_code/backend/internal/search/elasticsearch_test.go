package search

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"incidenthub/backend/internal/config"
)

func TestSearchFiltersByTenant(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST search request, got %s", r.Method)
		}
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"hits": map[string]any{
				"hits": []map[string]any{
					{
						"_id":    "alert-1",
						"_index": "incidenthub-alerts",
						"_score": 1.0,
						"_source": map[string]any{
							"tenant_id": "tenant-a",
							"title":     "Probe A",
						},
					},
					{
						"_id":    "alert-2",
						"_index": "incidenthub-alerts",
						"_score": 0.8,
						"_source": map[string]any{
							"tenant_id": "tenant-b",
							"title":     "Probe B",
						},
					},
				},
			},
		})
	}))
	defer testServer.Close()

	client, err := New(config.ElasticConfig{
		Enabled:     true,
		Addresses:   testServer.URL,
		IndexPrefix: "incidenthub",
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	hits, err := client.Search(context.Background(), "tenant-a", "probe", []string{"alerts"}, 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit after tenant filtering, got %d", len(hits))
	}
	if hits[0].ID != "alert-1" {
		t.Fatalf("expected alert-1, got %s", hits[0].ID)
	}
	if hits[0].Kind != "alerts" {
		t.Fatalf("expected kind alerts, got %s", hits[0].Kind)
	}
}

func TestSearchDisabled(t *testing.T) {
	client, err := New(config.ElasticConfig{Enabled: false})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	hits, err := client.Search(context.Background(), "", "probe", []string{"alerts"}, 5)
	if err != nil {
		t.Fatalf("search on disabled client: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("expected zero hits for disabled client, got %d", len(hits))
	}
}

func TestSearchUsesAndOperatorAndPrefix(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawBody, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		var payload map[string]any
		if err := json.Unmarshal(rawBody, &payload); err != nil {
			t.Fatalf("decode request payload: %v", err)
		}

		queryObj, ok := payload["query"].(map[string]any)
		if !ok {
			t.Fatalf("missing query object: %#v", payload["query"])
		}
		boolObj, ok := queryObj["bool"].(map[string]any)
		if !ok {
			t.Fatalf("expected bool query: %#v", queryObj)
		}
		must, ok := boolObj["must"].([]any)
		if !ok || len(must) == 0 {
			t.Fatalf("expected must clauses: %#v", boolObj["must"])
		}
		firstMust, ok := must[0].(map[string]any)
		if !ok {
			t.Fatalf("expected must clause map: %#v", must[0])
		}
		multiMatch, ok := firstMust["multi_match"].(map[string]any)
		if !ok {
			t.Fatalf("expected multi_match in must clause: %#v", firstMust)
		}
		if multiMatch["operator"] != "and" {
			t.Fatalf("expected operator=and, got %#v", multiMatch["operator"])
		}

		should, ok := boolObj["should"].([]any)
		if !ok || len(should) == 0 {
			t.Fatalf("expected should clauses: %#v", boolObj["should"])
		}
		firstShould, ok := should[0].(map[string]any)
		if !ok {
			t.Fatalf("expected should clause map: %#v", should[0])
		}
		prefixMatch, ok := firstShould["multi_match"].(map[string]any)
		if !ok {
			t.Fatalf("expected multi_match in should clause: %#v", firstShould)
		}
		if prefixMatch["type"] != "phrase_prefix" {
			t.Fatalf("expected phrase_prefix type, got %#v", prefixMatch["type"])
		}

		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"hits": map[string]any{
				"hits": []map[string]any{},
			},
		})
	}))
	defer testServer.Close()

	client, err := New(config.ElasticConfig{
		Enabled:     true,
		Addresses:   testServer.URL,
		IndexPrefix: "incidenthub",
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	if _, err := client.Search(context.Background(), "tenant-a", "smoke case", []string{"alerts"}, 10); err != nil {
		t.Fatalf("search: %v", err)
	}
}

func TestRecentUsesTenantFilterAndMatchAll(t *testing.T) {
	requests := 0
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		rawBody, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		var payload map[string]any
		if err := json.Unmarshal(rawBody, &payload); err != nil {
			t.Fatalf("decode request payload: %v", err)
		}
		queryObj, ok := payload["query"].(map[string]any)
		if !ok {
			t.Fatalf("missing query object: %#v", payload["query"])
		}
		boolObj, ok := queryObj["bool"].(map[string]any)
		if !ok {
			t.Fatalf("expected bool query for recent retrieval: %#v", queryObj)
		}
		filters, ok := boolObj["filter"].([]any)
		if !ok || len(filters) == 0 {
			t.Fatalf("expected tenant filter in recent retrieval query: %#v", boolObj["filter"])
		}

		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"hits": map[string]any{
				"hits": []map[string]any{
					{
						"_id":    "case-1",
						"_index": "incidenthub-cases",
						"_score": 0.0,
						"_source": map[string]any{
							"tenant_id": "tenant-a",
							"title":     "Latest case",
						},
					},
				},
			},
		})
	}))
	defer testServer.Close()

	client, err := New(config.ElasticConfig{
		Enabled:     true,
		Addresses:   testServer.URL,
		IndexPrefix: "incidenthub",
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	hits, err := client.Recent(context.Background(), "tenant-a", []string{"cases"}, 5)
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	if requests != 1 {
		t.Fatalf("expected one request to elasticsearch, got %d", requests)
	}
	if len(hits) != 1 || hits[0].ID != "case-1" {
		t.Fatalf("unexpected recent hits: %#v", hits)
	}
}

func TestSearchAllowsUnavailableIndices(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("allow_no_indices"); got != "true" {
			t.Fatalf("expected allow_no_indices=true, got %q", got)
		}
		if got := r.URL.Query().Get("ignore_unavailable"); got != "true" {
			t.Fatalf("expected ignore_unavailable=true, got %q", got)
		}
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"hits": map[string]any{
				"hits": []map[string]any{},
			},
		})
	}))
	defer testServer.Close()

	client, err := New(config.ElasticConfig{
		Enabled:     true,
		Addresses:   testServer.URL,
		IndexPrefix: "incidenthub",
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	if _, err := client.Search(context.Background(), "tenant-a", "probe", []string{"alerts", "cases"}, 10); err != nil {
		t.Fatalf("search: %v", err)
	}
}
