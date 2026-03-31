package repository

import (
	"context"
	"fmt"
	"incidenthub/backend/internal/models"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type InboundConnectorRunRepository struct {
	pool *pgxpool.Pool
}

type CreateInboundConnectorRunParams struct {
	TenantID     uuid.UUID
	ConnectorID  uuid.UUID
	ScheduledFor *time.Time
	Trigger      string
}

type FinishInboundConnectorRunParams struct {
	Status            string
	RecordsSeen       int
	AlertsCreated     int
	DuplicatesSkipped int
	Errors            int
	Message           string
	Logs              string
}

func NewInboundConnectorRunRepository(pool *pgxpool.Pool) *InboundConnectorRunRepository {
	return &InboundConnectorRunRepository{pool: pool}
}

func (r *InboundConnectorRunRepository) Create(ctx context.Context, p CreateInboundConnectorRunParams) (*models.InboundConnectorRun, error) {
	trigger := p.Trigger
	if trigger == "" {
		trigger = "cron"
	}
	row := r.pool.QueryRow(ctx, `
		INSERT INTO inbound_connector_runs(tenant_id, connector_id, scheduled_for, trigger)
		VALUES ($1, $2, $3, $4)
		RETURNING
			id, tenant_id, connector_id, scheduled_for, trigger, started_at, finished_at, status,
			records_seen, alerts_created, duplicates_skipped, errors, message, logs, created_at, updated_at
	`, p.TenantID, p.ConnectorID, p.ScheduledFor, trigger)

	item, err := scanInboundConnectorRun(row)
	if err != nil {
		return nil, fmt.Errorf("create inbound connector run: %w", err)
	}
	return item, nil
}

func (r *InboundConnectorRunRepository) Finish(ctx context.Context, runID, tenantID uuid.UUID, p FinishInboundConnectorRunParams) error {
	if p.Status == "" {
		p.Status = "success"
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE inbound_connector_runs
		SET
			status = $3,
			records_seen = $4,
			alerts_created = $5,
			duplicates_skipped = $6,
			errors = $7,
			message = $8,
			logs = $9,
			finished_at = NOW(),
			updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2
	`, runID, tenantID, p.Status, p.RecordsSeen, p.AlertsCreated, p.DuplicatesSkipped, p.Errors, p.Message, p.Logs)
	if err != nil {
		return fmt.Errorf("finish inbound connector run: %w", err)
	}
	return nil
}

func (r *InboundConnectorRunRepository) GetLatestByConnector(ctx context.Context, tenantID, connectorID uuid.UUID) (*models.InboundConnectorRun, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT
			id, tenant_id, connector_id, scheduled_for, trigger, started_at, finished_at, status,
			records_seen, alerts_created, duplicates_skipped, errors, message, logs, created_at, updated_at
		FROM inbound_connector_runs
		WHERE tenant_id = $1 AND connector_id = $2
		ORDER BY started_at DESC
		LIMIT 1
	`, tenantID, connectorID)
	item, err := scanInboundConnectorRun(row)
	if err != nil {
		return nil, fmt.Errorf("get latest inbound connector run: %w", err)
	}
	return item, nil
}

func (r *InboundConnectorRunRepository) ListByConnector(ctx context.Context, tenantID, connectorID uuid.UUID, limit int) ([]models.InboundConnectorRun, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx, `
		SELECT
			id, tenant_id, connector_id, scheduled_for, trigger, started_at, finished_at, status,
			records_seen, alerts_created, duplicates_skipped, errors, message, logs, created_at, updated_at
		FROM inbound_connector_runs
		WHERE tenant_id = $1 AND connector_id = $2
		ORDER BY started_at DESC
		LIMIT $3
	`, tenantID, connectorID, limit)
	if err != nil {
		return nil, fmt.Errorf("list inbound connector runs by connector: %w", err)
	}
	defer rows.Close()
	return collectInboundConnectorRuns(rows)
}

func (r *InboundConnectorRunRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID, limit int) ([]models.InboundConnectorRun, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	rows, err := r.pool.Query(ctx, `
		SELECT
			id, tenant_id, connector_id, scheduled_for, trigger, started_at, finished_at, status,
			records_seen, alerts_created, duplicates_skipped, errors, message, logs, created_at, updated_at
		FROM inbound_connector_runs
		WHERE tenant_id = $1
		ORDER BY started_at DESC
		LIMIT $2
	`, tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("list inbound connector runs by tenant: %w", err)
	}
	defer rows.Close()
	return collectInboundConnectorRuns(rows)
}

func (r *InboundConnectorRunRepository) ListLatestByTenant(ctx context.Context, tenantID uuid.UUID) ([]models.InboundConnectorRun, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT ON (connector_id)
			id, tenant_id, connector_id, scheduled_for, trigger, started_at, finished_at, status,
			records_seen, alerts_created, duplicates_skipped, errors, message, logs, created_at, updated_at
		FROM inbound_connector_runs
		WHERE tenant_id = $1
		ORDER BY connector_id, started_at DESC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list latest inbound connector runs by tenant: %w", err)
	}
	defer rows.Close()
	return collectInboundConnectorRuns(rows)
}

func scanInboundConnectorRun(scanner rowScanner) (*models.InboundConnectorRun, error) {
	item := &models.InboundConnectorRun{}
	if err := scanner.Scan(
		&item.ID,
		&item.TenantID,
		&item.ConnectorID,
		&item.ScheduledFor,
		&item.Trigger,
		&item.StartedAt,
		&item.FinishedAt,
		&item.Status,
		&item.RecordsSeen,
		&item.AlertsCreated,
		&item.DuplicatesSkipped,
		&item.Errors,
		&item.Message,
		&item.Logs,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return item, nil
}

func collectInboundConnectorRuns(rows pgx.Rows) ([]models.InboundConnectorRun, error) {
	out := make([]models.InboundConnectorRun, 0)
	for rows.Next() {
		item, err := scanInboundConnectorRun(rows)
		if err != nil {
			return nil, fmt.Errorf("scan inbound connector run: %w", err)
		}
		out = append(out, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate inbound connector runs: %w", err)
	}
	return out, nil
}
