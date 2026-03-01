package models

import (
	"time"

	"github.com/google/uuid"
)

type CatalogItem struct {
	ID        uuid.UUID      `json:"id"`
	TenantID  *uuid.UUID     `json:"tenant_id,omitempty"`
	Kind      string         `json:"kind"`
	OwnerID   *uuid.UUID     `json:"owner_id,omitempty"`
	RefID     *uuid.UUID     `json:"ref_id,omitempty"`
	Data      map[string]any `json:"data"`
	CreatedBy *uuid.UUID     `json:"created_by,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}
