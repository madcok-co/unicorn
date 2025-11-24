package handler

import (
	"sync"
	"testing"

	"github.com/madcok-co/unicorn/core/pkg/context"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()

	if r == nil {
		t.Fatal("registry should not be nil")
	}
	if r.handlers == nil {
		t.Error("handlers map should be initialized")
	}
	if r.httpHandlers == nil {
		t.Error("httpHandlers map should be initialized")
	}
	if r.messageHandlers == nil {
		t.Error("messageHandlers map should be initialized")
	}
	if r.grpcHandlers == nil {
		t.Error("grpcHandlers map should be initialized")
	}
	if r.cronHandlers == nil {
		t.Error("cronHandlers slice should be initialized")
	}
	if r.Count() != 0 {
		t.Error("registry should be empty initially")
	}
}

func TestRegistry_Register(t *testing.T) {
	t.Run("registers handler with auto-generated name", func(t *testing.T) {
		r := NewRegistry()
		h := New(func(ctx *context.Context) error { return nil })

		err := r.Register(h)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if h.Name == "" {
			t.Error("name should be auto-generated")
		}
		if r.Count() != 1 {
			t.Error("count should be 1")
		}
	})

	t.Run("registers handler with custom name", func(t *testing.T) {
		r := NewRegistry()
		h := New(func(ctx *context.Context) error { return nil }).Named("my-handler")

		err := r.Register(h)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if h.Name != "my-handler" {
			t.Errorf("expected name 'my-handler', got %s", h.Name)
		}
	})

	t.Run("rejects duplicate handler name", func(t *testing.T) {
		r := NewRegistry()
		h1 := New(func(ctx *context.Context) error { return nil }).Named("handler")
		h2 := New(func(ctx *context.Context) error { return nil }).Named("handler")

		r.Register(h1)
		err := r.Register(h2)

		if err == nil {
			t.Error("should reject duplicate handler name")
		}
	})

	t.Run("indexes HTTP handler", func(t *testing.T) {
		r := NewRegistry()
		h := New(func(ctx *context.Context) error { return nil }).
			Named("users-list").
			HTTP("GET", "/users")

		r.Register(h)

		found, ok := r.GetHTTPHandler("GET", "/users")
		if !ok {
			t.Error("should find HTTP handler")
		}
		if found != h {
			t.Error("should return correct handler")
		}
	})

	t.Run("rejects duplicate HTTP route", func(t *testing.T) {
		r := NewRegistry()
		h1 := New(func(ctx *context.Context) error { return nil }).
			Named("h1").HTTP("GET", "/users")
		h2 := New(func(ctx *context.Context) error { return nil }).
			Named("h2").HTTP("GET", "/users")

		r.Register(h1)
		err := r.Register(h2)

		if err == nil {
			t.Error("should reject duplicate HTTP route")
		}
	})

	t.Run("indexes Message handler", func(t *testing.T) {
		r := NewRegistry()
		h := New(func(ctx *context.Context) error { return nil }).
			Named("order-processor").
			Message("orders.created")

		r.Register(h)

		found, ok := r.GetMessageHandler("orders.created")
		if !ok {
			t.Error("should find message handler")
		}
		if found != h {
			t.Error("should return correct handler")
		}
	})

	t.Run("rejects duplicate message topic", func(t *testing.T) {
		r := NewRegistry()
		h1 := New(func(ctx *context.Context) error { return nil }).
			Named("h1").Message("orders")
		h2 := New(func(ctx *context.Context) error { return nil }).
			Named("h2").Message("orders")

		r.Register(h1)
		err := r.Register(h2)

		if err == nil {
			t.Error("should reject duplicate message topic")
		}
	})

	t.Run("indexes gRPC handler", func(t *testing.T) {
		r := NewRegistry()
		h := New(func(ctx *context.Context) error { return nil }).
			Named("create-user").
			GRPC("UserService", "CreateUser")

		r.Register(h)

		found, ok := r.GetGRPCHandler("UserService", "CreateUser")
		if !ok {
			t.Error("should find gRPC handler")
		}
		if found != h {
			t.Error("should return correct handler")
		}
	})

	t.Run("rejects duplicate gRPC method", func(t *testing.T) {
		r := NewRegistry()
		h1 := New(func(ctx *context.Context) error { return nil }).
			Named("h1").GRPC("UserService", "Create")
		h2 := New(func(ctx *context.Context) error { return nil }).
			Named("h2").GRPC("UserService", "Create")

		r.Register(h1)
		err := r.Register(h2)

		if err == nil {
			t.Error("should reject duplicate gRPC method")
		}
	})

	t.Run("indexes Cron handler", func(t *testing.T) {
		r := NewRegistry()
		h := New(func(ctx *context.Context) error { return nil }).
			Named("daily-report").
			Cron("0 0 * * *")

		r.Register(h)

		handlers := r.GetCronHandlers()
		if len(handlers) != 1 {
			t.Fatalf("expected 1 cron handler, got %d", len(handlers))
		}
		if handlers[0] != h {
			t.Error("should return correct handler")
		}
	})

	t.Run("allows multiple cron handlers", func(t *testing.T) {
		r := NewRegistry()
		h1 := New(func(ctx *context.Context) error { return nil }).
			Named("h1").Cron("0 * * * *")
		h2 := New(func(ctx *context.Context) error { return nil }).
			Named("h2").Cron("0 0 * * *")

		r.Register(h1)
		r.Register(h2)

		handlers := r.GetCronHandlers()
		if len(handlers) != 2 {
			t.Errorf("expected 2 cron handlers, got %d", len(handlers))
		}
	})

	t.Run("indexes Kafka handler (legacy)", func(t *testing.T) {
		r := NewRegistry()
		h := New(func(ctx *context.Context) error { return nil }).
			Named("kafka-processor").
			Kafka("events")

		r.Register(h)

		// Should be found via both GetKafkaHandler and GetMessageHandler
		found, ok := r.GetKafkaHandler("events")
		if !ok {
			t.Error("should find Kafka handler")
		}
		if found != h {
			t.Error("should return correct handler")
		}

		found2, ok := r.GetMessageHandler("events")
		if !ok {
			t.Error("should also find via GetMessageHandler")
		}
		if found2 != h {
			t.Error("should return correct handler")
		}
	})

	t.Run("rejects duplicate Kafka topic", func(t *testing.T) {
		r := NewRegistry()
		h1 := New(func(ctx *context.Context) error { return nil }).
			Named("h1").Kafka("events")
		h2 := New(func(ctx *context.Context) error { return nil }).
			Named("h2").Kafka("events")

		r.Register(h1)
		err := r.Register(h2)

		if err == nil {
			t.Error("should reject duplicate Kafka topic")
		}
	})
}

