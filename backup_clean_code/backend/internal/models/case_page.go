package models

import (
	"time"

	"github.com/google/uuid"
)

type CasePage struct {
	ID        uuid.UUID  `json:"id"`
	TenantID  uuid.UUID  `json:"tenant_id"`
	CaseID    uuid.UUID  `json:"case_id"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	CreatedBy *uuid.UUID `json:"created_by,omitempty"`
	UpdatedBy *uuid.UUID `json:"updated_by,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}
