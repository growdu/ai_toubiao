// Package ratelimit provides a simple per-tenant token-bucket rate limiter.
// In production, replace with Redis-backed limiter.
package ratelimit

import (
	"sync"
	"time"
)

// Limiter is a thread-safe per-key token bucket.
type Limiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	rate     int           // tokens per window
	window   time.Duration // window size
	idleTTL  time.Duration // evict buckets after this idle period
	lastSweep time.Time
}

type bucket struct {
	tokens   int
	lastFill time.Time
}

// New returns a Limiter that allows `rate` requests per `window`.
// Example: New(60, time.Minute) = 60 req/min.
func New(rate int, window time.Duration) *Limiter {
	return &Limiter{
		buckets:  make(map[string]*bucket),
		rate:     rate,
		window:   window,
		idleTTL:  10 * time.Minute,
		lastSweep: time.Now(),
	}
}

// Allow returns true if the key is allowed to proceed; false if rate-limited.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	b, ok := l.buckets[key]
	if !ok {
		l.buckets[key] = &bucket{tokens: l.rate - 1, lastFill: now}
		return true
	}

	// Refill: tokens accrued since last fill
	elapsed := now.Sub(b.lastFill)
	refill := int(float64(l.rate) * elapsed.Seconds() / l.window.Seconds())
	if refill > 0 {
		b.tokens = min(b.tokens+refill, l.rate)
		b.lastFill = now
	}

	if b.tokens <= 0 {
		l.maybeSweep(now)
		return false
	}

	b.tokens--
	l.maybeSweep(now)
	return true
}

func (l *Limiter) maybeSweep(now time.Time) {
	if now.Sub(l.lastSweep) < time.Minute {
		return
	}
	l.lastSweep = now
	for k, b := range l.buckets {
		if now.Sub(b.lastFill) > l.idleTTL {
			delete(l.buckets, k)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}