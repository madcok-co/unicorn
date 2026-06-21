# Benchmarks

> **Note**: Benchmarks are meant to show relative performance characteristics, not absolute numbers. Your results may vary based on hardware and workload.

## Running Benchmarks

```bash
# Run all benchmarks
go test -bench=. -benchmem ./core/pkg/context/... ./core/pkg/handler/... ./core/pkg/middleware/... ./core/pkg/app/...

# Run specific package
go test -bench=. -benchmem ./core/pkg/context/...

# Run specific benchmark
go test -bench=BenchmarkContextAcquire -benchmem ./core/pkg/context/...

# Run with more iterations for accuracy
go test -bench=. -benchmem -count=5 ./core/pkg/context/...

# Run with CPU profiling
go test -bench=. -benchmem -cpuprofile=cpu.prof ./core/pkg/context/...

# Run with memory profiling
go test -bench=. -benchmem -memprofile=mem.prof ./core/pkg/context/...
```

## Latest Results

**Environment:**
- OS: Linux
- Arch: amd64
- CPU: Intel Core i7-9700 @ 3.00GHz
- Go Version: 1.24+
- Date: June 2026

---

### 1. Context Performance (Hot Path)

The context is the most critical performance path — it is acquired and released on every single request.

| Benchmark | ns/op | B/op | allocs/op | Description |
|-----------|------:|-----:|----------:|-------------|
| **ContextAcquire** | **36.94** | **0** | **0** | Get context from pool with adapters |
| **ContextNew** | **34.22** | **0** | **0** | Create new context (uses pool) |
| **ContextAcquireWithAccess** | **35.48** | **0** | **0** | Acquire + access DB/Cache/Logger/Auth/Authz |
| **ContextMetadata** | **247.0** | **0** | **0** | Set/Get metadata with RWMutex |
| **ContextRequest** | **93.67** | **0** | **0** | Set request properties |
| **ContextJSON** | **77.44** | **0** | **0** | Set JSON response |
| **ContextParallel** | **260.3** | **336** | **2** | 10 goroutines parallel ops |

**Key findings:**
- **36.94ns** per context acquire/release — matches docs claim of ~38ns
- **Zero allocations** on all single-goroutine paths
- Lazy adapter injection ensures no overhead when features are unused
- Parallel allocation (336 B/2 allocs) is from slice growth in benchmark harness, not the context itself

---

### 2. Handler Performance

| Benchmark | ns/op | B/op | allocs/op | Description |
|-----------|------:|-----:|----------:|-------------|
| **New** | **12.68** | **0** | **0** | Create handler from function |
| **Handler_HTTP** | **90.43** | **80** | **2** | Add HTTP trigger to handler |
| **Registry_Register** | **986.3** | **1,024** | **15** | Register handler in registry |
| **Registry_GetHTTPHandler** | **183.7** | **48** | **3** | Get handler by HTTP route |
| **Registry_ConcurrentReads** | **5,560** | **6,952** | **11** | Concurrent registry reads (10 goroutines) |

**Key findings:**
- Handler creation is **12.68ns**, zero alloc — just struct allocation
- Registry registration is heavier (~1µs) but only happens at startup, never at request time
- Concurrent reads scale well with RWMutex

---

### 3. Middleware Performance

| Benchmark | ns/op | B/op | allocs/op | Description |
|-----------|------:|-----:|----------:|-------------|
| **Recovery** (no panic) | **6.69** | **0** | **0** | Panic recovery — pass-through |
| **Recovery** (with panic) | **394.3** | **336** | **2** | Panic recovery — actual recovery |
| **CORS** (simple) | **35.31** | **0** | **0** | Non-preflight CORS check |
| **CORS** (preflight) | **1,094** | **1,472** | **14** | Preflight OPTIONS request |
| **RateLimit** | **456.1** | **305** | **4** | Rate limit middleware |
| **MemoryRateLimitStore** | **94.40** | **0** | **0** | In-memory rate limit store |
| **Compress** (gzip) | **3,057** | **2,481** | **8** | Response compression (gzip) |
| **Compress** (brotli) | **3,133** | **2,481** | **8** | Response compression (brotli) |
| **HealthCheck** | **12,670** | **2,000** | **21** | Full health check (all checkers) |
| **HealthCheck** (cached) | **2,958** | **1,344** | **11** | Health check with result cache |
| **Timeout** | **138,823** | **593** | **7** | Timeout middleware (~138µs for 100ms sleep) |

