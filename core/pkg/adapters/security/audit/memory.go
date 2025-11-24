package audit

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sort"
	"sync"
	"time"

	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

// InMemoryAuditLoggerConfig adalah konfigurasi untuk in-memory audit logger
type InMemoryAuditLoggerConfig struct {
	// Maximum number of events to keep
	MaxEvents int

	// Retention period for events
	RetentionPeriod time.Duration

	// Cleanup interval
	CleanupInterval time.Duration

	// Buffer size for async logging
	BufferSize int

	// Enable async logging
	Async bool
}

// DefaultInMemoryAuditLoggerConfig returns default configuration
func DefaultInMemoryAuditLoggerConfig() *InMemoryAuditLoggerConfig {
	return &InMemoryAuditLoggerConfig{
		MaxEvents:       10000,
		RetentionPeriod: 7 * 24 * time.Hour, // 7 days
		CleanupInterval: 1 * time.Hour,
		BufferSize:      1000,
		Async:           true,
	}
}

// InMemoryAuditLogger implements AuditLogger using in-memory storage
type InMemoryAuditLogger struct {
	config    *InMemoryAuditLoggerConfig
	events    []*contracts.AuditEvent
	mu        sync.RWMutex
	eventCh   chan *contracts.AuditEvent
	stopCh    chan struct{}
	stopOnce  sync.Once
	wg        sync.WaitGroup
	closed    bool
	closeMu   sync.RWMutex
	handlers  []AuditHandler
	handlerMu sync.RWMutex
}

// AuditHandler is called for each audit event
type AuditHandler func(event *contracts.AuditEvent)

// NewInMemoryAuditLogger creates a new in-memory audit logger
func NewInMemoryAuditLogger(config *InMemoryAuditLoggerConfig) *InMemoryAuditLogger {
	if config == nil {
		config = DefaultInMemoryAuditLoggerConfig()
	}

	// Ensure cleanup interval is set
	if config.CleanupInterval <= 0 {
		config.CleanupInterval = 1 * time.Hour
	}

	logger := &InMemoryAuditLogger{
		config:   config,
		events:   make([]*contracts.AuditEvent, 0, config.MaxEvents),
		stopCh:   make(chan struct{}),
		handlers: make([]AuditHandler, 0),
	}

	if config.Async {
		logger.eventCh = make(chan *contracts.AuditEvent, config.BufferSize)
		logger.wg.Add(1)
		go logger.processEvents()
	}

	// Start cleanup goroutine
	logger.wg.Add(1)
	go logger.cleanupLoop()

	return logger
}

// Log logs an audit event
func (l *InMemoryAuditLogger) Log(ctx context.Context, event *contracts.AuditEvent) error {
	// Check if closed
	l.closeMu.RLock()
	if l.closed {
		l.closeMu.RUnlock()
		return nil // Silently ignore logs after close
	}
	l.closeMu.RUnlock()

	// Generate ID if not set
	if event.ID == "" {
		event.ID = generateID()
	}

	// Set timestamp if not set
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	if l.config.Async && l.eventCh != nil {
		select {
		case l.eventCh <- event:
			return nil
		default:
			// Buffer full, log synchronously
			l.storeEvent(event)
		}
	} else {
		l.storeEvent(event)
	}

	return nil
}

// Query queries audit logs
func (l *InMemoryAuditLogger) Query(ctx context.Context, filter *contracts.AuditFilter) ([]*contracts.AuditEvent, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var result []*contracts.AuditEvent

	for _, event := range l.events {
		if l.matchesFilter(event, filter) {
			result = append(result, event)
		}
	}

	// Sort by timestamp descending (newest first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})

	// Apply offset and limit
	if filter != nil && filter.Offset > 0 {
		if filter.Offset >= len(result) {
			return []*contracts.AuditEvent{}, nil
		}
		result = result[filter.Offset:]
	}

	if filter != nil && filter.Limit > 0 && filter.Limit < len(result) {
		result = result[:filter.Limit]
	}

	return result, nil
}

