package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"incidenthub/backend/internal/models"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CaseEventRepository struct {
	pool *pgxpool.Pool
}

type CreateCaseEventParams struct {
	TenantID  uuid.UUID
	CaseID    uuid.UUID
	EventType string
	Title     string
	Body      string
	ActorID   *uuid.UUID
	Metadata  map[string]any
}

func NewCaseEventRepository(pool *pgxpool.Pool) *CaseEventRepository {
	return &CaseEventRepository{pool: pool}
}

func (r *CaseEventRepository) Create(ctx context.Context, p CreateCaseEventParams) (*models.CaseTimelineEvent, error) {
	metadata := p.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	payload, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal metadata: %w", err)
	}

	q := `
		INSERT INTO case_timeline_events(tenant_id, case_id, event_type, title, body, actor_id, metadata)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, tenant_id, case_id, event_type, title, body, actor_id, metadata, created_at
	`

	item, err := scanCaseEvent(r.pool.QueryRow(ctx, q,
		p.TenantID,
		p.CaseID,
		p.EventType,
		p.Title,
		p.Body,
		p.ActorID,
		payload,
	))
	if err != nil {
		return nil, fmt.Errorf("create case event: %w", err)
	}

	return item, nil
}

func (r *CaseEventRepository) ListByCase(ctx context.Context, tenantID, caseID uuid.UUID, limit, offset int) ([]models.CaseTimelineEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	q := `
		SELECT id, tenant_id, case_id, event_type, title, body, actor_id, metadata, created_at
		FROM case_timeline_events
		WHERE tenant_id=$1 AND case_id=$2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`

	rows, err := r.pool.Query(ctx, q, tenantID, caseID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list case events: %w", err)
	}
	defer rows.Close()

	items := make([]models.CaseTimelineEvent, 0, limit)
	for rows.Next() {
		item, scanErr := scanCaseEvent(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan case event: %w", scanErr)
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate case events: %w", err)
	}
	return items, nil
}

func (r *CaseEventRepository) CountByTenantAndTypesSince(ctx context.Context, tenantID uuid.UUID, eventTypes []string, since time.Time) (int, error) {
	normalizedEventTypes := make([]string, 0, len(eventTypes))
	seen := make(map[string]struct{}, len(eventTypes))
	for _, eventType := range eventTypes {
		normalized := strings.ToLower(strings.TrimSpace(eventType))
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		normalizedEventTypes = append(normalizedEventTypes, normalized)
	}
	if len(normalizedEventTypes) == 0 {
		return 0, nil
	}

	var count int
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM case_timeline_events
		WHERE tenant_id = $1
		  AND created_at >= $2
		  AND LOWER(event_type) = ANY($3::text[])
	`, tenantID, since, normalizedEventTypes).Scan(&count); err != nil {
		return 0, fmt.Errorf("count case timeline events by tenant and type: %w", err)
	}
	return count, nil
}

type caseEventScanner interface {
	Scan(dest ...any) error
}

func scanCaseEvent(scanner caseEventScanner) (*models.CaseTimelineEvent, error) {
	var item models.CaseTimelineEvent
	var metadataRaw []byte
	if err := scanner.Scan(
		&item.ID,
		&item.TenantID,
		&item.CaseID,
		&item.EventType,
		&item.Title,
		&item.Body,
		&item.ActorID,
		&metadataRaw,
		&item.CreatedAt,
	); err != nil {
		return nil, err
	}
	if len(metadataRaw) == 0 {
		item.Metadata = map[string]any{}
		return &item, nil
	}
	if err := json.Unmarshal(metadataRaw, &item.Metadata); err != nil {
		return nil, fmt.Errorf("unmarshal metadata: %w", err)
	}
	if item.Metadata == nil {
		item.Metadata = map[string]any{}
	}
	return &item, nil
}
