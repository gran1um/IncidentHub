INSERT INTO catalog_items (id, tenant_id, kind, data)
VALUES
    (
        '28b31f33-1df5-4ccf-84a8-0c73a36e0101',
        NULL,
        'connector_templates',
        $$
        {
          "name": "HTTP API",
          "description": "Generic HTTP and webhook connector for external APIs.",
          "type": "HTTP",
          "channel": "webhook",
          "category": "standard",
          "communication_mode": "",
          "capabilities": ["hub_execute"],
          "tags": ["http", "api"],
          "icon": "globe",
          "config_schema": [
            {"id": "baseUrl", "label": "Base URL", "type": "text", "required": true, "target_path": "config.baseUrl"},
            {"id": "defaultHeaders", "label": "Default headers", "type": "json", "required": false, "target_path": "config.defaultHeaders"},
            {"id": "authType", "label": "Auth type", "type": "select", "required": false, "default_value": "none", "target_path": "config.authType", "options": [{"label": "None", "value": "none"}, {"label": "API key", "value": "api_key"}, {"label": "Bearer", "value": "bearer"}, {"label": "Basic", "value": "basic"}]}
          ],
          "forms": {
            "manual_run": [
              {"id": "method", "label": "HTTP method", "type": "select", "required": true, "default_value": "GET", "target_path": "metadata.method", "options": [{"label": "GET", "value": "GET"}, {"label": "POST", "value": "POST"}, {"label": "PUT", "value": "PUT"}, {"label": "PATCH", "value": "PATCH"}, {"label": "DELETE", "value": "DELETE"}]},
              {"id": "path", "label": "Path", "type": "text", "required": true, "target_path": "metadata.path"},
              {"id": "body", "label": "Body", "type": "json", "required": false, "target_path": "input.body"}
            ],
            "forum_send": [],
            "case_communication": []
          }
        }
        $$::jsonb
    ),
    (
        '28b31f33-1df5-4ccf-84a8-0c73a36e0102',
        NULL,
        'connector_templates',
        $$
        {
          "name": "SQL / Postgres",
          "description": "Read-only database connector for enrichment queries.",
          "type": "SQL",
          "channel": "sql",
          "category": "infra",
          "communication_mode": "",
          "capabilities": ["hub_execute"],
          "tags": ["sql", "postgres"],
          "icon": "database",
          "config_schema": [
            {"id": "engine", "label": "Engine", "type": "select", "required": true, "default_value": "postgres", "target_path": "config.engine", "options": [{"label": "Postgres", "value": "postgres"}, {"label": "ClickHouse", "value": "clickhouse"}, {"label": "Cassandra", "value": "cassandra"}]},
            {"id": "host", "label": "Host", "type": "text", "required": false, "target_path": "config.host"},
            {"id": "port", "label": "Port", "type": "text", "required": false, "default_value": "5432", "target_path": "config.port"},
            {"id": "database", "label": "Database", "type": "text", "required": false, "target_path": "config.database"},
            {"id": "username", "label": "Username", "type": "text", "required": false, "target_path": "config.username"},
            {"id": "password", "label": "Password", "type": "secret", "required": false, "target_path": "config.password"},
            {"id": "sslMode", "label": "SSL mode", "type": "select", "required": false, "default_value": "disable", "target_path": "config.sslMode", "options": [{"label": "disable", "value": "disable"}, {"label": "require", "value": "require"}, {"label": "verify-full", "value": "verify-full"}]},
            {"id": "dsn", "label": "DSN", "type": "secret", "required": false, "target_path": "config.dsn"}
          ],
          "forms": {
            "manual_run": [
              {"id": "query", "label": "Query", "type": "textarea", "required": true, "target_path": "metadata.query"},
              {"id": "params", "label": "Params", "type": "json", "required": false, "target_path": "metadata.params"}
            ],
            "forum_send": [],
            "case_communication": []
          }
        }
        $$::jsonb
    ),
    (
        '28b31f33-1df5-4ccf-84a8-0c73a36e0103',
        NULL,
        'connector_templates',
        $$
        {
          "name": "S3 Storage",
          "description": "S3 or MinIO object storage connector.",
          "type": "S3",
          "channel": "object_storage",
          "category": "infra",
          "communication_mode": "",
          "capabilities": ["hub_execute"],
          "tags": ["s3", "storage"],
          "icon": "cloud",
          "config_schema": [
            {"id": "endpoint", "label": "Endpoint", "type": "text", "required": true, "target_path": "config.endpoint"},
            {"id": "region", "label": "Region", "type": "text", "required": false, "target_path": "config.region"},
            {"id": "bucket", "label": "Bucket", "type": "text", "required": true, "target_path": "config.bucket"},
            {"id": "accessKeyId", "label": "Access key ID", "type": "secret", "required": true, "target_path": "config.accessKeyId"},
            {"id": "secretAccessKey", "label": "Secret access key", "type": "secret", "required": true, "target_path": "config.secretAccessKey"},
            {"id": "pathStyle", "label": "Path style", "type": "boolean", "required": false, "default_value": false, "target_path": "config.pathStyle"}
          ],
          "forms": {
            "manual_run": [
              {"id": "mode", "label": "Mode", "type": "select", "required": true, "default_value": "get", "target_path": "metadata.mode", "options": [{"label": "Get", "value": "get"}, {"label": "Put", "value": "put"}]},
              {"id": "key", "label": "Object key", "type": "text", "required": true, "target_path": "metadata.key"},
              {"id": "contentType", "label": "Content type", "type": "text", "required": false, "target_path": "metadata.contentType"},
              {"id": "body", "label": "Body", "type": "textarea", "required": false, "target_path": "input.body"}
            ],
            "forum_send": [],
            "case_communication": []
          }
        }
        $$::jsonb
    ),
    (
        '28b31f33-1df5-4ccf-84a8-0c73a36e0104',
        NULL,
        'connector_templates',
        $$
        {
          "name": "Kafka Messaging",
          "description": "Publish messages to Kafka topics.",
          "type": "Kafka",
          "channel": "kafka",
          "category": "infra",
          "communication_mode": "",
          "capabilities": ["hub_execute"],
          "tags": ["kafka", "messaging"],
          "icon": "radio",
          "config_schema": [
            {"id": "brokers", "label": "Brokers", "type": "text", "required": true, "target_path": "config.brokers"},
            {"id": "topic", "label": "Topic", "type": "text", "required": true, "target_path": "config.topic"},
            {"id": "username", "label": "Username", "type": "text", "required": false, "target_path": "config.username"},
            {"id": "password", "label": "Password", "type": "secret", "required": false, "target_path": "config.password"},
            {"id": "useTls", "label": "Use TLS", "type": "boolean", "required": false, "default_value": false, "target_path": "config.useTls"}
          ],
          "forms": {
            "manual_run": [
              {"id": "key", "label": "Message key", "type": "text", "required": false, "target_path": "metadata.key"},
              {"id": "body", "label": "Body", "type": "textarea", "required": true, "target_path": "input.body"}
            ],
            "forum_send": [],
            "case_communication": []
          }
        }
        $$::jsonb
    ),
    (
        '28b31f33-1df5-4ccf-84a8-0c73a36e0105',
        NULL,
        'connector_templates',
        $$
        {
          "name": "Redis",
          "description": "Push payloads to Redis lists or streams.",
          "type": "Redis",
          "channel": "redis",
          "category": "infra",
          "communication_mode": "",
          "capabilities": ["hub_execute"],
          "tags": ["redis", "queue"],
          "icon": "database",
          "config_schema": [
            {"id": "address", "label": "Address", "type": "text", "required": true, "target_path": "config.address"},
            {"id": "password", "label": "Password", "type": "secret", "required": false, "target_path": "config.password"},
            {"id": "db", "label": "DB index", "type": "text", "required": false, "default_value": "0", "target_path": "config.db"},
            {"id": "list", "label": "List or stream", "type": "text", "required": true, "target_path": "config.list"},
            {"id": "useTls", "label": "Use TLS", "type": "boolean", "required": false, "default_value": false, "target_path": "config.useTls"}
          ],
          "forms": {
            "manual_run": [
              {"id": "body", "label": "Body", "type": "textarea", "required": true, "target_path": "input.body"}
            ],
            "forum_send": [],
            "case_communication": []
          }
        }
        $$::jsonb
    ),
    (
        '28b31f33-1df5-4ccf-84a8-0c73a36e0106',
        NULL,
        'connector_templates',
        $$
        {
          "name": "Telegram Chat",
          "description": "Telegram bot connector for case and forum conversations.",
          "type": "Telegram",
          "channel": "telegram",
          "category": "notifications",
          "communication_mode": "chat",
          "capabilities": ["hub_execute", "case_communications", "forum_threads", "sync_messages"],
          "tags": ["telegram", "chat"],
          "icon": "message-square",
          "config_schema": [
            {"id": "botToken", "label": "Bot token", "type": "secret", "required": true, "target_path": "config.botToken"},
            {"id": "chatId", "label": "Default chat ID", "type": "text", "required": false, "target_path": "config.chatId"},
            {"id": "username", "label": "Default username", "type": "text", "required": false, "target_path": "config.username"}
          ],
          "forms": {
            "manual_run": [
              {"id": "chatId", "label": "Chat ID", "type": "text", "required": true, "target_path": "metadata.chatId"},
              {"id": "username", "label": "Username", "type": "text", "required": false, "target_path": "metadata.username"}
            ],
            "forum_send": [
              {"id": "chatId", "label": "Chat ID", "type": "text", "required": true, "target_path": "metadata.chatId"},
              {"id": "username", "label": "Username", "type": "text", "required": false, "target_path": "metadata.username"}
            ],
            "case_communication": [
              {"id": "chatId", "label": "Chat ID", "type": "text", "required": true, "target_path": "metadata.chatId"},
              {"id": "username", "label": "Username", "type": "text", "required": false, "target_path": "metadata.username"}
            ]
          }
        }
        $$::jsonb
    ),
    (
        '28b31f33-1df5-4ccf-84a8-0c73a36e0107',
        NULL,
        'connector_templates',
        $$
        {
          "name": "Slack Chat",
          "description": "Slack Web API bot connector with thread sync support.",
          "type": "Slack",
          "channel": "slack",
          "category": "notifications",
          "communication_mode": "chat",
          "capabilities": ["hub_execute", "case_communications", "forum_threads", "sync_messages"],
          "tags": ["slack", "chat"],
          "icon": "message-square",
          "config_schema": [
            {"id": "botToken", "label": "Bot token", "type": "secret", "required": true, "target_path": "config.botToken"},
            {"id": "channelId", "label": "Default channel ID", "type": "text", "required": false, "target_path": "config.channelId"},
            {"id": "threadTs", "label": "Default thread TS", "type": "text", "required": false, "target_path": "config.threadTs"}
          ],
          "forms": {
            "manual_run": [
              {"id": "chatId", "label": "Channel ID", "type": "text", "required": true, "target_path": "metadata.channelId"},
              {"id": "externalUserId", "label": "Thread TS", "type": "text", "required": false, "target_path": "metadata.threadTs"}
            ],
            "forum_send": [
              {"id": "chatId", "label": "Channel ID", "type": "text", "required": true, "target_path": "metadata.channelId"},
              {"id": "externalUserId", "label": "Thread TS", "type": "text", "required": false, "target_path": "metadata.threadTs"}
            ],
            "case_communication": [
              {"id": "chatId", "label": "Channel ID", "type": "text", "required": true, "target_path": "metadata.channelId"},
              {"id": "externalUserId", "label": "Thread TS", "type": "text", "required": false, "target_path": "metadata.threadTs"}
            ]
          }
        }
        $$::jsonb
    ),
    (
        '28b31f33-1df5-4ccf-84a8-0c73a36e0108',
        NULL,
        'connector_templates',
        $$
        {
          "name": "Outlook Mail",
          "description": "Microsoft Graph mail connector for two-way case communications.",
          "type": "Outlook",
          "channel": "outlook",
          "category": "notifications",
          "communication_mode": "email",
          "capabilities": ["hub_execute", "case_communications", "forum_threads", "sync_messages"],
          "tags": ["outlook", "mail", "graph"],
          "icon": "mail",
          "config_schema": [
            {"id": "tenantId", "label": "Tenant ID", "type": "text", "required": true, "target_path": "config.tenantId"},
            {"id": "clientId", "label": "Client ID", "type": "text", "required": true, "target_path": "config.clientId"},
            {"id": "clientSecret", "label": "Client secret", "type": "secret", "required": true, "target_path": "config.clientSecret"},
            {"id": "mailbox", "label": "Mailbox", "type": "text", "required": true, "target_path": "config.mailbox"}
          ],
          "forms": {
            "manual_run": [
              {"id": "chatId", "label": "To", "type": "text", "required": true, "target_path": "metadata.to"},
              {"id": "username", "label": "Subject", "type": "text", "required": true, "target_path": "metadata.subject"},
              {"id": "externalUserId", "label": "CC", "type": "text", "required": false, "target_path": "metadata.cc"}
            ],
            "forum_send": [
              {"id": "chatId", "label": "To", "type": "text", "required": true, "target_path": "metadata.to"},
              {"id": "username", "label": "Subject", "type": "text", "required": true, "target_path": "metadata.subject"},
              {"id": "externalUserId", "label": "CC", "type": "text", "required": false, "target_path": "metadata.cc"}
            ],
            "case_communication": [
              {"id": "chatId", "label": "To", "type": "text", "required": true, "target_path": "metadata.to"},
              {"id": "username", "label": "Subject", "type": "text", "required": true, "target_path": "metadata.subject"},
              {"id": "externalUserId", "label": "CC", "type": "text", "required": false, "target_path": "metadata.cc"}
            ]
          }
        }
        $$::jsonb
    ),
    (
        '28b31f33-1df5-4ccf-84a8-0c73a36e0109',
        NULL,
        'connector_templates',
        $$
        {
          "name": "SMTP Mail",
          "description": "SMTP connector for outbound email delivery.",
          "type": "SMTP",
          "channel": "email",
          "category": "notifications",
          "communication_mode": "email",
          "capabilities": ["hub_execute", "case_communications", "forum_threads", "sync_messages"],
          "tags": ["smtp", "email"],
          "icon": "mail",
          "config_schema": [
            {"id": "endpoint", "label": "Endpoint", "type": "text", "required": true, "target_path": "config.endpoint"},
            {"id": "from", "label": "From", "type": "text", "required": true, "target_path": "config.from"},
            {"id": "username", "label": "Username", "type": "text", "required": false, "target_path": "config.username"},
            {"id": "password", "label": "Password", "type": "secret", "required": false, "target_path": "config.password"}
          ],
          "forms": {
            "manual_run": [
              {"id": "chatId", "label": "To", "type": "text", "required": true, "target_path": "metadata.to"},
              {"id": "username", "label": "Subject", "type": "text", "required": true, "target_path": "metadata.subject"},
              {"id": "externalUserId", "label": "CC", "type": "text", "required": false, "target_path": "metadata.cc"}
            ],
            "forum_send": [
              {"id": "chatId", "label": "To", "type": "text", "required": true, "target_path": "metadata.to"},
              {"id": "username", "label": "Subject", "type": "text", "required": true, "target_path": "metadata.subject"},
              {"id": "externalUserId", "label": "CC", "type": "text", "required": false, "target_path": "metadata.cc"}
            ],
            "case_communication": [
              {"id": "chatId", "label": "To", "type": "text", "required": true, "target_path": "metadata.to"},
              {"id": "username", "label": "Subject", "type": "text", "required": true, "target_path": "metadata.subject"},
              {"id": "externalUserId", "label": "CC", "type": "text", "required": false, "target_path": "metadata.cc"}
            ]
          }
        }
        $$::jsonb
    ),
    (
        '28b31f33-1df5-4ccf-84a8-0c73a36e0110',
        NULL,
        'connector_templates',
        $$
        {
          "name": "Time Chat",
          "description": "Simple chat connector for Time or similar messaging tools.",
          "type": "Time",
          "channel": "time",
          "category": "notifications",
          "communication_mode": "chat",
          "capabilities": ["hub_execute", "case_communications", "sync_messages"],
          "tags": ["time", "chat"],
          "icon": "timer",
          "config_schema": [
            {"id": "endpoint", "label": "Endpoint", "type": "text", "required": true, "target_path": "config.endpoint"},
            {"id": "token", "label": "Token", "type": "secret", "required": true, "target_path": "config.token"},
            {"id": "recipient", "label": "Default recipient", "type": "text", "required": false, "target_path": "config.recipient"}
          ],
          "forms": {
            "manual_run": [
              {"id": "chatId", "label": "Recipient", "type": "text", "required": true, "target_path": "metadata.recipient"}
            ],
            "forum_send": [
              {"id": "chatId", "label": "Recipient", "type": "text", "required": true, "target_path": "metadata.recipient"}
            ],
            "case_communication": [
              {"id": "chatId", "label": "Recipient", "type": "text", "required": true, "target_path": "metadata.recipient"}
            ]
          }
        }
        $$::jsonb
    )
ON CONFLICT (id) DO NOTHING;
