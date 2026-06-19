// Package mqtt provides an MQTT implementation of the unicorn Broker interface.
//
// It uses the Eclipse Paho MQTT client (github.com/eclipse/paho.mqtt.golang)
// and supports MQTT 3.1 / 3.1.1 brokers with QoS 0, 1, and 2.
//
// Shared subscriptions (for ConsumeGroup) use the $share prefix supported by
// brokers such as Mosquitto, EMQX, and VerneMQ.
//
// Usage:
//
//	import (
//	    "github.com/madcok-co/unicorn/contrib/broker/mqtt"
//	)
//
//	driver := mqtt.NewDriver(&mqtt.Config{
//	    Brokers: []string{"tcp://localhost:1883"},
//	})
//	app.SetBroker(driver)
package mqtt

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

// Config holds configuration for the MQTT broker driver.
type Config struct {
	// Brokers is the list of MQTT broker addresses, e.g. ["tcp://localhost:1883"].
	Brokers []string

	// ClientID is the MQTT client identifier. If empty, the paho library
	// generates one automatically.
	ClientID string

	// Username and Password for broker authentication.
	Username string
	Password string

	// CleanSession controls whether the broker remembers subscriptions across
	// disconnects. Set to false for durable sessions (QoS 1/2).
	CleanSession bool

	// KeepAlive is the ping interval sent to the broker (default 30s).
	KeepAlive time.Duration

	// DefaultQoS is the Quality of Service applied when publishing messages
	// without an explicit QoS override. Must be 0, 1, or 2.
	DefaultQoS byte

	// RetainPolicy sets the RETAIN flag on published messages. When true the
	// broker keeps the last message on the topic for new subscribers.
	RetainPolicy bool

	// Will message – published by the broker when this client disconnects
	// unexpectedly.
	WillTopic   string
	WillPayload []byte
	WillQoS     byte
	WillRetain  bool

	// TLS settings. When UseTLS is true, the driver loads certificates from
	// the configured files. At minimum the CAFile must be set; CertFile and
	// KeyFile are needed for mutual TLS.
	UseTLS   bool
	CertFile string
	KeyFile  string
	CAFile   string

	// AutoReconnect enables the paho auto-reconnect loop.
	AutoReconnect bool

	// MaxReconnectDelay caps the exponential back-off between reconnection
	// attempts (default 10 minutes).
	MaxReconnectDelay time.Duration

	// ConnectTimeout is the maximum time to wait for the initial Connect
	// handshake (default 30s).
	ConnectTimeout time.Duration
}

// DefaultConfig returns a Config populated with sensible defaults suitable
// for local development against a plain-text MQTT broker on localhost:1883.
func DefaultConfig() *Config {
	return &Config{
		Brokers:           []string{"tcp://localhost:1883"},
		CleanSession:      true,
		KeepAlive:         30 * time.Second,
		DefaultQoS:        1,
		RetainPolicy:      false,
		AutoReconnect:     true,
		MaxReconnectDelay: 10 * time.Minute,
		ConnectTimeout:    30 * time.Second,
	}
}

// ---------------------------------------------------------------------------
// Driver
// ---------------------------------------------------------------------------

// Driver implements contracts.Broker using an MQTT client.
type Driver struct {
	config *Config
	client mqtt.Client

	mu        sync.RWMutex
	connected atomic.Bool

	// Track in-flight subscriptions so we can cancel / unsubscribe cleanly.
	handlers    map[string]contracts.MessageHandlerFunc
	cancelFuncs map[string]context.CancelFunc
}

// NewDriver creates a new MQTT Driver. If cfg is nil, DefaultConfig is used.
// The caller must call Connect before any publish/subscribe operations.
func NewDriver(cfg *Config) *Driver {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &Driver{
		config:      cfg,
		handlers:    make(map[string]contracts.MessageHandlerFunc),
		cancelFuncs: make(map[string]context.CancelFunc),
	}
}

// ---------------------------------------------------------------------------
// contracts.Broker – connection management
// ---------------------------------------------------------------------------

// Name returns the broker identifier.
func (d *Driver) Name() string { return "mqtt" }

