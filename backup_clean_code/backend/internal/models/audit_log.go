package models

import (
	"time"

	"github.com/google/uuid"
)

type AuditLog struct {
	ID         uuid.UUID      `json:"id"`
	TenantID   *uuid.UUID     `json:"tenant_id,omitempty"`
	ActorID    *uuid.UUID     `json:"actor_id,omitempty"`
	Action     string         `json:"action"`
	ObjectType string         `json:"object_type"`
	ObjectID   *uuid.UUID     `json:"object_id,omitempty"`
	Details    map[string]any `json:"details"`
	CreatedAt  time.Time      `json:"created_at"`
}
