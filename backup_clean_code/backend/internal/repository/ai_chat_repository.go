package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"incidenthub/backend/internal/models"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AIChatRepository struct {
	pool *pgxpool.Pool
}

type CreateAIChatMessageParams struct {
	SessionID uuid.UUID
	TenantID  uuid.UUID
	UserID    uuid.UUID
	Role      string
	Content   string
	Sources   []map[string]any
	Metadata  map[string]any
}

type CreateAIChatSessionParams struct {
	TenantID uuid.UUID
	UserID   uuid.UUID
	Title    string
}

const (
	defaultAIChatSessionTitle = "SOC Assistant"
	maxAIChatSessionTitleLen  = 160
)

func NewAIChatRepository(pool *pgxpool.Pool) *AIChatRepository {
	return &AIChatRepository{pool: pool}
}

func (r *AIChatRepository) GetOrCreateDefaultSession(ctx context.Context, tenantID, userID uuid.UUID) (*models.AIChatSession, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, user_id, title, is_default, sort_order,
		       '' AS last_message_preview,
		       '' AS last_message_role,
		       updated_at AS last_message_at,
		       created_at, updated_at
		FROM ai_chat_sessions
		WHERE tenant_id = $1 AND user_id = $2 AND is_default = TRUE
		LIMIT 1
	`, tenantID, userID)

	session, err := scanAIChatSession(row)
	if err == nil {
		return session, nil
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("get default ai chat session: %w", err)
	}

	row = r.pool.QueryRow(ctx, `
		INSERT INTO ai_chat_sessions(tenant_id, user_id, title, is_default, sort_order)
		SELECT $1, $2, $3, TRUE, COALESCE(MAX(sort_order), 0) + 1
		FROM ai_chat_sessions
		WHERE tenant_id = $1 AND user_id = $2
		ON CONFLICT (tenant_id, user_id) WHERE is_default
		DO UPDATE SET updated_at = NOW()
		RETURNING id, tenant_id, user_id, title, is_default, sort_order,
		          '' AS last_message_preview,
		          '' AS last_message_role,
		          updated_at AS last_message_at,
		          created_at, updated_at
	`, tenantID, userID, defaultAIChatSessionTitle)
	session, err = scanAIChatSession(row)
	if err != nil {
		return nil, fmt.Errorf("create default ai chat session: %w", err)
	}
	return session, nil
}

func (r *AIChatRepository) ListSessions(ctx context.Context, tenantID, userID uuid.UUID, limit int) ([]models.AIChatSession, error) {
	limit = normalizeLimit(limit, 200, 30)

	rows, err := r.pool.Query(ctx, `
		SELECT s.id, s.tenant_id, s.user_id, s.title, s.is_default, s.sort_order,
		       COALESCE(LEFT(last_message.content, 260), '') AS last_message_preview,
		       COALESCE(last_message.role, '') AS last_message_role,
		       COALESCE(last_message.created_at, s.updated_at) AS last_message_at,
		       s.created_at, s.updated_at
		FROM ai_chat_sessions s
		LEFT JOIN LATERAL (
			SELECT role, content, created_at
			FROM ai_chat_messages
			WHERE tenant_id = $1 AND user_id = $2 AND session_id = s.id
			ORDER BY created_at DESC
			LIMIT 1
		) AS last_message ON TRUE
		WHERE s.tenant_id = $1 AND s.user_id = $2
		ORDER BY s.sort_order DESC, COALESCE(last_message.created_at, s.updated_at) DESC, s.created_at DESC
		LIMIT $3
	`, tenantID, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list ai chat sessions: %w", err)
	}

	return collectRows(rows, limit, scanAIChatSession, "scan ai chat session", "iterate ai chat sessions")
}

func (r *AIChatRepository) CreateSession(ctx context.Context, p CreateAIChatSessionParams) (*models.AIChatSession, error) {
	title := normalizeAIChatSessionTitle(p.Title)

	row := r.pool.QueryRow(ctx, `
		INSERT INTO ai_chat_sessions(tenant_id, user_id, title, is_default, sort_order)
		SELECT $1, $2, $3, FALSE, COALESCE(MAX(sort_order), 0) + 1
		FROM ai_chat_sessions
		WHERE tenant_id = $1 AND user_id = $2
		RETURNING id, tenant_id, user_id, title, is_default, sort_order,
		          '' AS last_message_preview,
		          '' AS last_message_role,
		          updated_at AS last_message_at,
		          created_at, updated_at
	`, p.TenantID, p.UserID, title)

	item, err := scanAIChatSession(row)
	if err != nil {
		return nil, fmt.Errorf("create ai chat session: %w", err)
	}
	return item, nil
}

func (r *AIChatRepository) GetSessionByID(ctx context.Context, tenantID, userID, sessionID uuid.UUID) (*models.AIChatSession, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, user_id, title, is_default, sort_order,
		       '' AS last_message_preview,
		       '' AS last_message_role,
		       updated_at AS last_message_at,
		       created_at, updated_at
		FROM ai_chat_sessions
		WHERE id = $1 AND tenant_id = $2 AND user_id = $3
	`, sessionID, tenantID, userID)

	item, err := scanAIChatSession(row)
	if err != nil {
		return nil, fmt.Errorf("get ai chat session: %w", err)
	}
	return item, nil
}

