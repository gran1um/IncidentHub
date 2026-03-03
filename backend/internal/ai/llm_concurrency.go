package ai

import (
	"context"
	"sync"
)

type llmConcurrencyLimiter struct {
	mu    sync.Mutex
	sem   chan struct{}
	limit int
}

//nolint:gochecknoglobals // Shared limiter for all LLM calls; initialized lazily.
var globalLLMConcurrencyLimiter llmConcurrencyLimiter

func acquireLLMSlot(ctx context.Context, configuredLimit int) (func(), error) {
	limit := configuredLimit
	if limit <= 0 {
		return func() {}, nil
	}
	if limit > 128 {
		limit = 128
	}
	sem := globalLLMConcurrencyLimiter.getOrInit(limit)
	select {
	case sem <- struct{}{}:
		return func() {
			<-sem
		}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (l *llmConcurrencyLimiter) getOrInit(limit int) chan struct{} {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.sem == nil {
		l.limit = limit
		l.sem = make(chan struct{}, limit)
	}
	return l.sem
}
