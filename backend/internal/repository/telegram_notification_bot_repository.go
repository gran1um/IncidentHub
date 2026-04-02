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

type TelegramNotificationBotRepository struct {
	pool *pgxpool.Pool
}

type CreateTelegramNotificationBotParams struct {
	TenantID                uuid.UUID
	Name                    string
	BotToken                string
	BotID                   int64
	BotUsername             string
	BotFirstName            string
	CanJoinGroups           bool
	CanReadAllGroupMessages bool
	SupportsInlineQueries   bool
	Enabled                 bool
	CreatedBy               *uuid.UUID
}

type UpdateTelegramNotificationBotParams struct {
	Name                    *string
	BotToken                *string
	BotID                   *int64
	BotUsername             *string
	BotFirstName            *string
	CanJoinGroups           *bool
	CanReadAllGroupMessages *bool
	SupportsInlineQueries   *bool
	Enabled                 *bool
}

func NewTelegramNotificationBotRepository(pool *pgxpool.Pool) *TelegramNotificationBotRepository {
	return &TelegramNotificationBotRepository{pool: pool}
}

func (r *TelegramNotificationBotRepository) Create(ctx context.Context, p CreateTelegramNotificationBotParams) (*models.TelegramNotificationBot, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO telegram_notification_bots(
			tenant_id, name, bot_token, bot_id, bot_username, bot_first_name,
			can_join_groups, can_read_all_group_messages, supports_inline_queries,
			enabled, created_by
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING
			id, tenant_id, name, bot_token, bot_id, bot_username, bot_first_name,
			can_join_groups, can_read_all_group_messages, supports_inline_queries,
			enabled, created_by, created_at, updated_at
	`,
		p.TenantID,
		strings.TrimSpace(p.Name),
		strings.TrimSpace(p.BotToken),
		p.BotID,
		strings.TrimSpace(strings.TrimPrefix(strings.ToLower(strings.TrimSpace(p.BotUsername)), "@")),
		strings.TrimSpace(p.BotFirstName),
		p.CanJoinGroups,
		p.CanReadAllGroupMessages,
		p.SupportsInlineQueries,
		p.Enabled,
		p.CreatedBy,
	)
	item, err := scanTelegramNotificationBot(row)
	if err != nil {
		return nil, fmt.Errorf("create telegram notification bot: %w", err)
	}
	return item, nil
}

func (r *TelegramNotificationBotRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID, includeDisabled bool, limit int) ([]models.TelegramNotificationBot, error) {
	return listByTenantWithOptionalClause(
		ctx,
		r.pool,
		`
			SELECT
				id, tenant_id, name, bot_token, bot_id, bot_username, bot_first_name,
				can_join_groups, can_read_all_group_messages, supports_inline_queries,
				enabled, created_by, created_at, updated_at
			FROM telegram_notification_bots
			WHERE tenant_id = $1
		`,
		tenantID,
		includeDisabled,
		"AND enabled = TRUE",
		"ORDER BY updated_at DESC LIMIT $2",
		limit,
		2000,
		300,
		scanTelegramNotificationBot,
		"list telegram notification bots",
		"scan telegram notification bot",
		"iterate telegram notification bots",
	)
}

func (r *TelegramNotificationBotRepository) GetByID(ctx context.Context, tenantID, botID uuid.UUID) (*models.TelegramNotificationBot, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT
			id, tenant_id, name, bot_token, bot_id, bot_username, bot_first_name,
			can_join_groups, can_read_all_group_messages, supports_inline_queries,
			enabled, created_by, created_at, updated_at
		FROM telegram_notification_bots
		WHERE tenant_id = $1
		  AND id = $2
	`, tenantID, botID)
	item, err := scanTelegramNotificationBot(row)
	if err != nil {
		return nil, fmt.Errorf("get telegram notification bot: %w", err)
	}
	return item, nil
}

func (r *TelegramNotificationBotRepository) Update(ctx context.Context, tenantID, botID uuid.UUID, p UpdateTelegramNotificationBotParams) (*models.TelegramNotificationBot, error) {
	name := nullableTrimmedString(p.Name)
	token := nullableTrimmedString(p.BotToken)
	username := nullableNormalizedUsername(p.BotUsername)
	firstName := nullableTrimmedString(p.BotFirstName)

	row := r.pool.QueryRow(ctx, `
		UPDATE telegram_notification_bots
		SET
			name = COALESCE($3, name),
			bot_token = COALESCE($4, bot_token),
			bot_id = COALESCE($5, bot_id),
			bot_username = COALESCE($6, bot_username),
			bot_first_name = COALESCE($7, bot_first_name),
			can_join_groups = COALESCE($8, can_join_groups),
			can_read_all_group_messages = COALESCE($9, can_read_all_group_messages),
			supports_inline_queries = COALESCE($10, supports_inline_queries),
			enabled = COALESCE($11, enabled),
			updated_at = NOW()
		WHERE tenant_id = $1
		  AND id = $2
		RETURNING
			id, tenant_id, name, bot_token, bot_id, bot_username, bot_first_name,
			can_join_groups, can_read_all_group_messages, supports_inline_queries,
			enabled, created_by, created_at, updated_at
	`,
		tenantID,
		botID,
		name,
		token,
		p.BotID,
		username,
		firstName,
		p.CanJoinGroups,
		p.CanReadAllGroupMessages,
		p.SupportsInlineQueries,
		p.Enabled,
	)
	item, err := scanTelegramNotificationBot(row)
	if err != nil {
		return nil, fmt.Errorf("update telegram notification bot: %w", err)
	}
	return item, nil
}

func (r *TelegramNotificationBotRepository) Delete(ctx context.Context, tenantID, botID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM telegram_notification_bots
		WHERE tenant_id = $1
		  AND id = $2
	`, tenantID, botID)
	if err != nil {
		return fmt.Errorf("delete telegram notification bot: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func scanTelegramNotificationBot(scanner rowScanner) (*models.TelegramNotificationBot, error) {
	var item models.TelegramNotificationBot
	if err := scanner.Scan(
		&item.ID,
		&item.TenantID,
		&item.Name,
		&item.BotToken,
		&item.BotID,
		&item.BotUsername,
		&item.BotFirstName,
		&item.CanJoinGroups,
		&item.CanReadAllGroupMessages,
		&item.SupportsInlineQueries,
		&item.Enabled,
		&item.CreatedBy,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &item, nil
}

func nullableTrimmedString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	return &trimmed
}

func nullableNormalizedUsername(value *string) *string {
	if value == nil {
		return nil
	}
	normalized := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(strings.TrimSpace(*value)), "@"))
	return &normalized
}
