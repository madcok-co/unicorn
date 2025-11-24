package resilience

import (
	"context"
	"errors"
	"math/rand"
	"time"
)

// RetryConfig defines configuration for retry mechanism
type RetryConfig struct {
	// MaxAttempts is the maximum number of attempts (including first try)
	MaxAttempts int

	// InitialInterval is the initial wait interval
	InitialInterval time.Duration

	// MaxInterval is the maximum wait interval (cap)
	MaxInterval time.Duration

	// Multiplier is the multiplier for exponential backoff
	Multiplier float64

	// RandomizationFactor adds jitter to intervals (0 = no jitter, 0.5 = +/- 50%)
	RandomizationFactor float64

	// RetryIf returns true if the error should trigger a retry
	RetryIf func(err error) bool

	// OnRetry is called on each retry attempt
	OnRetry func(attempt int, err error, delay time.Duration)
}

// DefaultRetryConfig returns sensible defaults
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:         3,
		InitialInterval:     100 * time.Millisecond,
		MaxInterval:         10 * time.Second,
		Multiplier:          2.0,
		RandomizationFactor: 0.5,
		RetryIf: func(err error) bool {
			return err != nil
		},
	}
}

// Retryer handles retry logic with exponential backoff
type Retryer struct {
	config *RetryConfig
}

// NewRetryer creates a new retryer
func NewRetryer(config *RetryConfig) *Retryer {
	if config == nil {
		config = DefaultRetryConfig()
	}

	if config.MaxAttempts <= 0 {
		config.MaxAttempts = 3
	}
	if config.InitialInterval <= 0 {
		config.InitialInterval = 100 * time.Millisecond
	}
	if config.MaxInterval <= 0 {
		config.MaxInterval = 10 * time.Second
	}
	if config.Multiplier <= 0 {
		config.Multiplier = 2.0
	}
	if config.RetryIf == nil {
		config.RetryIf = func(err error) bool { return err != nil }
	}

	return &Retryer{config: config}
}

// Do executes the function with retry logic
func (r *Retryer) Do(fn func() error) error {
	return r.DoWithContext(context.Background(), func(ctx context.Context) error {
		return fn()
	})
}

// DoWithContext executes the function with retry logic and context
func (r *Retryer) DoWithContext(ctx context.Context, fn func(context.Context) error) error {
	var lastErr error
	interval := r.config.InitialInterval

	for attempt := 1; attempt <= r.config.MaxAttempts; attempt++ {
		// Check context before each attempt
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Execute function
		err := fn(ctx)
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if we should retry
		if !r.config.RetryIf(err) {
			return err
		}

		// Don't wait if this was the last attempt
		if attempt == r.config.MaxAttempts {
			break
		}

		// Calculate delay with jitter
		delay := r.calculateDelay(interval)

		// Call OnRetry callback
		if r.config.OnRetry != nil {
			r.config.OnRetry(attempt, err, delay)
		}

		// Wait before next attempt
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}

		// Increase interval for next attempt
		interval = r.nextInterval(interval)
	}

	return &RetryError{
		Err:      lastErr,
		Attempts: r.config.MaxAttempts,
	}
}

func (r *Retryer) calculateDelay(interval time.Duration) time.Duration {
	if r.config.RandomizationFactor == 0 {
		return interval
	}

	// Add jitter
	delta := r.config.RandomizationFactor * float64(interval)
	minInterval := float64(interval) - delta
	maxInterval := float64(interval) + delta

	// Random value between min and max
	jitter := minInterval + (rand.Float64() * (maxInterval - minInterval))
	return time.Duration(jitter)
}

func (r *Retryer) nextInterval(current time.Duration) time.Duration {
	next := time.Duration(float64(current) * r.config.Multiplier)
	if next > r.config.MaxInterval {
		return r.config.MaxInterval
	}
	return next
}

// RetryError is returned when all retry attempts fail
type RetryError struct {
	Err      error
	Attempts int
}

func (e *RetryError) Error() string {
	return e.Err.Error()
}

func (e *RetryError) Unwrap() error {
	return e.Err
}

// ============ Convenience Functions ============

// Retry executes fn with default retry config
func Retry(fn func() error) error {
	return NewRetryer(nil).Do(fn)
}

// RetryWithContext executes fn with default retry config and context
func RetryWithContext(ctx context.Context, fn func(context.Context) error) error {
	return NewRetryer(nil).DoWithContext(ctx, fn)
}

