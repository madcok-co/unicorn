package kafka

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	contracts "github.com/madcok-co/unicorn/core/pkg/contracts"
)

// ============ Mock Implementations ============

// mockProducer implements the Producer interface for testing.
type mockProducer struct {
	produceFn      func(ctx context.Context, topic string, msg *contracts.BrokerMessage) error
	produceBatchFn func(ctx context.Context, topic string, msgs []*contracts.BrokerMessage) error
	closeFn        func() error

	mu                sync.Mutex
	produceCalls      []produceCall
	produceBatchCalls []produceBatchCall
	closeCalls        int
}

type produceCall struct {
	topic string
	msg   *contracts.BrokerMessage
}

type produceBatchCall struct {
	topic string
	msgs  []*contracts.BrokerMessage
}

func (m *mockProducer) Produce(ctx context.Context, topic string, msg *contracts.BrokerMessage) error {
	m.mu.Lock()
	m.produceCalls = append(m.produceCalls, produceCall{topic: topic, msg: msg})
	m.mu.Unlock()
	if m.produceFn != nil {
		return m.produceFn(ctx, topic, msg)
	}
	return nil
}

func (m *mockProducer) ProduceBatch(ctx context.Context, topic string, msgs []*contracts.BrokerMessage) error {
	m.mu.Lock()
	m.produceBatchCalls = append(m.produceBatchCalls, produceBatchCall{topic: topic, msgs: msgs})
	m.mu.Unlock()
	if m.produceBatchFn != nil {
		return m.produceBatchFn(ctx, topic, msgs)
	}
	return nil
}

func (m *mockProducer) Close() error {
	m.mu.Lock()
	m.closeCalls++
	m.mu.Unlock()
	if m.closeFn != nil {
		return m.closeFn()
	}
	return nil
}

func (m *mockProducer) produceCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.produceCalls)
}

// mockConsumer implements the Consumer interface for testing.
type mockConsumer struct {
	subscribeFn func(topics []string) error
	pollFn      func(ctx context.Context) (*contracts.BrokerMessage, error)
	commitFn    func(msg *contracts.BrokerMessage) error
	closeFn     func() error

	mu             sync.Mutex
	subscribeCalls [][]string
	pollCalls      int32
	commitCalls    []*contracts.BrokerMessage
	closeCalls     int
}

func (m *mockConsumer) Subscribe(topics []string) error {
	m.mu.Lock()
	m.subscribeCalls = append(m.subscribeCalls, topics)
	m.mu.Unlock()
	if m.subscribeFn != nil {
		return m.subscribeFn(topics)
	}
	return nil
}

func (m *mockConsumer) Poll(ctx context.Context) (*contracts.BrokerMessage, error) {
	atomic.AddInt32(&m.pollCalls, 1)
	if m.pollFn != nil {
		return m.pollFn(ctx)
	}
	return nil, nil
}

func (m *mockConsumer) Commit(msg *contracts.BrokerMessage) error {
	m.mu.Lock()
	m.commitCalls = append(m.commitCalls, msg)
	m.mu.Unlock()
	if m.commitFn != nil {
		return m.commitFn(msg)
	}
	return nil
}

func (m *mockConsumer) Close() error {
	m.mu.Lock()
	m.closeCalls++
	m.mu.Unlock()
	if m.closeFn != nil {
		return m.closeFn()
	}
	return nil
}

func (m *mockConsumer) subscribeCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.subscribeCalls)
}

func (m *mockConsumer) commitCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.commitCalls)
}

// ============ Tests ============

