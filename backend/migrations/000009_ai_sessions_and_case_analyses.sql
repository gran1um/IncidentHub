CREATE TABLE IF NOT EXISTS ai_chat_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title TEXT NOT NULL DEFAULT 'SOC Assistant',
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_ai_chat_sessions_default
    ON ai_chat_sessions (tenant_id, user_id)
    WHERE is_default = TRUE;

CREATE INDEX IF NOT EXISTS idx_ai_chat_sessions_tenant_user_updated
    ON ai_chat_sessions (tenant_id, user_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS ai_chat_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES ai_chat_sessions(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('system', 'user', 'assistant')),
    content TEXT NOT NULL,
    sources JSONB NOT NULL DEFAULT '[]'::jsonb,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ai_chat_messages_session_created
    ON ai_chat_messages (session_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ai_chat_messages_tenant_user_created
    ON ai_chat_messages (tenant_id, user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS case_ai_analyses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    case_id UUID NOT NULL REFERENCES cases(id) ON DELETE CASCADE,
    requested_by UUID REFERENCES users(id) ON DELETE SET NULL,
    model TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'completed' CHECK (status IN ('completed', 'failed')),
    verdict TEXT NOT NULL DEFAULT 'unknown',
    confidence NUMERIC(5,2) NOT NULL DEFAULT 0 CHECK (confidence >= 0 AND confidence <= 100),
    summary TEXT NOT NULL DEFAULT '',
    recommendations JSONB NOT NULL DEFAULT '[]'::jsonb,
    findings JSONB NOT NULL DEFAULT '[]'::jsonb,
    sources JSONB NOT NULL DEFAULT '[]'::jsonb,
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_case_ai_analyses_tenant_case_created
    ON case_ai_analyses (tenant_id, case_id, created_at DESC);
