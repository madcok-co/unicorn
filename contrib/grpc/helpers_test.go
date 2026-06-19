package grpc

import (
	"context"
	"errors"
	"testing"

	ucontext "github.com/madcok-co/unicorn/core/pkg/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// mockServerTransportStream implements grpc.ServerTransportStream for testing
// SetMetadata, which requires a server stream in the outgoing context.
type mockServerTransportStream struct {
	sentHeaders metadata.MD
}

func (m *mockServerTransportStream) Method() string                  { return "/test.Service/Method" }
func (m *mockServerTransportStream) SetHeader(md metadata.MD) error  { return nil }
func (m *mockServerTransportStream) SendHeader(md metadata.MD) error { m.sentHeaders = md; return nil }
func (m *mockServerTransportStream) SetTrailer(md metadata.MD) error { return nil }

// TestWrapHandler verifies that WrapHandler wraps a function and returns a HandlerFunc.
func TestWrapHandler(t *testing.T) {
	called := false

	fn := func(ctx *ucontext.Context, req interface{}) (interface{}, error) {
		called = true
		return "result", nil
	}

	wrapped := WrapHandler(fn)
	if wrapped == nil {
		t.Fatal("expected non-nil wrapped handler")
	}

	resp, err := wrapped(context.Background(), "input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected handler to be called")
	}
	if resp != "result" {
		t.Errorf("expected result, got %v", resp)
	}
}

// TestWrapHandler_PreservesError verifies WrapHandler propagates errors.
func TestWrapHandler_PreservesError(t *testing.T) {
	expectedErr := errors.New("handler error")
	fn := func(ctx *ucontext.Context, req interface{}) (interface{}, error) {
		return nil, expectedErr
	}

	wrapped := WrapHandler(fn)
	_, err := wrapped(context.Background(), "input")
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

// TestWrapHandler_WithIncomingMetadata verifies WrapHandler extracts incoming metadata.
func TestWrapHandler_WithIncomingMetadata(t *testing.T) {
	var capturedCtx *ucontext.Context
	fn := func(ctx *ucontext.Context, req interface{}) (interface{}, error) {
		capturedCtx = ctx
		return "ok", nil
	}

	wrapped := WrapHandler(fn)

	// Create context with incoming metadata
	md := metadata.Pairs("x-custom", "custom-value", "authorization", "bearer token")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err := wrapped(ctx, "input")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedCtx == nil {
		t.Fatal("expected unicorn context to be captured")
	}

	req := capturedCtx.Request()
	if req == nil {
		t.Fatal("expected request to be set")
	}

	if req.Method != "RPC" {
		t.Errorf("expected Method=RPC, got %s", req.Method)
	}
	if req.TriggerType != "grpc" {
		t.Errorf("expected TriggerType=grpc, got %s", req.TriggerType)
	}
	if req.Headers["x-custom"] != "custom-value" {
		t.Errorf("expected x-custom header, got %v", req.Headers)
	}
	if req.Headers["authorization"] != "bearer token" {
		t.Errorf("expected authorization header, got %v", req.Headers)
	}
}

// TestSetMetadata verifies SetMetadata works with an outgoing context.
func TestSetMetadata(t *testing.T) {
	mockStream := &mockServerTransportStream{}
	ctx := grpc.NewContextWithServerTransportStream(context.Background(), mockStream)

	err := SetMetadata(ctx, "response-key", "response-value")
	if err != nil {
		t.Fatalf("unexpected error from SetMetadata: %v", err)
	}

	if mockStream.sentHeaders == nil {
		t.Fatal("expected headers to be sent")
	}
	if mockStream.sentHeaders.Get("response-key")[0] != "response-value" {
		t.Errorf("expected response-key=response-value, got %v", mockStream.sentHeaders)
	}
}

// TestSetMetadata_NoStream verifies SetMetadata returns an error when there
// is no server transport stream in the context.
func TestSetMetadata_NoStream(t *testing.T) {
	err := SetMetadata(context.Background(), "key", "value")
	if err == nil {
		t.Error("expected error when setting metadata without server stream")
	}
}

// TestGetMetadata_Present verifies GetMetadata returns a present value.
func TestGetMetadata_Present(t *testing.T) {
	md := metadata.Pairs("x-key", "my-value")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	val, ok := GetMetadata(ctx, "x-key")
	if !ok {
		t.Error("expected key to be present")
	}
	if val != "my-value" {
		t.Errorf("expected my-value, got %s", val)
	}
}

// TestGetMetadata_Missing verifies GetMetadata returns false for missing key.
func TestGetMetadata_Missing(t *testing.T) {
	md := metadata.Pairs("x-key", "my-value")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, ok := GetMetadata(ctx, "nonexistent")
	if ok {
		t.Error("expected key to be missing")
	}
}

// TestGetMetadata_NoMetadata verifies GetMetadata returns false when no metadata.
func TestGetMetadata_NoMetadata(t *testing.T) {
	_, ok := GetMetadata(context.Background(), "any-key")
	if ok {
		t.Error("expected false when no metadata in context")
	}
}

// TestGetMetadata_CaseInsensitive verifies gRPC metadata is case-insensitive.
func TestGetMetadata_CaseInsensitive(t *testing.T) {
	md := metadata.Pairs("X-Custom-Header", "value")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	val, ok := GetMetadata(ctx, "x-custom-header")
	if !ok {
		t.Error("expected key to be present (case-insensitive)")
	}
	if val != "value" {
		t.Errorf("expected value, got %s", val)
	}
}

// TestGetAllMetadata verifies GetAllMetadata returns all key-value pairs.
func TestGetAllMetadata(t *testing.T) {
	md := metadata.Pairs(
		"x-key-1", "value-1",
		"x-key-2", "value-2",
		"x-key-3", "value-3",
	)
	ctx := metadata.NewIncomingContext(context.Background(), md)

	result := GetAllMetadata(ctx)
	if len(result) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(result))
	}
	if result["x-key-1"] != "value-1" {
		t.Errorf("expected x-key-1=value-1, got %s", result["x-key-1"])
	}
	if result["x-key-2"] != "value-2" {
		t.Errorf("expected x-key-2=value-2, got %s", result["x-key-2"])
	}
	if result["x-key-3"] != "value-3" {
		t.Errorf("expected x-key-3=value-3, got %s", result["x-key-3"])
	}
}

// TestGetAllMetadata_NoMetadata verifies GetAllMetadata returns empty map.
func TestGetAllMetadata_NoMetadata(t *testing.T) {
	result := GetAllMetadata(context.Background())
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

// TestGetAllMetadata_MultipleValues verifies GetAllMetadata takes first value
// when a key has multiple values.
func TestGetAllMetadata_MultipleValues(t *testing.T) {
	md := metadata.MD{
		"x-multi": []string{"first", "second", "third"},
	}
	ctx := metadata.NewIncomingContext(context.Background(), md)

	result := GetAllMetadata(ctx)
	if result["x-multi"] != "first" {
		t.Errorf("expected first value 'first', got %s", result["x-multi"])
	}
}
