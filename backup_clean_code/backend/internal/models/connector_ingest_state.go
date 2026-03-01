package models

import (
	"time"

	"github.com/google/uuid"
)

type ConnectorIngestState struct {
	ID          uuid.UUID  `json:"id"`
	TenantID    uuid.UUID  `json:"tenant_id"`
	ConnectorID uuid.UUID  `json:"connector_id"`
	ExternalID  string     `json:"external_id"`
	PayloadHash string     `json:"payload_hash"`
	AlertID     *uuid.UUID `json:"alert_id,omitempty"`
	FirstSeenAt time.Time  `json:"first_seen_at"`
	LastSeenAt  time.Time  `json:"last_seen_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
