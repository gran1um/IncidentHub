package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

func (r *CaseRepository) HasAnyCaseMetaTag(
	ctx context.Context,
	tenantID uuid.UUID,
	caseID uuid.UUID,
	allowedTags []string,
) (bool, error) {
	normalized := normalizeListSearchTags(allowedTags)
	if len(normalized) == 0 {
		return true, nil
	}
	var allowed bool
	if err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM catalog_items cm
			WHERE cm.kind = 'case_meta'
			  AND cm.tenant_id = $1
			  AND cm.ref_id = $2
			  AND EXISTS (
				SELECT 1
				FROM jsonb_array_elements_text(COALESCE(cm.data->'tags', '[]'::jsonb)) AS t(value)
				WHERE LOWER(t.value) = ANY($3::text[])
			  )
		)
	`, tenantID, caseID, normalized).Scan(&allowed); err != nil {
		return false, fmt.Errorf("check case meta tags access: %w", err)
	}
	return allowed, nil
}
