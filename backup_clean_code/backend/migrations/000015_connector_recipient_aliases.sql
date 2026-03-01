CREATE TABLE IF NOT EXISTS connector_recipient_aliases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    connector_id UUID NOT NULL REFERENCES catalog_items(id) ON DELETE CASCADE,
    channel TEXT NOT NULL,
    recipient_id TEXT NOT NULL,
    alias_type TEXT NOT NULL,
    alias_value TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (length(trim(channel)) > 0),
    CHECK (length(trim(recipient_id)) > 0),
    CHECK (length(trim(alias_type)) > 0),
    CHECK (length(trim(alias_value)) > 0),
    UNIQUE (tenant_id, connector_id, channel, alias_type, alias_value)
);

CREATE INDEX IF NOT EXISTS idx_connector_recipient_aliases_lookup
    ON connector_recipient_aliases (tenant_id, connector_id, channel, alias_value, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_connector_recipient_aliases_recipient
    ON connector_recipient_aliases (tenant_id, connector_id, channel, recipient_id, updated_at DESC);
