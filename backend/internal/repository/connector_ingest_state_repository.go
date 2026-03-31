package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ConnectorIngestStateRepository struct {
	pool *pgxpool.Pool
}

func NewConnectorIngestStateRepository(pool *pgxpool.Pool) *ConnectorIngestStateRepository {
	return &ConnectorIngestStateRepository{pool: pool}
}

func (r *ConnectorIngestStateRepository) TryAcquire(ctx context.Context, tenantID, connectorID uuid.UUID, externalID, payloadHash string) (bool, error) {
	cmd, err := r.pool.Exec(ctx, `
		INSERT INTO connector_ingest_states(tenant_id, connector_id, external_id, payload_hash)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (tenant_id, connector_id, external_id) DO NOTHING
	`, tenantID, connectorID, externalID, payloadHash)
	if err != nil {
		return false, fmt.Errorf("insert connector ingest state: %w", err)
	}
	return cmd.RowsAffected() == 1, nil
}

func (r *ConnectorIngestStateRepository) BindAlert(ctx context.Context, tenantID, connectorID uuid.UUID, externalID string, alertID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE connector_ingest_states
		SET
			alert_id = $4,
			last_seen_at = NOW(),
			updated_at = NOW()
		WHERE tenant_id = $1 AND connector_id = $2 AND external_id = $3
	`, tenantID, connectorID, externalID, alertID)
	if err != nil {
		return fmt.Errorf("bind alert to connector ingest state: %w", err)
	}
	return nil
}

func (r *ConnectorIngestStateRepository) ReleaseOnFailure(ctx context.Context, tenantID, connectorID uuid.UUID, externalID string) error {
	_, err := r.pool.Exec(ctx, `
		DELETE FROM connector_ingest_states
		WHERE tenant_id = $1 AND connector_id = $2 AND external_id = $3 AND alert_id IS NULL
	`, tenantID, connectorID, externalID)
	if err != nil {
		return fmt.Errorf("release connector ingest state: %w", err)
	}
	return nil
}
