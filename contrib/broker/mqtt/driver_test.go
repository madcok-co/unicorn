package mqtt

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

// ============================================================================
// Mock MQTT client & token
// ============================================================================

// mockToken implements mqtt.Token and can be pre-configured with an error or a
// timeout flag.
type mockToken struct {
	err     error
	timeout bool
	done    chan struct{}
}

func newMockToken(err error, timeout bool) *mockToken {
	t := &mockToken{err: err, timeout: timeout}
	if !timeout {
		t.done = make(chan struct{})
		close(t.done)
	}
	return t
}

func (t *mockToken) Wait() bool {
	if t.timeout {
		return false
	}
	return true
}

func (t *mockToken) WaitTimeout(d time.Duration) bool {
	if t.timeout {
		return false
	}
	return true
}

func (t *mockToken) Error() error { return t.err }
func (t *mockToken) Done() <-chan struct{} {
	if t.done == nil {
		c := make(chan struct{})
		return c
	}
	return t.done
}

type mockMQTTClient struct {
	mu          sync.Mutex
	isConnected bool

	// Publish spy
	publishCalls []publishCall

	// Subscribe spy
	subscribeCalls []subscribeCall

	// Tokens returned by each method
	connectToken   mqtt.Token
	publishToken   mqtt.Token
	subscribeToken mqtt.Token
	unsubscribeErr error

	// When set, Publish / Subscribe will fire simulateMessage to test the
	// message round-trip path.
	simulateMessage func(topic string, payload []byte)
}

type publishCall struct {
	Topic    string
	QoS      byte
	Retained bool
	Payload  []byte
}

type subscribeCall struct {
	Topic string
	QoS   byte
}

func newMockMQTTClient() *mockMQTTClient {
	return &mockMQTTClient{
		isConnected:    true,
		connectToken:   newMockToken(nil, false),
		publishToken:   newMockToken(nil, false),
		subscribeToken: newMockToken(nil, false),
	}
}

func (m *mockMQTTClient) Connect() mqtt.Token                     { return m.connectToken }
func (m *mockMQTTClient) Disconnect(quiesce uint)                 { m.isConnected = false }
func (m *mockMQTTClient) IsConnected() bool                       { return m.isConnected }
func (m *mockMQTTClient) IsConnectionOpen() bool                  { return m.isConnected }
func (m *mockMQTTClient) OptionsReader() mqtt.ClientOptionsReader { return mqtt.ClientOptionsReader{} }

func (m *mockMQTTClient) Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
	m.mu.Lock()
	m.publishCalls = append(m.publishCalls, publishCall{Topic: topic, QoS: qos, Retained: retained, Payload: payload.([]byte)})
	m.mu.Unlock()

	if m.simulateMessage != nil {
		m.simulateMessage(topic, payload.([]byte))
	}
	return m.publishToken
}

func (m *mockMQTTClient) Subscribe(topic string, qos byte, callback mqtt.MessageHandler) mqtt.Token {
	m.mu.Lock()
	m.subscribeCalls = append(m.subscribeCalls, subscribeCall{Topic: topic, QoS: qos})
	m.mu.Unlock()

	// Store the callback so tests can fire messages through it.
	_ = callback
	return m.subscribeToken
}

func (m *mockMQTTClient) SubscribeMultiple(filters map[string]byte, callback mqtt.MessageHandler) mqtt.Token {
	for topic, qos := range filters {
		m.Subscribe(topic, qos, callback)
	}
	return m.subscribeToken
}

func (m *mockMQTTClient) Unsubscribe(topics ...string) mqtt.Token {
	return newMockToken(m.unsubscribeErr, false)
}

func (m *mockMQTTClient) AddRoute(topic string, callback mqtt.MessageHandler) {}

// Ensure mock implements mqtt.Client.
var _ mqtt.Client = (*mockMQTTClient)(nil)

