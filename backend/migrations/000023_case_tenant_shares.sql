CREATE TABLE IF NOT EXISTS case_tenant_shares (
    case_id UUID NOT NULL REFERENCES cases(id) ON DELETE CASCADE,
    owner_tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    shared_tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (case_id, shared_tenant_id),
    CONSTRAINT chk_case_tenant_shares_not_self CHECK (owner_tenant_id <> shared_tenant_id)
);

CREATE INDEX IF NOT EXISTS idx_case_tenant_shares_shared_tenant
    ON case_tenant_shares(shared_tenant_id);

CREATE INDEX IF NOT EXISTS idx_case_tenant_shares_owner_tenant
    ON case_tenant_shares(owner_tenant_id);
