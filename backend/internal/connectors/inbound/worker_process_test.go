package inbound

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"

	"github.com/google/uuid"
)

type testConnectorStore struct {
	itemsByKind map[string][]models.CatalogItem
	err         error
}

func (s *testConnectorStore) ListByKindAllTenants(_ context.Context, kind string, limit, offset int) ([]models.CatalogItem, error) {
	if s.err != nil {
		return nil, s.err
	}
	items := s.itemsByKind[kind]
	if offset >= len(items) {
		return []models.CatalogItem{}, nil
	}
	if limit <= 0 {
		limit = len(items)
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end], nil
}

type testAlertStore struct {
	createErr error
	created   []repository.CreateAlertParams
}

func (s *testAlertStore) Create(_ context.Context, p repository.CreateAlertParams) (*models.Alert, error) {
	s.created = append(s.created, p)
	if s.createErr != nil {
		return nil, s.createErr
	}
	return &models.Alert{
		ID:        uuid.New(),
		TenantID:  p.TenantID,
		Title:     p.Title,
		Source:    p.Source,
		Status:    p.Status,
		Severity:  p.Severity,
		TLP:       p.TLP,
		PAP:       p.PAP,
		CreatedBy: p.CreatedBy,
	}, nil
}

type testIngestStore struct {
	tryAcquireResult bool
	tryAcquireErr    error
	bindErr          error
	tryAcquireCalls  int
	bindCalls        int
	releaseCalls     int
}

func (s *testIngestStore) TryAcquire(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ string, _ string) (bool, error) {
	s.tryAcquireCalls++
	if s.tryAcquireErr != nil {
		return false, s.tryAcquireErr
	}
	if !s.tryAcquireResult {
		return false, nil
	}
	return true, nil
}

func (s *testIngestStore) BindAlert(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ string, _ uuid.UUID) error {
	s.bindCalls++
	return s.bindErr
}

func (s *testIngestStore) ReleaseOnFailure(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ string) error {
	s.releaseCalls++
	return nil
}

type testRunStore struct {
	createErr      error
	latest         *models.InboundConnectorRun
	finishCalls    int
	lastFinish     repository.FinishInboundConnectorRunParams
	lastFinishLogs string
}

func (s *testRunStore) Create(_ context.Context, p repository.CreateInboundConnectorRunParams) (*models.InboundConnectorRun, error) {
	if s.createErr != nil {
		return nil, s.createErr
	}
	return &models.InboundConnectorRun{
		ID:          uuid.New(),
		TenantID:    p.TenantID,
		ConnectorID: p.ConnectorID,
		StartedAt:   time.Now().UTC(),
		Status:      "running",
	}, nil
}

func (s *testRunStore) Finish(_ context.Context, _ uuid.UUID, _ uuid.UUID, p repository.FinishInboundConnectorRunParams) error {
	s.finishCalls++
	s.lastFinish = p
	s.lastFinishLogs = p.Logs
	return nil
}

func (s *testRunStore) GetLatestByConnector(_ context.Context, _ uuid.UUID, _ uuid.UUID) (*models.InboundConnectorRun, error) {
	return s.latest, nil
}

type testSearchIndexer struct {
	calls int
}

func (s *testSearchIndexer) IndexDocument(_ context.Context, _ string, _ string, _ any) error {
	s.calls++
	return nil
}

func TestWorkerRunNowRequiresDependencies(t *testing.T) {
	worker := &Worker{cfg: config.InboundConnectorsConfig{Enabled: true}}
	_, err := worker.RunNow(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected dependency error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "dependencies") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWorkerRunNowWithNoConnectors(t *testing.T) {
	alerts := &testAlertStore{}
	ingest := &testIngestStore{tryAcquireResult: true}
	worker := &Worker{
		cfg:     config.InboundConnectorsConfig{Enabled: true, BatchLimit: 10, PollInterval: time.Minute},
		catalog: &testConnectorStore{itemsByKind: map[string][]models.CatalogItem{}},
		alerts:  alerts,
		ingest:  ingest,
	}

	stats, err := worker.RunNow(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("run now error: %v", err)
	}
	if stats.ConnectorsScanned != 0 || stats.AlertsCreated != 0 {
		t.Fatalf("expected empty stats, got %+v", stats)
	}
}

func TestProcessConnectorSuccessFlow(t *testing.T) {
	tenantID := uuid.New()
	ownerID := uuid.New()
	connector := models.CatalogItem{
		ID:       uuid.New(),
		TenantID: &tenantID,
		OwnerID:  &ownerID,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{
					"id":          "ext-1",
					"title":       "Suspicious login",
					"description": "Unexpected geo",
					"source":      "iam",
					"severity":    "high",
					"status":      "triaged",
					"tlp":         "red",
					"pap":         "green",
				},
			},
		})
	}))
	defer server.Close()

	alerts := &testAlertStore{}
	ingest := &testIngestStore{tryAcquireResult: true}
	runs := &testRunStore{}
	search := &testSearchIndexer{}
	worker := &Worker{
		cfg:        config.InboundConnectorsConfig{BatchLimit: 100, HTTPTimeout: time.Second},
		alerts:     alerts,
		ingest:     ingest,
		runs:       runs,
		search:     search,
		httpClient: server.Client(),
	}

	stats, err := worker.processConnector(context.Background(), connector, inboundConnectorConfig{
		DisplayName:      "IAM Feed",
		SourceType:       "http",
		ArrayPath:        "items",
		IDField:          "id",
		TitleField:       "title",
		DescriptionField: "description",
		SourceField:      "source",
		SeverityField:    "severity",
		StatusField:      "status",
		TLPField:         "tlp",
		PAPField:         "pap",
		Severity:         "medium",
		Status:           "new",
		TLP:              "amber",
		PAP:              "amber",
		HTTP: inboundHTTPSourceConfig{
			URL:    server.URL,
			Method: http.MethodGet,
		},
	}, nil, "manual")
	if err != nil {
		t.Fatalf("process connector error: %v", err)
	}
	if stats.AlertsCreated != 1 || stats.RecordsSeen != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if len(alerts.created) != 1 {
		t.Fatalf("expected 1 created alert, got %d", len(alerts.created))
	}
	if alerts.created[0].Severity != "high" || alerts.created[0].Status != "triaged" {
		t.Fatalf("unexpected mapped alert params: %+v", alerts.created[0])
	}
	if ingest.bindCalls != 1 {
		t.Fatalf("expected bind alert call, got %d", ingest.bindCalls)
	}
	if search.calls != 1 {
		t.Fatalf("expected 1 search index call, got %d", search.calls)
	}
	if runs.finishCalls != 1 {
		t.Fatalf("expected run finish call, got %d", runs.finishCalls)
	}
	if runs.lastFinish.Status != "success" {
		t.Fatalf("expected success run status, got %q", runs.lastFinish.Status)
	}
}

