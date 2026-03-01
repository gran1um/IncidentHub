ALTER TABLE ai_agent_workloads
    ADD COLUMN IF NOT EXISTS execution_policy TEXT NOT NULL DEFAULT 'all_matching',
    ADD COLUMN IF NOT EXISTS execution_priority INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS execution_index INT NOT NULL DEFAULT 0;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'ai_agent_workloads_execution_policy_ck'
    ) THEN
        ALTER TABLE ai_agent_workloads
            ADD CONSTRAINT ai_agent_workloads_execution_policy_ck
            CHECK (execution_policy IN ('all_matching', 'exclusive', 'first_match', 'fallback_chain'));
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_ai_agent_workloads_queue_policy_order
    ON ai_agent_workloads (queue_event_id, execution_policy, execution_index ASC, execution_priority DESC, created_at ASC);
