package models

import (
	"time"

	"github.com/google/uuid"
)

type CaseAIAnalysis struct {
	ID              uuid.UUID        `json:"id"`
	TenantID        uuid.UUID        `json:"tenant_id"`
	CaseID          uuid.UUID        `json:"case_id"`
	RequestedBy     *uuid.UUID       `json:"requested_by,omitempty"`
	Model           string           `json:"model"`
	Status          string           `json:"status"`
	Verdict         string           `json:"verdict"`
	Confidence      float64          `json:"confidence"`
	Summary         string           `json:"summary"`
	Recommendations []string         `json:"recommendations"`
	Findings        []string         `json:"findings"`
	Sources         []map[string]any `json:"sources"`
	ErrorMessage    string           `json:"error_message"`
	CreatedAt       time.Time        `json:"created_at"`
}
