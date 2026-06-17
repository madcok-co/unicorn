# Benchmark

> **Catatan**: Benchmark dimaksudkan untuk menunjukkan karakteristik performa relatif, bukan angka absolut. Hasil kamu mungkin berbeda tergantung hardware dan workload.

## Menjalankan Benchmark

```bash
# Jalankan semua benchmark
go test -bench=. -benchmem ./core/pkg/context/... ./core/pkg/handler/... ./core/pkg/middleware/... ./core/pkg/app/...

# Package spesifik
go test -bench=. -benchmem ./core/pkg/context/...

# Benchmark tertentu
go test -bench=BenchmarkContextAcquire -benchmem ./core/pkg/context/...

# Dengan lebih banyak iterasi
go test -bench=. -benchmem -count=5 ./core/pkg/context/...

# CPU profiling
go test -bench=BenchmarkContextAcquire -cpuprofile=cpu.prof ./core/pkg/context/...

# Memory profiling
go test -bench=BenchmarkContextAcquire -memprofile=mem.prof ./core/pkg/context/...
```

## Hasil Terbaru

**Environment:**
- OS: Linux
- Arch: amd64
- CPU: Intel Core i7-9700 @ 3.00GHz
- Go Version: 1.24+
- Date: June 2026

---

### 1. Context Performance (Hot Path)

Context adalah jalur performa paling kritis — diakuisisi dan dilepaskan di setiap request.

| Benchmark | ns/op | B/op | allocs/op | Deskripsi |
|-----------|------:|-----:|----------:|-----------|
| **ContextAcquire** | **36.94** | **0** | **0** | Ambil context dari pool |
| **ContextNew** | **34.22** | **0** | **0** | Buat context baru (pakai pool) |
| **ContextAcquireWithAccess** | **35.48** | **0** | **0** | Acquire + akses DB/Cache/Logger |
| **ContextMetadata** | **247.0** | **0** | **0** | Set/Get metadata dengan RWMutex |
| **ContextRequest** | **93.67** | **0** | **0** | Set properti request |
| **ContextJSON** | **77.44** | **0** | **0** | Set response JSON |
| **ContextParallel** | **260.3** | **336** | **2** | 10 goroutine paralel |

**Temuan penting:**
- **36.94ns** per siklus acquire/release
- **Zero alokasi** di semua jalur single-goroutine
- Alokasi Parallel (336 B/2 allocs) dari slice growth benchmark, bukan dari context

---

### 2. Handler Performance

| Benchmark | ns/op | B/op | allocs/op | Deskripsi |
|-----------|------:|-----:|----------:|-----------|
| **New** | **12.68** | **0** | **0** | Buat handler dari fungsi |
| **Handler_HTTP** | **90.43** | **80** | **2** | Tambah trigger HTTP ke handler |
| **Registry_Register** | **986.3** | **1,024** | **15** | Daftarkan handler di registry |
| **Registry_GetHTTPHandler** | **183.7** | **48** | **3** | Ambil handler berdasarkan route |
| **Registry_ConcurrentReads** | **5,560** | **6,952** | **11** | Baca registry konkuren (10 goroutine) |

---

### 3. Middleware Performance

| Benchmark | ns/op | B/op | allocs/op | Deskripsi |
|-----------|------:|-----:|----------:|-----------|
| **Recovery** (tanpa panic) | **6.69** | **0** | **0** | Tanpa panic — pass-through |
| **Recovery** (dengan panic) | **394.3** | **336** | **2** | Actual panic recovery |
| **CORS** (simple) | **35.31** | **0** | **0** | Non-preflight CORS |
| **CORS** (preflight) | **1,094** | **1,472** | **14** | OPTIONS preflight |
| **RateLimit** | **456.1** | **305** | **4** | Rate limit middleware |
| **MemoryRateLimitStore** | **94.40** | **0** | **0** | Rate limit store (memory) |
| **Compress** (gzip) | **3,057** | **2,481** | **8** | Kompresi response (gzip) |
| **Compress** (brotli) | **3,133** | **2,481** | **8** | Kompresi response (brotli) |
| **HealthCheck** | **12,670** | **2,000** | **21** | Full health check |
| **HealthCheck** (cache) | **2,958** | **1,344** | **11** | Health check dengan cache |
| **Timeout** | **138,823** | **593** | **7** | Timeout middleware |

**Temuan penting:**
- **Recovery dan CORS virtually free** — 6.69ns dan 35.31ns, zero alloc
- Memory rate limit store **zero alloc** di jalur Allow
- Health check dengan cache **4.3x lebih cepat**

---

### 4. App Initialization (Startup)

Benchmark startup — sekali jalan saat aplikasi mulai, bukan per request.

| Benchmark | ns/op | B/op | allocs/op | Deskripsi |
|-----------|------:|-----:|----------:|-----------|
| **New** (app) | **1,113** | **1,432** | **22** | Buat instance app baru |
| **NewContext** | **540.7** | **592** | **9** | Buat context untuk handler |
| **RegisterHandler** | **1,722** | **487** | **12** | Daftarkan handler + trigger |

---

### 5. Idle Memory Footprint

Memory saat aplikasi sudah inisialisasi penuh tapi idle:

| Komponen | Dampak Memory | Catatan |
|----------|--------------|---------|
| Baseline (after GC) | 0.20 MB | Go runtime overhead |
| + Config Management | +0.00 MB | Viper initialization |
| + Multi-tenancy | +0.01 MB | 1 tenant terdaftar |
| + OAuth2 Driver | +0.00 MB | Google provider config |
| + RBAC Driver | +0.00 MB | 2 roles terkonfigurasi |
| **Full App (IDLE)** | **0.21 MB** | Semua enterprise features aktif |

**System Memory Reserved:** 6.96 MB

---

### 6. Binary Size

| Mode Build | Ukuran | Catatan |
|-----------|--------|---------|
| Unstripped | 9.0 MB | Debug symbols lengkap |
| Stripped (`-ldflags="-s -w"`) | 6.2 MB | Build produksi |
| Stripped + UPX | ~2 MB | Artefak deploy terkompresi |

Framework Unicorn sendiri hanya **~600KB**. Sisanya Go runtime + standard library.

---

### 7. Sidecar Overhead

| Metrik | Nilai |
|--------|-------|
| Biaya goroutine per sidecar | ~8 KB (stack) |
| Alokasi `startSidecar()` | 0 B (no heap alloc) |
| Watchdog goroutine | ~8 KB (hanya saat sidecar stuck) |
| Management server memory | ~0.5 MB idle |

**Sidecar mode zero heap allocation.**

---

## Ringkasan

| Klaim | Hasil Benchmark | Status |
|-------|----------------|--------|
| **~38ns** context acquire | **36.94 ns** | ✅ Lebih baik dari klaim |
| **0 B/op** hot path | **0 B/op** | ✅ Terverifikasi |
| Object pooling | `sync.Pool` + map reuse | ✅ Terimplementasi |
| Lazy adapter injection | Pointer reference, no copy | ✅ Terverifikasi |
| Framework overhead < 0.001% | ~134ns vs 50ms request | ✅ Tidak signifikan |

---

*"Yang penting jalan dulu, baru optimize. Tapi kalau udah jalan, ya optimize biar ga malu-maluin."*
