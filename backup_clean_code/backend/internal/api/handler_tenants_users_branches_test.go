package api

import (
	"context"
	"net/http"
	"testing"
)

func TestCreateUserBranchCoverage(t *testing.T) {
	env := newAPITestEnv(t)

	{
		c, _ := env.jsonContext(http.MethodPost, "/api/v1/users", map[string]any{
			"username":  "tenant-user-a",
			"email":     "tenant-user-a@example.com",
			"full_name": "Tenant User A",
			"password":  "Password123!",
		})
		setIdentity(c, env.identity)
		err := env.handler.CreateUser(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("expected tenant header required for non-platform create, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodPost, "/api/v1/users", map[string]any{
			"username":          "tenant-user-b",
			"email":             "tenant-user-b@example.com",
			"full_name":         "Tenant User B",
			"password":          "short",
			"is_platform_admin": true,
		})
		setIdentity(c, env.identity)
		setTenant(c, env.tenantID)
		err := env.handler.CreateUser(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("expected short password error, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodPost, "/api/v1/users", map[string]any{
			"username":  "platform-invalid-tenant",
			"email":     "platform-invalid-tenant@example.com",
			"full_name": "Platform Invalid Tenant",
			"password":  "Password123!",
			"tenant_id": "not-uuid",
		})
		setIdentity(c, env.platformAdmin)
		err := env.handler.CreateUser(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("expected invalid tenant id error, got %d", code)
		}
	}

	{
		c, rec := env.jsonContext(http.MethodPost, "/api/v1/users", map[string]any{
			"username":          "ldap-tenant-user",
			"email":             "ldap-tenant-user@example.com",
			"full_name":         "LDAP Tenant User",
			"tenant_id":         env.tenantID.String(),
			"ldap_enabled":      true,
			"role":              "analyst",
			"is_platform_admin": false,
		})
		setIdentity(c, env.platformAdmin)
		err := env.handler.CreateUser(c)
		mustStatusOK(t, err, rec, http.StatusCreated)
	}

	{
		c, _ := env.jsonContext(http.MethodPost, "/api/v1/users", map[string]any{
			"username":  "invalid-role-user",
			"email":     "invalid-role-user@example.com",
			"full_name": "Invalid Role User",
			"password":  "Password123!",
			"tenant_id": env.tenantID.String(),
			"role":      "super-admin",
		})
		setIdentity(c, env.platformAdmin)
		err := env.handler.CreateUser(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("expected invalid role error, got %d", code)
		}
	}
}

