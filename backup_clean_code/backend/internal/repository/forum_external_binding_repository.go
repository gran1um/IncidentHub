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

type ForumExternalBindingUpsertParams struct {
	BindingKey     string
	ConversationID string
	Cursor         string
	Metadata       map[string]any
	LastSyncedAt   bool
}

type ForumExternalBindingRepository struct {
	pool *pgxpool.Pool
}

func NewForumExternalBindingRepository(pool *pgxpool.Pool) *ForumExternalBindingRepository {
	return &ForumExternalBindingRepository{pool: pool}
}

func (r *ForumExternalBindingRepository) GetByThreadAndConnector(ctx context.Context, tenantID, threadID, connectorID uuid.UUID) (*models.ForumExternalBinding, error) {
	return r.GetByThreadConnectorAndKey(ctx, tenantID, threadID, connectorID, "")
}

func (r *ForumExternalBindingRepository) GetByThreadConnectorAndKey(ctx context.Context, tenantID, threadID, connectorID uuid.UUID, bindingKey string) (*models.ForumExternalBinding, error) {
	normalizedBindingKey := normalizeForumBindingKey(bindingKey)
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, thread_id, connector_id, binding_key, conversation_id, cursor, metadata, last_synced_at, created_at, updated_at
		FROM forum_external_bindings
		WHERE tenant_id = $1 AND thread_id = $2 AND connector_id = $3 AND binding_key = $4
	`, tenantID, threadID, connectorID, normalizedBindingKey)
	item, err := scanForumExternalBinding(row)
	if err != nil {
		return nil, fmt.Errorf("get forum external binding: %w", err)
	}
	return item, nil
}

func (r *ForumExternalBindingRepository) ListByThread(ctx context.Context, tenantID, threadID uuid.UUID) ([]models.ForumExternalBinding, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, thread_id, connector_id, binding_key, conversation_id, cursor, metadata, last_synced_at, created_at, updated_at
		FROM forum_external_bindings
		WHERE tenant_id = $1 AND thread_id = $2
		ORDER BY updated_at DESC
	`, tenantID, threadID)
	if err != nil {
		return nil, fmt.Errorf("list forum external bindings: %w", err)
	}
	defer rows.Close()

	out := make([]models.ForumExternalBinding, 0)
	for rows.Next() {
		item, scanErr := scanForumExternalBinding(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate forum external bindings: %w", err)
	}
	return out, nil
}

func (r *ForumExternalBindingRepository) Upsert(ctx context.Context, tenantID, threadID, connectorID uuid.UUID, p ForumExternalBindingUpsertParams) (*models.ForumExternalBinding, error) {
	normalizedBindingKey := normalizeForumBindingKey(p.BindingKey)
	if p.Metadata == nil {
		p.Metadata = map[string]any{}
	}
	rawMetadata, err := json.Marshal(p.Metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal forum external binding metadata: %w", err)
	}

	q := `
		INSERT INTO forum_external_bindings(tenant_id, thread_id, connector_id, binding_key, conversation_id, cursor, metadata, last_synced_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, CASE WHEN $8 THEN NOW() ELSE NULL END)
		ON CONFLICT (tenant_id, thread_id, connector_id, binding_key)
		DO UPDATE SET
			conversation_id = CASE WHEN EXCLUDED.conversation_id <> '' THEN EXCLUDED.conversation_id ELSE forum_external_bindings.conversation_id END,
			cursor = CASE WHEN EXCLUDED.cursor <> '' THEN EXCLUDED.cursor ELSE forum_external_bindings.cursor END,
			metadata = forum_external_bindings.metadata || EXCLUDED.metadata,
			last_synced_at = CASE WHEN $8 THEN NOW() ELSE forum_external_bindings.last_synced_at END,
			updated_at = NOW()
		RETURNING id, tenant_id, thread_id, connector_id, binding_key, conversation_id, cursor, metadata, last_synced_at, created_at, updated_at
	`
	row := r.pool.QueryRow(
		ctx,
		q,
		tenantID,
		threadID,
		connectorID,
		normalizedBindingKey,
		p.ConversationID,
		p.Cursor,
		rawMetadata,
		p.LastSyncedAt,
	)
	item, err := scanForumExternalBinding(row)
	if err != nil {
		return nil, fmt.Errorf("upsert forum external binding: %w", err)
	}
	return item, nil
}

type forumBindingScanner interface {
	Scan(dest ...any) error
}

func scanForumExternalBinding(scanner forumBindingScanner) (*models.ForumExternalBinding, error) {
	var (
		item        models.ForumExternalBinding
		rawMetadata []byte
	)
	err := scanner.Scan(
		&item.ID,
		&item.TenantID,
		&item.ThreadID,
		&item.ConnectorID,
		&item.BindingKey,
		&item.ConversationID,
		&item.Cursor,
		&rawMetadata,
		&item.LastSyncedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	item.Metadata = map[string]any{}
	if len(rawMetadata) > 0 {
		if err := json.Unmarshal(rawMetadata, &item.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal forum external binding metadata: %w", err)
		}
	}
	return &item, nil
}

func IsNoRows(err error) bool {
	return err != nil && errors.Is(err, pgx.ErrNoRows)
}

func normalizeForumBindingKey(raw string) string {
	return strings.TrimSpace(raw)
}
