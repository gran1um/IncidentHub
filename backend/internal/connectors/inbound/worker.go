package inbound

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/logger"
	"incidenthub/backend/internal/models"
	"incidenthub/backend/internal/repository"
	"incidenthub/backend/internal/search"
	"incidenthub/backend/internal/tracing"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/robfig/cron/v3"
)

type ConnectorStore interface {
	ListByKindAllTenants(ctx context.Context, kind string, limit, offset int) ([]models.CatalogItem, error)
}

type AlertStore interface {
	Create(ctx context.Context, p repository.CreateAlertParams) (*models.Alert, error)
}

type IngestStateStore interface {
	TryAcquire(ctx context.Context, tenantID, connectorID uuid.UUID, externalID, payloadHash string) (bool, error)
	BindAlert(ctx context.Context, tenantID, connectorID uuid.UUID, externalID string, alertID uuid.UUID) error
	ReleaseOnFailure(ctx context.Context, tenantID, connectorID uuid.UUID, externalID string) error
}

type RunStore interface {
	Create(ctx context.Context, p repository.CreateInboundConnectorRunParams) (*models.InboundConnectorRun, error)
	Finish(ctx context.Context, runID, tenantID uuid.UUID, p repository.FinishInboundConnectorRunParams) error
	GetLatestByConnector(ctx context.Context, tenantID, connectorID uuid.UUID) (*models.InboundConnectorRun, error)
}

type SearchIndexer interface {
	IndexDocument(ctx context.Context, kind, id string, doc any) error
}

type Worker struct {
	cfg        config.InboundConnectorsConfig
	catalog    ConnectorStore
	alerts     AlertStore
	ingest     IngestStateStore
	runs       RunStore
	search     SearchIndexer
	httpClient *http.Client
	startOnce  sync.Once
}

type RunStats struct {
	ConnectorsScanned int `json:"connectors_scanned"`
	RecordsSeen       int `json:"records_seen"`
	AlertsCreated     int `json:"alerts_created"`
	DuplicatesSkipped int `json:"duplicates_skipped"`
	Errors            int `json:"errors"`
}

func NewWorker(
	cfg config.InboundConnectorsConfig,
	catalog ConnectorStore,
	alerts AlertStore,
	ingest IngestStateStore,
	runs RunStore,
	searchIndexer SearchIndexer,
) *Worker {
	httpTimeout := cfg.HTTPTimeout
	if httpTimeout <= 0 {
		httpTimeout = 20 * time.Second
	}
	if cfg.BatchLimit <= 0 {
		cfg.BatchLimit = 200
	}
	return &Worker{
		cfg:        cfg,
		catalog:    catalog,
		alerts:     alerts,
		ingest:     ingest,
		runs:       runs,
		search:     searchIndexer,
		httpClient: &http.Client{Timeout: httpTimeout},
	}
}

func (w *Worker) Start(ctx context.Context) {
	if w == nil || !w.cfg.Enabled {
		return
	}
	if w.cfg.PollInterval <= 0 {
		w.cfg.PollInterval = 5 * time.Minute
	}

	w.startOnce.Do(func() {
		go func() {
			w.runAndLog(ctx, nil, nil, false, "cron")

			ticker := time.NewTicker(w.cfg.PollInterval)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					w.runAndLog(ctx, nil, nil, false, "cron")
				}
			}
		}()
	})
}

func (w *Worker) RunNow(ctx context.Context, tenantID *uuid.UUID, connectorID *uuid.UUID) (RunStats, error) {
	ctx, span, startedAt := tracing.StartModuleOperation(ctx, "connectors_inbound", "run_now")
	var err error
	defer func() {
		tracing.FinishModuleOperation(span, startedAt, "connectors_inbound", "run_now", err)
	}()

	if w == nil || !w.cfg.Enabled {
		return RunStats{}, nil
	}
	stats, err := w.run(ctx, tenantID, connectorID, true, "manual")
	return stats, err
}