func TestUpdateUserForbiddenBranch(t *testing.T) {
	env := newAPITestEnv(t)

	{
		c, _ := env.jsonContext(http.MethodPatch, "/api/v1/users/"+env.userID.String(), map[string]any{
			"full_name": "Should not update",
		})
		setPath(c, "/api/v1/users/:id", []string{"id"}, []string{env.userID.String()})
		otherIdentity := env.identity
		otherIdentity.UserID = env.secondUserID
		setIdentity(c, otherIdentity)
		c.Request().Header.Set("X-Tenant-ID", env.tenantID.String())
		err := env.handler.UpdateUser(c)
		if code := httpErrorCode(t, err); code != http.StatusForbidden {
			t.Fatalf("expected forbidden update user branch, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodPatch, "/api/v1/users/not-uuid", map[string]any{
			"full_name": "bad id",
		})
		setPath(c, "/api/v1/users/:id", []string{"id"}, []string{"not-uuid"})
		setIdentity(c, env.identity)
		err := env.handler.UpdateUser(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("expected invalid user id error, got %d", code)
		}
	}

	{
		c, rec := env.jsonContext(http.MethodPatch, "/api/v1/users/"+env.userID.String(), map[string]any{})
		setPath(c, "/api/v1/users/:id", []string{"id"}, []string{env.userID.String()})
		setIdentity(c, env.identity)
		err := env.handler.UpdateUser(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	{
		c, rec := env.jsonContext(http.MethodPatch, "/api/v1/users/"+env.secondUserID.String(), map[string]any{
			"full_name": "Updated by platform admin",
		})
		setPath(c, "/api/v1/users/:id", []string{"id"}, []string{env.secondUserID.String()})
		setIdentity(c, env.platformAdmin)
		err := env.handler.UpdateUser(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}
}

func TestUpdateUserPasswordCurrentPasswordValidation(t *testing.T) {
	env := newAPITestEnv(t)

	{
		c, _ := env.jsonContext(http.MethodPatch, "/api/v1/users/"+env.userID.String(), map[string]any{
			"password": "Password123!9",
		})
		setPath(c, "/api/v1/users/:id", []string{"id"}, []string{env.userID.String()})
		setIdentity(c, env.identity)
		err := env.handler.UpdateUser(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("expected current password required error, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodPatch, "/api/v1/users/"+env.userID.String(), map[string]any{
			"password":         "Password123!9",
			"current_password": "wrong-password",
		})
		setPath(c, "/api/v1/users/:id", []string{"id"}, []string{env.userID.String()})
		setIdentity(c, env.identity)
		err := env.handler.UpdateUser(c)
		if code := httpErrorCode(t, err); code != http.StatusUnauthorized {
			t.Fatalf("expected invalid current password error, got %d", code)
		}
	}

	{
		c, rec := env.jsonContext(http.MethodPatch, "/api/v1/users/"+env.userID.String(), map[string]any{
			"password":         "Password123!9",
			"current_password": "Password123!",
		})
		setPath(c, "/api/v1/users/:id", []string{"id"}, []string{env.userID.String()})
		setIdentity(c, env.identity)
		err := env.handler.UpdateUser(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	{
		c, _ := env.jsonContext(http.MethodPost, "/api/v1/auth/login", map[string]any{
			"email":    "analyst-main@example.com",
			"password": "Password123!9",
		})
		err := env.handler.Login(c)
		if err != nil {
			t.Fatalf("expected login with updated password to succeed: %v", err)
		}
	}

	{
		c, rec := env.jsonContext(http.MethodPatch, "/api/v1/users/"+env.secondUserID.String(), map[string]any{
			"password": "Password123!9",
		})
		setPath(c, "/api/v1/users/:id", []string{"id"}, []string{env.secondUserID.String()})
		setIdentity(c, env.platformAdmin)
		err := env.handler.UpdateUser(c)
		mustStatusOK(t, err, rec, http.StatusOK)
	}

	{
		c, _ := env.jsonContext(http.MethodPost, "/api/v1/auth/login", map[string]any{
			"email":    "analyst-second@example.com",
			"password": "Password123!9",
		})
		err := env.handler.Login(c)
		if err != nil {
			t.Fatalf("expected login after admin password reset to succeed: %v", err)
		}
	}
}

func TestListUsersAndGetUserBranchCoverage(t *testing.T) {
	env := newAPITestEnv(t)

	{
		c, _ := env.jsonContext(http.MethodGet, "/api/v1/users", nil)
		setIdentity(c, env.identity)
		err := env.handler.ListUsers(c)
		if code := httpErrorCode(t, err); code != http.StatusForbidden {
			t.Fatalf("expected forbidden list users without filter, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodGet, "/api/v1/users?tenant_id=bad", nil)
		c.Request().URL.RawQuery = "tenant_id=bad"
		setIdentity(c, env.identity)
		err := env.handler.ListUsers(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("expected invalid tenant_id error, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodGet, "/api/v1/users?tenant_id="+env.secondTenant.String(), nil)
		c.Request().URL.RawQuery = "tenant_id=" + env.secondTenant.String()
		setIdentity(c, env.identity)
		err := env.handler.ListUsers(c)
		if code := httpErrorCode(t, err); code != http.StatusForbidden {
			t.Fatalf("expected forbidden tenant list for non-member, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodGet, "/api/v1/tenant-users", nil)
		setIdentity(c, env.identity)
		err := env.handler.ListTenantUsers(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("expected tenant header required for tenant-users, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodGet, "/api/v1/users/not-uuid", nil)
		setPath(c, "/api/v1/users/:id", []string{"id"}, []string{"not-uuid"})
		setIdentity(c, env.identity)
		err := env.handler.GetUser(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("expected invalid user id in get user, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodGet, "/api/v1/users/"+env.secondUserID.String(), nil)
		setPath(c, "/api/v1/users/:id", []string{"id"}, []string{env.secondUserID.String()})
		setIdentity(c, env.identity)
		err := env.handler.GetUser(c)
		if code := httpErrorCode(t, err); code != http.StatusForbidden {
			t.Fatalf("expected forbidden get user without tenant header, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodGet, "/api/v1/users/"+env.secondUserID.String(), nil)
		setPath(c, "/api/v1/users/:id", []string{"id"}, []string{env.secondUserID.String()})
		setIdentity(c, env.identity)
		c.Request().Header.Set("X-Tenant-ID", "bad-uuid")
		err := env.handler.GetUser(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("expected invalid tenant header in get user, got %d", code)
		}
	}
}

func TestUpdateUserTenantAdminCanEditOtherUser(t *testing.T) {
	env := newAPITestEnv(t)

	c, rec := env.jsonContext(http.MethodPatch, "/api/v1/users/"+env.secondUserID.String(), map[string]any{
		"full_name": "Second Analyst Updated",
		"team":      "SOC Delta",
		"role":      "viewer",
	})
	setPath(c, "/api/v1/users/:id", []string{"id"}, []string{env.secondUserID.String()})
	setIdentity(c, env.identity)
	c.Request().Header.Set("X-Tenant-ID", env.tenantID.String())

	err := env.handler.UpdateUser(c)
	mustStatusOK(t, err, rec, http.StatusOK)

	updatedMembership, err := env.memberships.Get(context.Background(), env.tenantID, env.secondUserID)
	if err != nil {
		t.Fatalf("load updated membership: %v", err)
	}
	if string(updatedMembership.Role) != "viewer" {
		t.Fatalf("expected role=viewer, got %s", updatedMembership.Role)
	}
}

func TestDeleteUserBranchCoverage(t *testing.T) {
	env := newAPITestEnv(t)

	{
		c, _ := env.jsonContext(http.MethodDelete, "/api/v1/users/"+env.secondUserID.String(), nil)
		setPath(c, "/api/v1/users/:id", []string{"id"}, []string{env.secondUserID.String()})
		setIdentity(c, env.identity)
		err := env.handler.DeleteUser(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("expected tenant header required for delete user, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodDelete, "/api/v1/users/"+env.userID.String(), nil)
		setPath(c, "/api/v1/users/:id", []string{"id"}, []string{env.userID.String()})
		setIdentity(c, env.identity)
		c.Request().Header.Set("X-Tenant-ID", env.tenantID.String())
		err := env.handler.DeleteUser(c)
		if code := httpErrorCode(t, err); code != http.StatusBadRequest {
			t.Fatalf("expected self-delete guard, got %d", code)
		}
	}

	{
		c, _ := env.jsonContext(http.MethodDelete, "/api/v1/users/"+env.userID.String(), nil)
		setPath(c, "/api/v1/users/:id", []string{"id"}, []string{env.userID.String()})
		otherIdentity := env.identity
		otherIdentity.UserID = env.secondUserID
		setIdentity(c, otherIdentity)
		c.Request().Header.Set("X-Tenant-ID", env.tenantID.String())
		err := env.handler.DeleteUser(c)
		if code := httpErrorCode(t, err); code != http.StatusForbidden {
			t.Fatalf("expected forbidden delete for non-admin requester, got %d", code)
		}
	}

	{
		c, rec := env.jsonContext(http.MethodDelete, "/api/v1/users/"+env.secondUserID.String(), nil)
		setPath(c, "/api/v1/users/:id", []string{"id"}, []string{env.secondUserID.String()})
		setIdentity(c, env.identity)
		c.Request().Header.Set("X-Tenant-ID", env.tenantID.String())
		err := env.handler.DeleteUser(c)
		mustStatusOK(t, err, rec, http.StatusNoContent)
	}

	{
		membership, err := env.memberships.Get(context.Background(), env.tenantID, env.secondUserID)
		if err != nil {
			t.Fatalf("load membership after delete: %v", err)
		}
		if membership.IsActive {
			t.Fatal("expected membership to be inactive after delete")
		}
		user, err := env.users.GetByID(context.Background(), env.secondUserID)
		if err != nil {
			t.Fatalf("load user after delete: %v", err)
		}
		if user.IsActive {
			t.Fatal("expected user to be inactive after removing the last membership")
		}
	}
}
