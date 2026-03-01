package models

type TenantRole string

const (
	TenantRoleAdmin   TenantRole = "tenant_admin"
	TenantRoleAnalyst TenantRole = "analyst"
	TenantRoleViewer  TenantRole = "viewer"
)
