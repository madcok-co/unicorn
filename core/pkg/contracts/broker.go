package contracts

import (
	"context"
	"time"
)

// Broker adalah generic interface untuk message broker
// Implementasi bisa Kafka, RabbitMQ, Redis Pub/Sub, NATS, Google Pub/Sub, AWS SQS, dll
type Broker interface {
	// Publishing
	Publish(ctx context.Context, topic string, msg *BrokerMessage) error
	PublishBatch(ctx context.Context, topic string, msgs []*BrokerMessage) error

	// Subscribing - untuk simple pub/sub
	Subscribe(ctx context.Context, topic string, handler MessageHandlerFunc) error
	SubscribeMultiple(ctx context.Context, topics []string, handler MessageHandlerFunc) error
	Unsubscribe(topic string) error

	// Consumer Group - untuk load balancing across instances
	// Kafka: consumer group, RabbitMQ: queue with multiple consumers, etc
	ConsumeGroup(ctx context.Context, group string, topics []string, handler MessageHandlerFunc) error
	LeaveGroup(group string) error

	// Queue operations (for queue-based brokers like RabbitMQ, SQS)
	// Returns number of messages in queue
	QueueLength(ctx context.Context, queue string) (int64, error)

	// Acknowledge message (for brokers that require explicit ack)
	Ack(ctx context.Context, msg *BrokerMessage) error
	Nack(ctx context.Context, msg *BrokerMessage, requeue bool) error

	// Connection management
	Connect(ctx context.Context) error
	Disconnect(ctx context.Context) error
	Ping(ctx context.Context) error
	IsConnected() bool

	// Info
	Name() string // "kafka", "rabbitmq", "redis", "nats", etc
}

// BrokerMessage represents a generic message across all brokers
type BrokerMessage struct {
	// Identifiers
	ID    string // Message ID
	Topic string // Topic/Queue/Channel name

	// Content
	Key     []byte            // Partition key (Kafka), routing key (RabbitMQ)
	Body    []byte            // Message body
	Headers map[string]string // Message headers/attributes

	// Metadata (set by broker on receive)
	Partition int       // Kafka partition
	Offset    int64     // Kafka offset
	Timestamp time.Time // Message timestamp

	// Delivery info
	DeliveryTag   uint64 // RabbitMQ delivery tag
	ConsumerGroup string // Consumer group that received this
	Redelivered   bool   // Whether this is a redelivery

	// For retry/DLQ handling
	RetryCount int
	MaxRetries int
	Error      string // Last error if any

	// Raw message from underlying broker (for advanced use)
	Raw any
}

// MessageHandlerFunc adalah function untuk handle incoming messages
type MessageHandlerFunc func(ctx context.Context, msg *BrokerMessage) error

// BrokerConfig adalah base config untuk semua brokers
type BrokerConfig struct {
	// Common settings
	Name    string   // Broker name for logging
	Brokers []string // Broker addresses

	// Authentication
	Username string
	Password string
	UseTLS   bool
	CertFile string
	KeyFile  string
	CAFile   string

	// Consumer settings
	ConsumerGroup     string
	AutoAck           bool // Auto acknowledge messages
	PrefetchCount     int  // Number of messages to prefetch
	MaxRetries        int  // Max retries before DLQ
	RetryBackoff      time.Duration
	ProcessingTimeout time.Duration

	// Producer settings
	BatchSize       int
	BatchTimeout    time.Duration
	Compression     string // none, gzip, snappy, lz4, zstd
	RequiredAcks    string // none, leader, all
	MaxMessageBytes int

	// Dead Letter Queue
	DLQEnabled bool
	DLQTopic   string // Topic/queue for failed messages

	// Reconnection
	ReconnectBackoff    time.Duration
	MaxReconnectBackoff time.Duration

	// Extra options (broker-specific)
	Options map[string]any
}

// ============ Broker-specific configs ============

// KafkaBrokerConfig extends BrokerConfig for Kafka
type KafkaBrokerConfig struct {
	BrokerConfig

	// Kafka-specific
	SessionTimeout    time.Duration
	HeartbeatInterval time.Duration
	RebalanceStrategy string // range, roundrobin, sticky
	OffsetReset       string // earliest, latest
	IsolationLevel    string // read_uncommitted, read_committed
}

// RabbitMQBrokerConfig extends BrokerConfig for RabbitMQ
type RabbitMQBrokerConfig struct {
	BrokerConfig

	// RabbitMQ-specific
	VHost           string
	Exchange        string
	ExchangeType    string // direct, fanout, topic, headers
	RoutingKey      string
	QueueDurable    bool
	QueueAutoDelete bool
	ExclusiveQueue  bool
}

// RedisBrokerConfig extends BrokerConfig for Redis Pub/Sub
type RedisBrokerConfig struct {
	BrokerConfig

	// Redis-specific
	Database     int
	PoolSize     int
	MinIdleConns int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	// Stream settings (for Redis Streams)
	UseStreams   bool
	StreamMaxLen int64
	StreamApprox bool
}

// NATSBrokerConfig extends BrokerConfig for NATS
type NATSBrokerConfig struct {
	BrokerConfig

	// NATS-specific
	ClusterName   string
	ClientName    string
	MaxReconnects int
	DurableName   string // For JetStream durable consumers
	DeliverPolicy string // all, last, new, by_start_sequence, by_start_time

	// JetStream settings
	UseJetStream   bool
	StreamName     string
	StreamSubjects []string
}

// ============ Publisher/Consumer interfaces ============

// Publisher is a simplified interface for only publishing
type Publisher interface {
	Publish(ctx context.Context, topic string, msg *BrokerMessage) error
	PublishBatch(ctx context.Context, topic string, msgs []*BrokerMessage) error
}

// Consumer is a simplified interface for only consuming
type Consumer interface {
	Subscribe(ctx context.Context, topic string, handler MessageHandlerFunc) error
	ConsumeGroup(ctx context.Context, group string, topics []string, handler MessageHandlerFunc) error
	Unsubscribe(topic string) error
}

// ============ Helper functions ============

// NewBrokerMessage creates a new message with defaults
func NewBrokerMessage(topic string, body []byte) *BrokerMessage {
	return &BrokerMessage{
		Topic:     topic,
		Body:      body,
		Headers:   make(map[string]string),
		Timestamp: time.Now(),
	}
}

// NewBrokerMessageWithKey creates a new message with key
func NewBrokerMessageWithKey(topic string, key, body []byte) *BrokerMessage {
	msg := NewBrokerMessage(topic, body)
	msg.Key = key
	return msg
}

// SetHeader is a fluent method to set header
func (m *BrokerMessage) SetHeader(key, value string) *BrokerMessage {
	if m.Headers == nil {
		m.Headers = make(map[string]string)
	}
	m.Headers[key] = value
	return m
}

// GetHeader gets a header value
func (m *BrokerMessage) GetHeader(key string) string {
	if m.Headers == nil {
		return ""
	}
	return m.Headers[key]
}

// WithRetry sets retry configuration
func (m *BrokerMessage) WithRetry(maxRetries int) *BrokerMessage {
	m.MaxRetries = maxRetries
	return m
}
