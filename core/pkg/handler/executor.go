package handler

import (
	"encoding/json"
	"fmt"
	"reflect"

	ucontext "github.com/madcok-co/unicorn/core/pkg/context"
)

// Executor handles the execution of handlers
type Executor struct {
	handler *Handler
}

// NewExecutor creates a new executor for a handler
func NewExecutor(h *Handler) *Executor {
	return &Executor{handler: h}
}

// Execute runs the handler with the given context
func (e *Executor) Execute(ctx *ucontext.Context) error {
	// Build middleware chain
	executor := e.buildExecutor()

	// Apply middlewares in reverse order
	for i := len(e.handler.middlewares) - 1; i >= 0; i-- {
		executor = e.handler.middlewares[i](executor)
	}

	// Execute
	return executor(ctx)
}

// buildExecutor creates the final executor that calls the handler
func (e *Executor) buildExecutor() HandlerExecutor {
	return func(ctx *ucontext.Context) error {
		fn := reflect.ValueOf(e.handler.fn)
		fnType := fn.Type()

		// Prepare arguments
		args := make([]reflect.Value, fnType.NumIn())

		// First argument is always *Context
		args[0] = reflect.ValueOf(ctx)

		// Second argument is request (if exists)
		if fnType.NumIn() > 1 {
			reqType := fnType.In(1)
			req, err := e.deserializeRequest(ctx, reqType)
			if err != nil {
				return fmt.Errorf("failed to deserialize request: %w", err)
			}
			args[1] = req
		}

		// Call the handler
		results := fn.Call(args)

		// Process results
		return e.processResults(ctx, results)
	}
}

// deserializeRequest deserializes request body to the appropriate type
func (e *Executor) deserializeRequest(ctx *ucontext.Context, reqType reflect.Type) (reflect.Value, error) {
	// Create new instance of request type
	var req reflect.Value
	if reqType.Kind() == reflect.Ptr {
		req = reflect.New(reqType.Elem())
	} else {
		req = reflect.New(reqType).Elem()
	}

	// Get request body
	body := ctx.Request().Body
	if len(body) == 0 {
		return req, nil
	}

	// Deserialize based on content type (default JSON)
	var target any
	if reqType.Kind() == reflect.Ptr {
		target = req.Interface()
	} else {
		target = req.Addr().Interface()
	}

	if err := json.Unmarshal(body, target); err != nil {
		return req, fmt.Errorf("failed to unmarshal request: %w", err)
	}

	return req, nil
}

// processResults processes handler return values
func (e *Executor) processResults(ctx *ucontext.Context, results []reflect.Value) error {
	if len(results) == 0 {
		return nil
	}

	// Last result is always error
	errVal := results[len(results)-1]

	// Check if error is non-nil
	// Must check both IsValid and IsNil to handle interface containing nil value
	if errVal.IsValid() && !isNilValue(errVal) {
		if err, ok := errVal.Interface().(error); ok {
			return err
		}
	}

	// If there's a response value, set it
	if len(results) > 1 {
		respVal := results[0]

		// Check if response is valid and not nil
		if respVal.IsValid() && !isNilValue(respVal) {
			// Set response body
			resp := respVal.Interface()

			// Default to 200 OK with JSON response
			if ctx.Response().StatusCode == 0 {
				_ = ctx.JSON(200, resp) // Best-effort response write
			} else {
				ctx.Response().Body = resp
			}
		}
	}

	return nil
}

// isNilValue checks if a reflect.Value is nil
// This handles both nil pointers and nil interfaces properly
func isNilValue(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}

	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Ptr, reflect.Interface, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

// ExecuteWithRawBody executes handler with raw body (for non-JSON requests)
func (e *Executor) ExecuteWithRawBody(ctx *ucontext.Context, body []byte) error {
	ctx.Request().Body = body
	return e.Execute(ctx)
}
