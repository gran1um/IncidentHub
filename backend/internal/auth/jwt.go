package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/models"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	UserID          string `json:"uid"`
	Username        string `json:"username"`
	IsPlatformAdmin bool   `json:"platform_admin"`
	TenantID        string `json:"tenant_id,omitempty"`
	TenantRole      string `json:"tenant_role,omitempty"`
	jwt.RegisteredClaims
}

type Tokens struct {
	AccessToken           string    `json:"access_token"`
	RefreshToken          string    `json:"refresh_token"`
	AccessTokenExpiresAt  time.Time `json:"access_token_expires_at"`
	RefreshTokenExpiresAt time.Time `json:"refresh_token_expires_at"`
}

type Service struct {
	cfg config.AuthConfig
}

func New(cfg config.AuthConfig) *Service {
	return &Service{cfg: cfg}
}

func (s *Service) Generate(identity models.Identity) (Tokens, error) {
	now := time.Now().UTC()
	accessExp := now.Add(s.cfg.AccessTTL)
	refreshExp := now.Add(s.cfg.RefreshTTL)

	claims := Claims{
		UserID:          identity.UserID.String(),
		Username:        identity.Username,
		IsPlatformAdmin: identity.IsPlatformAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.cfg.Issuer,
			Subject:   identity.UserID.String(),
			ExpiresAt: jwt.NewNumericDate(accessExp),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
	if identity.TenantID != nil {
		claims.TenantID = identity.TenantID.String()
		claims.TenantRole = string(identity.TenantRole)
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedAccess, err := accessToken.SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return Tokens{}, fmt.Errorf("sign access token: %w", err)
	}

	refreshToken, err := newRefreshToken()
	if err != nil {
		return Tokens{}, err
	}

	return Tokens{
		AccessToken:           signedAccess,
		RefreshToken:          refreshToken,
		AccessTokenExpiresAt:  accessExp,
		RefreshTokenExpiresAt: refreshExp,
	}, nil
}

func (s *Service) ParseAccess(token string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(s.cfg.JWTSecret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse jwt: %w", err)
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, fmt.Errorf("invalid jwt claims")
	}
	return claims, nil
}

func RefreshHash(token string) string {
	h := sha256.Sum256([]byte(token))
	return base64.RawStdEncoding.EncodeToString(h[:])
}

func IdentityFromClaims(c *Claims) (models.Identity, error) {
	uid, err := uuid.Parse(c.UserID)
	if err != nil {
		return models.Identity{}, fmt.Errorf("invalid claim user id: %w", err)
	}

	identity := models.Identity{
		UserID:          uid,
		Username:        c.Username,
		IsPlatformAdmin: c.IsPlatformAdmin,
		TenantRole:      models.TenantRole(c.TenantRole),
	}
	if c.TenantID != "" {
		tid, err := uuid.Parse(c.TenantID)
		if err != nil {
			return models.Identity{}, fmt.Errorf("invalid claim tenant id: %w", err)
		}
		identity.TenantID = &tid
	}
	return identity, nil
}

func newRefreshToken() (string, error) {
	buf := make([]byte, 48)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}
	return base64.RawStdEncoding.EncodeToString(buf), nil
}
