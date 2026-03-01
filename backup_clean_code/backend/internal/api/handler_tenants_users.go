package api

import (
	"context"
	"errors"
	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/middleware"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"
	"incidenthub/backend/internal/security"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labstack/echo/v5"
)

func (h *Handler) ListTenants(c *echo.Context) error {
	identity, _ := middleware.GetIdentity(c)
	if identity.IsPlatformAdmin {
		items, err := h.tenants.List(c.Request().Context(), 200, 0)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to list tenants")
		}
		return c.JSON(http.StatusOK, items)
	}
	memberships, err := h.memberships.ListByUser(c.Request().Context(), identity.UserID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list memberships")
	}
	items := make([]models.Tenant, 0, len(memberships))
	for _, m := range memberships {
		t, err := h.tenants.GetByID(c.Request().Context(), m.TenantID)
		if err == nil {
			items = append(items, *t)
		}
	}
	return c.JSON(http.StatusOK, items)
}

func (h *Handler) CreateTenant(c *echo.Context) error {
	var req createTenantRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	req.Slug = strings.TrimSpace(strings.ToLower(req.Slug))
	req.Name = strings.TrimSpace(req.Name)
	if req.Slug == "" || req.Name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "slug and name are required")
	}
	if _, err := h.tenants.GetBySlug(c.Request().Context(), req.Slug); err == nil {
		return echo.NewHTTPError(http.StatusConflict, "tenant slug already exists")
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to validate tenant slug")
	}
	maxUsers := 50
	if req.MaxUsers != nil && *req.MaxUsers > 0 {
		maxUsers = *req.MaxUsers
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	var responsible *uuid.UUID
	if req.Responsible != nil && strings.TrimSpace(*req.Responsible) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(*req.Responsible))
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid responsible_user_id")
		}
		responsible = &parsed
	}
	item, err := h.tenants.Create(c.Request().Context(), repository.CreateTenantParams{
		Slug:        req.Slug,
		Name:        req.Name,
		Description: strings.TrimSpace(req.Description),
		MaxUsers:    maxUsers,
		Responsible: responsible,
		IsActive:    isActive,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return echo.NewHTTPError(http.StatusConflict, "tenant slug already exists")
		}
		return echo.NewHTTPError(http.StatusBadRequest, "failed to create tenant")
	}
	identity, _ := middleware.GetIdentity(c)
	_ = h.audits.Log(c.Request().Context(), &item.ID, &identity.UserID, "tenant_create", "tenant", &item.ID, map[string]any{"slug": item.Slug})
	return c.JSON(http.StatusCreated, item)
}

func (h *Handler) UpdateTenant(c *echo.Context) error {
	identity, _ := middleware.GetIdentity(c)
	tenantID, parseErr := uuid.Parse(strings.TrimSpace(c.Param("tenantID")))
	if parseErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid tenant id")
	}

	var req updateTenantRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	var responsible *uuid.UUID
	if req.Responsible != nil {
		trimmed := strings.TrimSpace(*req.Responsible)
		if trimmed != "" {
			parsed, err := uuid.Parse(trimmed)
			if err != nil {
				return echo.NewHTTPError(http.StatusBadRequest, "invalid responsible_user_id")
			}
			responsible = &parsed
		}
	}

	isActive := req.IsActive
	if req.Active != nil {
		isActive = req.Active
	}

	item, err := h.tenants.Update(c.Request().Context(), tenantID, repository.UpdateTenantParams{
		Name:        trimOptional(req.Name),
		Description: trimOptional(req.Description),
		IsActive:    isActive,
		MaxUsers:    req.MaxUsers,
		Responsible: responsible,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to update tenant")
	}
	_ = h.audits.Log(c.Request().Context(), &tenantID, &identity.UserID, "tenant_update", "tenant", &tenantID, nil)
	return c.JSON(http.StatusOK, item)
}

