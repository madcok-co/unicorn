package ratelimiter

import (
	"context"
	"sync"
	"time"

	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

// bucket represents a token bucket for a specific key
type bucket struct {
	mu         sync.Mutex
	tokens     int
	lastUpdate time.Time
}

// InMemoryRateLimiterConfig adalah konfigurasi untuk in-memory rate limiter
type InMemoryRateLimiterConfig struct {
	// Maximum requests per window
	Limit int

	// Time window duration
	Window time.Duration

	// Burst allowance (additional tokens above limit)
	Burst int

	// Cleanup interval for expired buckets
	CleanupInterval time.Duration
}

// DefaultInMemoryRateLimiterConfig returns default configuration
func DefaultInMemoryRateLimiterConfig() *InMemoryRateLimiterConfig {
	return &InMemoryRateLimiterConfig{
		Limit:           100,
		Window:          time.Minute,
		Burst:           10,
		CleanupInterval: 5 * time.Minute,
	}
}

// InMemoryRateLimiter implements RateLimiter using in-memory storage
type InMemoryRateLimiter struct {
	config   *InMemoryRateLimiterConfig
	buckets  sync.Map
	stopCh   chan struct{}
	stopOnce sync.Once
}

// NewInMemoryRateLimiter creates a new in-memory rate limiter
func NewInMemoryRateLimiter(config *InMemoryRateLimiterConfig) *InMemoryRateLimiter {
	if config == nil {
		config = DefaultInMemoryRateLimiterConfig()
	}

	// Ensure cleanup interval is set
	if config.CleanupInterval <= 0 {
		config.CleanupInterval = 5 * time.Minute
	}

	rl := &InMemoryRateLimiter{
		config: config,
		stopCh: make(chan struct{}),
	}

	// Start cleanup goroutine
	go rl.cleanupLoop()

	return rl
}

// Allow checks if a single request is allowed
func (r *InMemoryRateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	return r.AllowN(ctx, key, 1)
}

// AllowN checks if N requests are allowed
func (r *InMemoryRateLimiter) AllowN(ctx context.Context, key string, n int) (bool, error) {
	now := time.Now()
	maxTokens := r.config.Limit + r.config.Burst

	// Get or create bucket
	val, _ := r.buckets.LoadOrStore(key, &bucket{
		tokens:     maxTokens,
		lastUpdate: now,
	})

	b := val.(*bucket)

	// Lock the bucket for thread-safe access
	b.mu.Lock()
	defer b.mu.Unlock()

	// Calculate token refill
	elapsed := now.Sub(b.lastUpdate)
	refillRate := float64(r.config.Limit) / float64(r.config.Window)
	tokensToAdd := int(elapsed.Seconds() * refillRate)

	// Update bucket
	b.tokens = min(maxTokens, b.tokens+tokensToAdd)
	b.lastUpdate = now

	// Check if request is allowed
	if b.tokens >= n {
		b.tokens -= n
		return true, nil
	}

	return false, nil
}

// Remaining returns remaining requests in window
func (r *InMemoryRateLimiter) Remaining(ctx context.Context, key string) (int, error) {
	val, ok := r.buckets.Load(key)
	if !ok {
		return r.config.Limit + r.config.Burst, nil
	}

	b := val.(*bucket)

	b.mu.Lock()
	defer b.mu.Unlock()

	// Calculate current tokens with refill
	now := time.Now()
	elapsed := now.Sub(b.lastUpdate)
	refillRate := float64(r.config.Limit) / float64(r.config.Window)
	tokensToAdd := int(elapsed.Seconds() * refillRate)

	maxTokens := r.config.Limit + r.config.Burst
	currentTokens := min(maxTokens, b.tokens+tokensToAdd)

	return currentTokens, nil
}

// Reset resets the limit for a key
func (r *InMemoryRateLimiter) Reset(ctx context.Context, key string) error {
	r.buckets.Delete(key)
	return nil
}

// Close stops the cleanup goroutine
func (r *InMemoryRateLimiter) Close() error {
	r.stopOnce.Do(func() {
		close(r.stopCh)
	})
	return nil
}

// cleanupLoop periodically removes expired buckets
func (r *InMemoryRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(r.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.cleanup()
		case <-r.stopCh:
			return
		}
	}
}

