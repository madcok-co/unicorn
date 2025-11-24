package middleware

import (
	"context"
	"sync"
	"testing"
	"time"

	ucontext "github.com/madcok-co/unicorn/core/pkg/context"
)

func TestRateLimit(t *testing.T) {
	t.Run("allows requests within limit", func(t *testing.T) {
		middleware := RateLimit(5, time.Minute)

		for i := 0; i < 5; i++ {
			ctx := newTestContextWithRequest("GET", "/test", map[string]string{"X-Forwarded-For": "192.168.1.1"})

			handlerCalled := false
			handler := middleware(func(ctx *ucontext.Context) error {
				handlerCalled = true
				return nil
			})

			handler(ctx)

			if !handlerCalled {
				t.Errorf("request %d should be allowed", i+1)
			}
		}
	})

	t.Run("blocks requests exceeding limit", func(t *testing.T) {
		middleware := RateLimit(3, time.Minute)

		// Make 3 allowed requests
		for i := 0; i < 3; i++ {
			ctx := newTestContextWithRequest("GET", "/test", map[string]string{"X-Forwarded-For": "10.0.0.1"})

			handler := middleware(func(ctx *ucontext.Context) error {
				return nil
			})

			handler(ctx)
		}

		// 4th request should be blocked
		ctx := newTestContextWithRequest("GET", "/test", map[string]string{"X-Forwarded-For": "10.0.0.1"})

		handlerCalled := false
		handler := middleware(func(ctx *ucontext.Context) error {
			handlerCalled = true
			return nil
		})

		handler(ctx)

		if handlerCalled {
			t.Error("4th request should be blocked")
		}

		if ctx.Response().StatusCode != 429 {
			t.Errorf("expected status 429, got %d", ctx.Response().StatusCode)
		}
	})

	t.Run("sets rate limit headers", func(t *testing.T) {
		middleware := RateLimit(10, time.Minute)

		ctx := newTestContextWithRequest("GET", "/test", map[string]string{"X-Forwarded-For": "172.16.0.1"})

		handler := middleware(func(ctx *ucontext.Context) error {
			return nil
		})

		handler(ctx)

		if ctx.Response().Headers["X-RateLimit-Limit"] != "10" {
			t.Errorf("expected limit header 10, got %v", ctx.Response().Headers["X-RateLimit-Limit"])
		}
		if ctx.Response().Headers["X-RateLimit-Remaining"] != "9" {
			t.Errorf("expected remaining 9, got %v", ctx.Response().Headers["X-RateLimit-Remaining"])
		}
	})

	t.Run("different keys have separate limits", func(t *testing.T) {
		middleware := RateLimit(2, time.Minute)

		// Client 1 - 2 requests
		for i := 0; i < 2; i++ {
			ctx := newTestContextWithRequest("GET", "/test", map[string]string{"X-Forwarded-For": "client1"})

			handler := middleware(func(ctx *ucontext.Context) error {
				return nil
			})
			handler(ctx)
		}

		// Client 2 should still have full quota
		ctx := newTestContextWithRequest("GET", "/test", map[string]string{"X-Forwarded-For": "client2"})

		handlerCalled := false
		handler := middleware(func(ctx *ucontext.Context) error {
			handlerCalled = true
			return nil
		})

		handler(ctx)

		if !handlerCalled {
			t.Error("client2 should have full quota")
		}
	})

	t.Run("empty key bypasses rate limiting", func(t *testing.T) {
		config := DefaultRateLimitConfig()
		config.Limit = 1
		config.Window = time.Minute
		config.Store = NewMemoryRateLimitStore()
		config.KeyFunc = func(ctx *ucontext.Context) string {
			return "" // Empty key
		}
		middleware := RateLimitWithConfig(config)

		// Should allow unlimited requests when key is empty
		for i := 0; i < 10; i++ {
			ctx := newTestContextWithRequest("GET", "/test", nil)

			handlerCalled := false
			handler := middleware(func(ctx *ucontext.Context) error {
				handlerCalled = true
				return nil
			})

			handler(ctx)

			if !handlerCalled {
				t.Errorf("request %d should be allowed with empty key", i+1)
			}
		}
	})

	t.Run("skipper skips rate limiting", func(t *testing.T) {
		config := DefaultRateLimitConfig()
		config.Limit = 1
		config.Window = time.Minute
		config.Store = NewMemoryRateLimitStore()
		config.Skipper = func(ctx *ucontext.Context) bool {
			return ctx.Request().Path == "/health"
		}
		middleware := RateLimitWithConfig(config)

		// Health endpoint should bypass rate limiting
		for i := 0; i < 5; i++ {
			ctx := newTestContextWithRequest("GET", "/health", map[string]string{"X-Forwarded-For": "skip-test"})

			handlerCalled := false
			handler := middleware(func(ctx *ucontext.Context) error {
				handlerCalled = true
				return nil
			})

			handler(ctx)

			if !handlerCalled {
				t.Errorf("health check %d should bypass rate limiting", i+1)
			}
		}
	})

	t.Run("calls ExceedHandler on limit exceeded", func(t *testing.T) {
		exceedCalled := false
		exceedKey := ""

		config := DefaultRateLimitConfig()
		config.Limit = 1
		config.Window = time.Minute
		config.Store = NewMemoryRateLimitStore()
		config.ExceedHandler = func(ctx *ucontext.Context, key string) {
			exceedCalled = true
			exceedKey = key
		}
		middleware := RateLimitWithConfig(config)

		// First request
		ctx1 := newTestContextWithRequest("GET", "/test", map[string]string{"X-Forwarded-For": "exceed-test"})

		handler := middleware(func(ctx *ucontext.Context) error {
			return nil
		})
		handler(ctx1)

		// Second request - should exceed
		ctx2 := newTestContextWithRequest("GET", "/test", map[string]string{"X-Forwarded-For": "exceed-test"})

		handler2 := middleware(func(ctx *ucontext.Context) error {
			return nil
		})
		handler2(ctx2)

		if !exceedCalled {
			t.Error("ExceedHandler should be called")
		}
		if exceedKey != "exceed-test" {
			t.Errorf("expected key 'exceed-test', got %v", exceedKey)
		}
	})

	t.Run("nil config uses defaults", func(t *testing.T) {
		middleware := RateLimitWithConfig(nil)

		ctx := newTestContextWithRequest("GET", "/test", map[string]string{"X-Forwarded-For": "nil-config-test"})

		handlerCalled := false
		handler := middleware(func(ctx *ucontext.Context) error {
			handlerCalled = true
			return nil
		})

		handler(ctx)

		if !handlerCalled {
			t.Error("handler should be called")
		}
	})
}

