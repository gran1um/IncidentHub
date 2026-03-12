package api

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"incidenthub/backend/internal/middleware"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"

	"github.com/labstack/echo/v5"
)

const (
	dutyDefaultShiftLimit = 2000
	dutyMaxShiftLimit     = 5000
	dutyUserLimit         = 1000
)

type dutyShiftSnapshot struct {
	Item      models.CatalogItem
	AnalystID string
	StartAt   time.Time
	EndAt     time.Time
	Overnight bool
}

func (shift dutyShiftSnapshot) IsActiveAt(at time.Time) bool {
	return !at.Before(shift.StartAt) && at.Before(shift.EndAt)
}

func (h *Handler) GetDutyOverview(c *echo.Context) error {
	if h.catalog == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "catalog repository is unavailable")
	}
	if h.users == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "user repository is unavailable")
	}

	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "tenant header required")
	}

	asOf, err := parseDutyAsOf(c.QueryParam("at"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid at timestamp, expected RFC3339")
	}
	limit := parseDutyLimit(c.QueryParam("limit"))

	shifts, err := h.catalog.List(c.Request().Context(), repository.CatalogListParams{
		Kind:     "shifts",
		TenantID: &tenantID,
		Limit:    limit,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list shifts")
	}

	tenantUsers, err := h.users.ListByTenantWithRole(c.Request().Context(), tenantID, dutyUserLimit, 0)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list tenant users")
	}
	for idx := range tenantUsers {
		h.enrichTenantUserMediaURLs(c.Request().Context(), &tenantUsers[idx])
	}
	usersByID := make(map[string]models.TenantUser, len(tenantUsers))
	for _, user := range tenantUsers {
		usersByID[strings.ToLower(strings.TrimSpace(user.ID.String()))] = user
	}

	snapshots := make([]dutyShiftSnapshot, 0, len(shifts))
	invalidShifts := 0
	for _, item := range shifts {
		snapshot, ok := buildDutyShiftSnapshot(item)
		if !ok {
			invalidShifts++
			continue
		}
		snapshots = append(snapshots, snapshot)
	}
	sort.SliceStable(snapshots, func(i, j int) bool {
		if snapshots[i].StartAt.Equal(snapshots[j].StartAt) {
			return snapshots[i].Item.ID.String() < snapshots[j].Item.ID.String()
		}
		return snapshots[i].StartAt.Before(snapshots[j].StartAt)
	})

	currentShifts := make([]dutyShiftSnapshot, 0, 8)
	nextShifts := make([]dutyShiftSnapshot, 0, 8)
	var nextStartsAt time.Time
	for _, shift := range snapshots {
		if shift.IsActiveAt(asOf) {
			currentShifts = append(currentShifts, shift)
		}
		if !shift.StartAt.After(asOf) {
			continue
		}
		if nextStartsAt.IsZero() || shift.StartAt.Before(nextStartsAt) {
			nextStartsAt = shift.StartAt
			nextShifts = nextShifts[:0]
		}
		if shift.StartAt.Equal(nextStartsAt) {
			nextShifts = append(nextShifts, shift)
		}
	}

	currentPayloads := buildDutyShiftPayloads(currentShifts, usersByID)
	nextPayloads := buildDutyShiftPayloads(nextShifts, usersByID)
	onDutyAnalysts := buildDutyAnalystPayloads(currentShifts, usersByID)
	nextAnalysts := buildDutyAnalystPayloads(nextShifts, usersByID)

	var nextAt any
	if !nextStartsAt.IsZero() {
		nextAt = nextStartsAt.UTC()
	}

	return c.JSON(http.StatusOK, map[string]any{
		"tenant_id": tenantID.String(),
		"as_of":     asOf.UTC(),
		"on_duty":   onDutyAnalysts,
		"current": map[string]any{
			"count":    len(currentPayloads),
			"shifts":   currentPayloads,
			"analysts": onDutyAnalysts,
		},
		"next": map[string]any{
			"starts_at": nextAt,
			"count":     len(nextPayloads),
			"shifts":    nextPayloads,
			"analysts":  nextAnalysts,
		},
		"meta": map[string]any{
			"loaded_shifts":  len(shifts),
			"parsed_shifts":  len(snapshots),
			"invalid_shifts": invalidShifts,
			"limit":          limit,
		},
	})
}

func parseDutyAsOf(raw string) (time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Now().UTC(), nil
	}
	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}

func parseDutyLimit(raw string) int {
	limit := dutyDefaultShiftLimit
	if parsed, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil {
		limit = parsed
	}
	if limit <= 0 {
		limit = dutyDefaultShiftLimit
	}
	if limit > dutyMaxShiftLimit {
		limit = dutyMaxShiftLimit
	}
	return limit
}

