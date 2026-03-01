CREATE INDEX IF NOT EXISTS idx_user_xp_events_user_tenant_created
    ON user_xp_events (user_id, tenant_id, created_at DESC);
