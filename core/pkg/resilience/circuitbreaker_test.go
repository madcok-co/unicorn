package resilience

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCircuitBreaker(t *testing.T) {
	t.Run("starts in closed state", func(t *testing.T) {
		cb := NewCircuitBreaker(nil)

		if cb.State() != StateClosed {
			t.Errorf("expected StateClosed, got %v", cb.State())
		}
	})

	t.Run("allows requests when closed", func(t *testing.T) {
		cb := NewCircuitBreaker(nil)

		err := cb.Execute(func() error {
			return nil
		})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("opens after consecutive failures", func(t *testing.T) {
		config := &CircuitBreakerConfig{
			MaxRequests: 1,
			Timeout:     1 * time.Second,
			ReadyToTrip: func(counts Counts) bool {
				return counts.ConsecutiveFailures >= 3
			},
		}
		cb := NewCircuitBreaker(config)

		testErr := errors.New("test error")

		// Fail 3 times
		for i := 0; i < 3; i++ {
			_ = cb.Execute(func() error {
				return testErr
			})
		}

		if cb.State() != StateOpen {
			t.Errorf("expected StateOpen, got %v", cb.State())
		}
	})

	t.Run("rejects requests when open", func(t *testing.T) {
		config := &CircuitBreakerConfig{
			MaxRequests: 1,
			Timeout:     10 * time.Second,
			ReadyToTrip: func(counts Counts) bool {
				return counts.ConsecutiveFailures >= 1
			},
		}
		cb := NewCircuitBreaker(config)

		// Trip the circuit
		_ = cb.Execute(func() error {
			return errors.New("fail")
		})

		// Next request should be rejected
		err := cb.Execute(func() error {
			return nil
		})

		if !errors.Is(err, ErrCircuitOpen) {
			t.Errorf("expected ErrCircuitOpen, got %v", err)
		}
	})

	t.Run("transitions to half-open after timeout", func(t *testing.T) {
		config := &CircuitBreakerConfig{
			MaxRequests: 1,
			Timeout:     50 * time.Millisecond,
			ReadyToTrip: func(counts Counts) bool {
				return counts.ConsecutiveFailures >= 1
			},
		}
		cb := NewCircuitBreaker(config)

		// Trip the circuit
		_ = cb.Execute(func() error {
			return errors.New("fail")
		})

		// Wait for timeout
		time.Sleep(100 * time.Millisecond)

		if cb.State() != StateHalfOpen {
			t.Errorf("expected StateHalfOpen, got %v", cb.State())
		}
	})

	t.Run("closes after successful request in half-open", func(t *testing.T) {
		config := &CircuitBreakerConfig{
			MaxRequests: 1,
			Timeout:     50 * time.Millisecond,
			ReadyToTrip: func(counts Counts) bool {
				return counts.ConsecutiveFailures >= 1
			},
		}
		cb := NewCircuitBreaker(config)

		// Trip the circuit
		_ = cb.Execute(func() error {
			return errors.New("fail")
		})

		// Wait for half-open
		time.Sleep(100 * time.Millisecond)

		// Successful request
		err := cb.Execute(func() error {
			return nil
		})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if cb.State() != StateClosed {
			t.Errorf("expected StateClosed, got %v", cb.State())
		}
	})

	t.Run("reopens after failure in half-open", func(t *testing.T) {
		config := &CircuitBreakerConfig{
			MaxRequests: 1,
			Timeout:     50 * time.Millisecond,
			ReadyToTrip: func(counts Counts) bool {
				return counts.ConsecutiveFailures >= 1
			},
		}
		cb := NewCircuitBreaker(config)

		// Trip the circuit
		_ = cb.Execute(func() error {
			return errors.New("fail")
		})

		// Wait for half-open
		time.Sleep(100 * time.Millisecond)

		// Failed request
		_ = cb.Execute(func() error {
			return errors.New("fail again")
		})

		if cb.State() != StateOpen {
			t.Errorf("expected StateOpen, got %v", cb.State())
		}
	})

	t.Run("concurrent access", func(t *testing.T) {
		cb := NewCircuitBreaker(nil)

		var wg sync.WaitGroup
		var successCount int64

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := cb.Execute(func() error {
					return nil
				})
				if err == nil {
					atomic.AddInt64(&successCount, 1)
				}
			}()
		}

		wg.Wait()

		if successCount != 100 {
			t.Errorf("expected 100 successes, got %d", successCount)
		}
	})

	t.Run("calls OnStateChange callback", func(t *testing.T) {
		var stateChanges []struct{ from, to State }

		config := &CircuitBreakerConfig{
			MaxRequests: 1,
			Timeout:     50 * time.Millisecond,
			ReadyToTrip: func(counts Counts) bool {
				return counts.ConsecutiveFailures >= 1
			},
			OnStateChange: func(name string, from, to State) {
				stateChanges = append(stateChanges, struct{ from, to State }{from, to})
			},
		}
		cb := NewCircuitBreaker(config)

		// Trip the circuit
		_ = cb.Execute(func() error {
			return errors.New("fail")
		})

		if len(stateChanges) != 1 {
			t.Errorf("expected 1 state change, got %d", len(stateChanges))
		}

		if stateChanges[0].from != StateClosed || stateChanges[0].to != StateOpen {
			t.Errorf("expected Closed->Open, got %v->%v", stateChanges[0].from, stateChanges[0].to)
		}
	})
}

func TestCircuitBreakerRegistry(t *testing.T) {
	t.Run("creates and retrieves circuit breakers", func(t *testing.T) {
		registry := NewCircuitBreakerRegistry(nil)

		cb1 := registry.Get("service-a")
		cb2 := registry.Get("service-a")
		cb3 := registry.Get("service-b")

		if cb1 != cb2 {
			t.Error("expected same circuit breaker for same name")
		}

		if cb1 == cb3 {
			t.Error("expected different circuit breakers for different names")
		}
	})

	t.Run("returns stats", func(t *testing.T) {
		registry := NewCircuitBreakerRegistry(nil)

		cb := registry.Get("test-service")
		_ = cb.Execute(func() error { return nil })

		stats := registry.Stats()

		if _, ok := stats["test-service"]; !ok {
			t.Error("expected stats for test-service")
		}
	})
}