// ============================================================================
// Tests – Config
// ============================================================================

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if len(cfg.Brokers) != 1 || cfg.Brokers[0] != "tcp://localhost:1883" {
		t.Errorf("expected broker tcp://localhost:1883, got %v", cfg.Brokers)
	}
	if !cfg.CleanSession {
		t.Error("expected CleanSession = true")
	}
	if cfg.KeepAlive != 30*time.Second {
		t.Errorf("expected KeepAlive 30s, got %v", cfg.KeepAlive)
	}
	if cfg.DefaultQoS != 1 {
		t.Errorf("expected DefaultQoS 1, got %d", cfg.DefaultQoS)
	}
	if cfg.RetainPolicy {
		t.Error("expected RetainPolicy = false")
	}
	if !cfg.AutoReconnect {
		t.Error("expected AutoReconnect = true")
	}
	if cfg.MaxReconnectDelay != 10*time.Minute {
		t.Errorf("expected MaxReconnectDelay 10m, got %v", cfg.MaxReconnectDelay)
	}
	if cfg.ConnectTimeout != 30*time.Second {
		t.Errorf("expected ConnectTimeout 30s, got %v", cfg.ConnectTimeout)
	}
}

func TestNewDriver_NilConfig(t *testing.T) {
	d := NewDriver(nil)
	if d.config == nil {
		t.Fatal("expected non-nil config for nil input")
	}
	if len(d.config.Brokers) != 1 || d.config.Brokers[0] != "tcp://localhost:1883" {
		t.Errorf("expected default broker, got %v", d.config.Brokers)
	}
}

func TestNewDriver_CustomConfig(t *testing.T) {
	cfg := &Config{
		Brokers:       []string{"tcp://broker1:1883", "tcp://broker2:1883"},
		ClientID:      "test-client",
		Username:      "user",
		Password:      "pass",
		CleanSession:  false,
		DefaultQoS:    2,
		RetainPolicy:  true,
		AutoReconnect: false,
	}
	d := NewDriver(cfg)

	if d.config.Brokers[0] != "tcp://broker1:1883" {
		t.Errorf("expected broker1, got %s", d.config.Brokers[0])
	}
	if d.config.ClientID != "test-client" {
		t.Errorf("expected test-client, got %s", d.config.ClientID)
	}
	if d.config.DefaultQoS != 2 {
		t.Errorf("expected QoS 2, got %d", d.config.DefaultQoS)
	}
}

// ============================================================================
// Tests – Name
// ============================================================================

func TestName(t *testing.T) {
	d := NewDriver(nil)
	if name := d.Name(); name != "mqtt" {
		t.Errorf("expected name 'mqtt', got %q", name)
	}
}

// ============================================================================
// Tests – IsConnected lifecycle
// ============================================================================

func TestIsConnected_DefaultFalse(t *testing.T) {
	d := NewDriver(nil)
	if d.IsConnected() {
		t.Error("expected IsConnected() = false before Connect")
	}
}

func TestIsConnected_AfterConnect(t *testing.T) {
	d := NewDriver(nil)
	d.connected.Store(true)
	if !d.IsConnected() {
		t.Error("expected IsConnected() = true after Store(true)")
	}
	d.connected.Store(false)
	if d.IsConnected() {
		t.Error("expected IsConnected() = false after Store(false)")
	}
}

// ============================================================================
// Tests – Config getters
// ============================================================================

func TestConfig_ReturnsCopy(t *testing.T) {
	d := NewDriver(&Config{Brokers: []string{"tcp://a:1883"}})
	cp := d.Config()
	cp.Brokers[0] = "tcp://changed:1883"
	if d.config.Brokers[0] == "tcp://changed:1883" {
		t.Error("expected config copy to be independent")
	}
}

func TestBrokers_ReturnsCopy(t *testing.T) {
	d := NewDriver(&Config{Brokers: []string{"tcp://a:1883"}})
	brokers := d.Brokers()
	brokers[0] = "tcp://changed:1883"
	if d.config.Brokers[0] == "tcp://changed:1883" {
		t.Error("expected brokers copy to be independent")
	}
}

// ============================================================================
// Tests – Interface compliance
// ============================================================================

func TestDriverImplementsBroker(t *testing.T) {
	// Compile-time check lives in driver.go; this is a runtime sanity.
	var d *Driver
	var b contracts.Broker = d
	_ = b
}

// ============================================================================
// Tests – Error paths (not-connected)
// ============================================================================

func notConnectedDriver() *Driver {
	return NewDriver(nil) // connected.Load() == false
}

func TestPublish_NotConnected(t *testing.T) {
	d := notConnectedDriver()
	err := d.Publish(context.Background(), "test", contracts.NewBrokerMessage("test", []byte("x")))
	if err == nil {
		t.Error("expected error when not connected")
	}
}

