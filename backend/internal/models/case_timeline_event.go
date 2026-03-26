package models

import (
	"time"

	"github.com/google/uuid"
)

type CaseTimelineEvent struct {
	ID        uuid.UUID      `json:"id"`
	TenantID  uuid.UUID      `json:"tenant_id"`
	CaseID    uuid.UUID      `json:"case_id"`
	EventType string         `json:"event_type"`
	Title     string         `json:"title"`
	Body      string         `json:"body"`
	ActorID   *uuid.UUID     `json:"actor_id,omitempty"`
	Metadata  map[string]any `json:"metadata"`
	CreatedAt time.Time      `json:"created_at"`
}