func (w *Worker) runAndLog(ctx context.Context, tenantID *uuid.UUID, connectorID *uuid.UUID, force bool, trigger string) {
	ctx, span, startedAt := tracing.StartModuleOperation(ctx, "connectors_inbound", "run")
	stats, err := w.run(ctx, tenantID, connectorID, force, trigger)
	tracing.FinishModuleOperation(span, startedAt, "connectors_inbound", "run", err)
	if err != nil {
		logger.Errorf("inbound connectors run failed: %v", err)
		return
	}
	logger.Infof(
		"inbound connectors run completed: connectors=%d records=%d created=%d duplicates=%d errors=%d",
		stats.ConnectorsScanned,
		stats.RecordsSeen,
		stats.AlertsCreated,
		stats.DuplicatesSkipped,
		stats.Errors,
	)
}

func (w *Worker) run(ctx context.Context, tenantID *uuid.UUID, connectorID *uuid.UUID, force bool, trigger string) (RunStats, error) {
	stats := RunStats{}
	if w.catalog == nil || w.alerts == nil || w.ingest == nil {
		return stats, fmt.Errorf("inbound worker dependencies are not configured")
	}

	seen := map[uuid.UUID]struct{}{}
	kinds := []string{"inbound_connectors", "connectors"}
	for _, kind := range kinds {
		kindStats, err := w.runForKind(ctx, kind, tenantID, connectorID, force, trigger, seen)
		stats.ConnectorsScanned += kindStats.ConnectorsScanned
		stats.RecordsSeen += kindStats.RecordsSeen
		stats.AlertsCreated += kindStats.AlertsCreated
		stats.DuplicatesSkipped += kindStats.DuplicatesSkipped
		stats.Errors += kindStats.Errors
		if err != nil {
			return stats, err
		}
	}

	return stats, nil
}

func (w *Worker) runForKind(
	ctx context.Context,
	kind string,
	tenantID *uuid.UUID,
	connectorID *uuid.UUID,
	force bool,
	trigger string,
	seen map[uuid.UUID]struct{},
) (RunStats, error) {
	stats := RunStats{}
	offset := 0
	for {
		connectors, err := w.catalog.ListByKindAllTenants(ctx, kind, 200, offset)
		if err != nil {
			return stats, fmt.Errorf("list %s for inbound worker: %w", kind, err)
		}
		if len(connectors) == 0 {
			break
		}

		for _, connector := range connectors {
			if _, exists := seen[connector.ID]; exists {
				continue
			}
			seen[connector.ID] = struct{}{}

			if tenantID != nil {
				if connector.TenantID == nil || *connector.TenantID != *tenantID {
					continue
				}
			}
			if connectorID != nil && connector.ID != *connectorID {
				continue
			}
			if kind == "connectors" && !isInboundConnector(connector) {
				continue
			}
			if !isConnectorEnabled(connector.Data) {
				continue
			}

			cfg, cfgErr := parseInboundConnectorConfig(connector)
			if cfgErr != nil {
				logger.Warnf("inbound connector %s config error: %v", connector.ID.String(), cfgErr)
				stats.Errors++
				continue
			}

			execute, scheduledFor, scheduleErr := w.shouldExecuteConnector(ctx, connector, cfg, force)
			if scheduleErr != nil {
				logger.Warnf("inbound connector %s schedule error: %v", connector.ID.String(), scheduleErr)
				stats.Errors++
				continue
			}
			if !execute {
				continue
			}

			stats.ConnectorsScanned++
			runStats, runErr := w.processConnector(ctx, connector, cfg, scheduledFor, trigger)
			stats.RecordsSeen += runStats.RecordsSeen
			stats.AlertsCreated += runStats.AlertsCreated
			stats.DuplicatesSkipped += runStats.DuplicatesSkipped
			stats.Errors += runStats.Errors
			if runErr != nil {
				logger.Warnf("inbound connector %s processing failed: %v", connector.ID.String(), runErr)
			}
		}

		if len(connectors) < 200 {
			break
		}
		offset += 200
	}
	return stats, nil
}

