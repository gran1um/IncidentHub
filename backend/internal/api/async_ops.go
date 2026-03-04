package api

import (
	"context"
	"encoding/json"
)

type AsyncOperationType string

const (
	AsyncOperationAlertCreate AsyncOperationType = "alert.create"
	AsyncOperationAlertUpdate AsyncOperationType = "alert.update"
	AsyncOperationAlertDelete AsyncOperationType = "alert.delete"
	AsyncOperationCaseCreate  AsyncOperationType = "case.create"
	AsyncOperationCaseUpdate  AsyncOperationType = "case.update"
	AsyncOperationCaseDelete  AsyncOperationType = "case.delete"
)

type AsyncOperation struct {
	OperationID string             `json:"operation_id"`
	Type        AsyncOperationType `json:"type"`
	TenantID    string             `json:"tenant_id"`
	ActorID     string             `json:"actor_id"`
	ResourceID  string             `json:"resource_id,omitempty"`
	RequestedAt string             `json:"requested_at"`
	Payload     json.RawMessage    `json:"payload"`
}

type AsyncOpsQueue interface {
	Enabled() bool
	Enqueue(ctx context.Context, operation AsyncOperation) error
}

type queuedAsyncOperationResponse struct {
	OperationID   string `json:"operation_id"`
	Resource      string `json:"resource"`
	ResourceID    string `json:"resource_id"`
	OperationType string `json:"operation_type,omitempty"`
	Status        string `json:"status"`
}

type asyncAlertCreatePayload struct {
	AlertID     string `json:"alert_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Source      string `json:"source"`
	Status      string `json:"status"`
	Severity    string `json:"severity"`
	TLP         string `json:"tlp"`
	PAP         string `json:"pap"`
}

type asyncAlertUpdatePayload struct {
	AlertID     string  `json:"alert_id"`
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	Source      *string `json:"source,omitempty"`
	Status      *string `json:"status,omitempty"`
	Severity    *string `json:"severity,omitempty"`
	TLP         *string `json:"tlp,omitempty"`
	PAP         *string `json:"pap,omitempty"`
	AssignedTo  string  `json:"assigned_to,omitempty"`
}

type asyncCaseCreatePayload struct {
	CaseID             string `json:"case_id"`
	CaseNumber         string `json:"case_number"`
	Title              string `json:"title"`
	Description        string `json:"description"`
	Source             string `json:"source"`
	IncidentType       string `json:"incident_type"`
	Status             string `json:"status"`
	Priority           string `json:"priority"`
	Impact             string `json:"impact"`
	Confidence         int    `json:"confidence"`
	Severity           string `json:"severity"`
	TLP                string `json:"tlp"`
	PAP                string `json:"pap"`
	DetectedAt         string `json:"detected_at,omitempty"`
	OccurredAt         string `json:"occurred_at,omitempty"`
	ClosedAt           string `json:"closed_at,omitempty"`
	ResolutionSummary  string `json:"resolution_summary"`
	AssignedTo         string `json:"assigned_to,omitempty"`
	SourceCaseID       string `json:"source_case_id,omitempty"`
	IncludeObservables bool   `json:"include_observables,omitempty"`
}

type asyncCaseUpdatePayload struct {
	CaseID                 string  `json:"case_id"`
	CaseNumber             *string `json:"case_number,omitempty"`
	Title                  *string `json:"title,omitempty"`
	Description            *string `json:"description,omitempty"`
	Source                 *string `json:"source,omitempty"`
	IncidentType           *string `json:"incident_type,omitempty"`
	Status                 *string `json:"status,omitempty"`
	Priority               *string `json:"priority,omitempty"`
	Impact                 *string `json:"impact,omitempty"`
	Confidence             *int    `json:"confidence,omitempty"`
	Severity               *string `json:"severity,omitempty"`
	TLP                    *string `json:"tlp,omitempty"`
	PAP                    *string `json:"pap,omitempty"`
	DetectedAt             *string `json:"detected_at,omitempty"`
	OccurredAt             *string `json:"occurred_at,omitempty"`
	ClosedAt               *string `json:"closed_at,omitempty"`
	ExpectedUpdatedAt      string  `json:"expected_updated_at,omitempty"`
	ResolutionSummary      *string `json:"resolution_summary,omitempty"`
	AssignedTo             string  `json:"assigned_to,omitempty"`
	ClearAssignedTo        bool    `json:"clear_assigned_to,omitempty"`
	RewardCaseClosureToID  string  `json:"reward_case_closure_to_id,omitempty"`
	RewardCaseClosureEvent bool    `json:"reward_case_closure_event,omitempty"`
}

type asyncDeletePayload struct {
	ID string `json:"id"`
}
