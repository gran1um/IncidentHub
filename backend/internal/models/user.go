package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID               uuid.UUID `json:"id"`
	Username         string    `json:"username"`
	Email            string    `json:"email"`
	FullName         string    `json:"full_name"`
	Team             string    `json:"team"`
	AvatarURL        string    `json:"avatar_url"`
	CoverImageURL    string    `json:"cover_image_url"`
	PersonalLink     string    `json:"personal_link"`
	ExperiencePoints int64     `json:"experience_points"`
	PasswordHash     string    `json:"-"`
	IsActive         bool      `json:"is_active"`
	IsPlatformAdmin  bool      `json:"is_platform_admin"`
	LDAPEnabled      bool      `json:"ldap_enabled"`
	MFAEnabled       bool      `json:"mfa_enabled"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
