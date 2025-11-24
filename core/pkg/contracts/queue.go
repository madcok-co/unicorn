package contracts

import (
	"context"
	"time"
)

// Queue adalah generic interface untuk message queue
// Implementasi bisa Kafka, RabbitMQ, Redis Pub/Sub, NATS, dll
type Queue interface {
	// Publishing
	Publish(ctx context.Context, topic string, message *Message) error
	PublishBatch(ctx context.Context, topic string, messages []*Message) error

	// Subscribing
	Subscribe(ctx context.Context, topics []string, handler MessageHandler) error
	Unsubscribe(topics ...string) error

	// Consumer group (untuk Kafka-like semantics)
	JoinGroup(ctx context.Context, groupID string, topics []string, handler MessageHandler) error
	LeaveGroup() error

	// Connection
	Ping(ctx context.Context) error
	Close() error
}

// Message represents a queue message
type Message struct {
	// Identifiers
	ID        string
	Key       []byte // untuk partitioning di Kafka
	Topic     string
	Partition int

	// Content
	Body    []byte
	Headers map[string]string

	// Metadata
	Timestamp time.Time
	Offset    int64

	// Retry info
	RetryCount int
	MaxRetries int
}

// MessageHandler adalah function untuk handle incoming messages
type MessageHandler func(ctx context.Context, msg *Message) error

// QueueConfig untuk konfigurasi queue
type QueueConfig struct {
	Driver  string   // kafka, rabbitmq, redis, nats
	Brokers []string // list of broker addresses

	// Kafka specific
	GroupID           string
	AutoOffsetReset   string // earliest, latest
	EnableAutoCommit  bool
	SessionTimeout    time.Duration
	HeartbeatInterval time.Duration

	// RabbitMQ specific
	VHost    string
	Exchange string
	Queue    string

	// Security
	Username string
	Password string
	UseTLS   bool
	CertFile string
	KeyFile  string
	CAFile   string

	// Producer settings
	BatchSize    int
	LingerMs     int
	Compression  string // none, gzip, snappy, lz4
	Acks         string // 0, 1, all
	Retries      int
	RetryBackoff time.Duration

	// Consumer settings
	FetchMinBytes  int
	FetchMaxBytes  int
	MaxPollRecords int
	PollTimeout    time.Duration
	CommitInterval time.Duration

	// Additional options
	Options map[string]string
}

// DeadLetterConfig untuk dead letter queue
type DeadLetterConfig struct {
	Enabled    bool
	Topic      string
	MaxRetries int
}
