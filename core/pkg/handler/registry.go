package handler

import (
	"fmt"
	"sync"
)

// Registry menyimpan semua registered handlers
type Registry struct {
	mu       sync.RWMutex
	handlers map[string]*Handler

	// Index by trigger type for quick lookup
	httpHandlers    map[string]*Handler // key: "METHOD:PATH"
	messageHandlers map[string]*Handler // key: "topic" (generic broker)
	kafkaHandlers   map[string]*Handler // key: "topic" (legacy)
	grpcHandlers    map[string]*Handler // key: "service.method"
	cronHandlers    []*Handler
}

// NewRegistry creates a new handler registry
func NewRegistry() *Registry {
	return &Registry{
		handlers:        make(map[string]*Handler),
		httpHandlers:    make(map[string]*Handler),
		messageHandlers: make(map[string]*Handler),
		kafkaHandlers:   make(map[string]*Handler),
		grpcHandlers:    make(map[string]*Handler),
		cronHandlers:    make([]*Handler, 0),
	}
}

// Register adds a handler to the registry
func (r *Registry) Register(h *Handler) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Generate name if not set
	if h.Name == "" {
		h.Name = fmt.Sprintf("handler_%d", len(r.handlers)+1)
	}

	// Check for duplicate
	if _, exists := r.handlers[h.Name]; exists {
		return fmt.Errorf("handler already registered: %s", h.Name)
	}

	// Store handler
	r.handlers[h.Name] = h

	// Index by trigger type
	for _, trigger := range h.Triggers() {
		switch t := trigger.(type) {
		case *HTTPTrigger:
			key := fmt.Sprintf("%s:%s", t.Method, t.Path)
			if _, exists := r.httpHandlers[key]; exists {
				return fmt.Errorf("HTTP handler already registered: %s", key)
			}
			r.httpHandlers[key] = h

		case *MessageTrigger:
			if _, exists := r.messageHandlers[t.Topic]; exists {
				return fmt.Errorf("Message handler already registered for topic: %s", t.Topic)
			}
			r.messageHandlers[t.Topic] = h

		case *KafkaTrigger:
			// Legacy support - also register to messageHandlers
			if _, exists := r.kafkaHandlers[t.Topic]; exists {
				return fmt.Errorf("Kafka handler already registered for topic: %s", t.Topic)
			}
			r.kafkaHandlers[t.Topic] = h
			r.messageHandlers[t.Topic] = h // Also add to generic message handlers

		case *GRPCTrigger:
			key := fmt.Sprintf("%s.%s", t.Service, t.Method)
			if _, exists := r.grpcHandlers[key]; exists {
				return fmt.Errorf("gRPC handler already registered: %s", key)
			}
			r.grpcHandlers[key] = h

		case *CronTrigger:
			r.cronHandlers = append(r.cronHandlers, h)
		}
	}

	return nil
}

// Get retrieves a handler by name
func (r *Registry) Get(name string) (*Handler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.handlers[name]
	return h, ok
}

// GetHTTPHandler retrieves handler for HTTP route
func (r *Registry) GetHTTPHandler(method, path string) (*Handler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	key := fmt.Sprintf("%s:%s", method, path)
	h, ok := r.httpHandlers[key]
	return h, ok
}

// GetMessageHandler retrieves handler for message topic (generic broker)
func (r *Registry) GetMessageHandler(topic string) (*Handler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.messageHandlers[topic]
	return h, ok
}

// GetKafkaHandler retrieves handler for Kafka topic (legacy)
func (r *Registry) GetKafkaHandler(topic string) (*Handler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.kafkaHandlers[topic]
	return h, ok
}

// GetGRPCHandler retrieves handler for gRPC method
func (r *Registry) GetGRPCHandler(service, method string) (*Handler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	key := fmt.Sprintf("%s.%s", service, method)
	h, ok := r.grpcHandlers[key]
	return h, ok
}

// GetCronHandlers returns all cron handlers
func (r *Registry) GetCronHandlers() []*Handler {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cronHandlers
}

// All returns all handlers
func (r *Registry) All() map[string]*Handler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*Handler)
	for k, v := range r.handlers {
		result[k] = v
	}
	return result
}

// HTTPRoutes returns all HTTP routes
func (r *Registry) HTTPRoutes() map[string]*Handler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*Handler)
	for k, v := range r.httpHandlers {
		result[k] = v
	}
	return result
}

// MessageTopics returns all message topics (generic broker)
func (r *Registry) MessageTopics() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	topics := make([]string, 0, len(r.messageHandlers))
	for topic := range r.messageHandlers {
		topics = append(topics, topic)
	}
	return topics
}

// KafkaTopics returns all Kafka topics (legacy)
func (r *Registry) KafkaTopics() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	topics := make([]string, 0, len(r.kafkaHandlers))
	for topic := range r.kafkaHandlers {
		topics = append(topics, topic)
	}
	return topics
}

// MessageHandlers returns all message handlers
func (r *Registry) MessageHandlers() map[string]*Handler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*Handler)
	for k, v := range r.messageHandlers {
		result[k] = v
	}
	return result
}

// Count returns total number of handlers
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.handlers)
}

// HasMessageHandlers returns true if there are message handlers
func (r *Registry) HasMessageHandlers() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.messageHandlers) > 0
}

// HasHTTPHandlers returns true if there are HTTP handlers
func (r *Registry) HasHTTPHandlers() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.httpHandlers) > 0
}

// HasCronHandlers returns true if there are cron handlers
func (r *Registry) HasCronHandlers() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.cronHandlers) > 0
}

// CronSchedules returns all cron schedules
func (r *Registry) CronSchedules() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	schedules := make([]string, 0, len(r.cronHandlers))
	for _, h := range r.cronHandlers {
		for _, trigger := range h.Triggers() {
			if cronTrigger, ok := trigger.(*CronTrigger); ok {
				schedules = append(schedules, cronTrigger.Schedule)
			}
		}
	}
	return schedules
}
