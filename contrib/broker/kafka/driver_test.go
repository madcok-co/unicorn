package kafka

import (
	"context"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

// =============================================================================
// DefaultConfig
// =============================================================================

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	if len(cfg.Brokers) != 1 || cfg.Brokers[0] != "localhost:9092" {
		t.Errorf("Brokers = %v, want [localhost:9092]", cfg.Brokers)
	}
	if cfg.GroupID != "unicorn-service" {
		t.Errorf("GroupID = %q, want %q", cfg.GroupID, "unicorn-service")
	}
	if cfg.ClientID != "unicorn-client" {
		t.Errorf("ClientID = %q, want %q", cfg.ClientID, "unicorn-client")
	}
	if cfg.Version != "2.8.0" {
		t.Errorf("Version = %q, want %q", cfg.Version, "2.8.0")
	}
	if cfg.RequiredAcks != sarama.WaitForAll {
		t.Errorf("RequiredAcks = %v, want %v", cfg.RequiredAcks, sarama.WaitForAll)
	}
	if cfg.Compression != sarama.CompressionSnappy {
		t.Errorf("Compression = %v, want %v", cfg.Compression, sarama.CompressionSnappy)
	}
	if cfg.MaxMessageBytes != 1024*1024 {
		t.Errorf("MaxMessageBytes = %d, want %d", cfg.MaxMessageBytes, 1024*1024)
	}
	if cfg.BatchSize != 100 {
		t.Errorf("BatchSize = %d, want %d", cfg.BatchSize, 100)
	}
	if cfg.BatchTimeout != 10*time.Millisecond {
		t.Errorf("BatchTimeout = %v, want %v", cfg.BatchTimeout, 10*time.Millisecond)
	}
	if cfg.OffsetInitial != sarama.OffsetNewest {
		t.Errorf("OffsetInitial = %d, want %d", cfg.OffsetInitial, sarama.OffsetNewest)
	}
	if cfg.SessionTimeout != 10*time.Second {
		t.Errorf("SessionTimeout = %v, want %v", cfg.SessionTimeout, 10*time.Second)
	}
	if cfg.HeartbeatInterval != 3*time.Second {
		t.Errorf("HeartbeatInterval = %v, want %v", cfg.HeartbeatInterval, 3*time.Second)
	}
	if cfg.RebalanceStrategy != "roundrobin" {
		t.Errorf("RebalanceStrategy = %q, want %q", cfg.RebalanceStrategy, "roundrobin")
	}
	if cfg.AutoCommit != true {
		t.Errorf("AutoCommit = %v, want true", cfg.AutoCommit)
	}
	if cfg.AutoCommitInterval != 1*time.Second {
		t.Errorf("AutoCommitInterval = %v, want %v", cfg.AutoCommitInterval, 1*time.Second)
	}
}

// =============================================================================
// NewDriver
// =============================================================================

func TestNewDriver_NilConfig(t *testing.T) {
	d := NewDriver(nil)

	if d == nil {
		t.Fatal("NewDriver(nil) returned nil")
	}
	// nil config should fall back to DefaultConfig
	if d.config == nil {
		t.Fatal("config is nil, expected DefaultConfig")
	}
	if d.config.Brokers[0] != "localhost:9092" {
		t.Errorf("Brokers = %v, want [localhost:9092]", d.config.Brokers)
	}
	if d.config.GroupID != "unicorn-service" {
		t.Errorf("GroupID = %q, want %q", d.config.GroupID, "unicorn-service")
	}
	if d.consumers == nil {
		t.Error("consumers map is nil, expected initialized map")
	}
	if d.connected {
		t.Error("connected = true, want false")
	}
}

func TestNewDriver_CustomConfig(t *testing.T) {
	customCfg := &Config{
		Brokers:   []string{"broker1:9092", "broker2:9092"},
		GroupID:   "custom-group",
		ClientID:  "custom-client",
		Version:   "3.0.0",
		BatchSize: 500,
	}
	d := NewDriver(customCfg)

	if d == nil {
		t.Fatal("NewDriver(cfg) returned nil")
	}
	if d.config != customCfg {
		t.Error("config pointer mismatch, expected the same config instance")
	}
	if len(d.config.Brokers) != 2 {
		t.Errorf("Brokers length = %d, want 2", len(d.config.Brokers))
	}
	if d.config.GroupID != "custom-group" {
		t.Errorf("GroupID = %q, want %q", d.config.GroupID, "custom-group")
	}
	if d.config.ClientID != "custom-client" {
		t.Errorf("ClientID = %q, want %q", d.config.ClientID, "custom-client")
	}
	if d.config.BatchSize != 500 {
		t.Errorf("BatchSize = %d, want 500", d.config.BatchSize)
	}
	if d.consumers == nil {
		t.Error("consumers map is nil, expected initialized map")
	}
	if d.connected {
		t.Error("connected = true, want false")
	}
}

