package tracing

import (
	"context"
	"errors"
	"testing"

	"incidenthub/backend/internal/config"

	"go.opentelemetry.io/otel/attribute"
)

func TestInitDisabled(t *testing.T) {
	cfg := config.App{Trace: config.TraceConfig{Enabled: false}}
	shutdown, err := Init(context.Background(), cfg)
	if err != nil {
		t.Fatalf("init disabled tracing: %v", err)
	}
	if shutdown == nil {
		t.Fatalf("shutdown callback must not be nil")
	}
	shutdown()
}

func TestStartSpanAndHelpers(t *testing.T) {
	ctx, sp := Start(context.Background(), "tracing.test", attribute.String("k", "v"))
	if ctx == nil {
		t.Fatalf("start should return context")
	}
	sp.Error(nil)
	sp.Error(errors.New("boom"))
	sp.End()

	attrs := Attrs(attribute.String("x", "y"))
	if len(attrs) != 1 || attrs[0].Key != "x" {
		t.Fatalf("unexpected attrs: %#v", attrs)
	}
}
