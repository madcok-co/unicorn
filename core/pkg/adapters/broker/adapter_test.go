package broker

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	ucontext "github.com/madcok-co/unicorn/core/pkg/context"

	"github.com/madcok-co/unicorn/core/pkg/contracts"
	"github.com/madcok-co/unicorn/core/pkg/handler"
)

// ============ Mock Broker ============

type mockBroker struct {
	mu            sync.Mutex
	connected     bool
	messages      []*contracts.BrokerMessage
	consumeFunc   contracts.MessageHandlerFunc
	consumeTopics []string
	publishErr    error
}

func (b *mockBroker) Name() string { return "mock" }
func (b *mockBroker) Connect(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.connected = true
	return nil
}
func (b *mockBroker) Disconnect(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.connected = false
	return nil
}
func (b *mockBroker) Ping(ctx context.Context) error { return nil }
func (b *mockBroker) IsConnected() bool              { b.mu.Lock(); defer b.mu.Unlock(); return b.connected }

func (b *mockBroker) Publish(ctx context.Context, topic string, msg *contracts.BrokerMessage) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.messages = append(b.messages, msg)
	return b.publishErr
}

func (b *mockBroker) PublishBatch(ctx context.Context, topic string, msgs []*contracts.BrokerMessage) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.messages = append(b.messages, msgs...)
	return nil
}

func (b *mockBroker) Subscribe(ctx context.Context, topic string, handlerFunc contracts.MessageHandlerFunc) error {
	return nil
}

func (b *mockBroker) SubscribeMultiple(ctx context.Context, topics []string, handlerFunc contracts.MessageHandlerFunc) error {
	return nil
}

func (b *mockBroker) Unsubscribe(topic string) error { return nil }

func (b *mockBroker) ConsumeGroup(ctx context.Context, group string, topics []string, handlerFunc contracts.MessageHandlerFunc) error {
	b.consumeFunc = handlerFunc
	b.consumeTopics = topics
	<-ctx.Done()
	return nil
}

func (b *mockBroker) LeaveGroup(group string) error                                { return nil }
func (b *mockBroker) QueueLength(ctx context.Context, queue string) (int64, error) { return 0, nil }
func (b *mockBroker) Ack(ctx context.Context, msg *contracts.BrokerMessage) error  { return nil }
func (b *mockBroker) Nack(ctx context.Context, msg *contracts.BrokerMessage, requeue bool) error {
	return nil
}

func (b *mockBroker) Deliver(topic string, msg *contracts.BrokerMessage) error {
	if b.consumeFunc == nil {
		return fmt.Errorf("no consumer registered")
	}
	return b.consumeFunc(context.Background(), msg)
}

// ============ Tests ============

func TestNewAdapter(t *testing.T) {
	t.Run("creates with default config", func(t *testing.T) {
		reg := handler.NewRegistry()
		a := New(&mockBroker{}, reg, nil)

		if a == nil {
			t.Fatal("adapter should not be nil")
		}
		if a.config.GroupID != "unicorn-consumer" {
			t.Errorf("expected default group, got %s", a.config.GroupID)
		}
		if a.config.MaxRetries != 3 {
			t.Errorf("expected 3 retries, got %d", a.config.MaxRetries)
		}
		if a.config.DLQEnabled != true {
			t.Error("DLQ should be enabled by default")
		}
	})

	t.Run("creates with custom config", func(t *testing.T) {
		reg := handler.NewRegistry()
		cfg := &Config{
			GroupID:    "my-group",
			MaxRetries: 5,
			AutoAck:    false,
		}
		a := New(&mockBroker{}, reg, cfg)

		if a.config.GroupID != "my-group" {
			t.Errorf("expected custom group, got %s", a.config.GroupID)
		}
		if a.config.MaxRetries != 5 {
			t.Errorf("expected 5 retries, got %d", a.config.MaxRetries)
		}
		if a.config.AutoAck != false {
			t.Error("AutoAck should be false")
		}
	})
}

