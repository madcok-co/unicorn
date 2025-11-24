package middleware

import (
	"context"
	"time"

	ucontext "github.com/madcok-co/unicorn/core/pkg/context"
)

// TimeoutConfig defines timeout middleware configuration
type TimeoutConfig struct {
	// Timeout duration for each request
	Timeout time.Duration

	// OnTimeout is called when request times out
	OnTimeout func(ctx *ucontext.Context)

	// Skipper defines a function to skip middleware
	Skipper func(ctx *ucontext.Context) bool
}

// DefaultTimeoutConfig returns default timeout configuration
func DefaultTimeoutConfig() *TimeoutConfig {
	return &TimeoutConfig{
		Timeout: 30 * time.Second,
	}
}

// Timeout returns middleware that times out requests
func Timeout(timeout time.Duration) ucontext.MiddlewareFunc {
	return TimeoutWithConfig(&TimeoutConfig{
		Timeout: timeout,
	})
}

// TimeoutWithConfig returns timeout middleware with custom config
func TimeoutWithConfig(config *TimeoutConfig) ucontext.MiddlewareFunc {
	if config == nil {
		config = DefaultTimeoutConfig()
	}

	if config.Timeout <= 0 {
		config.Timeout = 30 * time.Second
	}

	return func(next ucontext.HandlerFunc) ucontext.HandlerFunc {
		return func(ctx *ucontext.Context) error {
			// Check skipper
			if config.Skipper != nil && config.Skipper(ctx) {
				return next(ctx)
			}

			// Create timeout context
			timeoutCtx, cancel := context.WithTimeout(ctx.Context(), config.Timeout)
			defer cancel()

			// Update context with timeout context
			ctx.WithContext(timeoutCtx)

			// Run handler in goroutine
			done := make(chan error, 1)
			go func() {
				done <- next(ctx)
			}()

			// Wait for handler or timeout
			select {
			case err := <-done:
				return err
			case <-timeoutCtx.Done():
				if config.OnTimeout != nil {
					config.OnTimeout(ctx)
				}
				return ctx.Error(504, "Gateway Timeout")
			}
		}
	}
}
