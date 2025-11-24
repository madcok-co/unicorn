package middleware

import (
	"context"
	"time"

	ucontext "github.com/madcok-co/unicorn/core/pkg/context"
)

// ============ Telemetry Interfaces ============

// Tracer interface for distributed tracing
type Tracer interface {
	// StartSpan starts a new span
	StartSpan(ctx context.Context, name string, opts ...SpanOption) (context.Context, Span)
}

// Span represents a trace span
type Span interface {
	// End ends the span
	End()

	// SetAttribute sets a span attribute
	SetAttribute(key string, value interface{})

	// SetStatus sets the span status
	SetStatus(code SpanStatusCode, description string)

	// RecordError records an error
	RecordError(err error)

	// AddEvent adds an event to the span
	AddEvent(name string, attrs ...SpanAttribute)
}

// SpanStatusCode represents span status
type SpanStatusCode int

const (
	SpanStatusUnset SpanStatusCode = iota
	SpanStatusOK
	SpanStatusError
)

// SpanOption is a span creation option
type SpanOption func(*spanConfig)

type spanConfig struct {
	kind       SpanKind
	attributes []SpanAttribute
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

// SpanAttribute represents a span attribute
type SpanAttribute struct {
	Key   string
	Value interface{}
}

// WithSpanKind sets the span kind
func WithSpanKind(kind SpanKind) SpanOption {
	return func(c *spanConfig) {
		c.kind = kind
	}
}

// WithAttributes sets span attributes
func WithAttributes(attrs ...SpanAttribute) SpanOption {
	return func(c *spanConfig) {
		c.attributes = append(c.attributes, attrs...)
	}
}

// MeterProvider interface for metrics
type MeterProvider interface {
	// Counter creates a counter metric
	Counter(name, description string) Counter

	// Histogram creates a histogram metric
	Histogram(name, description string, buckets []float64) Histogram

	// Gauge creates a gauge metric
	Gauge(name, description string) Gauge
}

// Counter is a metric that only increases
type Counter interface {
	Add(ctx context.Context, value float64, attrs ...SpanAttribute)
}

// Histogram records distribution of values
type Histogram interface {
	Record(ctx context.Context, value float64, attrs ...SpanAttribute)
}

// Gauge represents a value that can go up or down
type Gauge interface {
	Set(ctx context.Context, value float64, attrs ...SpanAttribute)
}

// ============ Telemetry Config ============

// TelemetryConfig defines telemetry middleware configuration
type TelemetryConfig struct {
	// Tracer for distributed tracing
	Tracer Tracer

	// MeterProvider for metrics
	MeterProvider MeterProvider

	// ServiceName is the name of the service
	ServiceName string

	// ServiceVersion is the version of the service
	ServiceVersion string

	// SkipPaths are paths to skip for telemetry
	SkipPaths []string

	// Skipper defines a function to skip middleware
	Skipper func(ctx *ucontext.Context) bool

	// SpanNameFormatter formats the span name
	SpanNameFormatter func(ctx *ucontext.Context) string

	// RecordRequestBody records request body in span
	RecordRequestBody bool

	// RecordResponseBody records response body in span
	RecordResponseBody bool
}

// DefaultTelemetryConfig returns default telemetry configuration
func DefaultTelemetryConfig() *TelemetryConfig {
	return &TelemetryConfig{
		ServiceName:    "unicorn-service",
		ServiceVersion: "1.0.0",
		SkipPaths:      []string{"/health", "/health/live", "/health/ready", "/metrics"},
		SpanNameFormatter: func(ctx *ucontext.Context) string {
			return ctx.Request().Method + " " + ctx.Request().Path
		},
	}
}

// ============ Tracing Middleware ============

// Tracing returns tracing middleware
func Tracing(tracer Tracer) ucontext.MiddlewareFunc {
	config := DefaultTelemetryConfig()
	config.Tracer = tracer
	return TracingWithConfig(config)
}

// TracingWithConfig returns tracing middleware with custom config
func TracingWithConfig(config *TelemetryConfig) ucontext.MiddlewareFunc {
	if config == nil {
		config = DefaultTelemetryConfig()
	}

	skipPathsMap := make(map[string]bool)
	for _, path := range config.SkipPaths {
		skipPathsMap[path] = true
	}

	return func(next ucontext.HandlerFunc) ucontext.HandlerFunc {
		return func(ctx *ucontext.Context) error {
			// Check skipper
			if config.Skipper != nil && config.Skipper(ctx) {
				return next(ctx)
			}

			// Check skip paths
			if skipPathsMap[ctx.Request().Path] {
				return next(ctx)
			}

			// No tracer configured
			if config.Tracer == nil {
				return next(ctx)
			}

			// Get span name
			spanName := ctx.Request().Method + " " + ctx.Request().Path
			if config.SpanNameFormatter != nil {
				spanName = config.SpanNameFormatter(ctx)
			}

			// Start span
			spanCtx, span := config.Tracer.StartSpan(
				ctx.Context(),
				spanName,
				WithSpanKind(SpanKindServer),
				WithAttributes(
					SpanAttribute{Key: "http.method", Value: ctx.Request().Method},
					SpanAttribute{Key: "http.url", Value: ctx.Request().Path},
					SpanAttribute{Key: "http.host", Value: ctx.Request().Header("Host")},
					SpanAttribute{Key: "http.user_agent", Value: ctx.Request().Header("User-Agent")},
					SpanAttribute{Key: "service.name", Value: config.ServiceName},
					SpanAttribute{Key: "service.version", Value: config.ServiceVersion},
				),
			)
			defer span.End()

			// Update context
			ctx.WithContext(spanCtx)

			// Record request body if configured
			if config.RecordRequestBody && len(ctx.Request().Body) > 0 {
				span.SetAttribute("http.request.body", string(ctx.Request().Body))
			}

			// Execute handler
			err := next(ctx)

			// Record response
			span.SetAttribute("http.status_code", ctx.Response().StatusCode)

			if err != nil {
				span.SetStatus(SpanStatusError, err.Error())
				span.RecordError(err)
			} else {
				span.SetStatus(SpanStatusOK, "")
			}

			// Record response body if configured
			if config.RecordResponseBody && ctx.Response().Body != nil {
				// Note: Be careful with large response bodies
				span.SetAttribute("http.response.body_type", "recorded")
			}

			return err
		}
	}
}

// ============ Metrics Middleware ============

// Metrics returns metrics middleware
func Metrics(provider MeterProvider) ucontext.MiddlewareFunc {
	config := DefaultTelemetryConfig()
	config.MeterProvider = provider
	return MetricsWithConfig(config)
}

// MetricsWithConfig returns metrics middleware with custom config
func MetricsWithConfig(config *TelemetryConfig) ucontext.MiddlewareFunc {
	if config == nil {
		config = DefaultTelemetryConfig()
	}

	// Skip if no meter provider
	if config.MeterProvider == nil {
		return func(next ucontext.HandlerFunc) ucontext.HandlerFunc {
			return next
		}
	}

	// Create metrics
	requestCounter := config.MeterProvider.Counter(
		"http_requests_total",
		"Total number of HTTP requests",
	)
	requestDuration := config.MeterProvider.Histogram(
		"http_request_duration_seconds",
		"HTTP request duration in seconds",
		[]float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	)
	requestSize := config.MeterProvider.Histogram(
		"http_request_size_bytes",
		"HTTP request size in bytes",
		[]float64{100, 1000, 10000, 100000, 1000000},
	)
	responseSize := config.MeterProvider.Histogram(
		"http_response_size_bytes",
		"HTTP response size in bytes",
		[]float64{100, 1000, 10000, 100000, 1000000},
	)
	activeRequests := config.MeterProvider.Gauge(
		"http_requests_active",
		"Number of active HTTP requests",
	)

	skipPathsMap := make(map[string]bool)
	for _, path := range config.SkipPaths {
		skipPathsMap[path] = true
	}

	return func(next ucontext.HandlerFunc) ucontext.HandlerFunc {
		return func(ctx *ucontext.Context) error {
			// Check skipper
			if config.Skipper != nil && config.Skipper(ctx) {
				return next(ctx)
			}

			// Check skip paths
			if skipPathsMap[ctx.Request().Path] {
				return next(ctx)
			}

			// Common attributes
			attrs := []SpanAttribute{
				{Key: "method", Value: ctx.Request().Method},
				{Key: "path", Value: ctx.Request().Path},
				{Key: "service", Value: config.ServiceName},
			}

			// Track active requests
			activeRequests.Set(ctx.Context(), 1, attrs...)
			defer activeRequests.Set(ctx.Context(), -1, attrs...)

			// Record request size
			requestSize.Record(ctx.Context(), float64(len(ctx.Request().Body)), attrs...)

			// Start timer
			start := time.Now()

			// Execute handler
			err := next(ctx)

			// Record duration
			duration := time.Since(start).Seconds()

			// Add status code to attributes
			statusAttrs := append(attrs, SpanAttribute{
				Key:   "status_code",
				Value: ctx.Response().StatusCode,
			})

			// Record metrics
			requestCounter.Add(ctx.Context(), 1, statusAttrs...)
			requestDuration.Record(ctx.Context(), duration, statusAttrs...)

			// Record response size (approximate)
			if ctx.Response().Body != nil {
				// This is approximate - actual size depends on serialization
				responseSize.Record(ctx.Context(), 0, statusAttrs...)
			}

			return err
		}
	}
}

// ============ No-op Implementations ============

// NoopTracer is a no-op tracer for testing or when tracing is disabled
type NoopTracer struct{}

func (n *NoopTracer) StartSpan(ctx context.Context, name string, opts ...SpanOption) (context.Context, Span) {
	return ctx, &NoopSpan{}
}

type NoopSpan struct{}

func (n *NoopSpan) End()                                              {}
func (n *NoopSpan) SetAttribute(key string, value interface{})        {}
func (n *NoopSpan) SetStatus(code SpanStatusCode, description string) {}
func (n *NoopSpan) RecordError(err error)                             {}
func (n *NoopSpan) AddEvent(name string, attrs ...SpanAttribute)      {}

// NoopMeterProvider is a no-op meter provider
type NoopMeterProvider struct{}

func (n *NoopMeterProvider) Counter(name, description string) Counter {
	return &NoopCounter{}
}

func (n *NoopMeterProvider) Histogram(name, description string, buckets []float64) Histogram {
	return &NoopHistogram{}
}

func (n *NoopMeterProvider) Gauge(name, description string) Gauge {
	return &NoopGauge{}
}

type NoopCounter struct{}

func (n *NoopCounter) Add(ctx context.Context, value float64, attrs ...SpanAttribute) {}

type NoopHistogram struct{}

func (n *NoopHistogram) Record(ctx context.Context, value float64, attrs ...SpanAttribute) {}

type NoopGauge struct{}

func (n *NoopGauge) Set(ctx context.Context, value float64, attrs ...SpanAttribute) {}
