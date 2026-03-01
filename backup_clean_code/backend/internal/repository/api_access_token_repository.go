package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"incidenthub/backend/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type APIAccessTokenRepository struct {
	pool *pgxpool.Pool
}

type CreateAPIAccessTokenParams struct {
	TenantID    uuid.UUID
	Name        string
	Description string
	TokenHash   string
	TokenPrefix string
	Scopes      []string
	FullAccess  bool
	CreatedBy   uuid.UUID
	ExpiresAt   *time.Time
}

func NewAPIAccessTokenRepository(pool *pgxpool.Pool) *APIAccessTokenRepository {
	return &APIAccessTokenRepository{pool: pool}
}

func (r *APIAccessTokenRepository) Create(ctx context.Context, p CreateAPIAccessTokenParams) (*models.APIAccessToken, error) {
	scopes := normalizeTokenScopes(p.Scopes)
	row := r.pool.QueryRow(ctx, `
		INSERT INTO api_access_tokens(
			tenant_id, name, description, token_hash, token_prefix, scopes, full_access, created_by, expires_at
		)
		VALUES ($1, $2, $3, $4, $5, $6::text[], $7, $8, $9)
		RETURNING id, tenant_id, name, description, token_hash, token_prefix, scopes, full_access, created_by,
		          expires_at, last_used_at, revoked_at, created_at, updated_at
	`, p.TenantID, strings.TrimSpace(p.Name), strings.TrimSpace(p.Description), strings.TrimSpace(p.TokenHash), strings.TrimSpace(p.TokenPrefix), scopes, p.FullAccess, p.CreatedBy, p.ExpiresAt)
	item, err := scanAPIAccessToken(row)
	if err != nil {
		return nil, fmt.Errorf("create api access token: %w", err)
	}
	return item, nil
}

func (r *APIAccessTokenRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID, includeRevoked bool, limit int) ([]models.APIAccessToken, error) {
	return listByTenantWithOptionalClause(
		ctx,
		r.pool,
		`
			SELECT id, tenant_id, name, description, token_hash, token_prefix, scopes, full_access, created_by,
			       expires_at, last_used_at, revoked_at, created_at, updated_at
			FROM api_access_tokens
			WHERE tenant_id = $1
		`,
		tenantID,
		includeRevoked,
		"AND revoked_at IS NULL",
		"ORDER BY created_at DESC LIMIT $2",
		limit,
		1000,
		200,
		scanAPIAccessToken,
		"list api access tokens",
		"scan api access token",
		"iterate api access tokens",
	)
}

func (r *APIAccessTokenRepository) GetActiveByHash(ctx context.Context, tokenHash string) (*models.APIAccessToken, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, name, description, token_hash, token_prefix, scopes, full_access, created_by,
		       expires_at, last_used_at, revoked_at, created_at, updated_at
		FROM api_access_tokens
		WHERE token_hash = $1
		  AND revoked_at IS NULL
		  AND (expires_at IS NULL OR expires_at > NOW())
	`, strings.TrimSpace(tokenHash))
	item, err := scanAPIAccessToken(row)
	if err != nil {
		return nil, fmt.Errorf("get api access token by hash: %w", err)
	}
	return item, nil
}

func (r *APIAccessTokenRepository) TouchLastUsed(ctx context.Context, tokenID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE api_access_tokens
		SET last_used_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
	`, tokenID)
	if err != nil {
		return fmt.Errorf("touch api access token last_used_at: %w", err)
	}
	return nil
}

func (r *APIAccessTokenRepository) Revoke(ctx context.Context, tenantID, tokenID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE api_access_tokens
		SET revoked_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
		  AND tenant_id = $2
		  AND revoked_at IS NULL
	`, tokenID, tenantID)
	if err != nil {
		return fmt.Errorf("revoke api access token: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func scanAPIAccessToken(scanner rowScanner) (*models.APIAccessToken, error) {
	var item models.APIAccessToken
	var scopes []string
	err := scanner.Scan(
		&item.ID,
		&item.TenantID,
		&item.Name,
		&item.Description,
		&item.TokenHash,
		&item.TokenPrefix,
		&scopes,
		&item.FullAccess,
		&item.CreatedBy,
		&item.ExpiresAt,
		&item.LastUsedAt,
		&item.RevokedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	item.Scopes = normalizeTokenScopes(scopes)
	return &item, nil
}

func normalizeTokenScopes(scopes []string) []string {
	if len(scopes) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(scopes))
	out := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		normalized := strings.ToLower(strings.TrimSpace(scope))
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}
