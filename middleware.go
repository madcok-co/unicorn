// ============================================
// UNICORN Framework - Middleware System
// ============================================

package unicorn

import "errors"

var (
	ErrMiddlewareChainBroken = errors.New("middleware chain broken")
)

// Middleware represents a middleware that can process requests.
type Middleware interface {
	// Handle processes the request and calls next to continue the chain
	Handle(ctx *Context, next func() (interface{}, error)) (interface{}, error)
}

// MiddlewareFunc is a function that implements Middleware interface.
type MiddlewareFunc func(ctx *Context, next func() (interface{}, error)) (interface{}, error)

// Handle implements Middleware interface.
func (f MiddlewareFunc) Handle(ctx *Context, next func() (interface{}, error)) (interface{}, error) {
	return f(ctx, next)
}

// MiddlewareChain represents a chain of middlewares.
type MiddlewareChain struct {
	middlewares []Middleware
}

// NewMiddlewareChain creates a new middleware chain.
func NewMiddlewareChain() *MiddlewareChain {
	return &MiddlewareChain{
		middlewares: make([]Middleware, 0),
	}
}

// Use adds a middleware to the chain.
func (c *MiddlewareChain) Use(m Middleware) *MiddlewareChain {
	c.middlewares = append(c.middlewares, m)
	return c
}

// UseFunc adds a middleware function to the chain.
func (c *MiddlewareChain) UseFunc(f MiddlewareFunc) *MiddlewareChain {
	return c.Use(f)
}

// Execute executes the middleware chain with the final handler.
func (c *MiddlewareChain) Execute(ctx *Context, handler Service, request interface{}) (interface{}, error) {
	if len(c.middlewares) == 0 {
		return handler.Handle(ctx, request)
	}

	// Build the chain from end to start
	next := func() (interface{}, error) {
		return handler.Handle(ctx, request)
	}

	// Wrap with middlewares in reverse order
	for i := len(c.middlewares) - 1; i >= 0; i-- {
		middleware := c.middlewares[i]
		currentNext := next
		next = func() (interface{}, error) {
			return middleware.Handle(ctx, currentNext)
		}
	}

	// Execute the chain
	return next()
}

// MiddlewareRegistry manages global middleware instances.
type MiddlewareRegistry struct {
	middlewares map[string]Middleware
}

// NewMiddlewareRegistry creates a new middleware registry.
func NewMiddlewareRegistry() *MiddlewareRegistry {
	return &MiddlewareRegistry{
		middlewares: make(map[string]Middleware),
	}
}

// Register registers a middleware with a name.
func (r *MiddlewareRegistry) Register(name string, middleware Middleware) {
	r.middlewares[name] = middleware
}

// Get retrieves a middleware by name.
func (r *MiddlewareRegistry) Get(name string) Middleware {
	return r.middlewares[name]
}

// Global middleware registry
var globalMiddlewareRegistry = NewMiddlewareRegistry()

// RegisterMiddleware registers a middleware globally.
func RegisterMiddleware(name string, middleware Middleware) {
	globalMiddlewareRegistry.Register(name, middleware)
}

// GetMiddleware retrieves a middleware by name.
func GetMiddleware(name string) Middleware {
	return globalMiddlewareRegistry.Get(name)
}
