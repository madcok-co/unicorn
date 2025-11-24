package app

import (
	"context"
	"errors"
	"testing"

	ucontext "github.com/madcok-co/unicorn/core/pkg/context"
)

func TestNew(t *testing.T) {
	t.Run("creates app with default config", func(t *testing.T) {
		app := New(nil)

		if app == nil {
			t.Fatal("app should not be nil")
		}
		if app.name != "unicorn-app" {
			t.Errorf("expected default name, got %s", app.name)
		}
		if app.version != "1.0.0" {
			t.Errorf("expected default version, got %s", app.version)
		}
		if !app.config.EnableHTTP {
			t.Error("HTTP should be enabled by default")
		}
	})

	t.Run("creates app with custom config", func(t *testing.T) {
		config := &Config{
			Name:       "my-app",
			Version:    "2.0.0",
			EnableHTTP: true,
		}
		app := New(config)

		if app.name != "my-app" {
			t.Errorf("expected 'my-app', got %s", app.name)
		}
		if app.version != "2.0.0" {
			t.Errorf("expected '2.0.0', got %s", app.version)
		}
	})

	t.Run("initializes all required components", func(t *testing.T) {
		app := New(nil)

		if app.registry == nil {
			t.Error("registry should be initialized")
		}
		if app.services == nil {
			t.Error("services should be initialized")
		}
		if app.customServices == nil {
			t.Error("customServices should be initialized")
		}
		if app.adapters == nil {
			t.Error("adapters should be initialized")
		}
		if app.ctx == nil {
			t.Error("context should be initialized")
		}
	})

	t.Run("initializes adapter maps", func(t *testing.T) {
		app := New(nil)

		if app.adapters.Databases == nil {
			t.Error("databases map should be initialized")
		}
		if app.adapters.Caches == nil {
			t.Error("caches map should be initialized")
		}
		if app.adapters.Loggers == nil {
			t.Error("loggers map should be initialized")
		}
	})
}

func TestApp_LifecycleHooks(t *testing.T) {
	t.Run("OnStart adds hooks", func(t *testing.T) {
		app := New(nil)

		app.OnStart(func() error {
			return nil
		})

		if len(app.onStart) != 1 {
			t.Error("hook should be added")
		}
	})

	t.Run("OnStop adds hooks", func(t *testing.T) {
		app := New(nil)

		app.OnStop(func() error {
			return nil
		})

		if len(app.onStop) != 1 {
			t.Error("hook should be added")
		}
	})

	t.Run("OnStart returns self for chaining", func(t *testing.T) {
		app := New(nil)

		result := app.OnStart(func() error { return nil })

		if result != app {
			t.Error("should return app for chaining")
		}
	})

	t.Run("OnStop returns self for chaining", func(t *testing.T) {
		app := New(nil)

		result := app.OnStop(func() error { return nil })

		if result != app {
			t.Error("should return app for chaining")
		}
	})

	t.Run("multiple OnStart hooks added correctly", func(t *testing.T) {
		app := New(&Config{
			Name:       "test-app",
			EnableHTTP: false,
		})

		app.OnStart(func() error { return nil })
		app.OnStart(func() error { return nil })
		app.OnStart(func() error { return nil })

		if len(app.onStart) != 3 {
			t.Errorf("expected 3 hooks, got %d", len(app.onStart))
		}
	})

	t.Run("OnStart hook failure stops startup", func(t *testing.T) {
		app := New(&Config{
			Name:       "test-app",
			EnableHTTP: false,
		})

		app.OnStart(func() error {
			return errors.New("startup failed")
		})

		// RunServices will fail on hook error before entering the wait loop
		err := app.RunServices()
		if err == nil {
			t.Error("expected error from failed hook")
		}
	})

	t.Run("OnStop hooks are called on shutdown", func(t *testing.T) {
		app := New(&Config{
			Name:       "test-app",
			EnableHTTP: false,
		})

		called := false
		app.OnStop(func() error {
			called = true
			return nil
		})

		app.Shutdown()

		if !called {
			t.Error("OnStop hook should be called")
		}
	})

	t.Run("OnStop hooks continue even on error", func(t *testing.T) {
		app := New(&Config{
			Name:       "test-app",
			EnableHTTP: false,
		})

		hook1Called := false
		hook2Called := false

		app.OnStop(func() error {
			hook1Called = true
			return errors.New("hook1 error")
		})
		app.OnStop(func() error {
			hook2Called = true
			return nil
		})

		app.Shutdown()

		if !hook1Called {
			t.Error("first hook should be called")
		}
		if !hook2Called {
			t.Error("second hook should be called even after first fails")
		}
	})
}

