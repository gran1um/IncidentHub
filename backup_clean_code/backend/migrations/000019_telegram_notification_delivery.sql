CREATE TABLE IF NOT EXISTS telegram_notification_bots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    bot_token TEXT NOT NULL,
    bot_id BIGINT NOT NULL,
    bot_username TEXT NOT NULL,
    bot_first_name TEXT NOT NULL DEFAULT '',
    can_join_groups BOOLEAN NOT NULL DEFAULT FALSE,
    can_read_all_group_messages BOOLEAN NOT NULL DEFAULT FALSE,
    supports_inline_queries BOOLEAN NOT NULL DEFAULT FALSE,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, bot_id),
    UNIQUE (tenant_id, bot_username)
);

CREATE INDEX IF NOT EXISTS idx_telegram_notification_bots_tenant_enabled
    ON telegram_notification_bots (tenant_id, enabled, updated_at DESC);

CREATE TABLE IF NOT EXISTS user_notification_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    delivery_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    delivery_channel TEXT NOT NULL DEFAULT 'in_app',
    telegram_bot_id UUID REFERENCES telegram_notification_bots(id) ON DELETE SET NULL,
    telegram_chat_id TEXT NOT NULL DEFAULT '',
    telegram_username TEXT NOT NULL DEFAULT '',
    updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_user_notification_settings_tenant_user
    ON user_notification_settings (tenant_id, user_id);
