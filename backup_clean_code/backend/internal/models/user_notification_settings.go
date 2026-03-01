package models

import (
	"time"

	"github.com/google/uuid"
)

type UserNotificationSettings struct {
	ID                uuid.UUID  `json:"id"`
	TenantID          uuid.UUID  `json:"tenant_id"`
	UserID            uuid.UUID  `json:"user_id"`
	DeliveryEnabled   bool       `json:"delivery_enabled"`
	DeliveryChannel   string     `json:"delivery_channel"`
	TelegramBotID     *uuid.UUID `json:"telegram_bot_id,omitempty"`
	TelegramChatID    string     `json:"telegram_chat_id"`
	TelegramUsername  string     `json:"telegram_username"`
	NotificationEmail string     `json:"notification_email"`
	TimeRecipient     string     `json:"time_recipient"`
	UpdatedBy         *uuid.UUID `json:"updated_by,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}
