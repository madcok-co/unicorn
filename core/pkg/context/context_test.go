package context

import (
	"context"
	"sync"
	"testing"
)

func TestContext_New(t *testing.T) {
	ctx := New(context.Background())

	if ctx == nil {
		t.Fatal("context should not be nil")
	}

	if ctx.Context() == nil {
		t.Error("underlying context should not be nil")
	}

	if ctx.Request() == nil {
		t.Error("request should be initialized")
	}

	if ctx.Response() == nil {
		t.Error("response should be initialized")
	}
}

func TestContext_Metadata(t *testing.T) {
	ctx := New(context.Background())

	t.Run("Set and Get", func(t *testing.T) {
		ctx.Set("key", "value")

		val, ok := ctx.Get("key")
		if !ok {
			t.Error("key should exist")
		}
		if val != "value" {
			t.Errorf("expected value, got %v", val)
		}
	})

	t.Run("Get non-existent key", func(t *testing.T) {
		_, ok := ctx.Get("nonexistent")
		if ok {
			t.Error("non-existent key should return false")
		}
	})

	t.Run("MustGet panics for missing key", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("MustGet should panic for missing key")
			}
		}()
		ctx.MustGet("missing")
	})

	t.Run("GetString", func(t *testing.T) {
		ctx.Set("str", "hello")
		ctx.Set("int", 123)

		if ctx.GetString("str") != "hello" {
			t.Error("should get string value")
		}
		if ctx.GetString("int") != "" {
			t.Error("should return empty for non-string")
		}
		if ctx.GetString("missing") != "" {
			t.Error("should return empty for missing key")
		}
	})

	t.Run("GetInt", func(t *testing.T) {
		ctx.Set("num", 42)
		ctx.Set("str", "not a number")

		if ctx.GetInt("num") != 42 {
			t.Error("should get int value")
		}
		if ctx.GetInt("str") != 0 {
			t.Error("should return 0 for non-int")
		}
	})

	t.Run("GetBool", func(t *testing.T) {
		ctx.Set("flag", true)
		ctx.Set("str", "not a bool")

		if ctx.GetBool("flag") != true {
			t.Error("should get bool value")
		}
		if ctx.GetBool("str") != false {
			t.Error("should return false for non-bool")
		}
	})

	t.Run("Keys", func(t *testing.T) {
		newCtx := New(context.Background())
		newCtx.Set("a", 1)
		newCtx.Set("b", 2)
		newCtx.Set("c", 3)

		keys := newCtx.Keys()
		if len(keys) != 3 {
			t.Errorf("expected 3 keys, got %d", len(keys))
		}
	})
}

func TestContext_Metadata_Concurrent(t *testing.T) {
	ctx := New(context.Background())

	var wg sync.WaitGroup
	iterations := 100

	// Concurrent writes
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ctx.Set("key", idx)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx.Get("key")
		}()
	}

	wg.Wait()
	// Test passes if no race condition panic
}

func TestContext_Response(t *testing.T) {
	ctx := New(context.Background())

	t.Run("JSON sets response", func(t *testing.T) {
		ctx.JSON(200, map[string]string{"message": "ok"})

		if ctx.Response().StatusCode != 200 {
			t.Errorf("expected status 200, got %d", ctx.Response().StatusCode)
		}
		if ctx.Response().Headers["Content-Type"] != "application/json" {
			t.Error("should set content type")
		}
	})

	t.Run("Error sets error response", func(t *testing.T) {
		ctx := New(context.Background())
		ctx.Error(400, "bad request")

		if ctx.Response().StatusCode != 400 {
			t.Errorf("expected status 400, got %d", ctx.Response().StatusCode)
		}
	})

	t.Run("Success sets 200 response", func(t *testing.T) {
		ctx := New(context.Background())
		ctx.Success(map[string]string{"data": "test"})

		if ctx.Response().StatusCode != 200 {
			t.Error("should set 200 status")
		}
	})

	t.Run("Created sets 201 response", func(t *testing.T) {
		ctx := New(context.Background())
		ctx.Created(map[string]int{"id": 123})

		if ctx.Response().StatusCode != 201 {
			t.Error("should set 201 status")
		}
	})

	t.Run("NoContent sets 204 response", func(t *testing.T) {
		ctx := New(context.Background())
		ctx.NoContent()

		if ctx.Response().StatusCode != 204 {
			t.Error("should set 204 status")
		}
	})
}

func TestContext_WithContext(t *testing.T) {
	ctx := New(context.Background())

	newCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx.WithContext(newCtx)

	if ctx.Context() != newCtx {
		t.Error("should update underlying context")
	}
}

