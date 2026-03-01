package api

import (
	"context"
	"incidenthub/backend/internal/config"
	"testing"
	"time"
)

func TestAIRequestContextClampsToWriteTimeout(t *testing.T) {
	h := &Handler{
		cfg: config.App{
			AI: config.AIConfig{
				Timeout: 20 * time.Second,
			},
			HTTP: config.HTTPConfig{
				WriteTimeout: 15 * time.Second,
			},
		},
	}

	ctx, cancel := h.aiRequestContext(context.Background())
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatalf("expected deadline in ai request context")
	}
	remaining := time.Until(deadline)
	if remaining > 15*time.Second || remaining < 12*time.Second {
		t.Fatalf("unexpected clamped timeout: %s", remaining)
	}
}

func TestAIRequestContextUsesConfiguredTimeoutWhenLower(t *testing.T) {
	h := &Handler{
		cfg: config.App{
			AI: config.AIConfig{
				Timeout: 4 * time.Second,
			},
			HTTP: config.HTTPConfig{
				WriteTimeout: 15 * time.Second,
			},
		},
	}

	ctx, cancel := h.aiRequestContext(context.Background())
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatalf("expected deadline in ai request context")
	}
	remaining := time.Until(deadline)
	if remaining > 5*time.Second || remaining < 2*time.Second {
		t.Fatalf("expected configured timeout to be used, got %s", remaining)
	}
}

func TestAIRequestContextUsesDefaultTimeoutAndWriteCap(t *testing.T) {
	h := &Handler{
		cfg: config.App{
			AI: config.AIConfig{
				Timeout: 0,
			},
			HTTP: config.HTTPConfig{
				WriteTimeout: 7 * time.Second,
			},
		},
	}

	ctx, cancel := h.aiRequestContext(context.Background())
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatalf("expected deadline in ai request context")
	}
	remaining := time.Until(deadline)
	if remaining > 7*time.Second || remaining < 5*time.Second {
		t.Fatalf("expected timeout capped by write timeout, got %s", remaining)
	}
}