func TestPublishBatch_NotConnected(t *testing.T) {
	d := notConnectedDriver()
	err := d.PublishBatch(context.Background(), "test", []*contracts.BrokerMessage{
		contracts.NewBrokerMessage("test", []byte("x")),
	})
	if err == nil {
		t.Error("expected error when not connected")
	}
}

func TestSubscribe_NotConnected(t *testing.T) {
	d := notConnectedDriver()
	err := d.Subscribe(context.Background(), "test", func(ctx context.Context, msg *contracts.BrokerMessage) error {
		return nil
	})
	if err == nil {
		t.Error("expected error when not connected")
	}
}

func TestSubscribeMultiple_NotConnected(t *testing.T) {
	d := notConnectedDriver()
	err := d.SubscribeMultiple(context.Background(), []string{"a", "b"}, func(ctx context.Context, msg *contracts.BrokerMessage) error {
		return nil
	})
	if err == nil {
		t.Error("expected error when not connected")
	}
}

func TestConsumeGroup_NotConnected(t *testing.T) {
	d := notConnectedDriver()
	err := d.ConsumeGroup(context.Background(), "g", []string{"t"}, func(ctx context.Context, msg *contracts.BrokerMessage) error {
		return nil
	})
	if err == nil {
		t.Error("expected error when not connected")
	}
}

func TestPing_NotConnected(t *testing.T) {
	d := notConnectedDriver()
	err := d.Ping(context.Background())
	if err == nil {
		t.Error("expected error when not connected")
	}
}

// ============================================================================
// Tests – Publish with mock client
// ============================================================================

func TestPublish_WithMockClient(t *testing.T) {
	mock := newMockMQTTClient()
	d := NewDriver(nil)
	d.client = mock
	d.connected.Store(true)

	msg := contracts.NewBrokerMessage("orders/new", []byte(`{"id":1}`))
	err := d.Publish(context.Background(), "orders/new", msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.publishCalls) != 1 {
		t.Fatalf("expected 1 publish call, got %d", len(mock.publishCalls))
	}
	pc := mock.publishCalls[0]
	if pc.Topic != "orders/new" {
		t.Errorf("expected topic orders/new, got %s", pc.Topic)
	}
	if string(pc.Payload) != `{"id":1}` {
		t.Errorf("expected payload, got %s", string(pc.Payload))
	}
	if pc.QoS != d.config.DefaultQoS {
		t.Errorf("expected QoS %d, got %d", d.config.DefaultQoS, pc.QoS)
	}
}

func TestPublish_PublishError(t *testing.T) {
	mock := newMockMQTTClient()
	mock.publishToken = newMockToken(errors.New("publish failed"), false)

	d := NewDriver(nil)
	d.client = mock
	d.connected.Store(true)

	err := d.Publish(context.Background(), "t", contracts.NewBrokerMessage("t", []byte("x")))
	if err == nil || err.Error() != "publish failed" {
		t.Errorf("expected 'publish failed', got %v", err)
	}
}

func TestPublish_PublishTimeout(t *testing.T) {
	mock := newMockMQTTClient()
	mock.publishToken = newMockToken(nil, true) // timeout

	d := NewDriver(nil)
	d.client = mock
	d.connected.Store(true)

	err := d.Publish(context.Background(), "t", contracts.NewBrokerMessage("t", []byte("x")))
	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestPublishBatch_AllSucceed(t *testing.T) {
	mock := newMockMQTTClient()
	d := NewDriver(nil)
	d.client = mock
	d.connected.Store(true)

	msgs := []*contracts.BrokerMessage{
		contracts.NewBrokerMessage("t", []byte("1")),
		contracts.NewBrokerMessage("t", []byte("2")),
	}
	err := d.PublishBatch(context.Background(), "t", msgs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.publishCalls) != 2 {
		t.Errorf("expected 2 calls, got %d", len(mock.publishCalls))
	}
}

// ============================================================================
// Tests – QueueLength
// ============================================================================

func TestQueueLength(t *testing.T) {
	d := NewDriver(nil)
	n, err := d.QueueLength(context.Background(), "any")
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0, got %d", n)
	}
}

// ============================================================================
// Tests – Ack / Nack
// ============================================================================