func TestRegistry_Get(t *testing.T) {
	r := NewRegistry()
	h := New(func(ctx *context.Context) error { return nil }).Named("test-handler")
	r.Register(h)

	t.Run("finds existing handler", func(t *testing.T) {
		found, ok := r.Get("test-handler")
		if !ok {
			t.Error("should find handler")
		}
		if found != h {
			t.Error("should return correct handler")
		}
	})

	t.Run("returns false for non-existent handler", func(t *testing.T) {
		_, ok := r.Get("non-existent")
		if ok {
			t.Error("should not find non-existent handler")
		}
	})
}

func TestRegistry_All(t *testing.T) {
	r := NewRegistry()
	h1 := New(func(ctx *context.Context) error { return nil }).Named("h1")
	h2 := New(func(ctx *context.Context) error { return nil }).Named("h2")
	r.Register(h1)
	r.Register(h2)

	all := r.All()

	if len(all) != 2 {
		t.Errorf("expected 2 handlers, got %d", len(all))
	}
	if all["h1"] != h1 {
		t.Error("should contain h1")
	}
	if all["h2"] != h2 {
		t.Error("should contain h2")
	}
}

func TestRegistry_HTTPRoutes(t *testing.T) {
	r := NewRegistry()
	h1 := New(func(ctx *context.Context) error { return nil }).Named("h1").HTTP("GET", "/users")
	h2 := New(func(ctx *context.Context) error { return nil }).Named("h2").HTTP("POST", "/users")
	r.Register(h1)
	r.Register(h2)

	routes := r.HTTPRoutes()

	if len(routes) != 2 {
		t.Errorf("expected 2 routes, got %d", len(routes))
	}
	if routes["GET:/users"] != h1 {
		t.Error("should contain GET:/users")
	}
	if routes["POST:/users"] != h2 {
		t.Error("should contain POST:/users")
	}
}

func TestRegistry_MessageTopics(t *testing.T) {
	r := NewRegistry()
	h1 := New(func(ctx *context.Context) error { return nil }).Named("h1").Message("orders")
	h2 := New(func(ctx *context.Context) error { return nil }).Named("h2").Message("payments")
	r.Register(h1)
	r.Register(h2)

	topics := r.MessageTopics()

	if len(topics) != 2 {
		t.Errorf("expected 2 topics, got %d", len(topics))
	}
}

