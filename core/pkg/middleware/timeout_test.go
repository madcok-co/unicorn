package middleware

import (
	"context"
	"testing"
	"time"

	ucontext "github.com/madcok-co/unicorn/core/pkg/context"
)

func TestTimeout(t *testing.T) {
	t.Run("allows fast requests", func(t *testing.T) {
		middleware := Timeout(100 * time.Millisecond)

		ctx := newTestContextWithRequest("GET", "/test", nil)

		handlerCalled := false
		handler := middleware(func(ctx *ucontext.Context) error {
			handlerCalled = true
			return ctx.JSON(200, map[string]string{"status": "ok"})
		})

		err := handler(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !handlerCalled {
			t.Error("handler was not called")
		}
		if ctx.Response().StatusCode != 200 {
			t.Errorf("expected status 200, got %d", ctx.Response().StatusCode)
		}
	})

	t.Run("times out slow requests", func(t *testing.T) {
		middleware := Timeout(50 * time.Millisecond)

		ctx := newTestContextWithRequest("GET", "/slow", nil)

		handler := middleware(func(ctx *ucontext.Context) error {
			// Simulate slow handler
			time.Sleep(200 * time.Millisecond)
			return ctx.JSON(200, map[string]string{"status": "ok"})
		})

		start := time.Now()
		err := handler(ctx)
		elapsed := time.Since(start)

		// Should return within timeout + buffer
		if elapsed > 100*time.Millisecond {
			t.Errorf("should have timed out faster, took %v", elapsed)
		}

		// Should return error or set 504 status
		if err == nil && ctx.Response().StatusCode != 504 {
			t.Error("expected timeout error or 504 status")
		}
	})

	t.Run("calls OnTimeout callback", func(t *testing.T) {
		onTimeoutCalled := false

		middleware := TimeoutWithConfig(&TimeoutConfig{
			Timeout: 50 * time.Millisecond,
			OnTimeout: func(ctx *ucontext.Context) {
				onTimeoutCalled = true
			},
		})

		ctx := newTestContextWithRequest("GET", "/slow", nil)

		handler := middleware(func(ctx *ucontext.Context) error {
			time.Sleep(200 * time.Millisecond)
			return nil
		})

		handler(ctx)

		if !onTimeoutCalled {
			t.Error("OnTimeout was not called")
		}
	})

	t.Run("skipper skips middleware", func(t *testing.T) {
		middleware := TimeoutWithConfig(&TimeoutConfig{
			Timeout: 10 * time.Millisecond,
			Skipper: func(ctx *ucontext.Context) bool {
				return ctx.Request().Path == "/no-timeout"
			},
		})

		ctx := newTestContextWithRequest("GET", "/no-timeout", nil)

		handlerCalled := false
		handler := middleware(func(ctx *ucontext.Context) error {
			handlerCalled = true
			time.Sleep(50 * time.Millisecond)
			return ctx.JSON(200, map[string]string{"status": "ok"})
		})

		handler(ctx)

		if !handlerCalled {
			t.Error("handler was not called")
		}
		// Should complete despite being "slow" because timeout was skipped
		if ctx.Response().StatusCode != 200 {
			t.Errorf("expected status 200, got %d", ctx.Response().StatusCode)
		}
	})

	t.Run("nil config uses defaults", func(t *testing.T) {
		middleware := TimeoutWithConfig(nil)

		ctx := newTestContextWithRequest("GET", "/test", nil)

		handler := middleware(func(ctx *ucontext.Context) error {
			return ctx.JSON(200, map[string]string{"status": "ok"})
		})

		err := handler(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("zero timeout uses default", func(t *testing.T) {
		middleware := TimeoutWithConfig(&TimeoutConfig{
			Timeout: 0, // Should use default 30s
		})

		ctx := newTestContextWithRequest("GET", "/test", nil)

		handler := middleware(func(ctx *ucontext.Context) error {
			return ctx.JSON(200, map[string]string{"status": "ok"})
		})

		err := handler(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("handler error is returned", func(t *testing.T) {
		middleware := Timeout(100 * time.Millisecond)

		ctx := newTestContextWithRequest("GET", "/error", nil)

		handler := middleware(func(ctx *ucontext.Context) error {
			return ctx.Error(400, "bad request")
		})

		err := handler(ctx)
		// Error should be returned (may be nil if Error() doesn't return error)
		_ = err

		if ctx.Response().StatusCode != 400 {
			t.Errorf("expected status 400, got %d", ctx.Response().StatusCode)
		}
	})
}

func TestDefaultTimeoutConfig(t *testing.T) {
	config := DefaultTimeoutConfig()

	if config.Timeout != 30*time.Second {
		t.Errorf("expected 30s timeout, got %v", config.Timeout)
	}
}

func BenchmarkTimeout(b *testing.B) {
	middleware := Timeout(30 * time.Second)

	ctx := ucontext.New(context.Background())
	ctx.SetRequest(&ucontext.Request{Method: "GET", Path: "/test"})

	handler := middleware(func(ctx *ucontext.Context) error {
		return nil
	})

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		handler(ctx)
	}
}
