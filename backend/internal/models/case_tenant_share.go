package models

import (
	"time"

	"github.com/google/uuid"
)

type CaseTenantShare struct {
	CaseID         uuid.UUID  `json:"case_id"`
	OwnerTenantID  uuid.UUID  `json:"owner_tenant_id"`
	SharedTenantID uuid.UUID  `json:"shared_tenant_id"`
	CreatedBy      *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}
