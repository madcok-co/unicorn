package middleware

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestHealthHandler(t *testing.T) {
	t.Run("returns healthy status with no checkers", func(t *testing.T) {
		handler := NewHealthHandler(nil)

		ctx := newTestContextWithRequest("GET", "/health", nil)

		err := handler.Handler()(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if ctx.Response().StatusCode != 200 {
			t.Errorf("expected status 200, got %d", ctx.Response().StatusCode)
		}
	})

	t.Run("returns healthy status when all checkers pass", func(t *testing.T) {
		handler := NewHealthHandler(nil)

		handler.AddChecker("database", func(ctx context.Context) HealthCheckResult {
			return HealthCheckResult{Status: HealthStatusUp}
		})
		handler.AddChecker("cache", func(ctx context.Context) HealthCheckResult {
			return HealthCheckResult{Status: HealthStatusUp}
		})

		ctx := newTestContextWithRequest("GET", "/health", nil)

		err := handler.Handler()(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if ctx.Response().StatusCode != 200 {
			t.Errorf("expected status 200, got %d", ctx.Response().StatusCode)
		}
	})

	t.Run("returns 503 when any checker is down", func(t *testing.T) {
		handler := NewHealthHandler(nil)

		handler.AddChecker("database", func(ctx context.Context) HealthCheckResult {
			return HealthCheckResult{Status: HealthStatusUp}
		})
		handler.AddChecker("cache", func(ctx context.Context) HealthCheckResult {
			return HealthCheckResult{Status: HealthStatusDown, Message: "connection refused"}
		})

		ctx := newTestContextWithRequest("GET", "/health", nil)

		err := handler.Handler()(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if ctx.Response().StatusCode != 503 {
			t.Errorf("expected status 503, got %d", ctx.Response().StatusCode)
		}
	})

	t.Run("returns degraded status", func(t *testing.T) {
		handler := NewHealthHandler(nil)

		handler.AddChecker("database", func(ctx context.Context) HealthCheckResult {
			return HealthCheckResult{Status: HealthStatusUp}
		})
		handler.AddChecker("cache", func(ctx context.Context) HealthCheckResult {
			return HealthCheckResult{Status: HealthStatusDegraded, Message: "slow response"}
		})

		results := handler.Check(context.Background())

		dbResult := results["database"]
		if dbResult.Status != HealthStatusUp {
			t.Errorf("expected database up, got %v", dbResult.Status)
		}

		cacheResult := results["cache"]
		if cacheResult.Status != HealthStatusDegraded {
			t.Errorf("expected cache degraded, got %v", cacheResult.Status)
		}
	})
}

func TestHealthHandler_Liveness(t *testing.T) {
	t.Run("always returns 200", func(t *testing.T) {
		handler := NewHealthHandler(nil)

		// Add a failing checker
		handler.AddChecker("database", func(ctx context.Context) HealthCheckResult {
			return HealthCheckResult{Status: HealthStatusDown}
		})

		ctx := newTestContextWithRequest("GET", "/health/live", nil)

		err := handler.LivenessHandler()(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Liveness should always return 200 (process is alive)
		if ctx.Response().StatusCode != 200 {
			t.Errorf("expected status 200, got %d", ctx.Response().StatusCode)
		}
	})
}

func TestHealthHandler_Readiness(t *testing.T) {
	t.Run("returns 200 when all checkers pass", func(t *testing.T) {
		handler := NewHealthHandler(nil)

		handler.AddChecker("database", func(ctx context.Context) HealthCheckResult {
			return HealthCheckResult{Status: HealthStatusUp}
		})

		ctx := newTestContextWithRequest("GET", "/health/ready", nil)

		err := handler.ReadinessHandler()(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if ctx.Response().StatusCode != 200 {
			t.Errorf("expected status 200, got %d", ctx.Response().StatusCode)
		}
	})

	t.Run("returns 503 when any checker is down", func(t *testing.T) {
		handler := NewHealthHandler(nil)

		handler.AddChecker("database", func(ctx context.Context) HealthCheckResult {
			return HealthCheckResult{Status: HealthStatusDown, Message: "connection failed"}
		})

		ctx := newTestContextWithRequest("GET", "/health/ready", nil)

		err := handler.ReadinessHandler()(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if ctx.Response().StatusCode != 503 {
			t.Errorf("expected status 503, got %d", ctx.Response().StatusCode)
		}
	})

	t.Run("returns 200 for degraded (not down)", func(t *testing.T) {
		handler := NewHealthHandler(nil)

		handler.AddChecker("cache", func(ctx context.Context) HealthCheckResult {
			return HealthCheckResult{Status: HealthStatusDegraded, Message: "slow"}
		})

		ctx := newTestContextWithRequest("GET", "/health/ready", nil)

		err := handler.ReadinessHandler()(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Degraded is not down, so should return 200
		if ctx.Response().StatusCode != 200 {
			t.Errorf("expected status 200, got %d", ctx.Response().StatusCode)
		}
	})
}

func TestHealthHandler_IsHealthy(t *testing.T) {
	t.Run("returns true when all healthy", func(t *testing.T) {
		handler := NewHealthHandler(nil)

		handler.AddChecker("db", func(ctx context.Context) HealthCheckResult {
			return HealthCheckResult{Status: HealthStatusUp}
		})

		if !handler.IsHealthy(context.Background()) {
			t.Error("expected healthy")
		}
	})

	t.Run("returns false when any is down", func(t *testing.T) {
		handler := NewHealthHandler(nil)

		handler.AddChecker("db", func(ctx context.Context) HealthCheckResult {
			return HealthCheckResult{Status: HealthStatusDown}
		})

		if handler.IsHealthy(context.Background()) {
			t.Error("expected unhealthy")
		}
	})

	t.Run("returns true when degraded (not down)", func(t *testing.T) {
		handler := NewHealthHandler(nil)

		handler.AddChecker("cache", func(ctx context.Context) HealthCheckResult {
			return HealthCheckResult{Status: HealthStatusDegraded}
		})

		if !handler.IsHealthy(context.Background()) {
			t.Error("degraded should still be considered healthy")
		}
	})
}

func TestHealthHandler_Caching(t *testing.T) {
	t.Run("caches results", func(t *testing.T) {
		var callCount int32

		handler := NewHealthHandler(&HealthConfig{
			CacheDuration: 100 * time.Millisecond,
			Timeout:       5 * time.Second,
		})

		handler.AddChecker("db", func(ctx context.Context) HealthCheckResult {
			atomic.AddInt32(&callCount, 1)
			return HealthCheckResult{Status: HealthStatusUp}
		})

		// First call
		handler.Check(context.Background())

		// Second call (should use cache)
		handler.Check(context.Background())

		// Should only have called checker once
		if atomic.LoadInt32(&callCount) != 1 {
			t.Errorf("expected 1 call, got %d", callCount)
		}

		// Wait for cache to expire
		time.Sleep(150 * time.Millisecond)

		// Third call (cache expired)
		handler.Check(context.Background())

		if atomic.LoadInt32(&callCount) != 2 {
			t.Errorf("expected 2 calls, got %d", callCount)
		}
	})
}

func TestHealthHandler_Timeout(t *testing.T) {
	t.Run("times out slow checkers", func(t *testing.T) {
		handler := NewHealthHandler(&HealthConfig{
			Timeout: 50 * time.Millisecond,
		})

		handler.AddChecker("slow", func(ctx context.Context) HealthCheckResult {
			select {
			case <-time.After(200 * time.Millisecond):
				return HealthCheckResult{Status: HealthStatusUp}
			case <-ctx.Done():
				return HealthCheckResult{Status: HealthStatusDown, Message: "timeout"}
			}
		})

		start := time.Now()
		results := handler.Check(context.Background())
		elapsed := time.Since(start)

		// Should complete within timeout + buffer
		if elapsed > 100*time.Millisecond {
			t.Errorf("should have timed out faster, took %v", elapsed)
		}

		if results["slow"].Status != HealthStatusDown {
			t.Errorf("expected down status after timeout, got %v", results["slow"].Status)
		}
	})
}

func TestHealthHandler_Concurrent(t *testing.T) {
	t.Run("runs checkers concurrently", func(t *testing.T) {
		handler := NewHealthHandler(&HealthConfig{
			Timeout: 5 * time.Second,
		})

		// Add multiple slow checkers
		for i := 0; i < 5; i++ {
			name := string(rune('A' + i))
			handler.AddChecker("checker"+name, func(ctx context.Context) HealthCheckResult {
				time.Sleep(50 * time.Millisecond)
				return HealthCheckResult{Status: HealthStatusUp}
			})
		}

		start := time.Now()
		handler.Check(context.Background())
		elapsed := time.Since(start)

		// If run sequentially: 5 * 50ms = 250ms
		// If run concurrently: ~50ms
		if elapsed > 150*time.Millisecond {
			t.Errorf("expected concurrent execution, took %v", elapsed)
		}
	})
}

func TestCommonCheckers(t *testing.T) {
	t.Run("DatabaseChecker success", func(t *testing.T) {
		pinger := &mockPinger{err: nil}
		checker := DatabaseChecker(pinger)

		result := checker(context.Background())

		if result.Status != HealthStatusUp {
			t.Errorf("expected up, got %v", result.Status)
		}
	})

	t.Run("DatabaseChecker failure", func(t *testing.T) {
		pinger := &mockPinger{err: errors.New("connection refused")}
		checker := DatabaseChecker(pinger)

		result := checker(context.Background())

		if result.Status != HealthStatusDown {
			t.Errorf("expected down, got %v", result.Status)
		}
		if result.Message != "connection refused" {
			t.Errorf("expected error message, got %v", result.Message)
		}
	})

	t.Run("CacheChecker success", func(t *testing.T) {
		pinger := &mockPinger{err: nil}
		checker := CacheChecker(pinger)

		result := checker(context.Background())

		if result.Status != HealthStatusUp {
			t.Errorf("expected up, got %v", result.Status)
		}
	})

	t.Run("CacheChecker failure returns degraded", func(t *testing.T) {
		pinger := &mockPinger{err: errors.New("timeout")}
		checker := CacheChecker(pinger)

		result := checker(context.Background())

		// Cache failure is degraded, not down
		if result.Status != HealthStatusDegraded {
			t.Errorf("expected degraded, got %v", result.Status)
		}
	})

	t.Run("CustomChecker success", func(t *testing.T) {
		checker := CustomChecker("custom", func() error {
			return nil
		})

		result := checker(context.Background())

		if result.Status != HealthStatusUp {
			t.Errorf("expected up, got %v", result.Status)
		}
	})

	t.Run("CustomChecker failure", func(t *testing.T) {
		checker := CustomChecker("custom", func() error {
			return errors.New("custom error")
		})

		result := checker(context.Background())

		if result.Status != HealthStatusDown {
			t.Errorf("expected down, got %v", result.Status)
		}
	})

	t.Run("MemoryChecker", func(t *testing.T) {
		checker := MemoryChecker(80.0)

		result := checker(context.Background())

		if result.Status != HealthStatusUp {
			t.Errorf("expected up, got %v", result.Status)
		}
		if result.Details["max_percent"] != 80.0 {
			t.Errorf("expected max_percent 80, got %v", result.Details["max_percent"])
		}
	})

	t.Run("DiskChecker", func(t *testing.T) {
		checker := DiskChecker("/", 90.0)

		result := checker(context.Background())

		if result.Status != HealthStatusUp {
			t.Errorf("expected up, got %v", result.Status)
		}
		if result.Details["path"] != "/" {
			t.Errorf("expected path '/', got %v", result.Details["path"])
		}
	})

	t.Run("URLChecker", func(t *testing.T) {
		checker := URLChecker("http://localhost:8080/health", 5*time.Second)

		result := checker(context.Background())

		if result.Status != HealthStatusUp {
			t.Errorf("expected up, got %v", result.Status)
		}
		if result.Details["url"] != "http://localhost:8080/health" {
			t.Errorf("expected url, got %v", result.Details["url"])
		}
	})
}

func TestDefaultHealthConfig(t *testing.T) {
	config := DefaultHealthConfig()

	if config.Path != "/health" {
		t.Errorf("expected /health, got %v", config.Path)
	}
	if config.LivenessPath != "/health/live" {
		t.Errorf("expected /health/live, got %v", config.LivenessPath)
	}
	if config.ReadinessPath != "/health/ready" {
		t.Errorf("expected /health/ready, got %v", config.ReadinessPath)
	}
	if config.Timeout != 5*time.Second {
		t.Errorf("expected 5s timeout, got %v", config.Timeout)
	}
}

// mockPinger for testing
type mockPinger struct {
	err error
}

func (m *mockPinger) Ping(ctx context.Context) error {
	return m.err
}

func BenchmarkHealthCheck(b *testing.B) {
	handler := NewHealthHandler(nil)

	handler.AddChecker("db", func(ctx context.Context) HealthCheckResult {
		return HealthCheckResult{Status: HealthStatusUp}
	})
	handler.AddChecker("cache", func(ctx context.Context) HealthCheckResult {
		return HealthCheckResult{Status: HealthStatusUp}
	})

	ctx := newTestContextWithRequest("GET", "/health", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		handler.Handler()(ctx)
	}
}

func BenchmarkHealthCheckWithCache(b *testing.B) {
	handler := NewHealthHandler(&HealthConfig{
		CacheDuration: time.Minute,
		Timeout:       5 * time.Second,
	})

	handler.AddChecker("db", func(ctx context.Context) HealthCheckResult {
		return HealthCheckResult{Status: HealthStatusUp}
	})

	ctx := newTestContextWithRequest("GET", "/health", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		handler.Handler()(ctx)
	}
}
