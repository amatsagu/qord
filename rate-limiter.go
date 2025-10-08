package qord

import (
	"sync"
	"time"

	"github.com/amatsagu/tempest"
)

type rateLimiterEntry struct {
	mu        sync.Mutex
	uses      int
	maxUses   int
	recovery  time.Duration
	expiresAt time.Time
}

// RateLimiter manages independent rate limits per key.
type RateLimiter struct {
	mu      sync.Mutex
	entries map[tempest.Snowflake]*rateLimiterEntry
}

// NewRateLimiter creates a new empty limiter.
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		entries: make(map[tempest.Snowflake]*rateLimiterEntry),
	}
}

// SetLimit configures a key's maxUses and recovery time.
func (rl *RateLimiter) Set(key tempest.Snowflake, maxUses int, recovery time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	e, exists := rl.entries[key]
	if !exists {
		rl.entries[key] = &rateLimiterEntry{
			maxUses:   maxUses,
			recovery:  recovery,
			expiresAt: time.Now(),
		}
	} else {
		e.maxUses = maxUses
		e.recovery = recovery
	}
}

// Allow checks whether a key can be used immediately (non-blocking).
func (rl *RateLimiter) Allow(key tempest.Snowflake) bool {
	rl.mu.Lock()
	e, exists := rl.entries[key]
	rl.mu.Unlock()

	if !exists {
		return true
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	if now.After(e.expiresAt) {
		e.uses = 1
		e.expiresAt = now.Add(e.recovery)
		return true
	}

	if e.uses < e.maxUses {
		e.uses++
		return true
	}

	return false
}

// Wait blocks until the key is allowed again.
func (rl *RateLimiter) Wait(key tempest.Snowflake) {
	rl.mu.Lock()
	e, exists := rl.entries[key]
	rl.mu.Unlock()

	if !exists {
		return
	}

	for {
		e.mu.Lock()
		now := time.Now()

		if now.After(e.expiresAt) {
			e.uses = 1
			e.expiresAt = now.Add(e.recovery)
			e.mu.Unlock()
			return
		}

		if e.uses < e.maxUses {
			e.uses++
			e.mu.Unlock()
			return
		}

		waitTime := time.Until(e.expiresAt)
		e.mu.Unlock()
		time.Sleep(waitTime)
	}
}

// WaitOrSet waits if the key exists, otherwise creates it and consumes first use.
func (rl *RateLimiter) WaitOrSet(key tempest.Snowflake, maxUses int, recovery time.Duration) {
	rl.mu.Lock()
	_, exists := rl.entries[key]
	if !exists {
		e := &rateLimiterEntry{
			maxUses:   maxUses,
			recovery:  recovery,
			uses:      1,
			expiresAt: time.Now().Add(recovery),
		}
		rl.entries[key] = e
		rl.mu.Unlock()
		return
	}
	rl.mu.Unlock()

	// Key exists → behave like Wait
	rl.Wait(key)
}
