WITH tenant_users AS (
    SELECT
        tm.tenant_id,
        tm.user_id,
        ROW_NUMBER() OVER (PARTITION BY tm.tenant_id ORDER BY tm.created_at ASC) AS rn
    FROM tenant_memberships tm
    WHERE tm.is_active = TRUE
)
INSERT INTO cases(
    tenant_id, case_number, title, description, source, incident_type, status, priority, impact, confidence,
    severity, tlp, pap, resolution_summary, created_by, assigned_to
)
SELECT
    tu.tenant_id,
    'DEMO-0001',
    'Suspicious OAuth Application Consent',
    'User granted consent to an untrusted OAuth application with high-privilege scopes.',
    'siem',
    'identity_compromise',
    'investigating',
    'high',
    'email_access',
    82,
    'high',
    'amber',
    'amber',
    '',
    tu.user_id,
    tu.user_id
FROM tenant_users tu
WHERE tu.rn = 1
  AND NOT EXISTS (
      SELECT 1
      FROM cases c
      WHERE c.tenant_id = tu.tenant_id
        AND c.case_number = 'DEMO-0001'
  );

WITH tenant_users AS (
    SELECT
        tm.tenant_id,
        tm.user_id,
        ROW_NUMBER() OVER (PARTITION BY tm.tenant_id ORDER BY tm.created_at ASC) AS rn
    FROM tenant_memberships tm
    WHERE tm.is_active = TRUE
)
INSERT INTO cases(
    tenant_id, case_number, title, description, source, incident_type, status, priority, impact, confidence,
    severity, tlp, pap, resolution_summary, created_by, assigned_to
)
SELECT
    tu.tenant_id,
    'DEMO-0002',
    'Phishing Campaign with Credential Harvesting',
    'Multiple users reported a phishing email that redirects to a fake SSO page.',
    'mail-gateway',
    'phishing',
    'contained',
    'high',
    'account_takeover',
    89,
    'critical',
    'amber',
    'amber',
    'IOC set blocked on secure mail gateway.',
    tu.user_id,
    tu.user_id
FROM tenant_users tu
WHERE tu.rn = 1
  AND NOT EXISTS (
      SELECT 1
      FROM cases c
      WHERE c.tenant_id = tu.tenant_id
        AND c.case_number = 'DEMO-0002'
  );

WITH tenant_users AS (
    SELECT
        tm.tenant_id,
        tm.user_id,
        ROW_NUMBER() OVER (PARTITION BY tm.tenant_id ORDER BY tm.created_at ASC) AS rn
    FROM tenant_memberships tm
    WHERE tm.is_active = TRUE
)
INSERT INTO cases(
    tenant_id, case_number, title, description, source, incident_type, status, priority, impact, confidence,
    severity, tlp, pap, resolution_summary, created_by, assigned_to
)
SELECT
    tu.tenant_id,
    'DEMO-0003',
    'Endpoint Malware Detection',
    'EDR detected suspicious PowerShell execution with encoded command line.',
    'edr',
    'malware',
    'new',
    'medium',
    'endpoint',
    64,
    'medium',
    'green',
    'amber',
    '',
    tu.user_id,
    tu.user_id
FROM tenant_users tu
WHERE tu.rn = 1
  AND NOT EXISTS (
      SELECT 1
      FROM cases c
      WHERE c.tenant_id = tu.tenant_id
        AND c.case_number = 'DEMO-0003'
  );
