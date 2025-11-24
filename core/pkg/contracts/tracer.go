package contracts

import (
	"context"
	"time"
)

// Tracer adalah generic interface untuk distributed tracing
// Implementasi bisa OpenTelemetry, Jaeger, Zipkin, Datadog APM, dll
type Tracer interface {
	// Start starts a new span
	Start(ctx context.Context, name string, opts ...SpanOption) (context.Context, Span)

	// Extract extracts span context from carrier (for incoming requests)
	Extract(ctx context.Context, carrier Carrier) context.Context

	// Inject injects span context into carrier (for outgoing requests)
	Inject(ctx context.Context, carrier Carrier) error

	// Close flushes and closes the tracer
	Close() error
}

// Span represents a unit of work in a trace
type Span interface {
	// End finishes the span
	End()

	// SetName sets/overrides the span name
	SetName(name string)

	// SetStatus sets the span status
	SetStatus(code SpanStatus, message string)

	// SetAttributes sets span attributes
	SetAttributes(attrs ...Attribute)

	// AddEvent adds an event to the span
	AddEvent(name string, attrs ...Attribute)

	// RecordError records an error on the span
	RecordError(err error)

	// SpanContext returns the span context
	SpanContext() SpanContext

	// IsRecording returns true if the span is recording
	IsRecording() bool
}

// SpanContext contains identifying trace information
type SpanContext struct {
	TraceID    string
	SpanID     string
	TraceFlags byte
	TraceState string
	Remote     bool
}

// IsValid returns true if the span context is valid
func (sc SpanContext) IsValid() bool {
	return sc.TraceID != "" && sc.SpanID != ""
}

// SpanStatus represents the status of a span
type SpanStatus int

const (
	SpanStatusUnset SpanStatus = iota
	SpanStatusOK
	SpanStatusError
)

// Attribute untuk span attributes
type Attribute struct {
	Key   string
	Value any
}

// Attr adalah shortcut untuk membuat Attribute
func Attr(key string, value any) Attribute {
	return Attribute{Key: key, Value: value}
}

// SpanOption untuk konfigurasi span
type SpanOption func(*SpanConfig)

// SpanConfig untuk span configuration
type SpanConfig struct {
	Kind       SpanKind
	Attributes []Attribute
	StartTime  time.Time
	Links      []Link
}

// SpanKind represents the span kind
type SpanKind int

const (
	SpanKindInternal SpanKind = iota
	SpanKindServer
	SpanKindClient
	SpanKindProducer
	SpanKindConsumer
)

// Link represents a link to another span
type Link struct {
	SpanContext SpanContext
	Attributes  []Attribute
}

// Carrier untuk context propagation
type Carrier interface {
	Get(key string) string
	Set(key, value string)
	Keys() []string
}

// MapCarrier implements Carrier untuk map[string]string
type MapCarrier map[string]string

func (c MapCarrier) Get(key string) string {
	return c[key]
}

func (c MapCarrier) Set(key, value string) {
	c[key] = value
}

