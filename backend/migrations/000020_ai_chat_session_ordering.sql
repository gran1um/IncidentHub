ALTER TABLE ai_chat_sessions
    ADD COLUMN IF NOT EXISTS sort_order BIGINT NOT NULL DEFAULT 0;

WITH ranked AS (
    SELECT id,
           ROW_NUMBER() OVER (
               PARTITION BY tenant_id, user_id
               ORDER BY is_default DESC, updated_at DESC, created_at DESC, id DESC
           ) AS rn
    FROM ai_chat_sessions
)
UPDATE ai_chat_sessions AS sessions
SET sort_order = ranked.rn
FROM ranked
WHERE sessions.id = ranked.id
  AND sessions.sort_order = 0;

CREATE INDEX IF NOT EXISTS idx_ai_chat_sessions_tenant_user_sort
    ON ai_chat_sessions (tenant_id, user_id, sort_order DESC, updated_at DESC, created_at DESC);
