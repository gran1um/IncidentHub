package models

import (
	"time"

	"github.com/google/uuid"
)

type ConnectorHubExecutionStatus string

const (
	ConnectorHubExecutionStatusAccepted         ConnectorHubExecutionStatus = "accepted"
	ConnectorHubExecutionStatusQueued           ConnectorHubExecutionStatus = "queued"
	ConnectorHubExecutionStatusDispatching      ConnectorHubExecutionStatus = "dispatching"
	ConnectorHubExecutionStatusProviderAccepted ConnectorHubExecutionStatus = "provider_accepted"
	ConnectorHubExecutionStatusRetryScheduled   ConnectorHubExecutionStatus = "retry_scheduled"
	ConnectorHubExecutionStatusCompleted        ConnectorHubExecutionStatus = "completed"
	ConnectorHubExecutionStatusDryRun           ConnectorHubExecutionStatus = "dry_run"
	ConnectorHubExecutionStatusFailed           ConnectorHubExecutionStatus = "failed"
	ConnectorHubExecutionStatusCancelled        ConnectorHubExecutionStatus = "canceled"
	ConnectorHubExecutionStatusDeadLetter       ConnectorHubExecutionStatus = "dead_letter"
)

type ConnectorHubExecution struct {
	ID               uuid.UUID                   `json:"id"`
	TenantID         uuid.UUID                   `json:"tenant_id"`
	ActorID          *uuid.UUID                  `json:"actor_id,omitempty"`
	ConnectorID      uuid.UUID                   `json:"connector_id"`
	ConnectorName    string                      `json:"connector_name"`
	ConnectorChannel string                      `json:"connector_channel"`
	MethodID         *uuid.UUID                  `json:"method_id,omitempty"`
	MethodName       string                      `json:"method_name"`
	MethodKey        string                      `json:"method_key"`
	Action           string                      `json:"action"`
	CaseID           *uuid.UUID                  `json:"case_id,omitempty"`
	AlertID          *uuid.UUID                  `json:"alert_id,omitempty"`
	ExecutionMode    string                      `json:"execution_mode"`
	DryRun           bool                        `json:"dry_run"`
	Status           ConnectorHubExecutionStatus `json:"status"`
	Request          map[string]any              `json:"request"`
	Response         map[string]any              `json:"response"`
	Metadata         map[string]any              `json:"metadata"`
	Error            string                      `json:"error"`
	ProviderStatus   string                      `json:"provider_status"`
	ExternalID       string                      `json:"external_id"`
	CorrelationID    string                      `json:"correlation_id"`
	IdempotencyKey   string                      `json:"idempotency_key"`
	AttemptCount     int                         `json:"attempt_count"`
	MaxAttempts      int                         `json:"max_attempts"`
	NextAttemptAt    *time.Time                  `json:"next_attempt_at,omitempty"`
	CancelledByUser  bool                        `json:"canceled_by_user"`
	CancelledAt      *time.Time                  `json:"canceled_at,omitempty"`
	CreatedAt        time.Time                   `json:"created_at"`
	StartedAt        *time.Time                  `json:"started_at,omitempty"`
	FinishedAt       *time.Time                  `json:"finished_at,omitempty"`
	UpdatedAt        time.Time                   `json:"updated_at"`
}

type ConnectorHubExecutionAttempt struct {
	ID             uuid.UUID                   `json:"id"`
	ExecutionID    uuid.UUID                   `json:"execution_id"`
	AttemptNo      int                         `json:"attempt_no"`
	Status         ConnectorHubExecutionStatus `json:"status"`
	Request        map[string]any              `json:"request"`
	Response       map[string]any              `json:"response"`
	Error          string                      `json:"error"`
	ProviderStatus string                      `json:"provider_status"`
	ExternalID     string                      `json:"external_id"`
	CorrelationID  string                      `json:"correlation_id"`
	CreatedAt      time.Time                   `json:"created_at"`
	StartedAt      time.Time                   `json:"started_at"`
	FinishedAt     *time.Time                  `json:"finished_at,omitempty"`
	DurationMS     int64                       `json:"duration_ms"`
	UpdatedAt      time.Time                   `json:"updated_at"`
}

type ConnectorHubExecutionEvent struct {
	ID          uuid.UUID                   `json:"id"`
	ExecutionID uuid.UUID                   `json:"execution_id"`
	AttemptNo   *int                        `json:"attempt_no,omitempty"`
	EventType   string                      `json:"event_type"`
	Status      ConnectorHubExecutionStatus `json:"status"`
	Message     string                      `json:"message"`
	Data        map[string]any              `json:"data"`
	CreatedAt   time.Time                   `json:"created_at"`
}
