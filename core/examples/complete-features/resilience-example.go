package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	httpAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/http"
	"github.com/madcok-co/unicorn/core/pkg/app"
	"github.com/madcok-co/unicorn/core/pkg/context"
	"github.com/madcok-co/unicorn/core/pkg/resilience"

	loggerAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/logger"
)

// ============================================================
// EXAMPLE: Resilience Patterns
// ============================================================
// This example demonstrates:
// - Circuit Breaker pattern
// - Retry with exponential backoff
// - Timeout handling
// - Bulkhead pattern
// ============================================================

// ============================================================
// Circuit Breaker Example
// ============================================================

var (
	// Simulate external service failures
	externalServiceFailureRate = 0.5 // 50% failure rate
	externalServiceCallCount   = 0
)

// ExternalService simulates an unreliable external service
type ExternalService struct {
	circuitBreaker *resilience.CircuitBreaker
}

func NewExternalService() *ExternalService {
	cb := resilience.NewCircuitBreaker(resilience.CircuitBreakerConfig{
		Name:             "external-service",
		MaxFailures:      3,                // Open after 3 failures
		Timeout:          10 * time.Second, // Try again after 10s
		HalfOpenRequests: 2,                // Allow 2 requests in half-open state
	})

	return &ExternalService{
		circuitBreaker: cb,
	}
}

func (s *ExternalService) Call() (string, error) {
	// Use circuit breaker
	result, err := s.circuitBreaker.Execute(func() (interface{}, error) {
		return s.actualCall()
	})

	if err != nil {
		return "", err
	}

	return result.(string), nil
}

func (s *ExternalService) actualCall() (string, error) {
	externalServiceCallCount++

	// Simulate random failures
	if rand.Float64() < externalServiceFailureRate {
		return "", fmt.Errorf("external service error: service unavailable")
	}

	// Simulate some latency
	time.Sleep(100 * time.Millisecond)

	return fmt.Sprintf("Success! Call #%d", externalServiceCallCount), nil
}

// CallExternalService - Handler with circuit breaker
func CallExternalService(ctx *context.Context) (map[string]interface{}, error) {
	externalSvc := ctx.GetService("externalService").(*ExternalService)

	result, err := externalSvc.Call()
	if err != nil {
		// Circuit breaker might be open
		ctx.Logger().Error("external service call failed", "error", err)

		return map[string]interface{}{
			"success": false,
			"error":   err.Error(),
			"state":   externalSvc.circuitBreaker.State().String(),
		}, nil
	}

	return map[string]interface{}{
		"success": true,
		"result":  result,
		"state":   externalSvc.circuitBreaker.State().String(),
	}, nil
}

// ============================================================
// Retry with Backoff Example
// ============================================================

// UnstableOperation simulates an operation that might fail
func unstableOperation(attemptNumber int) (string, error) {
	// First 2 attempts fail, 3rd succeeds
	if attemptNumber < 3 {
		return "", fmt.Errorf("operation failed on attempt %d", attemptNumber)
	}
	return fmt.Sprintf("Success on attempt %d", attemptNumber), nil
}

// RetryExample - Handler with retry logic
func RetryExample(ctx *context.Context) (map[string]interface{}, error) {
	logger := ctx.Logger()

	// Configure retry with exponential backoff
	retryConfig := resilience.RetryConfig{
		MaxAttempts:     5,
		InitialInterval: 100 * time.Millisecond,
		MaxInterval:     2 * time.Second,
		Multiplier:      2.0,
		OnRetry: func(attempt int, err error) {
			logger.Warn("retrying operation",
				"attempt", attempt,
				"error", err.Error())
		},
	}

	attemptCount := 0

	// Execute with retry
	result, err := resilience.Retry(retryConfig, func() (interface{}, error) {
		attemptCount++
		return unstableOperation(attemptCount)
	})

	if err != nil {
		return nil, fmt.Errorf("operation failed after %d attempts: %w", attemptCount, err)
	}

	return map[string]interface{}{
		"success":  true,
		"result":   result,
		"attempts": attemptCount,
	}, nil
}

// ============================================================
// Combined: Circuit Breaker + Retry
// ============================================================

// DatabaseService with circuit breaker and retry
type DatabaseService struct {
	circuitBreaker *resilience.CircuitBreaker
	retryConfig    resilience.RetryConfig
	failureRate    float64
}