**Key findings:**
- **Recovery and CORS are virtually free** — 6.69ns and 35.31ns, zero alloc
- Rate limiter memory store is **zero alloc** on the Allow path
- Compression cost is proportional to payload size (benchmark compresses 1KB payload)
- Health check with cache is **4.3x faster** than uncached

---

### 4. App Initialization (Startup Only)

These benchmarks measure startup cost — they run once per application start, not per request.

| Benchmark | ns/op | B/op | allocs/op | Description |
|-----------|------:|-----:|----------:|-------------|
| **New** (app) | **1,113** | **1,432** | **22** | Create new app instance |
| **NewContext** | **540.7** | **592** | **9** | Create context for handler |
| **RegisterHandler** | **1,722** | **487** | **12** | Register handler with trigger |

**Key findings:**
- App creation is **~1.1µs** — entirely negligible (runs once at startup)
- Handler registration is **~1.7µs** per handler — fine for hundreds of handlers

---

### 5. Idle Memory Footprint

Memory consumption when application is fully initialized but idle:

| Component | Memory Impact | Notes |
|-----------|--------------|-------|
| Baseline (after GC) | 0.20 MB | Go runtime overhead |
| + Config Management | +0.00 MB | Viper initialization |
| + Multi-tenancy | +0.01 MB | 1 tenant registered |
| + OAuth2 Driver | +0.00 MB | Google provider config |
| + RBAC Driver | +0.00 MB | 2 roles configured |
| **Full App (IDLE)** | **0.21 MB** | All enterprise features enabled |

**System Memory Reserved:** 6.96 MB (includes Go runtime, GC buffers, and stack space)

---

### 6. Binary Size

| Build Mode | Size | Notes |
|-----------|------|-------|
| Unstripped | 9.0 MB | Full debug symbols |
| Stripped (`-ldflags="-s -w"`) | 6.2 MB | Production build |
| Stripped + UPX | ~2 MB | Compressed deploy artifact |

Unicorn core framework contributes **~600KB** to the binary. The rest is Go runtime + standard library (crypto, HTTP server, JSON encoding).

---

### 7. Sidecar Overhead

| Metric | Value |
|--------|-------|
| Per-sidecar goroutine cost | ~8 KB (stack) |
| `startSidecar()` wrapper allocation | 0 B (no heap alloc) |
| Watchdog goroutine | ~8 KB (only when sidecar is stuck) |
| Management server memory | ~0.5 MB idle |

**Sidecar mode adds zero heap allocation.** Each sidecar is just one extra goroutine (~8KB stack).

---

## Summary

| Claim | Benchmark Result | Status |
|-------|-----------------|--------|
| **~38ns** context acquire | **36.94 ns** | ✅ Better than claimed |
| **0 B/op** hot path | **0 B/op** all single-goroutine paths | ✅ Verified |
| Zero allocation pool | `sync.Pool` with map reuse | ✅ Implemented |
| Lazy adapter injection | Pointer reference, no copy | ✅ Verified |
| Framework overhead < 0.001% | ~130ns vs 50ms request | ✅ Insignificant |

## Performance Optimizations

### 1. Object Pooling (sync.Pool)

```go
// Context objects are reused via sync.Pool
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

// Acquire from pool
ctx := Acquire(context.Background(), adapters)

// Release back to pool when done
defer ctx.Release()
```

