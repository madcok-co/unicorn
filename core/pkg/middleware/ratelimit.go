package middleware

import (
	"context"
	"sync"
	"time"

	ucontext "github.com/madcok-co/unicorn/core/pkg/context"
	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

// RateLimitConfig defines rate limiter middleware configuration
type RateLimitConfig struct {
	// Limit is the maximum number of requests per window
	Limit int

	// Window is the time window for rate limiting
	Window time.Duration

	// KeyFunc extracts the rate limit key from context (e.g., IP, user ID)
	KeyFunc func(ctx *ucontext.Context) string

	// Store is the rate limit store (memory or Redis-based)
	Store RateLimitStore

	// ErrorHandler is called when rate limit is exceeded
	ErrorHandler func(ctx *ucontext.Context, retryAfter time.Duration) error

	// Skipper defines a function to skip middleware
	Skipper func(ctx *ucontext.Context) bool

	// ExceedHandler is called when limit is exceeded (for logging/metrics)
	ExceedHandler func(ctx *ucontext.Context, key string)
}

// RateLimitStore interface for rate limit storage
type RateLimitStore interface {
	// Allow checks if request is allowed and increments counter
	// Returns: allowed (bool), remaining (int), retryAfter (time.Duration), error
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, time.Duration, error)

	// Reset resets the rate limit for a key
	Reset(ctx context.Context, key string) error
}

// DefaultRateLimitConfig returns default rate limit configuration
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		Limit:  100,
		Window: time.Minute,
		KeyFunc: func(ctx *ucontext.Context) string {
			// Default: use client IP
			return ctx.Request().Header("X-Forwarded-For")
		},
		ErrorHandler: func(ctx *ucontext.Context, retryAfter time.Duration) error {
			ctx.Response().SetHeader("Retry-After", retryAfter.String())
			return ctx.Error(429, "Too Many Requests")
		},
	}
}

// RateLimit returns rate limit middleware with default config
func RateLimit(limit int, window time.Duration) ucontext.MiddlewareFunc {
	config := DefaultRateLimitConfig()
	config.Limit = limit
	config.Window = window
	config.Store = NewMemoryRateLimitStore()
	return RateLimitWithConfig(config)
}

// RateLimitWithConfig returns rate limit middleware with custom config
func RateLimitWithConfig(config *RateLimitConfig) ucontext.MiddlewareFunc {
	if config == nil {
		config = DefaultRateLimitConfig()
	}

	if config.Limit <= 0 {
		config.Limit = 100
	}
	if config.Window <= 0 {
		config.Window = time.Minute
	}
	if config.KeyFunc == nil {
		config.KeyFunc = func(ctx *ucontext.Context) string {
			return ctx.Request().Header("X-Forwarded-For")
		}
	}
	if config.Store == nil {
		config.Store = NewMemoryRateLimitStore()
	}
	if config.ErrorHandler == nil {
		config.ErrorHandler = func(ctx *ucontext.Context, retryAfter time.Duration) error {
			return ctx.Error(429, "Too Many Requests")
		}
	}

	return func(next ucontext.HandlerFunc) ucontext.HandlerFunc {
		return func(ctx *ucontext.Context) error {
			// Check skipper
			if config.Skipper != nil && config.Skipper(ctx) {
				return next(ctx)
			}

			// Get rate limit key
			key := config.KeyFunc(ctx)
			if key == "" {
				// No key = no rate limiting
				return next(ctx)
			}

			// Check rate limit
			allowed, remaining, retryAfter, err := config.Store.Allow(
				ctx.Context(),
				key,
				config.Limit,
				config.Window,
			)
			if err != nil {
				// On error, allow request (fail-open)
				return next(ctx)
			}

			// Set rate limit headers
			ctx.Response().SetHeader("X-RateLimit-Limit", itoa(config.Limit))
			ctx.Response().SetHeader("X-RateLimit-Remaining", itoa(remaining))

			if !allowed {
				// Rate limit exceeded
				if config.ExceedHandler != nil {
					config.ExceedHandler(ctx, key)
				}
				return config.ErrorHandler(ctx, retryAfter)
			}

			return next(ctx)
		}
	}
}

// ============ Memory-based Rate Limit Store ============

// MemoryRateLimitStore is an in-memory rate limit store
type MemoryRateLimitStore struct {
	mu      sync.RWMutex
	entries map[string]*rateLimitEntry
	stopCh  chan struct{}
}

type rateLimitEntry struct {
	count     int
	expiresAt time.Time
}

// NewMemoryRateLimitStore creates a new memory-based rate limit store
func NewMemoryRateLimitStore() *MemoryRateLimitStore {
	store := &MemoryRateLimitStore{
		entries: make(map[string]*rateLimitEntry),
		stopCh:  make(chan struct{}),
	}

	// Start cleanup goroutine
	go store.cleanup()

	return store
}

