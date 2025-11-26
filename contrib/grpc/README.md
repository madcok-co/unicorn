# gRPC Adapter for Unicorn Framework

A production-ready gRPC adapter that integrates seamlessly with the Unicorn framework, providing a robust foundation for building high-performance RPC services.

## Features

- ✅ **Full gRPC Server Integration** - Complete gRPC server implementation with Unicorn's handler registry
- ✅ **Interceptor Support** - Chain unary and stream interceptors for cross-cutting concerns
- ✅ **Built-in Interceptors** - Logging, recovery, timeout, auth, metrics, and rate limiting
- ✅ **TLS Support** - Configurable TLS with modern cipher suites
- ✅ **Service Reflection** - Enable/disable reflection for debugging and development
- ✅ **Metadata Handling** - Helper functions for extracting and setting gRPC metadata
- ✅ **Graceful Shutdown** - Proper cleanup with timeout support
- ✅ **Customizable** - Extensive configuration options for tuning performance

## Installation

```bash
go get github.com/madcok-co/unicorn/contrib/grpc
```

## Quick Start

```go
package main

import (
    "context"
    grpcAdapter "github.com/madcok-co/unicorn/contrib/grpc"
    "github.com/madcok-co/unicorn/core/pkg/app"
)

func main() {
    // Create application
    application := app.New(&app.Config{
        Name:    "my-grpc-service",
        Version: "1.0.0",
    })

    // Create gRPC adapter
    grpcConfig := &grpcAdapter.Config{
        Host:             "0.0.0.0",
        Port:             9090,
        EnableReflection: true,
    }
    
    adapter := grpcAdapter.New(application.Registry(), grpcConfig)
    
    // Add interceptors
    adapter.UseUnaryInterceptor(grpcAdapter.RecoveryInterceptor())
    
    // Register your gRPC services
    adapter.RegisterService(&pb.YourService_ServiceDesc, yourServiceImpl)
    
    // Start server
    ctx := context.Background()
    adapter.Start(ctx)
}
```

## Configuration

### Basic Configuration

```go
config := &grpcAdapter.Config{
    Host: "0.0.0.0",
    Port: 9090,
}
```

### Advanced Configuration

```go
config := &grpcAdapter.Config{
    Host:             "0.0.0.0",
    Port:             9090,
    EnableReflection: true,
    
    // Message size limits
    MaxRecvMsgSize: 10 << 20,  // 10MB
    MaxSendMsgSize: 10 << 20,  // 10MB
    
    // Connection parameters
    MaxConnectionIdle:     15 * time.Minute,
    MaxConnectionAge:      30 * time.Minute,
    MaxConnectionAgeGrace: 5 * time.Second,
    KeepAliveTime:         5 * time.Minute,
    KeepAliveTimeout:      1 * time.Minute,
}
```

### TLS Configuration

```go
config := &grpcAdapter.Config{
    Host: "0.0.0.0",
    Port: 9090,
    TLS: &contracts.TLSConfig{
        Enabled:  true,
        CertFile: "/path/to/server.crt",
        KeyFile:  "/path/to/server.key",
        MinVersion: tls.VersionTLS12,
    },
}
```

## Interceptors

### Built-in Interceptors

#### Recovery Interceptor

Recovers from panics in RPC handlers:

```go
adapter.UseUnaryInterceptor(grpcAdapter.RecoveryInterceptor())
```

#### Logging Interceptor

Logs all RPC calls with duration and status:

```go
adapter.UseUnaryInterceptor(grpcAdapter.LoggingInterceptor(logger))
```

#### Timeout Interceptor

Adds timeout to requests:

```go
adapter.UseUnaryInterceptor(grpcAdapter.TimeoutInterceptor(30 * time.Second))
```

#### Auth Interceptor

Validates authentication:

```go
validator := func(ctx context.Context) error {
    token, ok := grpcAdapter.GetMetadata(ctx, "authorization")
    if !ok {
        return errors.New("missing authorization header")
    }
    // Validate token
    return nil
}

adapter.UseUnaryInterceptor(grpcAdapter.AuthInterceptor(validator))
```

#### Metrics Interceptor

Tracks RPC metrics:

```go
adapter.UseUnaryInterceptor(grpcAdapter.MetricsInterceptor(metricsCollector))
```

#### Rate Limit Interceptor

Limits request rate:

```go
adapter.UseUnaryInterceptor(grpcAdapter.RateLimitInterceptor(rateLimiter))
```

### Custom Interceptors

Create your own interceptors:

```go
customInterceptor := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
    // Pre-processing
    log.Printf("Before: %s", info.FullMethod)
    
    // Call handler
    resp, err := handler(ctx, req)
    
    // Post-processing
    log.Printf("After: %s", info.FullMethod)
    
    return resp, err
}

adapter.UseUnaryInterceptor(customInterceptor)
```

### Interceptor Chain

Interceptors are executed in the order they are added:

