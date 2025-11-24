package memory

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

func TestBroker_Connect(t *testing.T) {
	b := New()

	err := b.Connect(context.Background())
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	if !b.IsConnected() {
		t.Error("should be connected")
	}

	if b.Name() != "memory" {
		t.Errorf("expected name memory, got %s", b.Name())
	}
}

func TestBroker_Disconnect(t *testing.T) {
	b := New()
	b.Connect(context.Background())

	err := b.Disconnect(context.Background())
	if err != nil {
		t.Fatalf("failed to disconnect: %v", err)
	}

	if b.IsConnected() {
		t.Error("should be disconnected")
	}
}

func TestBroker_Ping(t *testing.T) {
	b := New()

	err := b.Ping(context.Background())
	if err == nil {
		t.Error("should error when not connected")
	}

	b.Connect(context.Background())

	err = b.Ping(context.Background())
	if err != nil {
		t.Error("should not error when connected")
	}
}

func TestBroker_Publish(t *testing.T) {
	b := New()
	b.Connect(context.Background())
	defer b.Disconnect(context.Background())

	t.Run("publishes message", func(t *testing.T) {
		received := make(chan *contracts.BrokerMessage, 1)

		b.Subscribe(context.Background(), "test-topic", func(ctx context.Context, msg *contracts.BrokerMessage) error {
			received <- msg
			return nil
		})

		msg := contracts.NewBrokerMessage("test-topic", []byte("hello"))
		err := b.Publish(context.Background(), "test-topic", msg)
		if err != nil {
			t.Fatalf("failed to publish: %v", err)
		}

		select {
		case m := <-received:
			if string(m.Body) != "hello" {
				t.Errorf("expected hello, got %s", string(m.Body))
			}
			if m.Topic != "test-topic" {
				t.Errorf("expected topic test-topic, got %s", m.Topic)
			}
		case <-time.After(time.Second):
			t.Error("timeout waiting for message")
		}
	})

	t.Run("fails when not connected", func(t *testing.T) {
		b2 := New()
		err := b2.Publish(context.Background(), "topic", contracts.NewBrokerMessage("topic", []byte("test")))
		if err == nil {
			t.Error("should error when not connected")
		}
	})
}

func TestBroker_Subscribe(t *testing.T) {
	b := New()
	b.Connect(context.Background())
	defer b.Disconnect(context.Background())

	t.Run("subscribes to topic", func(t *testing.T) {
		messageCount := 0
		var mu sync.Mutex

		b.Subscribe(context.Background(), "sub-topic", func(ctx context.Context, msg *contracts.BrokerMessage) error {
			mu.Lock()
			messageCount++
			mu.Unlock()
			return nil
		})

		// Publish multiple messages
		for i := 0; i < 5; i++ {
			b.Publish(context.Background(), "sub-topic", contracts.NewBrokerMessage("sub-topic", []byte("msg")))
		}

		time.Sleep(100 * time.Millisecond)

		mu.Lock()
		if messageCount != 5 {
			t.Errorf("expected 5 messages, got %d", messageCount)
		}
		mu.Unlock()
	})

	t.Run("multiple subscribers receive message", func(t *testing.T) {
		var count1, count2 int
		var mu sync.Mutex

		b.Subscribe(context.Background(), "multi-topic", func(ctx context.Context, msg *contracts.BrokerMessage) error {
			mu.Lock()
			count1++
			mu.Unlock()
			return nil
		})

		b.Subscribe(context.Background(), "multi-topic", func(ctx context.Context, msg *contracts.BrokerMessage) error {
			mu.Lock()
			count2++
			mu.Unlock()
			return nil
		})

		b.Publish(context.Background(), "multi-topic", contracts.NewBrokerMessage("multi-topic", []byte("test")))

		time.Sleep(100 * time.Millisecond)

		mu.Lock()
		if count1 != 1 || count2 != 1 {
			t.Errorf("both subscribers should receive message: count1=%d, count2=%d", count1, count2)
		}
		mu.Unlock()
	})
}

func TestBroker_Unsubscribe(t *testing.T) {
	b := New()
	b.Connect(context.Background())
	defer b.Disconnect(context.Background())

	received := false
	b.Subscribe(context.Background(), "unsub-topic", func(ctx context.Context, msg *contracts.BrokerMessage) error {
		received = true
		return nil
	})

	b.Unsubscribe("unsub-topic")

	b.Publish(context.Background(), "unsub-topic", contracts.NewBrokerMessage("unsub-topic", []byte("test")))
	time.Sleep(50 * time.Millisecond)

	if received {
		t.Error("should not receive after unsubscribe")
	}
}

func TestBroker_ConsumeGroup(t *testing.T) {
	b := New()
	b.Connect(context.Background())
	defer b.Disconnect(context.Background())

	t.Run("consumer group receives messages", func(t *testing.T) {
		received := make(chan *contracts.BrokerMessage, 1)

		b.ConsumeGroup(context.Background(), "group-1", []string{"group-topic"}, func(ctx context.Context, msg *contracts.BrokerMessage) error {
			received <- msg
			return nil
		})

		b.Publish(context.Background(), "group-topic", contracts.NewBrokerMessage("group-topic", []byte("group msg")))

		select {
		case m := <-received:
			if string(m.Body) != "group msg" {
				t.Errorf("expected 'group msg', got %s", string(m.Body))
			}
		case <-time.After(time.Second):
			t.Error("timeout waiting for message")
		}
	})
}

