package audit

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

func TestInMemoryAuditLogger(t *testing.T) {
	t.Run("logs events", func(t *testing.T) {
		logger := NewInMemoryAuditLogger(&InMemoryAuditLoggerConfig{
			MaxEvents: 100,
			Async:     false,
		})
		defer logger.Close()

		event := &contracts.AuditEvent{
			Action:   ActionCreate,
			Resource: "users",
			ActorID:  "user-123",
			Success:  true,
		}

		err := logger.Log(context.Background(), event)
		if err != nil {
			t.Fatalf("failed to log: %v", err)
		}

		if event.ID == "" {
			t.Error("should generate ID")
		}

		if event.Timestamp.IsZero() {
			t.Error("should set timestamp")
		}

		if logger.GetEventCount() != 1 {
			t.Errorf("expected 1 event, got %d", logger.GetEventCount())
		}
	})

	t.Run("queries events", func(t *testing.T) {
		logger := NewInMemoryAuditLogger(&InMemoryAuditLoggerConfig{
			MaxEvents: 100,
			Async:     false,
		})
		defer logger.Close()

		// Log multiple events
		events := []*contracts.AuditEvent{
			{Action: ActionCreate, Resource: "users", ActorID: "user-1", Success: true},
			{Action: ActionRead, Resource: "users", ActorID: "user-2", Success: true},
			{Action: ActionUpdate, Resource: "orders", ActorID: "user-1", Success: false},
			{Action: ActionDelete, Resource: "users", ActorID: "user-1", Success: true},
		}

		for _, e := range events {
			logger.Log(context.Background(), e)
		}

		// Query by actor
		results, _ := logger.Query(context.Background(), &contracts.AuditFilter{
			ActorID: "user-1",
		})
		if len(results) != 3 {
			t.Errorf("expected 3 events for user-1, got %d", len(results))
		}

		// Query by action
		results, _ = logger.Query(context.Background(), &contracts.AuditFilter{
			Action: ActionCreate,
		})
		if len(results) != 1 {
			t.Errorf("expected 1 create event, got %d", len(results))
		}

		// Query by resource
		results, _ = logger.Query(context.Background(), &contracts.AuditFilter{
			Resource: "users",
		})
		if len(results) != 3 {
			t.Errorf("expected 3 user events, got %d", len(results))
		}

		// Query by success
		success := true
		results, _ = logger.Query(context.Background(), &contracts.AuditFilter{
			Success: &success,
		})
		if len(results) != 3 {
			t.Errorf("expected 3 successful events, got %d", len(results))
		}
	})

	t.Run("applies limit and offset", func(t *testing.T) {
		logger := NewInMemoryAuditLogger(&InMemoryAuditLoggerConfig{
			MaxEvents: 100,
			Async:     false,
		})
		defer logger.Close()

		for i := 0; i < 10; i++ {
			logger.Log(context.Background(), &contracts.AuditEvent{
				Action: ActionRead,
			})
		}

		results, _ := logger.Query(context.Background(), &contracts.AuditFilter{
			Limit: 5,
		})
		if len(results) != 5 {
			t.Errorf("expected 5 events with limit, got %d", len(results))
		}

		results, _ = logger.Query(context.Background(), &contracts.AuditFilter{
			Offset: 8,
		})
		if len(results) != 2 {
			t.Errorf("expected 2 events with offset, got %d", len(results))
		}
	})

	t.Run("respects max events", func(t *testing.T) {
		logger := NewInMemoryAuditLogger(&InMemoryAuditLoggerConfig{
			MaxEvents: 10,
			Async:     false,
		})
		defer logger.Close()

		for i := 0; i < 20; i++ {
			logger.Log(context.Background(), &contracts.AuditEvent{
				Action: ActionRead,
			})
		}

		if logger.GetEventCount() > 10 {
			t.Errorf("should not exceed max events, got %d", logger.GetEventCount())
		}
	})

	t.Run("clears events", func(t *testing.T) {
		logger := NewInMemoryAuditLogger(&InMemoryAuditLoggerConfig{
			MaxEvents: 100,
			Async:     false,
		})
		defer logger.Close()

		logger.Log(context.Background(), &contracts.AuditEvent{Action: ActionRead})
		logger.Log(context.Background(), &contracts.AuditEvent{Action: ActionRead})

		logger.Clear()

		if logger.GetEventCount() != 0 {
			t.Error("should have no events after clear")
		}
	})

	t.Run("calls handlers", func(t *testing.T) {
		logger := NewInMemoryAuditLogger(&InMemoryAuditLoggerConfig{
			MaxEvents: 100,
			Async:     false,
		})
		defer logger.Close()

		handlerCalled := false
		logger.AddHandler(func(event *contracts.AuditEvent) {
			handlerCalled = true
		})

		logger.Log(context.Background(), &contracts.AuditEvent{Action: ActionRead})

		if !handlerCalled {
			t.Error("handler should be called")
		}
	})

	t.Run("recovers from handler panic", func(t *testing.T) {
		logger := NewInMemoryAuditLogger(&InMemoryAuditLoggerConfig{
			MaxEvents: 100,
			Async:     false,
		})
		defer logger.Close()

		logger.AddHandler(func(event *contracts.AuditEvent) {
			panic("handler panic")
		})

		// Should not panic
		logger.Log(context.Background(), &contracts.AuditEvent{Action: ActionRead})

		if logger.GetEventCount() != 1 {
			t.Error("event should still be logged")
		}
	})
}

