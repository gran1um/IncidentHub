-- Add idempotency key support for connector hub executions.

ALTER TABLE connector_hub_executions
    ADD COLUMN IF NOT EXISTS idempotency_key TEXT NOT NULL DEFAULT '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_connector_hub_executions_tenant_idempotency
    ON connector_hub_executions (tenant_id, idempotency_key)
    WHERE idempotency_key <> '';
