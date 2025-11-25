# gRPC Example

## Status: Framework Support Coming Soon

The Unicorn framework does not yet have a built-in gRPC adapter. 

However, you can integrate gRPC manually with the framework by:

1. Using standard gRPC server alongside Unicorn's HTTP server
2. Sharing services registered in Unicorn with gRPC handlers
3. Using Unicorn's logger, metrics, and other adapters in gRPC methods

## Example Integration Pattern

```go
package main

import (
    "google.golang.org/grpc"
    "github.com/madcok-co/unicorn/core/pkg/app"
)

func main() {
    // Create Unicorn app
    application := app.New("my-app", "1.0.0")
    
    // Register shared services
    application.RegisterService("userService", NewUserService())
    
    // Start Unicorn HTTP server (for REST API)
    go func() {
        application.Start(":8080")
    }()
    
    // Start gRPC server
    grpcServer := grpc.NewServer()
    // Register your gRPC services here
    // Use application.GetService() to access shared services
    
    lis, _ := net.Listen("tcp", ":9090")
    grpcServer.Serve(lis)
}
```

## Planned Features

When gRPC adapter is implemented, it will support:

- [ ] gRPC server adapter
- [ ] Handler registration similar to HTTP
- [ ] Automatic service registration from proto files
- [ ] Middleware support (auth, logging, metrics)
- [ ] Reflection API
- [ ] Health checks
- [ ] Streaming support

## Alternative: HTTP + gRPC Gateway

In the meantime, you can use [grpc-gateway](https://github.com/grpc-ecosystem/grpc-gateway) to:
1. Define your API in proto files
2. Generate gRPC server code
3. Generate HTTP reverse proxy
4. Use Unicorn for the HTTP layer

## Contributing

Want to help build the gRPC adapter? Check out:
- `core/pkg/adapters/http` for reference implementation
- `core/pkg/contracts/adapter.go` for interface definitions
- Submit a PR!
