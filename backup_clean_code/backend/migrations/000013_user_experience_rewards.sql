ALTER TABLE users
    ADD COLUMN IF NOT EXISTS experience_points BIGINT NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS xp_reward_rules (
    key TEXT PRIMARY KEY,
    points INTEGER NOT NULL CHECK (points >= 0),
    description TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS user_xp_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID REFERENCES tenants(id) ON DELETE SET NULL,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_key TEXT NOT NULL UNIQUE,
    event_type TEXT NOT NULL,
    points INTEGER NOT NULL CHECK (points >= 0),
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_xp_events_user_created
    ON user_xp_events (user_id, created_at DESC);

INSERT INTO xp_reward_rules (key, points, description)
VALUES
    ('case_closed_low', 120, 'Case closed (low severity)'),
    ('case_closed_medium', 220, 'Case closed (medium severity)'),
    ('case_closed_high', 350, 'Case closed (high severity)'),
    ('case_closed_critical', 500, 'Case closed (critical severity)'),
    ('achievement_granted_default', 150, 'Fallback reward for granted achievement'),
    ('connector_invoked', 60, 'Connector invocation'),
    ('analyzer_invoked', 80, 'Analyzer invocation'),
    ('responder_invoked', 70, 'Responder invocation'),
    ('incident_first_message', 100, 'First outbound incident message')
ON CONFLICT (key) DO UPDATE
SET
    points = EXCLUDED.points,
    description = EXCLUDED.description,
    updated_at = NOW();
