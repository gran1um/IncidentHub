ALTER TABLE forum_external_bindings
    ADD COLUMN IF NOT EXISTS binding_key TEXT NOT NULL DEFAULT '';

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'forum_external_bindings_tenant_id_thread_id_connector_id_key'
    ) THEN
        ALTER TABLE forum_external_bindings
            DROP CONSTRAINT forum_external_bindings_tenant_id_thread_id_connector_id_key;
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'forum_external_bindings_tenant_thread_connector_binding_key_key'
    ) THEN
        ALTER TABLE forum_external_bindings
            ADD CONSTRAINT forum_external_bindings_tenant_thread_connector_binding_key_key
            UNIQUE (tenant_id, thread_id, connector_id, binding_key);
    END IF;
END $$;
