package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ExperienceRepository struct {
	pool *pgxpool.Pool
}

type AwardExperienceParams struct {
	TenantID    *uuid.UUID
	UserID      uuid.UUID
	EventKey    string
	EventType   string
	Points      int
	Description string
	Details     map[string]any
}

type AwardExperienceResult struct {
	Awarded     bool
	TotalPoints int64
}

type UserExperienceEvent struct {
	ID          uuid.UUID
	TenantID    *uuid.UUID
	UserID      uuid.UUID
	EventKey    string
	EventType   string
	Points      int
	Description string
	Details     map[string]any
	CreatedAt   time.Time
}

type UserCasePerformance struct {
	ClosedCasesTotal                     int      `json:"closed_cases_total"`
	AvgInvestigationMinutes              float64  `json:"avg_investigation_minutes"`
	CurrentMonthClosedCases              int      `json:"current_month_closed_cases"`
	PreviousMonthClosedCases             int      `json:"previous_month_closed_cases"`
	CurrentMonthAvgInvestigationMinutes  *float64 `json:"current_month_avg_investigation_minutes"`
	PreviousMonthAvgInvestigationMinutes *float64 `json:"previous_month_avg_investigation_minutes"`
}

func NewExperienceRepository(pool *pgxpool.Pool) *ExperienceRepository {
	return &ExperienceRepository{pool: pool}
}

func (r *ExperienceRepository) RulePoints(ctx context.Context, key string) (int, error) {
	var points int
	if err := r.pool.QueryRow(ctx, `SELECT points FROM xp_reward_rules WHERE key = $1`, key).Scan(&points); err != nil {
		return 0, fmt.Errorf("load xp reward rule %q: %w", key, err)
	}
	return points, nil
}

