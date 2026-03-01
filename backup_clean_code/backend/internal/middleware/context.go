package middleware

import (
	"incidenthub/backend/internal/models"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

const (
	contextIdentityKey = "identity"
	contextTenantKey   = "tenant_id"
)

func setIdentity(c *echo.Context, identity models.Identity) {
	c.Set(contextIdentityKey, identity)
}

func GetIdentity(c *echo.Context) (models.Identity, bool) {
	v := c.Get(contextIdentityKey)
	if v == nil {
		return models.Identity{}, false
	}
	id, ok := v.(models.Identity)
	return id, ok
}

func setTenantID(c *echo.Context, tenantID uuid.UUID) {
	c.Set(contextTenantKey, tenantID)
}

func GetTenantID(c *echo.Context) (uuid.UUID, bool) {
	v := c.Get(contextTenantKey)
	if v == nil {
		return uuid.Nil, false
	}
	tenantID, ok := v.(uuid.UUID)
	return tenantID, ok
}