func buildDutyShiftSnapshot(item models.CatalogItem) (dutyShiftSnapshot, bool) {
	day, ok := intFromMap(item.Data, "day")
	if !ok {
		return dutyShiftSnapshot{}, false
	}
	month, ok := intFromMap(item.Data, "month")
	if !ok {
		return dutyShiftSnapshot{}, false
	}
	year, ok := intFromMap(item.Data, "year")
	if !ok {
		return dutyShiftSnapshot{}, false
	}

	startRaw := strings.TrimSpace(stringFromMap(item.Data, "start", "start_time"))
	if startRaw == "" {
		startRaw = "08:00"
	}
	endRaw := strings.TrimSpace(stringFromMap(item.Data, "end", "end_time"))
	if endRaw == "" {
		endRaw = "16:00"
	}

	startHour, startMinute, ok := parseDutyClock(startRaw)
	if !ok {
		return dutyShiftSnapshot{}, false
	}
	endHour, endMinute, ok := parseDutyClock(endRaw)
	if !ok {
		return dutyShiftSnapshot{}, false
	}

	startAt := time.Date(year, time.Month(month), day, startHour, startMinute, 0, 0, time.UTC)
	if startAt.Year() != year || int(startAt.Month()) != month || startAt.Day() != day {
		return dutyShiftSnapshot{}, false
	}
	endAt := time.Date(year, time.Month(month), day, endHour, endMinute, 0, 0, time.UTC)
	if !endAt.After(startAt) {
		endAt = endAt.Add(24 * time.Hour)
	}

	analystID := strings.TrimSpace(stringFromMap(item.Data, "analyst_id", "analystId", "user_id", "user", "owner_id"))
	if analystID == "" && item.OwnerID != nil {
		analystID = item.OwnerID.String()
	}

	return dutyShiftSnapshot{
		Item:      item,
		AnalystID: analystID,
		StartAt:   startAt,
		EndAt:     endAt,
		Overnight: startAt.Year() != endAt.Year() || startAt.YearDay() != endAt.YearDay(),
	}, true
}

func parseDutyClock(raw string) (outHour int, outMinute int, ok bool) {
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) != 2 {
		return 0, 0, false
	}
	hour, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || hour < 0 || hour > 23 {
		return 0, 0, false
	}
	minute, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || minute < 0 || minute > 59 {
		return 0, 0, false
	}
	return hour, minute, true
}

func buildDutyShiftPayloads(shifts []dutyShiftSnapshot, usersByID map[string]models.TenantUser) []map[string]any {
	payloads := make([]map[string]any, 0, len(shifts))
	for _, shift := range shifts {
		payload := catalogItemToPayload(shift.Item)
		payload["starts_at"] = shift.StartAt.UTC()
		payload["ends_at"] = shift.EndAt.UTC()
		payload["is_overnight"] = shift.Overnight
		payload["duration_minutes"] = int(shift.EndAt.Sub(shift.StartAt).Minutes())
		if shift.AnalystID != "" {
			payload["analyst_id"] = shift.AnalystID
			if user, ok := usersByID[strings.ToLower(strings.TrimSpace(shift.AnalystID))]; ok {
				payload["analyst"] = dutyAnalystPayload(user)
			}
		}
		payloads = append(payloads, payload)
	}
	return payloads
}

func buildDutyAnalystPayloads(shifts []dutyShiftSnapshot, usersByID map[string]models.TenantUser) []map[string]any {
	payloads := make([]map[string]any, 0, len(shifts))
	seen := make(map[string]struct{}, len(shifts))
	for _, shift := range shifts {
		analystID := strings.TrimSpace(strings.ToLower(shift.AnalystID))
		if analystID == "" {
			continue
		}
		if _, exists := seen[analystID]; exists {
			continue
		}
		seen[analystID] = struct{}{}

		if user, ok := usersByID[analystID]; ok {
			payloads = append(payloads, dutyAnalystPayload(user))
			continue
		}
		payloads = append(payloads, map[string]any{
			"id": shift.AnalystID,
		})
	}
	return payloads
}

func dutyAnalystPayload(user models.TenantUser) map[string]any {
	return map[string]any{
		"id":                user.ID.String(),
		"tenant_id":         user.TenantID.String(),
		"role":              user.Role,
		"username":          user.Username,
		"email":             user.Email,
		"full_name":         user.FullName,
		"team":              user.Team,
		"avatar_url":        user.AvatarURL,
		"is_active":         user.IsActive,
		"is_platform_admin": user.IsPlatformAdmin,
		"experience_points": user.ExperiencePoints,
	}
}
