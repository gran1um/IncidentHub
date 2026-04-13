CREATE TABLE IF NOT EXISTS connector_hub_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    actor_id UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    connector_id UUID NOT NULL,
    connector_name TEXT NOT NULL DEFAULT '',
    connector_channel TEXT NOT NULL DEFAULT '',
    method_id UUID NULL,
    method_name TEXT NOT NULL DEFAULT '',
    method_key TEXT NOT NULL DEFAULT '',
    action TEXT NOT NULL DEFAULT '',
    case_id UUID NULL REFERENCES cases(id) ON DELETE SET NULL,
    alert_id UUID NULL REFERENCES alerts(id) ON DELETE SET NULL,
    execution_mode TEXT NOT NULL DEFAULT 'manual',
    dry_run BOOLEAN NOT NULL DEFAULT FALSE,
    status TEXT NOT NULL DEFAULT 'accepted',
    request JSONB NOT NULL DEFAULT '{}'::jsonb,
    response JSONB NOT NULL DEFAULT '{}'::jsonb,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    error TEXT NOT NULL DEFAULT '',
    provider_status TEXT NOT NULL DEFAULT '',
    external_id TEXT NOT NULL DEFAULT '',
    correlation_id TEXT NOT NULL DEFAULT '',
    attempt_count INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 3,
    next_attempt_at TIMESTAMPTZ NULL,
    cancelled_by_user BOOLEAN NOT NULL DEFAULT FALSE,
    cancelled_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ NULL,
    finished_at TIMESTAMPTZ NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT connector_hub_executions_status_ck CHECK (status IN (
        'accepted', 'queued', 'dispatching', 'provider_accepted', 'retry_scheduled',
        'completed', 'dry_run', 'failed', 'cancelled', 'dead_letter'
    )),
    CONSTRAINT connector_hub_executions_attempt_count_ck CHECK (attempt_count >= 0),
    CONSTRAINT connector_hub_executions_max_attempts_ck CHECK (max_attempts >= 1),
    CONSTRAINT connector_hub_executions_mode_ck CHECK (execution_mode IN ('manual', 'ai_agent', 'automation', 'system'))
);

CREATE INDEX IF NOT EXISTS idx_connector_hub_executions_tenant_created
    ON connector_hub_executions (tenant_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_connector_hub_executions_status_next_attempt
    ON connector_hub_executions (status, next_attempt_at ASC, created_at ASC)
    WHERE status IN ('accepted', 'queued', 'retry_scheduled');

CREATE INDEX IF NOT EXISTS idx_connector_hub_executions_case_created
    ON connector_hub_executions (case_id, created_at DESC)
    WHERE case_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_connector_hub_executions_alert_created
    ON connector_hub_executions (alert_id, created_at DESC)
    WHERE alert_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_connector_hub_executions_connector_created
    ON connector_hub_executions (connector_id, created_at DESC);

CREATE TABLE IF NOT EXISTS connector_hub_execution_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id UUID NOT NULL REFERENCES connector_hub_executions(id) ON DELETE CASCADE,
    attempt_no INT NOT NULL,
    status TEXT NOT NULL DEFAULT 'dispatching',
    request JSONB NOT NULL DEFAULT '{}'::jsonb,
    response JSONB NOT NULL DEFAULT '{}'::jsonb,
    error TEXT NOT NULL DEFAULT '',
    provider_status TEXT NOT NULL DEFAULT '',
    external_id TEXT NOT NULL DEFAULT '',
    correlation_id TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ NULL,
    duration_ms BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT connector_hub_execution_attempts_attempt_no_ck CHECK (attempt_no >= 1),
    CONSTRAINT connector_hub_execution_attempts_status_ck CHECK (status IN (
        'dispatching', 'provider_accepted', 'completed', 'dry_run', 'failed', 'cancelled', 'dead_letter'
    )),
    CONSTRAINT connector_hub_execution_attempts_unique_execution_attempt UNIQUE (execution_id, attempt_no)
);

CREATE INDEX IF NOT EXISTS idx_connector_hub_execution_attempts_execution_created
    ON connector_hub_execution_attempts (execution_id, created_at DESC);

CREATE TABLE IF NOT EXISTS connector_hub_execution_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id UUID NOT NULL REFERENCES connector_hub_executions(id) ON DELETE CASCADE,
    attempt_no INT NULL,
    event_type TEXT NOT NULL,
    status TEXT NOT NULL,
    message TEXT NOT NULL DEFAULT '',
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT connector_hub_execution_events_status_ck CHECK (status IN (
        'accepted', 'queued', 'dispatching', 'provider_accepted', 'retry_scheduled',
        'completed', 'dry_run', 'failed', 'cancelled', 'dead_letter'
    ))
);

CREATE INDEX IF NOT EXISTS idx_connector_hub_execution_events_execution_created
    ON connector_hub_execution_events (execution_id, created_at DESC);
