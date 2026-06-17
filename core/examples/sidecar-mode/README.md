# Sidecar Mode Example — Hybrid Trigger & Cross-Cutting Sidecars

This example demonstrates the three deployment modes supported by Unicorn's
sidecar architecture.

## What It Shows

| File | Purpose |
|------|---------|
| `main.go` | Three deployment modes in one file |

## Deployment Modes

### 1. Inline Mode (`mode = "inline"`)

```
app.AddSidecar(management.New(config))  // cross-cutting only
app.Start()
                              ▼
┌─────────────────────────────────────┐
│  HTTP Adapter  (goroutine)          │
│  Broker Adapter (goroutine)         │
│  Cron Adapter  (goroutine)          │
│  Sidecar: ManagementServer          │
└─────────────────────────────────────┘
```

All triggers run inline. Sidecars are used only for cross-cutting concerns
(health, metrics, config). This is the default Unicorn mode, unchanged.

### 2. Full Sidecar Mode (`mode = "sidecar"`)

```
app.AddSidecar(HTTPSidecar{...})
app.AddSidecar(BrokerSidecar{...})
app.AddSidecar(CronSidecar{...})
app.AddSidecar(ManagementServer{...})
app.AddSidecar(CustomProtocolConsumer{...})
app.RunSidecars()
                              ▼
┌── 1 Process ─────────────────────────────┐
│  [Sidecar] HTTP Trigger   :8080          │
│  [Sidecar] Broker Trigger (memory)       │
│  [Sidecar] Cron Trigger   (@every 30s)   │
│  [Sidecar] Management     :9090          │
│  [Sidecar] Custom Protocol:9091          │
│                                           │
│  App core: handler registry (shared)     │
└───────────────────────────────────────────┘
```

Each trigger runs as an isolated sidecar. **No built-in adapters start.**
A custom `ProtocolConsumer` sidecar demonstrates how to plug in any
protocol (TCP, WebSocket, SIP, MQTT, etc.).

### 3. Hybrid Mode (`mode = "hybrid"`)

```
// Broker runs inline (goroutine inside app)
app.SetBroker(memBroker)

// HTTP runs as sidecar
app.AddSidecar(NewHTTPSidecar(...))

// Cross-cutting sidecars
app.AddSidecar(ManagementServer{...})
                              ▼
┌─────────────────────────────────────┐
│  Broker Adapter (goroutine) inline  │
│                                      │
│  [Sidecar] HTTP Trigger   :8080     │
│  [Sidecar] Management     :9090     │
└─────────────────────────────────────┘
```

Mix and match. Critical triggers stay inline, less stable ones go in sidecars.

## How to Run

```bash
cd core/examples/sidecar-mode

# Change mode at the top of main.go: "inline" | "sidecar" | "hybrid"
go run main.go
```

## Key Takeaway

**Same handlers, same registry, three deployment modes.** No code changes needed
when switching between inline and sidecar — only the registration pattern changes.
