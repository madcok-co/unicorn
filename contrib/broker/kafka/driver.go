// Package kafka provides a Kafka implementation of the unicorn Broker interface.
//
// Usage:
//
//	import (
//	    "github.com/madcok-co/unicorn/contrib/broker/kafka"
//	)
//
//	driver := kafka.NewDriver(&kafka.Config{
//	    Brokers: []string{"localhost:9092"},
//	    GroupID: "my-service",
//	})
//	app.SetBroker(driver)
package kafka

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

// Driver implements contracts.Broker using Kafka (Sarama)
type Driver struct {
	config        *Config
	client        sarama.Client
	producer      sarama.SyncProducer
	consumerGroup sarama.ConsumerGroup
	consumers     map[string]context.CancelFunc
	mu            sync.RWMutex
	connected     bool
}

// Config for Kafka driver
type Config struct {
	Brokers  []string
	GroupID  string
	ClientID string
	Version  string // Kafka version, e.g., "2.8.0"

	// Producer settings
	RequiredAcks    sarama.RequiredAcks // NoResponse, WaitForLocal, WaitForAll
	Compression     sarama.CompressionCodec
	MaxMessageBytes int
	BatchSize       int
	BatchTimeout    time.Duration

	// Consumer settings
	OffsetInitial      int64 // OffsetNewest or OffsetOldest
	SessionTimeout     time.Duration
	HeartbeatInterval  time.Duration
	RebalanceStrategy  string // "range", "roundrobin", "sticky"
	AutoCommit         bool
	AutoCommitInterval time.Duration

	// TLS/SASL
	UseTLS        bool
	TLSConfig     *TLSConfig
	UseSASL       bool
	SASLMechanism string // "PLAIN", "SCRAM-SHA-256", "SCRAM-SHA-512"
	SASLUser      string
	SASLPassword  string
}

// TLSConfig for TLS connections
type TLSConfig struct {
	CertFile   string
	KeyFile    string
	CAFile     string
	SkipVerify bool
}

// DefaultConfig returns sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Brokers:            []string{"localhost:9092"},
		GroupID:            "unicorn-service",
		ClientID:           "unicorn-client",
		Version:            "2.8.0",
		RequiredAcks:       sarama.WaitForAll,
		Compression:        sarama.CompressionSnappy,
		MaxMessageBytes:    1024 * 1024, // 1MB
		BatchSize:          100,
		BatchTimeout:       10 * time.Millisecond,
		OffsetInitial:      sarama.OffsetNewest,
		SessionTimeout:     10 * time.Second,
		HeartbeatInterval:  3 * time.Second,
		RebalanceStrategy:  "roundrobin",
		AutoCommit:         true,
		AutoCommitInterval: 1 * time.Second,
	}
}

// NewDriver creates a new Kafka driver
func NewDriver(cfg *Config) *Driver {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &Driver{
		config:    cfg,
		consumers: make(map[string]context.CancelFunc),
	}
}

// buildSaramaConfig builds Sarama configuration from our config
func (d *Driver) buildSaramaConfig() (*sarama.Config, error) {
	cfg := sarama.NewConfig()

	// Parse version
	version, err := sarama.ParseKafkaVersion(d.config.Version)
	if err != nil {
		version = sarama.V2_8_0_0
	}
	cfg.Version = version

	// Client
	cfg.ClientID = d.config.ClientID

	// Producer
	cfg.Producer.RequiredAcks = d.config.RequiredAcks
	cfg.Producer.Compression = d.config.Compression
	cfg.Producer.MaxMessageBytes = d.config.MaxMessageBytes
	cfg.Producer.Flush.Messages = d.config.BatchSize
	cfg.Producer.Flush.Frequency = d.config.BatchTimeout
	cfg.Producer.Return.Successes = true
	cfg.Producer.Return.Errors = true

	// Consumer
	cfg.Consumer.Offsets.Initial = d.config.OffsetInitial
	cfg.Consumer.Group.Session.Timeout = d.config.SessionTimeout
	cfg.Consumer.Group.Heartbeat.Interval = d.config.HeartbeatInterval

	switch d.config.RebalanceStrategy {
	case "range":
		cfg.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRange()}
	case "sticky":
		cfg.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategySticky()}
	default:
		cfg.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}
	}

	cfg.Consumer.Offsets.AutoCommit.Enable = d.config.AutoCommit
	cfg.Consumer.Offsets.AutoCommit.Interval = d.config.AutoCommitInterval

	// SASL
	if d.config.UseSASL {
		cfg.Net.SASL.Enable = true
		cfg.Net.SASL.User = d.config.SASLUser
		cfg.Net.SASL.Password = d.config.SASLPassword
		cfg.Net.SASL.Mechanism = sarama.SASLMechanism(d.config.SASLMechanism)
	}

	return cfg, nil
}

// Connect establishes connection to Kafka
func (d *Driver) Connect(ctx context.Context) error {
	cfg, err := d.buildSaramaConfig()
	if err != nil {
		return err
	}

	// Create client
	client, err := sarama.NewClient(d.config.Brokers, cfg)
	if err != nil {
		return err
	}
	d.client = client

	// Create producer
	producer, err := sarama.NewSyncProducerFromClient(client)
	if err != nil {
		_ = client.Close() // Best-effort cleanup on error
		return err
	}
	d.producer = producer

	d.connected = true
	return nil
}

// Disconnect closes connections
func (d *Driver) Disconnect(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Cancel all consumers
	for _, cancel := range d.consumers {
		cancel()
	}
	d.consumers = make(map[string]context.CancelFunc)

	// Close consumer group
	if d.consumerGroup != nil {
		_ = d.consumerGroup.Close() // Best-effort close
	}

	// Close producer
	if d.producer != nil {
		_ = d.producer.Close() // Best-effort close
	}

	// Close client
	if d.client != nil {
		_ = d.client.Close() // Best-effort close
	}

	d.connected = false
	return nil
}

