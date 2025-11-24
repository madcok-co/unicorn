# Benchmarks

> **Catatan**: Benchmark dimaksudkan untuk menunjukkan karakteristik performa relatif, bukan angka absolut. Hasil kamu mungkin berbeda tergantung hardware dan workload.

## Menjalankan Benchmarks

```bash
# Jalankan semua benchmarks
go test -bench=. -benchmem ./pkg/context/...

# Jalankan benchmark tertentu
go test -bench=BenchmarkContextAcquire -benchmem ./pkg/context/...

# Jalankan dengan lebih banyak iterasi untuk akurasi
go test -bench=. -benchmem -count=5 ./pkg/context/...

# Jalankan dengan CPU profiling
go test -bench=. -benchmem -cpuprofile=cpu.prof ./pkg/context/...

# Jalankan dengan memory profiling
go test -bench=. -benchmem -memprofile=mem.prof ./pkg/context/...
```

## Hasil Terbaru

**Environment:**
- OS: macOS (Darwin)
- Arch: amd64
- CPU: Intel Core i5-8257U @ 1.40GHz

**Hasil:**

| Benchmark | ns/op | B/op | allocs/op | Deskripsi |
|-----------|------:|-----:|----------:|-----------|
| ContextAcquire | 38.26 | 0 | 0 | Ambil context dari pool dengan adapters |
| ContextNew | 37.24 | 0 | 0 | Buat context baru (pakai pool) |
| ContextAcquireWithAccess | 38.96 | 0 | 0 | Acquire + akses DB/Cache/Logger |
| ContextMetadata | 226.8 | 0 | 0 | Set/Get metadata values |
| ContextRequest | 83.81 | 0 | 0 | Set request properties |
| ContextJSON | 72.05 | 0 | 0 | Set JSON response |
| ContextParallel | 152.2 | 336 | 2 | Operasi context paralel |

### Metrik Penting

- **Zero allocation** untuk siklus context acquire/release
- **~38ns** per operasi context (acquire + release)
- **Object pooling** menghilangkan tekanan GC
- **Lazy adapter injection** - tidak ada copying, hanya pointer reference

## Optimisasi Performa

Unicorn menggunakan beberapa teknik untuk mencapai performa tinggi:

### 1. Object Pooling (sync.Pool)

```go
// Object Context di-reuse via sync.Pool
var contextPool = sync.Pool{
    New: func() interface{} {
        return &Context{
            metadata: make(map[string]any, 8),
            // ... pre-allocated maps
        }
    },
}

// Ambil dari pool
ctx := Acquire(context.Background(), adapters)

// Kembalikan ke pool setelah selesai
defer ctx.Release()
```

### 2. Lazy Adapter Injection

```go
// Daripada copy semua adapters per request:
// LAMA: ctx.db = app.db (copy untuk setiap adapter)

// Kita pakai shared reference:
// BARU: ctx.app = app.adapters (single pointer)

func (c *Context) DB() contracts.Database {
    if c.app == nil {
        return nil
    }
    return c.app.DB  // Akses langsung, tidak ada copy
}
```

### 3. Map Reuse

```go
// Maps di-clear, tidak di-reallocate
func (c *Context) reset() {
    // Clear maps (pertahankan capacity)
    for k := range c.metadata {
        delete(c.metadata, k)
    }
    // ... sama untuk maps lainnya
}
```

## Perbandingan dengan Framework Lain

### Perbandingan Teoritis

| Framework | Context Alloc | Catatan |
|-----------|--------------|---------|
| **Unicorn** | 0 B/op | Object pooling + lazy injection |
| **Gin** | 0 B/op | Object pooling |
| **Echo** | ~100 B/op | Alokasi minimal |
| **Fiber** | 0 B/op | Fasthttp + pooling |

### Dampak di Dunia Nyata

Dalam request tipikal dengan database query:

```
Total waktu request: 50ms
├── Database query:      45ms  (90%)
├── Business logic:       4ms  (8%)
├── JSON serialization: 0.5ms  (1%)
└── Framework overhead: 0.5ms  (1%)  ← Unicorn dioptimasi di sini

Breakdown framework overhead:
├── Context acquire:    ~40ns
├── Adapter access:     ~10ns
├── Response set:       ~70ns
└── Context release:    ~10ns
Total: ~130ns = 0.00013ms
```

Framework overhead adalah **< 0.001%** dari total waktu request.

## Menulis Benchmark Sendiri

```go
package mypackage

import (
    "context"
    "testing"
    
    ucontext "github.com/madcok-co/unicorn/pkg/context"
)

func BenchmarkMyHandler(b *testing.B) {
    adapters := &ucontext.AppAdapters{
        // Setup adapters kamu
    }
    
    b.ResetTimer()
    b.ReportAllocs()
    
    for i := 0; i < b.N; i++ {
        ctx := ucontext.Acquire(context.Background(), adapters)
        
        // Logic handler kamu di sini
        _ = ctx.JSON(200, map[string]string{"status": "ok"})
        
        ctx.Release()
    }
}
```

## Tips Profiling

### CPU Profiling

```bash
# Generate profile
go test -bench=BenchmarkContextAcquire -cpuprofile=cpu.prof ./pkg/context/...

# Analisa dengan pprof
go tool pprof cpu.prof

# Web UI
go tool pprof -http=:8080 cpu.prof
```

### Memory Profiling

```bash
# Generate profile
go test -bench=BenchmarkContextAcquire -memprofile=mem.prof ./pkg/context/...

# Analisa allocations
go tool pprof -alloc_space mem.prof

# Analisa in-use memory
go tool pprof -inuse_space mem.prof
```

### Trace

```bash
# Generate trace
go test -bench=BenchmarkContextParallel -trace=trace.out ./pkg/context/...

# Lihat trace
go tool trace trace.out
```

## Continuous Benchmarking

Untuk tracking performa dari waktu ke waktu, pertimbangkan:

```bash
# Install benchstat
go install golang.org/x/perf/cmd/benchstat@latest

# Jalankan benchmarks dan simpan hasil
go test -bench=. -benchmem -count=10 ./pkg/context/... > old.txt

# Setelah perubahan, jalankan lagi
go test -bench=. -benchmem -count=10 ./pkg/context/... > new.txt

# Bandingkan hasil
benchstat old.txt new.txt
```

---

*"Premature optimization is the root of all evil, but mature optimization is the root of all performance."*

*"Yang penting jalan dulu, baru optimize. Tapi kalau udah jalan, ya optimize biar ga malu-maluin." - Developer yang udah capek di-bully soal performa*
