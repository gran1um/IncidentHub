package models

import (
	"time"

	"github.com/google/uuid"
)

type AIAgentQueueStatus string

const (
	AIAgentQueueStatusQueued     AIAgentQueueStatus = "queued"
	AIAgentQueueStatusProcessing AIAgentQueueStatus = "processing"
	AIAgentQueueStatusDone       AIAgentQueueStatus = "done"
	AIAgentQueueStatusFailed     AIAgentQueueStatus = "failed"
)

type AIAgentQueueEvent struct {
	ID              uuid.UUID          `json:"id"`
	TenantID        uuid.UUID          `json:"tenant_id"`
	ActorID         *uuid.UUID         `json:"actor_id,omitempty"`
	EntityType      string             `json:"entity_type"`
	EntityID        uuid.UUID          `json:"entity_id"`
	Source          string             `json:"source"`
	Status          AIAgentQueueStatus `json:"status"`
	MatchedAgents   int                `json:"matched_agents"`
	ProcessedAgents int                `json:"processed_agents"`
	AttemptCount    int                `json:"attempt_count"`
	MaxAttempts     int                `json:"max_attempts"`
	WorkflowID      string             `json:"workflow_id"`
	LastError       string             `json:"last_error"`
	ClosedByUser    bool               `json:"closed_by_user"`
	CreatedAt       time.Time          `json:"created_at"`
	StartedAt       *time.Time         `json:"started_at,omitempty"`
	FinishedAt      *time.Time         `json:"finished_at,omitempty"`
	ClosedAt        *time.Time         `json:"closed_at,omitempty"`
	UpdatedAt       time.Time          `json:"updated_at"`
}