func (r *ExperienceRepository) Award(ctx context.Context, p AwardExperienceParams) (AwardExperienceResult, error) {
	if stringsTrim(p.EventKey) == "" {
		return AwardExperienceResult{}, fmt.Errorf("award experience: event key is required")
	}
	if stringsTrim(p.EventType) == "" {
		return AwardExperienceResult{}, fmt.Errorf("award experience: event type is required")
	}
	if p.Points <= 0 {
		total, err := r.currentTotal(ctx, p.UserID)
		if err != nil {
			return AwardExperienceResult{}, err
		}
		return AwardExperienceResult{Awarded: false, TotalPoints: total}, nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return AwardExperienceResult{}, fmt.Errorf("award experience begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var eventID uuid.UUID
	insertErr := tx.QueryRow(ctx, `
		INSERT INTO user_xp_events(tenant_id, user_id, event_key, event_type, points, description, details)
		VALUES ($1, $2, $3, $4, $5, $6, COALESCE($7, '{}'::jsonb))
		ON CONFLICT (event_key) DO NOTHING
		RETURNING id
	`, p.TenantID, p.UserID, p.EventKey, p.EventType, p.Points, stringsTrim(p.Description), p.Details).Scan(&eventID)
	if insertErr != nil && !errors.Is(insertErr, pgx.ErrNoRows) {
		return AwardExperienceResult{}, fmt.Errorf("insert user xp event: %w", insertErr)
	}

	var total int64
	if errors.Is(insertErr, pgx.ErrNoRows) {
		if err := tx.QueryRow(ctx, `SELECT experience_points FROM users WHERE id = $1`, p.UserID).Scan(&total); err != nil {
			return AwardExperienceResult{}, fmt.Errorf("load user experience total: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return AwardExperienceResult{}, fmt.Errorf("award experience commit existing total: %w", err)
		}
		return AwardExperienceResult{
			Awarded:     false,
			TotalPoints: total,
		}, nil
	}

	if err := tx.QueryRow(ctx, `
		UPDATE users
		SET experience_points = experience_points + $2, updated_at = NOW()
		WHERE id = $1
		RETURNING experience_points
	`, p.UserID, p.Points).Scan(&total); err != nil {
		return AwardExperienceResult{}, fmt.Errorf("update user experience total: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return AwardExperienceResult{}, fmt.Errorf("award experience commit: %w", err)
	}

	return AwardExperienceResult{
		Awarded:     true,
		TotalPoints: total,
	}, nil
}

func (r *ExperienceRepository) currentTotal(ctx context.Context, userID uuid.UUID) (int64, error) {
	var total int64
	if err := r.pool.QueryRow(ctx, `SELECT experience_points FROM users WHERE id = $1`, userID).Scan(&total); err != nil {
		return 0, fmt.Errorf("load current user experience total: %w", err)
	}
	return total, nil
}

func (r *ExperienceRepository) ListByUser(ctx context.Context, userID uuid.UUID, tenantID *uuid.UUID, limit, offset int) ([]UserExperienceEvent, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, user_id, event_key, event_type, points, description, details, created_at
		FROM user_xp_events
		WHERE user_id = $1
		  AND ($2::uuid IS NULL OR tenant_id = $2)
		ORDER BY created_at DESC
		LIMIT $3
		OFFSET $4
	`, userID, tenantID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list user experience events: %w", err)
	}
	defer rows.Close()

	events := make([]UserExperienceEvent, 0, limit)
	for rows.Next() {
		var item UserExperienceEvent
		var rawDetails []byte
		if err := rows.Scan(
			&item.ID,
			&item.TenantID,
			&item.UserID,
			&item.EventKey,
			&item.EventType,
			&item.Points,
			&item.Description,
			&rawDetails,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan user experience event: %w", err)
		}
		item.Description = stringsTrim(item.Description)
		if len(rawDetails) > 0 && string(rawDetails) != "null" {
			if err := json.Unmarshal(rawDetails, &item.Details); err != nil {
				return nil, fmt.Errorf("decode user experience event details: %w", err)
			}
		}
		if item.Details == nil {
			item.Details = map[string]any{}
		}
		events = append(events, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user experience events: %w", err)
	}
	return events, nil
}

func (r *ExperienceRepository) UserCasePerformance(ctx context.Context, userID uuid.UUID, tenantID *uuid.UUID) (UserCasePerformance, error) {
	var out UserCasePerformance
	var overallAvg sql.NullFloat64
	var currentMonthAvg sql.NullFloat64
	var previousMonthAvg sql.NullFloat64

	err := r.pool.QueryRow(ctx, `
		WITH periods AS (
			SELECT
				date_trunc('month', NOW()) AS current_start,
				date_trunc('month', NOW()) + INTERVAL '1 month' AS next_start,
				date_trunc('month', NOW()) - INTERVAL '1 month' AS previous_start
		),
		events AS (
			SELECT
				e.created_at,
				CASE
					WHEN (e.details->>'case_id') ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
						THEN (e.details->>'case_id')::uuid
					ELSE NULL
				END AS case_id
			FROM user_xp_events e
			WHERE e.user_id = $1
				AND e.event_type = 'case_closed'
				AND ($2::uuid IS NULL OR e.tenant_id = $2)
		),
		event_cases AS (
			SELECT
				e.created_at AS event_created_at,
				EXTRACT(EPOCH FROM (COALESCE(c.closed_at, c.updated_at) - c.created_at)) / 60.0 AS duration_min
			FROM events e
			LEFT JOIN cases c ON c.id = e.case_id
			WHERE e.case_id IS NOT NULL
				AND ($2::uuid IS NULL OR c.tenant_id = $2)
		)
		SELECT
			(SELECT COUNT(*) FROM events) AS closed_cases_total,
			(SELECT AVG(duration_min) FROM event_cases WHERE duration_min IS NOT NULL AND duration_min >= 0) AS avg_investigation_minutes,
			(SELECT COUNT(*) FROM events, periods p WHERE events.created_at >= p.current_start AND events.created_at < p.next_start) AS current_month_closed_cases,
			(SELECT COUNT(*) FROM events, periods p WHERE events.created_at >= p.previous_start AND events.created_at < p.current_start) AS previous_month_closed_cases,
			(
				SELECT AVG(duration_min)
				FROM event_cases, periods p
				WHERE event_cases.event_created_at >= p.current_start
					AND event_cases.event_created_at < p.next_start
					AND duration_min IS NOT NULL
					AND duration_min >= 0
			) AS current_month_avg_investigation_minutes,
			(
				SELECT AVG(duration_min)
				FROM event_cases, periods p
				WHERE event_cases.event_created_at >= p.previous_start
					AND event_cases.event_created_at < p.current_start
					AND duration_min IS NOT NULL
					AND duration_min >= 0
			) AS previous_month_avg_investigation_minutes
	`, userID, tenantID).Scan(
		&out.ClosedCasesTotal,
		&overallAvg,
		&out.CurrentMonthClosedCases,
		&out.PreviousMonthClosedCases,
		&currentMonthAvg,
		&previousMonthAvg,
	)
	if err != nil {
		return UserCasePerformance{}, fmt.Errorf("query user case performance: %w", err)
	}

	if overallAvg.Valid {
		out.AvgInvestigationMinutes = roundFloat(overallAvg.Float64, 2)
	}
	if currentMonthAvg.Valid {
		value := roundFloat(currentMonthAvg.Float64, 2)
		out.CurrentMonthAvgInvestigationMinutes = &value
	}
	if previousMonthAvg.Valid {
		value := roundFloat(previousMonthAvg.Float64, 2)
		out.PreviousMonthAvgInvestigationMinutes = &value
	}

	return out, nil
}

func roundFloat(value float64, precision int) float64 {
	if precision < 0 {
		precision = 0
	}
	multiplier := math.Pow10(precision)
	return math.Round(value*multiplier) / multiplier
}

func stringsTrim(input string) string {
	start := 0
	for start < len(input) && (input[start] == ' ' || input[start] == '\n' || input[start] == '\t' || input[start] == '\r') {
		start++
	}
	end := len(input)
	for end > start && (input[end-1] == ' ' || input[end-1] == '\n' || input[end-1] == '\t' || input[end-1] == '\r') {
		end--
	}
	return input[start:end]
}
