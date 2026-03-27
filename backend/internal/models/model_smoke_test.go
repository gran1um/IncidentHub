package models

import "testing"

func TestTenantRoleConstants(t *testing.T) {
	if TenantRoleAdmin != "tenant_admin" {
		t.Fatalf("unexpected admin role constant: %s", TenantRoleAdmin)
	}
	if TenantRoleAnalyst != "analyst" {
		t.Fatalf("unexpected analyst role constant: %s", TenantRoleAnalyst)
	}
	if TenantRoleViewer != "viewer" {
		t.Fatalf("unexpected viewer role constant: %s", TenantRoleViewer)
	}
}
