package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ForumThreadStats struct {
	ThreadID           uuid.UUID
	PostsCount         int
	LastPostID         uuid.UUID
	LastPostAt         time.Time
	LastPostAuthorID   string
	LastPostAuthorName string
	LastPostPreview    string
}

func (r *CatalogRepository) ListForumThreadStats(ctx context.Context, tenantID uuid.UUID, threadIDs []uuid.UUID) (map[uuid.UUID]ForumThreadStats, error) {
	result := make(map[uuid.UUID]ForumThreadStats, len(threadIDs))
	if len(threadIDs) == 0 {
		return result, nil
	}

	rows, err := r.pool.Query(ctx, `
		WITH posts AS (
			SELECT
				ref_id AS thread_id,
				id AS post_id,
				COALESCE(NULLIF(data->>'author_id', ''), '') AS author_id,
				COALESCE(NULLIF(data->>'author_name', ''), NULLIF(data->>'authorName', ''), '') AS author_name,
				COALESCE(NULLIF(data->>'content', ''), '') AS content,
				CASE
					WHEN COALESCE(data->>'timestamp', '') ~ '^[0-9]{4}-[0-9]{2}-[0-9]{2}T'
						THEN (data->>'timestamp')::timestamptz
					WHEN COALESCE(data->>'created_at', '') ~ '^[0-9]{4}-[0-9]{2}-[0-9]{2}T'
						THEN (data->>'created_at')::timestamptz
					ELSE created_at
				END AS post_time
			FROM catalog_items
			WHERE kind = 'forum_post'
				AND tenant_id = $1
				AND ref_id = ANY($2::uuid[])
		),
		ranked AS (
			SELECT
				thread_id,
				post_id,
				author_id,
				author_name,
				content,
				post_time,
				COUNT(*) OVER (PARTITION BY thread_id) AS posts_count,
				ROW_NUMBER() OVER (PARTITION BY thread_id ORDER BY post_time DESC, post_id DESC) AS row_num
			FROM posts
		)
		SELECT
			thread_id,
			posts_count,
			post_id,
			post_time,
			author_id,
			author_name,
			LEFT(content, 280)
		FROM ranked
		WHERE row_num = 1
	`, tenantID, threadIDs)
	if err != nil {
		return nil, fmt.Errorf("list forum thread stats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			stats      ForumThreadStats
			postsCount int64
		)
		if err := rows.Scan(
			&stats.ThreadID,
			&postsCount,
			&stats.LastPostID,
			&stats.LastPostAt,
			&stats.LastPostAuthorID,
			&stats.LastPostAuthorName,
			&stats.LastPostPreview,
		); err != nil {
			return nil, fmt.Errorf("scan forum thread stats: %w", err)
		}
		stats.PostsCount = int(postsCount)
		stats.LastPostAuthorID = strings.TrimSpace(stats.LastPostAuthorID)
		stats.LastPostAuthorName = strings.TrimSpace(stats.LastPostAuthorName)
		stats.LastPostPreview = strings.TrimSpace(stats.LastPostPreview)
		result[stats.ThreadID] = stats
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate forum thread stats: %w", err)
	}

	return result, nil
}
