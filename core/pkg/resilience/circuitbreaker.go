// Package resilience provides fault tolerance patterns for production systems
package resilience

import (
	"context"
	"errors"
	"sync"
	"time"
)

// State represents the circuit breaker state
type State int

const (
	StateClosed   State = iota // Normal operation, requests pass through
	StateOpen                  // Circuit is open, requests fail immediately
	StateHalfOpen              // Testing if service recovered
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

var (
	ErrCircuitOpen    = errors.New("circuit breaker is open")
	ErrTooManyRequest = errors.New("too many requests in half-open state")
)

// CircuitBreakerConfig defines configuration for circuit breaker
type CircuitBreakerConfig struct {
	// Name of the circuit breaker (for logging/metrics)
	Name string

	// MaxRequests is max number of requests allowed in half-open state
	MaxRequests uint32

	// Interval is the cyclic period of the closed state for clearing internal counts
	// If 0, internal counts are never cleared in closed state
	Interval time.Duration

	// Timeout is the period of open state before changing to half-open
	Timeout time.Duration

	// ReadyToTrip is called with a copy of Counts when a request fails in closed state
	// Returns true if circuit should trip to open state
	ReadyToTrip func(counts Counts) bool

	// OnStateChange is called when state changes
	OnStateChange func(name string, from, to State)

	// IsSuccessful returns true if the error should be counted as success
	// Default: err == nil
	IsSuccessful func(err error) bool
}

// Counts holds the numbers of requests and their successes/failures
type Counts struct {
	Requests             uint32
	TotalSuccesses       uint32
	TotalFailures        uint32
	ConsecutiveSuccesses uint32
	ConsecutiveFailures  uint32
}

func (c *Counts) onRequest() {
	c.Requests++
}

func (c *Counts) onSuccess() {
	c.TotalSuccesses++
	c.ConsecutiveSuccesses++
	c.ConsecutiveFailures = 0
}

func (c *Counts) onFailure() {
	c.TotalFailures++
	c.ConsecutiveFailures++
	c.ConsecutiveSuccesses = 0
}

func (c *Counts) clear() {
	c.Requests = 0
	c.TotalSuccesses = 0
	c.TotalFailures = 0
	c.ConsecutiveSuccesses = 0
	c.ConsecutiveFailures = 0
}

// DefaultCircuitBreakerConfig returns sensible defaults
func DefaultCircuitBreakerConfig() *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		Name:        "default",
		MaxRequests: 1,
		Interval:    0,
		Timeout:     60 * time.Second,
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures > 5
		},
		IsSuccessful: func(err error) bool {
			return err == nil
		},
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	config *CircuitBreakerConfig

	mu         sync.Mutex
	state      State
	generation uint64
	counts     Counts
	expiry     time.Time
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config *CircuitBreakerConfig) *CircuitBreaker {
	if config == nil {
		config = DefaultCircuitBreakerConfig()
	}

	if config.MaxRequests == 0 {
		config.MaxRequests = 1
	}
	if config.Timeout == 0 {
		config.Timeout = 60 * time.Second
	}
	if config.ReadyToTrip == nil {
		config.ReadyToTrip = func(counts Counts) bool {
			return counts.ConsecutiveFailures > 5
		}
	}
	if config.IsSuccessful == nil {
		config.IsSuccessful = func(err error) bool {
			return err == nil
		}
	}

	cb := &CircuitBreaker{
		config: config,
		state:  StateClosed,
	}

	cb.toNewGeneration(time.Now())

	return cb
}

// Name returns the name of the circuit breaker
func (cb *CircuitBreaker) Name() string {
	return cb.config.Name
}

// State returns current state
func (cb *CircuitBreaker) State() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()
	state, _ := cb.currentState(now)
	return state
}

// Counts returns current counts
func (cb *CircuitBreaker) Counts() Counts {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	return cb.counts
}

// Execute runs the given function if circuit allows
func (cb *CircuitBreaker) Execute(fn func() error) error {
	generation, err := cb.beforeRequest()
	if err != nil {
		return err
	}

	defer func() {
		e := recover()
		if e != nil {
			cb.afterRequest(generation, false)
			panic(e)
		}
	}()

	result := fn()
	cb.afterRequest(generation, cb.config.IsSuccessful(result))
	return result
}

// ExecuteWithContext runs the given function with context
func (cb *CircuitBreaker) ExecuteWithContext(ctx context.Context, fn func(context.Context) error) error {
	generation, err := cb.beforeRequest()
	if err != nil {
		return err
	}

	defer func() {
		e := recover()
		if e != nil {
			cb.afterRequest(generation, false)
			panic(e)
		}
	}()

	result := fn(ctx)
	cb.afterRequest(generation, cb.config.IsSuccessful(result))
	return result
}