func TestNew(t *testing.T) {
	t.Run("nil config applies defaults", func(t *testing.T) {
		b := New(nil)

		if b.config == nil {
			t.Fatal("expected non-nil config")
		}
		if len(b.config.Brokers) != 1 || b.config.Brokers[0] != "localhost:9092" {
			t.Errorf("expected default broker [localhost:9092], got %v", b.config.Brokers)
		}
		if b.config.ConsumerGroup != "unicorn-consumer" {
			t.Errorf("expected default consumer group 'unicorn-consumer', got %q", b.config.ConsumerGroup)
		}
		if !b.config.AutoAck {
			t.Error("expected AutoAck to default to true")
		}
		if b.config.OffsetReset != "earliest" {
			t.Errorf("expected OffsetReset 'earliest', got %q", b.config.OffsetReset)
		}
		if b.connected {
			t.Error("expected not connected initially")
		}
		if b.producer != nil {
			t.Error("expected nil producer initially")
		}
		if b.consumer != nil {
			t.Error("expected nil consumer initially")
		}
	})

	t.Run("non-nil config is used as-is", func(t *testing.T) {
		cfg := &contracts.KafkaBrokerConfig{
			BrokerConfig: contracts.BrokerConfig{
				Brokers:       []string{"kafka:9092", "kafka:9093"},
				ConsumerGroup: "my-group",
				AutoAck:       false,
			},
			OffsetReset: "latest",
		}
		b := New(cfg)

		if b.config != cfg {
			t.Fatal("expected config to be the exact pointer passed in")
		}
		if len(b.config.Brokers) != 2 {
			t.Errorf("expected 2 brokers, got %d", len(b.config.Brokers))
		}
		if b.config.AutoAck {
			t.Error("expected AutoAck to be false")
		}
	})
}

func TestSetProducer(t *testing.T) {
	b := New(nil)
	mp := &mockProducer{}

	b.SetProducer(mp)

	if b.producer != mp {
		t.Fatal("expected producer to be set")
	}
}

func TestSetConsumer(t *testing.T) {
	b := New(nil)
	mc := &mockConsumer{}

	b.SetConsumer(mc)

	if b.consumer != mc {
		t.Fatal("expected consumer to be set")
	}
}

func TestName(t *testing.T) {
	b := New(nil)
	if name := b.Name(); name != "kafka" {
		t.Errorf("expected Name() to return 'kafka', got %q", name)
	}
}

