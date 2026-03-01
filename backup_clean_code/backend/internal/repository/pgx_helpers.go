package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type rowScanner interface {
	Scan(dest ...any) error
}

func normalizeLimit(limit int, maxLimit int, defaultValue int) int {
	if limit <= 0 || limit > maxLimit {
		return defaultValue
	}
	return limit
}

func normalizeOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

func collectRows[T any](
	rows pgx.Rows,
	capacity int,
	scanFn func(rowScanner) (*T, error),
	scanErrContext string,
	iterateErrContext string,
) ([]T, error) {
	defer rows.Close()

	out := make([]T, 0, capacity)
	for rows.Next() {
		item, scanErr := scanFn(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("%s: %w", scanErrContext, scanErr)
		}
		out = append(out, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", iterateErrContext, err)
	}
	return out, nil
}

func listByTenantAndStatus[T any](
	ctx context.Context,
	pool *pgxpool.Pool,
	query string,
	tenantID uuid.UUID,
	status string,
	limit int,
	offset int,
	scanFn func(rowScanner) (*T, error),
	listErrContext string,
	scanErrContext string,
	iterateErrContext string,
) ([]T, error) {
	limit = normalizeLimit(limit, 500, 100)
	offset = normalizeOffset(offset)
	rows, err := pool.Query(ctx, query, tenantID, status, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", listErrContext, err)
	}
	return collectRows(rows, limit, scanFn, scanErrContext, iterateErrContext)
}

func listByTenantWithOptionalClause[T any](
	ctx context.Context,
	pool *pgxpool.Pool,
	baseQuery string,
	tenantID uuid.UUID,
	includeAll bool,
	excludeClause string,
	orderLimitClause string,
	limit int,
	maxLimit int,
	defaultLimit int,
	scanFn func(rowScanner) (*T, error),
	listErrContext string,
	scanErrContext string,
	iterateErrContext string,
) ([]T, error) {
	limit = normalizeLimit(limit, maxLimit, defaultLimit)

	query := baseQuery
	if !includeAll && strings.TrimSpace(excludeClause) != "" {
		query += " " + excludeClause
	}
	query += " " + orderLimitClause

	rows, err := pool.Query(ctx, query, tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", listErrContext, err)
	}
	return collectRows(rows, limit, scanFn, scanErrContext, iterateErrContext)
}

func listByCase[T any](
	ctx context.Context,
	pool *pgxpool.Pool,
	query string,
	tenantID uuid.UUID,
	caseID uuid.UUID,
	limit int,
	offset int,
	scanFn func(rowScanner) (*T, error),
	listErrContext string,
	scanErrContext string,
	iterateErrContext string,
) ([]T, error) {
	limit = normalizeLimit(limit, 200, 50)
	offset = normalizeOffset(offset)
	rows, err := pool.Query(ctx, query, tenantID, caseID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", listErrContext, err)
	}
	return collectRows(rows, limit, scanFn, scanErrContext, iterateErrContext)
}

func listByExecutionID[T any](
	ctx context.Context,
	pool *pgxpool.Pool,
	query string,
	executionID uuid.UUID,
	limit int,
	maxLimit int,
	defaultLimit int,
	scanFn func(rowScanner) (*T, error),
	listErrContext string,
	scanErrContext string,
	iterateErrContext string,
) ([]T, error) {
	limit = normalizeLimit(limit, maxLimit, defaultLimit)
	rows, err := pool.Query(ctx, query, executionID, limit)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", listErrContext, err)
	}
	return collectRows(rows, limit, scanFn, scanErrContext, iterateErrContext)
}
