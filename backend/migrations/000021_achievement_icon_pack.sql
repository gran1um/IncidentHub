UPDATE catalog_items
SET data = data || jsonb_build_object('icon', '/achievement-icons/first-responder.svg', 'xp_reward', 150)
WHERE id = '19c47a20-404d-4d58-8ce2-3d2c8dd50001'
  AND kind = 'achievements'
  AND tenant_id IS NULL;

UPDATE catalog_items
SET data = data || jsonb_build_object('icon', '/achievement-icons/night-watch.svg', 'xp_reward', 220)
WHERE id = '19c47a20-404d-4d58-8ce2-3d2c8dd50002'
  AND kind = 'achievements'
  AND tenant_id IS NULL;

UPDATE catalog_items
SET data = data || jsonb_build_object('icon', '/achievement-icons/ioc-hunter.svg', 'xp_reward', 260)
WHERE id = '19c47a20-404d-4d58-8ce2-3d2c8dd50003'
  AND kind = 'achievements'
  AND tenant_id IS NULL;

UPDATE catalog_items
SET data = data || jsonb_build_object('icon', '/achievement-icons/containment-master.svg', 'xp_reward', 320)
WHERE id = '19c47a20-404d-4d58-8ce2-3d2c8dd50004'
  AND kind = 'achievements'
  AND tenant_id IS NULL;

UPDATE catalog_items
SET data = data || jsonb_build_object('icon', '/achievement-icons/intel-curator.svg', 'xp_reward', 340)
WHERE id = '19c47a20-404d-4d58-8ce2-3d2c8dd50005'
  AND kind = 'achievements'
  AND tenant_id IS NULL;

UPDATE catalog_items
SET data = data || jsonb_build_object('icon', '/achievement-icons/soc-architect.svg', 'xp_reward', 420)
WHERE id = '19c47a20-404d-4d58-8ce2-3d2c8dd50006'
  AND kind = 'achievements'
  AND tenant_id IS NULL;

UPDATE catalog_items
SET data = data || jsonb_build_object('icon', '/achievement-icons/incident-commander.svg', 'xp_reward', 500)
WHERE id = '19c47a20-404d-4d58-8ce2-3d2c8dd50007'
  AND kind = 'achievements'
  AND tenant_id IS NULL;

UPDATE catalog_items
SET data = data || jsonb_build_object('icon', '/achievement-icons/zero-day-sentinel.svg', 'xp_reward', 650)
WHERE id = '19c47a20-404d-4d58-8ce2-3d2c8dd50008'
  AND kind = 'achievements'
  AND tenant_id IS NULL;

INSERT INTO catalog_items (id, tenant_id, kind, data)
VALUES
  (
    '19c47a20-404d-4d58-8ce2-3d2c8dd50009',
    NULL,
    'achievements',
    '{"name":"Playbook Automator","description":"Built and shipped five production playbooks.","icon":"/achievement-icons/playbook-automator.svg","rarity":"Epic","xp_reward":380}'::jsonb
  ),
  (
    '19c47a20-404d-4d58-8ce2-3d2c8dd50010',
    NULL,
    'achievements',
    '{"name":"Telemetry Guardian","description":"Maintained clean telemetry ingestion for 30 days.","icon":"/achievement-icons/telemetry-guardian.svg","rarity":"Legendary","xp_reward":460}'::jsonb
  )
ON CONFLICT (id) DO NOTHING;
