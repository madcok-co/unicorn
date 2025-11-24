package handler

import (
	"reflect"

	ucontext "github.com/madcok-co/unicorn/core/pkg/context"
)

// HandlerFunc adalah type untuk handler function dari user
// Signature: func(ctx *unicorn.Context, req T) (R, error)
type HandlerFunc any

// Handler wraps user function dengan metadata dan triggers
type Handler struct {
	// Handler metadata
	Name        string
	Description string

	// The actual handler function
	fn HandlerFunc

	// Request/Response types (untuk serialization)
	requestType  reflect.Type
	responseType reflect.Type

	// Registered triggers
	triggers []Trigger

	// Middleware chain
	middlewares []Middleware
}

// Trigger represents bagaimana handler bisa di-invoke
type Trigger interface {
	Type() TriggerType
	Config() any
}

// TriggerType enum
type TriggerType string

const (
	TriggerHTTP    TriggerType = "http"
	TriggerMessage TriggerType = "message" // Generic message broker
	TriggerKafka   TriggerType = "kafka"   // Legacy, use TriggerMessage
	TriggerGRPC    TriggerType = "grpc"
	TriggerCron    TriggerType = "cron"
	TriggerPubSub  TriggerType = "pubsub" // Legacy, use TriggerMessage
)

// Middleware type
type Middleware func(next HandlerExecutor) HandlerExecutor

// HandlerExecutor adalah function untuk execute handler
type HandlerExecutor func(ctx *ucontext.Context) error

// New creates a new Handler dari user function
func New(fn HandlerFunc) *Handler {
	h := &Handler{
		fn:       fn,
		triggers: make([]Trigger, 0),
	}

	// Extract request/response types dari function signature
	h.extractTypes()

	return h
}

// extractTypes extracts request dan response types dari handler function
func (h *Handler) extractTypes() {
	fnType := reflect.TypeOf(h.fn)
	if fnType.Kind() != reflect.Func {
		panic("handler must be a function")
	}

	// Expected signature: func(ctx *Context, req T) (R, error)
	if fnType.NumIn() >= 2 {
		h.requestType = fnType.In(1)
	}

	if fnType.NumOut() >= 1 {
		h.responseType = fnType.Out(0)
	}
}

// Named sets handler name
func (h *Handler) Named(name string) *Handler {
	h.Name = name
	return h
}

// Describe sets handler description
func (h *Handler) Describe(desc string) *Handler {
	h.Description = desc
	return h
}

// Use adds middleware to handler
func (h *Handler) Use(middleware ...Middleware) *Handler {
	h.middlewares = append(h.middlewares, middleware...)
	return h
}

// ============ Trigger Registration ============

// HTTP registers HTTP trigger
func (h *Handler) HTTP(method, path string) *Handler {
	h.triggers = append(h.triggers, &HTTPTrigger{
		Method: method,
		Path:   path,
	})
	return h
}

// Message registers generic message broker trigger
// Works with Kafka, RabbitMQ, Redis, NATS, etc - broker agnostic
func (h *Handler) Message(topic string, opts ...MessageOption) *Handler {
	trigger := &MessageTrigger{
		Topic:   topic,
		AutoAck: true, // default auto ack
	}
	for _, opt := range opts {
		opt(trigger)
	}
	h.triggers = append(h.triggers, trigger)
	return h
}

// Kafka registers Kafka trigger (legacy, use Message for new code)
func (h *Handler) Kafka(topic string, opts ...KafkaOption) *Handler {
	trigger := &KafkaTrigger{
		Topic:   topic,
		GroupID: "", // will use default
	}
	for _, opt := range opts {
		opt(trigger)
	}
	h.triggers = append(h.triggers, trigger)
	return h
}

// GRPC registers gRPC trigger
func (h *Handler) GRPC(service, method string) *Handler {
	h.triggers = append(h.triggers, &GRPCTrigger{
		Service: service,
		Method:  method,
	})
	return h
}

// Cron registers Cron trigger
func (h *Handler) Cron(schedule string) *Handler {
	h.triggers = append(h.triggers, &CronTrigger{
		Schedule: schedule,
	})
	return h
}

// ============ Getters ============

// Triggers returns all registered triggers
func (h *Handler) Triggers() []Trigger {
	return h.triggers
}

// RequestType returns the request type
func (h *Handler) RequestType() reflect.Type {
	return h.requestType
}

// ResponseType returns the response type
func (h *Handler) ResponseType() reflect.Type {
	return h.responseType
}

// Fn returns the handler function
func (h *Handler) Fn() HandlerFunc {
	return h.fn
}

// Middlewares returns the middleware chain
func (h *Handler) Middlewares() []Middleware {
	return h.middlewares
}

// ============ Helper Methods ============

// GetMessageTriggers returns all message-based triggers (Message, Kafka, PubSub)
func (h *Handler) GetMessageTriggers() []*MessageTrigger {
	var triggers []*MessageTrigger
	for _, t := range h.triggers {
		switch v := t.(type) {
		case *MessageTrigger:
			triggers = append(triggers, v)
		case *KafkaTrigger:
			triggers = append(triggers, v.ToMessageTrigger())
		}
	}
	return triggers
}

// HasTriggerType checks if handler has a specific trigger type
func (h *Handler) HasTriggerType(tt TriggerType) bool {
	for _, t := range h.triggers {
		if t.Type() == tt {
			return true
		}
	}
	return false
}
