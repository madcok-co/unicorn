package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	ucontext "github.com/madcok-co/unicorn/core/pkg/context"
	"github.com/madcok-co/unicorn/core/pkg/contracts"
	"github.com/madcok-co/unicorn/core/pkg/handler"
)

// Adapter adalah generic message broker adapter
// Wraps any Broker implementation dan connects to handler registry
type Adapter struct {
	broker      contracts.Broker
	registry    *handler.Registry
	config      *Config
	appAdapters *ucontext.AppAdapters

	mu       sync.Mutex
	running  bool
	stopCh   chan struct{}
	handlers map[string]*handler.Handler // topic -> handler mapping
}

// Config untuk broker adapter
type Config struct {
	// Consumer group name
	GroupID string

	// Auto acknowledge messages
	AutoAck bool

	// Retry configuration
	MaxRetries   int
	RetryBackoff time.Duration

	// Dead Letter Queue
	DLQEnabled bool
	DLQSuffix  string // e.g., ".dlq" -> topic "orders" becomes "orders.dlq"
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	return &Config{
		GroupID:      "unicorn-consumer",
		AutoAck:      true,
		MaxRetries:   3,
		RetryBackoff: time.Second,
		DLQEnabled:   true,
		DLQSuffix:    ".dlq",
	}
}

// New creates a new broker adapter
func New(broker contracts.Broker, registry *handler.Registry, config *Config) *Adapter {
	if config == nil {
		config = DefaultConfig()
	}

	return &Adapter{
		broker:   broker,
		registry: registry,
		config:   config,
		stopCh:   make(chan struct{}),
		handlers: make(map[string]*handler.Handler),
	}
}

// SetAppAdapters sets the app-level adapters
func (a *Adapter) SetAppAdapters(adapters *ucontext.AppAdapters) {
	a.appAdapters = adapters
}

// Start starts consuming messages
func (a *Adapter) Start(ctx context.Context) error {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return fmt.Errorf("broker adapter already running")
	}
	a.running = true
	// Reset stopCh if it was closed before (for restart capability)
	select {
	case <-a.stopCh:
		a.stopCh = make(chan struct{})
	default:
	}
	a.mu.Unlock()

	// Connect to broker
	if err := a.broker.Connect(ctx); err != nil {
		a.mu.Lock()
		a.running = false
		a.mu.Unlock()
		return fmt.Errorf("failed to connect to broker: %w", err)
	}

	// Collect topics from registry
	topics := a.collectTopics()
	if len(topics) == 0 {
		a.mu.Lock()
		a.running = false
		a.mu.Unlock()
		return fmt.Errorf("no message handlers registered")
	}

	// Subscribe to topics with consumer group
	err := a.broker.ConsumeGroup(ctx, a.config.GroupID, topics, a.handleMessage)
	if err != nil {
		a.mu.Lock()
		a.running = false
		a.mu.Unlock()
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	// Wait for context cancellation
	<-ctx.Done()
	return a.Stop(context.Background())
}

// Stop stops the adapter
func (a *Adapter) Stop(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.running {
		return nil
	}

	// Close stopCh to signal goroutines
	select {
	case <-a.stopCh:
		// Already closed
	default:
		close(a.stopCh)
	}

	a.running = false

	return a.broker.Disconnect(ctx)
}

// collectTopics collects all message topics from registry
func (a *Adapter) collectTopics() []string {
	topicSet := make(map[string]bool)

	for _, h := range a.registry.All() {
		for _, trigger := range h.GetMessageTriggers() {
			topicSet[trigger.Topic] = true
			a.handlers[trigger.Topic] = h
		}
	}

	topics := make([]string, 0, len(topicSet))
	for topic := range topicSet {
		topics = append(topics, topic)
	}
	return topics
}

// handleMessage processes incoming messages
func (a *Adapter) handleMessage(ctx context.Context, msg *contracts.BrokerMessage) error {
	h, ok := a.handlers[msg.Topic]
	if !ok {
		return fmt.Errorf("no handler for topic: %s", msg.Topic)
	}

	// Create unicorn context
	uctx := ucontext.New(ctx)

	// Set app adapters if available
	if a.appAdapters != nil {
		uctx.SetAppAdapters(a.appAdapters)
	}

	req := &ucontext.Request{
		Body:        msg.Body,
		Headers:     msg.Headers,
		Topic:       msg.Topic,
		Partition:   msg.Partition,
		Offset:      msg.Offset,
		Key:         msg.Key,
		TriggerType: "message",
	}
	uctx.SetRequest(req)

	// Execute handler
	executor := handler.NewExecutor(h)
	err := executor.Execute(uctx)

	if err != nil {
		return a.handleError(ctx, msg, h, err)
	}

	// Acknowledge if not auto-ack
	if !a.config.AutoAck {
		return a.broker.Ack(ctx, msg)
	}

	return nil
}

