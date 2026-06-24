package ratelimit

import (
	"sync"
	"time"
)

// BucketConfig defines the rate limit configuration for a principal.
type BucketConfig struct {
	Rate  float64 // tokens per second
	Burst int     // max tokens (bucket capacity)
}

// BucketStats exposes the current state of a bucket.
type BucketStats struct {
	Tokens float64 `json:"tokens"`
	Rate   float64 `json:"rate"`
	Burst  int     `json:"burst"`
}

type bucket struct {
	config     BucketConfig
	tokens     float64
	lastRefill time.Time
	mu         sync.Mutex
}

// refill adds tokens based on elapsed time since last refill, capped at capacity.
func (b *bucket) refill() {
	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	if elapsed <= 0 {
		return
	}
	b.tokens += elapsed * b.config.Rate
	if b.tokens > float64(b.config.Burst) {
		b.tokens = float64(b.config.Burst)
	}
	b.lastRefill = now
}

// tryConsume attempts to refill and then consume 1 token. Returns true if successful.
func (b *bucket) tryConsume() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.refill()

	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// stats returns a snapshot of the bucket state.
func (b *bucket) stats() BucketStats {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.refill()
	return BucketStats{
		Tokens: b.tokens,
		Rate:   b.config.Rate,
		Burst:  b.config.Burst,
	}
}

// Limiter manages per-principal token buckets.
type Limiter struct {
	mu            sync.RWMutex
	buckets       map[string]*bucket
	defaultConfig BucketConfig
}

// New creates a Limiter with the given default rate (tokens/sec) and burst (max tokens).
func New(defaultRate float64, defaultBurst int) *Limiter {
	return &Limiter{
		buckets: make(map[string]*bucket),
		defaultConfig: BucketConfig{
			Rate:  defaultRate,
			Burst: defaultBurst,
		},
	}
}

// getOrCreate returns the bucket for the principal, creating one with defaults if absent.
func (l *Limiter) getOrCreate(principalID string) *bucket {
	l.mu.RLock()
	b, ok := l.buckets[principalID]
	l.mu.RUnlock()
	if ok {
		return b
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Double-check after acquiring write lock.
	if b, ok = l.buckets[principalID]; ok {
		return b
	}

	b = &bucket{
		config:     l.defaultConfig,
		tokens:     float64(l.defaultConfig.Burst), // start full
		lastRefill: time.Now(),
	}
	l.buckets[principalID] = b
	return b
}

// SetConfig overrides the rate limit configuration for a specific principal.
// On config change the bucket is reset to full capacity so new limits take
// effect immediately (admin raising a throttled user's quota should work at once).
func (l *Limiter) SetConfig(principalID string, rate float64, burst int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[principalID]
	if !ok {
		b = &bucket{
			lastRefill: time.Now(),
		}
		l.buckets[principalID] = b
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	b.config = BucketConfig{Rate: rate, Burst: burst}
	b.tokens = float64(burst)
	b.lastRefill = time.Now()
}

// Allow checks whether 1 token is available for the principal. Consumes it if so.
func (l *Limiter) Allow(principalID string) bool {
	b := l.getOrCreate(principalID)
	return b.tryConsume()
}

// Remove deletes a principal's bucket, freeing memory.
func (l *Limiter) Remove(principalID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.buckets, principalID)
}

// Stats returns a snapshot of all current buckets.
func (l *Limiter) Stats() map[string]BucketStats {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make(map[string]BucketStats, len(l.buckets))
	for id, b := range l.buckets {
		result[id] = b.stats()
	}
	return result
}
