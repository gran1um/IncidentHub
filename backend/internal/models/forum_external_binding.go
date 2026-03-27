package models

import (
	"time"

	"github.com/google/uuid"
)

type ForumExternalBinding struct {
	ID             uuid.UUID      `json:"id"`
	TenantID       uuid.UUID      `json:"tenant_id"`
	ThreadID       uuid.UUID      `json:"thread_id"`
	ConnectorID    uuid.UUID      `json:"connector_id"`
	BindingKey     string         `json:"binding_key"`
	ConversationID string         `json:"conversation_id"`
	Cursor         string         `json:"cursor"`
	Metadata       map[string]any `json:"metadata"`
	LastSyncedAt   *time.Time     `json:"last_synced_at,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}