func (h *Handler) CreateUser(c *echo.Context) error {
	identity, _ := middleware.GetIdentity(c)
	var req createUserRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.FullName = strings.TrimSpace(req.FullName)
	if req.Username == "" || req.Email == "" || req.FullName == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "username, email and full_name are required")
	}

	var tenantID *uuid.UUID
	if req.TenantID != "" {
		parsed, err := uuid.Parse(req.TenantID)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid tenant_id")
		}
		tenantID = &parsed
	}

	if !identity.IsPlatformAdmin {
		ctxTenantID, ok := middleware.GetTenantID(c)
		if !ok {
			return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
		}
		tenantID = &ctxTenantID
		req.IsPlatformAdmin = false
	}

	passwordHash := ""
	if !req.LDAPEnabled {
		if len(req.Password) < h.cfg.Security.PasswordMinLength {
			return echo.NewHTTPError(http.StatusBadRequest, "password is too short")
		}
		hash, err := security.HashPassword(req.Password)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to hash password")
		}
		passwordHash = hash
	}

	created, err := h.users.Create(c.Request().Context(), repository.CreateUserParams{
		Username:        req.Username,
		Email:           req.Email,
		FullName:        req.FullName,
		PasswordHash:    passwordHash,
		IsPlatformAdmin: req.IsPlatformAdmin,
		LDAPEnabled:     req.LDAPEnabled,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to create user")
	}

	if tenantID != nil {
		role, err := parseRole(req.Role)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		if err := h.memberships.Upsert(c.Request().Context(), *tenantID, created.ID, role); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to assign tenant role")
		}
	}

	_ = h.audits.Log(c.Request().Context(), tenantID, &identity.UserID, "user_create", "user", &created.ID, map[string]any{"email": created.Email})
	created.PasswordHash = ""
	return c.JSON(http.StatusCreated, created)
}

func (h *Handler) ListUsers(c *echo.Context) error {
	identity, ok := middleware.GetIdentity(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}

	tenantFilter := strings.TrimSpace(c.QueryParam("tenant_id"))
	if tenantFilter != "" {
		tenantID, err := uuid.Parse(tenantFilter)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid tenant_id")
		}
		if !identity.IsPlatformAdmin {
			membership, membershipErr := h.memberships.Get(c.Request().Context(), tenantID, identity.UserID)
			if membershipErr != nil || !membership.IsActive {
				return echo.NewHTTPError(http.StatusForbidden, "forbidden")
			}
		}
		items, err := h.users.ListByTenantWithRole(c.Request().Context(), tenantID, 200, 0)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to list users")
		}
		for idx := range items {
			h.enrichTenantUserMediaURLs(c.Request().Context(), &items[idx])
		}
		return c.JSON(http.StatusOK, items)
	}

	if !identity.IsPlatformAdmin {
		return echo.NewHTTPError(http.StatusForbidden, "platform admin role required")
	}

	items, err := h.users.List(c.Request().Context(), 200, 0)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list users")
	}
	for idx := range items {
		h.enrichUserMediaURLs(c.Request().Context(), &items[idx])
	}
	return c.JSON(http.StatusOK, items)
}

func (h *Handler) ListTenantUsers(c *echo.Context) error {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	items, err := h.users.ListByTenantWithRole(c.Request().Context(), tenantID, 300, 0)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list tenant users")
	}
	for idx := range items {
		h.enrichTenantUserMediaURLs(c.Request().Context(), &items[idx])
	}
	return c.JSON(http.StatusOK, items)
}

