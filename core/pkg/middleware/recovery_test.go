package middleware

import (
	"context"
	"errors"
	"sync"
	"testing"

	ucontext "github.com/madcok-co/unicorn/core/pkg/context"
	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

// mockLogger for testing
type mockLogger struct {
	mu       sync.Mutex
	messages []map[string]interface{}
}

func (m *mockLogger) Debug(msg string, args ...interface{}) {
	m.log("debug", msg, args...)
}
func (m *mockLogger) Info(msg string, args ...interface{}) {
	m.log("info", msg, args...)
}
func (m *mockLogger) Warn(msg string, args ...interface{}) {
	m.log("warn", msg, args...)
}
func (m *mockLogger) Error(msg string, args ...interface{}) {
	m.log("error", msg, args...)
}
func (m *mockLogger) Fatal(msg string, args ...interface{}) {
	m.log("fatal", msg, args...)
}
func (m *mockLogger) WithContext(ctx context.Context) contracts.Logger {
	return m
}
func (m *mockLogger) WithFields(fields ...interface{}) contracts.Logger {
	return m
}
func (m *mockLogger) WithError(err error) contracts.Logger {
	return m
}
func (m *mockLogger) Named(name string) contracts.Logger {
	return m
}
func (m *mockLogger) Sync() error {
	return nil
}

func (m *mockLogger) log(level, msg string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry := map[string]interface{}{
		"level":   level,
		"message": msg,
	}
	for i := 0; i < len(args)-1; i += 2 {
		if key, ok := args[i].(string); ok {
			entry[key] = args[i+1]
		}
	}
	m.messages = append(m.messages, entry)
}

func (m *mockLogger) getMessages() []map[string]interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.messages
}

func TestRecovery(t *testing.T) {
	t.Run("recovers from panic", func(t *testing.T) {
		middleware := Recovery()

		ctx := newTestContextWithRequest("GET", "/test", nil)

		handler := middleware(func(ctx *ucontext.Context) error {
			panic("test panic")
		})

		// Should not panic
		err := handler(ctx)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		// Should set 500 status code
		if ctx.Response().StatusCode != 500 {
			t.Errorf("expected status 500, got %d", ctx.Response().StatusCode)
		}
	})

	t.Run("passes through normal requests", func(t *testing.T) {
		middleware := Recovery()

		ctx := newTestContextWithRequest("GET", "/test", nil)

		handler := middleware(func(ctx *ucontext.Context) error {
			return ctx.JSON(200, map[string]string{"status": "ok"})
		})

		err := handler(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if ctx.Response().StatusCode != 200 {
			t.Errorf("expected status 200, got %d", ctx.Response().StatusCode)
		}
	})

	t.Run("passes through handler errors", func(t *testing.T) {
		middleware := Recovery()

		ctx := newTestContextWithRequest("GET", "/test", nil)

		expectedErr := errors.New("handler error")
		handler := middleware(func(ctx *ucontext.Context) error {
			return expectedErr
		})

		err := handler(ctx)
		if err != expectedErr {
			t.Errorf("expected handler error, got %v", err)
		}
	})

	t.Run("logs panic with logger", func(t *testing.T) {
		logger := &mockLogger{}

		middleware := RecoveryWithConfig(&RecoveryConfig{
			EnableStackTrace: true,
			Logger:           logger,
		})

		ctx := newTestContextWithRequest("GET", "/panic", nil)

		handler := middleware(func(ctx *ucontext.Context) error {
			panic("logged panic")
		})

		handler(ctx)

		messages := logger.getMessages()
		if len(messages) == 0 {
			t.Error("expected log message")
		}

		if messages[0]["level"] != "error" {
			t.Errorf("expected error level, got %v", messages[0]["level"])
		}
		if messages[0]["message"] != "panic recovered" {
			t.Errorf("expected 'panic recovered' message, got %v", messages[0]["message"])
		}
	})

	t.Run("calls OnPanic callback", func(t *testing.T) {
		var panicErr interface{}
		var panicStack []byte
		var onPanicCalled bool

		middleware := RecoveryWithConfig(&RecoveryConfig{
			EnableStackTrace: true,
			OnPanic: func(ctx *ucontext.Context, err interface{}, stack []byte) {
				onPanicCalled = true
				panicErr = err
				panicStack = stack
			},
		})

		ctx := newTestContextWithRequest("GET", "/test", nil)

		handler := middleware(func(ctx *ucontext.Context) error {
			panic("custom panic")
		})

		handler(ctx)

		if !onPanicCalled {
			t.Error("OnPanic was not called")
		}
		if panicErr != "custom panic" {
			t.Errorf("expected 'custom panic', got %v", panicErr)
		}
		if len(panicStack) == 0 {
			t.Error("expected stack trace")
		}
	})

	t.Run("disables stack trace", func(t *testing.T) {
		var capturedStack []byte

		middleware := RecoveryWithConfig(&RecoveryConfig{
			EnableStackTrace: false,
			OnPanic: func(ctx *ucontext.Context, err interface{}, stack []byte) {
				capturedStack = stack
			},
		})

		ctx := newTestContextWithRequest("GET", "/test", nil)

		handler := middleware(func(ctx *ucontext.Context) error {
			panic("no stack")
		})

		handler(ctx)

		if len(capturedStack) != 0 {
			t.Error("expected no stack trace")
		}
	})

	t.Run("nil config uses defaults", func(t *testing.T) {
		middleware := RecoveryWithConfig(nil)

		ctx := newTestContextWithRequest("GET", "/test", nil)

		handler := middleware(func(ctx *ucontext.Context) error {
			panic("test")
		})

		// Should not panic
		handler(ctx)

		if ctx.Response().StatusCode != 500 {
			t.Errorf("expected status 500, got %d", ctx.Response().StatusCode)
		}
	})
}

func TestDefaultRecoveryConfig(t *testing.T) {
	config := DefaultRecoveryConfig()

	if !config.EnableStackTrace {
		t.Error("expected EnableStackTrace to be true by default")
	}

	if config.StackSize != 4<<10 {
		t.Errorf("expected StackSize to be 4KB, got %d", config.StackSize)
	}
}

func BenchmarkRecovery(b *testing.B) {
	middleware := Recovery()

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

func BenchmarkRecoveryWithPanic(b *testing.B) {
	middleware := RecoveryWithConfig(&RecoveryConfig{
		EnableStackTrace: false, // Disable for benchmark
	})

	ctx := ucontext.New(context.Background())
	ctx.SetRequest(&ucontext.Request{Method: "GET", Path: "/test"})

	handler := middleware(func(ctx *ucontext.Context) error {
		panic("benchmark panic")
	})

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		handler(ctx)
	}
}
