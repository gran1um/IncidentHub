CREATE TABLE IF NOT EXISTS api_access_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    token_hash TEXT NOT NULL UNIQUE,
    token_prefix TEXT NOT NULL,
    scopes TEXT[] NOT NULL DEFAULT '{}'::text[],
    full_access BOOLEAN NOT NULL DEFAULT FALSE,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    expires_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (length(trim(name)) > 0),
    CHECK (length(trim(token_hash)) > 0),
    CHECK (length(trim(token_prefix)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_api_access_tokens_tenant_created
    ON api_access_tokens (tenant_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_api_access_tokens_tenant_active
    ON api_access_tokens (tenant_id, revoked_at, expires_at);

CREATE INDEX IF NOT EXISTS idx_api_access_tokens_hash_active
    ON api_access_tokens (token_hash)
    WHERE revoked_at IS NULL;
