package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ConnectorRecipientAliasUpsertParams struct {
	Channel     string
	RecipientID string
	DisplayName string
	Metadata    map[string]any
	Aliases     map[string]string
}

type ConnectorRecipientAliasRepository struct {
	pool *pgxpool.Pool
}

func NewConnectorRecipientAliasRepository(pool *pgxpool.Pool) *ConnectorRecipientAliasRepository {
	return &ConnectorRecipientAliasRepository{pool: pool}
}

func (r *ConnectorRecipientAliasRepository) UpsertAliases(ctx context.Context, tenantID, connectorID uuid.UUID, p ConnectorRecipientAliasUpsertParams) error {
	channel := normalizeAliasToken(p.Channel)
	recipientID := strings.TrimSpace(p.RecipientID)
	if channel == "" || recipientID == "" {
		return fmt.Errorf("channel and recipient id are required")
	}

	aliasPairs := map[string]string{
		"recipient_id": recipientID,
	}
	for aliasType, aliasValue := range p.Aliases {
		normalizedType := normalizeAliasToken(aliasType)
		normalizedValue := normalizeAliasValue(aliasValue)
		if normalizedType == "" || normalizedValue == "" {
			continue
		}
		aliasPairs[normalizedType] = normalizedValue
	}

	if len(aliasPairs) == 0 {
		return nil
	}

	metadata := p.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	rawMetadata, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal connector recipient metadata: %w", err)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin connector recipient alias upsert: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	displayName := strings.TrimSpace(p.DisplayName)
	for aliasType, aliasValue := range aliasPairs {
		_, err := tx.Exec(ctx, `
			INSERT INTO connector_recipient_aliases(
				tenant_id, connector_id, channel, recipient_id, alias_type, alias_value, display_name, metadata, last_seen_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, NOW())
			ON CONFLICT (tenant_id, connector_id, channel, alias_type, alias_value)
			DO UPDATE SET
				recipient_id = EXCLUDED.recipient_id,
				display_name = CASE
					WHEN EXCLUDED.display_name <> '' THEN EXCLUDED.display_name
					ELSE connector_recipient_aliases.display_name
				END,
				metadata = connector_recipient_aliases.metadata || EXCLUDED.metadata,
				last_seen_at = NOW(),
				updated_at = NOW()
		`, tenantID, connectorID, channel, recipientID, aliasType, aliasValue, displayName, rawMetadata)
		if err != nil {
			return fmt.Errorf("upsert connector recipient alias (%s=%s): %w", aliasType, aliasValue, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit connector recipient alias upsert: %w", err)
	}
	return nil
}

func (r *ConnectorRecipientAliasRepository) ResolveRecipientID(ctx context.Context, tenantID, connectorID uuid.UUID, channel string, aliases []string) (string, error) {
	normalizedChannel := normalizeAliasToken(channel)
	if normalizedChannel == "" {
		return "", fmt.Errorf("channel is required")
	}

	candidates := make([]string, 0, len(aliases))
	seen := map[string]struct{}{}
	for _, alias := range aliases {
		normalized := normalizeAliasValue(alias)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		candidates = append(candidates, normalized)
	}
	if len(candidates) == 0 {
		return "", pgx.ErrNoRows
	}

	var recipientID string
	err := r.pool.QueryRow(ctx, `
		SELECT recipient_id
		FROM connector_recipient_aliases
		WHERE tenant_id = $1
		  AND connector_id = $2
		  AND channel = $3
		  AND alias_value = ANY($4::text[])
		ORDER BY last_seen_at DESC, updated_at DESC
		LIMIT 1
	`, tenantID, connectorID, normalizedChannel, candidates).Scan(&recipientID)
	if err != nil {
		return "", fmt.Errorf("resolve connector recipient alias: %w", err)
	}
	return strings.TrimSpace(recipientID), nil
}

func normalizeAliasToken(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func normalizeAliasValue(raw string) string {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	normalized = strings.TrimPrefix(normalized, "@")
	return normalized
}
