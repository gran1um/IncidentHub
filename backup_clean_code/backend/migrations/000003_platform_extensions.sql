ALTER TABLE tenants
    ADD COLUMN IF NOT EXISTS max_users INTEGER NOT NULL DEFAULT 50,
    ADD COLUMN IF NOT EXISTS responsible_user_id UUID REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS team TEXT NOT NULL DEFAULT 'SOC',
    ADD COLUMN IF NOT EXISTS avatar_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS cover_image_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS personal_link TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS catalog_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    owner_id UUID REFERENCES users(id) ON DELETE CASCADE,
    ref_id UUID,
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (length(trim(kind)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_users_active
    ON users(is_active);

CREATE INDEX IF NOT EXISTS idx_users_platform_admin
    ON users(is_platform_admin);

CREATE INDEX IF NOT EXISTS idx_tenant_memberships_user_active
    ON tenant_memberships(user_id, is_active);

CREATE INDEX IF NOT EXISTS idx_tenant_memberships_tenant_role
    ON tenant_memberships(tenant_id, role, is_active);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_active
    ON refresh_tokens(user_id, expires_at DESC)
    WHERE revoked_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_alerts_tenant_status_severity
    ON alerts(tenant_id, status, severity, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_cases_tenant_status_severity
    ON cases(tenant_id, status, severity, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_tasks_tenant_case_updated
    ON tasks(tenant_id, case_id, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_tasks_tenant_assignee
    ON tasks(tenant_id, assignee_id, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_case_observables_tenant_type
    ON case_observables(tenant_id, type, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_case_observables_tenant_verdict
    ON case_observables(tenant_id, verdict, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_case_timeline_tenant_type
    ON case_timeline_events(tenant_id, event_type, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_case_pages_tenant_title
    ON case_pages(tenant_id, case_id, title);

CREATE INDEX IF NOT EXISTS idx_case_attachments_tenant_storage
    ON case_attachments(tenant_id, storage_key);

CREATE INDEX IF NOT EXISTS idx_catalog_items_kind_tenant_updated
    ON catalog_items(kind, tenant_id, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_catalog_items_kind_owner
    ON catalog_items(kind, owner_id, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_catalog_items_kind_ref
    ON catalog_items(kind, ref_id, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_catalog_items_data_gin
    ON catalog_items USING GIN (data);
