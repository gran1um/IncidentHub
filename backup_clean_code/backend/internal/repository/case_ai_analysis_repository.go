package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"incidenthub/backend/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CaseAIAnalysisRepository struct {
	pool *pgxpool.Pool
}

type CreateCaseAIAnalysisParams struct {
	TenantID        uuid.UUID
	CaseID          uuid.UUID
	RequestedBy     *uuid.UUID
	Model           string
	Status          string
	Verdict         string
	Confidence      float64
	Summary         string
	Recommendations []string
	Findings        []string
	Sources         []map[string]any
	ErrorMessage    string
}

func NewCaseAIAnalysisRepository(pool *pgxpool.Pool) *CaseAIAnalysisRepository {
	return &CaseAIAnalysisRepository{pool: pool}
}

func (r *CaseAIAnalysisRepository) Create(ctx context.Context, p CreateCaseAIAnalysisParams) (*models.CaseAIAnalysis, error) {
	if p.Status == "" {
		p.Status = "completed"
	}
	if p.Verdict == "" {
		p.Verdict = "unknown"
	}
	if p.Recommendations == nil {
		p.Recommendations = []string{}
	}
	if p.Findings == nil {
		p.Findings = []string{}
	}
	if p.Sources == nil {
		p.Sources = []map[string]any{}
	}

	recommendationsRaw, err := json.Marshal(p.Recommendations)
	if err != nil {
		return nil, fmt.Errorf("marshal recommendations: %w", err)
	}
	findingsRaw, err := json.Marshal(p.Findings)
	if err != nil {
		return nil, fmt.Errorf("marshal findings: %w", err)
	}
	sourcesRaw, err := json.Marshal(p.Sources)
	if err != nil {
		return nil, fmt.Errorf("marshal sources: %w", err)
	}

	row := r.pool.QueryRow(ctx, `
		INSERT INTO case_ai_analyses(
			tenant_id, case_id, requested_by, model, status, verdict, confidence, summary,
			recommendations, findings, sources, error_message
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,$10::jsonb,$11::jsonb,$12)
		RETURNING
			id, tenant_id, case_id, requested_by, model, status, verdict, confidence,
			summary, recommendations, findings, sources, error_message, created_at
	`, p.TenantID, p.CaseID, p.RequestedBy, p.Model, p.Status, p.Verdict, p.Confidence, p.Summary, recommendationsRaw, findingsRaw, sourcesRaw, p.ErrorMessage)

	item, err := scanCaseAIAnalysis(row)
	if err != nil {
		return nil, fmt.Errorf("create case ai analysis: %w", err)
	}
	return item, nil
}

func (r *CaseAIAnalysisRepository) ListByCase(ctx context.Context, tenantID, caseID uuid.UUID, limit int) ([]models.CaseAIAnalysis, error) {
	limit = normalizeLimit(limit, 500, 100)

	rows, err := r.pool.Query(ctx, `
		SELECT
			id, tenant_id, case_id, requested_by, model, status, verdict, confidence,
			summary, recommendations, findings, sources, error_message, created_at
		FROM case_ai_analyses
		WHERE tenant_id = $1 AND case_id = $2
		ORDER BY created_at DESC
		LIMIT $3
	`, tenantID, caseID, limit)
	if err != nil {
		return nil, fmt.Errorf("list case ai analyses: %w", err)
	}

	return collectRows(rows, limit, scanCaseAIAnalysis, "scan case ai analysis", "iterate case ai analyses")
}

func scanCaseAIAnalysis(scanner rowScanner) (*models.CaseAIAnalysis, error) {
	var (
		item               models.CaseAIAnalysis
		rawRecommendations []byte
		rawFindings        []byte
		rawSources         []byte
	)
	if err := scanner.Scan(
		&item.ID,
		&item.TenantID,
		&item.CaseID,
		&item.RequestedBy,
		&item.Model,
		&item.Status,
		&item.Verdict,
		&item.Confidence,
		&item.Summary,
		&rawRecommendations,
		&rawFindings,
		&rawSources,
		&item.ErrorMessage,
		&item.CreatedAt,
	); err != nil {
		return nil, err
	}

	if err := json.Unmarshal(rawRecommendations, &item.Recommendations); err != nil {
		return nil, err
	}
	if item.Recommendations == nil {
		item.Recommendations = []string{}
	}
	if err := json.Unmarshal(rawFindings, &item.Findings); err != nil {
		return nil, err
	}
	if item.Findings == nil {
		item.Findings = []string{}
	}
	if err := json.Unmarshal(rawSources, &item.Sources); err != nil {
		return nil, err
	}
	if item.Sources == nil {
		item.Sources = []map[string]any{}
	}
	return &item, nil
}
