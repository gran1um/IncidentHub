package repository

import (
	"context"
	"fmt"
	"strings"

	"incidenthub/backend/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserNotificationSettingsRepository struct {
	pool *pgxpool.Pool
}

type UpsertUserNotificationSettingsParams struct {
	TenantID          uuid.UUID
	UserID            uuid.UUID
	DeliveryEnabled   bool
	DeliveryChannel   string
	TelegramBotID     *uuid.UUID
	TelegramChatID    string
	TelegramUsername  string
	NotificationEmail string
	TimeRecipient     string
	UpdatedBy         *uuid.UUID
}

func NewUserNotificationSettingsRepository(pool *pgxpool.Pool) *UserNotificationSettingsRepository {
	return &UserNotificationSettingsRepository{pool: pool}
}

func (r *UserNotificationSettingsRepository) GetByUser(ctx context.Context, tenantID, userID uuid.UUID) (*models.UserNotificationSettings, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT
			id, tenant_id, user_id, delivery_enabled, delivery_channel,
			telegram_bot_id, telegram_chat_id, telegram_username, notification_email, time_recipient, updated_by, created_at, updated_at
		FROM user_notification_settings
		WHERE tenant_id = $1
		  AND user_id = $2
	`, tenantID, userID)
	item, err := scanUserNotificationSettings(row)
	if err != nil {
		return nil, fmt.Errorf("get user notification settings: %w", err)
	}
	return item, nil
}

func (r *UserNotificationSettingsRepository) Upsert(ctx context.Context, p UpsertUserNotificationSettingsParams) (*models.UserNotificationSettings, error) {
	channel := strings.ToLower(strings.TrimSpace(p.DeliveryChannel))
	if channel == "" {
		channel = "in_app"
	}
	telegramChatID := strings.TrimSpace(p.TelegramChatID)
	telegramUsername := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(strings.TrimSpace(p.TelegramUsername)), "@"))
	notificationEmail := strings.TrimSpace(strings.ToLower(p.NotificationEmail))
	timeRecipient := strings.TrimSpace(p.TimeRecipient)

	row := r.pool.QueryRow(ctx, `
		INSERT INTO user_notification_settings(
			tenant_id, user_id, delivery_enabled, delivery_channel,
			telegram_bot_id, telegram_chat_id, telegram_username, notification_email, time_recipient, updated_by
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (tenant_id, user_id)
		DO UPDATE SET
			delivery_enabled = EXCLUDED.delivery_enabled,
			delivery_channel = EXCLUDED.delivery_channel,
			telegram_bot_id = EXCLUDED.telegram_bot_id,
			telegram_chat_id = EXCLUDED.telegram_chat_id,
			telegram_username = EXCLUDED.telegram_username,
			notification_email = EXCLUDED.notification_email,
			time_recipient = EXCLUDED.time_recipient,
			updated_by = EXCLUDED.updated_by,
			updated_at = NOW()
		RETURNING
			id, tenant_id, user_id, delivery_enabled, delivery_channel,
			telegram_bot_id, telegram_chat_id, telegram_username, notification_email, time_recipient, updated_by, created_at, updated_at
	`,
		p.TenantID,
		p.UserID,
		p.DeliveryEnabled,
		channel,
		p.TelegramBotID,
		telegramChatID,
		telegramUsername,
		notificationEmail,
		timeRecipient,
		p.UpdatedBy,
	)
	item, err := scanUserNotificationSettings(row)
	if err != nil {
		return nil, fmt.Errorf("upsert user notification settings: %w", err)
	}
	return item, nil
}

func (r *UserNotificationSettingsRepository) Delete(ctx context.Context, tenantID, userID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM user_notification_settings
		WHERE tenant_id = $1
		  AND user_id = $2
	`, tenantID, userID)
	if err != nil {
		return fmt.Errorf("delete user notification settings: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *UserNotificationSettingsRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]models.UserNotificationSettings, error) {
	if limit <= 0 || limit > 2000 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := r.pool.Query(ctx, `
		SELECT
			id, tenant_id, user_id, delivery_enabled, delivery_channel,
			telegram_bot_id, telegram_chat_id, telegram_username, notification_email, time_recipient, updated_by, created_at, updated_at
		FROM user_notification_settings
		WHERE tenant_id = $1
		ORDER BY updated_at DESC
		LIMIT $2 OFFSET $3
	`, tenantID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list user notification settings by tenant: %w", err)
	}
	defer rows.Close()

	result := make([]models.UserNotificationSettings, 0, limit)
	for rows.Next() {
		item, scanErr := scanUserNotificationSettings(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan user notification settings: %w", scanErr)
		}
		result = append(result, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user notification settings by tenant: %w", err)
	}
	return result, nil
}

func (r *UserNotificationSettingsRepository) ListDeliveryEnabledByTenant(ctx context.Context, tenantID uuid.UUID, limit int) ([]models.UserNotificationSettings, error) {
	if limit <= 0 || limit > 2000 {
		limit = 300
	}
	rows, err := r.pool.Query(ctx, `
		SELECT
			id, tenant_id, user_id, delivery_enabled, delivery_channel,
			telegram_bot_id, telegram_chat_id, telegram_username, notification_email, time_recipient, updated_by, created_at, updated_at
		FROM user_notification_settings
		WHERE tenant_id = $1
		  AND delivery_enabled = TRUE
		  AND (
			(
			  lower(trim(delivery_channel)) = 'telegram'
			  AND telegram_bot_id IS NOT NULL
			  AND (trim(telegram_chat_id) <> '' OR trim(telegram_username) <> '')
			)
			OR (
			  lower(trim(delivery_channel)) = 'email'
			  AND trim(notification_email) <> ''
			)
			OR (
			  lower(trim(delivery_channel)) = 'time'
			  AND trim(time_recipient) <> ''
			)
		  )
		ORDER BY updated_at DESC
		LIMIT $2
	`, tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("list delivery-enabled notification settings: %w", err)
	}
	defer rows.Close()

	result := make([]models.UserNotificationSettings, 0, limit)
	for rows.Next() {
		item, scanErr := scanUserNotificationSettings(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan user notification settings: %w", scanErr)
		}
		result = append(result, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user notification settings: %w", err)
	}
	return result, nil
}

type userNotificationSettingsScanner interface {
	Scan(dest ...any) error
}

func scanUserNotificationSettings(scanner userNotificationSettingsScanner) (*models.UserNotificationSettings, error) {
	var item models.UserNotificationSettings
	if err := scanner.Scan(
		&item.ID,
		&item.TenantID,
		&item.UserID,
		&item.DeliveryEnabled,
		&item.DeliveryChannel,
		&item.TelegramBotID,
		&item.TelegramChatID,
		&item.TelegramUsername,
		&item.NotificationEmail,
		&item.TimeRecipient,
		&item.UpdatedBy,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &item, nil
}