func (h *Handler) GetUser(c *echo.Context) error {
	identity, ok := middleware.GetIdentity(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}

	id, parseErr := uuid.Parse(c.Param("id"))
	if parseErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user id")
	}

	if id != identity.UserID && !identity.IsPlatformAdmin {
		tenantRaw := strings.TrimSpace(c.Request().Header.Get(config.TenantHeader))
		if tenantRaw == "" {
			return echo.NewHTTPError(http.StatusForbidden, "forbidden")
		}
		tenantID, tenantParseErr := uuid.Parse(tenantRaw)
		if tenantParseErr != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid tenant header")
		}
		requesterMembership, err := h.memberships.Get(c.Request().Context(), tenantID, identity.UserID)
		if err != nil || !requesterMembership.IsActive {
			return echo.NewHTTPError(http.StatusForbidden, "forbidden")
		}
		targetMembership, err := h.memberships.Get(c.Request().Context(), tenantID, id)
		if err != nil || !targetMembership.IsActive {
			return echo.NewHTTPError(http.StatusForbidden, "forbidden")
		}
	}

	user, err := h.users.GetByID(c.Request().Context(), id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}
	h.enrichUserMediaURLs(c.Request().Context(), user)
	user.PasswordHash = ""
	return c.JSON(http.StatusOK, user)
}

func (h *Handler) GetUserCasePerformance(c *echo.Context) error {
	if h.experience == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "experience repository is not configured")
	}
	identity, ok := middleware.GetIdentity(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}

	userID, parseErr := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if parseErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user id")
	}
	if _, err := h.users.GetByID(c.Request().Context(), userID); err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}

	var tenantID *uuid.UUID
	if resolvedTenantID, tenantFound := middleware.GetTenantID(c); tenantFound {
		tenantID = &resolvedTenantID
	} else {
		tenantRaw := strings.TrimSpace(c.Request().Header.Get(config.TenantHeader))
		if tenantRaw != "" {
			parsedTenantID, tenantParseErr := uuid.Parse(tenantRaw)
			if tenantParseErr != nil {
				return echo.NewHTTPError(http.StatusBadRequest, "invalid tenant header")
			}
			tenantID = &parsedTenantID
		}
	}
	if !identity.IsPlatformAdmin {
		if tenantID == nil && identity.TenantID != nil {
			tenantID = identity.TenantID
		}
		if tenantID == nil {
			return echo.NewHTTPError(http.StatusForbidden, "forbidden")
		}
		requesterMembership, membershipErr := h.memberships.Get(c.Request().Context(), *tenantID, identity.UserID)
		if membershipErr != nil || !requesterMembership.IsActive {
			return echo.NewHTTPError(http.StatusForbidden, "forbidden")
		}
		targetMembership, targetMembershipErr := h.memberships.Get(c.Request().Context(), *tenantID, userID)
		if targetMembershipErr != nil || !targetMembership.IsActive {
			return echo.NewHTTPError(http.StatusForbidden, "forbidden")
		}
	}

	performance, err := h.experience.UserCasePerformance(c.Request().Context(), userID, tenantID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load user performance")
	}
	return c.JSON(http.StatusOK, performance)
}

