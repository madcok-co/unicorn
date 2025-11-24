// Package tracer provides a generic tracer adapter
// that wraps any tracing library (OpenTelemetry, Jaeger, Zipkin, etc.)
package tracer

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

// Driver is the interface that any tracer must implement
type Driver interface {
	// StartSpan starts a new span
	StartSpan(ctx context.Context, name string, opts ...contracts.SpanOption) (context.Context, SpanDriver)
	// Extract extracts span context from carrier
	Extract(ctx context.Context, carrier contracts.Carrier) context.Context
	// Inject injects span context into carrier
	Inject(ctx context.Context, carrier contracts.Carrier) error
	// Close flushes and closes the tracer
	Close() error
}

// SpanDriver is the interface for spans
type SpanDriver interface {
	End()
	SetName(name string)
	SetStatus(code contracts.SpanStatus, message string)
	SetAttributes(attrs ...contracts.Attribute)
	AddEvent(name string, attrs ...contracts.Attribute)
	RecordError(err error)
	SpanContext() contracts.SpanContext
	IsRecording() bool
}

// Adapter implements contracts.Tracer
type Adapter struct {
	driver      Driver
	serviceName string
}

// New creates a new tracer adapter
func New(driver Driver) *Adapter {
	return &Adapter{
		driver: driver,
	}
}

// WithServiceName sets the service name
func (a *Adapter) WithServiceName(name string) *Adapter {
	a.serviceName = name
	return a
}

// ============ contracts.Tracer Implementation ============

func (a *Adapter) Start(ctx context.Context, name string, opts ...contracts.SpanOption) (context.Context, contracts.Span) {
	newCtx, spanDriver := a.driver.StartSpan(ctx, name, opts...)
	return newCtx, &spanWrapper{driver: spanDriver}
}

func (a *Adapter) Extract(ctx context.Context, carrier contracts.Carrier) context.Context {
	return a.driver.Extract(ctx, carrier)
}

func (a *Adapter) Inject(ctx context.Context, carrier contracts.Carrier) error {
	return a.driver.Inject(ctx, carrier)
}

func (a *Adapter) Close() error {
	return a.driver.Close()
}

// spanWrapper wraps SpanDriver to implement contracts.Span
type spanWrapper struct {
	driver SpanDriver
}

func (s *spanWrapper) End() {
	s.driver.End()
}

func (s *spanWrapper) SetName(name string) {
	s.driver.SetName(name)
}

func (s *spanWrapper) SetStatus(code contracts.SpanStatus, message string) {
	s.driver.SetStatus(code, message)
}

func (s *spanWrapper) SetAttributes(attrs ...contracts.Attribute) {
	s.driver.SetAttributes(attrs...)
}

func (s *spanWrapper) AddEvent(name string, attrs ...contracts.Attribute) {
	s.driver.AddEvent(name, attrs...)
}

func (s *spanWrapper) RecordError(err error) {
	s.driver.RecordError(err)
}

func (s *spanWrapper) SpanContext() contracts.SpanContext {
	return s.driver.SpanContext()
}

func (s *spanWrapper) IsRecording() bool {
	return s.driver.IsRecording()
}

// ============ In-Memory Tracer Driver ============

// MemoryDriver stores traces in memory (useful for testing)
type MemoryDriver struct {
	spans []*MemorySpan
	mu    sync.RWMutex
	idGen uint64
}

// MemorySpan represents a span stored in memory
type MemorySpan struct {
	Name       string
	TraceID    string
	SpanID     string
	ParentID   string
	StartTime  time.Time
	EndTime    time.Time
	Status     contracts.SpanStatus
	StatusMsg  string
	Attributes []contracts.Attribute
	Events     []SpanEvent
	Errors     []error
	mu         sync.Mutex
}

// SpanEvent represents an event on a span
type SpanEvent struct {
	Name       string
	Time       time.Time
	Attributes []contracts.Attribute
}

// NewMemoryDriver creates an in-memory tracer
func NewMemoryDriver() *MemoryDriver {
	return &MemoryDriver{
		spans: make([]*MemorySpan, 0),
	}
}

