package models

import (
	"time"

	"github.com/google/uuid"
)

type AIChatSession struct {
	ID                 uuid.UUID `json:"id"`
	TenantID           uuid.UUID `json:"tenant_id"`
	UserID             uuid.UUID `json:"user_id"`
	Title              string    `json:"title"`
	IsDefault          bool      `json:"is_default"`
	SortOrder          int64     `json:"sort_order"`
	LastMessagePreview string    `json:"last_message_preview"`
	LastMessageRole    string    `json:"last_message_role"`
	LastMessageAt      time.Time `json:"last_message_at"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}