// =============================================================================
// Name
// =============================================================================

func TestName(t *testing.T) {
	d := NewDriver(nil)
	if name := d.Name(); name != "kafka" {
		t.Errorf("Name() = %q, want %q", name, "kafka")
	}
}

// =============================================================================
// IsConnected
// =============================================================================

func TestIsConnected_DefaultFalse(t *testing.T) {
	d := NewDriver(nil)
	if d.IsConnected() {
		t.Error("IsConnected() = true, want false for new driver")
	}
}

func TestIsConnected_AfterManualSet(t *testing.T) {
	d := NewDriver(nil)
	d.connected = true
	if !d.IsConnected() {
		t.Error("IsConnected() = false, want true after setting connected=true")
	}
	d.connected = false
	if d.IsConnected() {
		t.Error("IsConnected() = true, want false after setting connected=false")
	}
}

// =============================================================================
// Unsubscribe
// =============================================================================

func TestUnsubscribe_NoOpForMissingTopic(t *testing.T) {
	d := NewDriver(nil)
	// Should not panic and should return nil for missing topic
	err := d.Unsubscribe("nonexistent-topic")
	if err != nil {
		t.Errorf("Unsubscribe(missing) returned error: %v", err)
	}
}

func TestUnsubscribe_CancelsAndRemovesConsumer(t *testing.T) {
	d := NewDriver(nil)

	ctx, cancel := context.WithCancel(context.Background())

	// Manually add a consumer (simulating what Subscribe would do)
	d.mu.Lock()
	d.consumers["test-topic"] = cancel
	d.mu.Unlock()

	// Verify consumer exists
	d.mu.RLock()
	_, ok := d.consumers["test-topic"]
	d.mu.RUnlock()
	if !ok {
		t.Fatal("consumer not found before unsubscribe")
	}

	err := d.Unsubscribe("test-topic")
	if err != nil {
		t.Errorf("Unsubscribe(test-topic) returned error: %v", err)
	}

	// Context should be cancelled
	select {
	case <-ctx.Done():
		// Expected - context cancelled
	default:
		t.Error("context was not cancelled after Unsubscribe")
	}

	// Consumer should be removed from map
	d.mu.RLock()
	_, ok = d.consumers["test-topic"]
	d.mu.RUnlock()
	if ok {
		t.Error("consumer still exists in map after unsubscribe")
	}
}

// =============================================================================
// LeaveGroup
// =============================================================================

func TestLeaveGroup_NoConsumerGroup(t *testing.T) {
	d := NewDriver(nil)
	err := d.LeaveGroup("some-group")
	if err != nil {
		t.Errorf("LeaveGroup() with no consumer group returned error: %v", err)
	}
}

// =============================================================================
// Ack
// =============================================================================

func TestAck(t *testing.T) {
	d := NewDriver(nil)
	ctx := context.Background()
	msg := &contracts.BrokerMessage{
		Topic: "test",
		Body:  []byte("hello"),
	}
	err := d.Ack(ctx, msg)
	if err != nil {
		t.Errorf("Ack() returned error: %v", err)
	}
}

// =============================================================================
// Nack
// =============================================================================

func TestNack(t *testing.T) {
	d := NewDriver(nil)
	ctx := context.Background()
	msg := &contracts.BrokerMessage{
		Topic: "test",
		Body:  []byte("hello"),
	}

	t.Run("requeue true", func(t *testing.T) {
		err := d.Nack(ctx, msg, true)
		if err != nil {
			t.Errorf("Nack(requeue=true) returned error: %v", err)
		}
	})

	t.Run("requeue false", func(t *testing.T) {
		err := d.Nack(ctx, msg, false)
		if err != nil {
			t.Errorf("Nack(requeue=false) returned error: %v", err)
		}
	})
}

// =============================================================================
// QueueLength
// =============================================================================

