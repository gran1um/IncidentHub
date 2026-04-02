package repository

import (
	"context"
	"fmt"
	"time"

	"incidenthub/backend/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CreateUserParams struct {
	Username        string
	Email           string
	FullName        string
	PasswordHash    string
	IsPlatformAdmin bool
	LDAPEnabled     bool
}

type UserRepository struct {
	pool *pgxpool.Pool
}

const userSelectColumns = `
	id, username, email, full_name, team,
	avatar_url, cover_image_url, personal_link,
	experience_points, password_hash, is_active,
	is_platform_admin, ldap_enabled, mfa_enabled,
	created_at, updated_at
`

func scanUser(scanner rowScanner) (*models.User, error) {
	var u models.User
	if scanErr := scanner.Scan(
		&u.ID,
		&u.Username,
		&u.Email,
		&u.FullName,
		&u.Team,
		&u.AvatarURL,
		&u.CoverImageURL,
		&u.PersonalLink,
		&u.ExperiencePoints,
		&u.PasswordHash,
		&u.IsActive,
		&u.IsPlatformAdmin,
		&u.LDAPEnabled,
		&u.MFAEnabled,
		&u.CreatedAt,
		&u.UpdatedAt,
	); scanErr != nil {
		return nil, scanErr
	}
	return &u, nil
}

func scanUserWithoutPassword(scanner rowScanner) (*models.User, error) {
	u, err := scanUser(scanner)
	if err != nil {
		return nil, err
	}
	u.PasswordHash = ""
	return u, nil
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) Create(ctx context.Context, p CreateUserParams) (*models.User, error) {
	q := `
		INSERT INTO users(username, email, full_name, password_hash, is_platform_admin, ldap_enabled)
		VALUES($1,$2,$3,$4,$5,$6)
		RETURNING
		` + userSelectColumns + `
	`
	row := r.pool.QueryRow(ctx, q,
		p.Username,
		p.Email,
		p.FullName,
		p.PasswordHash,
		p.IsPlatformAdmin,
		p.LDAPEnabled,
	)
	u, err := scanUser(row)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	q := `
		SELECT
		` + userSelectColumns + `
		FROM users
		WHERE email = $1
	`
	u, err := scanUser(r.pool.QueryRow(ctx, q, email))
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return u, nil
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	q := `
		SELECT
		` + userSelectColumns + `
		FROM users
		WHERE username = $1
	`
	u, err := scanUser(r.pool.QueryRow(ctx, q, username))
	if err != nil {
		return nil, fmt.Errorf("get user by username: %w", err)
	}
	return u, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	q := `
		SELECT
		` + userSelectColumns + `
		FROM users
		WHERE id = $1
	`
	u, err := scanUser(r.pool.QueryRow(ctx, q, id))
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return u, nil
}

func (r *UserRepository) List(ctx context.Context, limit, offset int) ([]models.User, error) {
	limit = normalizeLimit(limit, 200, 50)
	offset = normalizeOffset(offset)
	q := `
		SELECT
		` + userSelectColumns + `
		FROM users
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.pool.Query(ctx, q, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	items, err := collectRows(rows, limit, scanUserWithoutPassword, "scan users", "iterate users")
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (r *UserRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]models.User, error) {
	limit = normalizeLimit(limit, 200, 50)
	offset = normalizeOffset(offset)
	q := `
		SELECT
		u.id, u.username, u.email, u.full_name, u.team,
		u.avatar_url, u.cover_image_url, u.personal_link,
		u.experience_points, u.password_hash, u.is_active,
		u.is_platform_admin, u.ldap_enabled, u.mfa_enabled,
		u.created_at, u.updated_at
		FROM users u
		INNER JOIN tenant_memberships tm ON tm.user_id = u.id
		WHERE tm.tenant_id = $1 AND tm.is_active = TRUE
		ORDER BY tm.created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.pool.Query(ctx, q, tenantID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list tenant users: %w", err)
	}
	items, err := collectRows(rows, limit, scanUserWithoutPassword, "scan tenant users", "iterate tenant users")
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (r *UserRepository) ListByTenantWithRole(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]models.TenantUser, error) {
	limit = normalizeLimit(limit, 300, 100)
	offset = normalizeOffset(offset)
	q := `
		SELECT u.id, tm.tenant_id, tm.role,
			u.username, u.email, u.full_name, u.team, u.avatar_url, u.cover_image_url, u.personal_link, u.experience_points,
			u.is_active, u.is_platform_admin, u.ldap_enabled, u.mfa_enabled, u.created_at, u.updated_at
		FROM users u
		INNER JOIN tenant_memberships tm ON tm.user_id = u.id
		WHERE tm.tenant_id = $1 AND tm.is_active = TRUE
		ORDER BY tm.created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.pool.Query(ctx, q, tenantID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list tenant users with role: %w", err)
	}
	items, err := collectRows(rows, limit, func(scanner rowScanner) (*models.TenantUser, error) {
		var item models.TenantUser
		if scanErr := scanner.Scan(
			&item.ID,
			&item.TenantID,
			&item.Role,
			&item.Username,
			&item.Email,
			&item.FullName,
			&item.Team,
			&item.AvatarURL,
			&item.CoverImageURL,
			&item.PersonalLink,
			&item.ExperiencePoints,
			&item.IsActive,
			&item.IsPlatformAdmin,
			&item.LDAPEnabled,
			&item.MFAEnabled,
			&item.CreatedAt,
			&item.UpdatedAt,
		); scanErr != nil {
			return nil, scanErr
		}
		return &item, nil
	}, "scan tenant users with role", "iterate tenant users with role")
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (r *UserRepository) TouchLogin(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET updated_at=$2 WHERE id=$1`, id, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("touch user login: %w", err)
	}
	return nil
}

func (r *UserRepository) UpdateProfile(ctx context.Context, id uuid.UUID, fullName, email, team, avatarURL, coverImageURL, personalLink *string) (*models.User, error) {
	q := `
		UPDATE users
		SET
			full_name = COALESCE($2, full_name),
			email = COALESCE($3, email),
			team = COALESCE($4, team),
			avatar_url = COALESCE($5, avatar_url),
			cover_image_url = COALESCE($6, cover_image_url),
			personal_link = COALESCE($7, personal_link),
			updated_at = NOW()
		WHERE id = $1
		RETURNING
		` + userSelectColumns + `
	`
	u, err := scanUser(r.pool.QueryRow(ctx, q, id, fullName, email, team, avatarURL, coverImageURL, personalLink))
	if err != nil {
		return nil, fmt.Errorf("update user profile: %w", err)
	}
	u.PasswordHash = ""
	return u, nil
}

func (r *UserRepository) UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET password_hash=$2, updated_at=NOW() WHERE id=$1`, id, passwordHash)
	if err != nil {
		return fmt.Errorf("update user password: %w", err)
	}
	return nil
}

func (r *UserRepository) SetPlatformAdmin(ctx context.Context, id uuid.UUID, enabled bool) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET is_platform_admin=$2, updated_at=NOW() WHERE id=$1`, id, enabled)
	if err != nil {
		return fmt.Errorf("update platform admin flag: %w", err)
	}
	return nil
}

func (r *UserRepository) SetActive(ctx context.Context, id uuid.UUID, active bool) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET is_active=$2, updated_at=NOW() WHERE id=$1`, id, active)
	if err != nil {
		return fmt.Errorf("update user active flag: %w", err)
	}
	return nil
}
