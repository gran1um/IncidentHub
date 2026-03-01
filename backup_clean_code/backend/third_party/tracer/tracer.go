package tracer

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	ottrace "go.opentelemetry.io/otel/trace"
)

const (
	ProdCollectorEnv = "prod"
	DevCollectorEnv  = "dev"
)

type Config struct {
	EnvCollector  string
	Log           any
	TenantKey     string
	ServiceKey    string
	SageLogGroup  string
	SageLogSystem string
}

var (
	mu       sync.Mutex
	provider *trace.TracerProvider
)

func Init(_ context.Context, _ Config) (func(context.Context) error, error) {
	mu.Lock()
	defer mu.Unlock()

	if provider != nil {
		p := provider
		return p.Shutdown, nil
	}

	p := trace.NewTracerProvider()
	otel.SetTracerProvider(p)
	provider = p
	return p.Shutdown, nil
}

func StartSpan(ctx context.Context, name string) (context.Context, ottrace.Span) {
	return otel.Tracer("alerter").Start(ctx, name)
}
