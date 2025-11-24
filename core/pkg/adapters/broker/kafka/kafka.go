package kafka

import (
	"context"
	"fmt"
	"sync"

	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

// Broker adalah Kafka implementation dari contracts.Broker
// Ini adalah interface - implementasi sebenarnya akan menggunakan
// kafka-go, sarama, atau confluent-kafka-go
type Broker struct {
	config    *contracts.KafkaBrokerConfig
	mu        sync.RWMutex
	connected bool

	// These would be actual Kafka client instances
	// Using interface to allow different implementations
	producer Producer
	consumer Consumer
}

// Producer interface for Kafka producer
type Producer interface {
	Produce(ctx context.Context, topic string, msg *contracts.BrokerMessage) error
	ProduceBatch(ctx context.Context, topic string, msgs []*contracts.BrokerMessage) error
	Close() error
}

// Consumer interface for Kafka consumer
type Consumer interface {
	Subscribe(topics []string) error
	Poll(ctx context.Context) (*contracts.BrokerMessage, error)
	Commit(msg *contracts.BrokerMessage) error
	Close() error
}

// New creates a new Kafka broker
func New(config *contracts.KafkaBrokerConfig) *Broker {
	if config == nil {
		config = &contracts.KafkaBrokerConfig{
			BrokerConfig: contracts.BrokerConfig{
				Brokers:       []string{"localhost:9092"},
				ConsumerGroup: "unicorn-consumer",
				AutoAck:       true,
			},
			OffsetReset: "earliest",
		}
	}

	return &Broker{
		config: config,
	}
}

// SetProducer sets the producer implementation
func (b *Broker) SetProducer(p Producer) {
	b.producer = p
}

// SetConsumer sets the consumer implementation
func (b *Broker) SetConsumer(c Consumer) {
	b.consumer = c
}

// Name returns broker name
func (b *Broker) Name() string {
	return "kafka"
}

// Connect connects to Kafka
func (b *Broker) Connect(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// In real implementation:
	// 1. Create producer client
	// 2. Create consumer client
	// 3. Verify connectivity

	b.connected = true
	return nil
}

// Disconnect disconnects from Kafka
func (b *Broker) Disconnect(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.producer != nil {
		_ = b.producer.Close() // Best-effort close during disconnect
	}
	if b.consumer != nil {
		_ = b.consumer.Close() // Best-effort close during disconnect
	}

	b.connected = false
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

// Publish publishes a message to Kafka
func (b *Broker) Publish(ctx context.Context, topic string, msg *contracts.BrokerMessage) error {
	if b.producer == nil {
		return fmt.Errorf("producer not initialized")
	}
	return b.producer.Produce(ctx, topic, msg)
}

// PublishBatch publishes multiple messages
func (b *Broker) PublishBatch(ctx context.Context, topic string, msgs []*contracts.BrokerMessage) error {
	if b.producer == nil {
		return fmt.Errorf("producer not initialized")
	}
	return b.producer.ProduceBatch(ctx, topic, msgs)
}

// Subscribe subscribes to a topic (simple subscription without group)
func (b *Broker) Subscribe(ctx context.Context, topic string, handler contracts.MessageHandlerFunc) error {
	return b.SubscribeMultiple(ctx, []string{topic}, handler)
}

// SubscribeMultiple subscribes to multiple topics
func (b *Broker) SubscribeMultiple(ctx context.Context, topics []string, handler contracts.MessageHandlerFunc) error {
	if b.consumer == nil {
		return fmt.Errorf("consumer not initialized")
	}

	if err := b.consumer.Subscribe(topics); err != nil {
		return err
	}

	// Start consuming in background
	go b.consumeLoop(ctx, handler)
	return nil
}

// Unsubscribe unsubscribes from a topic
func (b *Broker) Unsubscribe(topic string) error {
	// Kafka doesn't support per-topic unsubscribe easily
	// Would need to resubscribe with updated topic list
	return nil
}

// ConsumeGroup starts consuming as part of a consumer group
func (b *Broker) ConsumeGroup(ctx context.Context, group string, topics []string, handler contracts.MessageHandlerFunc) error {
	if b.consumer == nil {
		return fmt.Errorf("consumer not initialized")
	}

	// In real implementation, would create consumer with group.id = group
	if err := b.consumer.Subscribe(topics); err != nil {
		return err
	}

	go b.consumeLoop(ctx, handler)
	return nil
}

// LeaveGroup leaves consumer group
func (b *Broker) LeaveGroup(group string) error {
	if b.consumer != nil {
		return b.consumer.Close()
	}
	return nil
}

// consumeLoop is the main consume loop
func (b *Broker) consumeLoop(ctx context.Context, handler contracts.MessageHandlerFunc) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, err := b.consumer.Poll(ctx)
			if err != nil {
				continue
			}
			if msg == nil {
				continue
			}

			// Process message
			if err := handler(ctx, msg); err != nil {
				// Error handling done by adapter layer
				continue
			}

			// Commit if auto-ack enabled
			if b.config.AutoAck {
				_ = b.consumer.Commit(msg) // Best-effort commit in auto-ack mode
			}
		}
	}
}

// QueueLength returns number of messages (not typically used in Kafka)
func (b *Broker) QueueLength(ctx context.Context, queue string) (int64, error) {
	// Kafka doesn't have queue length in traditional sense
	// Would need to calculate lag
	return 0, nil
}

// Ack commits the message offset
func (b *Broker) Ack(ctx context.Context, msg *contracts.BrokerMessage) error {
	if b.consumer == nil {
		return fmt.Errorf("consumer not initialized")
	}
	return b.consumer.Commit(msg)
}

// Nack negative acknowledges (Kafka doesn't support NACK, just don't commit)
func (b *Broker) Nack(ctx context.Context, msg *contracts.BrokerMessage, requeue bool) error {
	// Kafka doesn't have NACK
	// If requeue, republish to topic
	if requeue {
		return b.Publish(ctx, msg.Topic, msg)
	}
	return nil
}

// Config returns the broker config
func (b *Broker) Config() *contracts.KafkaBrokerConfig {
	return b.config
}