// Allow implements RateLimitStore
func (s *MemoryRateLimitStore) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, time.Duration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	entry, exists := s.entries[key]

	// Create new entry if not exists or expired
	if !exists || now.After(entry.expiresAt) {
		s.entries[key] = &rateLimitEntry{
			count:     1,
			expiresAt: now.Add(window),
		}
		return true, limit - 1, 0, nil
	}

	// Check if limit exceeded
	if entry.count >= limit {
		retryAfter := entry.expiresAt.Sub(now)
		return false, 0, retryAfter, nil
	}

	// Increment counter
	entry.count++
	remaining := limit - entry.count

	return true, remaining, 0, nil
}

// Reset implements RateLimitStore
func (s *MemoryRateLimitStore) Reset(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.entries, key)
	return nil
}

// Close stops the cleanup goroutine
func (s *MemoryRateLimitStore) Close() error {
	close(s.stopCh)
	return nil
}

func (s *MemoryRateLimitStore) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			now := time.Now()
			for key, entry := range s.entries {
				if now.After(entry.expiresAt) {
					delete(s.entries, key)
				}
			}
			s.mu.Unlock()
		case <-s.stopCh:
			return
		}
	}
}

// ============ Redis-based Rate Limit Store ============

// RedisRateLimitStore uses Redis for distributed rate limiting
type RedisRateLimitStore struct {
	cache  contracts.Cache
	prefix string
}

// NewRedisRateLimitStore creates a Redis-based rate limit store
func NewRedisRateLimitStore(cache contracts.Cache, prefix string) *RedisRateLimitStore {
	if prefix == "" {
		prefix = "ratelimit:"
	}
	return &RedisRateLimitStore{
		cache:  cache,
		prefix: prefix,
	}
}

// Allow implements RateLimitStore using sliding window algorithm
func (s *RedisRateLimitStore) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, time.Duration, error) {
	fullKey := s.prefix + key

	// Get current count
	var data []byte
	err := s.cache.Get(ctx, fullKey, &data)
	if err != nil {
		// Key doesn't exist, create new
		countBytes := []byte{1}
		if err := s.cache.Set(ctx, fullKey, countBytes, window); err != nil {
			return true, limit - 1, 0, err // Fail-open
		}
		return true, limit - 1, 0, nil
	}

	// Parse count
	count := 1
	if len(data) > 0 {
		count = int(data[0])
	}

	// Check limit
	if count >= limit {
		// Get TTL for retry-after
		ttl, _ := s.cache.TTL(ctx, fullKey)
		return false, 0, ttl, nil
	}

	// Increment
	count++
	countBytes := []byte{byte(count)}
	// Note: This should ideally use INCR in Redis, but using Set for simplicity
	// In production, use Redis INCR command directly
	if err := s.cache.Set(ctx, fullKey, countBytes, window); err != nil {
		return true, limit - count, 0, err
	}

	return true, limit - count, 0, nil
}

// Reset implements RateLimitStore
func (s *RedisRateLimitStore) Reset(ctx context.Context, key string) error {
	return s.cache.Delete(ctx, s.prefix+key)
}

// ============ Sliding Window Rate Limiter ============

// SlidingWindowConfig for more accurate rate limiting
type SlidingWindowConfig struct {
	Limit     int
	Window    time.Duration
	Precision int // Number of sub-windows
	Store     RateLimitStore
	KeyFunc   func(ctx *ucontext.Context) string
	Skipper   func(ctx *ucontext.Context) bool
}

// ============ Helper Functions ============

func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return intToString(i)
}

func intToString(i int) string {
	if i == 0 {
		return "0"
	}

	var b [20]byte
	pos := len(b)
	negative := i < 0
	if negative {
		i = -i
	}

	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}

	if negative {
		pos--
		b[pos] = '-'
	}

	return string(b[pos:])
}

// ============ Rate Limit by User ID ============

// RateLimitByUserID creates a rate limiter that uses user ID from context
func RateLimitByUserID(limit int, window time.Duration, userKey string) ucontext.MiddlewareFunc {
	config := DefaultRateLimitConfig()
	config.Limit = limit
	config.Window = window
	config.Store = NewMemoryRateLimitStore()
	config.KeyFunc = func(ctx *ucontext.Context) string {
		if user, ok := ctx.Get(userKey); ok && user != nil {
			if m, ok := user.(map[string]interface{}); ok {
				if id, ok := m["id"].(string); ok {
					return "user:" + id
				}
			}
		}
		// Fallback to IP
		return "ip:" + ctx.Request().Header("X-Forwarded-For")
	}
	return RateLimitWithConfig(config)
}

// RateLimitByIP creates a rate limiter that uses client IP
func RateLimitByIP(limit int, window time.Duration) ucontext.MiddlewareFunc {
	config := DefaultRateLimitConfig()
	config.Limit = limit
	config.Window = window
	config.Store = NewMemoryRateLimitStore()
	config.KeyFunc = func(ctx *ucontext.Context) string {
		// Try X-Forwarded-For first (behind proxy)
		if xff := ctx.Request().Header("X-Forwarded-For"); xff != "" {
			return "ip:" + xff
		}
		// Fallback to X-Real-IP
		if xri := ctx.Request().Header("X-Real-IP"); xri != "" {
			return "ip:" + xri
		}
		return "ip:unknown"
	}
	return RateLimitWithConfig(config)
}