// AddHandler adds an event handler
func (l *InMemoryAuditLogger) AddHandler(handler AuditHandler) {
	l.handlerMu.Lock()
	defer l.handlerMu.Unlock()
	l.handlers = append(l.handlers, handler)
}

// GetEventCount returns the total number of events
func (l *InMemoryAuditLogger) GetEventCount() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.events)
}

// GetEvents returns all events (for testing/debugging)
func (l *InMemoryAuditLogger) GetEvents() []*contracts.AuditEvent {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make([]*contracts.AuditEvent, len(l.events))
	copy(result, l.events)
	return result
}

// Clear removes all events
func (l *InMemoryAuditLogger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = make([]*contracts.AuditEvent, 0, l.config.MaxEvents)
}

// Close stops the audit logger gracefully
func (l *InMemoryAuditLogger) Close() error {
	l.stopOnce.Do(func() {
		// Mark as closed
		l.closeMu.Lock()
		l.closed = true
		l.closeMu.Unlock()

		// Signal goroutines to stop
		close(l.stopCh)

		// Close event channel if async
		if l.eventCh != nil {
			close(l.eventCh)
		}
	})

	// Wait for goroutines to finish
	l.wg.Wait()

	return nil
}

// storeEvent stores an event
func (l *InMemoryAuditLogger) storeEvent(event *contracts.AuditEvent) {
	l.mu.Lock()

	// Trim if at capacity
	if len(l.events) >= l.config.MaxEvents {
		// Remove oldest events (first 10%)
		removeCount := l.config.MaxEvents / 10
		if removeCount < 1 {
			removeCount = 1
		}
		l.events = l.events[removeCount:]
	}

	l.events = append(l.events, event)
	l.mu.Unlock()

	// Call handlers outside of lock to prevent deadlock
	l.handlerMu.RLock()
	handlers := make([]AuditHandler, len(l.handlers))
	copy(handlers, l.handlers)
	l.handlerMu.RUnlock()

	for _, handler := range handlers {
		// Recover from handler panics
		func() {
			defer func() {
				_ = recover() // Intentionally ignore panic value
			}()
			handler(event)
		}()
	}
}

// processEvents processes events from the channel
func (l *InMemoryAuditLogger) processEvents() {
	defer l.wg.Done()

	for event := range l.eventCh {
		l.storeEvent(event)
	}
}

// cleanupLoop periodically removes old events
func (l *InMemoryAuditLogger) cleanupLoop() {
	defer l.wg.Done()

	ticker := time.NewTicker(l.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			l.cleanup()
		case <-l.stopCh:
			return
		}
	}
}

// cleanup removes events older than retention period
func (l *InMemoryAuditLogger) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()

	threshold := time.Now().Add(-l.config.RetentionPeriod)

	// Filter out old events
	newEvents := make([]*contracts.AuditEvent, 0, len(l.events))
	for _, event := range l.events {
		if event.Timestamp.After(threshold) {
			newEvents = append(newEvents, event)
		}
	}

	l.events = newEvents
}

// matchesFilter checks if an event matches the filter
func (l *InMemoryAuditLogger) matchesFilter(event *contracts.AuditEvent, filter *contracts.AuditFilter) bool {
	if filter == nil {
		return true
	}

	if filter.ActorID != "" && event.ActorID != filter.ActorID {
		return false
	}

	if filter.Action != "" && event.Action != filter.Action {
		return false
	}

	if filter.Resource != "" && event.Resource != filter.Resource {
		return false
	}

	if filter.ResourceID != "" && event.ResourceID != filter.ResourceID {
		return false
	}

	if filter.Success != nil && event.Success != *filter.Success {
		return false
	}

	if !filter.StartTime.IsZero() && event.Timestamp.Before(filter.StartTime) {
		return false
	}

	if !filter.EndTime.IsZero() && event.Timestamp.After(filter.EndTime) {
		return false
	}

	return true
}

// generateID generates a unique ID for audit events
func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b) // Error ignored - crypto/rand.Read rarely fails
	return hex.EncodeToString(b)
}

// ============ Audit Event Builder ============

// AuditEventBuilder provides a fluent API for creating audit events
type AuditEventBuilder struct {
	event *contracts.AuditEvent
}

