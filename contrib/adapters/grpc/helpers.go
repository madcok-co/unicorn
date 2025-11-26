package grpc

import (
	"context"
	"encoding/json"

	ucontext "github.com/madcok-co/unicorn/core/pkg/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// HandlerFunc is a function signature for simple gRPC handlers
type HandlerFunc func(ctx context.Context, req interface{}) (interface{}, error)

// WrapHandler wraps a simple function into a gRPC unary handler with Unicorn context
func WrapHandler(fn func(ctx *ucontext.Context, req interface{}) (interface{}, error)) HandlerFunc {
	return func(ctx context.Context, req interface{}) (interface{}, error) {
		// Create unicorn context
		uCtx := ucontext.New(ctx)

		// Extract metadata
		md, ok := metadata.FromIncomingContext(ctx)
		if ok {
			headers := make(map[string]string)
			for k, v := range md {
				if len(v) > 0 {
					headers[k] = v[0]
				}
			}

			uReq := &ucontext.Request{
				Method:      "RPC",
				Headers:     headers,
				Params:      make(map[string]string),
				Query:       make(map[string]string),
				TriggerType: "grpc",
			}

			// Marshal request to JSON and set as body
			if reqBytes, err := json.Marshal(req); err == nil {
				uReq.Body = reqBytes
			}

			uCtx.SetRequest(uReq)
		}

		// Call handler
		return fn(uCtx, req)
	}
}

// SetMetadata sets outgoing metadata in the context
func SetMetadata(ctx context.Context, key, value string) error {
	return grpc.SendHeader(ctx, metadata.Pairs(key, value))
}

// GetMetadata gets metadata from incoming context
func GetMetadata(ctx context.Context, key string) (string, bool) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", false
	}

	values := md.Get(key)
	if len(values) == 0 {
		return "", false
	}

	return values[0], true
}

// GetAllMetadata gets all metadata from incoming context
func GetAllMetadata(ctx context.Context) map[string]string {
	result := make(map[string]string)

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return result
	}

	for k, v := range md {
		if len(v) > 0 {
			result[k] = v[0]
		}
	}

	return result
}
