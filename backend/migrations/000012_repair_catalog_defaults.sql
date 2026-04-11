INSERT INTO catalog_items (id, tenant_id, kind, data)
VALUES
    ('08b31f33-1df5-4ccf-84a8-0c73a36e0011', NULL, 'observable_types', '{"id":"ip","name":"IP Address","dataType":"string","validatorRegex":"^(?:[0-9]{1,3}\\.){3}[0-9]{1,3}$","tenantId":"global"}'::jsonb),
    ('08b31f33-1df5-4ccf-84a8-0c73a36e0012', NULL, 'observable_types', '{"id":"domain","name":"Domain Name","dataType":"string","validatorRegex":"^([a-z0-9]+(-[a-z0-9]+)*\\.)+[a-z]{2,}$","tenantId":"global"}'::jsonb),
    ('08b31f33-1df5-4ccf-84a8-0c73a36e0013', NULL, 'observable_types', '{"id":"hash_md5","name":"MD5 Hash","dataType":"string","validatorRegex":"^[a-f0-9]{32}$","tenantId":"global"}'::jsonb),
    ('08b31f33-1df5-4ccf-84a8-0c73a36e0014', NULL, 'observable_types', '{"id":"hash_sha256","name":"SHA256 Hash","dataType":"string","validatorRegex":"^[a-f0-9]{64}$","tenantId":"global"}'::jsonb),
    ('08b31f33-1df5-4ccf-84a8-0c73a36e0015', NULL, 'observable_types', '{"id":"email","name":"Email Address","dataType":"string","validatorRegex":"^[\\w-\\.]+@([\\w-]+\\.)+[\\w-]{2,4}$","tenantId":"global"}'::jsonb),
    ('08b31f33-1df5-4ccf-84a8-0c73a36e0016', NULL, 'observable_types', '{"id":"user_agent","name":"User Agent","dataType":"string","tenantId":"global"}'::jsonb)
ON CONFLICT (id) DO NOTHING;

INSERT INTO catalog_items (id, tenant_id, kind, data)
VALUES
    ('19c47a20-404d-4d58-8ce2-3d2c8dd50001', NULL, 'achievements', '{"name":"First Responder","description":"Closed the first incident case","icon":"🛡️","rarity":"Common"}'::jsonb),
    ('19c47a20-404d-4d58-8ce2-3d2c8dd50002', NULL, 'achievements', '{"name":"Night Watch","description":"Handled incidents in three overnight shifts","icon":"🌙","rarity":"Rare"}'::jsonb),
    ('19c47a20-404d-4d58-8ce2-3d2c8dd50003', NULL, 'achievements', '{"name":"IOC Hunter","description":"Added 100 validated observables","icon":"🎯","rarity":"Rare"}'::jsonb),
    ('19c47a20-404d-4d58-8ce2-3d2c8dd50004', NULL, 'achievements', '{"name":"Containment Master","description":"Contained five critical incidents","icon":"🚧","rarity":"Epic"}'::jsonb),
    ('19c47a20-404d-4d58-8ce2-3d2c8dd50005', NULL, 'achievements', '{"name":"Intel Curator","description":"Published 50 high-confidence threat notes","icon":"📚","rarity":"Epic"}'::jsonb),
    ('19c47a20-404d-4d58-8ce2-3d2c8dd50006', NULL, 'achievements', '{"name":"SOC Architect","description":"Designed and deployed a detection playbook","icon":"🏗️","rarity":"Legendary"}'::jsonb),
    ('19c47a20-404d-4d58-8ce2-3d2c8dd50007', NULL, 'achievements', '{"name":"Incident Commander","description":"Led ten major incident bridges","icon":"🎖️","rarity":"Legendary"}'::jsonb),
    ('19c47a20-404d-4d58-8ce2-3d2c8dd50008', NULL, 'achievements', '{"name":"Zero-Day Sentinel","description":"Detected and stopped a zero-day campaign","icon":"💎","rarity":"Diamond"}'::jsonb)
ON CONFLICT (id) DO NOTHING;