func (h *Handler) ListUserExperienceEvents(c *echo.Context) error {
	if h.experience == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "experience repository is not configured")
	}
	identity, ok := middleware.GetIdentity(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}

	userID, parseErr := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if parseErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user id")
	}

	var tenantID *uuid.UUID
	if resolvedTenantID, tenantFound := middleware.GetTenantID(c); tenantFound {
		tenantID = &resolvedTenantID
	} else {
		tenantRaw := strings.TrimSpace(c.Request().Header.Get(config.TenantHeader))
		if tenantRaw != "" {
			parsedTenantID, tenantParseErr := uuid.Parse(tenantRaw)
			if tenantParseErr != nil {
				return echo.NewHTTPError(http.StatusBadRequest, "invalid tenant header")
			}
			tenantID = &parsedTenantID
		}
	}
	if !identity.IsPlatformAdmin && tenantID != nil {
		requesterMembership, membershipErr := h.memberships.Get(c.Request().Context(), *tenantID, identity.UserID)
		if membershipErr != nil || !requesterMembership.IsActive {
			return echo.NewHTTPError(http.StatusForbidden, "forbidden")
		}
	}

	if userID != identity.UserID && !identity.IsPlatformAdmin {
		if tenantID == nil {
			return echo.NewHTTPError(http.StatusForbidden, "forbidden")
		}
		targetMembership, targetMembershipErr := h.memberships.Get(c.Request().Context(), *tenantID, userID)
		if targetMembershipErr != nil || !targetMembership.IsActive {
			return echo.NewHTTPError(http.StatusForbidden, "forbidden")
		}
	}
	if !identity.IsPlatformAdmin && tenantID == nil && identity.TenantID != nil {
		tenantID = identity.TenantID
	}

	limit := 20
	if rawLimit := strings.TrimSpace(c.QueryParam("limit")); rawLimit != "" {
		parsedLimit, parseErr := strconv.Atoi(rawLimit)
		if parseErr != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid limit")
		}
		limit = parsedLimit
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	offset := 0
	if rawOffset := strings.TrimSpace(c.QueryParam("offset")); rawOffset != "" {
		parsedOffset, parseErr := strconv.Atoi(rawOffset)
		if parseErr != nil || parsedOffset < 0 {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid offset")
		}
		offset = parsedOffset
	}

	items, err := h.experience.ListByUser(c.Request().Context(), userID, tenantID, limit, offset)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list experience events")
	}
	payload := make([]map[string]any, 0, len(items))
	for _, item := range items {
		description := strings.TrimSpace(item.Description)
		if description == "" {
			description = defaultExperienceDescription(item.EventType, item.Details)
		}
		entry := map[string]any{
			"id":          item.ID.String(),
			"user_id":     item.UserID.String(),
			"event_key":   item.EventKey,
			"event_type":  item.EventType,
			"points":      item.Points,
			"description": description,
			"details":     item.Details,
			"created_at":  item.CreatedAt,
		}
		if item.TenantID != nil {
			entry["tenant_id"] = item.TenantID.String()
		}
		payload = append(payload, entry)
	}
	return c.JSON(http.StatusOK, payload)
}

func (h *Handler) AwardUserExperience(c *echo.Context) error {
	if h.experience == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "experience repository is not configured")
	}
	identity, ok := middleware.GetIdentity(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	userID, parseErr := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if parseErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user id")
	}

	targetMembership, err := h.memberships.Get(c.Request().Context(), tenantID, userID)
	if err != nil || !targetMembership.IsActive {
		return echo.NewHTTPError(http.StatusNotFound, "user is not active in tenant")
	}

	var req awardUserExperienceRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	req.Description = strings.TrimSpace(req.Description)
	if req.Points <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "points must be greater than zero")
	}
	if req.Points > 1_000_000_000 {
		return echo.NewHTTPError(http.StatusBadRequest, "points is too large")
	}
	if req.Description == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "description is required")
	}

	details := map[string]any{
		"actor_id":       identity.UserID.String(),
		"actor_username": identity.Username,
		"tenant_id":      tenantID.String(),
		"triggered":      xpEventTypeManualGrant,
		"source":         "admin_panel",
	}
	result, err := h.experience.Award(c.Request().Context(), repository.AwardExperienceParams{
		TenantID:    &tenantID,
		UserID:      userID,
		EventKey:    "manual_grant:" + userID.String() + ":" + uuid.NewString(),
		EventType:   xpEventTypeManualGrant,
		Points:      req.Points,
		Description: req.Description,
		Details:     details,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to award experience")
	}

	_ = h.audits.Log(c.Request().Context(), &tenantID, &identity.UserID, "user_experience_award", "user", &userID, map[string]any{
		"points":      req.Points,
		"description": req.Description,
	})
	return c.JSON(http.StatusOK, map[string]any{
		"user_id":      userID.String(),
		"tenant_id":    tenantID.String(),
		"points":       req.Points,
		"description":  req.Description,
		"awarded":      result.Awarded,
		"total_points": result.TotalPoints,
	})
}

