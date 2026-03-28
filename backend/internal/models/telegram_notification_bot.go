package models

import (
	"time"

	"github.com/google/uuid"
)

type TelegramNotificationBot struct {
	ID                      uuid.UUID  `json:"id"`
	TenantID                uuid.UUID  `json:"tenant_id"`
	Name                    string     `json:"name"`
	BotToken                string     `json:"-"`
	BotID                   int64      `json:"bot_id"`
	BotUsername             string     `json:"bot_username"`
	BotFirstName            string     `json:"bot_first_name"`
	CanJoinGroups           bool       `json:"can_join_groups"`
	CanReadAllGroupMessages bool       `json:"can_read_all_group_messages"`
	SupportsInlineQueries   bool       `json:"supports_inline_queries"`
	Enabled                 bool       `json:"enabled"`
	CreatedBy               *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
}