func TestMemoryRateLimitStore(t *testing.T) {
	t.Run("allows requests within limit", func(t *testing.T) {
		store := NewMemoryRateLimitStore()
		defer store.Close()

		for i := 0; i < 5; i++ {
			allowed, remaining, _, err := store.Allow(nil, "test-key", 5, time.Minute)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !allowed {
				t.Errorf("request %d should be allowed", i+1)
			}
			expectedRemaining := 5 - i - 1
			if remaining != expectedRemaining {
				t.Errorf("expected remaining %d, got %d", expectedRemaining, remaining)
			}
		}
	})

	t.Run("blocks after limit exceeded", func(t *testing.T) {
		store := NewMemoryRateLimitStore()
		defer store.Close()

		// Use up the limit
		for i := 0; i < 3; i++ {
			store.Allow(nil, "block-key", 3, time.Minute)
		}

		// Next request should be blocked
		allowed, remaining, retryAfter, err := store.Allow(nil, "block-key", 3, time.Minute)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if allowed {
			t.Error("request should be blocked")
		}
		if remaining != 0 {
			t.Errorf("expected remaining 0, got %d", remaining)
		}
		if retryAfter <= 0 {
			t.Error("expected positive retry-after")
		}
	})

	t.Run("resets after window expires", func(t *testing.T) {
		store := NewMemoryRateLimitStore()
		defer store.Close()

		// Use up the limit with short window
		for i := 0; i < 2; i++ {
			store.Allow(nil, "expire-key", 2, 50*time.Millisecond)
		}

		// Should be blocked
		allowed, _, _, _ := store.Allow(nil, "expire-key", 2, 50*time.Millisecond)
		if allowed {
			t.Error("should be blocked")
		}

		// Wait for window to expire
		time.Sleep(100 * time.Millisecond)

		// Should be allowed again
		allowed, _, _, _ = store.Allow(nil, "expire-key", 2, 50*time.Millisecond)
		if !allowed {
			t.Error("should be allowed after window expires")
		}
	})

	t.Run("reset clears key", func(t *testing.T) {
		store := NewMemoryRateLimitStore()
		defer store.Close()

		// Use some of the limit
		store.Allow(nil, "reset-key", 5, time.Minute)
		store.Allow(nil, "reset-key", 5, time.Minute)

		// Reset
		err := store.Reset(nil, "reset-key")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Should have full quota again
		allowed, remaining, _, _ := store.Allow(nil, "reset-key", 5, time.Minute)
		if !allowed {
			t.Error("should be allowed after reset")
		}
		if remaining != 4 {
			t.Errorf("expected remaining 4, got %d", remaining)
		}
	})

	t.Run("concurrent access is safe", func(t *testing.T) {
		store := NewMemoryRateLimitStore()
		defer store.Close()

		var wg sync.WaitGroup
		allowedCount := 0
		var mu sync.Mutex

		// Concurrent requests
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				allowed, _, _, _ := store.Allow(nil, "concurrent-key", 50, time.Minute)
				if allowed {
					mu.Lock()
					allowedCount++
					mu.Unlock()
				}
			}()
		}

		wg.Wait()

		// Should allow exactly 50 requests
		if allowedCount != 50 {
			t.Errorf("expected 50 allowed, got %d", allowedCount)
		}
	})
}