func (r *AIChatRepository) GetFirstSession(ctx context.Context, tenantID, userID uuid.UUID) (*models.AIChatSession, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, user_id, title, is_default, sort_order,
		       '' AS last_message_preview,
		       '' AS last_message_role,
		       updated_at AS last_message_at,
		       created_at, updated_at
		FROM ai_chat_sessions
		WHERE tenant_id = $1 AND user_id = $2
		ORDER BY sort_order DESC, updated_at DESC, created_at DESC
		LIMIT 1
	`, tenantID, userID)

	item, err := scanAIChatSession(row)
	if err != nil {
		return nil, fmt.Errorf("get first ai chat session: %w", err)
	}
	return item, nil
}

func (r *AIChatRepository) DeleteSession(ctx context.Context, tenantID, userID, sessionID uuid.UUID) (bool, error) {
	result, err := r.pool.Exec(ctx, `
		DELETE FROM ai_chat_sessions
		WHERE id = $1 AND tenant_id = $2 AND user_id = $3
	`, sessionID, tenantID, userID)
	if err != nil {
		return false, fmt.Errorf("delete ai chat session: %w", err)
	}
	return result.RowsAffected() > 0, nil
}

func (r *AIChatRepository) ReorderSessions(ctx context.Context, tenantID, userID uuid.UUID, orderedIDs []uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin ai chat reorder tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `
		SELECT id
		FROM ai_chat_sessions
		WHERE tenant_id = $1 AND user_id = $2
		ORDER BY sort_order DESC, updated_at DESC, created_at DESC
	`, tenantID, userID)
	if err != nil {
		return fmt.Errorf("list sessions for ai chat reorder: %w", err)
	}
	defer rows.Close()

	existing := make([]uuid.UUID, 0)
	existingSet := make(map[uuid.UUID]struct{})
	for rows.Next() {
		var id uuid.UUID
		if scanErr := rows.Scan(&id); scanErr != nil {
			return fmt.Errorf("scan session id for ai chat reorder: %w", scanErr)
		}
		existing = append(existing, id)
		existingSet[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate sessions for ai chat reorder: %w", err)
	}
	if len(existing) == 0 {
		return nil
	}

	resultOrder := make([]uuid.UUID, 0, len(existing))
	used := make(map[uuid.UUID]struct{}, len(existing))

	for _, id := range orderedIDs {
		if _, ok := existingSet[id]; !ok {
			return fmt.Errorf("session %s not found", id.String())
		}
		if _, alreadyUsed := used[id]; alreadyUsed {
			continue
		}
		resultOrder = append(resultOrder, id)
		used[id] = struct{}{}
	}
	for _, id := range existing {
		if _, alreadyUsed := used[id]; alreadyUsed {
			continue
		}
		resultOrder = append(resultOrder, id)
	}

	orderValue := int64(len(resultOrder))
	for _, id := range resultOrder {
		if _, execErr := tx.Exec(ctx, `
			UPDATE ai_chat_sessions
			SET sort_order = $4
			WHERE id = $1 AND tenant_id = $2 AND user_id = $3
		`, id, tenantID, userID, orderValue); execErr != nil {
			return fmt.Errorf("update session order for ai chat reorder: %w", execErr)
		}
		orderValue--
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit ai chat reorder tx: %w", err)
	}
	return nil
}

func (r *AIChatRepository) ListMessages(ctx context.Context, tenantID, userID, sessionID uuid.UUID, limit int) ([]models.AIChatMessage, error) {
	if limit <= 0 || limit > 500 {
		limit = 80
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			id, session_id, tenant_id, user_id, role, content, sources, metadata, created_at
		FROM ai_chat_messages
		WHERE tenant_id = $1 AND user_id = $2 AND session_id = $3
		ORDER BY created_at DESC
		LIMIT $4
	`, tenantID, userID, sessionID, limit)
	if err != nil {
		return nil, fmt.Errorf("list ai chat messages: %w", err)
	}
	defer rows.Close()

	out := make([]models.AIChatMessage, 0, limit)
	for rows.Next() {
		item, err := scanAIChatMessage(rows)
		if err != nil {
			return nil, fmt.Errorf("scan ai chat message: %w", err)
		}
		out = append(out, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ai chat messages: %w", err)
	}
	return out, nil
}