func (h *Handler) UpdateUser(c *echo.Context) error {
	identity, ok := middleware.GetIdentity(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user id")
	}

	ctx := c.Request().Context()
	managedTenantID, managingOtherUser, manageErr := h.resolveManagedTenantForUserUpdate(ctx, identity, id, c.Request().Header.Get(config.TenantHeader))
	if manageErr != nil {
		return manageErr
	}

	var req updateUserRequest
	if bindErr := c.Bind(&req); bindErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	roleToUpdate, roleErr := parseOptionalTenantRole(req.Role)
	if roleErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid role")
	}
	if roleToUpdate != nil && id == identity.UserID && !identity.IsPlatformAdmin {
		return echo.NewHTTPError(http.StatusForbidden, "forbidden")
	}

	fullName := resolveUpdateUserFullName(req)
	email := lowerOptional(req.Email)
	team := trimOptional(req.Team)
	avatarURL := trimOptional(coalesceStringPtr(req.Avatar, req.AvatarURL))
	coverImageURL := trimOptional(coalesceStringPtr(req.CoverImage, req.CoverImageURL))
	personalLink := trimOptional(req.PersonalLink)

	passwordUpdated, passwordErr := h.maybeUpdateUserPassword(ctx, identity, id, req, managingOtherUser)
	if passwordErr != nil {
		return passwordErr
	}

	roleUpdated, roleTenantID, roleUpdateErr := h.maybeUpdateUserRole(ctx, identity, id, roleToUpdate, managedTenantID, c.Request().Header.Get(config.TenantHeader))
	if roleUpdateErr != nil {
		return roleUpdateErr
	}

	updated, updateErr := h.updateUserProfileOrFetch(ctx, id, fullName, email, team, avatarURL, coverImageURL, personalLink)
	if updateErr != nil {
		return updateErr
	}
	h.enrichUserMediaURLs(ctx, updated)

	auditDetails := map[string]any{
		"email_updated":    email != nil,
		"name_updated":     fullName != nil,
		"password_updated": passwordUpdated,
		"role_updated":     roleUpdated,
	}
	if roleUpdated && roleToUpdate != nil {
		auditDetails["new_role"] = string(*roleToUpdate)
		if roleTenantID != uuid.Nil {
			auditDetails["tenant_id"] = roleTenantID.String()
		}
	}

	_ = h.audits.Log(ctx, nil, &identity.UserID, "user_update", "user", &id, auditDetails)
	return c.JSON(http.StatusOK, updated)
}

