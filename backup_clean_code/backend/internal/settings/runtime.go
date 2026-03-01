package settings

import (
	"context"
	"incidenthub/backend/internal/config"
	"incidenthub/backend/internal/logger"
	"incidenthub/backend/internal/metrics"
	"incidenthub/backend/internal/tracing"
	"sync"
)

type Runtime struct {
	cfg             config.App
	metrics         *metrics.Collector
	shutdownTracing tracing.Shutdown
	closeOnce       sync.Once
}

func Init(ctx context.Context, cfg config.App) (*Runtime, error) {
	if err := logger.InitLogger(cfg); err != nil {
		return nil, err
	}

	shutdownTracing, err := tracing.Init(ctx, cfg)
	if err != nil {
		_ = logger.CloseLogger()
		return nil, err
	}

	rt := &Runtime{
		cfg:             cfg,
		metrics:         metrics.New(cfg.Metrics.Namespace),
		shutdownTracing: shutdownTracing,
	}
	metrics.SetGlobal(rt.metrics)
	return rt, nil
}

func (r *Runtime) Metrics() *metrics.Collector {
	if r == nil {
		return nil
	}
	return r.metrics
}

func (r *Runtime) Close() {
	if r == nil {
		return
	}
	r.closeOnce.Do(func() {
		metrics.SetGlobal(nil)
		if r.shutdownTracing != nil {
			r.shutdownTracing()
		}
		_ = logger.CloseLogger()
	})
}
