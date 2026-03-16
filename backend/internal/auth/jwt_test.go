package auth

import (
	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/models"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestJWTGenerateAndParse(t *testing.T) {
	t.Parallel()

	svc := New(config.AuthConfig{
		JWTSecret:  "test-secret",
		Issuer:     "incidenthub-test",
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 24 * time.Hour,
	})

	tenantID := uuid.New()
	identity := models.Identity{
		UserID:          uuid.New(),
		Username:        "analyst",
		IsPlatformAdmin: false,
		TenantID:        &tenantID,
		TenantRole:      models.TenantRoleAnalyst,
	}

	tokens, err := svc.Generate(identity)
	if err != nil {
		t.Fatalf("generate tokens: %v", err)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatalf("expected non-empty tokens")
	}

	claims, err := svc.ParseAccess(tokens.AccessToken)
	if err != nil {
		t.Fatalf("parse access: %v", err)
	}
	parsed, err := IdentityFromClaims(claims)
	if err != nil {
		t.Fatalf("claims to identity: %v", err)
	}

	if parsed.UserID != identity.UserID {
		t.Fatalf("unexpected user id")
	}
	if parsed.TenantID == nil || *parsed.TenantID != tenantID {
		t.Fatalf("unexpected tenant id")
	}
}
