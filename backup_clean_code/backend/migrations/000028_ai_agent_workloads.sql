CREATE TABLE IF NOT EXISTS ai_agent_workloads (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    queue_event_id UUID NOT NULL REFERENCES ai_agent_queue_events(id) ON DELETE CASCADE,
    agent_id UUID NOT NULL,
    agent_name TEXT NOT NULL DEFAULT '',
    entity_type TEXT NOT NULL,
    entity_id UUID NOT NULL,
    status TEXT NOT NULL DEFAULT 'queued',
    attempt_count INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 4,
    workflow_id TEXT NOT NULL DEFAULT '',
    run_id UUID NULL,
    last_stage TEXT NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT '',
    closed_by_user BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ NULL,
    finished_at TIMESTAMPTZ NULL,
    closed_at TIMESTAMPTZ NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT ai_agent_workloads_entity_type_ck CHECK (entity_type IN ('case', 'alert')),
    CONSTRAINT ai_agent_workloads_status_ck CHECK (status IN ('queued', 'processing', 'done', 'failed', 'cancelled')),
    CONSTRAINT ai_agent_workloads_attempt_count_ck CHECK (attempt_count >= 0),
    CONSTRAINT ai_agent_workloads_max_attempts_ck CHECK (max_attempts >= 1),
    CONSTRAINT ai_agent_workloads_unique_queue_agent UNIQUE (queue_event_id, agent_id)
);

CREATE INDEX IF NOT EXISTS idx_ai_agent_workloads_tenant_status_created
    ON ai_agent_workloads (tenant_id, status, created_at ASC);

CREATE INDEX IF NOT EXISTS idx_ai_agent_workloads_queue_event
    ON ai_agent_workloads (queue_event_id, created_at ASC);

CREATE INDEX IF NOT EXISTS idx_ai_agent_workloads_processing_updated
    ON ai_agent_workloads (status, updated_at ASC)
    WHERE status = 'processing';