func TestRateLimitByIP(t *testing.T) {
	middleware := RateLimitByIP(2, time.Minute)

	// Two requests from same IP
	for i := 0; i < 2; i++ {
		ctx := newTestContextWithRequest("GET", "/test", map[string]string{"X-Forwarded-For": "1.2.3.4"})

		handler := middleware(func(ctx *ucontext.Context) error {
			return nil
		})
		handler(ctx)
	}

	// Third should be blocked
	ctx := newTestContextWithRequest("GET", "/test", map[string]string{"X-Forwarded-For": "1.2.3.4"})

	handlerCalled := false
	handler := middleware(func(ctx *ucontext.Context) error {
		handlerCalled = true
		return nil
	})
	handler(ctx)

	if handlerCalled {
		t.Error("third request should be blocked")
	}
}

func TestRateLimitByUserID(t *testing.T) {
	middleware := RateLimitByUserID(2, time.Minute, "user")

	// Two requests from same user
	for i := 0; i < 2; i++ {
		ctx := newTestContextWithRequest("GET", "/test", map[string]string{"X-Forwarded-For": "different-ip"})
		ctx.Set("user", map[string]interface{}{"id": "user123"})

		handler := middleware(func(ctx *ucontext.Context) error {
			return nil
		})
		handler(ctx)
	}

	// Third should be blocked (same user, different IP)
	ctx := newTestContextWithRequest("GET", "/test", map[string]string{"X-Forwarded-For": "yet-another-ip"})
	ctx.Set("user", map[string]interface{}{"id": "user123"})

	handlerCalled := false
	handler := middleware(func(ctx *ucontext.Context) error {
		handlerCalled = true
		return nil
	})
	handler(ctx)

	if handlerCalled {
		t.Error("third request should be blocked for same user")
	}
}

func TestHelperFunctions(t *testing.T) {
	t.Run("itoa single digit", func(t *testing.T) {
		result := itoa(5)
		if result != "5" {
			t.Errorf("expected '5', got '%s'", result)
		}
	})

	t.Run("itoa multi digit", func(t *testing.T) {
		result := itoa(42)
		if result != "42" {
			t.Errorf("expected '42', got '%s'", result)
		}
	})

	t.Run("intToString", func(t *testing.T) {
		tests := []struct {
			input    int
			expected string
		}{
			{0, "0"},
			{1, "1"},
			{10, "10"},
			{100, "100"},
			{12345, "12345"},
		}

		for _, tt := range tests {
			result := intToString(tt.input)
			if result != tt.expected {
				t.Errorf("intToString(%d) = '%s', want '%s'", tt.input, result, tt.expected)
			}
		}
	})
}

func BenchmarkRateLimit(b *testing.B) {
	store := NewMemoryRateLimitStore()
	defer store.Close()

	config := DefaultRateLimitConfig()
	config.Limit = 1000000 // High limit to avoid blocking
	config.Window = time.Minute
	config.Store = store
	middleware := RateLimitWithConfig(config)

	ctx := ucontext.New(context.Background())
	ctx.SetRequest(&ucontext.Request{
		Method:  "GET",
		Path:    "/test",
		Headers: map[string]string{"X-Forwarded-For": "bench-ip"},
	})

	handler := middleware(func(ctx *ucontext.Context) error {
		return nil
	})

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		handler(ctx)
	}
}

func BenchmarkMemoryRateLimitStore(b *testing.B) {
	store := NewMemoryRateLimitStore()
	defer store.Close()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		store.Allow(nil, "bench-key", 1000000, time.Minute)
	}
}