func TestCollectTopics(t *testing.T) {
	t.Run("collects single topic", func(t *testing.T) {
		reg := handler.NewRegistry()
		h := handler.New(func(ctx *ucontext.Context) error { return nil }).
			Named("orders-handler").
			Message("orders.created")
		reg.Register(h)

		a := New(&mockBroker{}, reg, nil)
		topics := a.collectTopics()

		if len(topics) != 1 {
			t.Errorf("expected 1 topic, got %d", len(topics))
		}
		if topics[0] != "orders.created" {
			t.Errorf("expected 'orders.created', got %s", topics[0])
		}
	})

	t.Run("collects multiple topics", func(t *testing.T) {
		reg := handler.NewRegistry()
		h1 := handler.New(func(ctx *ucontext.Context) error { return nil }).
			Named("h1").Message("orders.created")
		h2 := handler.New(func(ctx *ucontext.Context) error { return nil }).
			Named("h2").Message("users.updated")
		reg.Register(h1)
		reg.Register(h2)

		a := New(&mockBroker{}, reg, nil)
		topics := a.collectTopics()

		if len(topics) != 2 {
			t.Errorf("expected 2 topics, got %d", len(topics))
		}
	})

	t.Run("fan-out: multiple handlers on same topic", func(t *testing.T) {
		reg := handler.NewRegistry()
		h1 := handler.New(func(ctx *ucontext.Context) error { return nil }).
			Named("h1").Message("events")
		h2 := handler.New(func(ctx *ucontext.Context) error { return nil }).
			Named("h2").Message("events")
		reg.Register(h1)
		reg.Register(h2)

		a := New(&mockBroker{}, reg, nil)
		topics := a.collectTopics()

		if len(topics) != 1 {
			t.Errorf("expected 1 unique topic, got %d", len(topics))
		}
		if len(a.handlers["events"]) != 2 {
			t.Errorf("expected 2 handlers for topic, got %d", len(a.handlers["events"]))
		}
	})
}

func TestHandleMessage(t *testing.T) {
	t.Run("processes message with registered handler", func(t *testing.T) {
		reg := handler.NewRegistry()
		called := false
		h := handler.New(func(ctx *ucontext.Context) error {
			called = true
			return nil
		}).Named("test-handler").Message("test.topic")
		reg.Register(h)

		a := New(&mockBroker{}, reg, nil)
		a.collectTopics()

		msg := contracts.NewBrokerMessage("test.topic", []byte(`{}`))
		err := a.handleMessage(context.Background(), msg)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !called {
			t.Error("handler should have been called")
		}
	})

	t.Run("fan-out: calls all handlers for topic", func(t *testing.T) {
		reg := handler.NewRegistry()
		var mu sync.Mutex
		callCount := 0

		handlerFunc := func(ctx *ucontext.Context) error {
			mu.Lock()
			callCount++
			mu.Unlock()
			return nil
		}

		h1 := handler.New(handlerFunc).Named("h1").Message("events")
		h2 := handler.New(handlerFunc).Named("h2").Message("events")
		reg.Register(h1)
		reg.Register(h2)

		a := New(&mockBroker{}, reg, nil)
		a.collectTopics()

		msg := contracts.NewBrokerMessage("events", []byte(`{}`))
		a.handleMessage(context.Background(), msg)

		mu.Lock()
		defer mu.Unlock()
		if callCount != 2 {
			t.Errorf("expected 2 handler calls, got %d", callCount)
		}
	})

	t.Run("returns error for unknown topic", func(t *testing.T) {
		reg := handler.NewRegistry()
		a := New(&mockBroker{}, reg, nil)
		a.collectTopics()

		msg := contracts.NewBrokerMessage("unknown.topic", []byte(`{}`))
		err := a.handleMessage(context.Background(), msg)

		if err == nil {
			t.Error("expected error for unknown topic")
		}
	})
}

func TestHandleError_Retry(t *testing.T) {
	t.Run("retries on handler error up to MaxRetries", func(t *testing.T) {
		reg := handler.NewRegistry()
		h := handler.New(func(ctx *ucontext.Context) error {
			return fmt.Errorf("transient error")
		}).Named("failing").Message("test.topic")
		reg.Register(h)

		mock := &mockBroker{}
		cfg := &Config{
			MaxRetries:   2,
			RetryBackoff: 10 * time.Millisecond,
			DLQEnabled:   false,
			AutoAck:      true,
		}
		a := New(mock, reg, cfg)
		a.collectTopics()

		msg := contracts.NewBrokerMessage("test.topic", []byte(`{}`))

		// Simulate retry loop: keep processing until retries exhausted.
		// Each handleMessage call that fails triggers a Publish (retry)
		// and returns nil (no error from Publish). We manually re-invoke
		// handleMessage for each retry, just like a real broker re-delivers.
		for msg.RetryCount < cfg.MaxRetries {
			a.handleMessage(context.Background(), msg)
			// Wait for the backoff timer inside handleError
			time.Sleep(15 * time.Millisecond)
		}
		// Final call: RetryCount now equals MaxRetries, so handleError
		// will Nack instead of republishing.
		a.handleMessage(context.Background(), msg)

		// Each retry attempt (except the final Nack) triggers a Publish call.
		// With MaxRetries=2: attempt 0→retry1 (publish), attempt 1→retry2 (publish), attempt 2→Nack.
		// That's 2 Publish calls.
		mock.mu.Lock()
		count := len(mock.messages)
		mock.mu.Unlock()

		if count != 2 {
			t.Errorf("expected 2 republishes, got %d", count)
		}
	})
}

