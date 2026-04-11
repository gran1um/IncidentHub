CREATE TABLE IF NOT EXISTS async_operations (
    operation_id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    actor_id UUID NOT NULL,
    resource TEXT NOT NULL,
    resource_id UUID,
    operation_type TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('queued', 'processing', 'done', 'failed')),
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    max_attempts INTEGER NOT NULL DEFAULT 1 CHECK (max_attempts > 0),
    last_error TEXT NOT NULL DEFAULT '',
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    queued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_async_operations_tenant_requested
    ON async_operations (tenant_id, queued_at DESC);

CREATE INDEX IF NOT EXISTS idx_async_operations_status
    ON async_operations (status, updated_at DESC);