func coalesceStringPtr(values ...*string) *string {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func parseOptionalTenantRole(raw *string) (*models.TenantRole, error) {
	if raw == nil {
		return nil, nil
	}
	parsedRole, err := parseRole(*raw)
	if err != nil {
		return nil, err
	}
	return &parsedRole, nil
}

func resolveUpdateUserFullName(req updateUserRequest) *string {
	if name := trimOptional(req.Name); name != nil {
		return name
	}
	return trimOptional(req.FullName)
}

func (h *Handler) resolveManagedTenantForUserUpdate(ctx context.Context, identity models.Identity, userID uuid.UUID, tenantRaw string) (uuid.UUID, bool, error) {
	if userID == identity.UserID || identity.IsPlatformAdmin {
		return uuid.Nil, false, nil
	}
	tenantRaw = strings.TrimSpace(tenantRaw)
	if tenantRaw == "" {
		return uuid.Nil, false, echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	tenantID, tenantParseErr := uuid.Parse(tenantRaw)
	if tenantParseErr != nil {
		return uuid.Nil, false, echo.NewHTTPError(http.StatusBadRequest, "invalid tenant header")
	}

	requesterMembership, membershipErr := h.memberships.Get(ctx, tenantID, identity.UserID)
	if membershipErr != nil || !requesterMembership.IsActive || requesterMembership.Role != models.TenantRoleAdmin {
		return uuid.Nil, false, echo.NewHTTPError(http.StatusForbidden, "forbidden")
	}
	targetMembership, targetMembershipErr := h.memberships.Get(ctx, tenantID, userID)
	if targetMembershipErr != nil || !targetMembership.IsActive {
		return uuid.Nil, false, echo.NewHTTPError(http.StatusNotFound, "user not found")
	}
	targetUser, targetUserErr := h.users.GetByID(ctx, userID)
	if targetUserErr != nil {
		return uuid.Nil, false, echo.NewHTTPError(http.StatusNotFound, "user not found")
	}
	if targetUser.IsPlatformAdmin {
		return uuid.Nil, false, echo.NewHTTPError(http.StatusForbidden, "forbidden")
	}

	return tenantID, true, nil
}

func (h *Handler) maybeUpdateUserPassword(ctx context.Context, identity models.Identity, userID uuid.UUID, req updateUserRequest, managingOtherUser bool) (bool, error) {
	if req.Password == nil {
		return false, nil
	}
	password := strings.TrimSpace(*req.Password)
	if password == "" {
		return false, nil
	}
	if len(password) < h.cfg.Security.PasswordMinLength {
		return false, echo.NewHTTPError(http.StatusBadRequest, "password is too short")
	}

	if userID == identity.UserID {
		currentPassword := ""
		if req.CurrentPassword != nil {
			currentPassword = strings.TrimSpace(*req.CurrentPassword)
		}
		if currentPassword == "" {
			return false, echo.NewHTTPError(http.StatusBadRequest, "current password is required")
		}
		user, userErr := h.users.GetByID(ctx, userID)
		if userErr != nil {
			return false, echo.NewHTTPError(http.StatusNotFound, "user not found")
		}
		if user.LDAPEnabled {
			return false, echo.NewHTTPError(http.StatusBadRequest, "password changes are managed by ldap")
		}
		ok, verifyErr := security.VerifyPassword(currentPassword, user.PasswordHash)
		if verifyErr != nil {
			return false, echo.NewHTTPError(http.StatusInternalServerError, "failed to verify current password")
		}
		if !ok {
			return false, echo.NewHTTPError(http.StatusUnauthorized, "current password is invalid")
		}
	} else {
		if !identity.IsPlatformAdmin && !managingOtherUser {
			return false, echo.NewHTTPError(http.StatusForbidden, "forbidden")
		}
		targetUser, targetUserErr := h.users.GetByID(ctx, userID)
		if targetUserErr != nil {
			return false, echo.NewHTTPError(http.StatusNotFound, "user not found")
		}
		if targetUser.LDAPEnabled {
			return false, echo.NewHTTPError(http.StatusBadRequest, "password changes are managed by ldap")
		}
	}

	hash, hashErr := security.HashPassword(password)
	if hashErr != nil {
		return false, echo.NewHTTPError(http.StatusInternalServerError, "failed to hash password")
	}
	if updateErr := h.users.UpdatePassword(ctx, userID, hash); updateErr != nil {
		return false, echo.NewHTTPError(http.StatusInternalServerError, "failed to update password")
	}
	return true, nil
}

func (h *Handler) maybeUpdateUserRole(
	ctx context.Context,
	identity models.Identity,
	userID uuid.UUID,
	roleToUpdate *models.TenantRole,
	managedTenantID uuid.UUID,
	tenantRaw string,
) (bool, uuid.UUID, error) {
	if roleToUpdate == nil {
		return false, uuid.Nil, nil
	}

	roleTenantID := managedTenantID
	if identity.IsPlatformAdmin {
		tenantRaw = strings.TrimSpace(tenantRaw)
		if tenantRaw == "" {
			return false, uuid.Nil, echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
		}
		parsedTenantID, tenantParseErr := uuid.Parse(tenantRaw)
		if tenantParseErr != nil {
			return false, uuid.Nil, echo.NewHTTPError(http.StatusBadRequest, "invalid tenant header")
		}
		roleTenantID = parsedTenantID
	}
	if roleTenantID == uuid.Nil {
		return false, uuid.Nil, echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}

	targetMembership, membershipErr := h.memberships.Get(ctx, roleTenantID, userID)
	if membershipErr != nil || !targetMembership.IsActive {
		return false, roleTenantID, echo.NewHTTPError(http.StatusNotFound, "user not found")
	}
	if !identity.IsPlatformAdmin && targetMembership.Role == models.TenantRoleAdmin && *roleToUpdate != models.TenantRoleAdmin {
		activeAdmins, countErr := h.memberships.CountActiveByTenantRole(ctx, roleTenantID, models.TenantRoleAdmin)
		if countErr != nil {
			return false, roleTenantID, echo.NewHTTPError(http.StatusInternalServerError, "failed to validate tenant admins")
		}
		if activeAdmins <= 1 {
			return false, roleTenantID, echo.NewHTTPError(http.StatusBadRequest, "cannot remove the last tenant admin")
		}
	}
	if upsertErr := h.memberships.Upsert(ctx, roleTenantID, userID, *roleToUpdate); upsertErr != nil {
		return false, roleTenantID, echo.NewHTTPError(http.StatusInternalServerError, "failed to update user role")
	}
	return true, roleTenantID, nil
}

func (h *Handler) updateUserProfileOrFetch(
	ctx context.Context,
	userID uuid.UUID,
	fullName *string,
	email *string,
	team *string,
	avatarURL *string,
	coverImageURL *string,
	personalLink *string,
) (*models.User, error) {
	if fullName == nil && email == nil && team == nil && avatarURL == nil && coverImageURL == nil && personalLink == nil {
		updated, err := h.users.GetByID(ctx, userID)
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusNotFound, "user not found")
		}
		return updated, nil
	}

	updated, err := h.users.UpdateProfile(ctx, userID, fullName, email, team, avatarURL, coverImageURL, personalLink)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "failed to update user")
	}
	return updated, nil
}

