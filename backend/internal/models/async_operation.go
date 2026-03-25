package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type AsyncOperationStatus string

const (
	AsyncOperationStatusQueued     AsyncOperationStatus = "queued"
	AsyncOperationStatusProcessing AsyncOperationStatus = "processing"
	AsyncOperationStatusDone       AsyncOperationStatus = "done"
	AsyncOperationStatusFailed     AsyncOperationStatus = "failed"
)

type AsyncOperationRecord struct {
	OperationID   uuid.UUID            `json:"operation_id"`
	TenantID      uuid.UUID            `json:"tenant_id"`
	ActorID       uuid.UUID            `json:"actor_id"`
	Resource      string               `json:"resource"`
	ResourceID    *uuid.UUID           `json:"resource_id,omitempty"`
	OperationType string               `json:"operation_type"`
	Status        AsyncOperationStatus `json:"status"`
	AttemptCount  int                  `json:"attempt_count"`
	MaxAttempts   int                  `json:"max_attempts"`
	LastError     string               `json:"last_error,omitempty"`
	Payload       json.RawMessage      `json:"payload,omitempty"`
	QueuedAt      time.Time            `json:"queued_at"`
	StartedAt     *time.Time           `json:"started_at,omitempty"`
	FinishedAt    *time.Time           `json:"finished_at,omitempty"`
	UpdatedAt     time.Time            `json:"updated_at"`
}