// handleError handles message processing errors
func (a *Adapter) handleError(ctx context.Context, msg *contracts.BrokerMessage, h *handler.Handler, err error) error {
	// Get trigger config for retry settings
	var maxRetries int
	var dlqTopic string

	triggers := h.GetMessageTriggers()
	for _, t := range triggers {
		if t.Topic == msg.Topic {
			maxRetries = t.MaxRetries
			dlqTopic = t.DLQTopic
			break
		}
	}

	// Use adapter defaults if not set
	if maxRetries == 0 {
		maxRetries = a.config.MaxRetries
	}
	if dlqTopic == "" && a.config.DLQEnabled {
		dlqTopic = msg.Topic + a.config.DLQSuffix
	}

	// Check retry count
	if msg.RetryCount >= maxRetries {
		// Send to DLQ
		if dlqTopic != "" {
			return a.sendToDLQ(ctx, msg, dlqTopic, err)
		}
		// No DLQ, just nack without requeue
		return a.broker.Nack(ctx, msg, false)
	}

	// Retry: increment counter and republish
	msg.RetryCount++
	msg.SetHeader("x-retry-count", fmt.Sprintf("%d", msg.RetryCount))
	msg.SetHeader("x-last-error", err.Error())

	// Wait before retry with context cancellation support
	backoffDuration := a.config.RetryBackoff * time.Duration(msg.RetryCount)
	if err := a.sleepWithContext(ctx, backoffDuration); err != nil {
		// Context cancelled, don't retry
		return err
	}

	// Republish to same topic
	return a.broker.Publish(ctx, msg.Topic, msg)
}

// sleepWithContext sleeps for duration but can be cancelled by context
func (a *Adapter) sleepWithContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-a.stopCh:
		return fmt.Errorf("adapter stopped")
	case <-timer.C:
		return nil
	}
}

// sendToDLQ sends failed message to Dead Letter Queue
func (a *Adapter) sendToDLQ(ctx context.Context, msg *contracts.BrokerMessage, dlqTopic string, err error) error {
	dlqMsg := &contracts.BrokerMessage{
		Key:     msg.Key,
		Body:    msg.Body,
		Headers: make(map[string]string),
	}

	// Copy original headers
	for k, v := range msg.Headers {
		dlqMsg.Headers[k] = v
	}

	// Add DLQ metadata
	dlqMsg.SetHeader("x-original-topic", msg.Topic)
	dlqMsg.SetHeader("x-error", err.Error())
	dlqMsg.SetHeader("x-failed-at", time.Now().Format(time.RFC3339))
	dlqMsg.SetHeader("x-retry-count", fmt.Sprintf("%d", msg.RetryCount))

	return a.broker.Publish(ctx, dlqTopic, dlqMsg)
}

// Publish publishes a message through the broker
func (a *Adapter) Publish(ctx context.Context, topic string, data any) error {
	var body []byte
	var err error

	switch v := data.(type) {
	case []byte:
		body = v
	case string:
		body = []byte(v)
	default:
		body, err = json.Marshal(data)
		if err != nil {
			return fmt.Errorf("failed to marshal data: %w", err)
		}
	}

	msg := contracts.NewBrokerMessage(topic, body)
	return a.broker.Publish(ctx, topic, msg)
}

// PublishWithKey publishes a message with key
func (a *Adapter) PublishWithKey(ctx context.Context, topic string, key string, data any) error {
	var body []byte
	var err error

	switch v := data.(type) {
	case []byte:
		body = v
	case string:
		body = []byte(v)
	default:
		body, err = json.Marshal(data)
		if err != nil {
			return fmt.Errorf("failed to marshal data: %w", err)
		}
	}

	msg := contracts.NewBrokerMessageWithKey(topic, []byte(key), body)
	return a.broker.Publish(ctx, topic, msg)
}

// Broker returns the underlying broker
func (a *Adapter) Broker() contracts.Broker {
	return a.broker
}

// IsRunning returns whether adapter is running
func (a *Adapter) IsRunning() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.running
}
