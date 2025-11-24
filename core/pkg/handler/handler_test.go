package handler

import (
	"testing"

	"github.com/madcok-co/unicorn/core/pkg/context"
)

func TestNew(t *testing.T) {
	t.Run("creates handler from function", func(t *testing.T) {
		fn := func(ctx *context.Context) error {
			return nil
		}

		h := New(fn)

		if h == nil {
			t.Fatal("handler should not be nil")
		}
		if h.fn == nil {
			t.Error("handler function should be set")
		}
	})

	t.Run("initializes empty triggers", func(t *testing.T) {
		h := New(func(ctx *context.Context) error { return nil })

		if h.triggers == nil {
			t.Error("triggers should be initialized")
		}
		if len(h.triggers) != 0 {
			t.Error("triggers should be empty initially")
		}
	})

	t.Run("extracts request type from function", func(t *testing.T) {
		type CreateUserRequest struct {
			Name string
		}

		fn := func(ctx *context.Context, req CreateUserRequest) error {
			return nil
		}

		h := New(fn)

		if h.requestType == nil {
			t.Error("request type should be extracted")
		}
	})
}

func TestHandler_Named(t *testing.T) {
	h := New(func(ctx *context.Context) error { return nil })

	result := h.Named("test-handler")

	if result != h {
		t.Error("should return handler for chaining")
	}
	if h.Name != "test-handler" {
		t.Errorf("expected name 'test-handler', got %s", h.Name)
	}
}

func TestHandler_Describe(t *testing.T) {
	h := New(func(ctx *context.Context) error { return nil })

	result := h.Describe("Test description")

	if result != h {
		t.Error("should return handler for chaining")
	}
	if h.Description != "Test description" {
		t.Errorf("expected description 'Test description', got %s", h.Description)
	}
}

func TestHandler_HTTP(t *testing.T) {
	t.Run("adds HTTP trigger", func(t *testing.T) {
		h := New(func(ctx *context.Context) error { return nil })

		result := h.HTTP("GET", "/users")

		if result != h {
			t.Error("should return handler for chaining")
		}
		if len(h.triggers) != 1 {
			t.Fatalf("expected 1 trigger, got %d", len(h.triggers))
		}

		trigger := h.triggers[0].(*HTTPTrigger)
		if trigger.Method != "GET" {
			t.Errorf("expected method GET, got %s", trigger.Method)
		}
		if trigger.Path != "/users" {
			t.Errorf("expected path /users, got %s", trigger.Path)
		}
	})

	t.Run("adds multiple HTTP triggers", func(t *testing.T) {
		h := New(func(ctx *context.Context) error { return nil })

		h.HTTP("GET", "/users").HTTP("POST", "/users")

		if len(h.triggers) != 2 {
			t.Errorf("expected 2 triggers, got %d", len(h.triggers))
		}
	})
}

func TestHandler_Message(t *testing.T) {
	t.Run("adds Message trigger with defaults", func(t *testing.T) {
		h := New(func(ctx *context.Context) error { return nil })

		h.Message("user.created")

		if len(h.triggers) != 1 {
			t.Fatalf("expected 1 trigger, got %d", len(h.triggers))
		}

		trigger := h.triggers[0].(*MessageTrigger)
		if trigger.Topic != "user.created" {
			t.Errorf("expected topic user.created, got %s", trigger.Topic)
		}
		if !trigger.AutoAck {
			t.Error("AutoAck should be true by default")
		}
	})

	t.Run("adds Message trigger with options", func(t *testing.T) {
		h := New(func(ctx *context.Context) error { return nil })

		h.Message("orders", WithGroup("order-processor"), WithAutoAck(false))

		trigger := h.triggers[0].(*MessageTrigger)
		if trigger.Group != "order-processor" {
			t.Errorf("expected group order-processor, got %s", trigger.Group)
		}
		if trigger.AutoAck {
			t.Error("AutoAck should be false with WithAutoAck(false)")
		}
	})
}

func TestHandler_Cron(t *testing.T) {
	h := New(func(ctx *context.Context) error { return nil })

	h.Cron("0 * * * *")

	if len(h.triggers) != 1 {
		t.Fatalf("expected 1 trigger, got %d", len(h.triggers))
	}

	trigger := h.triggers[0].(*CronTrigger)
	if trigger.Schedule != "0 * * * *" {
		t.Errorf("expected schedule '0 * * * *', got %s", trigger.Schedule)
	}
}

func TestHandler_GRPC(t *testing.T) {
	h := New(func(ctx *context.Context) error { return nil })

	h.GRPC("UserService", "CreateUser")

	if len(h.triggers) != 1 {
		t.Fatalf("expected 1 trigger, got %d", len(h.triggers))
	}

	trigger := h.triggers[0].(*GRPCTrigger)
	if trigger.Service != "UserService" {
		t.Errorf("expected service UserService, got %s", trigger.Service)
	}
	if trigger.Method != "CreateUser" {
		t.Errorf("expected method CreateUser, got %s", trigger.Method)
	}
}

func TestHandler_Use(t *testing.T) {
	h := New(func(ctx *context.Context) error { return nil })

	middleware := func(next HandlerExecutor) HandlerExecutor {
		return func(ctx *context.Context) error {
			return next(ctx)
		}
	}

	result := h.Use(middleware)

	if result != h {
		t.Error("should return handler for chaining")
	}
	if len(h.middlewares) != 1 {
		t.Errorf("expected 1 middleware, got %d", len(h.middlewares))
	}
}

func TestHandler_HasTriggerType(t *testing.T) {
	h := New(func(ctx *context.Context) error { return nil })
	h.HTTP("GET", "/test").Cron("0 * * * *")

	if !h.HasTriggerType(TriggerHTTP) {
		t.Error("should have HTTP trigger")
	}
	if !h.HasTriggerType(TriggerCron) {
		t.Error("should have Cron trigger")
	}
	if h.HasTriggerType(TriggerGRPC) {
		t.Error("should not have gRPC trigger")
	}
}

func TestHandler_Getters(t *testing.T) {
	h := New(func(ctx *context.Context) error { return nil })
	h.Named("test").HTTP("GET", "/test")

	if h.Fn() == nil {
		t.Error("Fn() should return function")
	}
	if len(h.Triggers()) != 1 {
		t.Error("Triggers() should return triggers")
	}

	// Add middleware and verify getter
	middleware := func(next HandlerExecutor) HandlerExecutor {
		return func(ctx *context.Context) error {
			return next(ctx)
		}
	}
	h.Use(middleware)

	if len(h.Middlewares()) != 1 {
		t.Error("Middlewares() should return 1 middleware")
	}
}

func TestHandler_Chaining(t *testing.T) {
	h := New(func(ctx *context.Context) error { return nil }).
		Named("multi-trigger").
		Describe("Handles multiple triggers").
		HTTP("POST", "/orders").
		Message("order.created").
		Cron("0 0 * * *")

	if h.Name != "multi-trigger" {
		t.Error("name not set")
	}
	if h.Description != "Handles multiple triggers" {
		t.Error("description not set")
	}
	if len(h.triggers) != 3 {
		t.Errorf("expected 3 triggers, got %d", len(h.triggers))
	}
}

func BenchmarkNew(b *testing.B) {
	fn := func(ctx *context.Context) error { return nil }

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		New(fn)
	}
}

func BenchmarkHandler_HTTP(b *testing.B) {
	fn := func(ctx *context.Context) error { return nil }

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		h := New(fn)
		h.HTTP("GET", "/test")
	}
}