func TestRegistry_KafkaTopics(t *testing.T) {
	r := NewRegistry()
	h1 := New(func(ctx *context.Context) error { return nil }).Named("h1").Kafka("events")
	h2 := New(func(ctx *context.Context) error { return nil }).Named("h2").Kafka("logs")
	r.Register(h1)
	r.Register(h2)

	topics := r.KafkaTopics()

	if len(topics) != 2 {
		t.Errorf("expected 2 topics, got %d", len(topics))
	}
}

func TestRegistry_MessageHandlers(t *testing.T) {
	r := NewRegistry()
	h := New(func(ctx *context.Context) error { return nil }).Named("h1").Message("orders")
	r.Register(h)

	handlers := r.MessageHandlers()

	if len(handlers) != 1 {
		t.Errorf("expected 1 handler, got %d", len(handlers))
	}
	if handlers["orders"] != h {
		t.Error("should contain orders handler")
	}
}

func TestRegistry_HasHandlers(t *testing.T) {
	t.Run("HasHTTPHandlers", func(t *testing.T) {
		r := NewRegistry()
		if r.HasHTTPHandlers() {
			t.Error("should not have HTTP handlers")
		}

		h := New(func(ctx *context.Context) error { return nil }).Named("h").HTTP("GET", "/")
		r.Register(h)

		if !r.HasHTTPHandlers() {
			t.Error("should have HTTP handlers")
		}
	})

	t.Run("HasMessageHandlers", func(t *testing.T) {
		r := NewRegistry()
		if r.HasMessageHandlers() {
			t.Error("should not have message handlers")
		}

		h := New(func(ctx *context.Context) error { return nil }).Named("h").Message("topic")
		r.Register(h)

		if !r.HasMessageHandlers() {
			t.Error("should have message handlers")
		}
	})

	t.Run("HasCronHandlers", func(t *testing.T) {
		r := NewRegistry()
		if r.HasCronHandlers() {
			t.Error("should not have cron handlers")
		}

		h := New(func(ctx *context.Context) error { return nil }).Named("h").Cron("* * * * *")
		r.Register(h)

		if !r.HasCronHandlers() {
			t.Error("should have cron handlers")
		}
	})
}

func TestRegistry_CronSchedules(t *testing.T) {
	r := NewRegistry()
	h1 := New(func(ctx *context.Context) error { return nil }).Named("h1").Cron("0 * * * *")
	h2 := New(func(ctx *context.Context) error { return nil }).Named("h2").Cron("0 0 * * *")
	r.Register(h1)
	r.Register(h2)

	schedules := r.CronSchedules()

	if len(schedules) != 2 {
		t.Errorf("expected 2 schedules, got %d", len(schedules))
	}
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	r := NewRegistry()
	var wg sync.WaitGroup

	// Concurrent registrations
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			h := New(func(ctx *context.Context) error { return nil })
			r.Register(h)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.Count()
			r.All()
			r.HTTPRoutes()
			r.MessageTopics()
			r.HasHTTPHandlers()
			r.HasMessageHandlers()
			r.HasCronHandlers()
		}()
	}

	wg.Wait()

	if r.Count() != 100 {
		t.Errorf("expected 100 handlers, got %d", r.Count())
	}
}

func TestRegistry_MultiTriggerHandler(t *testing.T) {
	r := NewRegistry()
	h := New(func(ctx *context.Context) error { return nil }).
		Named("multi-trigger").
		HTTP("POST", "/orders").
		Message("orders.webhook").
		Cron("0 0 * * *")

	err := r.Register(h)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be findable via all trigger types
	if _, ok := r.GetHTTPHandler("POST", "/orders"); !ok {
		t.Error("should find via HTTP")
	}
	if _, ok := r.GetMessageHandler("orders.webhook"); !ok {
		t.Error("should find via Message")
	}
	if len(r.GetCronHandlers()) != 1 {
		t.Error("should have cron handler")
	}
}

func BenchmarkRegistry_Register(b *testing.B) {
	fn := func(ctx *context.Context) error { return nil }

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		r := NewRegistry()
		h := New(fn).HTTP("GET", "/test")
		r.Register(h)
	}
}

func BenchmarkRegistry_GetHTTPHandler(b *testing.B) {
	r := NewRegistry()
	h := New(func(ctx *context.Context) error { return nil }).
		Named("test").HTTP("GET", "/users")
	r.Register(h)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		r.GetHTTPHandler("GET", "/users")
	}
}

func BenchmarkRegistry_ConcurrentReads(b *testing.B) {
	r := NewRegistry()
	for i := 0; i < 100; i++ {
		h := New(func(ctx *context.Context) error { return nil }).HTTP("GET", "/test")
		r.Register(h)
	}

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			r.All()
			r.Count()
		}
	})
}