func (h *Handler) DeleteUser(c *echo.Context) error {
	identity, ok := middleware.GetIdentity(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}

	userID, parseErr := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if parseErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user id")
	}
	if userID == identity.UserID {
		return echo.NewHTTPError(http.StatusBadRequest, "cannot delete current user")
	}

	tenantRaw := strings.TrimSpace(c.Request().Header.Get(config.TenantHeader))
	if tenantRaw == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	tenantID, err := uuid.Parse(tenantRaw)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid tenant header")
	}

	ctx := c.Request().Context()
	if !identity.IsPlatformAdmin {
		requesterMembership, membershipErr := h.memberships.Get(ctx, tenantID, identity.UserID)
		if membershipErr != nil || !requesterMembership.IsActive || requesterMembership.Role != models.TenantRoleAdmin {
			return echo.NewHTTPError(http.StatusForbidden, "forbidden")
		}
	}

	targetMembership, err := h.memberships.Get(ctx, tenantID, userID)
	if err != nil || !targetMembership.IsActive {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}
	targetUser, err := h.users.GetByID(ctx, userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}
	if targetUser.IsPlatformAdmin && !identity.IsPlatformAdmin {
		return echo.NewHTTPError(http.StatusForbidden, "forbidden")
	}

	if !identity.IsPlatformAdmin && targetMembership.Role == models.TenantRoleAdmin {
		activeAdmins, countErr := h.memberships.CountActiveByTenantRole(ctx, tenantID, models.TenantRoleAdmin)
		if countErr != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to validate tenant admins")
		}
		if activeAdmins <= 1 {
			return echo.NewHTTPError(http.StatusBadRequest, "cannot remove the last tenant admin")
		}
	}

	if setErr := h.memberships.SetActive(ctx, tenantID, userID, false); setErr != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete user")
	}

	remainingMemberships, err := h.memberships.ListByUser(ctx, userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to validate user memberships")
	}
	if len(remainingMemberships) == 0 && !targetUser.IsPlatformAdmin {
		if disableErr := h.users.SetActive(ctx, userID, false); disableErr != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to disable user")
		}
		if h.refresh != nil {
			_ = h.refresh.RevokeAllByUser(ctx, userID)
		}
	}

	_ = h.audits.Log(ctx, &tenantID, &identity.UserID, "user_delete", "user", &userID, map[string]any{
		"tenant_id": tenantID.String(),
	})
	return c.NoContent(http.StatusNoContent)
}