// Ping checks Kafka connectivity
func (d *Driver) Ping(ctx context.Context) error {
	if !d.connected || d.client == nil {
		return errors.New("kafka: not connected")
	}

	// Try to refresh metadata
	return d.client.RefreshMetadata()
}

// IsConnected returns connection status
func (d *Driver) IsConnected() bool {
	return d.connected
}

// Name returns broker name
func (d *Driver) Name() string {
	return "kafka"
}

// Publish publishes a message to a topic
func (d *Driver) Publish(ctx context.Context, topic string, msg *contracts.BrokerMessage) error {
	if !d.connected {
		return errors.New("kafka: not connected")
	}

	producerMsg := &sarama.ProducerMessage{
		Topic:     topic,
		Value:     sarama.ByteEncoder(msg.Body),
		Timestamp: time.Now(),
	}

	if len(msg.Key) > 0 {
		producerMsg.Key = sarama.ByteEncoder(msg.Key)
	}

	// Add headers
	for k, v := range msg.Headers {
		producerMsg.Headers = append(producerMsg.Headers, sarama.RecordHeader{
			Key:   []byte(k),
			Value: []byte(v),
		})
	}

	partition, offset, err := d.producer.SendMessage(producerMsg)
	if err != nil {
		return err
	}

	msg.Partition = int(partition)
	msg.Offset = offset
	return nil
}

// PublishBatch publishes multiple messages
func (d *Driver) PublishBatch(ctx context.Context, topic string, msgs []*contracts.BrokerMessage) error {
	for _, msg := range msgs {
		if err := d.Publish(ctx, topic, msg); err != nil {
			return err
		}
	}
	return nil
}

// Subscribe subscribes to a topic (simple pub/sub without consumer group)
func (d *Driver) Subscribe(ctx context.Context, topic string, handler contracts.MessageHandlerFunc) error {
	return d.ConsumeGroup(ctx, d.config.GroupID, []string{topic}, handler)
}

// SubscribeMultiple subscribes to multiple topics
func (d *Driver) SubscribeMultiple(ctx context.Context, topics []string, handler contracts.MessageHandlerFunc) error {
	return d.ConsumeGroup(ctx, d.config.GroupID, topics, handler)
}

// Unsubscribe stops consuming from a topic
func (d *Driver) Unsubscribe(topic string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if cancel, ok := d.consumers[topic]; ok {
		cancel()
		delete(d.consumers, topic)
	}
	return nil
}

// ConsumeGroup consumes messages using a consumer group
func (d *Driver) ConsumeGroup(ctx context.Context, group string, topics []string, handler contracts.MessageHandlerFunc) error {
	cfg, err := d.buildSaramaConfig()
	if err != nil {
		return err
	}

	consumerGroup, err := sarama.NewConsumerGroup(d.config.Brokers, group, cfg)
	if err != nil {
		return err
	}

	d.mu.Lock()
	d.consumerGroup = consumerGroup
	d.mu.Unlock()

	// Create consumer handler
	consumerHandler := &consumerGroupHandler{
		handler: handler,
		ready:   make(chan bool),
	}

	// Start consuming in goroutine
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				if err := consumerGroup.Consume(ctx, topics, consumerHandler); err != nil {
					// Log error, maybe reconnect
					time.Sleep(time.Second)
				}
			}
		}
	}()

	// Wait until consumer is ready
	<-consumerHandler.ready

	return nil
}

// LeaveGroup stops consuming from a group
func (d *Driver) LeaveGroup(group string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.consumerGroup != nil {
		return d.consumerGroup.Close()
	}
	return nil
}

// QueueLength returns approximate queue length (not directly supported in Kafka)
func (d *Driver) QueueLength(ctx context.Context, queue string) (int64, error) {
	// Kafka doesn't have traditional queue length
	// We could calculate lag by comparing current offset with high watermark
	return 0, errors.New("kafka: queue length not directly supported, use consumer lag metrics")
}

// Ack acknowledges a message (handled by auto-commit in Kafka)
func (d *Driver) Ack(ctx context.Context, msg *contracts.BrokerMessage) error {
	// In Kafka with auto-commit, this is handled automatically
	// For manual commit, we'd need to track the session
	return nil
}

// Nack negatively acknowledges a message
func (d *Driver) Nack(ctx context.Context, msg *contracts.BrokerMessage, requeue bool) error {
	// Kafka doesn't have native nack
	// For requeue, we could republish to the topic or a retry topic
	return nil
}

// consumerGroupHandler implements sarama.ConsumerGroupHandler
type consumerGroupHandler struct {
	handler contracts.MessageHandlerFunc
	ready   chan bool
}

func (h *consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	close(h.ready)
	return nil
}

func (h *consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for message := range claim.Messages() {
		// Convert to BrokerMessage
		msg := &contracts.BrokerMessage{
			Topic:     message.Topic,
			Key:       message.Key,
			Body:      message.Value,
			Partition: int(message.Partition),
			Offset:    message.Offset,
			Timestamp: message.Timestamp,
			Headers:   make(map[string]string),
			Raw:       message,
		}

		// Copy headers
		for _, h := range message.Headers {
			msg.Headers[string(h.Key)] = string(h.Value)
		}

		// Handle message
		if err := h.handler(session.Context(), msg); err != nil {
			// Could implement retry logic here
			continue
		}

		// Mark message as processed
		session.MarkMessage(message, "")
	}
	return nil
}

// Ensure Driver implements contracts.Broker
var _ contracts.Broker = (*Driver)(nil)