func (w *Worker) shouldExecuteConnector(
	ctx context.Context,
	connector models.CatalogItem,
	cfg inboundConnectorConfig,
	force bool,
) (bool, *time.Time, error) {
	if connector.TenantID == nil {
		return false, nil, nil
	}
	if force {
		return true, nil, nil
	}

	rawSchedule := strings.TrimSpace(cfg.Schedule)
	if rawSchedule == "" {
		return true, nil, nil
	}

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	schedule, err := parser.Parse(rawSchedule)
	if err != nil {
		return false, nil, fmt.Errorf("parse schedule %q: %w", rawSchedule, err)
	}

	now := time.Now().UTC()
	var lastStarted *time.Time
	if w.runs != nil {
		lastRun, runErr := w.runs.GetLatestByConnector(ctx, *connector.TenantID, connector.ID)
		if runErr != nil && !errors.Is(runErr, pgx.ErrNoRows) {
			return false, nil, fmt.Errorf("get latest connector run: %w", runErr)
		}
		if lastRun != nil {
			normalized := lastRun.StartedAt.UTC()
			lastStarted = &normalized
		}
	}

	var scheduledFor time.Time
	if lastStarted != nil {
		scheduledFor = schedule.Next(*lastStarted)
	} else {
		window := w.cfg.PollInterval
		if window < time.Minute {
			window = time.Minute
		}
		scheduledFor = schedule.Next(now.Add(-window))
	}
	if scheduledFor.After(now) {
		return false, &scheduledFor, nil
	}
	return true, &scheduledFor, nil
}

func (w *Worker) processConnector(
	ctx context.Context,
	connector models.CatalogItem,
	cfg inboundConnectorConfig,
	scheduledFor *time.Time,
	trigger string,
) (RunStats, error) {
	stats := RunStats{}
	if connector.TenantID == nil {
		return stats, nil
	}

	runID := uuid.Nil
	if w.runs != nil {
		created, err := w.runs.Create(ctx, repository.CreateInboundConnectorRunParams{
			TenantID:     *connector.TenantID,
			ConnectorID:  connector.ID,
			ScheduledFor: scheduledFor,
			Trigger:      trigger,
		})
		if err != nil {
			logger.Warnf("create inbound connector run failed for %s: %v", connector.ID.String(), err)
		} else {
			runID = created.ID
		}
	}

	var runLog strings.Builder
	_, _ = fmt.Fprintf(&runLog, "connector=%s started_at=%s\n", connector.ID.String(), time.Now().UTC().Format(time.RFC3339))

	records, err := w.fetchRecords(ctx, cfg)
	if err != nil {
		stats.Errors++
		_, _ = fmt.Fprintf(&runLog, "fetch_error=%s\n", err.Error())
		w.finishRun(ctx, runID, *connector.TenantID, stats, "error", err.Error(), runLog.String())
		return stats, err
	}
	if len(records) > w.cfg.BatchLimit {
		records = records[:w.cfg.BatchLimit]
	}
	_, _ = fmt.Fprintf(&runLog, "records_fetched=%d\n", len(records))

	for _, record := range records {
		stats.RecordsSeen++

		externalID := resolveExternalID(cfg, record)
		payloadHash := hashRecord(record)
		acquired, err := w.ingest.TryAcquire(ctx, *connector.TenantID, connector.ID, externalID, payloadHash)
		if err != nil {
			stats.Errors++
			continue
		}
		if !acquired {
			stats.DuplicatesSkipped++
			continue
		}

		title := strings.TrimSpace(extractString(record, cfg.TitleField))
		if title == "" {
			title = fmt.Sprintf("%s: %s", cfg.DisplayName, externalID)
		}
		alertSource := strings.TrimSpace(cfg.Source)
		if alertSource == "" {
			alertSource = strings.TrimSpace(extractString(record, cfg.SourceField))
		}
		if alertSource == "" {
			alertSource = cfg.DisplayName
		}

		description := strings.TrimSpace(extractString(record, cfg.DescriptionField))
		severity := normalizeSeverity(extractString(record, cfg.SeverityField), cfg.Severity)
		status := normalizeAlertStatus(extractString(record, cfg.StatusField), cfg.Status)
		tlp := normalizeTrafficLight(extractString(record, cfg.TLPField), cfg.TLP)
		pap := normalizeTrafficLight(extractString(record, cfg.PAPField), cfg.PAP)

		createdBy := connector.CreatedBy
		if createdBy == nil {
			createdBy = connector.OwnerID
		}

		alert, err := w.alerts.Create(ctx, repository.CreateAlertParams{
			TenantID:    *connector.TenantID,
			Title:       title,
			Description: description,
			Source:      alertSource,
			Status:      status,
			Severity:    severity,
			TLP:         tlp,
			PAP:         pap,
			CreatedBy:   createdBy,
		})
		if err != nil {
			stats.Errors++
			_ = w.ingest.ReleaseOnFailure(ctx, *connector.TenantID, connector.ID, externalID)
			continue
		}

		if err := w.ingest.BindAlert(ctx, *connector.TenantID, connector.ID, externalID, alert.ID); err != nil {
			stats.Errors++
		}
		stats.AlertsCreated++

		if w.search != nil {
			_ = w.search.IndexDocument(ctx, "alerts", alert.ID.String(), map[string]any{
				"id":           alert.ID.String(),
				"tenant_id":    alert.TenantID.String(),
				"title":        alert.Title,
				"description":  alert.Description,
				"source":       alert.Source,
				"status":       alert.Status,
				"severity":     alert.Severity,
				"external_id":  externalID,
				"connector_id": connector.ID.String(),
			})
		}
	}

	status := "success"
	message := fmt.Sprintf(
		"processed=%d created=%d duplicates=%d errors=%d",
		stats.RecordsSeen,
		stats.AlertsCreated,
		stats.DuplicatesSkipped,
		stats.Errors,
	)
	if stats.Errors > 0 {
		status = "error"
	}
	runLog.WriteString(message + "\n")
	w.finishRun(ctx, runID, *connector.TenantID, stats, status, message, runLog.String())

	return stats, nil
}

