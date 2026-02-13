# Benchmarks

> **Note**: Benchmarks are meant to show relative performance characteristics, not absolute numbers. Your results may vary based on hardware and workload.

## Running Benchmarks

```bash
# Run all benchmarks
go test -bench=. -benchmem ./core/pkg/context/... ./core/pkg/resilience/...

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
- OS: Linux (WSL2)
- Arch: amd64
- CPU: Intel Core i7-9700 @ 3.00GHz

**Results (Latest - Feb 2026):**

### Context Performance

| Benchmark | ns/op | B/op | allocs/op | Description |
|-----------|------:|-----:|----------:|-------------|
| ContextAcquire | 38.04 | 0 | 0 | Get context from pool with adapters |
| ContextNew | 38.51 | 0 | 0 | Create new context (uses pool) |
| ContextAcquireWithAccess | 39.91 | 0 | 0 | Acquire + access DB/Cache/Logger/Auth/Authz |
| ContextMetadata | 223.4 | 0 | 0 | Set/Get metadata values |
| ContextRequest | 93.21 | 0 | 0 | Set request properties |
| ContextJSON | 79.00 | 0 | 0 | Set JSON response |
| ContextParallel | 266.6 | 336 | 2 | Parallel context operations |

### Performance After Enterprise Features

**✅ Zero Performance Degradation**

After adding Auth() and Authz() adapter accessors to the context:
- Context acquire remains **~38ns** (same as before)
- Still **zero allocations** for acquire/release cycle
- Lazy adapter pattern ensures no overhead when features are not used
- Enterprise features only add cost when actively accessed

### Idle Memory Footprint

Memory consumption when application is fully initialized but idle (Feb 2026):

| Component | Memory Impact | Notes |
|-----------|--------------|-------|
| Baseline (after GC) | 0.20 MB | Go runtime overhead |
| + Config Management | +0.00 MB | Viper initialization |
| + Multi-tenancy | +0.01 MB | 1 tenant registered |
| + OAuth2 Driver | +0.00 MB | Google provider config |
| + RBAC Driver | +0.00 MB | 2 roles configured |
| **Full App (IDLE)** | **0.21 MB** | All enterprise features enabled |

**System Memory Reserved:** 6.96 MB (includes Go runtime, GC buffers, and stack space)

**Key Findings:**
- **Ultra-lightweight**: Only 0.21 MB heap allocation when idle
- **Minimal overhead**: Adding all 6 enterprise features adds < 0.01 MB
- **Efficient initialization**: All features initialized with zero bloat
- **Production ready**: Low memory footprint suitable for containerized deployments

### Key Metrics

- **Zero allocation** for context acquire/release cycle
- **~38ns** per context operation (acquire + release)
- **Object pooling** eliminates GC pressure
- **Lazy adapter injection** - no copying, just pointer reference

## Performance Optimizations

Unicorn uses several techniques to achieve high performance:

### 1. Object Pooling (sync.Pool)

```go
// Context objects are reused via sync.Pool
var contextPool = sync.Pool{
    New: func() interface{} {
        return &Context{
            metadata: make(map[string]any, 8),
            // ... pre-allocated maps
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
// OLD: ctx.db = app.db (copy for each adapter)

// We use a shared reference:
// NEW: ctx.app = app.adapters (single pointer)

func (c *Context) DB() contracts.Database {
    if c.app == nil {
        return nil
    }
    return c.app.DB  // Direct access, no copy
}
```

### 3. Map Reuse

```go
// Maps are cleared, not reallocated
func (c *Context) reset() {
    // Clear maps (keep capacity)
    for k := range c.metadata {
        delete(c.metadata, k)
    }
    // ... same for other maps
}
```

## Comparison with Other Frameworks

### Theoretical Comparison

| Framework | Context Alloc | Notes |
|-----------|--------------|-------|
| **Unicorn** | 0 B/op | Object pooling + lazy injection |
| **Gin** | 0 B/op | Object pooling |
| **Echo** | ~100 B/op | Minimal allocations |
| **Fiber** | 0 B/op | Fasthttp + pooling |

### Real-World Impact

In a typical request with database query:

```
Total request time: 50ms
├── Database query:      45ms  (90%)
├── Business logic:       4ms  (8%)
├── JSON serialization: 0.5ms  (1%)
└── Framework overhead: 0.5ms  (1%)  ← Unicorn optimized here

Framework overhead breakdown:
├── Context acquire:    ~40ns
├── Adapter access:     ~10ns
├── Response set:       ~70ns
└── Context release:    ~10ns
Total: ~130ns = 0.00013ms
```

Framework overhead is **< 0.001%** of total request time.

## Writing Your Own Benchmarks

```go
package mypackage

import (
    "context"
    "testing"
    
    ucontext "github.com/madcok-co/unicorn/pkg/context"
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
go test -bench=BenchmarkContextAcquire -cpuprofile=cpu.prof ./pkg/context/...

# Analyze with pprof
go tool pprof cpu.prof

# Web UI
go tool pprof -http=:8080 cpu.prof
```

### Memory Profiling

```bash
# Generate profile
go test -bench=BenchmarkContextAcquire -memprofile=mem.prof ./pkg/context/...

# Analyze allocations
go tool pprof -alloc_space mem.prof

# Analyze in-use memory
go tool pprof -inuse_space mem.prof
```

### Trace

```bash
# Generate trace
go test -bench=BenchmarkContextParallel -trace=trace.out ./pkg/context/...

# View trace
go tool trace trace.out
```

## Continuous Benchmarking

For tracking performance over time, consider using:

```bash
# Install benchstat
go install golang.org/x/perf/cmd/benchstat@latest

# Run benchmarks and save results
go test -bench=. -benchmem -count=10 ./pkg/context/... > old.txt

# After changes, run again
go test -bench=. -benchmem -count=10 ./pkg/context/... > new.txt

# Compare results
benchstat old.txt new.txt
```

---

*"Premature optimization is the root of all evil, but mature optimization is the root of all performance."*
