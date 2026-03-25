package models

import (
	"time"

	"github.com/google/uuid"
)

type AIChatMessage struct {
	ID        uuid.UUID        `json:"id"`
	SessionID uuid.UUID        `json:"session_id"`
	TenantID  uuid.UUID        `json:"tenant_id"`
	UserID    uuid.UUID        `json:"user_id"`
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	Sources   []map[string]any `json:"sources"`
	Metadata  map[string]any   `json:"metadata"`
	CreatedAt time.Time        `json:"created_at"`
}