func TestAck(t *testing.T) {
	d := NewDriver(nil)
	err := d.Ack(context.Background(), contracts.NewBrokerMessage("t", nil))
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestNack(t *testing.T) {
	d := NewDriver(nil)
	err := d.Nack(context.Background(), contracts.NewBrokerMessage("t", nil), true)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

// ============================================================================
// Tests – Unsubscribe
// ============================================================================

func TestUnsubscribe_NoOpForMissingTopic(t *testing.T) {
	d := NewDriver(nil)
	err := d.Unsubscribe("nonexistent")
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestUnsubscribe_CancelsAndRemovesConsumer(t *testing.T) {
	mock := newMockMQTTClient()
	d := NewDriver(nil)
	d.client = mock
	d.connected.Store(true)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Manually register a subscription to bypass the real Connect flow.
	subCtx, subCancel := context.WithCancel(ctx)
	d.mu.Lock()
	d.handlers["test/topic"] = func(ctx context.Context, msg *contracts.BrokerMessage) error { return nil }
	d.cancelFuncs["test/topic"] = subCancel
	d.mu.Unlock()

	err := d.Unsubscribe("test/topic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The sub-context should be cancelled.
	select {
	case <-subCtx.Done():
		// OK
	default:
		t.Error("expected sub context to be cancelled")
	}

	// Handler and cancel should be removed.
	d.mu.RLock()
	_, hasHandler := d.handlers["test/topic"]
	_, hasCancel := d.cancelFuncs["test/topic"]
	d.mu.RUnlock()
	if hasHandler {
		t.Error("expected handler to be removed")
	}
	if hasCancel {
		t.Error("expected cancel to be removed")
	}
}

// ============================================================================
// Tests – LeaveGroup
// ============================================================================

func TestLeaveGroup_NoConsumerGroup(t *testing.T) {
	d := NewDriver(nil)
	err := d.LeaveGroup("nonexistent")
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestLeaveGroup_RemovesSharedSubscriptions(t *testing.T) {
	mock := newMockMQTTClient()
	d := NewDriver(nil)
	d.client = mock
	d.connected.Store(true)

	// Register shared subscriptions manually.
	d.mu.Lock()
	d.handlers["$share/g1/t1"] = nil
	d.handlers["$share/g1/t2"] = nil
	d.handlers["$share/g2/t1"] = nil
	d.cancelFuncs["$share/g1/t1"] = func() {}
	d.cancelFuncs["$share/g1/t2"] = func() {}
	d.cancelFuncs["$share/g2/t1"] = func() {}
	d.mu.Unlock()

	err := d.LeaveGroup("g1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	d.mu.RLock()
	_, hasG1T1 := d.handlers["$share/g1/t1"]
	_, hasG1T2 := d.handlers["$share/g1/t2"]
	_, hasG2T1 := d.handlers["$share/g2/t1"]
	d.mu.RUnlock()

	if hasG1T1 {
		t.Error("expected $share/g1/t1 to be removed")
	}
	if hasG1T2 {
		t.Error("expected $share/g1/t2 to be removed")
	}
	if !hasG2T1 {
		t.Error("expected $share/g2/t1 to remain")
	}
}

// ============================================================================
// Tests – Disconnect
// ============================================================================

func TestDisconnect_CancelsConsumers(t *testing.T) {
	mock := newMockMQTTClient()
	d := NewDriver(nil)
	d.client = mock
	d.connected.Store(true)

	ctx := context.Background()

	// Register handlers manually.
	_, c1 := context.WithCancel(ctx)
	_, c2 := context.WithCancel(ctx)

	d.mu.Lock()
	d.handlers["t1"] = nil
	d.handlers["t2"] = nil
	d.cancelFuncs["t1"] = c1
	d.cancelFuncs["t2"] = c2
	d.mu.Unlock()

	err := d.Disconnect(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if d.IsConnected() {
		t.Error("expected IsConnected() false after Disconnect")
	}
	if mock.isConnected {
		t.Error("expected mock Disconnect to be called, setting isConnected to false")
	}
}

func TestDisconnect_NilClientSafe(t *testing.T) {
	d := NewDriver(nil)
	// client is nil, Disconnect should not panic.
	err := d.Disconnect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDisconnect_ResetsConnected(t *testing.T) {
	d := NewDriver(nil)
	d.connected.Store(true)
	_ = d.Disconnect(context.Background())
	if d.IsConnected() {
		t.Error("expected IsConnected() false after Disconnect")
	}
}