// RetryN executes fn up to n times
func RetryN(n int, fn func() error) error {
	return NewRetryer(&RetryConfig{
		MaxAttempts:     n,
		InitialInterval: 100 * time.Millisecond,
		Multiplier:      2.0,
	}).Do(fn)
}

// RetryWithBackoff executes fn with custom backoff settings
func RetryWithBackoff(maxAttempts int, initialInterval, maxInterval time.Duration, fn func() error) error {
	return NewRetryer(&RetryConfig{
		MaxAttempts:     maxAttempts,
		InitialInterval: initialInterval,
		MaxInterval:     maxInterval,
		Multiplier:      2.0,
	}).Do(fn)
}

// ============ Retry Conditions ============

// RetryableErrors returns a RetryIf that retries on specific errors
func RetryableErrors(errs ...error) func(error) bool {
	return func(err error) bool {
		for _, e := range errs {
			if errors.Is(err, e) {
				return true
			}
		}
		return false
	}
}

// NonRetryableErrors returns a RetryIf that doesn't retry on specific errors
func NonRetryableErrors(errs ...error) func(error) bool {
	return func(err error) bool {
		for _, e := range errs {
			if errors.Is(err, e) {
				return false
			}
		}
		return err != nil
	}
}

// ============ Combined Circuit Breaker + Retry ============

// ExecuteWithRetry combines circuit breaker and retry
func (cb *CircuitBreaker) ExecuteWithRetry(retryer *Retryer, fn func() error) error {
	return retryer.Do(func() error {
		return cb.Execute(fn)
	})
}

// ExecuteWithRetryContext combines circuit breaker and retry with context
func (cb *CircuitBreaker) ExecuteWithRetryContext(ctx context.Context, retryer *Retryer, fn func(context.Context) error) error {
	return retryer.DoWithContext(ctx, func(ctx context.Context) error {
		return cb.ExecuteWithContext(ctx, fn)
	})
}

// ============ Bulkhead Pattern ============

// Bulkhead limits concurrent executions
type Bulkhead struct {
	sem     chan struct{}
	timeout time.Duration
}

// NewBulkhead creates a new bulkhead with max concurrent executions
func NewBulkhead(maxConcurrent int, timeout time.Duration) *Bulkhead {
	return &Bulkhead{
		sem:     make(chan struct{}, maxConcurrent),
		timeout: timeout,
	}
}

// ErrBulkheadFull is returned when bulkhead is at capacity
var ErrBulkheadFull = errors.New("bulkhead is full")

// Execute runs the function if capacity allows
func (b *Bulkhead) Execute(fn func() error) error {
	return b.ExecuteWithContext(context.Background(), func(ctx context.Context) error {
		return fn()
	})
}

// ExecuteWithContext runs the function with context if capacity allows
func (b *Bulkhead) ExecuteWithContext(ctx context.Context, fn func(context.Context) error) error {
	// Try to acquire semaphore
	select {
	case b.sem <- struct{}{}:
		defer func() { <-b.sem }()
		return fn(ctx)
	case <-time.After(b.timeout):
		return ErrBulkheadFull
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Available returns number of available slots
func (b *Bulkhead) Available() int {
	return cap(b.sem) - len(b.sem)
}

// ============ Fallback Pattern ============

// WithFallback executes fn and falls back to fallbackFn on error
func WithFallback[T any](fn func() (T, error), fallbackFn func(error) (T, error)) (T, error) {
	result, err := fn()
	if err != nil {
		return fallbackFn(err)
	}
	return result, nil
}

// WithFallbackValue executes fn and returns fallbackValue on error
func WithFallbackValue[T any](fn func() (T, error), fallbackValue T) (T, error) {
	result, err := fn()
	if err != nil {
		return fallbackValue, nil
	}
	return result, nil
}

// ============ Timeout Pattern ============

// WithTimeout executes fn with timeout
func WithTimeout(timeout time.Duration, fn func() error) error {
	return WithTimeoutContext(context.Background(), timeout, func(ctx context.Context) error {
		return fn()
	})
}

// WithTimeoutContext executes fn with timeout and parent context
func WithTimeoutContext(parent context.Context, timeout time.Duration, fn func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- fn(ctx)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// init seeds random for jitter
func init() {
	rand.Seed(time.Now().UnixNano())
}
