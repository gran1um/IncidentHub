package security

import (
	"sync"
	"time"
)

type hitWindow struct {
	hits  int
	reset time.Time
}

type RateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	entries map[string]hitWindow
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	if limit <= 0 {
		limit = 10
	}
	if window <= 0 {
		window = 5 * time.Minute
	}
	return &RateLimiter{
		limit:   limit,
		window:  window,
		entries: make(map[string]hitWindow),
	}
}

func (r *RateLimiter) Allow(key string) bool {
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()

	rec, ok := r.entries[key]
	if !ok || now.After(rec.reset) {
		r.entries[key] = hitWindow{hits: 1, reset: now.Add(r.window)}
		return true
	}
	if rec.hits >= r.limit {
		return false
	}
	rec.hits++
	r.entries[key] = rec
	return true
}