func NewDatabaseService() *DatabaseService {
	cb := resilience.NewCircuitBreaker(resilience.CircuitBreakerConfig{
		Name:             "database",
		MaxFailures:      5,
		Timeout:          30 * time.Second,
		HalfOpenRequests: 3,
	})

	retryConfig := resilience.RetryConfig{
		MaxAttempts:     3,
		InitialInterval: 200 * time.Millisecond,
		MaxInterval:     2 * time.Second,
		Multiplier:      2.0,
	}

	return &DatabaseService{
		circuitBreaker: cb,
		retryConfig:    retryConfig,
		failureRate:    0.3, // 30% failure rate
	}
}

func (s *DatabaseService) Query(sql string) ([]map[string]interface{}, error) {
	// First apply circuit breaker
	result, err := s.circuitBreaker.Execute(func() (interface{}, error) {
		// Then apply retry logic
		return resilience.Retry(s.retryConfig, func() (interface{}, error) {
			return s.executeQuery(sql)
		})
	})

	if err != nil {
		return nil, err
	}

	return result.([]map[string]interface{}), nil
}

func (s *DatabaseService) executeQuery(sql string) ([]map[string]interface{}, error) {
	// Simulate random failures
	if rand.Float64() < s.failureRate {
		return nil, fmt.Errorf("database connection timeout")
	}

	// Simulate latency
	time.Sleep(50 * time.Millisecond)

	// Return mock data
	return []map[string]interface{}{
		{"id": 1, "name": "Product 1"},
		{"id": 2, "name": "Product 2"},
	}, nil
}

// QueryDatabase - Handler using database with resilience
func QueryDatabase(ctx *context.Context) (map[string]interface{}, error) {
	dbService := ctx.GetService("database").(*DatabaseService)

	query := ctx.Request().Query["q"]
	if query == "" {
		query = "SELECT * FROM products"
	}

	startTime := time.Now()
	results, err := dbService.Query(query)
	duration := time.Since(startTime)

	if err != nil {
		ctx.Logger().Error("database query failed",
			"error", err,
			"duration", duration)

		return map[string]interface{}{
			"success":  false,
			"error":    err.Error(),
			"duration": duration.String(),
			"state":    dbService.circuitBreaker.State().String(),
		}, nil
	}

	return map[string]interface{}{
		"success":  true,
		"results":  results,
		"count":    len(results),
		"duration": duration.String(),
		"state":    dbService.circuitBreaker.State().String(),
	}, nil
}

// ============================================================
// Circuit Breaker Status Monitoring
// ============================================================

// GetCircuitBreakerStatus - Get status of all circuit breakers
func GetCircuitBreakerStatus(ctx *context.Context) (map[string]interface{}, error) {
	externalSvc := ctx.GetService("externalService").(*ExternalService)
	dbSvc := ctx.GetService("database").(*DatabaseService)

	return map[string]interface{}{
		"circuit_breakers": map[string]interface{}{
			"external_service": map[string]interface{}{
				"state":         externalSvc.circuitBreaker.State().String(),
				"failure_count": externalSvc.circuitBreaker.Failures(),
			},
			"database": map[string]interface{}{
				"state":         dbSvc.circuitBreaker.State().String(),
				"failure_count": dbSvc.circuitBreaker.Failures(),
			},
		},
		"timestamp": time.Now(),
	}, nil
}

// ResetCircuitBreaker - Manually reset a circuit breaker
func ResetCircuitBreaker(ctx *context.Context) (map[string]interface{}, error) {
	name := ctx.Request().Query["name"]

	switch name {
	case "external":
		externalSvc := ctx.GetService("externalService").(*ExternalService)
		externalSvc.circuitBreaker.Reset()
		ctx.Logger().Info("circuit breaker reset", "service", "external")
	case "database":
		dbSvc := ctx.GetService("database").(*DatabaseService)
		dbSvc.circuitBreaker.Reset()
		ctx.Logger().Info("circuit breaker reset", "service", "database")
	default:
		return nil, fmt.Errorf("unknown circuit breaker: %s", name)
	}

	return map[string]interface{}{
		"message": "Circuit breaker reset successfully",
		"service": name,
	}, nil
}

// ============================================================
// Adjust Failure Rate (for testing)
// ============================================================