func (c MapCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

// HeaderCarrier implements Carrier untuk HTTP headers
type HeaderCarrier map[string][]string

func (c HeaderCarrier) Get(key string) string {
	if vals, ok := c[key]; ok && len(vals) > 0 {
		return vals[0]
	}
	return ""
}

func (c HeaderCarrier) Set(key, value string) {
	c[key] = []string{value}
}

func (c HeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

// ============ Span Options ============

// WithSpanKind sets the span kind
func WithSpanKind(kind SpanKind) SpanOption {
	return func(cfg *SpanConfig) {
		cfg.Kind = kind
	}
}

// WithAttributes sets span attributes
func WithAttributes(attrs ...Attribute) SpanOption {
	return func(cfg *SpanConfig) {
		cfg.Attributes = append(cfg.Attributes, attrs...)
	}
}

// WithStartTime sets the span start time
func WithStartTime(t time.Time) SpanOption {
	return func(cfg *SpanConfig) {
		cfg.StartTime = t
	}
}

// WithLinks sets span links
func WithLinks(links ...Link) SpanOption {
	return func(cfg *SpanConfig) {
		cfg.Links = append(cfg.Links, links...)
	}
}

// ============ Pre-defined Attribute Keys ============

const (
	// Service attributes
	AttrServiceName    = "service.name"
	AttrServiceVersion = "service.version"

	// HTTP attributes
	AttrHTTPMethod       = "http.method"
	AttrHTTPURL          = "http.url"
	AttrHTTPTarget       = "http.target"
	AttrHTTPHost         = "http.host"
	AttrHTTPScheme       = "http.scheme"
	AttrHTTPStatusCode   = "http.status_code"
	AttrHTTPUserAgent    = "http.user_agent"
	AttrHTTPRequestSize  = "http.request_content_length"
	AttrHTTPResponseSize = "http.response_content_length"

	// Database attributes
	AttrDBSystem    = "db.system"
	AttrDBName      = "db.name"
	AttrDBStatement = "db.statement"
	AttrDBOperation = "db.operation"
	AttrDBTable     = "db.sql.table"

	// Messaging attributes
	AttrMessagingSystem        = "messaging.system"
	AttrMessagingDestination   = "messaging.destination"
	AttrMessagingOperation     = "messaging.operation"
	AttrMessagingMessageID     = "messaging.message_id"
	AttrMessagingConsumerGroup = "messaging.consumer.group"

	// Error attributes
	AttrExceptionType    = "exception.type"
	AttrExceptionMessage = "exception.message"
	AttrExceptionStack   = "exception.stacktrace"
)

// TracerConfig untuk konfigurasi tracer
type TracerConfig struct {
	// Provider: opentelemetry, jaeger, zipkin, datadog
	Provider string

	// Service info
	ServiceName    string
	ServiceVersion string
	Environment    string

	// Sampling
	SampleRate float64 // 0.0 to 1.0

	// Exporter config
	ExporterEndpoint string
	ExporterType     string // otlp, jaeger, zipkin, stdout

	// Headers for exporter authentication
	ExporterHeaders map[string]string

	// Propagation format: w3c, b3, jaeger
	PropagationFormat string

	// Enable console output for debugging
	EnableConsoleExporter bool

	// Resource attributes
	ResourceAttributes []Attribute
}

// ============ Tracing Helper ============

// TracingHelper untuk common tracing patterns
type TracingHelper struct {
	tracer Tracer
}

// NewTracingHelper creates a new helper
func NewTracingHelper(t Tracer) *TracingHelper {
	return &TracingHelper{tracer: t}
}

// TraceHTTPRequest starts span for HTTP request
func (h *TracingHelper) TraceHTTPRequest(ctx context.Context, method, path string) (context.Context, Span) {
	return h.tracer.Start(ctx, method+" "+path,
		WithSpanKind(SpanKindServer),
		WithAttributes(
			Attr(AttrHTTPMethod, method),
			Attr(AttrHTTPTarget, path),
		),
	)
}

// TraceHandler starts span for handler execution
func (h *TracingHelper) TraceHandler(ctx context.Context, handlerName string) (context.Context, Span) {
	return h.tracer.Start(ctx, "handler."+handlerName,
		WithSpanKind(SpanKindInternal),
		WithAttributes(
			Attr("handler.name", handlerName),
		),
	)
}

// TraceDBQuery starts span for database query
func (h *TracingHelper) TraceDBQuery(ctx context.Context, operation, table string) (context.Context, Span) {
	return h.tracer.Start(ctx, operation+" "+table,
		WithSpanKind(SpanKindClient),
		WithAttributes(
			Attr(AttrDBOperation, operation),
			Attr(AttrDBTable, table),
		),
	)
}

// TraceMessagePublish starts span for message publish
func (h *TracingHelper) TraceMessagePublish(ctx context.Context, topic string) (context.Context, Span) {
	return h.tracer.Start(ctx, "publish "+topic,
		WithSpanKind(SpanKindProducer),
		WithAttributes(
			Attr(AttrMessagingDestination, topic),
			Attr(AttrMessagingOperation, "publish"),
		),
	)
}

// TraceMessageConsume starts span for message consume
func (h *TracingHelper) TraceMessageConsume(ctx context.Context, topic string) (context.Context, Span) {
	return h.tracer.Start(ctx, "consume "+topic,
		WithSpanKind(SpanKindConsumer),
		WithAttributes(
			Attr(AttrMessagingDestination, topic),
			Attr(AttrMessagingOperation, "consume"),
		),
	)
}

// TraceCacheOp starts span for cache operation
func (h *TracingHelper) TraceCacheOp(ctx context.Context, operation, key string) (context.Context, Span) {
	return h.tracer.Start(ctx, "cache."+operation,
		WithSpanKind(SpanKindClient),
		WithAttributes(
			Attr("cache.operation", operation),
			Attr("cache.key", key),
		),
	)
}

// TraceExternalCall starts span for external HTTP call
func (h *TracingHelper) TraceExternalCall(ctx context.Context, method, url string) (context.Context, Span) {
	return h.tracer.Start(ctx, method+" "+url,
		WithSpanKind(SpanKindClient),
		WithAttributes(
			Attr(AttrHTTPMethod, method),
			Attr(AttrHTTPURL, url),
		),
	)
}
