package grpc

import (
	"context"
	"testing"

	ucontext "github.com/madcok-co/unicorn/core/pkg/context"
	"github.com/madcok-co/unicorn/core/pkg/contracts"
	"github.com/madcok-co/unicorn/core/pkg/handler"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// dummyUnaryInterceptor is a simple interceptor for testing UseUnaryInterceptor.
func dummyUnaryInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	return handler(ctx, req)
}

// dummyStreamInterceptor is a simple interceptor for testing UseStreamInterceptor.
func dummyStreamInterceptor(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	return handler(srv, ss)
}

// TestNew_DefaultConfig verifies that New with nil config applies defaults.
func TestNew_DefaultConfig(t *testing.T) {
	registry := handler.NewRegistry()
	adapter := New(registry, nil)

	if adapter.registry != registry {
		t.Error("expected registry to be set")
	}

	if adapter.config.Host != "0.0.0.0" {
		t.Errorf("expected Host=0.0.0.0, got %s", adapter.config.Host)
	}
	if adapter.config.Port != 9090 {
		t.Errorf("expected Port=9090, got %d", adapter.config.Port)
	}
	if adapter.config.MaxRecvMsgSize != 4<<20 {
		t.Errorf("expected MaxRecvMsgSize=4MB, got %d", adapter.config.MaxRecvMsgSize)
	}
	if adapter.config.MaxSendMsgSize != 4<<20 {
		t.Errorf("expected MaxSendMsgSize=4MB, got %d", adapter.config.MaxSendMsgSize)
	}
	if !adapter.config.EnableReflection {
		t.Error("expected EnableReflection=true")
	}

	if len(adapter.unaryInterceptors) != 0 {
		t.Error("expected no unary interceptors")
	}
	if len(adapter.streamInterceptors) != 0 {
		t.Error("expected no stream interceptors")
	}
	if len(adapter.serviceRegistrations) != 0 {
		t.Error("expected no service registrations")
	}
}

// TestNew_CustomConfig verifies that New with custom config uses it.
func TestNew_CustomConfig(t *testing.T) {
	registry := handler.NewRegistry()
	config := &Config{
		Host:             "127.0.0.1",
		Port:             50051,
		MaxRecvMsgSize:   8 << 20,
		MaxSendMsgSize:   8 << 20,
		EnableReflection: false,
	}

	adapter := New(registry, config)

	if adapter.config.Host != "127.0.0.1" {
		t.Errorf("expected Host=127.0.0.1, got %s", adapter.config.Host)
	}
	if adapter.config.Port != 50051 {
		t.Errorf("expected Port=50051, got %d", adapter.config.Port)
	}
	if adapter.config.EnableReflection {
		t.Error("expected EnableReflection=false")
	}
}

// TestUseUnaryInterceptor verifies interceptors are appended.
func TestUseUnaryInterceptor(t *testing.T) {
	registry := handler.NewRegistry()
	adapter := New(registry, nil)

	if len(adapter.unaryInterceptors) != 0 {
		t.Error("expected 0 interceptors initially")
	}

	adapter.UseUnaryInterceptor(dummyUnaryInterceptor)
	if len(adapter.unaryInterceptors) != 1 {
		t.Errorf("expected 1 interceptor, got %d", len(adapter.unaryInterceptors))
	}

	adapter.UseUnaryInterceptor(dummyUnaryInterceptor)
	if len(adapter.unaryInterceptors) != 2 {
		t.Errorf("expected 2 interceptors, got %d", len(adapter.unaryInterceptors))
	}
}

// TestUseStreamInterceptor verifies stream interceptors are appended.
func TestUseStreamInterceptor(t *testing.T) {
	registry := handler.NewRegistry()
	adapter := New(registry, nil)

	if len(adapter.streamInterceptors) != 0 {
		t.Error("expected 0 stream interceptors initially")
	}

	adapter.UseStreamInterceptor(dummyStreamInterceptor)
	if len(adapter.streamInterceptors) != 1 {
		t.Errorf("expected 1 stream interceptor, got %d", len(adapter.streamInterceptors))
	}

	adapter.UseStreamInterceptor(dummyStreamInterceptor)
	if len(adapter.streamInterceptors) != 2 {
		t.Errorf("expected 2 stream interceptors, got %d", len(adapter.streamInterceptors))
	}
}

// TestRegisterService verifies service registrations are recorded.
func TestRegisterService(t *testing.T) {
	registry := handler.NewRegistry()
	adapter := New(registry, nil)

	if len(adapter.serviceRegistrations) != 0 {
		t.Error("expected 0 service registrations initially")
	}

	desc := &grpc.ServiceDesc{ServiceName: "test.Service"}
	impl := &struct{}{}
	adapter.RegisterService(desc, impl)

	if len(adapter.serviceRegistrations) != 1 {
		t.Fatalf("expected 1 registration, got %d", len(adapter.serviceRegistrations))
	}

	reg := adapter.serviceRegistrations[0]
	if reg.Desc.ServiceName != "test.Service" {
		t.Errorf("expected ServiceName=test.Service, got %s", reg.Desc.ServiceName)
	}
	if reg.Handler != impl {
		t.Error("expected handler to be the implementation")
	}
}

