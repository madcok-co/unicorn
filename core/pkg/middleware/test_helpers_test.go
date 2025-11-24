package middleware

import (
	"context"

	ucontext "github.com/madcok-co/unicorn/core/pkg/context"
)

// newTestContext creates a new context for testing with proper initialization
func newTestContext() *ucontext.Context {
	ctx := ucontext.New(context.Background())
	return ctx
}

// newTestContextWithRequest creates a context with request data for testing
func newTestContextWithRequest(method, path string, headers map[string]string) *ucontext.Context {
	ctx := ucontext.New(context.Background())
	ctx.SetRequest(&ucontext.Request{
		Method:  method,
		Path:    path,
		Headers: headers,
	})
	// Initialize response headers if nil
	if ctx.Response().Headers == nil {
		ctx.Response().Headers = make(map[string]string)
	}
	return ctx
}
