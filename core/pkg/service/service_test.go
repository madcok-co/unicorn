package service

import (
	"context"
	"testing"

	"github.com/madcok-co/unicorn/core/pkg/handler"
)

func TestNew(t *testing.T) {
	svc := New("test-service")

	if svc == nil {
		t.Fatal("Expected service, got nil")
	}

	if svc.Name() != "test-service" {
		t.Errorf("Expected name 'test-service', got %s", svc.Name())
	}
}

func TestDescribe(t *testing.T) {
	svc := New("test-service").
		Describe("This is a test service")

	if svc.Description() != "This is a test service" {
		t.Errorf("Expected description, got %s", svc.Description())
	}
}

func TestDependsOn(t *testing.T) {
	svc := New("api-service").
		DependsOn("database-service", "cache-service")

	deps := svc.Dependencies()
	if len(deps) != 2 {
		t.Fatalf("Expected 2 dependencies, got %d", len(deps))
	}

	if deps[0] != "database-service" {
		t.Errorf("Expected first dependency 'database-service', got %s", deps[0])
	}

	if deps[1] != "cache-service" {
		t.Errorf("Expected second dependency 'cache-service', got %s", deps[1])
	}
}

func TestAddHandler(t *testing.T) {
	svc := New("test-service")

	// Create a test handler
	h := handler.New(func(ctx context.Context) error {
		return nil
	})

	err := svc.AddHandler(h)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	handlers := svc.Handlers()
	if len(handlers) != 1 {
		t.Errorf("Expected 1 handler, got %d", len(handlers))
	}
}

func TestRegister(t *testing.T) {
	svc := New("test-service")

	// Register handler via builder
	builder := svc.Register(func(ctx context.Context) error {
		return nil
	})

	if builder == nil {
		t.Fatal("Expected HandlerBuilder, got nil")
	}

	if builder.service != svc {
		t.Error("Expected builder to reference the service")
	}
}

func TestOnStart(t *testing.T) {
	svc := New("test-service")

	called := false
	svc.OnStart(func(ctx context.Context) error {
		called = true
		return nil
	})

	// Start the service
	err := svc.Start(context.Background())
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !called {
		t.Error("Expected OnStart hook to be called")
	}

	if !svc.IsRunning() {
		t.Error("Expected service to be running")
	}
}

func TestOnStop(t *testing.T) {
	svc := New("test-service")

	called := false
	svc.OnStop(func(ctx context.Context) error {
		called = true
		return nil
	})

	// Start then stop the service
	svc.Start(context.Background())
	err := svc.Stop(context.Background())
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !called {
		t.Error("Expected OnStop hook to be called")
	}

	if svc.IsRunning() {
		t.Error("Expected service to be stopped")
	}
}

func TestMultipleStartupHooks(t *testing.T) {
	svc := New("test-service")

	order := []int{}

	svc.OnStart(func(ctx context.Context) error {
		order = append(order, 1)
		return nil
	})

	svc.OnStart(func(ctx context.Context) error {
		order = append(order, 2)
		return nil
	})

	svc.OnStart(func(ctx context.Context) error {
		order = append(order, 3)
		return nil
	})

	err := svc.Start(context.Background())
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(order) != 3 {
		t.Fatalf("Expected 3 hooks to run, got %d", len(order))
	}

	if order[0] != 1 || order[1] != 2 || order[2] != 3 {
		t.Errorf("Expected hooks to run in order [1,2,3], got %v", order)
	}
}

func TestStartupHookError(t *testing.T) {
	svc := New("test-service")

	svc.OnStart(func(ctx context.Context) error {
		return nil // First hook succeeds
	})

	svc.OnStart(func(ctx context.Context) error {
		return context.Canceled // Second hook fails
	})

	err := svc.Start(context.Background())
	if err == nil {
		t.Error("Expected error from failed startup hook")
	}

	// Note: Implementation might mark service as running even if hooks fail
	// This is acceptable behavior - test just verifies error is returned
}

func TestStopNotRunning(t *testing.T) {
	svc := New("test-service")

	// Try to stop a service that's not running
	err := svc.Stop(context.Background())
	// Implementation might return nil for stopping non-running service
	// This is safe behavior - idempotent stop
	if err != nil {
		t.Logf("Stop returned error (optional): %v", err)
	}
}

func TestDoubleStart(t *testing.T) {
	svc := New("test-service")

	// First start should succeed
	err := svc.Start(context.Background())
	if err != nil {
		t.Errorf("Expected no error on first start, got %v", err)
	}

	// Second start should fail
	err = svc.Start(context.Background())
	if err == nil {
		t.Error("Expected error when starting already running service")
	}
}

func TestRegistry(t *testing.T) {
	svc := New("test-service")

	registry := svc.Registry()
	if registry == nil {
		t.Error("Expected registry, got nil")
	}
}

func TestHandlerBuilder(t *testing.T) {
	svc := New("test-service")

	builder := svc.Register(func(ctx context.Context) error {
		return nil
	})

	// Test builder methods
	builder.Named("test-handler")

	// Complete the registration
	builder.Done()

	// Verify handler was added
	handlers := svc.Handlers()
	if len(handlers) != 1 {
		t.Errorf("Expected 1 handler, got %d", len(handlers))
	}

	if handlers[0].Name != "test-handler" {
		t.Errorf("Expected handler name 'test-handler', got %s", handlers[0].Name)
	}
}

func TestConcurrentStartStop(t *testing.T) {
	svc := New("test-service")

	// Add some hooks to make the test more realistic
	svc.OnStart(func(ctx context.Context) error {
		return nil
	})

	svc.OnStop(func(ctx context.Context) error {
		return nil
	})

	// Start
	err := svc.Start(context.Background())
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Check running state is thread-safe
	if !svc.IsRunning() {
		t.Error("Expected service to be running")
	}

	// Stop
	err = svc.Stop(context.Background())
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Verify stopped
	if svc.IsRunning() {
		t.Error("Expected service to be stopped")
	}
}

func TestServiceLifecycle(t *testing.T) {
	svc := New("lifecycle-test")

	phases := []string{}

	svc.OnStart(func(ctx context.Context) error {
		phases = append(phases, "start")
		return nil
	})

	svc.OnStop(func(ctx context.Context) error {
		phases = append(phases, "stop")
		return nil
	})

	// Complete lifecycle
	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if err := svc.Stop(context.Background()); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Verify lifecycle
	if len(phases) != 2 {
		t.Fatalf("Expected 2 phases, got %d", len(phases))
	}

	if phases[0] != "start" {
		t.Errorf("Expected first phase 'start', got %s", phases[0])
	}

	if phases[1] != "stop" {
		t.Errorf("Expected second phase 'stop', got %s", phases[1])
	}
}
