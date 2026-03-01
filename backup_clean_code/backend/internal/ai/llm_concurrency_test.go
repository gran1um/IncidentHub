package ai

import (
	"context"
	"testing"
	"time"
)

func TestAcquireLLMSlotBlocksWhenLimitReached(t *testing.T) {
	globalLLMConcurrencyLimiter = llmConcurrencyLimiter{}

	release, err := acquireLLMSlot(context.Background(), 1)
	if err != nil {
		t.Fatalf("acquire first slot: %v", err)
	}

	acquiredSecond := make(chan struct{}, 1)
	go func() {
		releaseSecond, secondErr := acquireLLMSlot(context.Background(), 1)
		if secondErr != nil {
			return
		}
		close(acquiredSecond)
		releaseSecond()
	}()

	select {
	case <-acquiredSecond:
		t.Fatalf("expected second acquire to block while first slot is held")
	case <-time.After(60 * time.Millisecond):
		// expected block
	}

	release()

	select {
	case <-acquiredSecond:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("expected second acquire to continue after release")
	}
}

func TestAcquireLLMSlotHonorsContextCancellation(t *testing.T) {
	globalLLMConcurrencyLimiter = llmConcurrencyLimiter{}

	release, err := acquireLLMSlot(context.Background(), 1)
	if err != nil {
		t.Fatalf("acquire first slot: %v", err)
	}
	defer release()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err = acquireLLMSlot(ctx, 1)
	if err == nil {
		t.Fatalf("expected context cancellation while waiting for llm slot")
	}
}
