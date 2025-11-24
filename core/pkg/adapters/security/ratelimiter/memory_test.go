package ratelimiter

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestInMemoryRateLimiter(t *testing.T) {
	t.Run("allows requests within limit", func(t *testing.T) {
		rl := NewInMemoryRateLimiter(&InMemoryRateLimiterConfig{
			Limit:  10,
			Window: time.Minute,
			Burst:  0,
		})
		defer rl.Close()

		for i := 0; i < 10; i++ {
			allowed, err := rl.Allow(context.Background(), "test-key")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !allowed {
				t.Errorf("request %d should be allowed", i)
			}
		}
	})

	t.Run("blocks requests over limit", func(t *testing.T) {
		rl := NewInMemoryRateLimiter(&InMemoryRateLimiterConfig{
			Limit:  5,
			Window: time.Minute,
			Burst:  0,
		})
		defer rl.Close()

		// Use all tokens
		for i := 0; i < 5; i++ {
			rl.Allow(context.Background(), "test-key")
		}

		// Next request should be blocked
		allowed, err := rl.Allow(context.Background(), "test-key")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if allowed {
			t.Error("request should be blocked")
		}
	})

	t.Run("allows burst", func(t *testing.T) {
		rl := NewInMemoryRateLimiter(&InMemoryRateLimiterConfig{
			Limit:  5,
			Window: time.Minute,
			Burst:  3,
		})
		defer rl.Close()

		// Should allow limit + burst = 8 requests
		for i := 0; i < 8; i++ {
			allowed, _ := rl.Allow(context.Background(), "test-key")
			if !allowed {
				t.Errorf("request %d should be allowed (within burst)", i)
			}
		}

		// 9th request should be blocked
		allowed, _ := rl.Allow(context.Background(), "test-key")
		if allowed {
			t.Error("request should be blocked after burst")
		}
	})

	t.Run("tokens refill over time", func(t *testing.T) {
		rl := NewInMemoryRateLimiter(&InMemoryRateLimiterConfig{
			Limit:           100, // 100 tokens per second
			Window:          1 * time.Second,
			Burst:           0,
			CleanupInterval: time.Minute,
		})
		defer rl.Close()

		// Use 50 tokens
		for i := 0; i < 50; i++ {
			rl.Allow(context.Background(), "test-key")
		}

		// Wait for some refill (500ms = 50 tokens at 100/sec rate)
		time.Sleep(600 * time.Millisecond)

		// Should have tokens refilled
		remaining, _ := rl.Remaining(context.Background(), "test-key")
		if remaining < 50 {
			t.Errorf("tokens should have refilled, got %d remaining", remaining)
		}
	})

	t.Run("separate limits per key", func(t *testing.T) {
		rl := NewInMemoryRateLimiter(&InMemoryRateLimiterConfig{
			Limit:  5,
			Window: time.Minute,
			Burst:  0,
		})
		defer rl.Close()

		// Use all tokens for key1
		for i := 0; i < 5; i++ {
			rl.Allow(context.Background(), "key1")
		}

		// key2 should still have tokens
		allowed, _ := rl.Allow(context.Background(), "key2")
		if !allowed {
			t.Error("key2 should have its own limit")
		}
	})

	t.Run("AllowN allows batch requests", func(t *testing.T) {
		rl := NewInMemoryRateLimiter(&InMemoryRateLimiterConfig{
			Limit:  10,
			Window: time.Minute,
			Burst:  0,
		})
		defer rl.Close()

		allowed, _ := rl.AllowN(context.Background(), "test-key", 5)
		if !allowed {
			t.Error("batch of 5 should be allowed")
		}

		allowed, _ = rl.AllowN(context.Background(), "test-key", 6)
		if allowed {
			t.Error("batch of 6 should be blocked (only 5 remaining)")
		}
	})

	t.Run("Remaining returns correct count", func(t *testing.T) {
		rl := NewInMemoryRateLimiter(&InMemoryRateLimiterConfig{
			Limit:  10,
			Window: time.Minute,
			Burst:  5,
		})
		defer rl.Close()

		remaining, _ := rl.Remaining(context.Background(), "test-key")
		if remaining != 15 { // limit + burst
			t.Errorf("expected 15 remaining, got %d", remaining)
		}

		rl.AllowN(context.Background(), "test-key", 7)

		remaining, _ = rl.Remaining(context.Background(), "test-key")
		if remaining != 8 {
			t.Errorf("expected 8 remaining, got %d", remaining)
		}
	})

	t.Run("Reset clears limit for key", func(t *testing.T) {
		rl := NewInMemoryRateLimiter(&InMemoryRateLimiterConfig{
			Limit:  5,
			Window: time.Minute,
			Burst:  0,
		})
		defer rl.Close()

		// Use all tokens
		for i := 0; i < 5; i++ {
			rl.Allow(context.Background(), "test-key")
		}

		// Reset
		rl.Reset(context.Background(), "test-key")

		// Should have tokens again
		allowed, _ := rl.Allow(context.Background(), "test-key")
		if !allowed {
			t.Error("should have tokens after reset")
		}
	})
}

func TestInMemoryRateLimiter_Concurrent(t *testing.T) {
	rl := NewInMemoryRateLimiter(&InMemoryRateLimiterConfig{
		Limit:  1000,
		Window: time.Minute,
		Burst:  100,
	})
	defer rl.Close()

	var wg sync.WaitGroup
	allowedCount := 0
	var mu sync.Mutex

	// 500 concurrent requests
	for i := 0; i < 500; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			allowed, _ := rl.Allow(context.Background(), "concurrent-key")
			if allowed {
				mu.Lock()
				allowedCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// All 500 should be allowed (within limit + burst = 1100)
	if allowedCount != 500 {
		t.Errorf("expected 500 allowed, got %d", allowedCount)
	}
}

func TestSlidingWindowRateLimiter(t *testing.T) {
	t.Run("allows requests within limit", func(t *testing.T) {
		rl := NewSlidingWindowRateLimiter(&InMemoryRateLimiterConfig{
			Limit:  10,
			Window: time.Minute,
			Burst:  0,
		})
		defer rl.Close()

		for i := 0; i < 10; i++ {
			allowed, err := rl.Allow(context.Background(), "test-key")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !allowed {
				t.Errorf("request %d should be allowed", i)
			}
		}
	})

	t.Run("blocks requests over limit", func(t *testing.T) {
		rl := NewSlidingWindowRateLimiter(&InMemoryRateLimiterConfig{
			Limit:  5,
			Window: time.Minute,
			Burst:  0,
		})
		defer rl.Close()

		for i := 0; i < 5; i++ {
			rl.Allow(context.Background(), "test-key")
		}

		allowed, _ := rl.Allow(context.Background(), "test-key")
		if allowed {
			t.Error("request should be blocked")
		}
	})

	t.Run("old requests expire from window", func(t *testing.T) {
		rl := NewSlidingWindowRateLimiter(&InMemoryRateLimiterConfig{
			Limit:  5,
			Window: 50 * time.Millisecond,
			Burst:  0,
		})
		defer rl.Close()

		// Use all tokens
		for i := 0; i < 5; i++ {
			rl.Allow(context.Background(), "test-key")
		}

		// Wait for window to pass
		time.Sleep(60 * time.Millisecond)

		// Old requests should have expired
		allowed, _ := rl.Allow(context.Background(), "test-key")
		if !allowed {
			t.Error("old requests should have expired")
		}
	})
}
