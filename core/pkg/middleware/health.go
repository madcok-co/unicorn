package middleware

import (
	"context"
	"sync"
	"time"

	ucontext "github.com/madcok-co/unicorn/core/pkg/context"
)

// HealthStatus represents the health status of a component
type HealthStatus string

const (
	HealthStatusUp       HealthStatus = "up"
	HealthStatusDown     HealthStatus = "down"
	HealthStatusDegraded HealthStatus = "degraded"
)

// HealthCheckResult represents the result of a health check
type HealthCheckResult struct {
	Status    HealthStatus           `json:"status"`
	Message   string                 `json:"message,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Duration  time.Duration          `json:"duration_ms"`
}

// HealthChecker is a function that performs a health check
type HealthChecker func(ctx context.Context) HealthCheckResult

// HealthConfig defines health check configuration
type HealthConfig struct {
	// Path is the health check endpoint path
	Path string

	// LivenessPath is the liveness probe path (Kubernetes)
	LivenessPath string

	// ReadinessPath is the readiness probe path (Kubernetes)
	ReadinessPath string

	// Checkers is a map of component name to health checker
	Checkers map[string]HealthChecker

	// Timeout for each health check
	Timeout time.Duration

	// CacheDuration caches health results to reduce load
	CacheDuration time.Duration
}

// DefaultHealthConfig returns default health configuration
func DefaultHealthConfig() *HealthConfig {
	return &HealthConfig{
		Path:          "/health",
		LivenessPath:  "/health/live",
		ReadinessPath: "/health/ready",
		Checkers:      make(map[string]HealthChecker),
		Timeout:       5 * time.Second,
		CacheDuration: 0, // No caching by default
	}
}

// HealthHandler creates a health check handler
type HealthHandler struct {
	config *HealthConfig
	mu     sync.RWMutex
	cache  map[string]cachedResult
}

type cachedResult struct {
	result    HealthCheckResult
	expiresAt time.Time
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(config *HealthConfig) *HealthHandler {
	if config == nil {
		config = DefaultHealthConfig()
	}

	if config.Timeout <= 0 {
		config.Timeout = 5 * time.Second
	}

	return &HealthHandler{
		config: config,
		cache:  make(map[string]cachedResult),
	}
}

// AddChecker adds a health checker for a component
func (h *HealthHandler) AddChecker(name string, checker HealthChecker) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.config.Checkers == nil {
		h.config.Checkers = make(map[string]HealthChecker)
	}
	h.config.Checkers[name] = checker
}

// Check performs all health checks
func (h *HealthHandler) Check(ctx context.Context) map[string]HealthCheckResult {
	h.mu.RLock()
	checkers := h.config.Checkers
	h.mu.RUnlock()

	results := make(map[string]HealthCheckResult)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for name, checker := range checkers {
		wg.Add(1)
		go func(name string, checker HealthChecker) {
			defer wg.Done()

			// Check cache first
			if h.config.CacheDuration > 0 {
				h.mu.RLock()
				if cached, ok := h.cache[name]; ok && time.Now().Before(cached.expiresAt) {
					h.mu.RUnlock()
					mu.Lock()
					results[name] = cached.result
					mu.Unlock()
					return
				}
				h.mu.RUnlock()
			}

			// Create timeout context
			checkCtx, cancel := context.WithTimeout(ctx, h.config.Timeout)
			defer cancel()

			// Run check
			start := time.Now()
			result := checker(checkCtx)
			result.Duration = time.Since(start)
			result.Timestamp = time.Now()

			// Cache result
			if h.config.CacheDuration > 0 {
				h.mu.Lock()
				h.cache[name] = cachedResult{
					result:    result,
					expiresAt: time.Now().Add(h.config.CacheDuration),
				}
				h.mu.Unlock()
			}

			mu.Lock()
			results[name] = result
			mu.Unlock()
		}(name, checker)
	}

	wg.Wait()
	return results
}

// IsHealthy returns true if all components are healthy
func (h *HealthHandler) IsHealthy(ctx context.Context) bool {
	results := h.Check(ctx)
	for _, result := range results {
		if result.Status == HealthStatusDown {
			return false
		}
	}
	return true
}

// Handler returns the main health check handler
func (h *HealthHandler) Handler() ucontext.HandlerFunc {
	return func(ctx *ucontext.Context) error {
		results := h.Check(ctx.Context())

		// Determine overall status
		overallStatus := HealthStatusUp
		for _, result := range results {
			if result.Status == HealthStatusDown {
				overallStatus = HealthStatusDown
				break
			}
			if result.Status == HealthStatusDegraded {
				overallStatus = HealthStatusDegraded
			}
		}

		response := map[string]interface{}{
			"status":     overallStatus,
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
			"components": results,
		}

		if overallStatus == HealthStatusUp {
			return ctx.JSON(200, response)
		}
		return ctx.JSON(503, response)
	}
}

// LivenessHandler returns handler for liveness probe (is the app running?)
func (h *HealthHandler) LivenessHandler() ucontext.HandlerFunc {
	return func(ctx *ucontext.Context) error {
		return ctx.JSON(200, map[string]interface{}{
			"status":    HealthStatusUp,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	}
}

// ReadinessHandler returns handler for readiness probe (is the app ready to receive traffic?)
func (h *HealthHandler) ReadinessHandler() ucontext.HandlerFunc {
	return func(ctx *ucontext.Context) error {
		results := h.Check(ctx.Context())

		// Check if any critical component is down
		for _, result := range results {
			if result.Status == HealthStatusDown {
				return ctx.JSON(503, map[string]interface{}{
					"status":     HealthStatusDown,
					"timestamp":  time.Now().UTC().Format(time.RFC3339),
					"components": results,
				})
			}
		}

		return ctx.JSON(200, map[string]interface{}{
			"status":     HealthStatusUp,
			"timestamp":  time.Now().UTC().Format(time.RFC3339),
			"components": results,
		})
	}
}

// ============ Common Health Checkers ============

// DatabaseChecker creates a health checker for database
func DatabaseChecker(pinger interface{ Ping(context.Context) error }) HealthChecker {
	return func(ctx context.Context) HealthCheckResult {
		start := time.Now()
		err := pinger.Ping(ctx)

		if err != nil {
			return HealthCheckResult{
				Status:   HealthStatusDown,
				Message:  err.Error(),
				Duration: time.Since(start),
			}
		}

		return HealthCheckResult{
			Status:   HealthStatusUp,
			Duration: time.Since(start),
		}
	}
}

// CacheChecker creates a health checker for cache
func CacheChecker(pinger interface{ Ping(context.Context) error }) HealthChecker {
	return func(ctx context.Context) HealthCheckResult {
		start := time.Now()
		err := pinger.Ping(ctx)

		if err != nil {
			// Cache down is usually degraded, not down
			return HealthCheckResult{
				Status:   HealthStatusDegraded,
				Message:  err.Error(),
				Duration: time.Since(start),
			}
		}

		return HealthCheckResult{
			Status:   HealthStatusUp,
			Duration: time.Since(start),
		}
	}
}

// MemoryChecker creates a health checker for memory usage
func MemoryChecker(maxPercent float64) HealthChecker {
	return func(ctx context.Context) HealthCheckResult {
		// This is a simplified version
		// In production, use runtime.MemStats

		return HealthCheckResult{
			Status: HealthStatusUp,
			Details: map[string]interface{}{
				"max_percent": maxPercent,
			},
		}
	}
}

// DiskChecker creates a health checker for disk usage
func DiskChecker(path string, maxPercent float64) HealthChecker {
	return func(ctx context.Context) HealthCheckResult {
		// This is a simplified version
		// In production, use syscall or third-party library

		return HealthCheckResult{
			Status: HealthStatusUp,
			Details: map[string]interface{}{
				"path":        path,
				"max_percent": maxPercent,
			},
		}
	}
}

// CustomChecker creates a custom health checker
func CustomChecker(name string, check func() error) HealthChecker {
	return func(ctx context.Context) HealthCheckResult {
		start := time.Now()
		err := check()

		if err != nil {
			return HealthCheckResult{
				Status:   HealthStatusDown,
				Message:  err.Error(),
				Duration: time.Since(start),
			}
		}

		return HealthCheckResult{
			Status:   HealthStatusUp,
			Duration: time.Since(start),
		}
	}
}

// URLChecker creates a health checker that checks a URL
func URLChecker(url string, timeout time.Duration) HealthChecker {
	return func(ctx context.Context) HealthCheckResult {
		// This is a simplified version
		// In production, use net/http client with timeout

		return HealthCheckResult{
			Status: HealthStatusUp,
			Details: map[string]interface{}{
				"url":     url,
				"timeout": timeout.String(),
			},
		}
	}
}
