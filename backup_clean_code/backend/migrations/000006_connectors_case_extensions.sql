ALTER TABLE cases
    ADD COLUMN IF NOT EXISTS case_number TEXT,
    ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS incident_type TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS priority TEXT NOT NULL DEFAULT 'medium',
    ADD COLUMN IF NOT EXISTS impact TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS confidence INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS detected_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS occurred_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS closed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS resolution_summary TEXT NOT NULL DEFAULT '';

ALTER TABLE cases
    DROP CONSTRAINT IF EXISTS cases_confidence_range;

ALTER TABLE cases
    ADD CONSTRAINT cases_confidence_range CHECK (confidence >= 0 AND confidence <= 100);

CREATE UNIQUE INDEX IF NOT EXISTS uq_cases_tenant_case_number
    ON cases (tenant_id, case_number)
    WHERE case_number IS NOT NULL AND length(trim(case_number)) > 0;

CREATE TABLE IF NOT EXISTS connector_ingest_states (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    connector_id UUID NOT NULL REFERENCES catalog_items(id) ON DELETE CASCADE,
    external_id TEXT NOT NULL,
    payload_hash TEXT NOT NULL,
    alert_id UUID REFERENCES alerts(id) ON DELETE SET NULL,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, connector_id, external_id)
);

CREATE INDEX IF NOT EXISTS idx_connector_ingest_states_tenant_connector
    ON connector_ingest_states (tenant_id, connector_id, last_seen_at DESC);

CREATE TABLE IF NOT EXISTS forum_external_bindings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    thread_id UUID NOT NULL REFERENCES catalog_items(id) ON DELETE CASCADE,
    connector_id UUID NOT NULL REFERENCES catalog_items(id) ON DELETE CASCADE,
    conversation_id TEXT NOT NULL DEFAULT '',
    cursor TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    last_synced_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, thread_id, connector_id)
);

CREATE INDEX IF NOT EXISTS idx_forum_external_bindings_tenant_thread
    ON forum_external_bindings (tenant_id, thread_id, updated_at DESC);