func TestQueueLength(t *testing.T) {
	d := NewDriver(nil)
	ctx := context.Background()

	length, err := d.QueueLength(ctx, "test-queue")
	if length != 0 {
		t.Errorf("QueueLength() length = %d, want 0", length)
	}
	if err == nil {
		t.Error("QueueLength() expected an error, got nil")
	}
}

// =============================================================================
// Ping
// =============================================================================

func TestPing_NotConnected(t *testing.T) {
	d := NewDriver(nil)
	ctx := context.Background()
	err := d.Ping(ctx)
	if err == nil {
		t.Error("Ping() expected error when not connected, got nil")
	}
	if err.Error() != "kafka: not connected" {
		t.Errorf("Ping() error = %q, want %q", err.Error(), "kafka: not connected")
	}
}

// =============================================================================
// Publish
// =============================================================================

func TestPublish_NotConnected(t *testing.T) {
	d := NewDriver(nil)
	ctx := context.Background()
	msg := &contracts.BrokerMessage{
		Topic: "test",
		Body:  []byte("hello"),
	}
	err := d.Publish(ctx, msg.Topic, msg)
	if err == nil {
		t.Error("Publish() expected error when not connected, got nil")
	}
	if err.Error() != "kafka: not connected" {
		t.Errorf("Publish() error = %q, want %q", err.Error(), "kafka: not connected")
	}
}

// =============================================================================
// PublishBatch
// =============================================================================

func TestPublishBatch_NotConnected(t *testing.T) {
	d := NewDriver(nil)
	ctx := context.Background()
	msgs := []*contracts.BrokerMessage{
		{Topic: "test", Body: []byte("msg1")},
		{Topic: "test", Body: []byte("msg2")},
	}
	err := d.PublishBatch(ctx, "test", msgs)
	if err == nil {
		t.Error("PublishBatch() expected error when not connected, got nil")
	}
	if err.Error() != "kafka: not connected" {
		t.Errorf("PublishBatch() error = %q, want %q", err.Error(), "kafka: not connected")
	}
}

// =============================================================================
// Subscribe
// =============================================================================

func TestSubscribe_DelegatesToConsumeGroup(t *testing.T) {
	d := NewDriver(nil)
	ctx := context.Background()
	handler := func(ctx context.Context, msg *contracts.BrokerMessage) error {
		return nil
	}

	// Subscribe delegates to ConsumeGroup, which tries to connect to Kafka.
	// Since no Kafka is running, this should return an error.
	err := d.Subscribe(ctx, "test-topic", handler)
	if err == nil {
		t.Error("Subscribe() expected error (no Kafka), got nil")
	}
}

// =============================================================================
// SubscribeMultiple
// =============================================================================

func TestSubscribeMultiple_DelegatesToConsumeGroup(t *testing.T) {
	d := NewDriver(nil)
	ctx := context.Background()
	handler := func(ctx context.Context, msg *contracts.BrokerMessage) error {
		return nil
	}

	// SubscribeMultiple delegates to ConsumeGroup, which tries to connect to Kafka.
	// Since no Kafka is running, this should return an error.
	err := d.SubscribeMultiple(ctx, []string{"topic1", "topic2"}, handler)
	if err == nil {
		t.Error("SubscribeMultiple() expected error (no Kafka), got nil")
	}
}

// =============================================================================
// Disconnect
// =============================================================================

func TestDisconnect_CancelsConsumers(t *testing.T) {
	d := NewDriver(nil)

	// Add a consumer with a cancel function
	ctx, cancel := context.WithCancel(context.Background())
	d.mu.Lock()
	d.consumers["topic-a"] = cancel
	d.mu.Unlock()

	err := d.Disconnect(context.Background())
	if err != nil {
		t.Errorf("Disconnect() returned error: %v", err)
	}

	// Consumer context should be cancelled
	select {
	case <-ctx.Done():
		// Expected
	default:
		t.Error("consumer context was not cancelled after Disconnect")
	}

	// Consumers map should be reinitialized (empty)
	d.mu.RLock()
	count := len(d.consumers)
	d.mu.RUnlock()
	if count != 0 {
		t.Errorf("consumers map has %d entries after disconnect, want 0", count)
	}

	// connected should be false
	if d.connected {
		t.Error("connected = true after disconnect, want false")
	}
}

