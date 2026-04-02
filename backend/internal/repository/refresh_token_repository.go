package repository

import (
	"context"
	"fmt"
	"incidenthub/backend/internal/models"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RefreshTokenRepository struct {
	pool *pgxpool.Pool
}

func NewRefreshTokenRepository(pool *pgxpool.Pool) *RefreshTokenRepository {
	return &RefreshTokenRepository{pool: pool}
}

func (r *RefreshTokenRepository) Create(ctx context.Context, token models.RefreshToken) error {
	q := `
		INSERT INTO refresh_tokens(user_id, token_hash, expires_at, user_agent, ip_address)
		VALUES($1,$2,$3,$4,$5)
	`
	if _, err := r.pool.Exec(ctx, q, token.UserID, token.TokenHash, token.ExpiresAt, token.UserAgent, token.IPAddress); err != nil {
		return fmt.Errorf("create refresh token: %w", err)
	}
	return nil
}

func (r *RefreshTokenRepository) GetActiveByHash(ctx context.Context, tokenHash string) (*models.RefreshToken, error) {
	q := `
		SELECT id, user_id, token_hash, expires_at, revoked_at, user_agent, ip_address, created_at
		FROM refresh_tokens
		WHERE token_hash=$1 AND revoked_at IS NULL AND expires_at > NOW()
	`
	var t models.RefreshToken
	if err := r.pool.QueryRow(ctx, q, tokenHash).Scan(
		&t.ID,
		&t.UserID,
		&t.TokenHash,
		&t.ExpiresAt,
		&t.RevokedAt,
		&t.UserAgent,
		&t.IPAddress,
		&t.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("get active refresh token: %w", err)
	}
	return &t, nil
}

func (r *RefreshTokenRepository) RevokeByHash(ctx context.Context, tokenHash string) error {
	q := `UPDATE refresh_tokens SET revoked_at=$2 WHERE token_hash=$1 AND revoked_at IS NULL`
	if _, err := r.pool.Exec(ctx, q, tokenHash, time.Now().UTC()); err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}

func (r *RefreshTokenRepository) RevokeAllByUser(ctx context.Context, userID uuid.UUID) error {
	q := `UPDATE refresh_tokens SET revoked_at=$2 WHERE user_id=$1 AND revoked_at IS NULL`
	if _, err := r.pool.Exec(ctx, q, userID, time.Now().UTC()); err != nil {
		return fmt.Errorf("revoke all refresh tokens by user: %w", err)
	}
	return nil
}

func (r *RefreshTokenRepository) RevokeAllByUserExceptHash(ctx context.Context, userID uuid.UUID, keepTokenHash string) (int64, error) {
	q := `
		UPDATE refresh_tokens
		SET revoked_at = $3
		WHERE user_id = $1
		  AND revoked_at IS NULL
		  AND expires_at > NOW()
		  AND token_hash <> $2
	`
	res, err := r.pool.Exec(ctx, q, userID, keepTokenHash, time.Now().UTC())
	if err != nil {
		return 0, fmt.Errorf("revoke all refresh tokens except hash: %w", err)
	}
	return res.RowsAffected(), nil
}

func (r *RefreshTokenRepository) ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]models.RefreshToken, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := `
		SELECT id, user_id, token_hash, expires_at, revoked_at, user_agent, ip_address, created_at
		FROM refresh_tokens
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`
	rows, err := r.pool.Query(ctx, q, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list refresh tokens by user: %w", err)
	}
	defer rows.Close()

	out := make([]models.RefreshToken, 0, limit)
	for rows.Next() {
		var token models.RefreshToken
		if err := rows.Scan(
			&token.ID,
			&token.UserID,
			&token.TokenHash,
			&token.ExpiresAt,
			&token.RevokedAt,
			&token.UserAgent,
			&token.IPAddress,
			&token.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan refresh token row: %w", err)
		}
		out = append(out, token)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate refresh token rows: %w", err)
	}
	return out, nil
}
