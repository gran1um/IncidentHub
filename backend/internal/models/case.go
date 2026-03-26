package models

import (
	"time"

	"github.com/google/uuid"
)

type Case struct {
	ID                uuid.UUID  `json:"id"`
	TenantID          uuid.UUID  `json:"tenant_id"`
	CaseNumber        string     `json:"case_number"`
	Title             string     `json:"title"`
	Description       string     `json:"description"`
	Source            string     `json:"source"`
	IncidentType      string     `json:"incident_type"`
	Status            string     `json:"status"`
	Priority          string     `json:"priority"`
	Impact            string     `json:"impact"`
	Confidence        int        `json:"confidence"`
	Severity          string     `json:"severity"`
	TLP               string     `json:"tlp"`
	PAP               string     `json:"pap"`
	DetectedAt        *time.Time `json:"detected_at,omitempty"`
	OccurredAt        *time.Time `json:"occurred_at,omitempty"`
	ClosedAt          *time.Time `json:"closed_at,omitempty"`
	ResolutionSummary string     `json:"resolution_summary"`
	CreatedBy         *uuid.UUID `json:"created_by,omitempty"`
	AssignedTo        *uuid.UUID `json:"assigned_to,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}
