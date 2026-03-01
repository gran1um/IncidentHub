-- Seed security-oriented outbound connectors (HTTP-based) and example methods.
-- These are global templates; tenants can clone or override them via the UI.

INSERT INTO catalog_items (id, tenant_id, kind, data)
VALUES
    (
      'a4f327d9-3fb8-497b-b287-1a432463127f',
      NULL,
      'outbound_connectors',
      '{
        "name": "VirusTotal API",
        "description": "Threat enrichment via VirusTotal HTTP API (configure API key before use)",
        "type": "HTTP",
        "direction": "outbound",
        "channel": "webhook",
        "enabled": true,
        "category": "security",
        "config": {
          "baseUrl": "https://www.virustotal.com/api/v3",
          "authType": "api_key",
          "apiKey": "",
          "defaultHeaders": {
            "Accept": "application/json"
          }
        }
      }'::jsonb
    ),
    (
      '5e17ad64-4e49-4d08-aba9-42e1855ab1ec',
      NULL,
      'outbound_connectors',
      '{
        "name": "Microsoft 365 / Outlook",
        "description": "Send investigation emails via Microsoft Graph (configure bearer token before use)",
        "type": "HTTP",
        "direction": "outbound",
        "channel": "webhook",
        "enabled": true,
        "category": "security",
        "config": {
          "baseUrl": "https://graph.microsoft.com/v1.0",
          "authType": "bearer",
          "bearerToken": ""
        }
      }'::jsonb
    )
ON CONFLICT (id) DO NOTHING;

-- Example VirusTotal methods: hash and URL reputation.
INSERT INTO catalog_items (id, tenant_id, kind, ref_id, data)
VALUES
    (
      '09156da0-4d4f-4efc-9f56-bae4fd8d6d38',
      NULL,
      'connector_methods',
      'a4f327d9-3fb8-497b-b287-1a432463127f',
      '{
        "name": "Hash reputation (VirusTotal)",
        "action": "scan_hash",
        "observable_types": ["hash"],
        "message_template": "Lookup hash={{input.hash}} on VirusTotal",
        "metadata_template": {
          "target": "{{input.hash}}",
          "kind": "hash"
        }
      }'::jsonb
    ),
    (
      'ef170296-8880-41a8-b238-8f787ae452d9',
      NULL,
      'connector_methods',
      'a4f327d9-3fb8-497b-b287-1a432463127f',
      '{
        "name": "URL reputation (VirusTotal)",
        "action": "scan_url",
        "observable_types": ["url"],
        "message_template": "Lookup url={{input.url}} on VirusTotal",
        "metadata_template": {
          "target": "{{input.url}}",
          "kind": "url"
        }
      }'::jsonb
    )
ON CONFLICT (id) DO NOTHING;

-- Example Outlook method: send investigation summary email.
INSERT INTO catalog_items (id, tenant_id, kind, ref_id, data)
VALUES
    (
      '99052239-283c-40e7-a8a3-43f2f3528688',
      NULL,
      'connector_methods',
      '5e17ad64-4e49-4d08-aba9-42e1855ab1ec',
      '{
        "name": "Send investigation email",
        "action": "send_email",
        "observable_types": ["email"],
        "message_template": "Case {{case_id}} — {{input.subject}}",
        "metadata_template": {
          "to": "{{input.to}}",
          "subject": "{{input.subject}}",
          "body": "{{input.body}}"
        }
      }'::jsonb
    )
ON CONFLICT (id) DO NOTHING;