func TestApp_RegisterHandler(t *testing.T) {
	t.Run("registers handler with HTTP trigger", func(t *testing.T) {
		app := New(nil)

		err := app.RegisterHandler(func(ctx *ucontext.Context) error {
			return nil
		}).HTTP("GET", "/test").Done()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !app.registry.HasHTTPHandlers() {
			t.Error("should have HTTP handlers registered")
		}
	})

	t.Run("registers named handler", func(t *testing.T) {
		app := New(nil)

		err := app.RegisterHandler(func(ctx *ucontext.Context) error {
			return nil
		}).Named("test-handler").HTTP("POST", "/users").Done()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("registers handler with multiple triggers", func(t *testing.T) {
		app := New(nil)

		err := app.RegisterHandler(func(ctx *ucontext.Context) error {
			return nil
		}).
			Named("multi-trigger").
			HTTP("POST", "/orders").
			Cron("0 * * * *").
			Done()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !app.registry.HasHTTPHandlers() {
			t.Error("should have HTTP handlers")
		}
		if !app.registry.HasCronHandlers() {
			t.Error("should have Cron handlers")
		}
	})
}

func TestApp_RegisterService(t *testing.T) {
	t.Run("registers singleton service", func(t *testing.T) {
		app := New(nil)
		svc := &testService{name: "email"}

		result := app.RegisterService("email", svc)

		if result != app {
			t.Error("should return app for chaining")
		}
		if app.customServices == nil {
			t.Error("custom services should exist")
		}
	})

	t.Run("registers multiple services", func(t *testing.T) {
		app := New(nil)

		app.RegisterService("email", &testService{name: "email"})
		app.RegisterService("sms", &testService{name: "sms"})

		// No error means success
	})
}

func TestApp_Service(t *testing.T) {
	t.Run("creates new service", func(t *testing.T) {
		app := New(nil)

		svc := app.Service("user-service")

		if svc == nil {
			t.Error("service should be created")
		}
	})

	t.Run("returns same service on multiple calls", func(t *testing.T) {
		app := New(nil)

		svc1 := app.Service("user-service")
		svc2 := app.Service("user-service")

		if svc1 != svc2 {
			t.Error("should return same service instance")
		}
	})
}

func TestApp_NewContext(t *testing.T) {
	t.Run("creates context", func(t *testing.T) {
		app := New(nil)

		ctx := app.NewContext(context.Background())

		if ctx == nil {
			t.Error("context should not be nil")
		}
	})

	t.Run("context has underlying context", func(t *testing.T) {
		app := New(nil)

		ctx := app.NewContext(context.Background())

		if ctx.Context() == nil {
			t.Error("underlying context should exist")
		}
	})
}

func TestApp_Shutdown(t *testing.T) {
	t.Run("cancels context", func(t *testing.T) {
		app := New(nil)

		// Get a reference to the app's context
		appCtx := app.ctx

		app.Shutdown()

		// Context should be cancelled
		select {
		case <-appCtx.Done():
			// Expected
		default:
			t.Error("context should be cancelled after shutdown")
		}
	})

	t.Run("returns no error on clean shutdown", func(t *testing.T) {
		app := New(nil)

		err := app.Shutdown()

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})
}

func TestApp_Adapters(t *testing.T) {
	t.Run("returns adapters reference", func(t *testing.T) {
		app := New(nil)

		adapters := app.Adapters()

		if adapters == nil {
			t.Error("adapters should not be nil")
		}
		if adapters != app.adapters {
			t.Error("should return same adapters reference")
		}
	})
}

func TestApp_Services(t *testing.T) {
	t.Run("returns services registry", func(t *testing.T) {
		app := New(nil)

		services := app.Services()

		if services == nil {
			t.Error("services should not be nil")
		}
	})
}

func TestApp_CustomServices(t *testing.T) {
	t.Run("returns custom services registry", func(t *testing.T) {
		app := New(nil)

		customServices := app.CustomServices()

		if customServices == nil {
			t.Error("custom services should not be nil")
		}
	})
}

// ============ Test Helpers ============

type testService struct {
	name string
}

// ============ Benchmarks ============

func BenchmarkNew(b *testing.B) {
	config := &Config{
		Name:       "bench-app",
		EnableHTTP: true,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		New(config)
	}
}

func BenchmarkNewContext(b *testing.B) {
	app := New(nil)
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		app.NewContext(ctx)
	}
}

func BenchmarkRegisterHandler(b *testing.B) {
	app := New(nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		app.RegisterHandler(func(ctx *ucontext.Context) error {
			return nil
		}).HTTP("GET", "/test").Done()
	}
}
