package contracts

import (
	"context"
	"time"
)

// Metrics adalah generic interface untuk metrics collection
// Implementasi bisa Prometheus, Datadog, StatsD, OpenTelemetry, dll
type Metrics interface {
	// Counter - nilai yang hanya naik (request count, error count)
	Counter(name string, tags ...Tag) Counter

	// Gauge - nilai yang bisa naik/turun (active connections, queue size)
	Gauge(name string, tags ...Tag) Gauge

	// Histogram - distribusi nilai (latency, request size)
	Histogram(name string, tags ...Tag) Histogram

	// Timer - shortcut untuk mengukur durasi
	Timer(name string, tags ...Tag) Timer

	// WithTags returns metrics with additional default tags
	WithTags(tags ...Tag) Metrics

	// Handler returns HTTP handler for metrics endpoint (e.g., /metrics for Prometheus)
	Handler() any

	// Close flushes and closes the metrics client
	Close() error
}

// Counter untuk counting events
type Counter interface {
	Inc()
	Add(delta float64)
}

// Gauge untuk nilai yang bisa berubah
type Gauge interface {
	Set(value float64)
	Inc()
	Dec()
	Add(delta float64)
	Sub(delta float64)
}

// Histogram untuk distribusi nilai
type Histogram interface {
	Observe(value float64)
}

// Timer untuk mengukur durasi
type Timer interface {
	// Start memulai timer, returns function untuk stop
	Start() func()

	// Record records a duration directly
	Record(duration time.Duration)

	// Time executes function and records duration
	Time(fn func())
}

// Tag untuk labeling metrics
type Tag struct {
	Key   string
	Value string
}

// T adalah shortcut untuk membuat Tag
func T(key, value string) Tag {
	return Tag{Key: key, Value: value}
}

// ============ Pre-defined Metric Names ============

const (
	// HTTP Metrics
	MetricHTTPRequestTotal      = "http_request_total"
	MetricHTTPRequestDuration   = "http_request_duration_seconds"
	MetricHTTPRequestSize       = "http_request_size_bytes"
	MetricHTTPResponseSize      = "http_response_size_bytes"
	MetricHTTPActiveConnections = "http_active_connections"

	// Handler Metrics
	MetricHandlerExecutionTotal    = "handler_execution_total"
	MetricHandlerExecutionDuration = "handler_execution_duration_seconds"
	MetricHandlerErrorTotal        = "handler_error_total"

	// Message Broker Metrics
	MetricMessagePublishedTotal  = "message_published_total"
	MetricMessageConsumedTotal   = "message_consumed_total"
	MetricMessageProcessDuration = "message_process_duration_seconds"
	MetricMessageErrorTotal      = "message_error_total"
	MetricMessageRetryTotal      = "message_retry_total"
	MetricMessageDLQTotal        = "message_dlq_total"
	MetricConsumerLag            = "consumer_lag"

	// Database Metrics
	MetricDBQueryTotal       = "db_query_total"
	MetricDBQueryDuration    = "db_query_duration_seconds"
	MetricDBConnectionsOpen  = "db_connections_open"
	MetricDBConnectionsInUse = "db_connections_in_use"
	MetricDBErrorTotal       = "db_error_total"

	// Cache Metrics
	MetricCacheHitTotal   = "cache_hit_total"
	MetricCacheMissTotal  = "cache_miss_total"
	MetricCacheErrorTotal = "cache_error_total"
	MetricCacheOpDuration = "cache_operation_duration_seconds"

	// Application Metrics
	MetricAppInfo    = "app_info"
	MetricAppUptime  = "app_uptime_seconds"
	MetricGoRoutines = "go_routines"
)

// ============ Common Tag Keys ============

const (
	TagMethod     = "method"
	TagPath       = "path"
	TagStatusCode = "status_code"
	TagHandler    = "handler"
	TagService    = "service"
	TagTopic      = "topic"
	TagBroker     = "broker"
	TagOperation  = "operation"
	TagTable      = "table"
	TagCacheKey   = "cache_key"
	TagErrorType  = "error_type"
	TagSuccess    = "success"
)

