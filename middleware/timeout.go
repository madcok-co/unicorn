// ============================================
// 6. TIMEOUT MIDDLEWARE
// ============================================
package middleware

import (
	"context"
	"fmt"
	"time"

	"github.com/madcok-co/unicorn"
)

type TimeoutMiddleware struct {
	timeout time.Duration
}

func NewTimeoutMiddleware(timeout time.Duration) *TimeoutMiddleware {
	return &TimeoutMiddleware{
		timeout: timeout,
	}
}

func (m *TimeoutMiddleware) Handle(ctx *unicorn.Context, next func() (interface{}, error)) (interface{}, error) {
	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx.Context(), m.timeout)
	defer cancel()

	// Replace context
	originalCtx := ctx.Context()

	// Channel for result
	type result struct {
		data interface{}
		err  error
	}
	resultChan := make(chan result, 1)

	// Execute in goroutine
	go func() {
		data, err := next()
		resultChan <- result{data, err}
	}()

	// Wait for result or timeout
	select {
	case res := <-resultChan:
		// Restore original context
		ctx.WithContext(originalCtx)
		return res.data, res.err
	case <-timeoutCtx.Done():
		return nil, fmt.Errorf("request timeout after %v", m.timeout)
	}
}