func (d *MemoryDriver) StartSpan(ctx context.Context, name string, opts ...contracts.SpanOption) (context.Context, SpanDriver) {
	id := atomic.AddUint64(&d.idGen, 1)

	span := &MemorySpan{
		Name:       name,
		TraceID:    fmt.Sprintf("trace-%d", id),
		SpanID:     fmt.Sprintf("span-%d", id),
		StartTime:  time.Now(),
		Attributes: make([]contracts.Attribute, 0),
		Events:     make([]SpanEvent, 0),
		Errors:     make([]error, 0),
	}

	// Check for parent span
	if parentSpan := SpanFromContext(ctx); parentSpan != nil {
		span.TraceID = parentSpan.TraceID
		span.ParentID = parentSpan.SpanID
	}

	// Apply options
	cfg := &contracts.SpanConfig{}
	for _, opt := range opts {
		opt(cfg)
	}
	span.Attributes = append(span.Attributes, cfg.Attributes...)

	d.mu.Lock()
	d.spans = append(d.spans, span)
	d.mu.Unlock()

	return ContextWithSpan(ctx, span), span
}

func (d *MemoryDriver) Extract(ctx context.Context, carrier contracts.Carrier) context.Context {
	traceID := carrier.Get("traceparent")
	if traceID == "" {
		traceID = carrier.Get("X-Trace-ID")
	}

	if traceID != "" {
		span := &MemorySpan{
			TraceID: traceID,
			SpanID:  carrier.Get("X-Span-ID"),
		}
		return ContextWithSpan(ctx, span)
	}

	return ctx
}

func (d *MemoryDriver) Inject(ctx context.Context, carrier contracts.Carrier) error {
	span := SpanFromContext(ctx)
	if span != nil {
		carrier.Set("traceparent", span.TraceID)
		carrier.Set("X-Trace-ID", span.TraceID)
		carrier.Set("X-Span-ID", span.SpanID)
	}
	return nil
}

func (d *MemoryDriver) Close() error {
	return nil
}

// GetSpans returns all recorded spans (for testing)
func (d *MemoryDriver) GetSpans() []*MemorySpan {
	d.mu.RLock()
	defer d.mu.RUnlock()
	result := make([]*MemorySpan, len(d.spans))
	copy(result, d.spans)
	return result
}

// Clear clears all recorded spans
func (d *MemoryDriver) Clear() {
	d.mu.Lock()
	d.spans = make([]*MemorySpan, 0)
	d.mu.Unlock()
}

// MemorySpan implements SpanDriver
func (s *MemorySpan) End() {
	s.mu.Lock()
	s.EndTime = time.Now()
	s.mu.Unlock()
}

func (s *MemorySpan) SetName(name string) {
	s.mu.Lock()
	s.Name = name
	s.mu.Unlock()
}

func (s *MemorySpan) SetStatus(code contracts.SpanStatus, message string) {
	s.mu.Lock()
	s.Status = code
	s.StatusMsg = message
	s.mu.Unlock()
}

func (s *MemorySpan) SetAttributes(attrs ...contracts.Attribute) {
	s.mu.Lock()
	s.Attributes = append(s.Attributes, attrs...)
	s.mu.Unlock()
}

func (s *MemorySpan) AddEvent(name string, attrs ...contracts.Attribute) {
	s.mu.Lock()
	s.Events = append(s.Events, SpanEvent{
		Name:       name,
		Time:       time.Now(),
		Attributes: attrs,
	})
	s.mu.Unlock()
}

func (s *MemorySpan) RecordError(err error) {
	s.mu.Lock()
	s.Errors = append(s.Errors, err)
	s.Status = contracts.SpanStatusError
	s.mu.Unlock()
}

func (s *MemorySpan) SpanContext() contracts.SpanContext {
	return contracts.SpanContext{
		TraceID: s.TraceID,
		SpanID:  s.SpanID,
	}
}

func (s *MemorySpan) IsRecording() bool {
	return true
}

// Context key for spans
type spanContextKey struct{}

