package tracing

import (
	"context"
	"fmt"
	"strings"
	"time"

	"incidenthub/backend/internal/metrics"

	"go.opentelemetry.io/otel/attribute"
)

func StartModuleOperation(ctx context.Context, module, operation string, attrs ...attribute.KeyValue) (context.Context, Span, time.Time) {
	module = sanitizeModuleLabel(module)
	operation = sanitizeModuleLabel(operation)
	baseAttrs := []attribute.KeyValue{
		attribute.String("component.module", module),
		attribute.String("component.operation", operation),
	}
	if len(attrs) > 0 {
		baseAttrs = append(baseAttrs, attrs...)
	}
	name := fmt.Sprintf("%s.%s", module, operation)
	ctx, span := Start(ctx, name, baseAttrs...)
	return ctx, span, time.Now()
}

func FinishModuleOperation(span Span, startedAt time.Time, module, operation string, err error) {
	status := "ok"
	if err != nil {
		status = "error"
	}
	FinishModuleOperationWithStatus(span, startedAt, module, operation, status, err)
}

func FinishModuleOperationWithStatus(span Span, startedAt time.Time, module, operation, status string, err error) {
	module = sanitizeModuleLabel(module)
	operation = sanitizeModuleLabel(operation)
	status = strings.TrimSpace(strings.ToLower(status))
	if status == "" {
		status = "ok"
	}
	if err != nil && status == "error" {
		span.Error(err)
	}
	span.End()
	metrics.ObserveModuleOperation(module, operation, status, time.Since(startedAt))
}

func sanitizeModuleLabel(input string) string {
	trimmed := strings.TrimSpace(strings.ToLower(input))
	if trimmed == "" {
		return "unknown"
	}
	return trimmed
}
