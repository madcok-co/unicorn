// Package middleware provides production-ready middleware for Unicorn framework.
//
// Available middleware:
//
// Recovery - Panic recovery with stack trace logging
//
//	app.Use(middleware.Recovery())
//
// Timeout - Request timeout handling
//
//	app.Use(middleware.Timeout(30 * time.Second))
//
// CORS - Cross-Origin Resource Sharing
//
//	app.Use(middleware.CORS())
//
// RateLimit - Rate limiting (memory or Redis)
//
//	app.Use(middleware.RateLimit(100, time.Minute))
//
// JWT - JWT authentication
//
//	app.Use(middleware.JWT([]byte("secret")))
//
// APIKey - API key authentication
//
//	app.Use(middleware.APIKey(validator))
//
// BasicAuth - Basic authentication
//
//	app.Use(middleware.BasicAuth(validator))
//
// Tracing - Distributed tracing (OpenTelemetry compatible)
//
//	app.Use(middleware.Tracing(tracer))
//
// Metrics - HTTP metrics collection
//
//	app.Use(middleware.Metrics(meterProvider))
//
// Health - Health check endpoints
//
//	health := middleware.NewHealthHandler(config)
//	app.GET("/health", health.Handler())
package middleware

import (
	"time"

	"github.com/madcok-co/unicorn/core/pkg/context"
)

// Chain chains multiple middleware together
func Chain(middlewares ...context.MiddlewareFunc) context.MiddlewareFunc {
	return func(next context.HandlerFunc) context.HandlerFunc {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

// Stack is an alias for Chain (for those who prefer this term)
var Stack = Chain

// DefaultStack returns a production-ready middleware stack
func DefaultStack() context.MiddlewareFunc {
	return Chain(
		Recovery(),
		CORS(),
	)
}

// ProductionStack returns a full production middleware stack
func ProductionStack(config *ProductionStackConfig) context.MiddlewareFunc {
	if config == nil {
		config = &ProductionStackConfig{}
	}

	middlewares := []context.MiddlewareFunc{
		Recovery(),
	}

	// Add tracing if configured
	if config.Tracer != nil {
		middlewares = append(middlewares, Tracing(config.Tracer))
	}

	// Add metrics if configured
	if config.MeterProvider != nil {
		middlewares = append(middlewares, Metrics(config.MeterProvider))
	}

	// Add CORS
	if config.CORSConfig != nil {
		middlewares = append(middlewares, CORSWithConfig(config.CORSConfig))
	} else {
		middlewares = append(middlewares, CORS())
	}

	// Add rate limiting
	if config.RateLimitConfig != nil {
		middlewares = append(middlewares, RateLimitWithConfig(config.RateLimitConfig))
	}

	// Add timeout
	if config.Timeout > 0 {
		middlewares = append(middlewares, Timeout(config.Timeout))
	}

	return Chain(middlewares...)
}

// ProductionStackConfig configures the production middleware stack
type ProductionStackConfig struct {
	// Tracer for distributed tracing
	Tracer Tracer

	// MeterProvider for metrics
	MeterProvider MeterProvider

	// CORSConfig for CORS configuration
	CORSConfig *CORSConfig

	// RateLimitConfig for rate limiting
	RateLimitConfig *RateLimitConfig

	// Timeout for request timeout
	Timeout time.Duration
}

// ConditionalMiddleware returns middleware that only runs if condition is true
func ConditionalMiddleware(condition func(*context.Context) bool, mw context.MiddlewareFunc) context.MiddlewareFunc {
	return func(next context.HandlerFunc) context.HandlerFunc {
		return func(ctx *context.Context) error {
			if condition(ctx) {
				return mw(next)(ctx)
			}
			return next(ctx)
		}
	}
}

// PathMiddleware returns middleware that only runs for specific paths
func PathMiddleware(paths []string, mw context.MiddlewareFunc) context.MiddlewareFunc {
	pathMap := make(map[string]bool)
	for _, p := range paths {
		pathMap[p] = true
	}

	return ConditionalMiddleware(func(ctx *context.Context) bool {
		return pathMap[ctx.Request().Path]
	}, mw)
}

// MethodMiddleware returns middleware that only runs for specific methods
func MethodMiddleware(methods []string, mw context.MiddlewareFunc) context.MiddlewareFunc {
	methodMap := make(map[string]bool)
	for _, m := range methods {
		methodMap[m] = true
	}

	return ConditionalMiddleware(func(ctx *context.Context) bool {
		return methodMap[ctx.Request().Method]
	}, mw)
}