// SetFailureRate - Adjust failure rate for testing
func SetFailureRate(ctx *context.Context) (map[string]interface{}, error) {
	service := ctx.Request().Query["service"]
	rateStr := ctx.Request().Query["rate"]

	var rate float64
	fmt.Sscanf(rateStr, "%f", &rate)

	if rate < 0 || rate > 1 {
		return nil, fmt.Errorf("rate must be between 0 and 1")
	}

	switch service {
	case "external":
		externalServiceFailureRate = rate
	case "database":
		dbSvc := ctx.GetService("database").(*DatabaseService)
		dbSvc.failureRate = rate
	default:
		return nil, fmt.Errorf("unknown service: %s", service)
	}

	ctx.Logger().Info("failure rate updated", "service", service, "rate", rate)

	return map[string]interface{}{
		"message": "Failure rate updated",
		"service": service,
		"rate":    rate,
	}, nil
}

// ============================================================
// Main Application
// ============================================================

func runResilienceExample() {
	rand.Seed(time.Now().UnixNano())

	// Create application
	application := app.New(&app.Config{
		Name:       "resilience-example",
		Version:    "1.0.0",
		EnableHTTP: true,
		HTTP: &httpAdapter.Config{
			Host: "0.0.0.0",
			Port: 8083,
		},
	})

	// Setup infrastructure
	logger := loggerAdapter.NewConsoleLogger()
	application.SetLogger(logger)

	// Register resilient services
	externalService := NewExternalService()
	application.RegisterService("externalService", externalService)

	databaseService := NewDatabaseService()
	application.RegisterService("database", databaseService)

	// Register handlers
	application.RegisterHandler(CallExternalService).
		Named("call-external").
		HTTP("GET", "/external").
		Done()

	application.RegisterHandler(RetryExample).
		Named("retry-example").
		HTTP("GET", "/retry").
		Done()

	application.RegisterHandler(QueryDatabase).
		Named("query-database").
		HTTP("GET", "/database").
		Done()

	application.RegisterHandler(GetCircuitBreakerStatus).
		Named("cb-status").
		HTTP("GET", "/status/circuit-breakers").
		Done()

	application.RegisterHandler(ResetCircuitBreaker).
		Named("reset-cb").
		HTTP("POST", "/circuit-breaker/reset").
		Done()

	application.RegisterHandler(SetFailureRate).
		Named("set-failure-rate").
		HTTP("POST", "/testing/failure-rate").
		Done()

	// Startup hook
	application.OnStart(func() error {
		fmt.Println("🛡️  Resilience Patterns Example Started!")
		fmt.Println("\n📚 Available Endpoints:")
		fmt.Println("  GET  /external                           - Call external service (with circuit breaker)")
		fmt.Println("  GET  /retry                              - Retry example with backoff")
		fmt.Println("  GET  /database?q=SELECT...               - Query database (CB + Retry)")
		fmt.Println("  GET  /status/circuit-breakers            - Get circuit breaker status")
		fmt.Println("  POST /circuit-breaker/reset?name=X       - Reset circuit breaker (external|database)")
		fmt.Println("  POST /testing/failure-rate?service=X&rate=Y - Set failure rate (0.0-1.0)")
		fmt.Println()
		fmt.Println("🔧 Resilience Features:")
		fmt.Println("  ✓ Circuit Breaker - Prevents cascading failures")
		fmt.Println("  ✓ Retry with Backoff - Exponential backoff retry")
		fmt.Println("  ✓ Combined Patterns - CB + Retry for maximum resilience")
		fmt.Println()
		fmt.Println("💡 Try these commands:")
		fmt.Println("  # Call external service multiple times to trigger circuit breaker")
		fmt.Println("  for i in {1..10}; do curl http://localhost:8083/external; done")
		fmt.Println()
		fmt.Println("  # Check circuit breaker status")
		fmt.Println("  curl http://localhost:8083/status/circuit-breakers")
		fmt.Println()
		fmt.Println("  # Set failure rate to 80%")
		fmt.Println("  curl -X POST 'http://localhost:8083/testing/failure-rate?service=external&rate=0.8'")
		fmt.Println()
		fmt.Println("  # Reset circuit breaker")
		fmt.Println("  curl -X POST 'http://localhost:8083/circuit-breaker/reset?name=external'")
		fmt.Println()
		return nil
	})

	// Start
	log.Println("Starting Resilience Example on http://localhost:8083")
	if err := application.Start(); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}