func TestConnect(t *testing.T) {
	t.Run("sets connected to true", func(t *testing.T) {
		b := New(nil)
		if b.IsConnected() {
			t.Fatal("expected not connected initially")
		}

		err := b.Connect(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !b.IsConnected() {
			t.Error("expected connected after Connect")
		}
	})
}

func TestDisconnect(t *testing.T) {
	t.Run("closes producer and consumer and sets connected false", func(t *testing.T) {
		b := New(nil)
		mp := &mockProducer{}
		mc := &mockConsumer{}
		b.SetProducer(mp)
		b.SetConsumer(mc)
		b.Connect(context.Background()) // set connected = true

		err := b.Disconnect(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		mp.mu.Lock()
		prodCloseCalls := mp.closeCalls
		mp.mu.Unlock()

		mc.mu.Lock()
		consCloseCalls := mc.closeCalls
		mc.mu.Unlock()

		if prodCloseCalls != 1 {
			t.Errorf("expected producer Close to be called once, got %d", prodCloseCalls)
		}
		if consCloseCalls != 1 {
			t.Errorf("expected consumer Close to be called once, got %d", consCloseCalls)
		}
		if b.IsConnected() {
			t.Error("expected not connected after Disconnect")
		}
	})

	t.Run("does not panic when producer and consumer are nil", func(t *testing.T) {
		b := New(nil)
		b.Connect(context.Background())

		err := b.Disconnect(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if b.IsConnected() {
			t.Error("expected not connected after Disconnect")
		}
	})

	t.Run("best-effort close ignores errors", func(t *testing.T) {
		b := New(nil)
		mp := &mockProducer{
			closeFn: func() error {
				return errors.New("close failed")
			},
		}
		mc := &mockConsumer{
			closeFn: func() error {
				return errors.New("close failed")
			},
		}
		b.SetProducer(mp)
		b.SetConsumer(mc)
		b.Connect(context.Background())

		err := b.Disconnect(context.Background())
		if err != nil {
			t.Fatalf("expected Disconnect to swallow close errors, got: %v", err)
		}
		if b.IsConnected() {
			t.Error("expected not connected after Disconnect")
		}
	})
}

func TestPing(t *testing.T) {
	t.Run("returns nil when connected", func(t *testing.T) {
		b := New(nil)
		b.Connect(context.Background())

		err := b.Ping(context.Background())
		if err != nil {
			t.Errorf("expected nil error when connected, got: %v", err)
		}
	})

	t.Run("returns error when not connected", func(t *testing.T) {
		b := New(nil)

		err := b.Ping(context.Background())
		if err == nil {
			t.Error("expected error when not connected")
		}
	})
}

func TestIsConnected(t *testing.T) {
	b := New(nil)

	if b.IsConnected() {
		t.Error("expected false before Connect")
	}

	b.Connect(context.Background())
	if !b.IsConnected() {
		t.Error("expected true after Connect")
	}

	b.Disconnect(context.Background())
	if b.IsConnected() {
		t.Error("expected false after Disconnect")
	}
}

func TestPublish(t *testing.T) {
	t.Run("returns error when producer is nil", func(t *testing.T) {
		b := New(nil)
		msg := contracts.NewBrokerMessage("test", []byte("body"))

		err := b.Publish(context.Background(), "test", msg)
		if err == nil {
			t.Error("expected error when producer not initialized")
		}
	})

	t.Run("calls producer.Produce", func(t *testing.T) {
		b := New(nil)
		mp := &mockProducer{}
		b.SetProducer(mp)

		msg := contracts.NewBrokerMessage("test.topic", []byte("hello"))
		err := b.Publish(context.Background(), "test.topic", msg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if count := mp.produceCallCount(); count != 1 {
			t.Fatalf("expected 1 produce call, got %d", count)
		}

		mp.mu.Lock()
		call := mp.produceCalls[0]
		mp.mu.Unlock()

		if call.topic != "test.topic" {
			t.Errorf("expected topic 'test.topic', got %q", call.topic)
		}
		if call.msg != msg {
			t.Error("expected same message pointer")
		}
	})

	t.Run("returns error from producer.Produce", func(t *testing.T) {
		b := New(nil)
		produceErr := errors.New("kafka produce error")
		mp := &mockProducer{
			produceFn: func(ctx context.Context, topic string, msg *contracts.BrokerMessage) error {
				return produceErr
			},
		}
		b.SetProducer(mp)

		msg := contracts.NewBrokerMessage("test", []byte("body"))
		err := b.Publish(context.Background(), "test", msg)
		if err != produceErr {
			t.Errorf("expected error %v, got %v", produceErr, err)
		}
	})
}

func TestPublishBatch(t *testing.T) {
	t.Run("returns error when producer is nil", func(t *testing.T) {
		b := New(nil)
		msgs := []*contracts.BrokerMessage{
			contracts.NewBrokerMessage("test", []byte("a")),
		}

		err := b.PublishBatch(context.Background(), "test", msgs)
		if err == nil {
			t.Error("expected error when producer not initialized")
		}
	})

	t.Run("calls producer.ProduceBatch", func(t *testing.T) {
		b := New(nil)
		mp := &mockProducer{}
		b.SetProducer(mp)

		msgs := []*contracts.BrokerMessage{
			contracts.NewBrokerMessage("batch", []byte("1")),
			contracts.NewBrokerMessage("batch", []byte("2")),
			contracts.NewBrokerMessage("batch", []byte("3")),
		}
		err := b.PublishBatch(context.Background(), "batch", msgs)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		mp.mu.Lock()
		defer mp.mu.Unlock()

		if len(mp.produceBatchCalls) != 1 {
			t.Fatalf("expected 1 produceBatch call, got %d", len(mp.produceBatchCalls))
		}
		if mp.produceBatchCalls[0].topic != "batch" {
			t.Errorf("expected topic 'batch', got %q", mp.produceBatchCalls[0].topic)
		}
		if len(mp.produceBatchCalls[0].msgs) != 3 {
			t.Errorf("expected 3 messages, got %d", len(mp.produceBatchCalls[0].msgs))
		}
	})

	t.Run("returns error from producer.ProduceBatch", func(t *testing.T) {
		b := New(nil)
		batchErr := errors.New("batch failed")
		mp := &mockProducer{
			produceBatchFn: func(ctx context.Context, topic string, msgs []*contracts.BrokerMessage) error {
				return batchErr
			},
		}
		b.SetProducer(mp)

		msgs := []*contracts.BrokerMessage{
			contracts.NewBrokerMessage("test", []byte("body")),
		}
		err := b.PublishBatch(context.Background(), "test", msgs)
		if err != batchErr {
			t.Errorf("expected error %v, got %v", batchErr, err)
		}
	})
}

func TestSubscribe(t *testing.T) {
	t.Run("delegates to SubscribeMultiple with single topic", func(t *testing.T) {
		b := New(nil)
		mc := &mockConsumer{}
		b.SetConsumer(mc)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		handler := func(ctx context.Context, msg *contracts.BrokerMessage) error {
			return nil
		}

		err := b.Subscribe(ctx, "single-topic", handler)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if count := mc.subscribeCallCount(); count != 1 {
			t.Fatalf("expected 1 subscribe call, got %d", count)
		}
		mc.mu.Lock()
		topics := mc.subscribeCalls[0]
		mc.mu.Unlock()
		if len(topics) != 1 || topics[0] != "single-topic" {
			t.Errorf("expected [single-topic], got %v", topics)
		}
	})

	t.Run("returns error when consumer is nil", func(t *testing.T) {
		b := New(nil)
		handler := func(ctx context.Context, msg *contracts.BrokerMessage) error { return nil }
		err := b.Subscribe(context.Background(), "topic", handler)
		if err == nil {
			t.Error("expected error when consumer not initialized")
		}
	})
}

func TestSubscribeMultiple(t *testing.T) {
	t.Run("returns error when consumer is nil", func(t *testing.T) {
		b := New(nil)
		handler := func(ctx context.Context, msg *contracts.BrokerMessage) error { return nil }
		err := b.SubscribeMultiple(context.Background(), []string{"t1", "t2"}, handler)
		if err == nil {
			t.Error("expected error when consumer not initialized")
		}
	})

	t.Run("returns error from consumer.Subscribe", func(t *testing.T) {
		b := New(nil)
		subErr := errors.New("subscribe error")
		mc := &mockConsumer{
			subscribeFn: func(topics []string) error {
				return subErr
			},
		}
		b.SetConsumer(mc)

		handler := func(ctx context.Context, msg *contracts.BrokerMessage) error { return nil }
		err := b.SubscribeMultiple(context.Background(), []string{"t1"}, handler)
		if err != subErr {
			t.Errorf("expected error %v, got %v", subErr, err)
		}
	})

	t.Run("starts consumeLoop on success", func(t *testing.T) {
		b := New(nil)
		// Track that the consumeLoop ran by having pollFn return a message
		// that the handler receives.
		received := make(chan *contracts.BrokerMessage, 1)
		testMsg := contracts.NewBrokerMessage("topic-a", []byte("hello"))

		var pollCalled int32
		mc := &mockConsumer{
			pollFn: func(ctx context.Context) (*contracts.BrokerMessage, error) {
				if atomic.AddInt32(&pollCalled, 1) == 1 {
					return testMsg, nil
				}
				return nil, nil
			},
		}
		b.SetConsumer(mc)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := b.SubscribeMultiple(ctx, []string{"topic-a", "topic-b"}, func(ctx context.Context, msg *contracts.BrokerMessage) error {
			received <- msg
			return nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		select {
		case m := <-received:
			if m.Body == nil || string(m.Body) != "hello" {
				t.Errorf("unexpected message body: %s", string(m.Body))
			}
		case <-time.After(500 * time.Millisecond):
			t.Error("timed out waiting for message to be consumed")
		}
	})
}

func TestUnsubscribe(t *testing.T) {
	t.Run("always returns nil", func(t *testing.T) {
		b := New(nil)
		if err := b.Unsubscribe("any-topic"); err != nil {
			t.Errorf("expected nil error, got: %v", err)
		}
	})
}

func TestConsumeGroup(t *testing.T) {
	t.Run("returns error when consumer is nil", func(t *testing.T) {
		b := New(nil)
		handler := func(ctx context.Context, msg *contracts.BrokerMessage) error { return nil }
		err := b.ConsumeGroup(context.Background(), "g1", []string{"t1"}, handler)
		if err == nil {
			t.Error("expected error when consumer not initialized")
		}
	})

	t.Run("returns error from consumer.Subscribe", func(t *testing.T) {
		b := New(nil)
		subErr := errors.New("subscribe error")
		mc := &mockConsumer{
			subscribeFn: func(topics []string) error {
				return subErr
			},
		}
		b.SetConsumer(mc)

		handler := func(ctx context.Context, msg *contracts.BrokerMessage) error { return nil }
		err := b.ConsumeGroup(context.Background(), "group-1", []string{"t1"}, handler)
		if err != subErr {
			t.Errorf("expected error %v, got %v", subErr, err)
		}
	})

	t.Run("starts consumeLoop with subscribed topics", func(t *testing.T) {
		b := New(nil)
		received := make(chan *contracts.BrokerMessage, 1)
		testMsg := contracts.NewBrokerMessage("group-topic", []byte("group-data"))

		var pollCalled int32
		mc := &mockConsumer{
			pollFn: func(ctx context.Context) (*contracts.BrokerMessage, error) {
				if atomic.AddInt32(&pollCalled, 1) == 1 {
					return testMsg, nil
				}
				return nil, nil
			},
		}
		b.SetConsumer(mc)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := b.ConsumeGroup(ctx, "my-group", []string{"group-topic"}, func(ctx context.Context, msg *contracts.BrokerMessage) error {
			received <- msg
			return nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		select {
		case m := <-received:
			if m.Topic != "group-topic" {
				t.Errorf("expected topic 'group-topic', got %q", m.Topic)
			}
		case <-time.After(500 * time.Millisecond):
			t.Error("timed out waiting for consumed message")
		}

		// Verify Subscribe was called with the correct topics
		mc.mu.Lock()
		if len(mc.subscribeCalls) != 1 {
			t.Errorf("expected 1 subscribe call, got %d", len(mc.subscribeCalls))
		} else {
			if len(mc.subscribeCalls[0]) != 1 || mc.subscribeCalls[0][0] != "group-topic" {
				t.Errorf("expected [group-topic], got %v", mc.subscribeCalls[0])
			}
		}
		mc.mu.Unlock()
	})
}

func TestLeaveGroup(t *testing.T) {
	t.Run("calls consumer.Close when consumer is set", func(t *testing.T) {
		b := New(nil)
		mc := &mockConsumer{}
		b.SetConsumer(mc)

		err := b.LeaveGroup("some-group")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		mc.mu.Lock()
		closeCalls := mc.closeCalls
		mc.mu.Unlock()

		if closeCalls != 1 {
			t.Errorf("expected 1 close call, got %d", closeCalls)
		}
	})

	t.Run("returns nil when consumer is nil", func(t *testing.T) {
		b := New(nil)
		err := b.LeaveGroup("some-group")
		if err != nil {
			t.Errorf("expected nil error, got: %v", err)
		}
	})

	t.Run("returns error from consumer.Close", func(t *testing.T) {
		b := New(nil)
		closeErr := errors.New("close error")
		mc := &mockConsumer{
			closeFn: func() error {
				return closeErr
			},
		}
		b.SetConsumer(mc)

		err := b.LeaveGroup("some-group")
		if err != closeErr {
			t.Errorf("expected error %v, got %v", closeErr, err)
		}
	})
}

func TestConsumeLoop(t *testing.T) {
	t.Run("exits on context cancellation", func(t *testing.T) {
		b := New(nil)
		mc := &mockConsumer{
			pollFn: func(ctx context.Context) (*contracts.BrokerMessage, error) {
				// Keep returning nil messages so the loop spins
				return nil, nil
			},
		}
		b.SetConsumer(mc)

		ctx, cancel := context.WithCancel(context.Background())

		done := make(chan struct{})
		go func() {
			b.consumeLoop(ctx, func(ctx context.Context, msg *contracts.BrokerMessage) error {
				return nil
			})
			close(done)
		}()

		// Give the loop a moment to start polling
		time.Sleep(20 * time.Millisecond)

		// Cancel the context
		cancel()

		select {
		case <-done:
			// consumeLoop exited
		case <-time.After(500 * time.Millisecond):
			t.Fatal("consumeLoop did not exit after context cancellation")
		}
	})

	t.Run("skips nil messages", func(t *testing.T) {
		b := New(nil)
		handlerCalled := make(chan struct{}, 1)
		var pollCount int32
		mc := &mockConsumer{
			pollFn: func(ctx context.Context) (*contracts.BrokerMessage, error) {
				c := atomic.AddInt32(&pollCount, 1)
				// Return nil for first few calls, then a valid message
				if c <= 3 {
					return nil, nil
				}
				return contracts.NewBrokerMessage("test", []byte("msg")), nil
			},
		}
		b.SetConsumer(mc)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		done := make(chan struct{})
		go func() {
			b.consumeLoop(ctx, func(ctx context.Context, msg *contracts.BrokerMessage) error {
				select {
				case handlerCalled <- struct{}{}:
				default:
				}
				cancel() // stop after first real message
				return nil
			})
			close(done)
		}()

		select {
		case <-handlerCalled:
			// handler was eventually called
		case <-time.After(500 * time.Millisecond):
			t.Fatal("handler was never called - nil messages may have crashed the loop")
		}
	})

	t.Run("auto-ack commits message after successful handler", func(t *testing.T) {
		// Default config has AutoAck: true
		b := New(nil)
		testMsg := contracts.NewBrokerMessage("auto-ack-topic", []byte("payload"))

		var pollCount int32
		handlerCalled := make(chan struct{})
		mc := &mockConsumer{
			pollFn: func(ctx context.Context) (*contracts.BrokerMessage, error) {
				if atomic.AddInt32(&pollCount, 1) == 1 {
					return testMsg, nil
				}
				return nil, nil
			},
		}
		b.SetConsumer(mc)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		done := make(chan struct{})
		go func() {
			b.consumeLoop(ctx, func(ctx context.Context, msg *contracts.BrokerMessage) error {
				close(handlerCalled)
				return nil
			})
			close(done)
		}()

		select {
		case <-handlerCalled:
			// handler was called, now give a moment for commit
			time.Sleep(30 * time.Millisecond)
			cancel()
		case <-time.After(500 * time.Millisecond):
			t.Fatal("handler was never called")
		}

		<-done

		mc.mu.Lock()
		commitCalls := len(mc.commitCalls)
		mc.mu.Unlock()

		if commitCalls != 1 {
			t.Errorf("expected 1 commit call (auto-ack), got %d", commitCalls)
		}
	})

	t.Run("does not commit when AutoAck is disabled", func(t *testing.T) {
		cfg := &contracts.KafkaBrokerConfig{
			BrokerConfig: contracts.BrokerConfig{
				AutoAck: false,
			},
		}
		b := New(cfg)
		testMsg := contracts.NewBrokerMessage("no-auto-ack", []byte("data"))

		var pollCount int32
		handlerCalled := make(chan struct{})
		mc := &mockConsumer{
			pollFn: func(ctx context.Context) (*contracts.BrokerMessage, error) {
				if atomic.AddInt32(&pollCount, 1) == 1 {
					return testMsg, nil
				}
				return nil, nil
			},
		}
		b.SetConsumer(mc)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		done := make(chan struct{})
		go func() {
			b.consumeLoop(ctx, func(ctx context.Context, msg *contracts.BrokerMessage) error {
				close(handlerCalled)
				return nil
			})
			close(done)
		}()

		select {
		case <-handlerCalled:
			time.Sleep(30 * time.Millisecond)
			cancel()
		case <-time.After(500 * time.Millisecond):
			t.Fatal("handler was never called")
		}

		<-done

		mc.mu.Lock()
		commitCalls := len(mc.commitCalls)
		mc.mu.Unlock()

		if commitCalls != 0 {
			t.Errorf("expected 0 commit calls (auto-ack disabled), got %d", commitCalls)
		}
	})

	t.Run("continues on handler error", func(t *testing.T) {
		b := New(nil)
		handlerCalls := int32(0)
		var pollCount int32
		mc := &mockConsumer{
			pollFn: func(ctx context.Context) (*contracts.BrokerMessage, error) {
				c := atomic.AddInt32(&pollCount, 1)
				if c <= 3 {
					return contracts.NewBrokerMessage("test", []byte(fmt.Sprintf("msg-%d", c))), nil
				}
				return nil, nil
			},
		}
		b.SetConsumer(mc)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		done := make(chan struct{})
		go func() {
			b.consumeLoop(ctx, func(ctx context.Context, msg *contracts.BrokerMessage) error {
				atomic.AddInt32(&handlerCalls, 1)
				// Signal to cancel after the last expected call
				if atomic.LoadInt32(&handlerCalls) >= 3 {
					cancel()
				}
				return errors.New("handler error")
			})
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
			t.Fatal("consumeLoop did not exit")
		}

		if n := atomic.LoadInt32(&handlerCalls); n < 3 {
			t.Errorf("expected at least 3 handler calls despite errors, got %d", n)
		}
	})

	t.Run("continues on poll error", func(t *testing.T) {
		b := New(nil)
		var pollCount int32
		handlerCalled := make(chan struct{})
		mc := &mockConsumer{
			pollFn: func(ctx context.Context) (*contracts.BrokerMessage, error) {
				c := atomic.AddInt32(&pollCount, 1)
				if c <= 2 {
					return nil, errors.New("poll error")
				}
				return contracts.NewBrokerMessage("test", []byte("ok")), nil
			},
		}
		b.SetConsumer(mc)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		done := make(chan struct{})
		go func() {
			b.consumeLoop(ctx, func(ctx context.Context, msg *contracts.BrokerMessage) error {
				close(handlerCalled)
				cancel()
				return nil
			})
			close(done)
		}()

		select {
		case <-handlerCalled:
		case <-time.After(500 * time.Millisecond):
			t.Fatal("handler was never called after poll errors")
		}
	})
}

func TestQueueLength(t *testing.T) {
	t.Run("always returns 0, nil", func(t *testing.T) {
		b := New(nil)
		length, err := b.QueueLength(context.Background(), "any-queue")
		if err != nil {
			t.Errorf("expected nil error, got: %v", err)
		}
		if length != 0 {
			t.Errorf("expected 0, got %d", length)
		}
	})
}

func TestAck(t *testing.T) {
	t.Run("returns error when consumer is nil", func(t *testing.T) {
		b := New(nil)
		msg := contracts.NewBrokerMessage("test", []byte("body"))
		err := b.Ack(context.Background(), msg)
		if err == nil {
			t.Error("expected error when consumer not initialized")
		}
	})

	t.Run("calls consumer.Commit", func(t *testing.T) {
		b := New(nil)
		mc := &mockConsumer{}
		b.SetConsumer(mc)

		msg := contracts.NewBrokerMessage("ack-topic", []byte("ack-body"))
		err := b.Ack(context.Background(), msg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		mc.mu.Lock()
		defer mc.mu.Unlock()

		if len(mc.commitCalls) != 1 {
			t.Fatalf("expected 1 commit call, got %d", len(mc.commitCalls))
		}
		if mc.commitCalls[0] != msg {
			t.Error("expected commit called with same message")
		}
	})

	t.Run("returns error from consumer.Commit", func(t *testing.T) {
		b := New(nil)
		commitErr := errors.New("commit failed")
		mc := &mockConsumer{
			commitFn: func(msg *contracts.BrokerMessage) error {
				return commitErr
			},
		}
		b.SetConsumer(mc)

		msg := contracts.NewBrokerMessage("test", []byte("body"))
		err := b.Ack(context.Background(), msg)
		if err != commitErr {
			t.Errorf("expected error %v, got %v", commitErr, err)
		}
	})
}

func TestNack(t *testing.T) {
	t.Run("requeue=false returns nil", func(t *testing.T) {
		b := New(nil)
		msg := contracts.NewBrokerMessage("nack-topic", []byte("data"))

		err := b.Nack(context.Background(), msg, false)
		if err != nil {
			t.Errorf("expected nil error when requeue=false, got: %v", err)
		}
	})

	t.Run("requeue=true republishes via Publish", func(t *testing.T) {
		b := New(nil)
		mp := &mockProducer{}
		b.SetProducer(mp)

		msg := contracts.NewBrokerMessage("nack-topic", []byte("nack-data"))

		err := b.Nack(context.Background(), msg, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if count := mp.produceCallCount(); count != 1 {
			t.Fatalf("expected 1 produce call for requeue, got %d", count)
		}

		mp.mu.Lock()
		call := mp.produceCalls[0]
		mp.mu.Unlock()

		if call.topic != "nack-topic" {
			t.Errorf("expected topic 'nack-topic', got %q", call.topic)
		}
		if call.msg != msg {
			t.Error("expected same message pointer to be republished")
		}
	})

	t.Run("requeue=true returns error when producer is nil", func(t *testing.T) {
		b := New(nil)
		msg := contracts.NewBrokerMessage("test", []byte("body"))

		err := b.Nack(context.Background(), msg, true)
		if err == nil {
			t.Error("expected error when producer is nil and requeue=true")
		}
	})

	t.Run("requeue=true returns error from Publish", func(t *testing.T) {
		b := New(nil)
		pubErr := errors.New("publish error")
		mp := &mockProducer{
			produceFn: func(ctx context.Context, topic string, msg *contracts.BrokerMessage) error {
				return pubErr
			},
		}
		b.SetProducer(mp)

		msg := contracts.NewBrokerMessage("test", []byte("body"))
		err := b.Nack(context.Background(), msg, true)
		if err != pubErr {
			t.Errorf("expected error %v, got %v", pubErr, err)
		}
	})
}

func TestConfig(t *testing.T) {
	t.Run("nil config returns default config pointer", func(t *testing.T) {
		b := New(nil)
		cfg := b.Config()
		if cfg == nil {
			t.Fatal("expected non-nil config")
		}
		if !cfg.AutoAck {
			t.Error("expected default AutoAck=true")
		}
	})

	t.Run("custom config returns same pointer", func(t *testing.T) {
		custom := &contracts.KafkaBrokerConfig{
			BrokerConfig: contracts.BrokerConfig{
				Brokers:       []string{"custom:9092"},
				ConsumerGroup: "custom-group",
			},
			OffsetReset: "latest",
		}
		b := New(custom)
		cfg := b.Config()
		if cfg != custom {
			t.Error("expected exact same config pointer")
		}
	})
}

func TestConcurrentAccess(t *testing.T) {
	t.Run("IsConnected and Connect do not race", func(t *testing.T) {
		b := New(nil)
		var wg sync.WaitGroup
		iterations := 100

		// Writer goroutines
		for i := 0; i < iterations; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				b.Connect(context.Background())
			}()
		}

		// Reader goroutines
		for i := 0; i < iterations; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = b.IsConnected()
			}()
		}

		// Ping goroutines (read lock)
		for i := 0; i < iterations; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = b.Ping(context.Background())
			}()
		}

		wg.Wait()
	})

	t.Run("SetProducer and SetConsumer do not race with Publish/Ack", func(t *testing.T) {
		b := New(nil)
		mp := &mockProducer{}
		mc := &mockConsumer{}

		var wg sync.WaitGroup
		iterations := 50

		// Setters
		for i := 0; i < iterations; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				b.SetProducer(mp)
				b.SetConsumer(mc)
			}()
		}

		// Publish
		for i := 0; i < iterations; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				msg := contracts.NewBrokerMessage("test", []byte("data"))
				_ = b.Publish(context.Background(), "test", msg)
			}()
		}

		// Ack
		for i := 0; i < iterations; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				msg := contracts.NewBrokerMessage("test", []byte("data"))
				_ = b.Ack(context.Background(), msg)
			}()
		}

		wg.Wait()
	})
}
