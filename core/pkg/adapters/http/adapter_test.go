package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	ucontext "github.com/madcok-co/unicorn/core/pkg/context"
	"github.com/madcok-co/unicorn/core/pkg/contracts"
	"github.com/madcok-co/unicorn/core/pkg/handler"
)

func TestAdapter_New(t *testing.T) {
	registry := handler.NewRegistry()

	t.Run("with nil config uses defaults", func(t *testing.T) {
		adapter := New(registry, nil)

		if adapter.config.Port != 8080 {
			t.Errorf("expected port 8080, got %d", adapter.config.Port)
		}
		if adapter.config.Host != "0.0.0.0" {
			t.Errorf("expected host 0.0.0.0, got %s", adapter.config.Host)
		}
	})

	t.Run("with custom config", func(t *testing.T) {
		config := &Config{
			Host: "localhost",
			Port: 3000,
		}
		adapter := New(registry, config)

		if adapter.config.Port != 3000 {
			t.Errorf("expected port 3000, got %d", adapter.config.Port)
		}
	})
}

func TestAdapter_Use(t *testing.T) {
	registry := handler.NewRegistry()
	adapter := New(registry, nil)

	middleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}

	adapter.Use(middleware)

	if len(adapter.middlewares) != 1 {
		t.Error("middleware should be added")
	}
}

func TestAdapter_Address(t *testing.T) {
	registry := handler.NewRegistry()
	adapter := New(registry, &Config{
		Host: "localhost",
		Port: 9000,
	})

	if adapter.Address() != "localhost:9000" {
		t.Errorf("expected localhost:9000, got %s", adapter.Address())
	}
}

func TestAdapter_TLS(t *testing.T) {
	registry := handler.NewRegistry()

	t.Run("without TLS", func(t *testing.T) {
		adapter := New(registry, nil)

		if adapter.IsTLS() {
			t.Error("should not be TLS")
		}
		if adapter.Scheme() != "http" {
			t.Errorf("expected http, got %s", adapter.Scheme())
		}
	})

	t.Run("with TLS", func(t *testing.T) {
		adapter := New(registry, &Config{
			TLS: &contracts.TLSConfig{
				Enabled: true,
			},
		})

		if !adapter.IsTLS() {
			t.Error("should be TLS")
		}
		if adapter.Scheme() != "https" {
			t.Errorf("expected https, got %s", adapter.Scheme())
		}
	})
}

func TestHTTPError(t *testing.T) {
	err := NewHTTPError(404, "not found")

	if err.StatusCode != 404 {
		t.Errorf("expected 404, got %d", err.StatusCode)
	}
	if err.Message != "not found" {
		t.Errorf("expected 'not found', got %s", err.Message)
	}
	if err.Error() != "not found" {
		t.Error("Error() should return message")
	}
}

func TestDefaultParamExtractor(t *testing.T) {
	tests := []struct {
		pattern  string
		path     string
		expected map[string]string
	}{
		{
			pattern:  "/users/:id",
			path:     "/users/123",
			expected: map[string]string{"id": "123"},
		},
		{
			pattern:  "/users/{id}",
			path:     "/users/456",
			expected: map[string]string{"id": "456"},
		},
		{
			pattern:  "/users/:userId/posts/:postId",
			path:     "/users/1/posts/2",
			expected: map[string]string{"userId": "1", "postId": "2"},
		},
		{
			pattern:  "/static/path",
			path:     "/static/path",
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			result := defaultParamExtractor(tt.pattern, tt.path)

			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("expected %s=%s, got %s", k, v, result[k])
				}
			}
		})
	}
}

func TestAdapter_WriteError(t *testing.T) {
	registry := handler.NewRegistry()
	adapter := New(registry, nil)

	t.Run("generic error returns 500", func(t *testing.T) {
		w := httptest.NewRecorder()
		adapter.writeError(w, errors.New("some internal error"))

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", w.Code)
		}

		var resp map[string]string
		json.Unmarshal(w.Body.Bytes(), &resp)

		// Should not expose internal error message
		if resp["error"] != "Internal server error" {
			t.Errorf("should return generic message, got %s", resp["error"])
		}
	})

	t.Run("HTTPError returns specific status", func(t *testing.T) {
		w := httptest.NewRecorder()
		adapter.writeError(w, NewHTTPError(400, "validation failed"))

		if w.Code != 400 {
			t.Errorf("expected 400, got %d", w.Code)
		}

		var resp map[string]string
		json.Unmarshal(w.Body.Bytes(), &resp)

		if resp["error"] != "validation failed" {
			t.Errorf("expected 'validation failed', got %s", resp["error"])
		}
	})
}

func TestAdapter_WriteResponse(t *testing.T) {
	registry := handler.NewRegistry()
	adapter := New(registry, nil)

	t.Run("writes JSON response", func(t *testing.T) {
		w := httptest.NewRecorder()
		resp := &ucontext.Response{
			StatusCode: 200,
			Body:       map[string]string{"message": "success"},
			Headers:    map[string]string{"X-Custom": "header"},
		}

		adapter.writeResponse(w, resp)

		if w.Code != 200 {
			t.Errorf("expected 200, got %d", w.Code)
		}
		if w.Header().Get("X-Custom") != "header" {
			t.Error("should set custom header")
		}
		if w.Header().Get("Content-Type") != "application/json" {
			t.Error("should set content type")
		}
	})

	t.Run("handles empty body", func(t *testing.T) {
		w := httptest.NewRecorder()
		resp := &ucontext.Response{
			StatusCode: 204,
			Body:       nil,
			Headers:    map[string]string{},
		}

		adapter.writeResponse(w, resp)

		if w.Code != 204 {
			t.Errorf("expected 204, got %d", w.Code)
		}
	})

	t.Run("defaults to 200 if no status", func(t *testing.T) {
		w := httptest.NewRecorder()
		resp := &ucontext.Response{
			StatusCode: 0,
			Body:       "test",
			Headers:    map[string]string{},
		}

		adapter.writeResponse(w, resp)

		if w.Code != 200 {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})
}

func TestAdapter_Shutdown(t *testing.T) {
	registry := handler.NewRegistry()
	adapter := New(registry, nil)

	// Shutdown without starting should not error
	err := adapter.Shutdown(context.Background())
	if err != nil {
		t.Errorf("shutdown without server should not error: %v", err)
	}
}

func BenchmarkAdapter_WriteResponse(b *testing.B) {
	registry := handler.NewRegistry()
	adapter := New(registry, nil)

	resp := &ucontext.Response{
		StatusCode: 200,
		Body:       map[string]string{"message": "benchmark"},
		Headers:    map[string]string{},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		adapter.writeResponse(w, resp)
	}
}

func BenchmarkDefaultParamExtractor(b *testing.B) {
	pattern := "/users/:userId/posts/:postId"
	path := "/users/123/posts/456"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		defaultParamExtractor(pattern, path)
	}
}
