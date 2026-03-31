package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

func (r *CatalogRepository) ForumPostExistsByExternalID(ctx context.Context, tenantID, threadID uuid.UUID, externalID string) (bool, error) {
	normalizedExternalID := strings.TrimSpace(externalID)
	if normalizedExternalID == "" {
		return false, nil
	}

	var exists bool
	if err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM catalog_items
			WHERE kind = 'forum_post'
			  AND tenant_id = $1
			  AND ref_id = $2
			  AND COALESCE(NULLIF(data->>'external_id', ''), NULL) = $3
		)
	`, tenantID, threadID, normalizedExternalID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check forum post external_id existence: %w", err)
	}

	return exists, nil
}
