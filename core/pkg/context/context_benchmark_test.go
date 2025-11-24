package context

import (
	"context"
	"testing"
)

// BenchmarkContextAcquire benchmarks the optimized context acquisition with pooling
func BenchmarkContextAcquire(b *testing.B) {
	adapters := &AppAdapters{}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx := Acquire(context.Background(), adapters)
		ctx.Release()
	}
}

// BenchmarkContextNew benchmarks creating new context (still uses pool internally)
func BenchmarkContextNew(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx := New(context.Background())
		ctx.Release()
	}
}

// BenchmarkContextAcquireWithAccess benchmarks context with adapter access
func BenchmarkContextAcquireWithAccess(b *testing.B) {
	adapters := &AppAdapters{}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx := Acquire(context.Background(), adapters)
		// Simulate typical handler access pattern
		_ = ctx.DB()
		_ = ctx.Cache()
		_ = ctx.Logger()
		ctx.Release()
	}
}

// BenchmarkContextMetadata benchmarks metadata operations
func BenchmarkContextMetadata(b *testing.B) {
	adapters := &AppAdapters{}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx := Acquire(context.Background(), adapters)
		ctx.Set("key1", "value1")
		ctx.Set("key2", 123)
		ctx.Set("key3", true)
		_, _ = ctx.Get("key1")
		_, _ = ctx.Get("key2")
		ctx.Release()
	}
}

// BenchmarkContextRequest benchmarks request data operations
func BenchmarkContextRequest(b *testing.B) {
	adapters := &AppAdapters{}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx := Acquire(context.Background(), adapters)
		req := ctx.Request()
		req.Method = "POST"
		req.Path = "/api/users"
		req.Headers["Content-Type"] = "application/json"
		req.Headers["Authorization"] = "Bearer token"
		ctx.Release()
	}
}

// BenchmarkContextJSON benchmarks JSON response setting
func BenchmarkContextJSON(b *testing.B) {
	adapters := &AppAdapters{}
	data := map[string]interface{}{
		"id":    1,
		"name":  "test",
		"email": "test@example.com",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx := Acquire(context.Background(), adapters)
		_ = ctx.JSON(200, data)
		ctx.Release()
	}
}

// BenchmarkContextParallel benchmarks parallel context usage
func BenchmarkContextParallel(b *testing.B) {
	adapters := &AppAdapters{}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ctx := Acquire(context.Background(), adapters)
			_ = ctx.DB()
			_ = ctx.Cache()
			ctx.Set("request_id", "123")
			_ = ctx.JSON(200, map[string]string{"status": "ok"})
			ctx.Release()
		}
	})
}
