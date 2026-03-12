package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"incidenthub/backend/internal/repository"
)

func TestGetDutyOverviewReturnsCurrentAndNextShifts(t *testing.T) {
	env := newAPITestEnv(t)

	asOf := time.Date(2026, time.February, 10, 10, 30, 0, 0, time.UTC)
	_, err := env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID:  &env.tenantID,
		Kind:      "shifts",
		CreatedBy: &env.userID,
		Data: map[string]any{
			"analyst_id": env.userID.String(),
			"day":        10,
			"month":      2,
			"year":       2026,
			"start":      "09:00",
			"end":        "17:00",
		},
	})
	if err != nil {
		t.Fatalf("create current shift: %v", err)
	}
	_, err = env.catalog.Create(context.Background(), repository.CatalogCreateParams{
		TenantID:  &env.tenantID,
		Kind:      "shifts",
		CreatedBy: &env.userID,
		Data: map[string]any{
			"analyst_id": env.secondUserID.String(),
			"day":        10,
			"month":      2,
			"year":       2026,
			"start":      "17:00",
			"end":        "23:00",
		},
	})
	if err != nil {
		t.Fatalf("create next shift: %v", err)
	}

	c, rec := env.jsonContext(http.MethodGet, "/api/v1/duty/overview?at=2026-02-10T10:30:00Z", nil)
	setPath(c, "/api/v1/duty/overview", nil, nil)
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)
	err = env.handler.GetDutyOverview(c)
	mustStatusOK(t, err, rec, http.StatusOK)

	type dutyShiftPayload struct {
		ID        string    `json:"id"`
		AnalystID string    `json:"analyst_id"`
		StartsAt  time.Time `json:"starts_at"`
		EndsAt    time.Time `json:"ends_at"`
	}
	type dutyAnalystPayload struct {
		ID       string `json:"id"`
		FullName string `json:"full_name"`
		Role     string `json:"role"`
	}
	type dutyOverviewPayload struct {
		TenantID string               `json:"tenant_id"`
		AsOf     time.Time            `json:"as_of"`
		OnDuty   []dutyAnalystPayload `json:"on_duty"`
		Current  struct {
			Count    int                  `json:"count"`
			Shifts   []dutyShiftPayload   `json:"shifts"`
			Analysts []dutyAnalystPayload `json:"analysts"`
		} `json:"current"`
		Next struct {
			StartsAt *time.Time           `json:"starts_at"`
			Count    int                  `json:"count"`
			Shifts   []dutyShiftPayload   `json:"shifts"`
			Analysts []dutyAnalystPayload `json:"analysts"`
		} `json:"next"`
		Meta struct {
			InvalidShifts int `json:"invalid_shifts"`
			ParsedShifts  int `json:"parsed_shifts"`
		} `json:"meta"`
	}
	payload := decodeBody[dutyOverviewPayload](t, rec)

	if payload.TenantID != env.tenantID.String() {
		t.Fatalf("unexpected tenant id: %s", payload.TenantID)
	}
	if !payload.AsOf.Equal(asOf) {
		t.Fatalf("unexpected as_of: %s", payload.AsOf.Format(time.RFC3339))
	}
	if payload.Current.Count != 1 || len(payload.Current.Shifts) != 1 {
		t.Fatalf("expected one current shift, got count=%d len=%d", payload.Current.Count, len(payload.Current.Shifts))
	}
	if payload.Current.Shifts[0].AnalystID != env.userID.String() {
		t.Fatalf("unexpected current shift analyst_id: %s", payload.Current.Shifts[0].AnalystID)
	}
	if len(payload.OnDuty) != 1 || payload.OnDuty[0].ID != env.userID.String() {
		t.Fatalf("unexpected on_duty analysts: %+v", payload.OnDuty)
	}

	if payload.Next.StartsAt == nil {
		t.Fatalf("expected next.starts_at")
	}
	expectedNextStart := time.Date(2026, time.February, 10, 17, 0, 0, 0, time.UTC)
	if !payload.Next.StartsAt.Equal(expectedNextStart) {
		t.Fatalf("unexpected next.starts_at: %s", payload.Next.StartsAt.Format(time.RFC3339))
	}
	if payload.Next.Count != 1 || len(payload.Next.Shifts) != 1 {
		t.Fatalf("expected one next shift, got count=%d len=%d", payload.Next.Count, len(payload.Next.Shifts))
	}
	if payload.Next.Shifts[0].AnalystID != env.secondUserID.String() {
		t.Fatalf("unexpected next shift analyst_id: %s", payload.Next.Shifts[0].AnalystID)
	}
	if payload.Meta.InvalidShifts != 0 || payload.Meta.ParsedShifts != 2 {
		t.Fatalf("unexpected duty meta: %+v", payload.Meta)
	}
}

func TestGetDutyOverviewRejectsInvalidAt(t *testing.T) {
	env := newAPITestEnv(t)

	c, _ := env.jsonContext(http.MethodGet, "/api/v1/duty/overview?at=not-a-time", nil)
	setPath(c, "/api/v1/duty/overview", nil, nil)
	setIdentity(c, env.identity)
	setTenant(c, env.tenantID)

	err := env.handler.GetDutyOverview(c)
	if code := httpErrorCode(t, err); code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, code)
	}
}
