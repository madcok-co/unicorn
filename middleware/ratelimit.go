// ============================================
// 2. RATE LIMIT MIDDLEWARE (Token Bucket)
// ============================================
package middleware

import (
	"errors"
	"sync"
	"time"

	"github.com/madcok-co/unicorn"
)

type RateLimitMiddleware struct {
	RequestsPerMinute int
	BurstSize         int
	buckets           map[string]*tokenBucket
	mu                sync.RWMutex
}

type tokenBucket struct {
	tokens     float64
	lastRefill time.Time
	mu         sync.Mutex
}

func NewRateLimitMiddleware(rpm, burst int) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		RequestsPerMinute: rpm,
		BurstSize:         burst,
		buckets:           make(map[string]*tokenBucket),
	}
}

func (m *RateLimitMiddleware) Handle(ctx *unicorn.Context, next func() (interface{}, error)) (interface{}, error) {
	// Get identifier (IP or user ID)
	identifier := ctx.GetMetadataString("client_ip")
	if identifier == "" {
		identifier = ctx.GetMetadataString("user_id")
	}
	if identifier == "" {
		identifier = "default"
	}

	// Check rate limit
	if !m.allowRequest(identifier) {
		return nil, errors.New("rate limit exceeded")
	}

	return next()
}

func (m *RateLimitMiddleware) allowRequest(identifier string) bool {
	m.mu.Lock()
	bucket, exists := m.buckets[identifier]
	if !exists {
		bucket = &tokenBucket{
			tokens:     float64(m.BurstSize),
			lastRefill: time.Now(),
		}
		m.buckets[identifier] = bucket
	}
	m.mu.Unlock()

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	// Refill tokens
	now := time.Now()
	elapsed := now.Sub(bucket.lastRefill).Seconds()
	tokensToAdd := elapsed * float64(m.RequestsPerMinute) / 60.0
	bucket.tokens = min(bucket.tokens+tokensToAdd, float64(m.BurstSize))
	bucket.lastRefill = now

	// Check if we have tokens
	if bucket.tokens < 1 {
		return false
	}

	bucket.tokens--
	return true
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
