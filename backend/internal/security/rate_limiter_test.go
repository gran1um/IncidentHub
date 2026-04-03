package security

import (
	"testing"
	"time"
)

func TestRateLimiterAllow(t *testing.T) {
	t.Parallel()

	rl := NewRateLimiter(2, 100*time.Millisecond)
	if !rl.Allow("k") {
		t.Fatalf("first request should pass")
	}
	if !rl.Allow("k") {
		t.Fatalf("second request should pass")
	}
	if rl.Allow("k") {
		t.Fatalf("third request should be blocked")
	}

	time.Sleep(120 * time.Millisecond)
	if !rl.Allow("k") {
		t.Fatalf("request after window reset should pass")
	}
}