func TestProcessConnectorFetchErrorSetsRunError(t *testing.T) {
	tenantID := uuid.New()
	connector := models.CatalogItem{ID: uuid.New(), TenantID: &tenantID}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "upstream down", http.StatusBadGateway)
	}))
	defer server.Close()

	runs := &testRunStore{}
	worker := &Worker{
		cfg:        config.InboundConnectorsConfig{BatchLimit: 10, HTTPTimeout: time.Second},
		alerts:     &testAlertStore{},
		ingest:     &testIngestStore{tryAcquireResult: true},
		runs:       runs,
		httpClient: server.Client(),
	}

	stats, err := worker.processConnector(context.Background(), connector, inboundConnectorConfig{
		DisplayName: "Failing Feed",
		SourceType:  "http",
		HTTP: inboundHTTPSourceConfig{
			URL:    server.URL,
			Method: http.MethodGet,
		},
	}, nil, "manual")
	if err == nil {
		t.Fatal("expected process connector error")
	}
	if stats.Errors != 1 {
		t.Fatalf("expected error counter 1, got %+v", stats)
	}
	if runs.finishCalls != 1 {
		t.Fatalf("expected finish call on error, got %d", runs.finishCalls)
	}
	if runs.lastFinish.Status != "error" {
		t.Fatalf("expected run error status, got %q", runs.lastFinish.Status)
	}
}

func TestFinishRunTruncatesLogs(t *testing.T) {
	tenantID := uuid.New()
	runID := uuid.New()
	runs := &testRunStore{}
	worker := &Worker{runs: runs}

	longLogs := strings.Repeat("x", 9000)
	worker.finishRun(context.Background(), runID, tenantID, RunStats{}, "success", "ok", longLogs)
	if runs.finishCalls != 1 {
		t.Fatalf("expected finish call, got %d", runs.finishCalls)
	}
	if len(runs.lastFinishLogs) != 8000 {
		t.Fatalf("expected logs to be truncated to 8000, got %d", len(runs.lastFinishLogs))
	}
}

func TestNormalizeHelpersFallbacks(t *testing.T) {
	if got := normalizeSeverity("UNKNOWN", ""); got != "medium" {
		t.Fatalf("unexpected severity fallback: %q", got)
	}
	if got := normalizeAlertStatus("", "closed"); got != "closed" {
		t.Fatalf("unexpected alert status fallback: %q", got)
	}
	if got := normalizeTrafficLight("invalid", "green"); got != "amber" {
		t.Fatalf("unexpected traffic light fallback: %q", got)
	}
}

func TestConnectorFlagsHelpers(t *testing.T) {
	item := models.CatalogItem{Kind: "connectors", Data: map[string]any{"direction": "inbound"}}
	if !isInboundConnector(item) {
		t.Fatal("expected direction inbound to be treated as inbound connector")
	}
	if !isConnectorEnabled(map[string]any{"enabled": "true"}) {
		t.Fatal("expected enabled=true")
	}
	if isConnectorEnabled(map[string]any{"enabled": "disabled"}) {
		t.Fatal("expected disabled connector")
	}

	hashA := hashRecord(map[string]any{"id": "1", "value": "x"})
	hashB := hashRecord(map[string]any{"id": "1", "value": "x"})
	if hashA != hashB {
		t.Fatal("expected deterministic record hash")
	}
}
