package inbound

import (
	"context"
	"fmt"
	"incidenthub/backend/internal/tracing"
	"strings"
)

func (w *Worker) fetchRecords(ctx context.Context, cfg inboundConnectorConfig) ([]map[string]any, error) {
	ctx, span, startedAt := tracing.StartModuleOperation(ctx, "connectors_inbound", "fetch_records")
	var err error
	defer func() {
		tracing.FinishModuleOperation(span, startedAt, "connectors_inbound", "fetch_records", err)
	}()

	var records []map[string]any
	switch cfg.SourceType {
	case "http":
		records, err = w.fetchHTTPRecords(ctx, cfg)
	case "sql":
		records, err = w.fetchSQLRecords(ctx, cfg)
	case "s3":
		records, err = w.fetchS3Records(ctx, cfg)
	case "kafka":
		records, err = w.fetchKafkaRecords(ctx, cfg)
	case "redis":
		records, err = w.fetchRedisRecords(ctx, cfg)
	default:
		err = fmt.Errorf("inbound connector %q has unsupported source type %q", cfg.DisplayName, strings.TrimSpace(cfg.SourceType))
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	return records, nil
}

func resolveExternalID(cfg inboundConnectorConfig, record map[string]any) string {
	id := strings.TrimSpace(extractString(record, cfg.IDField))
	if id != "" {
		return id
	}
	return hashRecord(record)
}
