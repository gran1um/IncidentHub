package models

import (
	"time"

	"github.com/google/uuid"
)

type Alert struct {
	ID          uuid.UUID  `json:"id"`
	TenantID    uuid.UUID  `json:"tenant_id"`
	CaseID      *uuid.UUID `json:"case_id,omitempty"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Source      string     `json:"source"`
	Status      string     `json:"status"`
	Severity    string     `json:"severity"`
	TLP         string     `json:"tlp"`
	PAP         string     `json:"pap"`
	CreatedBy   *uuid.UUID `json:"created_by,omitempty"`
	AssignedTo  *uuid.UUID `json:"assigned_to,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
