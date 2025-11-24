// Package middleware provides production-ready middleware for Unicorn framework
package middleware

import (
	"fmt"
	"runtime"
	"runtime/debug"

	"github.com/madcok-co/unicorn/core/pkg/context"
	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

// RecoveryConfig defines configuration for recovery middleware
type RecoveryConfig struct {
	// EnableStackTrace enables stack trace in logs (disable in production for security)
	EnableStackTrace bool

	// Logger for panic logging
	Logger contracts.Logger

	// OnPanic is called when panic occurs (for custom alerting)
	OnPanic func(ctx *context.Context, err interface{}, stack []byte)

	// DisableStackAll disables getting all goroutines stack
	DisableStackAll bool

	// StackSize is the size of the stack to be printed (default: 4KB)
	StackSize int
}

// DefaultRecoveryConfig returns default recovery configuration
func DefaultRecoveryConfig() *RecoveryConfig {
	return &RecoveryConfig{
		EnableStackTrace: true,
		StackSize:        4 << 10, // 4KB
	}
}

// Recovery returns a middleware that recovers from panics
func Recovery() context.MiddlewareFunc {
	return RecoveryWithConfig(DefaultRecoveryConfig())
}

// RecoveryWithConfig returns recovery middleware with custom config
func RecoveryWithConfig(config *RecoveryConfig) context.MiddlewareFunc {
	if config == nil {
		config = DefaultRecoveryConfig()
	}

	if config.StackSize <= 0 {
		config.StackSize = 4 << 10
	}

	return func(next context.HandlerFunc) context.HandlerFunc {
		return func(ctx *context.Context) error {
			defer func() {
				if r := recover(); r != nil {
					// Get stack trace
					var stack []byte
					if config.EnableStackTrace {
						if config.DisableStackAll {
							stack = make([]byte, config.StackSize)
							length := runtime.Stack(stack, false)
							stack = stack[:length]
						} else {
							stack = debug.Stack()
						}
					}

					// Log the panic
					if config.Logger != nil {
						config.Logger.Error("panic recovered",
							"error", fmt.Sprintf("%v", r),
							"stack", string(stack),
							"path", ctx.Request().Path,
							"method", ctx.Request().Method,
						)
					}

					// Call custom panic handler if set
					if config.OnPanic != nil {
						config.OnPanic(ctx, r, stack)
					}

					// Return 500 Internal Server Error
					// Don't expose internal error details to client
					_ = ctx.Error(500, "Internal server error")
				}
			}()

			return next(ctx)
		}
	}
}
