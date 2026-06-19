package grpc

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ============================================================================
// Mocks
// ============================================================================

// mockLogger captures Info and Error calls for assertion.
type mockLogger struct {
	infoCalls  []mockLogCall
	errorCalls []mockLogCall
}

type mockLogCall struct {
	msg    string
	fields []interface{}
}

func (m *mockLogger) Info(msg string, fields ...interface{}) {
	m.infoCalls = append(m.infoCalls, mockLogCall{msg: msg, fields: fields})
}

func (m *mockLogger) Error(msg string, fields ...interface{}) {
	m.errorCalls = append(m.errorCalls, mockLogCall{msg: msg, fields: fields})
}

// mockMetrics captures IncrementCounter and RecordHistogram calls.
type mockMetrics struct {
	counterCalls  []mockCounterCall
	histogramCall *mockHistogramCall // last call only for simplicity
}

type mockCounterCall struct {
	name   string
	labels map[string]string
}

type mockHistogramCall struct {
	name   string
	value  float64
	labels map[string]string
}

func (m *mockMetrics) IncrementCounter(name string, labels map[string]string) {
	m.counterCalls = append(m.counterCalls, mockCounterCall{name: name, labels: labels})
}

func (m *mockMetrics) RecordHistogram(name string, value float64, labels map[string]string) {
	m.histogramCall = &mockHistogramCall{name: name, value: value, labels: labels}
}

// mockLimiter implements the Allow() interface for rate limiting tests.
type mockLimiter struct {
	allowed bool
}

func (m *mockLimiter) Allow() bool { return m.allowed }

// ============================================================================
// Helpers
// ============================================================================

// testServerInfo returns a standard *grpc.UnaryServerInfo for testing.
func testServerInfo() *grpc.UnaryServerInfo {
	return &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}
}

// simpleHandler returns a grpc.UnaryHandler that returns the given values.
func simpleHandler(resp interface{}, err error) grpc.UnaryHandler {
	return func(ctx context.Context, req interface{}) (interface{}, error) {
		return resp, err
	}
}

// panickingHandler returns a grpc.UnaryHandler that panics with the given value.
func panickingHandler(v interface{}) grpc.UnaryHandler {
	return func(ctx context.Context, req interface{}) (interface{}, error) {
		panic(v)
	}
}

// ============================================================================
// LoggingInterceptor Tests
// ============================================================================

