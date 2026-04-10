ALTER TABLE alerts
    ADD COLUMN IF NOT EXISTS case_id UUID REFERENCES cases(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_alerts_tenant_case_updated
    ON alerts(tenant_id, case_id, updated_at DESC);

INSERT INTO catalog_items (id, tenant_id, kind, data)
VALUES
    ('4dcb0d8f-e102-4df8-9a7e-d8d7f4d80011', NULL, 'case_statuses', '{"code":"new","label":"New","order":10,"is_closed":false,"color":"#3b82f6"}'::jsonb),
    ('4dcb0d8f-e102-4df8-9a7e-d8d7f4d80012', NULL, 'case_statuses', '{"code":"open","label":"Open","order":20,"is_closed":false,"color":"#0ea5e9"}'::jsonb),
    ('4dcb0d8f-e102-4df8-9a7e-d8d7f4d80013', NULL, 'case_statuses', '{"code":"resolved","label":"Resolved","order":80,"is_closed":true,"color":"#22c55e"}'::jsonb),
    ('4dcb0d8f-e102-4df8-9a7e-d8d7f4d80014', NULL, 'case_statuses', '{"code":"closed","label":"Closed","order":90,"is_closed":true,"color":"#64748b"}'::jsonb)
ON CONFLICT (id) DO NOTHING;
