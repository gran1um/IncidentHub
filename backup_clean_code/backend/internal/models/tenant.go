package models

import (
	"time"

	"github.com/google/uuid"
)

type Tenant struct {
	ID                uuid.UUID  `json:"id"`
	Slug              string     `json:"slug"`
	Name              string     `json:"name"`
	Description       string     `json:"description"`
	IsActive          bool       `json:"is_active"`
	MaxUsers          int        `json:"max_users"`
	ResponsibleUserID *uuid.UUID `json:"responsible_user_id,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}