func TestLoggingInterceptor_Success(t *testing.T) {
	logger := &mockLogger{}
	interceptor := LoggingInterceptor(logger)

	handler := simpleHandler("response", nil)
	ctx := context.Background()

	resp, err := interceptor(ctx, "request", testServerInfo(), handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "response" {
		t.Errorf("expected response, got %v", resp)
	}

	if len(logger.infoCalls) != 1 {
		t.Fatalf("expected 1 Info call, got %d", len(logger.infoCalls))
	}
	if logger.infoCalls[0].msg != "gRPC request completed" {
		t.Errorf("expected 'gRPC request completed', got %s", logger.infoCalls[0].msg)
	}

	if len(logger.errorCalls) != 0 {
		t.Errorf("expected 0 Error calls, got %d", len(logger.errorCalls))
	}

	// Verify fields contain method and duration
	foundMethod := false
	foundDuration := false
	for i := 0; i < len(logger.infoCalls[0].fields); i += 2 {
		key, _ := logger.infoCalls[0].fields[i].(string)
		if key == "method" {
			foundMethod = true
		}
		if key == "duration" {
			foundDuration = true
		}
	}
	if !foundMethod {
		t.Error("expected 'method' field in Info log")
	}
	if !foundDuration {
		t.Error("expected 'duration' field in Info log")
	}
}

func TestLoggingInterceptor_Error(t *testing.T) {
	logger := &mockLogger{}
	interceptor := LoggingInterceptor(logger)

	testErr := errors.New("test failure")
	handler := simpleHandler(nil, testErr)
	ctx := context.Background()

	resp, err := interceptor(ctx, "request", testServerInfo(), handler)
	if err != testErr {
		t.Errorf("expected test failure error, got %v", err)
	}
	if resp != nil {
		t.Errorf("expected nil response, got %v", resp)
	}

	if len(logger.errorCalls) != 1 {
		t.Fatalf("expected 1 Error call, got %d", len(logger.errorCalls))
	}
	if logger.errorCalls[0].msg != "gRPC request failed" {
		t.Errorf("expected 'gRPC request failed', got %s", logger.errorCalls[0].msg)
	}

	if len(logger.infoCalls) != 0 {
		t.Errorf("expected 0 Info calls, got %d", len(logger.infoCalls))
	}

	// Verify error field is present
	foundError := false
	for i := 0; i < len(logger.errorCalls[0].fields); i += 2 {
		key, _ := logger.errorCalls[0].fields[i].(string)
		if key == "error" {
			foundError = true
		}
	}
	if !foundError {
		t.Error("expected 'error' field in Error log")
	}
}

// ============================================================================
// RecoveryInterceptor Tests
// ============================================================================

func TestRecoveryInterceptor_NoPanic(t *testing.T) {
	interceptor := RecoveryInterceptor()
	handler := simpleHandler("ok", nil)
	ctx := context.Background()

	resp, err := interceptor(ctx, "request", testServerInfo(), handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "ok" {
		t.Errorf("expected ok, got %v", resp)
	}
}

func TestRecoveryInterceptor_Panic(t *testing.T) {
	interceptor := RecoveryInterceptor()
	handler := panickingHandler("something went wrong")
	ctx := context.Background()

	resp, err := interceptor(ctx, "request", testServerInfo(), handler)
	if resp != nil {
		t.Errorf("expected nil response after panic, got %v", resp)
	}
	if err == nil {
		t.Fatal("expected error after panic")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("expected status error")
	}
	if st.Code() != codes.Internal {
		t.Errorf("expected Internal code, got %v", st.Code())
	}
	if !strings.Contains(st.Message(), "panic recovered") {
		t.Errorf("expected 'panic recovered' in message, got %s", st.Message())
	}
	if !strings.Contains(st.Message(), "something went wrong") {
		t.Errorf("expected panic value in message, got %s", st.Message())
	}
}

func TestRecoveryInterceptor_PanicNil(t *testing.T) {
	interceptor := RecoveryInterceptor()
	handler := panickingHandler(nil)
	ctx := context.Background()

	resp, err := interceptor(ctx, "request", testServerInfo(), handler)
	if resp != nil {
		t.Errorf("expected nil response, got %v", resp)
	}
	if err == nil {
		t.Fatal("expected error after nil panic")
	}

	st, _ := status.FromError(err)
	if st.Code() != codes.Internal {
		t.Errorf("expected Internal code, got %v", st.Code())
	}
}

func TestRecoveryInterceptor_ErrorPassthrough(t *testing.T) {
	interceptor := RecoveryInterceptor()
	testErr := errors.New("normal error")
	handler := simpleHandler(nil, testErr)
	ctx := context.Background()

	_, err := interceptor(ctx, "request", testServerInfo(), handler)
	if err != testErr {
		t.Errorf("expected original error, got %v", err)
	}
}

// ============================================================================
// TimeoutInterceptor Tests
// ============================================================================

func TestTimeoutInterceptor_SetsDeadline(t *testing.T) {
	timeout := 100 * time.Millisecond
	interceptor := TimeoutInterceptor(timeout)

	var capturedCtx context.Context
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		capturedCtx = ctx
		return "ok", nil
	}

	ctx := context.Background()
	resp, err := interceptor(ctx, "request", testServerInfo(), handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "ok" {
		t.Errorf("expected ok, got %v", resp)
	}

	deadline, ok := capturedCtx.Deadline()
	if !ok {
		t.Error("expected deadline to be set")
	}
	// Deadline should be approximately now + timeout
	expected := time.Now().Add(timeout)
	diff := deadline.Sub(expected)
	if diff < -50*time.Millisecond || diff > 50*time.Millisecond {
		t.Errorf("deadline %v not within expected range of %v (diff: %v)", deadline, expected, diff)
	}
}

func TestTimeoutInterceptor_PropagatesError(t *testing.T) {
	interceptor := TimeoutInterceptor(1 * time.Second)
	testErr := errors.New("handler failed")
	handler := simpleHandler(nil, testErr)

	_, err := interceptor(context.Background(), "request", testServerInfo(), handler)
	if err != testErr {
		t.Errorf("expected original error, got %v", err)
	}
}

// ============================================================================
// AuthInterceptor Tests
// ============================================================================

func TestAuthInterceptor_Allowed(t *testing.T) {
	validator := func(ctx context.Context) error {
		return nil // authenticated
	}
	interceptor := AuthInterceptor(validator)
	handler := simpleHandler("data", nil)

	resp, err := interceptor(context.Background(), "request", testServerInfo(), handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "data" {
		t.Errorf("expected data, got %v", resp)
	}
}

func TestAuthInterceptor_Denied(t *testing.T) {
	validator := func(ctx context.Context) error {
		return errors.New("invalid token")
	}
	interceptor := AuthInterceptor(validator)
	handler := simpleHandler("data", nil)

	resp, err := interceptor(context.Background(), "request", testServerInfo(), handler)
	if resp != nil {
		t.Errorf("expected nil response, got %v", resp)
	}
	if err == nil {
		t.Fatal("expected error")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("expected status error")
	}
	if st.Code() != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated code, got %v", st.Code())
	}
	if st.Message() != "invalid token" {
		t.Errorf("expected 'invalid token' message, got %s", st.Message())
	}
}

func TestAuthInterceptor_DeniedDoesNotCallHandler(t *testing.T) {
	validator := func(ctx context.Context) error {
		return errors.New("blocked")
	}
	handlerCalled := false
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		handlerCalled = true
		return "data", nil
	}

	interceptor := AuthInterceptor(validator)
	_, err := interceptor(context.Background(), "request", testServerInfo(), handler)
	if err == nil {
		t.Fatal("expected error")
	}
	if handlerCalled {
		t.Error("handler should not be called when auth fails")
	}
}

// ============================================================================
// MetricsInterceptor Tests
// ============================================================================

func TestMetricsInterceptor_Success(t *testing.T) {
	mock := &mockMetrics{}
	interceptor := MetricsInterceptor(mock)
	handler := simpleHandler("response", nil)

	ctx := context.Background()
	resp, err := interceptor(ctx, "request", testServerInfo(), handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "response" {
		t.Errorf("expected response, got %v", resp)
	}

	if len(mock.counterCalls) != 1 {
		t.Fatalf("expected 1 IncrementCounter call, got %d", len(mock.counterCalls))
	}
	if mock.counterCalls[0].name != "grpc_requests_total" {
		t.Errorf("expected grpc_requests_total, got %s", mock.counterCalls[0].name)
	}
	if mock.counterCalls[0].labels["status"] != codes.OK.String() {
		t.Errorf("expected status=OK, got %s", mock.counterCalls[0].labels["status"])
	}
	if mock.counterCalls[0].labels["method"] != "/test.Service/Method" {
		t.Errorf("expected method=/test.Service/Method, got %s", mock.counterCalls[0].labels["method"])
	}

	if mock.histogramCall == nil {
		t.Fatal("expected RecordHistogram call")
	}
	if mock.histogramCall.name != "grpc_request_duration_seconds" {
		t.Errorf("expected grpc_request_duration_seconds, got %s", mock.histogramCall.name)
	}
	if mock.histogramCall.value < 0 {
		t.Errorf("expected non-negative duration, got %f", mock.histogramCall.value)
	}
	if mock.histogramCall.labels["status"] != codes.OK.String() {
		t.Errorf("expected status=OK, got %s", mock.histogramCall.labels["status"])
	}
}

func TestMetricsInterceptor_Error(t *testing.T) {
	mock := &mockMetrics{}
	testErr := status.Error(codes.InvalidArgument, "bad request")
	handler := simpleHandler(nil, testErr)

	interceptor := MetricsInterceptor(mock)
	_, err := interceptor(context.Background(), "request", testServerInfo(), handler)
	if err == nil {
		t.Fatal("expected error")
	}

	if len(mock.counterCalls) != 1 {
		t.Fatalf("expected 1 IncrementCounter call, got %d", len(mock.counterCalls))
	}
	if mock.counterCalls[0].labels["status"] != codes.InvalidArgument.String() {
		t.Errorf("expected status=InvalidArgument, got %s", mock.counterCalls[0].labels["status"])
	}

	if mock.histogramCall == nil {
		t.Fatal("expected RecordHistogram call")
	}
	if mock.histogramCall.labels["status"] != codes.InvalidArgument.String() {
		t.Errorf("expected status=InvalidArgument, got %s", mock.histogramCall.labels["status"])
	}
}

// ============================================================================
// RateLimitInterceptor Tests
// ============================================================================

func TestRateLimitInterceptor_Allowed(t *testing.T) {
	limiter := &mockLimiter{allowed: true}
	interceptor := RateLimitInterceptor(limiter)
	handler := simpleHandler("data", nil)

	resp, err := interceptor(context.Background(), "request", testServerInfo(), handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "data" {
		t.Errorf("expected data, got %v", resp)
	}
}

func TestRateLimitInterceptor_Denied(t *testing.T) {
	limiter := &mockLimiter{allowed: false}
	interceptor := RateLimitInterceptor(limiter)
	handler := simpleHandler("data", nil)

	resp, err := interceptor(context.Background(), "request", testServerInfo(), handler)
	if resp != nil {
		t.Errorf("expected nil response, got %v", resp)
	}
	if err == nil {
		t.Fatal("expected error")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("expected status error")
	}
	if st.Code() != codes.ResourceExhausted {
		t.Errorf("expected ResourceExhausted code, got %v", st.Code())
	}
	if st.Message() != "rate limit exceeded" {
		t.Errorf("expected 'rate limit exceeded', got %s", st.Message())
	}
}

func TestRateLimitInterceptor_DeniedDoesNotCallHandler(t *testing.T) {
	limiter := &mockLimiter{allowed: false}
	handlerCalled := false
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		handlerCalled = true
		return "data", nil
	}

	interceptor := RateLimitInterceptor(limiter)
	_, err := interceptor(context.Background(), "request", testServerInfo(), handler)
	if err == nil {
		t.Fatal("expected error")
	}
	if handlerCalled {
		t.Error("handler should not be called when rate limited")
	}
}
