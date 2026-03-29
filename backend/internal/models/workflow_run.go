package models

import (
	"time"

	"github.com/google/uuid"
)

type WorkflowRunStatus string

const (
	WorkflowRunStatusQueued  WorkflowRunStatus = "queued"
	WorkflowRunStatusRunning WorkflowRunStatus = "running"
	WorkflowRunStatusSuccess WorkflowRunStatus = "success"
	WorkflowRunStatusFailed  WorkflowRunStatus = "failed"
)

type WorkflowRun struct {
	ID           uuid.UUID         `json:"id"`
	TenantID     uuid.UUID         `json:"tenant_id"`
	WorkflowID   uuid.UUID         `json:"workflow_id"`
	WorkflowKind string            `json:"workflow_kind"`
	Trigger      string            `json:"trigger"`
	Status       WorkflowRunStatus `json:"status"`
	StartedAt    time.Time         `json:"started_at"`
	FinishedAt   *time.Time        `json:"finished_at,omitempty"`
	DurationMS   int               `json:"duration_ms"`
	Input        map[string]any    `json:"input"`
	Result       map[string]any    `json:"result"`
	Error        string            `json:"error"`
	CreatedBy    *uuid.UUID        `json:"created_by,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}