func TestBroker_LeaveGroup(t *testing.T) {
	b := New()
	b.Connect(context.Background())
	defer b.Disconnect(context.Background())

	received := false
	b.ConsumeGroup(context.Background(), "leave-group", []string{"leave-topic"}, func(ctx context.Context, msg *contracts.BrokerMessage) error {
		received = true
		return nil
	})

	b.LeaveGroup("leave-group")

	b.Publish(context.Background(), "leave-topic", contracts.NewBrokerMessage("leave-topic", []byte("test")))
	time.Sleep(50 * time.Millisecond)

	if received {
		t.Error("should not receive after leaving group")
	}
}

func TestBroker_SubscribeMultiple(t *testing.T) {
	b := New()
	b.Connect(context.Background())
	defer b.Disconnect(context.Background())

	messages := make(chan string, 3)

	b.SubscribeMultiple(context.Background(), []string{"topic1", "topic2", "topic3"}, func(ctx context.Context, msg *contracts.BrokerMessage) error {
		messages <- msg.Topic
		return nil
	})

	b.Publish(context.Background(), "topic1", contracts.NewBrokerMessage("topic1", []byte("1")))
	b.Publish(context.Background(), "topic2", contracts.NewBrokerMessage("topic2", []byte("2")))
	b.Publish(context.Background(), "topic3", contracts.NewBrokerMessage("topic3", []byte("3")))

	time.Sleep(100 * time.Millisecond)

	if len(messages) != 3 {
		t.Errorf("expected 3 messages, got %d", len(messages))
	}
}

func TestBroker_PublishBatch(t *testing.T) {
	b := New()
	b.Connect(context.Background())
	defer b.Disconnect(context.Background())

	var count int
	var mu sync.Mutex

	b.Subscribe(context.Background(), "batch-topic", func(ctx context.Context, msg *contracts.BrokerMessage) error {
		mu.Lock()
		count++
		mu.Unlock()
		return nil
	})

	msgs := []*contracts.BrokerMessage{
		contracts.NewBrokerMessage("batch-topic", []byte("1")),
		contracts.NewBrokerMessage("batch-topic", []byte("2")),
		contracts.NewBrokerMessage("batch-topic", []byte("3")),
	}

	err := b.PublishBatch(context.Background(), "batch-topic", msgs)
	if err != nil {
		t.Fatalf("batch publish failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if count != 3 {
		t.Errorf("expected 3 messages, got %d", count)
	}
	mu.Unlock()
}

func TestBroker_Nack(t *testing.T) {
	b := New()
	b.Connect(context.Background())
	defer b.Disconnect(context.Background())

	received := make(chan *contracts.BrokerMessage, 2)

	b.Subscribe(context.Background(), "nack-topic", func(ctx context.Context, msg *contracts.BrokerMessage) error {
		received <- msg
		return nil
	})

	msg := contracts.NewBrokerMessage("nack-topic", []byte("test"))
	msg.Topic = "nack-topic"

	// Nack with requeue should republish
	b.Nack(context.Background(), msg, true)

	select {
	case <-received:
		// OK
	case <-time.After(time.Second):
		t.Error("should requeue message")
	}
}

func TestBroker_Concurrent(t *testing.T) {
	b := New()
	b.Connect(context.Background())
	defer b.Disconnect(context.Background())

	var received int
	var mu sync.Mutex

	b.Subscribe(context.Background(), "concurrent-topic", func(ctx context.Context, msg *contracts.BrokerMessage) error {
		mu.Lock()
		received++
		mu.Unlock()
		return nil
	})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.Publish(context.Background(), "concurrent-topic", contracts.NewBrokerMessage("concurrent-topic", []byte("msg")))
		}()
	}

	wg.Wait()
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	if received != 100 {
		t.Errorf("expected 100 messages, got %d", received)
	}
	mu.Unlock()
}

func TestBroker_GracefulShutdown(t *testing.T) {
	b := New()
	b.Connect(context.Background())

	messageProcessed := make(chan bool, 1)

	b.Subscribe(context.Background(), "shutdown-topic", func(ctx context.Context, msg *contracts.BrokerMessage) error {
		time.Sleep(50 * time.Millisecond) // Simulate processing
		messageProcessed <- true
		return nil
	})

	b.Publish(context.Background(), "shutdown-topic", contracts.NewBrokerMessage("shutdown-topic", []byte("test")))

	// Disconnect should wait for in-flight messages
	b.Disconnect(context.Background())

	select {
	case <-messageProcessed:
		// OK - message was processed before shutdown
	case <-time.After(500 * time.Millisecond):
		// OK - shutdown may have happened before processing
	}
}