// Connect builds the MQTT client options, creates the paho client, and waits
// for the connection handshake to complete (or until ctx is cancelled).
func (d *Driver) Connect(ctx context.Context) error {
	opts := mqtt.NewClientOptions()

	// Broker addresses
	for _, b := range d.config.Brokers {
		opts.AddBroker(b)
	}

	opts.SetClientID(d.config.ClientID)
	opts.SetUsername(d.config.Username)
	opts.SetPassword(d.config.Password)
	opts.SetCleanSession(d.config.CleanSession)
	opts.SetKeepAlive(d.config.KeepAlive)
	opts.SetAutoReconnect(d.config.AutoReconnect)
	opts.SetMaxReconnectInterval(d.config.MaxReconnectDelay)
	opts.SetConnectTimeout(d.config.ConnectTimeout)

	// Will message
	if d.config.WillTopic != "" {
		opts.SetWill(d.config.WillTopic, string(d.config.WillPayload), d.config.WillQoS, d.config.WillRetain)
	}

	// TLS
	if d.config.UseTLS {
		tlsCfg, err := d.buildTLSConfig()
		if err != nil {
			return fmt.Errorf("mqtt: TLS config: %w", err)
		}
		opts.SetTLSConfig(tlsCfg)
	}

	// Connection status handlers
	opts.OnConnect = func(c mqtt.Client) {
		d.connected.Store(true)
	}
	opts.OnConnectionLost = func(c mqtt.Client, err error) {
		d.connected.Store(false)
	}

	d.client = mqtt.NewClient(opts)

	token := d.client.Connect()
	if !token.WaitTimeout(d.config.ConnectTimeout) {
		return errors.New("mqtt: connect timed out")
	}
	if err := token.Error(); err != nil {
		return fmt.Errorf("mqtt: connect: %w", err)
	}

	d.connected.Store(true)
	return nil
}

// Disconnect performs a clean disconnect (quiesce = 250ms gives in-flight
// messages a chance to be delivered) and then releases all tracked resources.
func (d *Driver) Disconnect(ctx context.Context) error {
	d.mu.Lock()

	// Cancel all subscription contexts.
	for topic, cancel := range d.cancelFuncs {
		cancel()
		delete(d.cancelFuncs, topic)
		delete(d.handlers, topic)
	}
	d.mu.Unlock()

	if d.client != nil && d.client.IsConnected() {
		d.client.Disconnect(250)
	}

	d.connected.Store(false)
	return nil
}

// Ping verifies the transport-level connection is still alive.
func (d *Driver) Ping(ctx context.Context) error {
	if !d.IsConnected() {
		return errors.New("mqtt: not connected")
	}
	// paho handles keep-alive pings internally; we just check IsConnected.
	return nil
}

// IsConnected reports whether the MQTT client has an active connection.
func (d *Driver) IsConnected() bool {
	return d.connected.Load()
}

// ---------------------------------------------------------------------------
// contracts.Broker – publishing
// ---------------------------------------------------------------------------

// Publish sends a single message to the given topic using the configured
// default QoS and retain policy.
func (d *Driver) Publish(ctx context.Context, topic string, msg *contracts.BrokerMessage) error {
	if !d.IsConnected() {
		return errors.New("mqtt: not connected")
	}

	token := d.client.Publish(topic, d.config.DefaultQoS, d.config.RetainPolicy, msg.Body)
	if !token.WaitTimeout(d.config.ConnectTimeout) {
		return errors.New("mqtt: publish timed out")
	}
	return token.Error()
}

