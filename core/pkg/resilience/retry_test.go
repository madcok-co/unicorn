package resilience

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetryer(t *testing.T) {
	t.Run("succeeds on first try", func(t *testing.T) {
		retryer := NewRetryer(nil)

		var attempts int
		err := retryer.Do(func() error {
			attempts++
			return nil
		})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if attempts != 1 {
			t.Errorf("expected 1 attempt, got %d", attempts)
		}
	})

	t.Run("retries on failure", func(t *testing.T) {
		config := &RetryConfig{
			MaxAttempts:     3,
			InitialInterval: 10 * time.Millisecond,
			Multiplier:      1.0,
		}
		retryer := NewRetryer(config)

		var attempts int
		testErr := errors.New("temporary error")

		err := retryer.Do(func() error {
			attempts++
			if attempts < 3 {
				return testErr
			}
			return nil
		})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if attempts != 3 {
			t.Errorf("expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("returns error after max attempts", func(t *testing.T) {
		config := &RetryConfig{
			MaxAttempts:     3,
			InitialInterval: 10 * time.Millisecond,
			Multiplier:      1.0,
		}
		retryer := NewRetryer(config)

		var attempts int
		testErr := errors.New("persistent error")

		err := retryer.Do(func() error {
			attempts++
			return testErr
		})

		if err == nil {
			t.Error("expected error, got nil")
		}

		var retryErr *RetryError
		if !errors.As(err, &retryErr) {
			t.Errorf("expected RetryError, got %T", err)
		}

		if attempts != 3 {
			t.Errorf("expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		config := &RetryConfig{
			MaxAttempts:     10,
			InitialInterval: 100 * time.Millisecond,
			Multiplier:      1.0,
		}
		retryer := NewRetryer(config)

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		var attempts int32
		err := retryer.DoWithContext(ctx, func(ctx context.Context) error {
			atomic.AddInt32(&attempts, 1)
			return errors.New("error")
		})

		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("expected context.DeadlineExceeded, got %v", err)
		}

		// Should have attempted at least once
		if atomic.LoadInt32(&attempts) < 1 {
			t.Error("expected at least 1 attempt")
		}
	})

	t.Run("calls OnRetry callback", func(t *testing.T) {
		var callbacks []int

		config := &RetryConfig{
			MaxAttempts:     3,
			InitialInterval: 10 * time.Millisecond,
			Multiplier:      1.0,
			OnRetry: func(attempt int, err error, delay time.Duration) {
				callbacks = append(callbacks, attempt)
			},
		}
		retryer := NewRetryer(config)

		_ = retryer.Do(func() error {
			return errors.New("error")
		})

		// OnRetry is called after each failed attempt except the last
		if len(callbacks) != 2 {
			t.Errorf("expected 2 callbacks, got %d", len(callbacks))
		}
	})

	t.Run("respects RetryIf condition", func(t *testing.T) {
		retryableErr := errors.New("retryable")
		nonRetryableErr := errors.New("non-retryable")

		config := &RetryConfig{
			MaxAttempts:     3,
			InitialInterval: 10 * time.Millisecond,
			RetryIf: func(err error) bool {
				return errors.Is(err, retryableErr)
			},
		}
		retryer := NewRetryer(config)

		// Should not retry non-retryable error
		var attempts int
		err := retryer.Do(func() error {
			attempts++
			return nonRetryableErr
		})

		if err != nonRetryableErr {
			t.Errorf("expected non-retryable error, got %v", err)
		}

		if attempts != 1 {
			t.Errorf("expected 1 attempt for non-retryable, got %d", attempts)
		}
	})

	t.Run("exponential backoff", func(t *testing.T) {
		config := &RetryConfig{
			MaxAttempts:         4,
			InitialInterval:     10 * time.Millisecond,
			MaxInterval:         100 * time.Millisecond,
			Multiplier:          2.0,
			RandomizationFactor: 0, // No jitter for predictable testing
		}
		retryer := NewRetryer(config)

		start := time.Now()
		_ = retryer.Do(func() error {
			return errors.New("error")
		})
		elapsed := time.Since(start)

		// Expected delays: 10ms + 20ms + 40ms = 70ms (approximately)
		// With some tolerance for execution time
		if elapsed < 50*time.Millisecond || elapsed > 150*time.Millisecond {
			t.Errorf("unexpected total elapsed time: %v", elapsed)
		}
	})
}

func TestBulkhead(t *testing.T) {
	t.Run("limits concurrent executions", func(t *testing.T) {
		bulkhead := NewBulkhead(2, 100*time.Millisecond)

		var running int32
		var maxRunning int32
		done := make(chan struct{})

		for i := 0; i < 5; i++ {
			go func() {
				_ = bulkhead.Execute(func() error {
					current := atomic.AddInt32(&running, 1)
					if current > atomic.LoadInt32(&maxRunning) {
						atomic.StoreInt32(&maxRunning, current)
					}
					time.Sleep(50 * time.Millisecond)
					atomic.AddInt32(&running, -1)
					return nil
				})
				done <- struct{}{}
			}()
		}

		// Wait for all goroutines
		for i := 0; i < 5; i++ {
			<-done
		}

		if maxRunning > 2 {
			t.Errorf("expected max 2 concurrent, got %d", maxRunning)
		}
	})

	t.Run("returns error when full", func(t *testing.T) {
		bulkhead := NewBulkhead(1, 10*time.Millisecond)

		started := make(chan struct{})
		blocked := make(chan struct{})

		// Fill the bulkhead
		go func() {
			_ = bulkhead.Execute(func() error {
				close(started)
				<-blocked
				return nil
			})
		}()

		<-started

		// This should timeout
		err := bulkhead.Execute(func() error {
			return nil
		})

		close(blocked)

		if !errors.Is(err, ErrBulkheadFull) {
			t.Errorf("expected ErrBulkheadFull, got %v", err)
		}
	})

	t.Run("Available returns correct count", func(t *testing.T) {
		bulkhead := NewBulkhead(5, time.Second)

		if bulkhead.Available() != 5 {
			t.Errorf("expected 5 available, got %d", bulkhead.Available())
		}

		blocked := make(chan struct{})
		go func() {
			_ = bulkhead.Execute(func() error {
				<-blocked
				return nil
			})
		}()

		// Give goroutine time to acquire semaphore
		time.Sleep(10 * time.Millisecond)

		if bulkhead.Available() != 4 {
			t.Errorf("expected 4 available, got %d", bulkhead.Available())
		}

		close(blocked)
	})
}

func TestWithTimeout(t *testing.T) {
	t.Run("completes before timeout", func(t *testing.T) {
		err := WithTimeout(100*time.Millisecond, func() error {
			time.Sleep(10 * time.Millisecond)
			return nil
		})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("times out", func(t *testing.T) {
		err := WithTimeout(10*time.Millisecond, func() error {
			time.Sleep(100 * time.Millisecond)
			return nil
		})

		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("expected DeadlineExceeded, got %v", err)
		}
	})
}

func TestConvenienceFunctions(t *testing.T) {
	t.Run("Retry with default config", func(t *testing.T) {
		var attempts int
		err := Retry(func() error {
			attempts++
			if attempts < 2 {
				return errors.New("error")
			}
			return nil
		})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("RetryN", func(t *testing.T) {
		var attempts int
		_ = RetryN(5, func() error {
			attempts++
			return errors.New("error")
		})

		if attempts != 5 {
			t.Errorf("expected 5 attempts, got %d", attempts)
		}
	})
}