func TestHandleError_DLQ(t *testing.T) {
	t.Run("sends to DLQ after max retries", func(t *testing.T) {
		reg := handler.NewRegistry()
		h := handler.New(func(ctx *ucontext.Context) error {
			return fmt.Errorf("fatal error")
		}).Named("failing").Message("test.topic")
		reg.Register(h)

		mock := &mockBroker{}
		cfg := &Config{
			MaxRetries:   1,
			RetryBackoff: 10 * time.Millisecond,
			DLQEnabled:   true,
			DLQSuffix:    ".dlq",
			AutoAck:      true,
		}
		a := New(mock, reg, cfg)
		a.collectTopics()

		msg := contracts.NewBrokerMessage("test.topic", []byte(`{}`))

		// Simulate retry loop: first call fails → retry once → then DLQ.
		for msg.RetryCount < cfg.MaxRetries {
			a.handleMessage(context.Background(), msg)
			time.Sleep(15 * time.Millisecond)
		}
		// Final call: RetryCount now equals MaxRetries, so handleError
		// will send to DLQ.
		a.handleMessage(context.Background(), msg)

		// Check that DLQ message was published
		mock.mu.Lock()
		defer mock.mu.Unlock()

		found := false
		for _, m := range mock.messages {
			if m.GetHeader("x-original-topic") == "test.topic" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected DLQ message, got %d messages", len(mock.messages))
		}
	})
}

func TestStartStop(t *testing.T) {
	t.Run("start fails when already running", func(t *testing.T) {
		reg := handler.NewRegistry()
		h := handler.New(func(ctx *ucontext.Context) error { return nil }).
			Named("h").Message("test.topic")
		reg.Register(h)

		mock := &mockBroker{}
		a := New(mock, reg, nil)

		ctx, cancel := context.WithCancel(context.Background())

		// Start in goroutine
		done := make(chan error, 1)
		go func() {
			done <- a.Start(ctx)
		}()

		// Try starting again immediately
		time.Sleep(10 * time.Millisecond)
		err := a.Start(ctx)
		if err == nil {
			t.Error("expected error when starting already-running adapter")
		}

		cancel()
		<-done
	})

	t.Run("start fails with no handlers", func(t *testing.T) {
		reg := handler.NewRegistry()
		mock := &mockBroker{}
		a := New(mock, reg, nil)

		ctx := context.Background()
		err := a.Start(ctx)

		if err == nil {
			t.Error("expected error when no handlers registered")
		}
	})
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.GroupID != "unicorn-consumer" {
		t.Errorf("expected default group, got %s", cfg.GroupID)
	}
	if !cfg.AutoAck {
		t.Error("AutoAck should default to true")
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("expected 3 retries, got %d", cfg.MaxRetries)
	}
	if cfg.RetryBackoff != time.Second {
		t.Errorf("expected 1s backoff, got %v", cfg.RetryBackoff)
	}
}

func TestIsRunning(t *testing.T) {
	reg := handler.NewRegistry()
	h := handler.New(func(ctx *ucontext.Context) error { return nil }).
		Named("h").Message("test.topic")
	reg.Register(h)

	a := New(&mockBroker{}, reg, nil)
	if a.IsRunning() {
		t.Error("should not be running before Start")
	}
}

func TestPublish(t *testing.T) {
	t.Run("publishes JSON message", func(t *testing.T) {
		reg := handler.NewRegistry()
		mock := &mockBroker{}
		a := New(mock, reg, nil)

		data := map[string]string{"key": "value"}
		err := a.Publish(context.Background(), "test.topic", data)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		mock.mu.Lock()
		defer mock.mu.Unlock()
		if len(mock.messages) != 1 {
			t.Errorf("expected 1 message, got %d", len(mock.messages))
		}
	})

	t.Run("publishes with key", func(t *testing.T) {
		reg := handler.NewRegistry()
		mock := &mockBroker{}
		a := New(mock, reg, nil)

		err := a.PublishWithKey(context.Background(), "test.topic", "key123", []byte("payload"))

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		mock.mu.Lock()
		defer mock.mu.Unlock()
		if len(mock.messages) != 1 {
			t.Errorf("expected 1 message, got %d", len(mock.messages))
		}
		if string(mock.messages[0].Key) != "key123" {
			t.Errorf("expected key 'key123', got '%s'", string(mock.messages[0].Key))
		}
	})
}
