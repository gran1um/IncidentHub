ALTER TABLE user_notification_settings
    ADD COLUMN IF NOT EXISTS notification_email TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS time_recipient TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_user_notification_settings_delivery_channel
    ON user_notification_settings (tenant_id, delivery_enabled, delivery_channel, updated_at DESC);