func (r *AIChatRepository) CreateMessage(ctx context.Context, p CreateAIChatMessageParams) (*models.AIChatMessage, error) {
	role := strings.ToLower(strings.TrimSpace(p.Role))
	if role == "" {
		role = "assistant"
	}
	if role != "assistant" && role != "user" && role != "system" {
		role = "assistant"
	}
	content := strings.TrimSpace(p.Content)
	if content == "" {
		return nil, fmt.Errorf("message content is required")
	}

	if p.Metadata == nil {
		p.Metadata = map[string]any{}
	}
	if p.Sources == nil {
		p.Sources = []map[string]any{}
	}

	sourcesRaw, err := json.Marshal(p.Sources)
	if err != nil {
		return nil, fmt.Errorf("marshal ai chat message sources: %w", err)
	}
	metadataRaw, err := json.Marshal(p.Metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal ai chat message metadata: %w", err)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin ai chat message tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row := tx.QueryRow(ctx, `
		INSERT INTO ai_chat_messages(session_id, tenant_id, user_id, role, content, sources, metadata)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7::jsonb)
		RETURNING id, session_id, tenant_id, user_id, role, content, sources, metadata, created_at
	`, p.SessionID, p.TenantID, p.UserID, role, content, sourcesRaw, metadataRaw)

	item, err := scanAIChatMessage(row)
	if err != nil {
		return nil, fmt.Errorf("insert ai chat message: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE ai_chat_sessions
		SET updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2 AND user_id = $3
	`, p.SessionID, p.TenantID, p.UserID); err != nil {
		return nil, fmt.Errorf("touch ai chat session: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit ai chat message tx: %w", err)
	}
	return item, nil
}

func scanAIChatSession(scanner rowScanner) (*models.AIChatSession, error) {
	var item models.AIChatSession
	if err := scanner.Scan(
		&item.ID,
		&item.TenantID,
		&item.UserID,
		&item.Title,
		&item.IsDefault,
		&item.SortOrder,
		&item.LastMessagePreview,
		&item.LastMessageRole,
		&item.LastMessageAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &item, nil
}

type aiChatMessageScanner interface {
	Scan(dest ...any) error
}

func scanAIChatMessage(scanner aiChatMessageScanner) (*models.AIChatMessage, error) {
	var (
		item        models.AIChatMessage
		rawSources  []byte
		rawMetadata []byte
	)
	if err := scanner.Scan(
		&item.ID,
		&item.SessionID,
		&item.TenantID,
		&item.UserID,
		&item.Role,
		&item.Content,
		&rawSources,
		&rawMetadata,
		&item.CreatedAt,
	); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(rawSources, &item.Sources); err != nil {
		return nil, err
	}
	if item.Sources == nil {
		item.Sources = []map[string]any{}
	}
	if err := json.Unmarshal(rawMetadata, &item.Metadata); err != nil {
		return nil, err
	}
	if item.Metadata == nil {
		item.Metadata = map[string]any{}
	}
	return &item, nil
}

func normalizeAIChatSessionTitle(value string) string {
	title := strings.TrimSpace(value)
	if title == "" {
		return defaultAIChatSessionTitle
	}
	runes := []rune(title)
	if len(runes) > maxAIChatSessionTitleLen {
		return strings.TrimSpace(string(runes[:maxAIChatSessionTitleLen]))
	}
	return title
}
