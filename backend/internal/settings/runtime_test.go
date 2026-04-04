package settings

import (
	"context"
	"testing"

	"incidenthub/backend/internal/config"
)

func TestRuntimeNilBranches(t *testing.T) {
	var rt *Runtime
	if rt.Metrics() != nil {
		t.Fatalf("nil runtime metrics must be nil")
	}
	rt.Close()
}

func TestInitFailsWithInvalidLogger(t *testing.T) {
	cfg := config.App{
		Env: "test",
		Logger: config.LoggerConfig{
			Group:  "g",
			System: "s",
			Level:  "bad-level",
		},
		Trace: config.TraceConfig{Enabled: false},
	}

	rt, err := Init(context.Background(), cfg)
	if err == nil {
		if rt != nil {
			rt.Close()
		}
		t.Fatalf("expected init error for invalid logger level")
	}
}

func TestInitAndClose(t *testing.T) {
	cfg := config.App{
		Env: "test",
		Logger: config.LoggerConfig{
			Group:  "g",
			System: "s",
			Level:  "info",
		},
		Trace: config.TraceConfig{Enabled: false},
		Metrics: config.MetricsConfig{
			Namespace: "incidenthub_test",
		},
	}

	rt, err := Init(context.Background(), cfg)
	if err != nil {
		t.Fatalf("init runtime: %v", err)
	}
	if rt.Metrics() == nil {
		t.Fatalf("metrics collector is required")
	}

	rt.Close()
	rt.Close()
}