// ContextWithSpan returns a new context with span
func ContextWithSpan(ctx context.Context, span *MemorySpan) context.Context {
	return context.WithValue(ctx, spanContextKey{}, span)
}

// SpanFromContext extracts span from context
func SpanFromContext(ctx context.Context) *MemorySpan {
	span, _ := ctx.Value(spanContextKey{}).(*MemorySpan)
	return span
}

// ============ Noop Driver ============

// NoopDriver discards all traces
type NoopDriver struct{}

// NewNoopDriver creates a no-op driver
func NewNoopDriver() *NoopDriver {
	return &NoopDriver{}
}

func (d *NoopDriver) StartSpan(ctx context.Context, name string, opts ...contracts.SpanOption) (context.Context, SpanDriver) {
	return ctx, &noopSpan{}
}

func (d *NoopDriver) Extract(ctx context.Context, carrier contracts.Carrier) context.Context {
	return ctx
}

func (d *NoopDriver) Inject(ctx context.Context, carrier contracts.Carrier) error {
	return nil
}

func (d *NoopDriver) Close() error {
	return nil
}

type noopSpan struct{}

func (s *noopSpan) End()                                    {}
func (s *noopSpan) SetName(string)                          {}
func (s *noopSpan) SetStatus(contracts.SpanStatus, string)  {}
func (s *noopSpan) SetAttributes(...contracts.Attribute)    {}
func (s *noopSpan) AddEvent(string, ...contracts.Attribute) {}
func (s *noopSpan) RecordError(error)                       {}
func (s *noopSpan) SpanContext() contracts.SpanContext      { return contracts.SpanContext{} }
func (s *noopSpan) IsRecording() bool                       { return false }

// ============ OpenTelemetry Wrapper ============

// OTelTracer is the interface that OpenTelemetry tracers implement
type OTelTracer interface {
	Start(ctx context.Context, spanName string, opts ...any) (context.Context, OTelSpan)
}

// OTelSpan is the interface for OpenTelemetry spans
type OTelSpan interface {
	End(options ...any)
	SetName(name string)
	SetStatus(code int, description string)
	SetAttributes(kv ...any)
	AddEvent(name string, options ...any)
	RecordError(err error, options ...any)
	SpanContext() OTelSpanContext
	IsRecording() bool
}

// OTelSpanContext is the interface for span context
type OTelSpanContext interface {
	TraceID() [16]byte
	SpanID() [8]byte
	TraceFlags() byte
	TraceState() string
	IsRemote() bool
	IsValid() bool
}

// OTelPropagator is the interface for context propagation
type OTelPropagator interface {
	Extract(ctx context.Context, carrier any) context.Context
	Inject(ctx context.Context, carrier any)
}

// OTelDriver wraps OpenTelemetry tracer
type OTelDriver struct {
	tracer     OTelTracer
	propagator OTelPropagator
}

// WrapOTel wraps an OpenTelemetry tracer
// Usage:
//
//	import "go.opentelemetry.io/otel"
//	tracer := otel.Tracer("my-service")
//	propagator := otel.GetTextMapPropagator()
//	driver := WrapOTel(tracer, propagator)
//	adapter := New(driver)
func WrapOTel(tracer OTelTracer, propagator OTelPropagator) *OTelDriver {
	return &OTelDriver{
		tracer:     tracer,
		propagator: propagator,
	}
}

func (d *OTelDriver) StartSpan(ctx context.Context, name string, opts ...contracts.SpanOption) (context.Context, SpanDriver) {
	newCtx, span := d.tracer.Start(ctx, name)
	return newCtx, &otelSpanWrapper{span: span}
}

func (d *OTelDriver) Extract(ctx context.Context, carrier contracts.Carrier) context.Context {
	if d.propagator != nil {
		return d.propagator.Extract(ctx, carrier)
	}
	return ctx
}

func (d *OTelDriver) Inject(ctx context.Context, carrier contracts.Carrier) error {
	if d.propagator != nil {
		d.propagator.Inject(ctx, carrier)
	}
	return nil
}

func (d *OTelDriver) Close() error {
	return nil
}

type otelSpanWrapper struct {
	span OTelSpan
}