// TestAddress verifies the Address method returns "host:port".
func TestAddress(t *testing.T) {
	registry := handler.NewRegistry()

	// Default config
	adapter := New(registry, nil)
	if adapter.Address() != "0.0.0.0:9090" {
		t.Errorf("expected 0.0.0.0:9090, got %s", adapter.Address())
	}

	// Custom config
	config := &Config{Host: "10.0.0.1", Port: 1234}
	adapter = New(registry, config)
	if adapter.Address() != "10.0.0.1:1234" {
		t.Errorf("expected 10.0.0.1:1234, got %s", adapter.Address())
	}
}

// TestIsTLS verifies TLS detection.
func TestIsTLS(t *testing.T) {
	registry := handler.NewRegistry()

	// No TLS
	adapter := New(registry, nil)
	if adapter.IsTLS() {
		t.Error("expected IsTLS=false when no TLS config")
	}

	// TLS not enabled
	config := &Config{TLS: &contracts.TLSConfig{Enabled: false}}
	adapter = New(registry, config)
	if adapter.IsTLS() {
		t.Error("expected IsTLS=false when TLS.Enabled=false")
	}

	// TLS enabled
	config = &Config{TLS: &contracts.TLSConfig{Enabled: true}}
	adapter = New(registry, config)
	if !adapter.IsTLS() {
		t.Error("expected IsTLS=true when TLS.Enabled=true")
	}
}

// TestShutdown_NilServer verifies Shutdown with nil server returns nil.
func TestShutdown_NilServer(t *testing.T) {
	registry := handler.NewRegistry()
	adapter := New(registry, nil)

	ctx := context.Background()
	if err := adapter.Shutdown(ctx); err != nil {
		t.Errorf("expected nil error for nil server, got %v", err)
	}
}

// TestExtractMetadata verifies extraction of metadata from unicorn context.
func TestExtractMetadata(t *testing.T) {
	// Context without response headers
	ctx := ucontext.New(context.Background())
	md := ExtractMetadata(ctx)
	if len(md) != 0 {
		t.Errorf("expected empty metadata, got %d entries", len(md))
	}

	// Context with response headers
	ctx2 := ucontext.New(context.Background())
	ctx2.Response().Headers = map[string]string{
		"x-request-id": "abc123",
		"x-trace-id":   "trace-456",
	}
	md = ExtractMetadata(ctx2)
	if len(md) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(md))
	}
	if len(md["x-request-id"]) != 1 || md["x-request-id"][0] != "abc123" {
		t.Errorf("expected x-request-id=abc123, got %v", md["x-request-id"])
	}
	if len(md["x-trace-id"]) != 1 || md["x-trace-id"][0] != "trace-456" {
		t.Errorf("expected x-trace-id=trace-456, got %v", md["x-trace-id"])
	}
}

// TestCreateUnaryHandler verifies that CreateUnaryHandler returns a grpc.UnaryHandler.
func TestCreateUnaryHandler(t *testing.T) {
	registry := handler.NewRegistry()
	adapter := New(registry, nil)

	// Create a simple handler that echoes its request.
	fn := func(ctx *ucontext.Context, req string) (string, error) {
		return req, nil
	}
	h := handler.New(fn).Named("test_handler")

	unaryHandler := adapter.CreateUnaryHandler(h)
	if unaryHandler == nil {
		t.Fatal("expected non-nil UnaryHandler")
	}

	// Invoke the unary handler with a basic context and request.
	// The handler expects a JSON-deserializable body in the unicorn request,
	// so we use incoming metadata to set up the unicorn request properly.
	md := metadata.Pairs("content-type", "application/json")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	resp, err := unaryHandler(ctx, `"hello"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}

	// The handler echoes the request. The executor sets resp.Body.
	// But note: the executor uses JSON unmarshalling of the request body (from
	// unicorn context) into the request type, and our raw req `"hello"` is a
	// JSON string. The response body should be the original `req` passed through.
	// However, the actual req value passed by CreateUnaryHandler to the executor
	// is set via uCtx.Set("grpc.request", req), while the body comes from
	// metadata. The handler fn receives the deserialized request from
	// ctx.Request().Body. Since there's no body in our incoming metadata,
	// the handler receives an empty string and echoes it back.
}

// TestCreateUnaryHandler_ReturnsUnaryHandlerType verifies the return type.
func TestCreateUnaryHandler_ReturnsUnaryHandlerType(t *testing.T) {
	registry := handler.NewRegistry()
	adapter := New(registry, nil)

	fn := func(ctx *ucontext.Context, req string) (string, error) {
		return "ok", nil
	}
	h := handler.New(fn)

	handler := adapter.CreateUnaryHandler(h)

	// Verify it's callable as a grpc.UnaryHandler
	var _ grpc.UnaryHandler = handler
	_ = handler
}
