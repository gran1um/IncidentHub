package models

import (
	"time"

	"github.com/google/uuid"
)

type Observable struct {
	ID        uuid.UUID  `json:"id"`
	TenantID  uuid.UUID  `json:"tenant_id"`
	CaseID    uuid.UUID  `json:"case_id"`
	Type      string     `json:"type"`
	Value     string     `json:"value"`
	Verdict   string     `json:"verdict"`
	Source    string     `json:"source"`
	Tags      []string   `json:"tags"`
	CreatedBy *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}
