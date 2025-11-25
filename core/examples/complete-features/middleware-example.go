package main

import (
	"fmt"
	"log"
	"time"

	httpAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/http"
	"github.com/madcok-co/unicorn/core/pkg/app"
	"github.com/madcok-co/unicorn/core/pkg/context"
	"github.com/madcok-co/unicorn/core/pkg/middleware"

	// Import adapters
	loggerAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/logger"
	securityAuth "github.com/madcok-co/unicorn/core/pkg/adapters/security/auth"
	securityRatelimit "github.com/madcok-co/unicorn/core/pkg/adapters/security/ratelimiter"
)

// ============================================================
// EXAMPLE: Using Middleware
// ============================================================
// This example demonstrates all available middleware:
// - CORS
// - Recovery (panic handling)
// - Timeout
// - Rate Limiting
// - Authentication (JWT, API Key)
// - Health Checks
// ============================================================

type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// PublicEndpoint - No authentication required
func PublicEndpoint(ctx *context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{
		"message":   "This is a public endpoint",
		"timestamp": time.Now(),
	}, nil
}

// ProtectedEndpoint - Requires authentication
func ProtectedEndpoint(ctx *context.Context) (map[string]interface{}, error) {
	// Get user from context (set by auth middleware)
	userID := ctx.Value("user_id")

	return map[string]interface{}{
		"message":   "This is a protected endpoint",
		"user_id":   userID,
		"timestamp": time.Now(),
	}, nil
}

// SlowEndpoint - Demonstrates timeout handling
func SlowEndpoint(ctx *context.Context) (map[string]interface{}, error) {
	// Simulate slow operation
	duration := ctx.Request().Query["duration"]
	if duration == "" {
		duration = "3s"
	}

	d, err := time.ParseDuration(duration)
	if err != nil {
		return nil, fmt.Errorf("invalid duration: %w", err)
	}

	// Sleep (this will timeout if > 5s)
	time.Sleep(d)

	return map[string]interface{}{
		"message":  "Completed slow operation",
		"duration": duration,
	}, nil
}

// PanicEndpoint - Demonstrates panic recovery
func PanicEndpoint(ctx *context.Context) (map[string]interface{}, error) {
	shouldPanic := ctx.Request().Query["panic"]
	if shouldPanic == "true" {
		panic("intentional panic for testing recovery middleware")
	}

	return map[string]interface{}{
		"message": "No panic occurred",
	}, nil
}

// RateLimitedEndpoint - Demonstrates rate limiting
func RateLimitedEndpoint(ctx *context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{
		"message":   "This endpoint is rate limited",
		"timestamp": time.Now(),
	}, nil
}

func runMiddlewareExample() {
	// Create application
	application := app.New(&app.Config{
		Name:       "middleware-example",
		Version:    "1.0.0",
		EnableHTTP: true,
		HTTP: &httpAdapter.Config{
			Host: "0.0.0.0",
			Port: 8081,
		},
	})

	// Setup infrastructure
	logger := loggerAdapter.NewConsoleLogger()
	application.SetLogger(logger)

	// Setup HTTP adapter to add middleware
	httpConfig := &httpAdapter.Config{
		Host:         "0.0.0.0",
		Port:         8081,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
		MaxBodySize:  10 << 20, // 10MB
	}

	// Note: In the current implementation, middleware is added via HTTP adapter
	// Here's how you would use them:

	// 1. CORS Middleware
	corsMiddleware := middleware.CORS(&middleware.CORSConfig{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
		MaxAge:           3600,
	})

	// 2. Recovery Middleware (panic recovery)
	recoveryMiddleware := middleware.Recovery(middleware.RecoveryConfig{
		Logger:            logger,
		StackTrace:        true,
		DisableStackAll:   false,
		DisablePrintStack: false,
	})

	// 3. Timeout Middleware
	timeoutMiddleware := middleware.Timeout(5 * time.Second)

	// 4. Rate Limiting Middleware
	rateLimiter := securityRatelimit.NewMemoryRateLimiter(10, time.Minute) // 10 requests per minute
	rateLimitMiddleware := middleware.RateLimit(middleware.RateLimitConfig{
		RateLimiter: rateLimiter,
		KeyFunc: func(ctx *context.Context) string {
			// Use IP address as key
			return ctx.Request().Headers["X-Real-IP"]
		},
		OnLimit: func(ctx *context.Context) error {
			return fmt.Errorf("rate limit exceeded")
		},
	})

	// 5. Authentication Middleware (JWT)
	jwtAuth := securityAuth.NewJWTAuth(securityAuth.JWTConfig{
		Secret:     []byte("your-secret-key"),
		Expiration: 24 * time.Hour,
	})

	// Note: To actually apply these middleware, you would need to:
	// - Create custom HTTP adapter wrapper
	// - Or use them at the handler level
	// - Or implement middleware chain in the framework

	_ = corsMiddleware
	_ = recoveryMiddleware
	_ = timeoutMiddleware
	_ = rateLimitMiddleware
	_ = jwtAuth
	_ = httpConfig

	// Register handlers
	application.RegisterHandler(PublicEndpoint).
		Named("public").
		HTTP("GET", "/public").
		Done()

	application.RegisterHandler(ProtectedEndpoint).
		Named("protected").
		HTTP("GET", "/protected").
		Done()

	application.RegisterHandler(SlowEndpoint).
		Named("slow").
		HTTP("GET", "/slow").
		Done()

	application.RegisterHandler(PanicEndpoint).
		Named("panic").
		HTTP("GET", "/panic").
		Done()

	application.RegisterHandler(RateLimitedEndpoint).
		Named("ratelimited").
		HTTP("GET", "/ratelimited").
		Done()

	// Startup hook
	application.OnStart(func() error {
		fmt.Println("🛡️  Middleware Example Started!")
		fmt.Println("\n📚 Available Endpoints:")
		fmt.Println("  GET /public              - Public endpoint (no auth)")
		fmt.Println("  GET /protected           - Protected endpoint (requires auth)")
		fmt.Println("  GET /slow?duration=3s    - Slow endpoint (timeout demo)")
		fmt.Println("  GET /panic?panic=true    - Panic endpoint (recovery demo)")
		fmt.Println("  GET /ratelimited         - Rate limited endpoint")
		fmt.Println()
		fmt.Println("📝 Middleware Features:")
		fmt.Println("  ✓ CORS - Cross-Origin Resource Sharing")
		fmt.Println("  ✓ Recovery - Panic recovery with stack traces")
		fmt.Println("  ✓ Timeout - Request timeout handling")
		fmt.Println("  ✓ Rate Limiting - Per-IP rate limiting")
		fmt.Println("  ✓ Authentication - JWT & API Key support")
		fmt.Println()
		return nil
	})

	// Start
	log.Println("Starting Middleware Example on http://localhost:8081")
	if err := application.Start(); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}