func TestContext_Request(t *testing.T) {
	ctx := New(context.Background())

	req := &Request{
		Method:  "POST",
		Path:    "/users",
		Body:    []byte(`{"name":"test"}`),
		Headers: map[string]string{"Content-Type": "application/json"},
		Params:  map[string]string{"id": "123"},
		Query:   map[string]string{"page": "1"},
	}

	ctx.SetRequest(req)

	if ctx.Request().Method != "POST" {
		t.Error("should set method")
	}
	if ctx.Request().Path != "/users" {
		t.Error("should set path")
	}
	if string(ctx.Request().Body) != `{"name":"test"}` {
		t.Error("should set body")
	}
}

func TestContext_Observability(t *testing.T) {
	ctx := New(context.Background())

	t.Run("StartSpan with nil tracer", func(t *testing.T) {
		span, end := ctx.StartSpan("test")
		if span != nil {
			t.Error("span should be nil without tracer")
		}
		// Should not panic
		end()
	})

	t.Run("RecordMetric with nil metrics", func(t *testing.T) {
		// Should not panic
		ctx.RecordMetric("test", 1.0)
	})

	t.Run("IncrementCounter with nil metrics", func(t *testing.T) {
		// Should not panic
		ctx.IncrementCounter("test")
	})

	t.Run("TraceID with nil tracer", func(t *testing.T) {
		traceID := ctx.TraceID()
		if traceID != "" {
			t.Error("trace ID should be empty without tracer")
		}
	})

	t.Run("Publish with nil broker", func(t *testing.T) {
		err := ctx.Publish("topic", nil)
		if err != nil {
			t.Error("should not error with nil broker")
		}
	})
}

// ============ Multiple Adapters Tests ============

func TestContext_MultipleAdapters_Names(t *testing.T) {
	ctx := New(context.Background())

	t.Run("DBNames empty initially", func(t *testing.T) {
		names := ctx.DBNames()
		if len(names) != 0 {
			t.Errorf("expected 0 names, got %d", len(names))
		}
	})

	t.Run("CacheNames empty initially", func(t *testing.T) {
		names := ctx.CacheNames()
		if len(names) != 0 {
			t.Errorf("expected 0 names, got %d", len(names))
		}
	})

	t.Run("BrokerNames empty initially", func(t *testing.T) {
		names := ctx.BrokerNames()
		if len(names) != 0 {
			t.Errorf("expected 0 names, got %d", len(names))
		}
	})
}

func TestContext_NamedAccessors_NilForNonExistent(t *testing.T) {
	ctx := New(context.Background())

	t.Run("DB returns nil for non-existent name", func(t *testing.T) {
		db := ctx.DB("nonexistent")
		if db != nil {
			t.Error("expected nil for non-existent named DB")
		}
	})

	t.Run("Cache returns nil for non-existent name", func(t *testing.T) {
		cache := ctx.Cache("nonexistent")
		if cache != nil {
			t.Error("expected nil for non-existent named Cache")
		}
	})

	t.Run("Logger returns nil for non-existent name", func(t *testing.T) {
		logger := ctx.Logger("nonexistent")
		if logger != nil {
			t.Error("expected nil for non-existent named Logger")
		}
	})

	t.Run("Broker returns nil for non-existent name", func(t *testing.T) {
		broker := ctx.Broker("nonexistent")
		if broker != nil {
			t.Error("expected nil for non-existent named Broker")
		}
	})

	t.Run("Metrics returns nil for non-existent name", func(t *testing.T) {
		metrics := ctx.Metrics("nonexistent")
		if metrics != nil {
			t.Error("expected nil for non-existent named Metrics")
		}
	})

	t.Run("Tracer returns nil for non-existent name", func(t *testing.T) {
		tracer := ctx.Tracer("nonexistent")
		if tracer != nil {
			t.Error("expected nil for non-existent named Tracer")
		}
	})

	t.Run("Validator returns nil for non-existent name", func(t *testing.T) {
		validator := ctx.Validator("nonexistent")
		if validator != nil {
			t.Error("expected nil for non-existent named Validator")
		}
	})
}

func TestContext_DefaultAccessors_BackwardCompatible(t *testing.T) {
	ctx := New(context.Background())

	// All default accessors should return nil when not set (backward compatible)
	t.Run("Default accessors return nil when not set", func(t *testing.T) {
		if ctx.DB() != nil {
			t.Error("DB should be nil")
		}
		if ctx.Cache() != nil {
			t.Error("Cache should be nil")
		}
		if ctx.Logger() != nil {
			t.Error("Logger should be nil")
		}
		if ctx.Queue() != nil {
			t.Error("Queue should be nil")
		}
		if ctx.Broker() != nil {
			t.Error("Broker should be nil")
		}
		if ctx.Metrics() != nil {
			t.Error("Metrics should be nil")
		}
		if ctx.Tracer() != nil {
			t.Error("Tracer should be nil")
		}
		if ctx.Validator() != nil {
			t.Error("Validator should be nil")
		}
	})
}
