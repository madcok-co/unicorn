# Unicorn Framework - Quick Reference Card

## 🚀 Ultra-Low Latency Features

```
Performance:  38 ns/op, 0 allocs/op, 30M+ ops/sec
Pooling:      sync.Pool for context reuse
GC Impact:    Zero allocations on hot path
Concurrency:  Thread-safe with RWMutex
```

## 📊 Benchmark Quick View

| Operation | ns/op | Allocs | Throughput |
|-----------|-------|--------|------------|
| Context Acquire | 38 | 0 | 30M/s |
| JSON Response | 77 | 0 | 15M/s |
| Request Data | 88 | 0 | 14M/s |
| Metadata Ops | 233 | 0 | 5M/s |

## ✅ Do's

```go
// ✅ Let framework manage context
func Handler(ctx *ucontext.Context) error {
    return ctx.JSON(200, data)
}

// ✅ Use defer for manual release
ctx := ucontext.Acquire(bg, adapters)
defer ctx.Release()

// ✅ Reuse context
user := getUser(ctx)
order := createOrder(ctx, user)

// ✅ Pre-size slices
items := make([]Item, 0, expectedCount)

// ✅ Batch operations
ctx.Logger().Info("event", "k1", v1, "k2", v2)
```

## ❌ Don'ts

```go
// ❌ Don't forget to release
ctx := ucontext.Acquire(bg, adapters)
// ... forgot ctx.Release()

// ❌ Don't store context
type Service struct {
    ctx *ucontext.Context  // BAD
}

// ❌ Don't create new contexts
newCtx := ucontext.New(ctx.Context())  // BAD

// ❌ Don't reallocate
var items []Item  // Will grow inefficiently

// ❌ Don't make many small calls
for _, item := range items {
    ctx.Logger().Info("item", item)  // BAD
}
```

## 🔍 Profiling Commands

```bash
# CPU profiling
go test -bench=. -cpuprofile=cpu.prof
go tool pprof cpu.prof

# Memory profiling
go test -bench=. -memprofile=mem.prof
go tool pprof mem.prof

# Production profiling
curl localhost:6060/debug/pprof/profile?seconds=30 > cpu.prof
curl localhost:6060/debug/pprof/heap > mem.prof
```

## 📈 Framework Comparison

| Framework | ns/op | Allocs | Notes |
|-----------|-------|--------|-------|
| **Unicorn** | **38** | **0** | Object pooling |
| Fiber | 50 | 0-1 | fasthttp-based |
| Echo | 120 | 1-2 | Popular |
| Gin | 150 | 2-3 | Most used |
| Chi | 140 | 1-2 | Lightweight |

## 🎯 Optimization Checklist

- [ ] Use object pooling (automatic)
- [ ] Pre-allocate slices with capacity
- [ ] Batch logger/metric calls
- [ ] Configure connection pools
- [ ] Enable response compression
- [ ] Profile production regularly
- [ ] Monitor pool efficiency
- [ ] Use caching for hot data

## 💡 Key Principles

1. **Reuse > Allocate** - Pool objects when possible
2. **Pre-allocate** - Know your sizes upfront
3. **Batch > Loop** - Reduce overhead
4. **Share > Copy** - Use pointers/references
5. **Defer cleanup** - Always release resources

## 🔗 Documentation Links

- [Full Performance Guide](./PERFORMANCE.md)
- [Ultra-Low Latency Details](./ULTRA_LOW_LATENCY.md)
- [Architecture Overview](./ARCHITECTURE.md)
- [Examples](../core/examples/)
