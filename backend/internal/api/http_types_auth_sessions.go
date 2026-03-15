package api

type authSessionResponse struct {
	ID               string  `json:"id"`
	UserAgent        string  `json:"user_agent"`
	IPAddress        string  `json:"ip_address"`
	CreatedAt        string  `json:"created_at"`
	ExpiresAt        string  `json:"expires_at"`
	RevokedAt        *string `json:"revoked_at,omitempty"`
	Status           string  `json:"status"`
	IsCurrent        bool    `json:"is_current"`
	DurationSeconds  int64   `json:"duration_seconds"`
	RemainingSeconds int64   `json:"remaining_seconds"`
}

type listAuthSessionsResponse struct {
	Sessions              []authSessionResponse `json:"sessions"`
	ActiveSessions        int                   `json:"active_sessions"`
	SessionTimeoutSeconds int64                 `json:"session_timeout_seconds"`
}
