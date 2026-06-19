package unicorn

import (
	"context"
	"testing"

	"github.com/madcok-co/unicorn/core/pkg/app"
)

// ============ New ============

func TestNew_NonNilConfig(t *testing.T) {
	a := New(&Config{
		Name:    "test-app",
		Version: "1.0.0",
	})
	if a == nil {
		t.Fatal("New() with non-nil config returned nil")
	}
	if a.Name() != "test-app" {
		t.Errorf("expected Name 'test-app', got %q", a.Name())
	}
}

func TestNew_NilConfig(t *testing.T) {
	a := New(nil)
	if a == nil {
		t.Fatal("New(nil) returned nil")
	}
	// nil config should default to "unicorn-app"
	if a.Name() != "unicorn-app" {
		t.Errorf("expected default Name 'unicorn-app', got %q", a.Name())
	}
}

// ============ Version ============

func TestVersion(t *testing.T) {
	v := Version()
	if v != "0.1.0" {
		t.Errorf("expected Version '0.1.0', got %q", v)
	}
}

// ============ Service Registries ============

func TestNewServiceRegistry(t *testing.T) {
	sr := NewServiceRegistry()
	if sr == nil {
		t.Fatal("NewServiceRegistry() returned nil")
	}
}

func TestNewAdvancedServiceRegistry(t *testing.T) {
	asr := NewAdvancedServiceRegistry()
	if asr == nil {
		t.Fatal("NewAdvancedServiceRegistry() returned nil")
	}
}

// ============ GetService / MustGetService (generic) ============

// Greeter is a simple interface for testing generic service retrieval.
type Greeter interface {
	Greet() string
}

type testGreeter struct{}

func (g *testGreeter) Greet() string { return "hello from test" }

func TestGetService(t *testing.T) {
	a := New(nil)
	ctx := a.NewContext(context.Background())
	defer ctx.Release()

	greeter := &testGreeter{}
	ctx.RegisterService("greeter", greeter)

	g, err := GetService[Greeter](ctx, "greeter")
	if err != nil {
		t.Fatalf("GetService failed: %v", err)
	}
	if g == nil {
		t.Fatal("GetService returned nil greeter")
	}
	if g.Greet() != "hello from test" {
		t.Errorf("expected 'hello from test', got %q", g.Greet())
	}
}

func TestGetService_NotFound(t *testing.T) {
	a := New(nil)
	ctx := a.NewContext(context.Background())
	defer ctx.Release()

	_, err := GetService[Greeter](ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for missing service, got nil")
	}
}

func TestGetService_WrongType(t *testing.T) {
	a := New(nil)
	ctx := a.NewContext(context.Background())
	defer ctx.Release()

	// Register an int, not a Greeter
	ctx.RegisterService("greeter", 42)

	_, err := GetService[Greeter](ctx, "greeter")
	if err == nil {
		t.Error("expected type error, got nil")
	}
}

func TestMustGetService(t *testing.T) {
	a := New(nil)
	ctx := a.NewContext(context.Background())
	defer ctx.Release()

	greeter := &testGreeter{}
	ctx.RegisterService("greeter", greeter)

	g := MustGetService[Greeter](ctx, "greeter")
	if g == nil {
		t.Fatal("MustGetService returned nil greeter")
	}
	if g.Greet() != "hello from test" {
		t.Errorf("expected 'hello from test', got %q", g.Greet())
	}
}

func TestMustGetService_PanicsOnMissing(t *testing.T) {
	a := New(nil)
	ctx := a.NewContext(context.Background())
	defer ctx.Release()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for missing service")
		}
	}()
	MustGetService[Greeter](ctx, "nonexistent")
}

func TestMustGetService_PanicsOnWrongType(t *testing.T) {
	a := New(nil)
	ctx := a.NewContext(context.Background())
	defer ctx.Release()

	ctx.RegisterService("greeter", 42)

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for wrong service type")
		}
	}()
	MustGetService[Greeter](ctx, "greeter")
}

// ============ Infrastructure Constructor Non-Nil Checks ============

