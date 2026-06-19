# Unicorn WebSocket Adapter

Real-time WebSocket communication for Unicorn applications.

## Installation

```bash
go get github.com/madcok-co/unicorn/contrib/websocket
```

## Usage

### Basic Setup

```go
package main

import (
    "context"
    "log"

    "github.com/madcok-co/unicorn/contrib/websocket"
)

func main() {
    adapter := websocket.NewAdapter(&websocket.Config{
        Host: "0.0.0.0",
        Port: 8081,
        Path: "/ws",
    })

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    go func() {
        if err := adapter.Start(ctx); err != nil {
            log.Fatalf("websocket server error: %v", err)
        }
    }()

    // Application logic here...

    cancel() // Graceful shutdown
}
```

### Broadcasting

Send a message to all connected clients:

```go
adapter.Broadcast([]byte("Hello, everyone!"))
```

### Targeted Messages

Send a message to a specific client:

```go
err := adapter.SendTo("client-id-here", []byte("Private message"))
if err != nil {
    log.Printf("failed to send: %v", err)
}
```

### Client Callbacks

React to client lifecycle events:

```go
adapter := websocket.NewAdapter(&websocket.Config{
    Host: "0.0.0.0",
    Port: 8081,
    OnConnect: func(clientID string) {
        log.Printf("client connected: %s", clientID)
    },
    OnDisconnect: func(clientID string) {
        log.Printf("client disconnected: %s", clientID)
    },
    OnMessage: func(clientID string, msg []byte) {
        log.Printf("message from %s: %s", clientID, string(msg))
    },
})
```

### Client Management

```go
// Get connected client count
count := adapter.ClientCount()

// List all client IDs
ids := adapter.ClientIDs()

// Force-disconnect a client
err := adapter.KickClient("problem-client-id")
```

### Integration with Unicorn App as Sidecar

```go
import (
    "github.com/madcok-co/unicorn/core/pkg/contracts"
    ws "github.com/madcok-co/unicorn/contrib/websocket"
)

// The adapter implements contracts.Sidecar, so it can be used
// directly with the Unicorn app lifecycle manager.
var _ contracts.Sidecar = (*ws.Adapter)(nil)

func setupSidecar() contracts.Sidecar {
    return ws.NewAdapter(&ws.Config{
        Port: 8081,
        OnMessage: func(clientID string, msg []byte) {
            // Handle incoming WebSocket messages
        },
    })
}
```

## Configuration

| Field          | Default     | Description                        |
|---------------|-------------|------------------------------------|
| `Host`        | `0.0.0.0`   | Address to bind on                 |
| `Port`        | `8081`      | Port to listen on                  |
| `Path`        | `/ws`       | WebSocket endpoint path            |
| `MaxMsgSize`  | `4096`      | Maximum message size in bytes      |
| `PingPeriod`  | `54s`       | Interval between keep-alive pings  |
| `PongWait`    | `60s`       | Time to wait for pong response     |
| `WriteTimeout`| `10s`       | Write deadline for messages        |