func TestDisconnect_NilSafe(t *testing.T) {
	d := NewDriver(nil)

	// Set fields to nil to test nil-safety
	d.client = nil
	d.producer = nil
	d.consumerGroup = nil
	d.consumers = nil

	// Should not panic
	err := d.Disconnect(context.Background())
	if err != nil {
		t.Errorf("Disconnect() with nil fields returned error: %v", err)
	}
}

func TestDisconnect_MultipleCallsSafe(t *testing.T) {
	d := NewDriver(nil)

	_, cancel := context.WithCancel(context.Background())
	d.mu.Lock()
	d.consumers["topic-x"] = cancel
	d.mu.Unlock()

	// First disconnect
	if err := d.Disconnect(context.Background()); err != nil {
		t.Errorf("first Disconnect() returned error: %v", err)
	}

	// Second disconnect should be safe (no panic)
	if err := d.Disconnect(context.Background()); err != nil {
		t.Errorf("second Disconnect() returned error: %v", err)
	}
}

func TestDisconnect_ResetsConnected(t *testing.T) {
	d := NewDriver(nil)
	d.connected = true

	err := d.Disconnect(context.Background())
	if err != nil {
		t.Errorf("Disconnect() returned error: %v", err)
	}
	if d.connected {
		t.Error("connected should be false after Disconnect")
	}
}

// =============================================================================
// buildSaramaConfig
// =============================================================================

func TestBuildSaramaConfig_Defaults(t *testing.T) {
	d := NewDriver(nil)
	cfg, err := d.buildSaramaConfig()

	if err != nil {
		t.Fatalf("buildSaramaConfig() returned error: %v", err)
	}
	if cfg == nil {
		t.Fatal("buildSaramaConfig() returned nil config")
	}

	// Version
	if cfg.Version != sarama.V2_8_0_0 {
		t.Errorf("Version = %v, want %v", cfg.Version, sarama.V2_8_0_0)
	}

	// ClientID
	if cfg.ClientID != "unicorn-client" {
		t.Errorf("ClientID = %q, want %q", cfg.ClientID, "unicorn-client")
	}

	// Producer settings
	if cfg.Producer.RequiredAcks != sarama.WaitForAll {
		t.Errorf("Producer.RequiredAcks = %v, want %v", cfg.Producer.RequiredAcks, sarama.WaitForAll)
	}
	if cfg.Producer.Compression != sarama.CompressionSnappy {
		t.Errorf("Producer.Compression = %v, want %v", cfg.Producer.Compression, sarama.CompressionSnappy)
	}
	if cfg.Producer.MaxMessageBytes != 1024*1024 {
		t.Errorf("Producer.MaxMessageBytes = %d, want %d", cfg.Producer.MaxMessageBytes, 1024*1024)
	}
	if cfg.Producer.Flush.Messages != 100 {
		t.Errorf("Producer.Flush.Messages = %d, want 100", cfg.Producer.Flush.Messages)
	}
	if cfg.Producer.Flush.Frequency != 10*time.Millisecond {
		t.Errorf("Producer.Flush.Frequency = %v, want %v", cfg.Producer.Flush.Frequency, 10*time.Millisecond)
	}
	if !cfg.Producer.Return.Successes {
		t.Error("Producer.Return.Successes = false, want true")
	}
	if !cfg.Producer.Return.Errors {
		t.Error("Producer.Return.Errors = false, want true")
	}

	// Consumer settings
	if cfg.Consumer.Offsets.Initial != sarama.OffsetNewest {
		t.Errorf("Consumer.Offsets.Initial = %d, want %d", cfg.Consumer.Offsets.Initial, sarama.OffsetNewest)
	}
	if cfg.Consumer.Group.Session.Timeout != 10*time.Second {
		t.Errorf("Consumer.Group.Session.Timeout = %v, want %v", cfg.Consumer.Group.Session.Timeout, 10*time.Second)
	}
	if cfg.Consumer.Group.Heartbeat.Interval != 3*time.Second {
		t.Errorf("Consumer.Group.Heartbeat.Interval = %v, want %v", cfg.Consumer.Group.Heartbeat.Interval, 3*time.Second)
	}

	// Auto commit
	if !cfg.Consumer.Offsets.AutoCommit.Enable {
		t.Error("Consumer.Offsets.AutoCommit.Enable = false, want true")
	}
	if cfg.Consumer.Offsets.AutoCommit.Interval != 1*time.Second {
		t.Errorf("Consumer.Offsets.AutoCommit.Interval = %v, want %v",
			cfg.Consumer.Offsets.AutoCommit.Interval, 1*time.Second)
	}

	// Default rebalance strategy should be roundrobin
	if len(cfg.Consumer.Group.Rebalance.GroupStrategies) != 1 {
		t.Fatalf("GroupStrategies length = %d, want 1",
			len(cfg.Consumer.Group.Rebalance.GroupStrategies))
	}
}

