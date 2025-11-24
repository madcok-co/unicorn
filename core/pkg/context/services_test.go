package context

import (
	"context"
	"errors"
	"sync"
	"testing"
)

// Mock services for testing
type MockEmailService interface {
	Send(to, subject, body string) error
}

type mockEmailServiceImpl struct {
	sendCalled bool
	lastTo     string
}

func (m *mockEmailServiceImpl) Send(to, subject, body string) error {
	m.sendCalled = true
	m.lastTo = to
	return nil
}

type MockPaymentService interface {
	Charge(amount float64) error
}

type mockPaymentServiceImpl struct {
	charged float64
}

func (m *mockPaymentServiceImpl) Charge(amount float64) error {
	m.charged = amount
	return nil
}

// Closable service for testing
type mockClosableService struct {
	closed bool
}

func (m *mockClosableService) Close() error {
	m.closed = true
	return nil
}

// HealthCheckable service
type mockHealthyService struct {
	healthy bool
}

func (m *mockHealthyService) Health() error {
	if !m.healthy {
		return errors.New("unhealthy")
	}
	return nil
}

// TestContextRegisterService tests basic service registration
func TestContextRegisterService(t *testing.T) {
	ctx := New(context.Background())

	emailSvc := &mockEmailServiceImpl{}
	ctx.RegisterService("email", emailSvc)

	// Verify service is registered
	if !ctx.HasService("email") {
		t.Error("expected email service to be registered")
	}

	// Verify we can retrieve it
	svc := ctx.GetService("email")
	if svc == nil {
		t.Error("expected to get email service")
	}

	// Verify it's the same instance
	retrieved, ok := svc.(*mockEmailServiceImpl)
	if !ok {
		t.Error("expected service to be *mockEmailServiceImpl")
	}
	if retrieved != emailSvc {
		t.Error("expected same instance")
	}
}

// TestContextGetServiceNotFound tests getting non-existent service
func TestContextGetServiceNotFound(t *testing.T) {
	ctx := New(context.Background())

	svc := ctx.GetService("nonexistent")
	if svc != nil {
		t.Error("expected nil for non-existent service")
	}

	if ctx.HasService("nonexistent") {
		t.Error("expected HasService to return false")
	}
}

// TestContextMustGetServicePanics tests panic on missing service
func TestContextMustGetServicePanics(t *testing.T) {
	ctx := New(context.Background())

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected MustGetService to panic")
		}
	}()

	ctx.MustGetService("nonexistent")
}

// TestContextServicesThreadSafety tests concurrent access
func TestContextServicesThreadSafety(t *testing.T) {
	ctx := New(context.Background())

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)

		// Writer
		go func(idx int) {
			defer wg.Done()
			svc := &mockEmailServiceImpl{}
			ctx.RegisterService("email", svc)
		}(i)

		// Reader
		go func(idx int) {
			defer wg.Done()
			ctx.GetService("email")
			ctx.HasService("email")
			ctx.Services()
		}(i)
	}

	wg.Wait()
}

// TestContextServicesList tests listing services
func TestContextServicesList(t *testing.T) {
	ctx := New(context.Background())

	ctx.RegisterService("email", &mockEmailServiceImpl{})
	ctx.RegisterService("payment", &mockPaymentServiceImpl{})

	services := ctx.Services()
	if len(services) != 2 {
		t.Errorf("expected 2 services, got %d", len(services))
	}

	// Check both are present
	found := make(map[string]bool)
	for _, name := range services {
		found[name] = true
	}

	if !found["email"] || !found["payment"] {
		t.Error("expected both email and payment services")
	}
}

// TestContextCopyServicesFrom tests copying services
func TestContextCopyServicesFrom(t *testing.T) {
	ctx1 := New(context.Background())
	ctx1.RegisterService("email", &mockEmailServiceImpl{})
	ctx1.RegisterService("payment", &mockPaymentServiceImpl{})

	ctx2 := New(context.Background())
	ctx2.CopyServicesFrom(ctx1)

	if !ctx2.HasService("email") || !ctx2.HasService("payment") {
		t.Error("expected services to be copied")
	}

	// Verify they're the same instances
	if ctx1.GetService("email") != ctx2.GetService("email") {
		t.Error("expected same email service instance")
	}
}

// TestGetServiceGeneric tests the generic GetService helper
func TestGetServiceGeneric(t *testing.T) {
	ctx := New(context.Background())

	emailSvc := &mockEmailServiceImpl{}
	ctx.RegisterService("email", emailSvc)

	// Test successful retrieval
	retrieved, err := GetService[MockEmailService](ctx, "email")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if retrieved == nil {
		t.Error("expected service to be retrieved")
	}

	// Test we can use it
	err = retrieved.Send("test@example.com", "Test", "Body")
	if err != nil {
		t.Errorf("unexpected error calling Send: %v", err)
	}
	if !emailSvc.sendCalled {
		t.Error("expected Send to be called")
	}
}

// TestGetServiceGenericNotFound tests GetService with missing service
func TestGetServiceGenericNotFound(t *testing.T) {
	ctx := New(context.Background())

	_, err := GetService[MockEmailService](ctx, "email")
	if err == nil {
		t.Error("expected error for missing service")
	}
}

