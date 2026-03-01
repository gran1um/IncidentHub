CREATE TABLE IF NOT EXISTS ai_agent_queue_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    actor_id UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    entity_type TEXT NOT NULL,
    entity_id UUID NOT NULL,
    source TEXT NOT NULL DEFAULT 'api',
    status TEXT NOT NULL DEFAULT 'queued',
    matched_agents INT NOT NULL DEFAULT 0,
    processed_agents INT NOT NULL DEFAULT 0,
    workflow_id TEXT NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ NULL,
    finished_at TIMESTAMPTZ NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT ai_agent_queue_events_entity_type_ck CHECK (entity_type IN ('case', 'alert')),
    CONSTRAINT ai_agent_queue_events_status_ck CHECK (status IN ('queued', 'processing', 'done', 'failed')),
    CONSTRAINT ai_agent_queue_events_unique_entity UNIQUE (tenant_id, entity_type, entity_id)
);

CREATE INDEX IF NOT EXISTS idx_ai_agent_queue_events_status_created
    ON ai_agent_queue_events (status, created_at ASC);

CREATE INDEX IF NOT EXISTS idx_ai_agent_queue_events_tenant_status
    ON ai_agent_queue_events (tenant_id, status, created_at ASC);