func (s *otelSpanWrapper) End() {
	s.span.End()
}

func (s *otelSpanWrapper) SetName(name string) {
	s.span.SetName(name)
}

func (s *otelSpanWrapper) SetStatus(code contracts.SpanStatus, message string) {
	s.span.SetStatus(int(code), message)
}

func (s *otelSpanWrapper) SetAttributes(attrs ...contracts.Attribute) {
	// Convert to OTel attributes - implementation depends on OTel version
	kvs := make([]any, 0, len(attrs)*2)
	for _, attr := range attrs {
		kvs = append(kvs, attr.Key, attr.Value)
	}
	s.span.SetAttributes(kvs...)
}

func (s *otelSpanWrapper) AddEvent(name string, attrs ...contracts.Attribute) {
	s.span.AddEvent(name)
}

func (s *otelSpanWrapper) RecordError(err error) {
	s.span.RecordError(err)
}

func (s *otelSpanWrapper) SpanContext() contracts.SpanContext {
	sc := s.span.SpanContext()
	return contracts.SpanContext{
		TraceID:    fmt.Sprintf("%x", sc.TraceID()),
		SpanID:     fmt.Sprintf("%x", sc.SpanID()),
		TraceFlags: sc.TraceFlags(),
		TraceState: sc.TraceState(),
		Remote:     sc.IsRemote(),
	}
}

func (s *otelSpanWrapper) IsRecording() bool {
	return s.span.IsRecording()
}

// ============ Console/Stdout Driver ============

// ConsoleDriver outputs traces to stdout (useful for development)
type ConsoleDriver struct {
	serviceName string
	idGen       uint64
}

// NewConsoleDriver creates a console tracer
func NewConsoleDriver(serviceName string) *ConsoleDriver {
	return &ConsoleDriver{serviceName: serviceName}
}

func (d *ConsoleDriver) StartSpan(ctx context.Context, name string, opts ...contracts.SpanOption) (context.Context, SpanDriver) {
	id := atomic.AddUint64(&d.idGen, 1)
	span := &consoleSpan{
		name:    name,
		traceID: fmt.Sprintf("trace-%s-%d", d.serviceName, id),
		spanID:  fmt.Sprintf("span-%d", id),
		start:   time.Now(),
	}

	fmt.Printf("[TRACE] Start span: %s (trace=%s, span=%s)\n", name, span.traceID, span.spanID)
	return ctx, span
}

func (d *ConsoleDriver) Extract(ctx context.Context, carrier contracts.Carrier) context.Context {
	return ctx
}

func (d *ConsoleDriver) Inject(ctx context.Context, carrier contracts.Carrier) error {
	return nil
}

func (d *ConsoleDriver) Close() error {
	return nil
}

type consoleSpan struct {
	name    string
	traceID string
	spanID  string
	start   time.Time
}

func (s *consoleSpan) End() {
	fmt.Printf("[TRACE] End span: %s (duration=%v)\n", s.name, time.Since(s.start))
}

func (s *consoleSpan) SetName(name string) {
	s.name = name
}

func (s *consoleSpan) SetStatus(code contracts.SpanStatus, message string) {
	fmt.Printf("[TRACE] Span %s status: %d %s\n", s.name, code, message)
}

func (s *consoleSpan) SetAttributes(attrs ...contracts.Attribute) {
	for _, attr := range attrs {
		fmt.Printf("[TRACE] Span %s attr: %s=%v\n", s.name, attr.Key, attr.Value)
	}
}

func (s *consoleSpan) AddEvent(name string, attrs ...contracts.Attribute) {
	fmt.Printf("[TRACE] Span %s event: %s\n", s.name, name)
}

func (s *consoleSpan) RecordError(err error) {
	fmt.Printf("[TRACE] Span %s error: %v\n", s.name, err)
}

func (s *consoleSpan) SpanContext() contracts.SpanContext {
	return contracts.SpanContext{
		TraceID: s.traceID,
		SpanID:  s.spanID,
	}
}

func (s *consoleSpan) IsRecording() bool {
	return true
}