func TestInfrastructureConstructorsNonNull(t *testing.T) {
	// Database
	if NewStandardSQLDriver == nil {
		t.Error("NewStandardSQLDriver is nil")
	}
	if NewSimpleQueryBuilder == nil {
		t.Error("NewSimpleQueryBuilder is nil")
	}

	// Cache
	if NewMemoryCacheDriver == nil {
		t.Error("NewMemoryCacheDriver is nil")
	}

	// Logger
	if NewStdLoggerDriver == nil {
		t.Error("NewStdLoggerDriver is nil")
	}
	if NewMultiLogger == nil {
		t.Error("NewMultiLogger is nil")
	}

	// Cron
	if NewSimpleCronScheduler == nil {
		t.Error("NewSimpleCronScheduler is nil")
	}

	// Validator
	if NewSimpleValidator == nil {
		t.Error("NewSimpleValidator is nil")
	}

	// Metrics
	if NewMemoryMetricsDriver == nil {
		t.Error("NewMemoryMetricsDriver is nil")
	}

	// Tracer
	if NewMemoryTracerDriver == nil {
		t.Error("NewMemoryTracerDriver is nil")
	}
	if NewConsoleTracer == nil {
		t.Error("NewConsoleTracer is nil")
	}
}

// ============ Security Constructor Non-Nil Checks ============

func TestSecurityConstructorsNonNull(t *testing.T) {
	// JWT
	if NewJWTAuthenticator == nil {
		t.Error("NewJWTAuthenticator is nil")
	}

	// API Key
	if NewAPIKeyAuthenticator == nil {
		t.Error("NewAPIKeyAuthenticator is nil")
	}
	if NewInMemoryAPIKeyStore == nil {
		t.Error("NewInMemoryAPIKeyStore is nil")
	}

	// Rate Limiter
	if NewInMemoryRateLimiter == nil {
		t.Error("NewInMemoryRateLimiter is nil")
	}
	if NewSlidingWindowRateLimiter == nil {
		t.Error("NewSlidingWindowRateLimiter is nil")
	}

	// Encryptor
	if NewAESEncryptor == nil {
		t.Error("NewAESEncryptor is nil")
	}
	if NewAESEncryptorFromString == nil {
		t.Error("NewAESEncryptorFromString is nil")
	}

	// Hasher
	if NewBcryptHasher == nil {
		t.Error("NewBcryptHasher is nil")
	}
	if NewArgon2Hasher == nil {
		t.Error("NewArgon2Hasher is nil")
	}
	if NewMultiHasher == nil {
		t.Error("NewMultiHasher is nil")
	}

	// Secret Manager
	if NewEnvSecretManager == nil {
		t.Error("NewEnvSecretManager is nil")
	}

	// Audit Logger
	if NewInMemoryAuditLogger == nil {
		t.Error("NewInMemoryAuditLogger is nil")
	}
	if NewAuditEvent == nil {
		t.Error("NewAuditEvent is nil")
	}
	if NewCompositeAuditLogger == nil {
		t.Error("NewCompositeAuditLogger is nil")
	}
}

// ============ Log Level Constants ============

func TestLogLevelConstants(t *testing.T) {
	if LogLevelDebug != 0 {
		t.Errorf("expected LogLevelDebug=0, got %v", LogLevelDebug)
	}
	if LogLevelInfo != 1 {
		t.Errorf("expected LogLevelInfo=1, got %v", LogLevelInfo)
	}
	if LogLevelWarn != 2 {
		t.Errorf("expected LogLevelWarn=2, got %v", LogLevelWarn)
	}
	if LogLevelError != 3 {
		t.Errorf("expected LogLevelError=3, got %v", LogLevelError)
	}
}

// ============ Audit Action Constants ============

func TestAuditActionConstants(t *testing.T) {
	if AuditActionCreate != "create" {
		t.Errorf("expected AuditActionCreate='create', got %q", AuditActionCreate)
	}
	if AuditActionRead != "read" {
		t.Errorf("expected AuditActionRead='read', got %q", AuditActionRead)
	}
	if AuditActionUpdate != "update" {
		t.Errorf("expected AuditActionUpdate='update', got %q", AuditActionUpdate)
	}
	if AuditActionDelete != "delete" {
		t.Errorf("expected AuditActionDelete='delete', got %q", AuditActionDelete)
	}
	if AuditActionLogin != "login" {
		t.Errorf("expected AuditActionLogin='login', got %q", AuditActionLogin)
	}
	if AuditActionLogout != "logout" {
		t.Errorf("expected AuditActionLogout='logout', got %q", AuditActionLogout)
	}
}

// ============ Sidecar Constructor Non-Nil Checks ============

func TestSidecarConstructorsNonNull(t *testing.T) {
	if NewHTTPSidecar == nil {
		t.Error("NewHTTPSidecar is nil")
	}
	if NewBrokerSidecar == nil {
		t.Error("NewBrokerSidecar is nil")
	}
	if NewCronSidecar == nil {
		t.Error("NewCronSidecar is nil")
	}
}

// ============ Compile-Time Type Alias Checks ============

// These package-level variable declarations verify at compile time that
// type aliases resolve to the expected underlying types.

var _ *App = (*app.App)(nil)
var _ *Config = (*app.Config)(nil)
