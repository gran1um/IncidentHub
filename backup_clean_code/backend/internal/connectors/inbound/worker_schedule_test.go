package inbound

import (
	"context"
	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"
	"testing"
	"time"

	"github.com/google/uuid"
)

type stubRunStore struct {
	latest *models.InboundConnectorRun
	err    error
}

func (s *stubRunStore) Create(context.Context, repository.CreateInboundConnectorRunParams) (*models.InboundConnectorRun, error) {
	return nil, nil
}

func (s *stubRunStore) Finish(context.Context, uuid.UUID, uuid.UUID, repository.FinishInboundConnectorRunParams) error {
	return nil
}

func (s *stubRunStore) GetLatestByConnector(context.Context, uuid.UUID, uuid.UUID) (*models.InboundConnectorRun, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.latest == nil {
		return nil, nil
	}
	return s.latest, nil
}

func TestShouldExecuteConnector_ForceBypassesSchedule(t *testing.T) {
	tenantID := uuid.New()
	connector := models.CatalogItem{ID: uuid.New(), TenantID: &tenantID}
	worker := &Worker{cfg: config.InboundConnectorsConfig{PollInterval: 5 * time.Minute}, runs: &stubRunStore{}}

	ok, _, err := worker.shouldExecuteConnector(context.Background(), connector, inboundConnectorConfig{Schedule: "0 0 1 1 *"}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected forced execution to bypass schedule")
	}
}

func TestShouldExecuteConnector_EveryMinuteDue(t *testing.T) {
	tenantID := uuid.New()
	connector := models.CatalogItem{ID: uuid.New(), TenantID: &tenantID}
	worker := &Worker{cfg: config.InboundConnectorsConfig{PollInterval: time.Minute}, runs: &stubRunStore{}}

	ok, scheduledFor, err := worker.shouldExecuteConnector(context.Background(), connector, inboundConnectorConfig{Schedule: "* * * * *"}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected execution for every-minute schedule")
	}
	if scheduledFor == nil {
		t.Fatal("expected scheduled_for timestamp")
	}
}

func TestShouldExecuteConnector_NotDueAfterRecentRun(t *testing.T) {
	tenantID := uuid.New()
	startedAt := time.Now().UTC().Truncate(time.Minute)
	connector := models.CatalogItem{ID: uuid.New(), TenantID: &tenantID}
	worker := &Worker{
		cfg:  config.InboundConnectorsConfig{PollInterval: time.Minute},
		runs: &stubRunStore{latest: &models.InboundConnectorRun{StartedAt: startedAt}},
	}

	ok, _, err := worker.shouldExecuteConnector(context.Background(), connector, inboundConnectorConfig{Schedule: "* * * * *"}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected connector to be skipped until next schedule tick")
	}
}

func TestShouldExecuteConnector_InvalidCron(t *testing.T) {
	tenantID := uuid.New()
	connector := models.CatalogItem{ID: uuid.New(), TenantID: &tenantID}
	worker := &Worker{cfg: config.InboundConnectorsConfig{PollInterval: time.Minute}, runs: &stubRunStore{}}

	_, _, err := worker.shouldExecuteConnector(context.Background(), connector, inboundConnectorConfig{Schedule: "invalid cron"}, false)
	if err == nil {
		t.Fatal("expected parse error for invalid cron")
	}
}

func TestIsInboundConnectorByKind(t *testing.T) {
	item := models.CatalogItem{Kind: "inbound_connectors"}
	if !isInboundConnector(item) {
		t.Fatal("expected inbound_connectors kind to be treated as inbound connector")
	}
}