func (cb *CircuitBreaker) beforeRequest() (uint64, error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()
	state, generation := cb.currentState(now)

	if state == StateOpen {
		return generation, ErrCircuitOpen
	}

	if state == StateHalfOpen && cb.counts.Requests >= cb.config.MaxRequests {
		return generation, ErrTooManyRequest
	}

	cb.counts.onRequest()
	return generation, nil
}

func (cb *CircuitBreaker) afterRequest(before uint64, success bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()
	state, generation := cb.currentState(now)

	if generation != before {
		return
	}

	if success {
		cb.onSuccess(state, now)
	} else {
		cb.onFailure(state, now)
	}
}

func (cb *CircuitBreaker) onSuccess(state State, now time.Time) {
	switch state {
	case StateClosed:
		cb.counts.onSuccess()
	case StateHalfOpen:
		cb.counts.onSuccess()
		if cb.counts.ConsecutiveSuccesses >= cb.config.MaxRequests {
			cb.setState(StateClosed, now)
		}
	}
}

func (cb *CircuitBreaker) onFailure(state State, now time.Time) {
	switch state {
	case StateClosed:
		cb.counts.onFailure()
		if cb.config.ReadyToTrip(cb.counts) {
			cb.setState(StateOpen, now)
		}
	case StateHalfOpen:
		cb.setState(StateOpen, now)
	}
}

func (cb *CircuitBreaker) currentState(now time.Time) (State, uint64) {
	switch cb.state {
	case StateClosed:
		if !cb.expiry.IsZero() && cb.expiry.Before(now) {
			cb.toNewGeneration(now)
		}
	case StateOpen:
		if cb.expiry.Before(now) {
			cb.setState(StateHalfOpen, now)
		}
	}
	return cb.state, cb.generation
}

func (cb *CircuitBreaker) setState(state State, now time.Time) {
	if cb.state == state {
		return
	}

	prev := cb.state
	cb.state = state

	cb.toNewGeneration(now)

	if cb.config.OnStateChange != nil {
		cb.config.OnStateChange(cb.config.Name, prev, state)
	}
}

func (cb *CircuitBreaker) toNewGeneration(now time.Time) {
	cb.generation++
	cb.counts.clear()

	var zero time.Time
	switch cb.state {
	case StateClosed:
		if cb.config.Interval == 0 {
			cb.expiry = zero
		} else {
			cb.expiry = now.Add(cb.config.Interval)
		}
	case StateOpen:
		cb.expiry = now.Add(cb.config.Timeout)
	default:
		cb.expiry = zero
	}
}

// ============ Circuit Breaker Registry ============

// CircuitBreakerRegistry manages multiple circuit breakers
type CircuitBreakerRegistry struct {
	mu       sync.RWMutex
	breakers map[string]*CircuitBreaker
	config   *CircuitBreakerConfig
}

// NewCircuitBreakerRegistry creates a registry with default config
func NewCircuitBreakerRegistry(defaultConfig *CircuitBreakerConfig) *CircuitBreakerRegistry {
	if defaultConfig == nil {
		defaultConfig = DefaultCircuitBreakerConfig()
	}
	return &CircuitBreakerRegistry{
		breakers: make(map[string]*CircuitBreaker),
		config:   defaultConfig,
	}
}

// Get returns or creates a circuit breaker by name
func (r *CircuitBreakerRegistry) Get(name string) *CircuitBreaker {
	r.mu.RLock()
	cb, ok := r.breakers[name]
	r.mu.RUnlock()

	if ok {
		return cb
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Double check
	if cb, ok = r.breakers[name]; ok {
		return cb
	}

	// Create new with default config
	config := *r.config
	config.Name = name
	cb = NewCircuitBreaker(&config)
	r.breakers[name] = cb

	return cb
}

// GetOrCreate returns existing or creates with custom config
func (r *CircuitBreakerRegistry) GetOrCreate(name string, config *CircuitBreakerConfig) *CircuitBreaker {
	r.mu.RLock()
	cb, ok := r.breakers[name]
	r.mu.RUnlock()

	if ok {
		return cb
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if cb, ok = r.breakers[name]; ok {
		return cb
	}

	if config == nil {
		config = r.config
	}
	config.Name = name
	cb = NewCircuitBreaker(config)
	r.breakers[name] = cb

	return cb
}

// Stats returns stats for all circuit breakers
func (r *CircuitBreakerRegistry) Stats() map[string]struct {
	State  string
	Counts Counts
} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := make(map[string]struct {
		State  string
		Counts Counts
	})

	for name, cb := range r.breakers {
		stats[name] = struct {
			State  string
			Counts Counts
		}{
			State:  cb.State().String(),
			Counts: cb.Counts(),
		}
	}

	return stats
}
