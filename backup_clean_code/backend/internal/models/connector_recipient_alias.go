package models

import (
	"time"

	"github.com/google/uuid"
)

type ConnectorRecipientAlias struct {
	ID          uuid.UUID      `json:"id"`
	TenantID    uuid.UUID      `json:"tenant_id"`
	ConnectorID uuid.UUID      `json:"connector_id"`
	Channel     string         `json:"channel"`
	RecipientID string         `json:"recipient_id"`
	AliasType   string         `json:"alias_type"`
	AliasValue  string         `json:"alias_value"`
	DisplayName string         `json:"display_name"`
	Metadata    map[string]any `json:"metadata"`
	LastSeenAt  time.Time      `json:"last_seen_at"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}