```go
adapter.UseUnaryInterceptor(grpcAdapter.RecoveryInterceptor())    // 1. Recover from panics
adapter.UseUnaryInterceptor(grpcAdapter.LoggingInterceptor(log))  // 2. Log requests
adapter.UseUnaryInterceptor(grpcAdapter.AuthInterceptor(auth))    // 3. Authenticate
adapter.UseUnaryInterceptor(grpcAdapter.MetricsInterceptor(met))  // 4. Track metrics
```

## Service Registration

### Standard gRPC Service

```go
// Register generated service
adapter.RegisterService(&pb.UserService_ServiceDesc, userServiceImpl)
```

### Multiple Services

```go
adapter.RegisterService(&pb.UserService_ServiceDesc, userServiceImpl)
adapter.RegisterService(&pb.OrderService_ServiceDesc, orderServiceImpl)
adapter.RegisterService(&pb.ProductService_ServiceDesc, productServiceImpl)
```

## Metadata Handling

### Extract Metadata

```go
// Get single value
token, ok := grpcAdapter.GetMetadata(ctx, "authorization")

// Get all metadata
allMetadata := grpcAdapter.GetAllMetadata(ctx)
```

### Set Metadata

```go
err := grpcAdapter.SetMetadata(ctx, "custom-header", "value")
```

## Helper Functions

### Wrap Handler with Unicorn Context

```go
handler := grpcAdapter.WrapHandler(func(ctx *ucontext.Context, req interface{}) (interface{}, error) {
    // Access Unicorn context features
    logger := ctx.Logger()
    logger.Info("Processing request")
    
    // Your business logic
    return response, nil
})
```

### Extract Response Metadata

```go
md := grpcAdapter.ExtractMetadata(unicornCtx)
```

## Complete Example

See the [gRPC example](../../core/examples/grpc) for a complete working implementation featuring:

- UserService with CRUD operations
- Server streaming RPC
- Interceptor usage
- Error handling
- Protocol buffer definitions
- Testing with grpcurl

## API Reference

### Adapter

```go
type Adapter struct {
    // Methods
    UseUnaryInterceptor(interceptor grpc.UnaryServerInterceptor)
    UseStreamInterceptor(interceptor grpc.StreamServerInterceptor)
    RegisterService(desc *grpc.ServiceDesc, handler interface{})
    Start(ctx context.Context) error
    Shutdown(ctx context.Context) error
    Address() string
    IsTLS() bool
}
```

### Config

```go
type Config struct {
    Host                  string
    Port                  int
    TLS                   *contracts.TLSConfig
    MaxRecvMsgSize        int
    MaxSendMsgSize        int
    MaxConnectionIdle     time.Duration
    MaxConnectionAge      time.Duration
    MaxConnectionAgeGrace time.Duration
    KeepAliveTime         time.Duration
    KeepAliveTimeout      time.Duration
    EnableReflection      bool
    ServerOptions         []grpc.ServerOption
}
```

## Performance Tuning

### Message Size Limits

Increase for large payloads:

```go
config.MaxRecvMsgSize = 50 << 20  // 50MB
config.MaxSendMsgSize = 50 << 20  // 50MB
```

### Connection Parameters

Adjust for your workload:

```go
config.MaxConnectionIdle = 5 * time.Minute   // Close idle connections faster
config.KeepAliveTime = 2 * time.Minute       // Send keepalive more frequently
```

## Best Practices

1. **Always use RecoveryInterceptor** - Prevents panics from crashing the server
2. **Enable reflection in development only** - Disable in production for security
3. **Use TLS in production** - Always encrypt traffic in production environments
4. **Set appropriate timeouts** - Prevent long-running requests from blocking resources
5. **Monitor metrics** - Use MetricsInterceptor to track performance
6. **Validate input** - Always validate request data before processing
7. **Handle errors properly** - Use gRPC status codes for standardized error responses

## Testing

Test your gRPC services using grpcurl:

```bash
# List services
grpcurl -plaintext localhost:9090 list

# Call a method
grpcurl -plaintext -d '{"name":"John"}' localhost:9090 pkg.Service/Method

# With TLS
grpcurl -cacert ca.crt server:9090 list
```

## Troubleshooting

### Server won't start

- Check if port is already in use: `lsof -i :9090`
- Verify TLS certificates are valid
- Check file permissions for cert/key files

### Connection refused

- Ensure server is listening on correct host/port
- Check firewall rules
- Verify network connectivity

### TLS handshake errors

- Verify certificate chain is complete
- Check certificate expiration
- Ensure client trusts CA certificate

## Contributing

Contributions are welcome! Please follow these guidelines:

1. Write tests for new features
2. Follow Go conventions and best practices
3. Update documentation
4. Add examples for new functionality

## License

See the [main repository](../../../) for license information.

## Resources

- [gRPC Documentation](https://grpc.io/docs/)
- [gRPC Go Tutorial](https://grpc.io/docs/languages/go/quickstart/)
- [Unicorn Framework](https://github.com/madcok-co/unicorn)
- [Example Implementation](../../core/examples/grpc)
