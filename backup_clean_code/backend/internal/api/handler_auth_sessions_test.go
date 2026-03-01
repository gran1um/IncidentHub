package api

import (
	"context"
	"encoding/json"
	"errors"
	"incidenthub/backend/internal/auth"
	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/models"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

type refreshTokenStoreMock struct {
	listByUser       []models.RefreshToken
	listByUserErr    error
	activeByHash     map[string]*models.RefreshToken
	getActiveErr     error
	revokeOthersErr  error
	revokeOthersRows int64

	lastUserID   uuid.UUID
	lastKeepHash string
}

func (m *refreshTokenStoreMock) Create(context.Context, models.RefreshToken) error {
	return nil
}

func (m *refreshTokenStoreMock) GetActiveByHash(_ context.Context, tokenHash string) (*models.RefreshToken, error) {
	if m.getActiveErr != nil {
		return nil, m.getActiveErr
	}
	if token, ok := m.activeByHash[tokenHash]; ok {
		return token, nil
	}
	return nil, errors.New("refresh token not found")
}

func (m *refreshTokenStoreMock) RevokeByHash(context.Context, string) error {
	return nil
}

func (m *refreshTokenStoreMock) RevokeAllByUser(context.Context, uuid.UUID) error {
	return nil
}

func (m *refreshTokenStoreMock) RevokeAllByUserExceptHash(_ context.Context, userID uuid.UUID, keepTokenHash string) (int64, error) {
	m.lastUserID = userID
	m.lastKeepHash = keepTokenHash
	if m.revokeOthersErr != nil {
		return 0, m.revokeOthersErr
	}
	return m.revokeOthersRows, nil
}

func (m *refreshTokenStoreMock) ListByUser(_ context.Context, _ uuid.UUID, _ int) ([]models.RefreshToken, error) {
	if m.listByUserErr != nil {
		return nil, m.listByUserErr
	}
	return m.listByUser, nil
}

func TestListAuthSessionsReturnsCurrentStatusAndTimeout(t *testing.T) {
	e := echo.New()
	userID := uuid.New()
	currentRaw := "refresh-current"
	currentHash := auth.RefreshHash(currentRaw)
	now := time.Now().UTC()
	revokedAt := now.Add(-1 * time.Hour)

	mockStore := &refreshTokenStoreMock{
		listByUser: []models.RefreshToken{
			{
				ID:        uuid.New(),
				UserID:    userID,
				TokenHash: currentHash,
				UserAgent: "Chrome",
				IPAddress: "10.0.0.1",
				CreatedAt: now.Add(-2 * time.Hour),
				ExpiresAt: now.Add(6 * time.Hour),
			},
			{
				ID:        uuid.New(),
				UserID:    userID,
				TokenHash: "expired",
				UserAgent: "Firefox",
				IPAddress: "10.0.0.2",
				CreatedAt: now.Add(-8 * time.Hour),
				ExpiresAt: now.Add(-30 * time.Minute),
			},
			{
				ID:        uuid.New(),
				UserID:    userID,
				TokenHash: "revoked",
				UserAgent: "Safari",
				IPAddress: "10.0.0.3",
				CreatedAt: now.Add(-3 * time.Hour),
				ExpiresAt: now.Add(5 * time.Hour),
				RevokedAt: &revokedAt,
			},
		},
	}

	h := &Handler{
		cfg: config.App{
			Auth: config.AuthConfig{
				CookieName: "ih_refresh_token",
				RefreshTTL: 12 * time.Hour,
			},
		},
		refresh: mockStore,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/sessions?limit=20", http.NoBody)
	req.AddCookie(&http.Cookie{Name: "ih_refresh_token", Value: currentRaw})
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("identity", models.Identity{UserID: userID})

	if err := h.ListAuthSessions(c); err != nil {
		t.Fatalf("ListAuthSessions returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var payload listAuthSessionsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}
	if len(payload.Sessions) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(payload.Sessions))
	}
	if payload.ActiveSessions != 1 {
		t.Fatalf("expected 1 active session, got %d", payload.ActiveSessions)
	}
	if payload.SessionTimeoutSeconds != int64((12 * time.Hour).Seconds()) {
		t.Fatalf("unexpected timeout seconds: %d", payload.SessionTimeoutSeconds)
	}

	var current *authSessionResponse
	var hasExpired bool
	var hasRevoked bool
	for idx := range payload.Sessions {
		item := payload.Sessions[idx]
		switch {
		case item.IsCurrent:
			current = &item
		case item.Status == refreshSessionStatusExpired:
			hasExpired = true
		case item.Status == refreshSessionStatusRevoked:
			hasRevoked = true
		}
	}
	if current == nil {
		t.Fatalf("expected current session in payload")
	}
	if current.Status != refreshSessionStatusActive {
		t.Fatalf("expected current status active, got %s", current.Status)
	}
	if current.RemainingSeconds <= 0 {
		t.Fatalf("expected remaining seconds > 0 for current session")
	}
	if !hasExpired || !hasRevoked {
		t.Fatalf("expected expired and revoked sessions in payload")
	}
}

func TestRevokeOtherAuthSessionsRequiresCookie(t *testing.T) {
	e := echo.New()
	h := &Handler{
		cfg: config.App{
			Auth: config.AuthConfig{CookieName: "ih_refresh_token"},
		},
		refresh: &refreshTokenStoreMock{},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/sessions/revoke-others", http.NoBody)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("identity", models.Identity{UserID: uuid.New()})

	err := h.RevokeOtherAuthSessions(c)
	if err == nil {
		t.Fatalf("expected error when cookie is missing")
	}
	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected *echo.HTTPError, got %T", err)
	}
	if httpErr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", httpErr.Code)
	}
}

func TestRevokeOtherAuthSessionsRevokesAllExceptCurrent(t *testing.T) {
	e := echo.New()
	userID := uuid.New()
	currentRaw := "refresh-current"
	currentHash := auth.RefreshHash(currentRaw)
	mockStore := &refreshTokenStoreMock{
		activeByHash: map[string]*models.RefreshToken{
			currentHash: {
				ID:        uuid.New(),
				UserID:    userID,
				TokenHash: currentHash,
				CreatedAt: time.Now().UTC().Add(-1 * time.Hour),
				ExpiresAt: time.Now().UTC().Add(2 * time.Hour),
			},
		},
		revokeOthersRows: 2,
	}
	h := &Handler{
		cfg: config.App{
			Auth: config.AuthConfig{CookieName: "ih_refresh_token"},
		},
		refresh: mockStore,
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/sessions/revoke-others", http.NoBody)
	req.AddCookie(&http.Cookie{Name: "ih_refresh_token", Value: currentRaw})
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("identity", models.Identity{UserID: userID})

	if err := h.RevokeOtherAuthSessions(c); err != nil {
		t.Fatalf("RevokeOtherAuthSessions returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if mockStore.lastUserID != userID {
		t.Fatalf("expected revoke to target user %s, got %s", userID, mockStore.lastUserID)
	}
	if mockStore.lastKeepHash != currentHash {
		t.Fatalf("expected keep hash %s, got %s", currentHash, mockStore.lastKeepHash)
	}

	var payload map[string]int64
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload["revoked"] != 2 {
		t.Fatalf("expected revoked=2, got %d", payload["revoked"])
	}
}