func TestBuildSaramaConfig_RebalanceRange(t *testing.T) {
	d := NewDriver(&Config{
		Brokers:           []string{"localhost:9092"},
		GroupID:           "test",
		RebalanceStrategy: "range",
	})

	cfg, err := d.buildSaramaConfig()
	if err != nil {
		t.Fatalf("buildSaramaConfig() returned error: %v", err)
	}

	if len(cfg.Consumer.Group.Rebalance.GroupStrategies) != 1 {
		t.Fatalf("GroupStrategies length = %d, want 1",
			len(cfg.Consumer.Group.Rebalance.GroupStrategies))
	}
	// Verify it's a RangeBalanceStrategy by checking the name
	name := cfg.Consumer.Group.Rebalance.GroupStrategies[0].Name()
	if name != "range" {
		t.Errorf("GroupStrategy name = %q, want %q", name, "range")
	}
}

func TestBuildSaramaConfig_RebalanceSticky(t *testing.T) {
	d := NewDriver(&Config{
		Brokers:           []string{"localhost:9092"},
		GroupID:           "test",
		RebalanceStrategy: "sticky",
	})

	cfg, err := d.buildSaramaConfig()
	if err != nil {
		t.Fatalf("buildSaramaConfig() returned error: %v", err)
	}

	if len(cfg.Consumer.Group.Rebalance.GroupStrategies) != 1 {
		t.Fatalf("GroupStrategies length = %d, want 1",
			len(cfg.Consumer.Group.Rebalance.GroupStrategies))
	}
	name := cfg.Consumer.Group.Rebalance.GroupStrategies[0].Name()
	if name != "sticky" {
		t.Errorf("GroupStrategy name = %q, want %q", name, "sticky")
	}
}

func TestBuildSaramaConfig_RebalanceRoundRobin(t *testing.T) {
	d := NewDriver(&Config{
		Brokers:           []string{"localhost:9092"},
		GroupID:           "test",
		RebalanceStrategy: "roundrobin",
	})

	cfg, err := d.buildSaramaConfig()
	if err != nil {
		t.Fatalf("buildSaramaConfig() returned error: %v", err)
	}

	if len(cfg.Consumer.Group.Rebalance.GroupStrategies) != 1 {
		t.Fatalf("GroupStrategies length = %d, want 1",
			len(cfg.Consumer.Group.Rebalance.GroupStrategies))
	}
	name := cfg.Consumer.Group.Rebalance.GroupStrategies[0].Name()
	if name != "roundrobin" {
		t.Errorf("GroupStrategy name = %q, want %q", name, "roundrobin")
	}
}

func TestBuildSaramaConfig_RebalanceUnknownDefaultsToRoundRobin(t *testing.T) {
	d := NewDriver(&Config{
		Brokers:           []string{"localhost:9092"},
		GroupID:           "test",
		RebalanceStrategy: "nonexistent-strategy",
	})

	cfg, err := d.buildSaramaConfig()
	if err != nil {
		t.Fatalf("buildSaramaConfig() returned error: %v", err)
	}

	if len(cfg.Consumer.Group.Rebalance.GroupStrategies) != 1 {
		t.Fatalf("GroupStrategies length = %d, want 1",
			len(cfg.Consumer.Group.Rebalance.GroupStrategies))
	}
	name := cfg.Consumer.Group.Rebalance.GroupStrategies[0].Name()
	if name != "roundrobin" {
		t.Errorf("GroupStrategy name = %q, want %q (fallback to roundrobin)", name, "roundrobin")
	}
}

func TestBuildSaramaConfig_InvalidVersionFallsBack(t *testing.T) {
	d := NewDriver(&Config{
		Brokers: []string{"localhost:9092"},
		GroupID: "test",
		Version: "not-a-version",
	})

	cfg, err := d.buildSaramaConfig()
	if err != nil {
		t.Fatalf("buildSaramaConfig() returned error: %v", err)
	}

	// Invalid version should fall back to V2_8_0_0
	if cfg.Version != sarama.V2_8_0_0 {
		t.Errorf("Version = %v, want V2_8_0_0 (fallback)", cfg.Version)
	}
}