// TestGetServiceGenericWrongType tests GetService with wrong type
func TestGetServiceGenericWrongType(t *testing.T) {
	ctx := New(context.Background())

	// Register email service
	ctx.RegisterService("email", &mockEmailServiceImpl{})

	// Try to get it as PaymentService
	_, err := GetService[MockPaymentService](ctx, "email")
	if err == nil {
		t.Error("expected error for wrong type")
	}
}

// TestMustGetServiceGeneric tests MustGetService helper
func TestMustGetServiceGeneric(t *testing.T) {
	ctx := New(context.Background())

	emailSvc := &mockEmailServiceImpl{}
	ctx.RegisterService("email", emailSvc)

	// Should not panic
	retrieved := MustGetService[MockEmailService](ctx, "email")
	if retrieved == nil {
		t.Error("expected service")
	}
}

// TestMustGetServiceGenericPanics tests MustGetService panic
func TestMustGetServiceGenericPanics(t *testing.T) {
	ctx := New(context.Background())

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic")
		}
	}()

	MustGetService[MockEmailService](ctx, "nonexistent")
}

// TestServiceRegistry tests simple service registry
func TestServiceRegistry(t *testing.T) {
	registry := NewServiceRegistry()

	emailSvc := &mockEmailServiceImpl{}
	registry.Register("email", emailSvc)

	if !registry.Has("email") {
		t.Error("expected email to be registered")
	}

	if registry.Get("email") != emailSvc {
		t.Error("expected same instance")
	}

	names := registry.Names()
	if len(names) != 1 || names[0] != "email" {
		t.Error("expected email in names")
	}
}

// TestServiceRegistryInjectInto tests injecting into context
func TestServiceRegistryInjectInto(t *testing.T) {
	registry := NewServiceRegistry()
	registry.Register("email", &mockEmailServiceImpl{})
	registry.Register("payment", &mockPaymentServiceImpl{})

	ctx := New(context.Background())
	registry.InjectInto(ctx)

	if !ctx.HasService("email") || !ctx.HasService("payment") {
		t.Error("expected services to be injected")
	}
}

// TestAdvancedServiceRegistrySingleton tests singleton registration
func TestAdvancedServiceRegistrySingleton(t *testing.T) {
	registry := NewAdvancedServiceRegistry()

	emailSvc := &mockEmailServiceImpl{}
	registry.RegisterSingleton("email", emailSvc)

	ctx := New(context.Background())
	err := registry.InjectInto(ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if ctx.GetService("email") != emailSvc {
		t.Error("expected same singleton instance")
	}
}

// TestAdvancedServiceRegistryFactory tests factory registration
func TestAdvancedServiceRegistryFactory(t *testing.T) {
	registry := NewAdvancedServiceRegistry()

	callCount := 0
	registry.RegisterFactory("email", func(ctx *Context) (any, error) {
		callCount++
		return &mockEmailServiceImpl{}, nil
	})

	// Inject into two contexts
	ctx1 := New(context.Background())
	registry.InjectInto(ctx1)

	ctx2 := New(context.Background())
	registry.InjectInto(ctx2)

	// Factory should be called twice
	if callCount != 2 {
		t.Errorf("expected factory to be called twice, got %d", callCount)
	}

	// Should be different instances
	if ctx1.GetService("email") == ctx2.GetService("email") {
		t.Error("expected different instances from factory")
	}
}

// TestAdvancedServiceRegistryFactoryError tests factory error handling
func TestAdvancedServiceRegistryFactoryError(t *testing.T) {
	registry := NewAdvancedServiceRegistry()

	registry.RegisterFactory("failing", func(ctx *Context) (any, error) {
		return nil, errors.New("factory failed")
	})

	ctx := New(context.Background())
	err := registry.InjectInto(ctx)
	if err == nil {
		t.Error("expected error from failing factory")
	}
}

// TestAdvancedServiceRegistryCloseAll tests closing services
func TestAdvancedServiceRegistryCloseAll(t *testing.T) {
	registry := NewAdvancedServiceRegistry()

	closable := &mockClosableService{}
	registry.RegisterSingleton("closable", closable)

	err := registry.CloseAll()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !closable.closed {
		t.Error("expected service to be closed")
	}
}

// TestAdvancedServiceRegistryHealthCheckAll tests health checks
func TestAdvancedServiceRegistryHealthCheckAll(t *testing.T) {
	registry := NewAdvancedServiceRegistry()

	healthy := &mockHealthyService{healthy: true}
	unhealthy := &mockHealthyService{healthy: false}

	registry.RegisterSingleton("healthy", healthy)
	registry.RegisterSingleton("unhealthy", unhealthy)

	results := registry.HealthCheckAll()

	if results["healthy"] != nil {
		t.Error("expected healthy service to pass")
	}

	if results["unhealthy"] == nil {
		t.Error("expected unhealthy service to fail")
	}
}

// TestIdentityInContext tests identity storage
func TestIdentityInContext(t *testing.T) {
	ctx := New(context.Background())

	// Initially nil
	if ctx.Identity() != nil {
		t.Error("expected nil identity initially")
	}

	// Normally we'd import contracts, but for this test we'll skip
	// as Identity is tested elsewhere
}
