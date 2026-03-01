package models

import (
	"time"

	"github.com/google/uuid"
)

type AIAgentWorkloadStatus string

type AIAgentExecutionPolicy string

const (
	AIAgentWorkloadStatusQueued     AIAgentWorkloadStatus = "queued"
	AIAgentWorkloadStatusProcessing AIAgentWorkloadStatus = "processing"
	AIAgentWorkloadStatusDone       AIAgentWorkloadStatus = "done"
	AIAgentWorkloadStatusFailed     AIAgentWorkloadStatus = "failed"
	AIAgentWorkloadStatusCancelled  AIAgentWorkloadStatus = "canceled"

	AIAgentExecutionPolicyAllMatching   AIAgentExecutionPolicy = "all_matching"
	AIAgentExecutionPolicyExclusive     AIAgentExecutionPolicy = "exclusive"
	AIAgentExecutionPolicyFirstMatch    AIAgentExecutionPolicy = "first_match"
	AIAgentExecutionPolicyFallbackChain AIAgentExecutionPolicy = "fallback_chain"
)

type AIAgentWorkload struct {
	ID                uuid.UUID              `json:"id"`
	TenantID          uuid.UUID              `json:"tenant_id"`
	QueueEventID      uuid.UUID              `json:"queue_event_id"`
	AgentID           uuid.UUID              `json:"agent_id"`
	AgentName         string                 `json:"agent_name"`
	ExecutionPolicy   AIAgentExecutionPolicy `json:"execution_policy"`
	ExecutionPriority int                    `json:"execution_priority"`
	ExecutionIndex    int                    `json:"execution_index"`
	EntityType        string                 `json:"entity_type"`
	EntityID          uuid.UUID              `json:"entity_id"`
	Status            AIAgentWorkloadStatus  `json:"status"`
	AttemptCount      int                    `json:"attempt_count"`
	MaxAttempts       int                    `json:"max_attempts"`
	WorkflowID        string                 `json:"workflow_id"`
	RunID             *uuid.UUID             `json:"run_id,omitempty"`
	LastStage         string                 `json:"last_stage"`
	LastError         string                 `json:"last_error"`
	ClosedByUser      bool                   `json:"closed_by_user"`
	CreatedAt         time.Time              `json:"created_at"`
	StartedAt         *time.Time             `json:"started_at,omitempty"`
	FinishedAt        *time.Time             `json:"finished_at,omitempty"`
	ClosedAt          *time.Time             `json:"closed_at,omitempty"`
	UpdatedAt         time.Time              `json:"updated_at"`
}