func TestBuildSaramaConfig_ValidVersion(t *testing.T) {
	d := NewDriver(&Config{
		Brokers: []string{"localhost:9092"},
		GroupID: "test",
		Version: "3.6.0",
	})

	cfg, err := d.buildSaramaConfig()
	if err != nil {
		t.Fatalf("buildSaramaConfig() returned error: %v", err)
	}

	expectedVer, _ := sarama.ParseKafkaVersion("3.6.0")
	if cfg.Version != expectedVer {
		t.Errorf("Version = %v, want %v", cfg.Version, expectedVer)
	}
}

func TestBuildSaramaConfig_ProducerSettings(t *testing.T) {
	d := NewDriver(&Config{
		Brokers:         []string{"localhost:9092"},
		GroupID:         "test",
		RequiredAcks:    sarama.WaitForLocal,
		Compression:     sarama.CompressionGZIP,
		MaxMessageBytes: 512 * 1024,
		BatchSize:       50,
		BatchTimeout:    5 * time.Millisecond,
	})

	cfg, err := d.buildSaramaConfig()
	if err != nil {
		t.Fatalf("buildSaramaConfig() returned error: %v", err)
	}

	if cfg.Producer.RequiredAcks != sarama.WaitForLocal {
		t.Errorf("Producer.RequiredAcks = %v, want %v", cfg.Producer.RequiredAcks, sarama.WaitForLocal)
	}
	if cfg.Producer.Compression != sarama.CompressionGZIP {
		t.Errorf("Producer.Compression = %v, want %v", cfg.Producer.Compression, sarama.CompressionGZIP)
	}
	if cfg.Producer.MaxMessageBytes != 512*1024 {
		t.Errorf("Producer.MaxMessageBytes = %d, want %d", cfg.Producer.MaxMessageBytes, 512*1024)
	}
	if cfg.Producer.Flush.Messages != 50 {
		t.Errorf("Producer.Flush.Messages = %d, want 50", cfg.Producer.Flush.Messages)
	}
	if cfg.Producer.Flush.Frequency != 5*time.Millisecond {
		t.Errorf("Producer.Flush.Frequency = %v, want %v", cfg.Producer.Flush.Frequency, 5*time.Millisecond)
	}
}

func TestBuildSaramaConfig_ConsumerSettings(t *testing.T) {
	d := NewDriver(&Config{
		Brokers:            []string{"localhost:9092"},
		GroupID:            "test",
		OffsetInitial:      sarama.OffsetOldest,
		SessionTimeout:     30 * time.Second,
		HeartbeatInterval:  5 * time.Second,
		AutoCommit:         false,
		AutoCommitInterval: 5 * time.Second,
	})

	cfg, err := d.buildSaramaConfig()
	if err != nil {
		t.Fatalf("buildSaramaConfig() returned error: %v", err)
	}

	if cfg.Consumer.Offsets.Initial != sarama.OffsetOldest {
		t.Errorf("Consumer.Offsets.Initial = %d, want %d", cfg.Consumer.Offsets.Initial, sarama.OffsetOldest)
	}
	if cfg.Consumer.Group.Session.Timeout != 30*time.Second {
		t.Errorf("Consumer.Group.Session.Timeout = %v, want %v", cfg.Consumer.Group.Session.Timeout, 30*time.Second)
	}
	if cfg.Consumer.Group.Heartbeat.Interval != 5*time.Second {
		t.Errorf("Consumer.Group.Heartbeat.Interval = %v, want %v", cfg.Consumer.Group.Heartbeat.Interval, 5*time.Second)
	}
	if cfg.Consumer.Offsets.AutoCommit.Enable {
		t.Error("Consumer.Offsets.AutoCommit.Enable = true, want false")
	}
	if cfg.Consumer.Offsets.AutoCommit.Interval != 5*time.Second {
		t.Errorf("Consumer.Offsets.AutoCommit.Interval = %v, want %v",
			cfg.Consumer.Offsets.AutoCommit.Interval, 5*time.Second)
	}
}

