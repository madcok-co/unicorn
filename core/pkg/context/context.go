package context

import (
	"context"
	"sync"

	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

// DefaultAdapterName is the name used for default/unnamed adapters
const DefaultAdapterName = "default"

// HandlerFunc defines the handler function signature
type HandlerFunc func(ctx *Context) error

// MiddlewareFunc defines the middleware function signature
type MiddlewareFunc func(next HandlerFunc) HandlerFunc

// contextPool is used to reuse Context objects and reduce GC pressure
var contextPool = sync.Pool{
	New: func() interface{} {
		return &Context{
			metadata: make(map[string]any, 8),
			services: make(map[string]any, 4),
			request: &Request{
				Headers: make(map[string]string, 8),
				Params:  make(map[string]string, 4),
				Query:   make(map[string]string, 8),
			},
			response: &Response{
				Headers: make(map[string]string, 4),
			},
		}
	},
}

// AppAdapters holds reference to app-level adapters for lazy injection
// This is set once at app initialization and shared across all contexts
type AppAdapters struct {
	// Default adapters
	DB        contracts.Database
	Cache     contracts.Cache
	Logger    contracts.Logger
	Queue     contracts.Queue
	Broker    contracts.Broker
	Validator contracts.Validator
	Metrics   contracts.Metrics
	Tracer    contracts.Tracer

	// Named adapters (shared, read-only after init)
	Databases  map[string]contracts.Database
	Caches     map[string]contracts.Cache
	Loggers    map[string]contracts.Logger
	Queues     map[string]contracts.Queue
	Brokers    map[string]contracts.Broker
	Validators map[string]contracts.Validator
	MetricsMap map[string]contracts.Metrics
	TracersMap map[string]contracts.Tracer
}

// Context adalah wrapper yang menyimpan semua dependency
// User hanya perlu interact dengan context ini untuk akses infrastructure
type Context struct {
	ctx context.Context

	// Reference to app-level adapters (lazy injection)
	app *AppAdapters

	// Security
	identity *contracts.Identity

	// Custom services registry (user-defined interfaces)
	services   map[string]any
	servicesMu sync.RWMutex

	// Request/Response data
	request  *Request
	response *Response

	// Metadata with mutex for thread-safe access
	metadata   map[string]any
	metadataMu sync.RWMutex
}

// Request menyimpan data request dari berbagai trigger
type Request struct {
	// Common fields
	Body    []byte
	Headers map[string]string

	// HTTP specific
	Method string
	Path   string
	Params map[string]string
	Query  map[string]string

	// Message broker specific
	Topic     string
	Partition int
	Offset    int64
	Key       []byte

	// Raw trigger type
	TriggerType string

	// Cookies (HTTP specific)
	Cookies map[string]string
}

// Header returns a header value by key
func (r *Request) Header(key string) string {
	if r.Headers == nil {
		return ""
	}
	return r.Headers[key]
}

// QueryParam returns a query parameter value by key
func (r *Request) QueryParam(key string) string {
	if r.Query == nil {
		return ""
	}
	return r.Query[key]
}

// Param returns a path parameter value by key
func (r *Request) Param(key string) string {
	if r.Params == nil {
		return ""
	}
	return r.Params[key]
}

// Cookie returns a cookie value by key
func (r *Request) Cookie(key string) string {
	if r.Cookies == nil {
		return ""
	}
	return r.Cookies[key]
}

// Response untuk menyimpan response data
type Response struct {
	StatusCode int
	Body       any
	Headers    map[string]string
}

// SetHeader sets a response header
func (r *Response) SetHeader(key, value string) {
	if r.Headers == nil {
		r.Headers = make(map[string]string)
	}
	r.Headers[key] = value
}

// Header returns a response header value
func (r *Response) Header(key string) string {
	if r.Headers == nil {
		return ""
	}
	return r.Headers[key]
}

// New creates a new Unicorn Context (uses object pooling for performance)
func New(ctx context.Context) *Context {
	c := contextPool.Get().(*Context)
	c.ctx = ctx
	c.app = nil
	c.identity = nil
	return c
}

// Acquire gets a Context from pool with AppAdapters reference (optimized path)
func Acquire(ctx context.Context, app *AppAdapters) *Context {
	c := contextPool.Get().(*Context)
	c.ctx = ctx
	c.app = app
	c.identity = nil
	return c
}

// Release returns Context to pool for reuse
// Must be called when done with the context to prevent memory leaks
func (c *Context) Release() {
	c.reset()
	contextPool.Put(c)
}

// reset clears all context data for reuse
func (c *Context) reset() {
	c.ctx = nil
	c.app = nil
	c.identity = nil

	// Clear maps instead of reallocating (keep capacity)
	for k := range c.metadata {
		delete(c.metadata, k)
	}
	for k := range c.services {
		delete(c.services, k)
	}

	// Reset request
	c.request.Body = nil
	c.request.Method = ""
	c.request.Path = ""
	c.request.Topic = ""
	c.request.Partition = 0
	c.request.Offset = 0
	c.request.Key = nil
	c.request.TriggerType = ""
	for k := range c.request.Headers {
		delete(c.request.Headers, k)
	}
	for k := range c.request.Params {
		delete(c.request.Params, k)
	}
	for k := range c.request.Query {
		delete(c.request.Query, k)
	}

	// Reset response
	c.response.StatusCode = 0
	c.response.Body = nil
	for k := range c.response.Headers {
		delete(c.response.Headers, k)
	}
}

// SetAppAdapters sets the app-level adapters reference
func (c *Context) SetAppAdapters(app *AppAdapters) *Context {
	c.app = app
	return c
}

// Context returns the underlying context.Context
func (c *Context) Context() context.Context {
	return c.ctx
}

// WithContext sets the underlying context
func (c *Context) WithContext(ctx context.Context) *Context {
	c.ctx = ctx
	return c
}

// ============ Infrastructure Accessors (Lazy Injection) ============

// DB returns the database adapter (lazy loaded from AppAdapters)
func (c *Context) DB(name ...string) contracts.Database {
	if c.app == nil {
		return nil
	}
	if len(name) > 0 && name[0] != "" {
		if c.app.Databases != nil {
			return c.app.Databases[name[0]]
		}
		return nil
	}
	return c.app.DB
}

// Cache returns the cache adapter (lazy loaded from AppAdapters)
func (c *Context) Cache(name ...string) contracts.Cache {
	if c.app == nil {
		return nil
	}
	if len(name) > 0 && name[0] != "" {
		if c.app.Caches != nil {
			return c.app.Caches[name[0]]
		}
		return nil
	}
	return c.app.Cache
}

// Logger returns the logger adapter (lazy loaded from AppAdapters)
func (c *Context) Logger(name ...string) contracts.Logger {
	if c.app == nil {
		return nil
	}
	if len(name) > 0 && name[0] != "" {
		if c.app.Loggers != nil {
			return c.app.Loggers[name[0]]
		}
		return nil
	}
	return c.app.Logger
}

// Queue returns the queue adapter (lazy loaded from AppAdapters)
func (c *Context) Queue(name ...string) contracts.Queue {
	if c.app == nil {
		return nil
	}
	if len(name) > 0 && name[0] != "" {
		if c.app.Queues != nil {
			return c.app.Queues[name[0]]
		}
		return nil
	}
	return c.app.Queue
}

// Broker returns the message broker (lazy loaded from AppAdapters)
func (c *Context) Broker(name ...string) contracts.Broker {
	if c.app == nil {
		return nil
	}
	if len(name) > 0 && name[0] != "" {
		if c.app.Brokers != nil {
			return c.app.Brokers[name[0]]
		}
		return nil
	}
	return c.app.Broker
}

// Metrics returns the metrics collector (lazy loaded from AppAdapters)
func (c *Context) Metrics(name ...string) contracts.Metrics {
	if c.app == nil {
		return nil
	}
	if len(name) > 0 && name[0] != "" {
		if c.app.MetricsMap != nil {
			return c.app.MetricsMap[name[0]]
		}
		return nil
	}
	return c.app.Metrics
}

// Tracer returns the tracer (lazy loaded from AppAdapters)
func (c *Context) Tracer(name ...string) contracts.Tracer {
	if c.app == nil {
		return nil
	}
	if len(name) > 0 && name[0] != "" {
		if c.app.TracersMap != nil {
			return c.app.TracersMap[name[0]]
		}
		return nil
	}
	return c.app.Tracer
}

// Validator returns the validator (lazy loaded from AppAdapters)
func (c *Context) Validator(name ...string) contracts.Validator {
	if c.app == nil {
		return nil
	}
	if len(name) > 0 && name[0] != "" {
		if c.app.Validators != nil {
			return c.app.Validators[name[0]]
		}
		return nil
	}
	return c.app.Validator
}

// ============ Named Adapter Accessors (explicit) ============

// DBNames returns all registered database adapter names
func (c *Context) DBNames() []string {
	if c.app == nil || c.app.Databases == nil {
		return nil
	}
	names := make([]string, 0, len(c.app.Databases))
	for name := range c.app.Databases {
		names = append(names, name)
	}
	return names
}

// CacheNames returns all registered cache adapter names
func (c *Context) CacheNames() []string {
	if c.app == nil || c.app.Caches == nil {
		return nil
	}
	names := make([]string, 0, len(c.app.Caches))
	for name := range c.app.Caches {
		names = append(names, name)
	}
	return names
}

// BrokerNames returns all registered broker adapter names
func (c *Context) BrokerNames() []string {
	if c.app == nil || c.app.Brokers == nil {
		return nil
	}
	names := make([]string, 0, len(c.app.Brokers))
	for name := range c.app.Brokers {
		names = append(names, name)
	}
	return names
}

// ============ Security ============

// Identity returns the authenticated identity
func (c *Context) Identity() *contracts.Identity {
	return c.identity
}

// SetIdentity sets the authenticated identity
func (c *Context) SetIdentity(identity *contracts.Identity) *Context {
	c.identity = identity
	return c
}

// ============ Custom Services ============

// RegisterService registers a custom service/interface by name (thread-safe)
// This allows users to inject their own interfaces into the context
//
// Example:
//
//	type EmailService interface {
//	    Send(to, subject, body string) error
//	}
//
//	ctx.RegisterService("email", myEmailService)
func (c *Context) RegisterService(name string, service any) *Context {
	c.servicesMu.Lock()
	defer c.servicesMu.Unlock()
	c.services[name] = service
	return c
}

// GetService retrieves a custom service by name (thread-safe)
// Returns nil if service is not found
//
// Example:
//
//	emailSvc := ctx.GetService("email").(EmailService)
func (c *Context) GetService(name string) any {
	c.servicesMu.RLock()
	defer c.servicesMu.RUnlock()
	return c.services[name]
}

// MustGetService retrieves a custom service or panics if not found (thread-safe)
func (c *Context) MustGetService(name string) any {
	c.servicesMu.RLock()
	defer c.servicesMu.RUnlock()
	svc, ok := c.services[name]
	if !ok {
		panic("service not found in context: " + name)
	}
	return svc
}

// HasService checks if a service is registered (thread-safe)
func (c *Context) HasService(name string) bool {
	c.servicesMu.RLock()
	defer c.servicesMu.RUnlock()
	_, ok := c.services[name]
	return ok
}

// Services returns all registered service names (thread-safe)
func (c *Context) Services() []string {
	c.servicesMu.RLock()
	defer c.servicesMu.RUnlock()
	names := make([]string, 0, len(c.services))
	for name := range c.services {
		names = append(names, name)
	}
	return names
}

// CopyServicesFrom copies all services from another context (thread-safe)
// Useful for creating child contexts with same services
func (c *Context) CopyServicesFrom(other *Context) *Context {
	other.servicesMu.RLock()
	defer other.servicesMu.RUnlock()
	c.servicesMu.Lock()
	defer c.servicesMu.Unlock()
	for name, svc := range other.services {
		c.services[name] = svc
	}
	return c
}

// ============ Request/Response ============

// Request returns the request data
func (c *Context) Request() *Request {
	return c.request
}

// SetRequest sets request data
func (c *Context) SetRequest(req *Request) *Context {
	c.request = req
	return c
}

// Response returns the response data
func (c *Context) Response() *Response {
	return c.response
}

// ============ Metadata ============

// Set stores a value in context metadata (thread-safe)
func (c *Context) Set(key string, value any) {
	c.metadataMu.Lock()
	defer c.metadataMu.Unlock()
	c.metadata[key] = value
}

// Get retrieves a value from context metadata (thread-safe)
func (c *Context) Get(key string) (any, bool) {
	c.metadataMu.RLock()
	defer c.metadataMu.RUnlock()
	val, ok := c.metadata[key]
	return val, ok
}

// MustGet retrieves a value or panics if not found (thread-safe)
func (c *Context) MustGet(key string) any {
	c.metadataMu.RLock()
	defer c.metadataMu.RUnlock()
	val, ok := c.metadata[key]
	if !ok {
		panic("key not found in context: " + key)
	}
	return val
}

// GetString retrieves a string value from metadata
func (c *Context) GetString(key string) string {
	val, ok := c.Get(key)
	if !ok {
		return ""
	}
	if str, ok := val.(string); ok {
		return str
	}
	return ""
}

// GetInt retrieves an int value from metadata
func (c *Context) GetInt(key string) int {
	val, ok := c.Get(key)
	if !ok {
		return 0
	}
	if i, ok := val.(int); ok {
		return i
	}
	return 0
}

// GetBool retrieves a bool value from metadata
func (c *Context) GetBool(key string) bool {
	val, ok := c.Get(key)
	if !ok {
		return false
	}
	if b, ok := val.(bool); ok {
		return b
	}
	return false
}

// Keys returns all metadata keys
func (c *Context) Keys() []string {
	c.metadataMu.RLock()
	defer c.metadataMu.RUnlock()
	keys := make([]string, 0, len(c.metadata))
	for k := range c.metadata {
		keys = append(keys, k)
	}
	return keys
}

// ============ Response Helpers ============

// JSON sets JSON response
func (c *Context) JSON(statusCode int, body any) error {
	c.response.StatusCode = statusCode
	c.response.Body = body
	c.response.Headers["Content-Type"] = "application/json"
	return nil
}

// Error sets error response
func (c *Context) Error(statusCode int, message string) error {
	c.response.StatusCode = statusCode
	c.response.Body = map[string]string{"error": message}
	c.response.Headers["Content-Type"] = "application/json"
	return nil
}

// Success sets success response
func (c *Context) Success(body any) error {
	return c.JSON(200, body)
}

// Created sets 201 created response
func (c *Context) Created(body any) error {
	return c.JSON(201, body)
}

// NoContent sets 204 no content response
func (c *Context) NoContent() error {
	c.response.StatusCode = 204
	return nil
}

// ============ Observability Helpers ============

// StartSpan starts a new tracing span
func (c *Context) StartSpan(name string, opts ...contracts.SpanOption) (contracts.Span, func()) {
	tracer := c.Tracer()
	if tracer == nil {
		return nil, func() {}
	}
	newCtx, span := tracer.Start(c.ctx, name, opts...)
	c.ctx = newCtx
	return span, span.End
}

// RecordMetric records a custom metric
func (c *Context) RecordMetric(name string, value float64, tags ...contracts.Tag) {
	metrics := c.Metrics()
	if metrics == nil {
		return
	}
	metrics.Histogram(name, tags...).Observe(value)
}

// IncrementCounter increments a counter metric
func (c *Context) IncrementCounter(name string, tags ...contracts.Tag) {
	metrics := c.Metrics()
	if metrics == nil {
		return
	}
	metrics.Counter(name, tags...).Inc()
}

// TraceID returns the current trace ID if available
func (c *Context) TraceID() string {
	tracer := c.Tracer()
	if tracer == nil {
		return ""
	}
	// Extract from context - implementation depends on tracer
	if traceID := c.GetString("trace_id"); traceID != "" {
		return traceID
	}
	return ""
}

// Publish publishes a message to the broker (convenience method)
func (c *Context) Publish(topic string, msg *contracts.BrokerMessage) error {
	broker := c.Broker()
	if broker == nil {
		return nil
	}
	return broker.Publish(c.ctx, topic, msg)
}

// Validate validates a struct using the configured validator
func (c *Context) Validate(data any) error {
	validator := c.Validator()
	if validator == nil {
		return nil
	}
	return validator.Validate(data)
}