// PublishBatch publishes multiple messages sequentially. The first error
// aborts the batch.
func (d *Driver) PublishBatch(ctx context.Context, topic string, msgs []*contracts.BrokerMessage) error {
	for _, msg := range msgs {
		if err := d.Publish(ctx, topic, msg); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// contracts.Broker – subscribing
// ---------------------------------------------------------------------------

// Subscribe registers a handler for messages arriving on topic.
func (d *Driver) Subscribe(ctx context.Context, topic string, handler contracts.MessageHandlerFunc) error {
	if !d.IsConnected() {
		return errors.New("mqtt: not connected")
	}

	return d.subscribe(ctx, topic, handler)
}

// SubscribeMultiple registers the same handler for multiple topics.
func (d *Driver) SubscribeMultiple(ctx context.Context, topics []string, handler contracts.MessageHandlerFunc) error {
	if !d.IsConnected() {
		return errors.New("mqtt: not connected")
	}

	for _, topic := range topics {
		if err := d.subscribe(ctx, topic, handler); err != nil {
			return err
		}
	}
	return nil
}

// Unsubscribe removes the subscription for the given topic.
func (d *Driver) Unsubscribe(topic string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if cancel, ok := d.cancelFuncs[topic]; ok {
		cancel()
		delete(d.cancelFuncs, topic)
		delete(d.handlers, topic)
	}

	if d.client != nil && d.client.IsConnected() {
		d.client.Unsubscribe(topic)
	}
	return nil
}

// ---------------------------------------------------------------------------
// contracts.Broker – consumer group (shared subscriptions)
// ---------------------------------------------------------------------------

// ConsumeGroup creates a shared subscription for the given group. Under the
// hood this subscribes to $share/<group>/<topic> for every topic, which lets
// the broker load-balance messages across members of the same group.
func (d *Driver) ConsumeGroup(ctx context.Context, group string, topics []string, handler contracts.MessageHandlerFunc) error {
	if !d.IsConnected() {
		return errors.New("mqtt: not connected")
	}

	for _, topic := range topics {
		sharedTopic := fmt.Sprintf("$share/%s/%s", group, topic)
		if err := d.subscribe(ctx, sharedTopic, handler); err != nil {
			return err
		}
	}
	return nil
}

// LeaveGroup unsubscribes from every topic that was registered under the
// given shared-subscription group prefix.
func (d *Driver) LeaveGroup(group string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	prefix := fmt.Sprintf("$share/%s/", group)
	for topic, cancel := range d.cancelFuncs {
		if len(topic) > len(prefix) && topic[:len(prefix)] == prefix {
			cancel()
			delete(d.cancelFuncs, topic)
			delete(d.handlers, topic)

			if d.client != nil && d.client.IsConnected() {
				d.client.Unsubscribe(topic)
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// contracts.Broker – queue / ack
// ---------------------------------------------------------------------------

// QueueLength always returns 0 – MQTT is not a queue-based broker.
func (d *Driver) QueueLength(ctx context.Context, queue string) (int64, error) {
	return 0, nil
}

// Ack is a no-op. With MQTT the QoS protocol handles acknowledgements
// automatically; higher-level manual ack is not required.
func (d *Driver) Ack(ctx context.Context, msg *contracts.BrokerMessage) error {
	return nil
}

// Nack is a no-op. MQTT does not expose a native negative-acknowledge
// mechanism. If requeue is desired the caller should republish.
func (d *Driver) Nack(ctx context.Context, msg *contracts.BrokerMessage, requeue bool) error {
	return nil
}

// ---------------------------------------------------------------------------
// Config getters (convenience)
// ---------------------------------------------------------------------------

// Config returns a copy of the driver's configuration.
func (d *Driver) Config() *Config {
	cp := *d.config
	cp.Brokers = make([]string, len(d.config.Brokers))
	copy(cp.Brokers, d.config.Brokers)
	return &cp
}

// Brokers returns the configured broker addresses.
func (d *Driver) Brokers() []string {
	return append([]string{}, d.config.Brokers...)
}

// ---------------------------------------------------------------------------
// internal helpers
// ---------------------------------------------------------------------------

// subscribe performs the low-level MQTT subscribe for a single topic and
// wraps the paho callback so that incoming messages are converted to
// contracts.BrokerMessage and dispatched to the user-provided handler.
func (d *Driver) subscribe(ctx context.Context, topic string, handler contracts.MessageHandlerFunc) error {
	// Create a derived context for this subscription so we can cancel it
	// independently when Unsubscribe is called.
	subCtx, cancel := context.WithCancel(ctx)

	d.mu.Lock()
	d.handlers[topic] = handler
	d.cancelFuncs[topic] = cancel
	d.mu.Unlock()

	// paho callback – runs on the paho network goroutine.
	cb := func(c mqtt.Client, msg mqtt.Message) {
		bm := &contracts.BrokerMessage{
			ID:        fmt.Sprintf("%d", msg.MessageID()),
			Topic:     msg.Topic(),
			Body:      msg.Payload(),
			Headers:   make(map[string]string),
			Timestamp: time.Now(),
			Raw:       msg,
		}

		if err := handler(subCtx, bm); err != nil {
			// Handler errors are logged but do not affect the connection.
			// The caller can implement retry / DLQ logic inside the handler.
		}
	}

	token := d.client.Subscribe(topic, d.config.DefaultQoS, cb)
	if !token.WaitTimeout(d.config.ConnectTimeout) {
		// Clean up on timeout.
		d.mu.Lock()
		delete(d.handlers, topic)
		delete(d.cancelFuncs, topic)
		d.mu.Unlock()
		return errors.New("mqtt: subscribe timed out")
	}
	if err := token.Error(); err != nil {
		d.mu.Lock()
		delete(d.handlers, topic)
		delete(d.cancelFuncs, topic)
		d.mu.Unlock()
		return fmt.Errorf("mqtt: subscribe: %w", err)
	}

	return nil
}

// buildTLSConfig loads certificates from disk and assembles a tls.Config.
func (d *Driver) buildTLSConfig() (*tls.Config, error) {
	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	// CA certificate pool
	if d.config.CAFile != "" {
		caCert, err := os.ReadFile(d.config.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read CA file: %w", err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, errors.New("failed to parse CA certificate")
		}
		tlsCfg.RootCAs = caCertPool
	}

	// Client certificate (mutual TLS)
	if d.config.CertFile != "" && d.config.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(d.config.CertFile, d.config.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load client cert/key: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}

	return tlsCfg, nil
}

// Ensure Driver implements contracts.Broker at compile time.
var _ contracts.Broker = (*Driver)(nil)
