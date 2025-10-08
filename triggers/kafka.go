// ============================================
// 3. KAFKA TRIGGER (Consumer)
// ============================================
package triggers

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/madcok-co/unicorn"
	"github.com/segmentio/kafka-go"
)

type KafkaTrigger struct {
	brokers  []string
	groupID  string
	readers  map[string]*kafka.Reader
	stopChan map[string]chan bool
	mu       sync.RWMutex
}

func NewKafkaTrigger(brokers []string, groupID string) *KafkaTrigger {
	return &KafkaTrigger{
		brokers:  brokers,
		groupID:  groupID,
		readers:  make(map[string]*kafka.Reader),
		stopChan: make(map[string]chan bool),
	}
}

func (t *KafkaTrigger) RegisterService(def *unicorn.Definition) error {
	// Subscribe to topic with service name
	topic := def.Name + "-events"
	return t.SubscribeTopic(topic, def.Name)
}

func (t *KafkaTrigger) SubscribeTopic(topic, serviceName string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: t.brokers,
		Topic:   topic,
		GroupID: t.groupID,
	})

	t.readers[topic] = reader
	t.stopChan[topic] = make(chan bool)

	// Start consumer goroutine
	go t.consume(topic, serviceName)

	return nil
}

func (t *KafkaTrigger) consume(topic, serviceName string) {
	reader := t.readers[topic]
	stopChan := t.stopChan[topic]

	for {
		select {
		case <-stopChan:
			return
		default:
			msg, err := reader.ReadMessage(context.Background())
			if err != nil {
				continue
			}

			// Get service
			handler, err := unicorn.GetHandler(serviceName)
			if err != nil {
				continue
			}

			// Parse message
			var request map[string]interface{}
			json.Unmarshal(msg.Value, &request)

			// Create context
			ctx := unicorn.NewContext(context.Background())
			ctx.SetMetadata("kafka_topic", msg.Topic)
			ctx.SetMetadata("kafka_offset", msg.Offset)
			ctx.SetMetadata("kafka_partition", msg.Partition)

			// Execute service
			handler.Handle(ctx, request)
		}
	}
}

func (t *KafkaTrigger) Start() error {
	// Already started in SubscribeTopic
	return nil
}

func (t *KafkaTrigger) Stop() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Stop all consumers
	for topic, stopChan := range t.stopChan {
		close(stopChan)
		if reader, ok := t.readers[topic]; ok {
			reader.Close()
		}
	}

	return nil
}
