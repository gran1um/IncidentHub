ALTER TABLE ai_agent_queue_events
    ADD COLUMN IF NOT EXISTS attempt_count INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS max_attempts INT NOT NULL DEFAULT 4,
    ADD COLUMN IF NOT EXISTS closed_by_user BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS closed_at TIMESTAMPTZ NULL;

UPDATE ai_agent_queue_events
SET attempt_count = GREATEST(COALESCE(attempt_count, 0), 0),
    max_attempts = CASE WHEN COALESCE(max_attempts, 0) > 0 THEN max_attempts ELSE 4 END
WHERE attempt_count < 0
   OR COALESCE(max_attempts, 0) <= 0;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'ai_agent_queue_events_attempt_count_ck'
    ) THEN
        ALTER TABLE ai_agent_queue_events
            ADD CONSTRAINT ai_agent_queue_events_attempt_count_ck CHECK (attempt_count >= 0);
    END IF;
END
$$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'ai_agent_queue_events_max_attempts_ck'
    ) THEN
        ALTER TABLE ai_agent_queue_events
            ADD CONSTRAINT ai_agent_queue_events_max_attempts_ck CHECK (max_attempts >= 1);
    END IF;
END
$$;

CREATE INDEX IF NOT EXISTS idx_ai_agent_queue_events_processing_updated
    ON ai_agent_queue_events (status, updated_at ASC)
    WHERE status = 'processing';
