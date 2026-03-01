package api

import (
	"net/http"
	"strings"
	"time"

	"incidenthub/backend/internal/middleware"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"

	"github.com/labstack/echo/v5"
)

type relatedObservablePayload struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type relatedCasePayload struct {
	models.Case
	MatchCount         int                        `json:"match_count"`
	MatchedObservables []relatedObservablePayload `json:"matched_observables"`
	MatchedFields      []relatedFieldPayload      `json:"matched_fields"`
}

type relatedFieldPayload struct {
	Field string `json:"field"`
	Value string `json:"value"`
}

type listCaseRelatedResponse struct {
	CaseID       string               `json:"case_id"`
	LinkBy       string               `json:"link_by"`
	GeneratedAt  string               `json:"generated_at"`
	ActiveRecent []relatedCasePayload `json:"active_recent"`
	AllTime      []relatedCasePayload `json:"all_time"`
}

func (h *Handler) ListRelatedCases(c *echo.Context) error {
	requestedTenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}
	_, caseID, err := h.resolveCaseInTenant(c)
	if err != nil {
		return err
	}

	linkBy := normalizeRelatedCasesLinkBy(c.QueryParam("link_by"))
	var (
		activeRecent []repository.RelatedCase
		allTime      []repository.RelatedCase
	)
	switch linkBy {
	case "observables":
		activeRecent, err = h.cases.ListRelatedCasesByObservables(c.Request().Context(), requestedTenantID, caseID, true, 50)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to list active related cases")
		}
		allTime, err = h.cases.ListRelatedCasesByObservables(c.Request().Context(), requestedTenantID, caseID, false, 200)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to list related cases")
		}
	case "incident_type", "source", "severity", "priority":
		activeRecent, err = h.cases.ListRelatedCasesByField(c.Request().Context(), requestedTenantID, caseID, linkBy, true, 50)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to list active related cases")
		}
		allTime, err = h.cases.ListRelatedCasesByField(c.Request().Context(), requestedTenantID, caseID, linkBy, false, 200)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to list related cases")
		}
	default:
		return echo.NewHTTPError(http.StatusBadRequest, "invalid link_by")
	}

	return c.JSON(http.StatusOK, listCaseRelatedResponse{
		CaseID:       caseID.String(),
		LinkBy:       linkBy,
		GeneratedAt:  time.Now().UTC().Format(time.RFC3339Nano),
		ActiveRecent: mapRelatedCasePayloads(activeRecent),
		AllTime:      mapRelatedCasePayloads(allTime),
	})
}

func mapRelatedCasePayloads(items []repository.RelatedCase) []relatedCasePayload {
	out := make([]relatedCasePayload, 0, len(items))
	for _, item := range items {
		row := relatedCasePayload{
			Case:               item.Case,
			MatchCount:         item.MatchCount,
			MatchedObservables: make([]relatedObservablePayload, 0, len(item.MatchedObservables)),
			MatchedFields:      make([]relatedFieldPayload, 0, len(item.MatchedFields)),
		}
		for _, observable := range item.MatchedObservables {
			row.MatchedObservables = append(row.MatchedObservables, relatedObservablePayload{
				Type:  observable.Type,
				Value: observable.Value,
			})
		}
		for _, field := range item.MatchedFields {
			row.MatchedFields = append(row.MatchedFields, relatedFieldPayload{
				Field: field.Field,
				Value: field.Value,
			})
		}
		out = append(out, row)
	}
	return out
}

func normalizeRelatedCasesLinkBy(value string) string {
	normalized := strings.TrimSpace(strings.ToLower(value))
	if normalized == "" {
		return "observables"
	}
	return normalized
}
