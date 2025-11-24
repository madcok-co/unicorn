package middleware

import (
	"context"
	"net/http"
	"testing"

	ucontext "github.com/madcok-co/unicorn/core/pkg/context"
)

func TestCORS(t *testing.T) {
	t.Run("allows requests without origin", func(t *testing.T) {
		middleware := CORS()

		ctx := newTestContextWithRequest("GET", "/test", map[string]string{})

		handlerCalled := false
		handler := middleware(func(ctx *ucontext.Context) error {
			handlerCalled = true
			return nil
		})

		err := handler(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !handlerCalled {
			t.Error("handler was not called")
		}
	})

	t.Run("sets CORS headers for allowed origin", func(t *testing.T) {
		middleware := CORSWithConfig(&CORSConfig{
			AllowOrigins: []string{"https://example.com"},
		})

		ctx := newTestContextWithRequest("GET", "/test", map[string]string{"Origin": "https://example.com"})

		handler := middleware(func(ctx *ucontext.Context) error {
			return nil
		})

		handler(ctx)

		if ctx.Response().Headers["Access-Control-Allow-Origin"] != "https://example.com" {
			t.Errorf("expected origin header, got %v", ctx.Response().Headers["Access-Control-Allow-Origin"])
		}
	})

	t.Run("handles wildcard origin", func(t *testing.T) {
		middleware := CORSWithConfig(&CORSConfig{
			AllowOrigins: []string{"*"},
		})

		ctx := newTestContextWithRequest("GET", "/test", map[string]string{"Origin": "https://any-origin.com"})

		handler := middleware(func(ctx *ucontext.Context) error {
			return nil
		})

		handler(ctx)

		if ctx.Response().Headers["Access-Control-Allow-Origin"] != "*" {
			t.Errorf("expected wildcard origin, got %v", ctx.Response().Headers["Access-Control-Allow-Origin"])
		}
	})

	t.Run("handles wildcard with credentials", func(t *testing.T) {
		middleware := CORSWithConfig(&CORSConfig{
			AllowOrigins:     []string{"*"},
			AllowCredentials: true,
		})

		ctx := newTestContextWithRequest("GET", "/test", map[string]string{"Origin": "https://example.com"})

		handler := middleware(func(ctx *ucontext.Context) error {
			return nil
		})

		handler(ctx)

		// With credentials, should echo origin instead of wildcard
		if ctx.Response().Headers["Access-Control-Allow-Origin"] != "https://example.com" {
			t.Errorf("expected echoed origin with credentials, got %v", ctx.Response().Headers["Access-Control-Allow-Origin"])
		}
		if ctx.Response().Headers["Access-Control-Allow-Credentials"] != "true" {
			t.Error("expected credentials header")
		}
	})

	t.Run("rejects disallowed origin", func(t *testing.T) {
		middleware := CORSWithConfig(&CORSConfig{
			AllowOrigins: []string{"https://allowed.com"},
		})

		ctx := newTestContextWithRequest("GET", "/test", map[string]string{"Origin": "https://disallowed.com"})

		handlerCalled := false
		handler := middleware(func(ctx *ucontext.Context) error {
			handlerCalled = true
			return nil
		})

		handler(ctx)

		// Handler should still be called
		if !handlerCalled {
			t.Error("handler was not called")
		}

		// But CORS headers should not be set
		if ctx.Response().Headers["Access-Control-Allow-Origin"] != "" {
			t.Error("CORS headers should not be set for disallowed origin")
		}
	})

	t.Run("handles preflight OPTIONS request", func(t *testing.T) {
		middleware := CORSWithConfig(&CORSConfig{
			AllowOrigins: []string{"https://example.com"},
			AllowMethods: []string{"GET", "POST", "PUT"},
			AllowHeaders: []string{"Authorization", "Content-Type"},
			MaxAge:       3600,
		})

		ctx := newTestContextWithRequest(http.MethodOptions, "/test", map[string]string{"Origin": "https://example.com"})

		handlerCalled := false
		handler := middleware(func(ctx *ucontext.Context) error {
			handlerCalled = true
			return nil
		})

		err := handler(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Handler should NOT be called for preflight
		if handlerCalled {
			t.Error("handler should not be called for preflight")
		}

		// Check preflight headers
		if ctx.Response().Headers["Access-Control-Allow-Methods"] != "GET, POST, PUT" {
			t.Errorf("expected methods header, got %v", ctx.Response().Headers["Access-Control-Allow-Methods"])
		}
		if ctx.Response().Headers["Access-Control-Allow-Headers"] != "Authorization, Content-Type" {
			t.Errorf("expected headers header, got %v", ctx.Response().Headers["Access-Control-Allow-Headers"])
		}
		if ctx.Response().Headers["Access-Control-Max-Age"] != "3600" {
			t.Errorf("expected max-age header, got %v", ctx.Response().Headers["Access-Control-Max-Age"])
		}
		if ctx.Response().StatusCode != http.StatusNoContent {
			t.Errorf("expected 204 status, got %d", ctx.Response().StatusCode)
		}
	})

	t.Run("exposes headers", func(t *testing.T) {
		middleware := CORSWithConfig(&CORSConfig{
			AllowOrigins:  []string{"*"},
			ExposeHeaders: []string{"X-Custom-Header", "X-Another-Header"},
		})

		ctx := newTestContextWithRequest("GET", "/test", map[string]string{"Origin": "https://example.com"})

		handler := middleware(func(ctx *ucontext.Context) error {
			return nil
		})

		handler(ctx)

		if ctx.Response().Headers["Access-Control-Expose-Headers"] != "X-Custom-Header, X-Another-Header" {
			t.Errorf("expected expose headers, got %v", ctx.Response().Headers["Access-Control-Expose-Headers"])
		}
	})

	t.Run("uses custom AllowOriginFunc", func(t *testing.T) {
		middleware := CORSWithConfig(&CORSConfig{
			AllowOriginFunc: func(origin string) bool {
				// Only allow .example.com subdomains
				return len(origin) > 12 && origin[len(origin)-12:] == ".example.com"
			},
		})

		// Test allowed subdomain
		ctx := newTestContextWithRequest("GET", "/test", map[string]string{"Origin": "https://api.example.com"})

		handler := middleware(func(ctx *ucontext.Context) error {
			return nil
		})

		handler(ctx)

		if ctx.Response().Headers["Access-Control-Allow-Origin"] != "https://api.example.com" {
			t.Errorf("expected allowed subdomain, got %v", ctx.Response().Headers["Access-Control-Allow-Origin"])
		}

		// Test disallowed origin
		ctx2 := newTestContextWithRequest("GET", "/test", map[string]string{"Origin": "https://other.com"})

		handler2 := middleware(func(ctx *ucontext.Context) error {
			return nil
		})

		handler2(ctx2)

		if ctx2.Response().Headers["Access-Control-Allow-Origin"] != "" {
			t.Error("should not allow other.com")
		}
	})

	t.Run("skipper skips middleware", func(t *testing.T) {
		middleware := CORSWithConfig(&CORSConfig{
			AllowOrigins: []string{"https://example.com"},
			Skipper: func(ctx *corsContext) bool {
				return ctx.Request().Path == "/skip"
			},
		})

		ctx := newTestContextWithRequest("GET", "/skip", map[string]string{"Origin": "https://example.com"})

		handlerCalled := false
		handler := middleware(func(ctx *ucontext.Context) error {
			handlerCalled = true
			return nil
		})

		handler(ctx)

		if !handlerCalled {
			t.Error("handler was not called")
		}
		// CORS headers should not be set when skipped
		if ctx.Response().Headers["Access-Control-Allow-Origin"] != "" {
			t.Error("CORS headers should not be set when skipped")
		}
	})

	t.Run("nil config uses defaults", func(t *testing.T) {
		middleware := CORSWithConfig(nil)

		ctx := newTestContextWithRequest("GET", "/test", map[string]string{"Origin": "https://example.com"})

		handler := middleware(func(ctx *ucontext.Context) error {
			return nil
		})

		handler(ctx)

		// Default allows all origins
		if ctx.Response().Headers["Access-Control-Allow-Origin"] != "*" {
			t.Errorf("expected wildcard origin with default config, got %v", ctx.Response().Headers["Access-Control-Allow-Origin"])
		}
	})
}

func TestCORSAllowAll(t *testing.T) {
	middleware := CORSAllowAll()

	ctx := newTestContextWithRequest("GET", "/test", map[string]string{"Origin": "https://any-origin.com"})

	handler := middleware(func(ctx *ucontext.Context) error {
		return nil
	})

	handler(ctx)

	if ctx.Response().Headers["Access-Control-Allow-Origin"] != "*" {
		t.Errorf("expected wildcard, got %v", ctx.Response().Headers["Access-Control-Allow-Origin"])
	}
}

func TestDefaultCORSConfig(t *testing.T) {
	config := DefaultCORSConfig()

	if len(config.AllowOrigins) != 1 || config.AllowOrigins[0] != "*" {
		t.Error("expected wildcard origin by default")
	}

	if len(config.AllowMethods) != 6 {
		t.Errorf("expected 6 methods, got %d", len(config.AllowMethods))
	}

	if config.AllowCredentials {
		t.Error("credentials should be false by default")
	}

	if config.MaxAge != 86400 {
		t.Errorf("expected MaxAge 86400, got %d", config.MaxAge)
	}
}

func BenchmarkCORS(b *testing.B) {
	middleware := CORS()

	ctx := ucontext.New(context.Background())
	ctx.SetRequest(&ucontext.Request{
		Method:  "GET",
		Path:    "/test",
		Headers: map[string]string{"Origin": "https://example.com"},
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

func BenchmarkCORSPreflight(b *testing.B) {
	middleware := CORSWithConfig(&CORSConfig{
		AllowOrigins: []string{"https://example.com"},
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE"},
		AllowHeaders: []string{"Authorization", "Content-Type"},
		MaxAge:       86400,
	})

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx := ucontext.New(context.Background())
		ctx.SetRequest(&ucontext.Request{
			Method:  http.MethodOptions,
			Path:    "/test",
			Headers: map[string]string{"Origin": "https://example.com"},
		})

		handler := middleware(func(ctx *ucontext.Context) error {
			return nil
		})
		handler(ctx)
	}
}