### 2. Lazy Adapter Injection

```go
// Instead of copying all adapters per request:
// We use a single shared pointer reference:

func (c *Context) DB() contracts.Database {
    if c.app == nil {
        return nil
    }
    return c.app.DB  // Direct access via pointer, no copy
}
```

### 3. Map Reuse (Clear, Not Reallocate)

```go
func (c *Context) reset() {
    // Clear maps (keep capacity allocated)
    for k := range c.metadata {
        delete(c.metadata, k)
    }
    // ... same for other maps
}
```

### 4. Struct Field Ordering

Fields are ordered by access frequency and alignment:
- Hot fields (ctx, app, identity) first — better cache locality
- Maps (metadata, services) in the middle — accessed via pointer
- Mutexes last — avoid false sharing

## Comparison with Other Frameworks

> **Note:** Framework comparison numbers are estimates based on published benchmarks.
> Actual results vary by hardware, workload, and benchmark methodology.

| Framework | Context Overhead | Allocs | Notes |
|-----------|----------------|--------|-------|
| **Unicorn** | **36.94 ns** | **0 B/op** | Object pooling + lazy injection |
| Gin | ~50 ns | 0-1 B/op | Object pooling (gin.Context) |
| Echo | ~120 ns | ~100 B/op | Minimal allocations |
| Fiber | ~50 ns | 0 B/op | Fasthttp-based |

### Real-World Impact

In a typical request with database query:

```
Total request time: 50ms
├── Database query:      45ms   (90%)
├── Business logic:       4ms   (8%)
├── JSON serialization: 0.5ms   (1%)
└── Framework overhead: 0.5ms   (1%)  ← Unicorn optimized here

Framework overhead breakdown:
├── Context acquire:    ~37ns
├── Adapter access:     ~10ns
├── Response set:       ~77ns
└── Context release:    ~10ns
Total: ~134ns = 0.000134ms
```

Framework overhead is **< 0.001%** of total request time.

## Writing Your Own Benchmarks

```go
package mypackage

import (
    "context"
    "testing"

    ucontext "github.com/madcok-co/unicorn/core/pkg/context"
)

func BenchmarkMyHandler(b *testing.B) {
    adapters := &ucontext.AppAdapters{
        // Setup your adapters
    }

    b.ResetTimer()
    b.ReportAllocs()

    for i := 0; i < b.N; i++ {
        ctx := ucontext.Acquire(context.Background(), adapters)

        // Your handler logic here
        _ = ctx.JSON(200, map[string]string{"status": "ok"})

        ctx.Release()
    }
}
```

## Profiling Tips

### CPU Profiling

```bash
# Generate profile
go test -bench=BenchmarkContextAcquire -cpuprofile=cpu.prof ./core/pkg/context/...

# Analyze with pprof
go tool pprof cpu.prof

# Web UI
go tool pprof -http=:8080 cpu.prof
```

### Memory Profiling

```bash
# Generate profile
go test -bench=BenchmarkContextAcquire -memprofile=mem.prof ./core/pkg/context/...

# Analyze allocations
go tool pprof -alloc_space mem.prof

# Analyze in-use memory
go tool pprof -inuse_space mem.prof
```

### Trace

```bash
# Generate trace
go test -bench=BenchmarkContextParallel -trace=trace.out ./core/pkg/context/...

# View trace
go tool trace trace.out
```

## Continuous Benchmarking

For tracking performance over time:

```bash
# Install benchstat
go install golang.org/x/perf/cmd/benchstat@latest

# Run benchmarks and save results
go test -bench=. -benchmem -count=10 ./core/pkg/context/... > old.txt

# After changes, run again
go test -bench=. -benchmem -count=10 ./core/pkg/context/... > new.txt

# Compare results
benchstat old.txt new.txt
```

---

*"Premature optimization is the root of all evil, but mature optimization is the root of all performance."*
