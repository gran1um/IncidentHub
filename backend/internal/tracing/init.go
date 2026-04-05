package tracing

import (
	"context"

	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/logger"

	"gitlab.anyinfra.ru/golang-core/tracer"
)

type Shutdown func()

func Init(ctx context.Context, cfg config.App) (Shutdown, error) {
	if !cfg.Trace.Enabled {
		return func() {}, nil
	}

	envCollector := tracer.ProdCollectorEnv
	if cfg.Env == "dev" || cfg.Env == "test" {
		envCollector = tracer.DevCollectorEnv
	}

	shutdownTracer, err := tracer.Init(ctx, tracer.Config{
		EnvCollector:  envCollector,
		Log:           logger.Logger,
		TenantKey:     cfg.Trace.Tenant,
		ServiceKey:    cfg.Trace.ServiceKey,
		SageLogGroup:  cfg.Logger.Group,
		SageLogSystem: cfg.Logger.System,
	})
	if err != nil {
		return nil, err
	}

	return func() { _ = shutdownTracer(ctx) }, nil
}