func TestInMemoryAuditLogger_Async(t *testing.T) {
	t.Run("logs asynchronously", func(t *testing.T) {
		logger := NewInMemoryAuditLogger(&InMemoryAuditLoggerConfig{
			MaxEvents:  100,
			BufferSize: 10,
			Async:      true,
		})
		defer logger.Close()

		for i := 0; i < 5; i++ {
			logger.Log(context.Background(), &contracts.AuditEvent{Action: ActionRead})
		}

		// Wait for async processing
		time.Sleep(50 * time.Millisecond)

		if logger.GetEventCount() != 5 {
			t.Errorf("expected 5 events, got %d", logger.GetEventCount())
		}
	})

	t.Run("handles buffer overflow gracefully", func(t *testing.T) {
		logger := NewInMemoryAuditLogger(&InMemoryAuditLoggerConfig{
			MaxEvents:  1000,
			BufferSize: 5,
			Async:      true,
		})
		defer logger.Close()

		// Flood with events
		for i := 0; i < 100; i++ {
			logger.Log(context.Background(), &contracts.AuditEvent{Action: ActionRead})
		}

		// Wait for processing
		time.Sleep(100 * time.Millisecond)

		// Should have logged all events (some sync, some async)
		if logger.GetEventCount() < 50 {
			t.Errorf("should have logged most events, got %d", logger.GetEventCount())
		}
	})
}

func TestInMemoryAuditLogger_Concurrent(t *testing.T) {
	logger := NewInMemoryAuditLogger(&InMemoryAuditLoggerConfig{
		MaxEvents: 10000,
		Async:     false,
	})
	defer logger.Close()

	var wg sync.WaitGroup
	eventCount := 100

	for i := 0; i < eventCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			logger.Log(context.Background(), &contracts.AuditEvent{
				Action: ActionRead,
			})
		}()
	}

	wg.Wait()

	if logger.GetEventCount() != eventCount {
		t.Errorf("expected %d events, got %d", eventCount, logger.GetEventCount())
	}
}

func TestAuditEventBuilder(t *testing.T) {
	t.Run("builds event with fluent API", func(t *testing.T) {
		event := NewAuditEvent().
			Action(ActionCreate).
			Resource("users").
			ResourceID("user-123").
			Actor("actor-1", "user", "John Doe").
			ActorIP("192.168.1.1").
			Request("POST", "/users", "Mozilla/5.0").
			Success(true).
			Metadata("extra", "data").
			Build()

		if event.Action != ActionCreate {
			t.Errorf("expected action %s, got %s", ActionCreate, event.Action)
		}
		if event.Resource != "users" {
			t.Errorf("expected resource users, got %s", event.Resource)
		}
		if event.ActorID != "actor-1" {
			t.Errorf("expected actor actor-1, got %s", event.ActorID)
		}
		if event.Success != true {
			t.Error("expected success true")
		}
		if event.Metadata["extra"] != "data" {
			t.Error("expected metadata to be set")
		}
	})

	t.Run("Error sets success to false", func(t *testing.T) {
		event := NewAuditEvent().
			Action(ActionUpdate).
			Error("something went wrong").
			Build()

		if event.Success != false {
			t.Error("Error should set success to false")
		}
		if event.Error != "something went wrong" {
			t.Error("error message should be set")
		}
	})

	t.Run("logs directly to logger", func(t *testing.T) {
		logger := NewInMemoryAuditLogger(&InMemoryAuditLoggerConfig{
			MaxEvents: 100,
			Async:     false,
		})
		defer logger.Close()

		err := NewAuditEvent().
			Action(ActionLogin).
			Actor("user-1", "user", "Test User").
			Success(true).
			Log(context.Background(), logger)

		if err != nil {
			t.Fatalf("failed to log: %v", err)
		}

		if logger.GetEventCount() != 1 {
			t.Error("should have logged event")
		}
	})
}

func TestCompositeAuditLogger(t *testing.T) {
	logger1 := NewInMemoryAuditLogger(&InMemoryAuditLoggerConfig{
		MaxEvents: 100,
		Async:     false,
	})
	defer logger1.Close()

	logger2 := NewInMemoryAuditLogger(&InMemoryAuditLoggerConfig{
		MaxEvents: 100,
		Async:     false,
	})
	defer logger2.Close()

	composite := NewCompositeAuditLogger(logger1, logger2)

	t.Run("logs to all loggers", func(t *testing.T) {
		composite.Log(context.Background(), &contracts.AuditEvent{
			Action: ActionRead,
		})

		if logger1.GetEventCount() != 1 {
			t.Error("logger1 should have event")
		}
		if logger2.GetEventCount() != 1 {
			t.Error("logger2 should have event")
		}
	})

	t.Run("queries from first logger", func(t *testing.T) {
		results, _ := composite.Query(context.Background(), nil)
		if len(results) != 1 {
			t.Errorf("expected 1 event, got %d", len(results))
		}
	})
}
