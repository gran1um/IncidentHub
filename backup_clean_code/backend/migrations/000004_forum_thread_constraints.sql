WITH ranked_threads AS (
    SELECT
        id,
        ROW_NUMBER() OVER (
            PARTITION BY tenant_id, ref_id
            ORDER BY updated_at DESC, created_at DESC, id
        ) AS rn
    FROM catalog_items
    WHERE kind = 'forum_thread' AND ref_id IS NOT NULL
)
DELETE FROM catalog_items ci
USING ranked_threads rt
WHERE ci.id = rt.id
  AND rt.rn > 1;

CREATE UNIQUE INDEX IF NOT EXISTS uq_catalog_forum_thread_case
    ON catalog_items(tenant_id, ref_id)
    WHERE kind = 'forum_thread' AND ref_id IS NOT NULL;