func (w *Worker) finishRun(
	ctx context.Context,
	runID uuid.UUID,
	tenantID uuid.UUID,
	stats RunStats,
	status string,
	message string,
	logs string,
) {
	if w.runs == nil || runID == uuid.Nil {
		return
	}
	if status == "" {
		status = "success"
	}
	if len(logs) > 8000 {
		logs = logs[:8000]
	}
	if err := w.runs.Finish(ctx, runID, tenantID, repository.FinishInboundConnectorRunParams{
		Status:            status,
		RecordsSeen:       stats.RecordsSeen,
		AlertsCreated:     stats.AlertsCreated,
		DuplicatesSkipped: stats.DuplicatesSkipped,
		Errors:            stats.Errors,
		Message:           message,
		Logs:              logs,
	}); err != nil {
		logger.Warnf("finish inbound connector run failed for run %s: %v", runID.String(), err)
	}
}

func hashRecord(record map[string]any) string {
	raw, _ := json.Marshal(record)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func isInboundConnector(item models.CatalogItem) bool {
	if strings.EqualFold(strings.TrimSpace(item.Kind), "inbound_connectors") {
		return true
	}
	direction := strings.ToLower(strings.TrimSpace(extractString(item.Data, "direction")))
	if direction == "" {
		direction = strings.ToLower(strings.TrimSpace(extractString(item.Data, "mode")))
	}
	return direction == "inbound"
}

func isConnectorEnabled(data map[string]any) bool {
	value, ok := data["enabled"]
	if !ok {
		return true
	}
	switch raw := value.(type) {
	case bool:
		return raw
	case string:
		v := strings.ToLower(strings.TrimSpace(raw))
		return v != "false" && v != "0" && v != "disabled"
	default:
		return true
	}
}

func normalizeSeverity(value string, fallback string) string {
	raw := strings.ToLower(strings.TrimSpace(value))
	if raw == "" {
		raw = strings.ToLower(strings.TrimSpace(fallback))
	}
	switch raw {
	case "critical", "high", "medium", "low":
		return raw
	default:
		return "medium"
	}
}

func normalizeAlertStatus(value string, fallback string) string {
	raw := strings.ToLower(strings.TrimSpace(value))
	if raw == "" {
		raw = strings.ToLower(strings.TrimSpace(fallback))
	}
	switch raw {
	case "new", "triaged", "closed":
		return raw
	default:
		return "new"
	}
}

func normalizeTrafficLight(value string, fallback string) string {
	raw := strings.ToLower(strings.TrimSpace(value))
	if raw == "" {
		raw = strings.ToLower(strings.TrimSpace(fallback))
	}
	switch raw {
	case "red", "amber", "green", "clear":
		return raw
	default:
		return "amber"
	}
}

var _ SearchIndexer = (*search.Client)(nil)
