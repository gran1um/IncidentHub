UPDATE catalog_items
SET
    kind = 'inbound_connectors',
    data = jsonb_set(data, '{direction}', '"inbound"'::jsonb, true),
    updated_at = NOW()
WHERE kind = 'connectors'
  AND lower(coalesce(data->>'direction', 'outbound')) = 'inbound';

UPDATE catalog_items
SET
    kind = 'outbound_connectors',
    data = jsonb_set(data, '{direction}', '"outbound"'::jsonb, true),
    updated_at = NOW()
WHERE kind = 'connectors'
  AND lower(coalesce(data->>'direction', 'outbound')) <> 'inbound';

CREATE TABLE IF NOT EXISTS inbound_connector_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    connector_id UUID NOT NULL REFERENCES catalog_items(id) ON DELETE CASCADE,
    scheduled_for TIMESTAMPTZ,
    trigger TEXT NOT NULL DEFAULT 'cron',
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'running',
    records_seen INTEGER NOT NULL DEFAULT 0,
    alerts_created INTEGER NOT NULL DEFAULT 0,
    duplicates_skipped INTEGER NOT NULL DEFAULT 0,
    errors INTEGER NOT NULL DEFAULT 0,
    message TEXT NOT NULL DEFAULT '',
    logs TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (status IN ('running', 'success', 'error', 'skipped')),
    CHECK (trigger IN ('cron', 'manual'))
);

CREATE INDEX IF NOT EXISTS idx_inbound_connector_runs_tenant_connector_started
    ON inbound_connector_runs (tenant_id, connector_id, started_at DESC);

CREATE INDEX IF NOT EXISTS idx_inbound_connector_runs_tenant_started
    ON inbound_connector_runs (tenant_id, started_at DESC);

CREATE INDEX IF NOT EXISTS idx_catalog_items_inbound_connectors
    ON catalog_items (tenant_id, updated_at DESC)
    WHERE kind = 'inbound_connectors';

CREATE INDEX IF NOT EXISTS idx_catalog_items_outbound_connectors
    ON catalog_items (tenant_id, updated_at DESC)
    WHERE kind = 'outbound_connectors';