func TestBuildSaramaConfig_SASLPlain(t *testing.T) {
	d := NewDriver(&Config{
		Brokers:       []string{"localhost:9092"},
		GroupID:       "test",
		UseSASL:       true,
		SASLMechanism: "PLAIN",
		SASLUser:      "admin",
		SASLPassword:  "secret",
	})

	cfg, err := d.buildSaramaConfig()
	if err != nil {
		t.Fatalf("buildSaramaConfig() returned error: %v", err)
	}

	if !cfg.Net.SASL.Enable {
		t.Error("Net.SASL.Enable = false, want true")
	}
	if cfg.Net.SASL.User != "admin" {
		t.Errorf("Net.SASL.User = %q, want %q", cfg.Net.SASL.User, "admin")
	}
	if cfg.Net.SASL.Password != "secret" {
		t.Errorf("Net.SASL.Password = %q, want %q", cfg.Net.SASL.Password, "secret")
	}
	if cfg.Net.SASL.Mechanism != sarama.SASLTypePlaintext {
		t.Errorf("Net.SASL.Mechanism = %q, want %q", cfg.Net.SASL.Mechanism, sarama.SASLTypePlaintext)
	}
}

func TestBuildSaramaConfig_SASLSCRAMSHA256(t *testing.T) {
	d := NewDriver(&Config{
		Brokers:       []string{"localhost:9092"},
		GroupID:       "test",
		UseSASL:       true,
		SASLMechanism: "SCRAM-SHA-256",
		SASLUser:      "scramuser",
		SASLPassword:  "scrampass",
	})

	cfg, err := d.buildSaramaConfig()
	if err != nil {
		t.Fatalf("buildSaramaConfig() returned error: %v", err)
	}

	if !cfg.Net.SASL.Enable {
		t.Error("Net.SASL.Enable = false, want true")
	}
	if cfg.Net.SASL.User != "scramuser" {
		t.Errorf("Net.SASL.User = %q, want %q", cfg.Net.SASL.User, "scramuser")
	}
	if cfg.Net.SASL.Mechanism != sarama.SASLTypeSCRAMSHA256 {
		t.Errorf("Net.SASL.Mechanism = %q, want %q", cfg.Net.SASL.Mechanism, sarama.SASLTypeSCRAMSHA256)
	}
}

func TestBuildSaramaConfig_SASLSCRAMSHA512(t *testing.T) {
	d := NewDriver(&Config{
		Brokers:       []string{"localhost:9092"},
		GroupID:       "test",
		UseSASL:       true,
		SASLMechanism: "SCRAM-SHA-512",
		SASLUser:      "scramuser",
		SASLPassword:  "scrampass",
	})

	cfg, err := d.buildSaramaConfig()
	if err != nil {
		t.Fatalf("buildSaramaConfig() returned error: %v", err)
	}

	if !cfg.Net.SASL.Enable {
		t.Error("Net.SASL.Enable = false, want true")
	}
	if cfg.Net.SASL.Mechanism != sarama.SASLTypeSCRAMSHA512 {
		t.Errorf("Net.SASL.Mechanism = %q, want %q", cfg.Net.SASL.Mechanism, sarama.SASLTypeSCRAMSHA512)
	}
}

func TestBuildSaramaConfig_SASLDisabledByDefault(t *testing.T) {
	d := NewDriver(nil)
	cfg, err := d.buildSaramaConfig()
	if err != nil {
		t.Fatalf("buildSaramaConfig() returned error: %v", err)
	}

	if cfg.Net.SASL.Enable {
		t.Error("Net.SASL.Enable = true, want false by default")
	}
}

// =============================================================================
// Interface compliance
// =============================================================================

func TestDriverImplementsBroker(t *testing.T) {
	var d *Driver
	// This will only compile if *Driver satisfies contracts.Broker
	var _ contracts.Broker = d
	_ = d // avoid unused variable warning in certain linters
}

// =============================================================================
// Error types
// =============================================================================

func TestNotConnectedError(t *testing.T) {
	d := NewDriver(nil)
	ctx := context.Background()

	tests := []struct {
		name string
		fn   func() error
	}{
		{"Ping", func() error { return d.Ping(ctx) }},
		{"Publish", func() error {
			return d.Publish(ctx, "t", &contracts.BrokerMessage{Body: []byte("x")})
		}},
		{"PublishBatch", func() error {
			return d.PublishBatch(ctx, "t", []*contracts.BrokerMessage{{Body: []byte("x")}})
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			if err == nil {
				t.Errorf("%s() expected error, got nil", tt.name)
				return
			}
			if err.Error() != "kafka: not connected" {
				t.Errorf("%s() error = %q, want %q", tt.name, err.Error(), "kafka: not connected")
			}
		})
	}
}
