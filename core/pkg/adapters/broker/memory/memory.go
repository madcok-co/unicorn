package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

// Broker adalah in-memory message broker untuk testing dan development
type Broker struct {
	mu          sync.RWMutex
	connected   bool
	subscribers map[string][]subscription
	groups      map[string]map[string][]subscription // group -> topic -> subscriptions
	queues      map[string][]*contracts.BrokerMessage
	wg          sync.WaitGroup // Track goroutines for clean shutdown
}

type subscription struct {
	handler contracts.MessageHandlerFunc
	cancel  context.CancelFunc
}

// New creates a new in-memory broker
func New() *Broker {
	return &Broker{
		subscribers: make(map[string][]subscription),
		groups:      make(map[string]map[string][]subscription),
		queues:      make(map[string][]*contracts.BrokerMessage),
	}
}

// Name returns broker name
func (b *Broker) Name() string {
	return "memory"
}

// Connect connects to broker (no-op for memory)
func (b *Broker) Connect(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.connected = true
	return nil
}

// Disconnect disconnects from broker
func (b *Broker) Disconnect(ctx context.Context) error {
	b.mu.Lock()
	b.connected = false

	// Cancel all subscriptions
	for _, subs := range b.subscribers {
		for _, sub := range subs {
			if sub.cancel != nil {
				sub.cancel()
			}
		}
	}
	for _, groupTopics := range b.groups {
		for _, subs := range groupTopics {
			for _, sub := range subs {
				if sub.cancel != nil {
					sub.cancel()
				}
			}
		}
	}

	b.subscribers = make(map[string][]subscription)
	b.groups = make(map[string]map[string][]subscription)
	b.mu.Unlock()

	// Wait for all goroutines to finish
	b.wg.Wait()

	return nil
}

// Ping checks connection
func (b *Broker) Ping(ctx context.Context) error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if !b.connected {
		return fmt.Errorf("not connected")
	}
	return nil
}

// IsConnected returns connection status
func (b *Broker) IsConnected() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.connected
}

// Publish publishes a message
func (b *Broker) Publish(ctx context.Context, topic string, msg *contracts.BrokerMessage) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.connected {
		return fmt.Errorf("not connected")
	}

	msg.Topic = topic
	msg.Timestamp = time.Now()

	// Deliver to direct subscribers
	if subs, ok := b.subscribers[topic]; ok {
		for _, sub := range subs {
			// Clone message for each subscriber
			clone := *msg
			handler := sub.handler

			b.wg.Add(1)
			go func(h contracts.MessageHandlerFunc, m *contracts.BrokerMessage) {
				defer b.wg.Done()
				// Use background context to ensure delivery completes
				// even if original context is cancelled
				// Error is intentionally ignored - in-memory broker for dev/testing
				_ = h(context.Background(), m)
			}(handler, &clone)
		}
	}

	// Deliver to consumer groups (round-robin)
	for _, groupTopics := range b.groups {
		if subs, ok := groupTopics[topic]; ok && len(subs) > 0 {
			// Simple round-robin: just pick first subscriber
			// In real implementation, would track offset per group
			clone := *msg
			handler := subs[0].handler

			b.wg.Add(1)
			go func(h contracts.MessageHandlerFunc, m *contracts.BrokerMessage) {
				defer b.wg.Done()
				// Error is intentionally ignored - in-memory broker for dev/testing
				_ = h(context.Background(), m)
			}(handler, &clone)
		}
	}

	return nil
}

// PublishBatch publishes multiple messages
func (b *Broker) PublishBatch(ctx context.Context, topic string, msgs []*contracts.BrokerMessage) error {
	for _, msg := range msgs {
		if err := b.Publish(ctx, topic, msg); err != nil {
			return err
		}
	}
	return nil
}

// Subscribe subscribes to a topic
func (b *Broker) Subscribe(ctx context.Context, topic string, handler contracts.MessageHandlerFunc) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.connected {
		return fmt.Errorf("not connected")
	}

	subCtx, cancel := context.WithCancel(ctx)
	sub := subscription{
		handler: handler,
		cancel:  cancel,
	}

	b.subscribers[topic] = append(b.subscribers[topic], sub)

	// Handle context cancellation
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		<-subCtx.Done()
		_ = b.Unsubscribe(topic) // Best-effort cleanup on context cancel
	}()

	return nil
}

// SubscribeMultiple subscribes to multiple topics
func (b *Broker) SubscribeMultiple(ctx context.Context, topics []string, handler contracts.MessageHandlerFunc) error {
	for _, topic := range topics {
		if err := b.Subscribe(ctx, topic, handler); err != nil {
			return err
		}
	}
	return nil
}

// Unsubscribe unsubscribes from a topic
func (b *Broker) Unsubscribe(topic string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if subs, ok := b.subscribers[topic]; ok {
		for _, sub := range subs {
			if sub.cancel != nil {
				sub.cancel()
			}
		}
	}
	delete(b.subscribers, topic)
	return nil
}

// ConsumeGroup starts consuming as part of a consumer group
func (b *Broker) ConsumeGroup(ctx context.Context, group string, topics []string, handler contracts.MessageHandlerFunc) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.connected {
		return fmt.Errorf("not connected")
	}

	if _, ok := b.groups[group]; !ok {
		b.groups[group] = make(map[string][]subscription)
	}

	subCtx, cancel := context.WithCancel(ctx)
	sub := subscription{
		handler: handler,
		cancel:  cancel,
	}

	for _, topic := range topics {
		b.groups[group][topic] = append(b.groups[group][topic], sub)
	}

	// Handle context cancellation
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		<-subCtx.Done()
		_ = b.LeaveGroup(group) // Best-effort cleanup on context cancel
	}()

	return nil
}

// LeaveGroup leaves a consumer group
func (b *Broker) LeaveGroup(group string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if groupTopics, ok := b.groups[group]; ok {
		for _, subs := range groupTopics {
			for _, sub := range subs {
				if sub.cancel != nil {
					sub.cancel()
				}
			}
		}
	}
	delete(b.groups, group)
	return nil
}

// QueueLength returns queue length (always 0 for pub/sub)
func (b *Broker) QueueLength(ctx context.Context, queue string) (int64, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return int64(len(b.queues[queue])), nil
}

// Ack acknowledges a message (no-op for memory)
func (b *Broker) Ack(ctx context.Context, msg *contracts.BrokerMessage) error {
	return nil
}

// Nack negative acknowledges a message
func (b *Broker) Nack(ctx context.Context, msg *contracts.BrokerMessage, requeue bool) error {
	if requeue {
		return b.Publish(ctx, msg.Topic, msg)
	}
	return nil
}
