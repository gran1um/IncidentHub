package models

import "github.com/google/uuid"

const (
	IdentityAuthTypeJWT      = "jwt"
	IdentityAuthTypeAPIToken = "api_token"
)

type Identity struct {
	UserID             uuid.UUID  `json:"user_id"`
	Username           string     `json:"username"`
	IsPlatformAdmin    bool       `json:"is_platform_admin"`
	TenantID           *uuid.UUID `json:"tenant_id,omitempty"`
	TenantRole         TenantRole `json:"tenant_role,omitempty"`
	AuthType           string     `json:"auth_type,omitempty"`
	APITokenID         *uuid.UUID `json:"api_token_id,omitempty"`
	APITokenScopes     []string   `json:"api_token_scopes,omitempty"`
	APITokenFullAccess bool       `json:"api_token_full_access,omitempty"`
}