// MetricsConfig untuk konfigurasi metrics
type MetricsConfig struct {
	// Provider: prometheus, datadog, statsd, otlp
	Provider string

	// Namespace/Prefix for all metrics
	Namespace string

	// Default tags applied to all metrics
	DefaultTags []Tag

	// Histogram buckets for latency metrics (in seconds)
	LatencyBuckets []float64

	// Enable runtime metrics (goroutines, memory, etc)
	EnableRuntimeMetrics bool

	// Prometheus specific
	PrometheusPath string // default: /metrics

	// Datadog/StatsD specific
	StatsdAddress string
	StatsdPrefix  string

	// Push gateway (for batch jobs)
	PushGatewayURL string
	PushInterval   time.Duration
}

// DefaultLatencyBuckets untuk HTTP/handler latency
var DefaultLatencyBuckets = []float64{
	0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10,
}

// ============ Metrics Helper ============

// MetricsRecorder helper untuk record common metrics patterns
type MetricsRecorder struct {
	metrics Metrics
}

// NewMetricsRecorder creates a new recorder
func NewMetricsRecorder(m Metrics) *MetricsRecorder {
	return &MetricsRecorder{metrics: m}
}

// RecordHTTPRequest records HTTP request metrics
func (r *MetricsRecorder) RecordHTTPRequest(ctx context.Context, method, path string, statusCode int, duration time.Duration) {
	tags := []Tag{
		T(TagMethod, method),
		T(TagPath, path),
		T(TagStatusCode, statusCodeToString(statusCode)),
	}

	r.metrics.Counter(MetricHTTPRequestTotal, tags...).Inc()
	r.metrics.Histogram(MetricHTTPRequestDuration, tags...).Observe(duration.Seconds())
}

// RecordHandlerExecution records handler execution metrics
func (r *MetricsRecorder) RecordHandlerExecution(ctx context.Context, handler string, success bool, duration time.Duration) {
	tags := []Tag{
		T(TagHandler, handler),
		T(TagSuccess, boolToString(success)),
	}

	r.metrics.Counter(MetricHandlerExecutionTotal, tags...).Inc()
	r.metrics.Histogram(MetricHandlerExecutionDuration, tags...).Observe(duration.Seconds())

	if !success {
		r.metrics.Counter(MetricHandlerErrorTotal, T(TagHandler, handler)).Inc()
	}
}

// RecordMessageProcessed records message processing metrics
func (r *MetricsRecorder) RecordMessageProcessed(ctx context.Context, broker, topic string, success bool, duration time.Duration) {
	tags := []Tag{
		T(TagBroker, broker),
		T(TagTopic, topic),
		T(TagSuccess, boolToString(success)),
	}

	r.metrics.Counter(MetricMessageConsumedTotal, tags...).Inc()
	r.metrics.Histogram(MetricMessageProcessDuration, tags...).Observe(duration.Seconds())

	if !success {
		r.metrics.Counter(MetricMessageErrorTotal, T(TagBroker, broker), T(TagTopic, topic)).Inc()
	}
}

// RecordDBQuery records database query metrics
func (r *MetricsRecorder) RecordDBQuery(ctx context.Context, operation, table string, success bool, duration time.Duration) {
	tags := []Tag{
		T(TagOperation, operation),
		T(TagTable, table),
		T(TagSuccess, boolToString(success)),
	}

	r.metrics.Counter(MetricDBQueryTotal, tags...).Inc()
	r.metrics.Histogram(MetricDBQueryDuration, tags...).Observe(duration.Seconds())

	if !success {
		r.metrics.Counter(MetricDBErrorTotal, T(TagOperation, operation)).Inc()
	}
}

// RecordCacheOp records cache operation metrics
func (r *MetricsRecorder) RecordCacheOp(ctx context.Context, operation string, hit bool, duration time.Duration) {
	r.metrics.Histogram(MetricCacheOpDuration, T(TagOperation, operation)).Observe(duration.Seconds())

	if hit {
		r.metrics.Counter(MetricCacheHitTotal, T(TagOperation, operation)).Inc()
	} else {
		r.metrics.Counter(MetricCacheMissTotal, T(TagOperation, operation)).Inc()
	}
}

func statusCodeToString(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "2xx"
	case code >= 300 && code < 400:
		return "3xx"
	case code >= 400 && code < 500:
		return "4xx"
	case code >= 500:
		return "5xx"
	default:
		return "unknown"
	}
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
