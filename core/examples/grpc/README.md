# gRPC Example

This example demonstrates how to build a gRPC service using the Unicorn framework with the gRPC adapter.

## Features Demonstrated

- ✅ gRPC server with Unicorn adapter
- ✅ Protocol Buffers (protobuf) integration
- ✅ Unary RPC methods (request-response)
- ✅ Server streaming RPC
- ✅ gRPC interceptors (logging, recovery, metrics)
- ✅ Service reflection for debugging
- ✅ CRUD operations via gRPC
- ✅ Error handling with gRPC status codes
- ✅ In-memory data storage

## Service Definition

The example implements a `UserService` with the following RPCs:

- `CreateUser` - Create a new user
- `GetUser` - Get user by ID
- `ListUsers` - List users with pagination and filtering
- `UpdateUser` - Update user information
- `DeleteUser` - Delete a user
- `StreamUsers` - Stream users (server streaming)

## Prerequisites

### 1. Install Protocol Buffers Compiler

```bash
# macOS
brew install protobuf

# Ubuntu/Debian
sudo apt install protobuf-compiler

# Or download from: https://grpc.io/docs/protoc-installation/
```

### 2. Install Go Plugins

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Make sure $GOPATH/bin is in your PATH
export PATH=$PATH:$(go env GOPATH)/bin
```

### 3. Install grpcurl (for testing)

```bash
# macOS
brew install grpcurl

# Or download from: https://github.com/fullstorydev/grpcurl
```

## Generate Proto Files

If you modify the `.proto` files, regenerate the Go code:

```bash
./generate.sh
```

Or manually:

```bash
protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    proto/user.proto

# Move generated files to pb directory
mv proto/*.pb.go pb/
```

## Running the Example

```bash
# Build and run
go run main.go
```

The server will start on `localhost:9090` with reflection enabled.

## Testing with grpcurl

### List available services

```bash
grpcurl -plaintext localhost:9090 list
```

### List service methods

```bash
grpcurl -plaintext localhost:9090 list user.UserService
```

### Describe a method

```bash
grpcurl -plaintext localhost:9090 describe user.UserService.CreateUser
```

### Create a user

```bash
grpcurl -plaintext -d '{
  "name": "John Doe",
  "email": "john@example.com",
  "role": "admin"
}' localhost:9090 user.UserService/CreateUser
```

### Get a user

```bash
grpcurl -plaintext -d '{
  "id": "USER_ID_HERE"
}' localhost:9090 user.UserService/GetUser
```

### List users

```bash
# List all users
grpcurl -plaintext -d '{
  "page": 1,
  "page_size": 10
}' localhost:9090 user.UserService/ListUsers

# Filter by role
grpcurl -plaintext -d '{
  "page": 1,
  "page_size": 10,
  "role": "admin"
}' localhost:9090 user.UserService/ListUsers
```

### Update a user

```bash
grpcurl -plaintext -d '{
  "id": "USER_ID_HERE",
  "name": "Jane Doe",
  "email": "jane@example.com",
  "role": "user"
}' localhost:9090 user.UserService/UpdateUser
```

### Delete a user

```bash
grpcurl -plaintext -d '{
  "id": "USER_ID_HERE"
}' localhost:9090 user.UserService/DeleteUser
```

### Stream users

```bash
# Stream all users
grpcurl -plaintext -d '{}' localhost:9090 user.UserService/StreamUsers

# Stream users with role filter
grpcurl -plaintext -d '{
  "role": "admin"
}' localhost:9090 user.UserService/StreamUsers
```

## gRPC Adapter Features

### Interceptors

The adapter supports both unary and stream interceptors:

```go
// Add recovery interceptor
adapter.UseUnaryInterceptor(grpcAdapter.RecoveryInterceptor())

// Add custom logging
adapter.UseUnaryInterceptor(func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
    start := time.Now()
    resp, err := handler(ctx, req)
    log.Printf("RPC %s completed in %v", info.FullMethod, time.Since(start))
    return resp, err
})

// Add timeout interceptor
adapter.UseUnaryInterceptor(grpcAdapter.TimeoutInterceptor(30 * time.Second))
```

### Service Registration

```go
// Register a gRPC service
adapter.RegisterService(&pb.UserService_ServiceDesc, userService)
```

### Configuration

```go
grpcConfig := &grpcAdapter.Config{
    Host:             "0.0.0.0",
    Port:             9090,
    EnableReflection: true,
    MaxRecvMsgSize:   4 << 20,  // 4MB
    MaxSendMsgSize:   4 << 20,  // 4MB
}
```

### TLS Support

```go
grpcConfig := &grpcAdapter.Config{
    Host: "0.0.0.0",
    Port: 9090,
    TLS: &contracts.TLSConfig{
        Enabled:  true,
        CertFile: "server.crt",
        KeyFile:  "server.key",
    },
}
```

## Project Structure

```
grpc/
├── main.go              # Main application
├── proto/               # Protocol buffer definitions
│   └── user.proto       # User service proto
├── pb/                  # Generated protobuf code
│   ├── user.pb.go       # Generated messages
│   └── user_grpc.pb.go  # Generated service
├── generate.sh          # Code generation script
└── README.md            # This file
```

## Error Handling

The example uses gRPC status codes for error handling:

```go
// Not found
return nil, status.Errorf(codes.NotFound, "user with ID %s not found", req.Id)

// Invalid argument
return nil, status.Error(codes.InvalidArgument, "name is required")

// Internal error
return nil, status.Error(codes.Internal, "database error")
```

## Code Organization

### Service Implementation

Services implement the generated service interface:

```go
type userServiceServer struct {
    pb.UnimplementedUserServiceServer
}

func (s *userServiceServer) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.CreateUserResponse, error) {
    // Implementation
}
```

### Metadata Handling

Extract and set metadata using helper functions:

```go
// Get metadata
value, ok := grpcAdapter.GetMetadata(ctx, "authorization")

// Set metadata
grpcAdapter.SetMetadata(ctx, "custom-header", "value")
```

## Performance Considerations

- **Message Size Limits**: Default 4MB, configurable via `MaxRecvMsgSize` and `MaxSendMsgSize`
- **Connection Pooling**: gRPC handles connection pooling automatically
- **Keep-Alive**: Configured via `KeepAliveTime` and `KeepAliveTimeout`
- **Streaming**: Use for large datasets or real-time updates

## Monitoring and Debugging

### Reflection

Reflection is enabled by default for development. Disable in production:

```go
grpcConfig := &grpcAdapter.Config{
    EnableReflection: false, // Disable in production
}
```

### Logging

The example includes a logging interceptor that logs all RPC calls with duration and status.

### Metrics

Use the `MetricsInterceptor` for monitoring:

```go
adapter.UseUnaryInterceptor(grpcAdapter.MetricsInterceptor(metricsCollector))
```

## Next Steps

1. **Add authentication**: Use the `AuthInterceptor` to validate JWT tokens
2. **Add rate limiting**: Use the `RateLimitInterceptor` to prevent abuse
3. **Add database**: Replace in-memory storage with real database
4. **Add validation**: Validate request data before processing
5. **Add tests**: Write unit and integration tests for your services

## Resources

- [gRPC Documentation](https://grpc.io/docs/)
- [Protocol Buffers](https://protobuf.dev/)
- [grpcurl](https://github.com/fullstorydev/grpcurl)
- [Unicorn Framework](https://github.com/madcok-co/unicorn)
