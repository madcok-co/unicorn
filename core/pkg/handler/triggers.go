package handler

import "time"

// ============ HTTP Trigger ============

// HTTPTrigger configuration for HTTP trigger
type HTTPTrigger struct {
	Method      string
	Path        string
	Middlewares []string // middleware names to apply
}

func (t *HTTPTrigger) Type() TriggerType { return TriggerHTTP }
func (t *HTTPTrigger) Config() any       { return t }

// ============ Message Trigger (Generic Broker) ============

// MessageTrigger configuration for generic message broker trigger
// Works with Kafka, RabbitMQ, Redis, NATS, Google Pub/Sub, AWS SQS, etc
type MessageTrigger struct {
	// Topic/Queue/Channel name
	Topic string

	// Consumer group (untuk Kafka, RabbitMQ queue consumers, etc)
	Group string

	// Processing options
	AutoAck    bool   // Auto acknowledge after successful processing
	MaxRetries int    // Max retries before sending to DLQ
	DLQTopic   string // Dead Letter Queue topic

	// Retry configuration
	RetryBackoff time.Duration

	// Broker-specific options
	Options map[string]any
}

func (t *MessageTrigger) Type() TriggerType { return TriggerMessage }
func (t *MessageTrigger) Config() any       { return t }

// MessageOption untuk konfigurasi Message trigger
type MessageOption func(*MessageTrigger)

// WithGroup sets consumer group
func WithGroup(group string) MessageOption {
	return func(t *MessageTrigger) {
		t.Group = group
	}
}

// WithAutoAck enables/disables auto acknowledgment
func WithAutoAck(enabled bool) MessageOption {
	return func(t *MessageTrigger) {
		t.AutoAck = enabled
	}
}

// WithRetries sets max retries and backoff
func WithRetries(maxRetries int, backoff time.Duration) MessageOption {
	return func(t *MessageTrigger) {
		t.MaxRetries = maxRetries
		t.RetryBackoff = backoff
	}
}

// WithDeadLetter sets dead letter queue topic
func WithDeadLetter(dlqTopic string) MessageOption {
	return func(t *MessageTrigger) {
		t.DLQTopic = dlqTopic
	}
}

// WithOption sets broker-specific option
func WithOption(key string, value any) MessageOption {
	return func(t *MessageTrigger) {
		if t.Options == nil {
			t.Options = make(map[string]any)
		}
		t.Options[key] = value
	}
}

// ============ Kafka Trigger (Legacy, wraps MessageTrigger) ============

// KafkaTrigger configuration for Kafka trigger
// Deprecated: Use MessageTrigger instead for broker-agnostic code
type KafkaTrigger struct {
	Topic      string
	GroupID    string
	Partition  int
	AutoCommit bool
	MaxRetries int
	DLQTopic   string // Dead Letter Queue topic
}

func (t *KafkaTrigger) Type() TriggerType { return TriggerKafka }
func (t *KafkaTrigger) Config() any       { return t }

// ToMessageTrigger converts to generic MessageTrigger
func (t *KafkaTrigger) ToMessageTrigger() *MessageTrigger {
	return &MessageTrigger{
		Topic:      t.Topic,
		Group:      t.GroupID,
		AutoAck:    t.AutoCommit,
		MaxRetries: t.MaxRetries,
		DLQTopic:   t.DLQTopic,
		Options: map[string]any{
			"partition": t.Partition,
		},
	}
}

// KafkaOption untuk konfigurasi Kafka trigger
type KafkaOption func(*KafkaTrigger)

// WithGroupID sets consumer group ID
func WithGroupID(groupID string) KafkaOption {
	return func(t *KafkaTrigger) {
		t.GroupID = groupID
	}
}

// WithPartition sets specific partition
func WithPartition(partition int) KafkaOption {
	return func(t *KafkaTrigger) {
		t.Partition = partition
	}
}

// WithAutoCommit enables/disables auto commit
func WithAutoCommit(enabled bool) KafkaOption {
	return func(t *KafkaTrigger) {
		t.AutoCommit = enabled
	}
}

// WithMaxRetries sets max retries before DLQ
func WithMaxRetries(retries int) KafkaOption {
	return func(t *KafkaTrigger) {
		t.MaxRetries = retries
	}
}

// WithDLQ sets dead letter queue topic
func WithDLQ(topic string) KafkaOption {
	return func(t *KafkaTrigger) {
		t.DLQTopic = topic
	}
}

// ============ gRPC Trigger ============

// GRPCTrigger configuration for gRPC trigger
type GRPCTrigger struct {
	Service string
	Method  string
}

func (t *GRPCTrigger) Type() TriggerType { return TriggerGRPC }
func (t *GRPCTrigger) Config() any       { return t }

// ============ Cron Trigger ============

// CronTrigger configuration for Cron trigger
type CronTrigger struct {
	Schedule    string // cron expression
	Timezone    string // timezone for schedule
	Overlap     bool   // allow overlapping runs
	MaxDuration int    // max duration in seconds before timeout
}

func (t *CronTrigger) Type() TriggerType { return TriggerCron }
func (t *CronTrigger) Config() any       { return t }

// CronOption untuk konfigurasi Cron trigger
type CronOption func(*CronTrigger)

// WithTimezone sets timezone for cron
func WithTimezone(tz string) CronOption {
	return func(t *CronTrigger) {
		t.Timezone = tz
	}
}

// AllowOverlap allows overlapping runs
func AllowOverlap() CronOption {
	return func(t *CronTrigger) {
		t.Overlap = true
	}
}

// WithMaxDuration sets max duration
func WithMaxDuration(seconds int) CronOption {
	return func(t *CronTrigger) {
		t.MaxDuration = seconds
	}
}

// ============ PubSub Trigger (Legacy) ============

// PubSubTrigger for generic pub/sub systems
// Deprecated: Use MessageTrigger instead
type PubSubTrigger struct {
	Channel  string
	Pattern  bool // use pattern subscription
	Provider string
}

func (t *PubSubTrigger) Type() TriggerType { return TriggerPubSub }
func (t *PubSubTrigger) Config() any       { return t }
