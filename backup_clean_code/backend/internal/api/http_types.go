package api

import "incidenthub/backend/internal/models"

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	AccessToken           string              `json:"access_token"`
	AccessTokenExpiresAt  string              `json:"access_token_expires_at"`
	RefreshTokenExpiresAt string              `json:"refresh_token_expires_at"`
	Identity              models.Identity     `json:"identity"`
	Memberships           []models.Membership `json:"memberships"`
}

type createTenantRequest struct {
	Slug        string  `json:"slug"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	MaxUsers    *int    `json:"max_users"`
	Responsible *string `json:"responsible_user_id"`
	IsActive    *bool   `json:"is_active"`
}

type updateTenantRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	MaxUsers    *int    `json:"max_users"`
	Responsible *string `json:"responsible_user_id"`
	Active      *bool   `json:"active"`
	IsActive    *bool   `json:"is_active"`
}

type createUserRequest struct {
	Username        string `json:"username"`
	Email           string `json:"email"`
	FullName        string `json:"full_name"`
	Password        string `json:"password"`
	TenantID        string `json:"tenant_id"`
	Role            string `json:"role"`
	IsPlatformAdmin bool   `json:"is_platform_admin"`
	LDAPEnabled     bool   `json:"ldap_enabled"`
}

type updateUserRequest struct {
	Name            *string `json:"name"`
	FullName        *string `json:"full_name"`
	Email           *string `json:"email"`
	Team            *string `json:"team"`
	Role            *string `json:"role"`
	AvatarURL       *string `json:"avatar_url"`
	Avatar          *string `json:"avatar"`
	CoverImage      *string `json:"cover_image"`
	CoverImageURL   *string `json:"cover_image_url"`
	PersonalLink    *string `json:"personal_link"`
	Password        *string `json:"password"`
	CurrentPassword *string `json:"current_password"`
}

type awardUserExperienceRequest struct {
	Points      int    `json:"points"`
	Description string `json:"description"`
}

type createAlertRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Source      string `json:"source"`
	Status      string `json:"status"`
	Severity    string `json:"severity"`
	TLP         string `json:"tlp"`
	PAP         string `json:"pap"`
}

type updateAlertRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Source      *string `json:"source"`
	Status      *string `json:"status"`
	Severity    *string `json:"severity"`
	TLP         *string `json:"tlp"`
	PAP         *string `json:"pap"`
	AssignedTo  *string `json:"assigned_to"`
}

type bulkBindAlertsToCaseRequest struct {
	AlertIDs []string `json:"alert_ids"`
	CaseID   string   `json:"case_id"`
}

type createCaseFromAlertsRequest struct {
	AlertIDs []string `json:"alert_ids"`
	Case     struct {
		CaseNumber        string `json:"case_number"`
		Title             string `json:"title"`
		Description       string `json:"description"`
		Source            string `json:"source"`
		IncidentType      string `json:"incident_type"`
		Status            string `json:"status"`
		Priority          string `json:"priority"`
		Impact            string `json:"impact"`
		Confidence        *int   `json:"confidence"`
		Severity          string `json:"severity"`
		TLP               string `json:"tlp"`
		PAP               string `json:"pap"`
		DetectedAt        string `json:"detected_at"`
		OccurredAt        string `json:"occurred_at"`
		ClosedAt          string `json:"closed_at"`
		ResolutionSummary string `json:"resolution_summary"`
		AssignedTo        string `json:"assigned_to"`
	} `json:"case"`
}

type caseStatusItemRequest struct {
	Code     string `json:"code"`
	Label    string `json:"label"`
	Order    int    `json:"order"`
	IsClosed bool   `json:"is_closed"`
	Color    string `json:"color"`
}

type upsertCaseStatusesRequest struct {
	Statuses []caseStatusItemRequest `json:"statuses"`
}

type createCaseRequest struct {
	CaseNumber        string `json:"case_number"`
	Title             string `json:"title"`
	Description       string `json:"description"`
	Source            string `json:"source"`
	IncidentType      string `json:"incident_type"`
	Status            string `json:"status"`
	Priority          string `json:"priority"`
	Impact            string `json:"impact"`
	Confidence        *int   `json:"confidence"`
	Severity          string `json:"severity"`
	TLP               string `json:"tlp"`
	PAP               string `json:"pap"`
	DetectedAt        string `json:"detected_at"`
	OccurredAt        string `json:"occurred_at"`
	ClosedAt          string `json:"closed_at"`
	ResolutionSummary string `json:"resolution_summary"`
	AssignedTo        string `json:"assigned_to"`
}

type updateCaseRequest struct {
	CaseNumber        *string `json:"case_number"`
	Title             *string `json:"title"`
	Description       *string `json:"description"`
	Source            *string `json:"source"`
	IncidentType      *string `json:"incident_type"`
	Status            *string `json:"status"`
	Priority          *string `json:"priority"`
	Impact            *string `json:"impact"`
	Confidence        *int    `json:"confidence"`
	Severity          *string `json:"severity"`
	TLP               *string `json:"tlp"`
	PAP               *string `json:"pap"`
	DetectedAt        *string `json:"detected_at"`
	OccurredAt        *string `json:"occurred_at"`
	ClosedAt          *string `json:"closed_at"`
	ExpectedUpdatedAt *string `json:"expected_updated_at"`
	ResolutionSummary *string `json:"resolution_summary"`
	AssignedTo        *string `json:"assigned_to"`
}

type copyCaseRequest struct {
	Title              string  `json:"title"`
	CaseNumber         string  `json:"case_number"`
	AssignedTo         *string `json:"assigned_to"`
	IncludeObservables *bool   `json:"include_observables"`
}

type escalateCaseRequest struct {
	TargetTenantID     string `json:"target_tenant_id"`
	TargetTenantSlug   string `json:"target_tenant_slug"`
	TargetAssigneeID   string `json:"target_assignee_id"`
	HandoffType        string `json:"handoff_type"`
	Summary            string `json:"summary"`
	IncludeObservables *bool  `json:"include_observables"`
}

type shareCaseRequest struct {
	TargetTenantID   string `json:"target_tenant_id"`
	TargetTenantSlug string `json:"target_tenant_slug"`
}

type createTaskRequest struct {
	CaseID      string  `json:"case_id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Status      string  `json:"status"`
	AssigneeID  string  `json:"assignee_id"`
	DueDate     *string `json:"due_date"`
}

type updateTaskRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
	AssigneeID  *string `json:"assignee_id"`
	DueDate     *string `json:"due_date"`
}

type createObservableRequest struct {
	Type    string   `json:"type"`
	Value   string   `json:"value"`
	Verdict string   `json:"verdict"`
	Source  string   `json:"source"`
	Tags    []string `json:"tags"`
}

type updateObservableRequest struct {
	Type    *string   `json:"type"`
	Value   *string   `json:"value"`
	Verdict *string   `json:"verdict"`
	Source  *string   `json:"source"`
	Tags    *[]string `json:"tags"`
}

type createCaseEventRequest struct {
	EventType string         `json:"event_type"`
	Title     string         `json:"title"`
	Body      string         `json:"body"`
	Metadata  map[string]any `json:"metadata"`
}

type createCasePageRequest struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type createCaseAttachmentRequest struct {
	FileName       string `json:"file_name"`
	ContentType    string `json:"content_type"`
	FileSizeBytes  int64  `json:"file_size_bytes"`
	StorageKey     string `json:"storage_key"`
	ChecksumSHA256 string `json:"checksum_sha256"`
}

type createCatalogItemRequest struct {
	OwnerID string         `json:"owner_id"`
	RefID   string         `json:"ref_id"`
	Data    map[string]any `json:"data"`
}

type updateCatalogItemRequest struct {
	Data map[string]any `json:"data"`
}