// NewAuditEvent creates a new audit event builder
func NewAuditEvent() *AuditEventBuilder {
	return &AuditEventBuilder{
		event: &contracts.AuditEvent{
			ID:        generateID(),
			Timestamp: time.Now(),
			Metadata:  make(map[string]any),
		},
	}
}

// Action sets the action
func (b *AuditEventBuilder) Action(action string) *AuditEventBuilder {
	b.event.Action = action
	return b
}

// Resource sets the resource
func (b *AuditEventBuilder) Resource(resource string) *AuditEventBuilder {
	b.event.Resource = resource
	return b
}

// ResourceID sets the resource ID
func (b *AuditEventBuilder) ResourceID(id string) *AuditEventBuilder {
	b.event.ResourceID = id
	return b
}

// Actor sets the actor information
func (b *AuditEventBuilder) Actor(id, actorType, name string) *AuditEventBuilder {
	b.event.ActorID = id
	b.event.ActorType = actorType
	b.event.ActorName = name
	return b
}

// ActorIP sets the actor IP
func (b *AuditEventBuilder) ActorIP(ip string) *AuditEventBuilder {
	b.event.ActorIP = ip
	return b
}

// Request sets request information
func (b *AuditEventBuilder) Request(method, path, userAgent string) *AuditEventBuilder {
	b.event.Method = method
	b.event.Path = path
	b.event.UserAgent = userAgent
	return b
}

// Success sets the success status
func (b *AuditEventBuilder) Success(success bool) *AuditEventBuilder {
	b.event.Success = success
	return b
}

// Error sets the error message
func (b *AuditEventBuilder) Error(err string) *AuditEventBuilder {
	b.event.Error = err
	b.event.Success = false
	return b
}

// OldValue sets the old value
func (b *AuditEventBuilder) OldValue(value any) *AuditEventBuilder {
	b.event.OldValue = value
	return b
}

// NewValue sets the new value
func (b *AuditEventBuilder) NewValue(value any) *AuditEventBuilder {
	b.event.NewValue = value
	return b
}

// Metadata adds metadata
func (b *AuditEventBuilder) Metadata(key string, value any) *AuditEventBuilder {
	b.event.Metadata[key] = value
	return b
}

// Build returns the audit event
func (b *AuditEventBuilder) Build() *contracts.AuditEvent {
	return b.event
}

// Log logs the event to the given logger
func (b *AuditEventBuilder) Log(ctx context.Context, logger contracts.AuditLogger) error {
	return logger.Log(ctx, b.event)
}

// ============ Predefined Actions ============

const (
	ActionCreate = "create"
	ActionRead   = "read"
	ActionUpdate = "update"
	ActionDelete = "delete"
	ActionLogin  = "login"
	ActionLogout = "logout"
	ActionExport = "export"
	ActionImport = "import"
	ActionAccess = "access"
	ActionDeny   = "deny"
)

// ============ Composite Logger ============

// CompositeAuditLogger logs to multiple audit loggers
type CompositeAuditLogger struct {
	loggers []contracts.AuditLogger
}

// NewCompositeAuditLogger creates a composite audit logger
func NewCompositeAuditLogger(loggers ...contracts.AuditLogger) *CompositeAuditLogger {
	return &CompositeAuditLogger{
		loggers: loggers,
	}
}

// Log logs to all loggers
func (c *CompositeAuditLogger) Log(ctx context.Context, event *contracts.AuditEvent) error {
	var lastErr error
	for _, logger := range c.loggers {
		if err := logger.Log(ctx, event); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// Query queries the first logger (primary)
func (c *CompositeAuditLogger) Query(ctx context.Context, filter *contracts.AuditFilter) ([]*contracts.AuditEvent, error) {
	if len(c.loggers) == 0 {
		return nil, nil
	}
	return c.loggers[0].Query(ctx, filter)
}

// Ensure implementations satisfy AuditLogger interface
var _ contracts.AuditLogger = (*InMemoryAuditLogger)(nil)
var _ contracts.AuditLogger = (*CompositeAuditLogger)(nil)
