CREATE INDEX IF NOT EXISTS idx_catalog_items_case_communication_threads
    ON catalog_items (tenant_id, ref_id, updated_at DESC)
    WHERE kind = 'case_communication_thread';

CREATE INDEX IF NOT EXISTS idx_catalog_items_case_communication_messages
    ON catalog_items (tenant_id, ref_id, created_at DESC)
    WHERE kind = 'case_communication_message';

CREATE INDEX IF NOT EXISTS idx_catalog_items_case_communication_messages_external_id
    ON catalog_items (tenant_id, ref_id, ((data ->> 'external_id')))
    WHERE kind = 'case_communication_message'
      AND data ? 'external_id'
      AND length(trim(coalesce(data ->> 'external_id', ''))) > 0;