// cleanup removes buckets that haven't been accessed in a while
func (r *InMemoryRateLimiter) cleanup() {
	threshold := time.Now().Add(-r.config.Window * 2)

	r.buckets.Range(func(key, value any) bool {
		b := value.(*bucket)
		b.mu.Lock()
		shouldDelete := b.lastUpdate.Before(threshold)
		b.mu.Unlock()

		if shouldDelete {
			r.buckets.Delete(key)
		}
		return true
	})
}

// GetConfig returns the rate limiter configuration
func (r *InMemoryRateLimiter) GetConfig() *InMemoryRateLimiterConfig {
	return r.config
}

// SlidingWindowRateLimiter implements a sliding window rate limiter
type SlidingWindowRateLimiter struct {
	config   *InMemoryRateLimiterConfig
	windows  sync.Map
	stopCh   chan struct{}
	stopOnce sync.Once
}

type windowEntry struct {
	mu        sync.Mutex
	requests  []time.Time
	lastClean time.Time
}

// NewSlidingWindowRateLimiter creates a sliding window rate limiter
func NewSlidingWindowRateLimiter(config *InMemoryRateLimiterConfig) *SlidingWindowRateLimiter {
	if config == nil {
		config = DefaultInMemoryRateLimiterConfig()
	}

	// Ensure cleanup interval is set
	if config.CleanupInterval <= 0 {
		config.CleanupInterval = 5 * time.Minute
	}

	rl := &SlidingWindowRateLimiter{
		config: config,
		stopCh: make(chan struct{}),
	}

	go rl.cleanupLoop()

	return rl
}

// Allow checks if a single request is allowed
func (r *SlidingWindowRateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	return r.AllowN(ctx, key, 1)
}

// AllowN checks if N requests are allowed
func (r *SlidingWindowRateLimiter) AllowN(ctx context.Context, key string, n int) (bool, error) {
	now := time.Now()
	windowStart := now.Add(-r.config.Window)

	val, _ := r.windows.LoadOrStore(key, &windowEntry{
		requests: make([]time.Time, 0),
	})

	entry := val.(*windowEntry)
	entry.mu.Lock()
	defer entry.mu.Unlock()

	// Remove expired requests
	validRequests := make([]time.Time, 0, len(entry.requests))
	for _, t := range entry.requests {
		if t.After(windowStart) {
			validRequests = append(validRequests, t)
		}
	}
	entry.requests = validRequests

	// Check limit
	maxAllowed := r.config.Limit + r.config.Burst
	if len(entry.requests)+n > maxAllowed {
		return false, nil
	}

	// Add new requests
	for i := 0; i < n; i++ {
		entry.requests = append(entry.requests, now)
	}

	return true, nil
}

// Remaining returns remaining requests in window
func (r *SlidingWindowRateLimiter) Remaining(ctx context.Context, key string) (int, error) {
	now := time.Now()
	windowStart := now.Add(-r.config.Window)

	val, ok := r.windows.Load(key)
	if !ok {
		return r.config.Limit + r.config.Burst, nil
	}

	entry := val.(*windowEntry)
	entry.mu.Lock()
	defer entry.mu.Unlock()

	// Count valid requests
	count := 0
	for _, t := range entry.requests {
		if t.After(windowStart) {
			count++
		}
	}

	return (r.config.Limit + r.config.Burst) - count, nil
}

// Reset resets the limit for a key
func (r *SlidingWindowRateLimiter) Reset(ctx context.Context, key string) error {
	r.windows.Delete(key)
	return nil
}

// Close stops the cleanup goroutine
func (r *SlidingWindowRateLimiter) Close() error {
	r.stopOnce.Do(func() {
		close(r.stopCh)
	})
	return nil
}

func (r *SlidingWindowRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(r.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.cleanup()
		case <-r.stopCh:
			return
		}
	}
}

func (r *SlidingWindowRateLimiter) cleanup() {
	threshold := time.Now().Add(-r.config.Window * 2)

	r.windows.Range(func(key, value any) bool {
		entry := value.(*windowEntry)
		entry.mu.Lock()
		shouldDelete := entry.lastClean.Before(threshold) && len(entry.requests) == 0
		entry.lastClean = time.Now()
		entry.mu.Unlock()

		if shouldDelete {
			r.windows.Delete(key)
		}
		return true
	})
}

// Helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Ensure implementations satisfy RateLimiter interface
var _ contracts.RateLimiter = (*InMemoryRateLimiter)(nil)
var _ contracts.RateLimiter = (*SlidingWindowRateLimiter)(nil)
