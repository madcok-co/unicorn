package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/madcok-co/unicorn/core/pkg/app"
	ucontext "github.com/madcok-co/unicorn/core/pkg/context"
	"github.com/madcok-co/unicorn/core/pkg/contracts"

	kafkaBroker "github.com/madcok-co/unicorn/core/pkg/adapters/broker/kafka"
	loggerAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/logger"
	metricsAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/metrics"
)

// ============================================================
// EVENT MODELS
// ============================================================

type OrderCreatedEvent struct {
	OrderID   string    `json:"order_id"`
	UserID    string    `json:"user_id"`
	ProductID string    `json:"product_id"`
	Amount    float64   `json:"amount"`
	Currency  string    `json:"currency"`
	Quantity  int       `json:"quantity"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type PaymentProcessedEvent struct {
	PaymentID     string    `json:"payment_id"`
	OrderID       string    `json:"order_id"`
	TransactionID string    `json:"transaction_id"`
	Amount        float64   `json:"amount"`
	Status        string    `json:"status"`
	ProcessedAt   time.Time `json:"processed_at"`
}

type InventoryUpdatedEvent struct {
	ProductID      string    `json:"product_id"`
	PreviousStock  int       `json:"previous_stock"`
	CurrentStock   int       `json:"current_stock"`
	ReservedAmount int       `json:"reserved_amount"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type NotificationEvent struct {
	UserID    string                 `json:"user_id"`
	Type      string                 `json:"type"` // email, sms, push
	Template  string                 `json:"template"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
}

// ============================================================
// HTTP HANDLERS - Publishers
// ============================================================

// CreateOrder publishes order.created event
func CreateOrder(ctx *ucontext.Context, req OrderCreatedEvent) (map[string]interface{}, error) {
	logger := ctx.Logger()
	broker := ctx.Broker()
	metrics := ctx.Metrics()

	// Set defaults
	req.OrderID = fmt.Sprintf("order_%d", time.Now().UnixNano())
	req.Status = "pending"
	req.CreatedAt = time.Now()

	// Publish to Kafka
	eventData, _ := json.Marshal(req)
	message := &contracts.BrokerMessage{
		Topic: "order.created",
		Key:   []byte(req.OrderID),
		Body:  eventData,
		Headers: map[string]string{
			"event_type":   "order.created",
			"user_id":      req.UserID,
			"published_at": time.Now().Format(time.RFC3339),
		},
	}

	if err := broker.Publish(ctx.Context(), "order.created", message); err != nil {
		logger.Error("failed to publish order.created event", "error", err)
		metrics.Counter("kafka_publish_failed", T("topic", "order.created")).Inc()
		return nil, fmt.Errorf("failed to publish event: %w", err)
	}

	logger.Info("order.created event published",
		"order_id", req.OrderID,
		"user_id", req.UserID,
		"amount", req.Amount)

	metrics.Counter("kafka_published", T("topic", "order.created")).Inc()

	return map[string]interface{}{
		"message":  "Order created and event published",
		"order_id": req.OrderID,
		"status":   "pending",
	}, nil
}

// ProcessPayment publishes payment.processed event
func ProcessPayment(ctx *ucontext.Context, req PaymentProcessedEvent) (map[string]interface{}, error) {
	logger := ctx.Logger()
	broker := ctx.Broker()
	metrics := ctx.Metrics()

	req.PaymentID = fmt.Sprintf("pay_%d", time.Now().UnixNano())
	req.TransactionID = fmt.Sprintf("txn_%d", time.Now().UnixNano())
	req.ProcessedAt = time.Now()

	eventData, _ := json.Marshal(req)
	message := &contracts.BrokerMessage{
		Topic: "payment.processed",
		Key:   []byte(req.OrderID),
		Body:  eventData,
		Headers: map[string]string{
			"event_type": "payment.processed",
			"order_id":   req.OrderID,
		},
	}

	if err := broker.Publish(ctx.Context(), "payment.processed", message); err != nil {
		logger.Error("failed to publish payment.processed event", "error", err)
		metrics.Counter("kafka_publish_failed", T("topic", "payment.processed")).Inc()
		return nil, fmt.Errorf("failed to publish event: %w", err)
	}

	logger.Info("payment.processed event published",
		"payment_id", req.PaymentID,
		"order_id", req.OrderID,
		"amount", req.Amount)

	metrics.Counter("kafka_published", T("topic", "payment.processed")).Inc()

	return map[string]interface{}{
		"message":        "Payment processed",
		"payment_id":     req.PaymentID,
		"transaction_id": req.TransactionID,
	}, nil
}

// ============================================================
// MESSAGE HANDLERS - Consumers
// ============================================================

// HandleOrderCreated - Process order.created event
func HandleOrderCreated(ctx *ucontext.Context, msg *contracts.BrokerMessage) error {
	logger := ctx.Logger()
	broker := ctx.Broker()
	metrics := ctx.Metrics()

	// Parse event
	var event OrderCreatedEvent
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		logger.Error("failed to unmarshal order.created event", "error", err)
		metrics.Counter("kafka_consume_failed", T("topic", "order.created"), T("reason", "parse_error")).Inc()
		return err
	}

	logger.Info("processing order.created event",
		"order_id", event.OrderID,
		"user_id", event.UserID,
		"amount", event.Amount)

	// Simulate business logic
	time.Sleep(100 * time.Millisecond)

	// 1. Update inventory
	inventoryEvent := InventoryUpdatedEvent{
		ProductID:      event.ProductID,
		PreviousStock:  100,
		CurrentStock:   100 - event.Quantity,
		ReservedAmount: event.Quantity,
		UpdatedAt:      time.Now(),
	}

	inventoryData, _ := json.Marshal(inventoryEvent)
	inventoryMsg := &contracts.BrokerMessage{
		Topic: "inventory.updated",
		Key:   []byte(event.ProductID),
		Body:  inventoryData,
	}
	broker.Publish(ctx.Context(), "inventory.updated", inventoryMsg)

	// 2. Send notification
	notificationEvent := NotificationEvent{
		UserID:   event.UserID,
		Type:     "email",
		Template: "order_confirmation",
		Data: map[string]interface{}{
			"order_id": event.OrderID,
			"amount":   event.Amount,
			"currency": event.Currency,
		},
		Timestamp: time.Now(),
	}

	notificationData, _ := json.Marshal(notificationEvent)
	notificationMsg := &contracts.BrokerMessage{
		Topic: "notification.send",
		Key:   []byte(event.UserID),
		Body:  notificationData,
	}
	broker.Publish(ctx.Context(), "notification.send", notificationMsg)

	logger.Info("order.created event processed successfully", "order_id", event.OrderID)
	metrics.Counter("kafka_consumed", T("topic", "order.created"), T("status", "success")).Inc()
	metrics.Histogram("kafka_processing_duration", T("topic", "order.created")).Observe(0.1)

	return nil
}

// HandlePaymentProcessed - Process payment.processed event
func HandlePaymentProcessed(ctx *ucontext.Context, msg *contracts.BrokerMessage) error {
	logger := ctx.Logger()
	metrics := ctx.Metrics()

	var event PaymentProcessedEvent
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		logger.Error("failed to unmarshal payment.processed event", "error", err)
		metrics.Counter("kafka_consume_failed", T("topic", "payment.processed")).Inc()
		return err
	}

	logger.Info("processing payment.processed event",
		"payment_id", event.PaymentID,
		"order_id", event.OrderID,
		"status", event.Status)

	// Simulate payment processing logic
	time.Sleep(50 * time.Millisecond)

	if event.Status == "success" {
		logger.Info("payment successful, order can be fulfilled", "order_id", event.OrderID)
	} else {
		logger.Warn("payment failed, need to handle rollback", "order_id", event.OrderID)
	}

	metrics.Counter("kafka_consumed", T("topic", "payment.processed"), T("status", event.Status)).Inc()

	return nil
}

// HandleInventoryUpdated - Process inventory.updated event
func HandleInventoryUpdated(ctx *ucontext.Context, msg *contracts.BrokerMessage) error {
	logger := ctx.Logger()
	metrics := ctx.Metrics()

	var event InventoryUpdatedEvent
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		logger.Error("failed to unmarshal inventory.updated event", "error", err)
		return err
	}

	logger.Info("processing inventory.updated event",
		"product_id", event.ProductID,
		"previous_stock", event.PreviousStock,
		"current_stock", event.CurrentStock)

	// Check if reorder needed
	if event.CurrentStock < 10 {
		logger.Warn("low stock alert",
			"product_id", event.ProductID,
			"current_stock", event.CurrentStock)
		metrics.Counter("inventory_low_stock_alert").Inc()
	}

	metrics.Counter("kafka_consumed", T("topic", "inventory.updated")).Inc()
	metrics.Gauge("inventory_stock", T("product_id", event.ProductID)).Set(float64(event.CurrentStock))

	return nil
}

// HandleNotification - Process notification.send event
func HandleNotification(ctx *ucontext.Context, msg *contracts.BrokerMessage) error {
	logger := ctx.Logger()
	metrics := ctx.Metrics()

	var event NotificationEvent
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		logger.Error("failed to unmarshal notification event", "error", err)
		return err
	}

	logger.Info("sending notification",
		"user_id", event.UserID,
		"type", event.Type,
		"template", event.Template)

	// Simulate sending notification
	time.Sleep(30 * time.Millisecond)

	switch event.Type {
	case "email":
		logger.Info("📧 email sent", "user_id", event.UserID, "template", event.Template)
	case "sms":
		logger.Info("📱 SMS sent", "user_id", event.UserID)
	case "push":
		logger.Info("🔔 Push notification sent", "user_id", event.UserID)
	}

	metrics.Counter("notifications_sent", T("type", event.Type), T("template", event.Template)).Inc()

	return nil
}

// ============================================================
// MAIN
// ============================================================

func main() {
	// Get Kafka configuration from environment
	kafkaBrokers := getEnv("KAFKA_BROKERS", "localhost:9092")
	consumerGroup := getEnv("KAFKA_CONSUMER_GROUP", "unicorn-app")

	// Create application
	application := app.New(&app.Config{
		Name:    "kafka-example",
		Version: "1.0.0",
	})

	// Setup logger
	logger := loggerAdapter.NewConsoleLogger("info")
	application.SetLogger(logger)

	// Setup metrics
	metrics := metricsAdapter.New(metricsAdapter.NewNoopDriver())
	application.SetMetrics(metrics)

	// Setup Kafka broker
	kafkaConfig := &contracts.KafkaBrokerConfig{
		BrokerConfig: contracts.BrokerConfig{
			Brokers:       []string{kafkaBrokers},
			ConsumerGroup: consumerGroup,
		},
	}

	broker := kafkaBroker.New(kafkaConfig)

	// Connect to Kafka
	if err := broker.Connect(context.Background()); err != nil {
		log.Fatal("Failed to connect to Kafka:", err)
	}

	application.SetBroker(broker)

	logger.Info("✅ Connected to Kafka", "brokers", kafkaBrokers)

	// Register HTTP handlers (Publishers)
	application.RegisterHandler(CreateOrder).
		Named("create-order").
		HTTP("POST", "/orders").
		Done()

	application.RegisterHandler(ProcessPayment).
		Named("process-payment").
		HTTP("POST", "/payments").
		Done()

	// Register Message handlers (Consumers)
	application.RegisterHandler(HandleOrderCreated).
		Named("order-created-consumer").
		Message("order.created").
		Done()

	application.RegisterHandler(HandlePaymentProcessed).
		Named("payment-processed-consumer").
		Message("payment.processed").
		Done()

	application.RegisterHandler(HandleInventoryUpdated).
		Named("inventory-updated-consumer").
		Message("inventory.updated").
		Done()

	application.RegisterHandler(HandleNotification).
		Named("notification-consumer").
		Message("notification.send").
		Done()

	logger.Info("📨 Kafka Example with Event-Driven Architecture Started!")
	logger.Info("🔗 Kafka Brokers:", kafkaBrokers)
	logger.Info("👥 Consumer Group:", consumerGroup)
	logger.Info("")
	logger.Info("📝 Topics:")
	logger.Info("  • order.created      - New orders")
	logger.Info("  • payment.processed  - Payment confirmations")
	logger.Info("  • inventory.updated  - Stock updates")
	logger.Info("  • notification.send  - User notifications")
	logger.Info("")
	logger.Info("🌐 HTTP Endpoints (Publishers):")
	logger.Info("  POST /orders   - Create order (publishes event)")
	logger.Info("  POST /payments - Process payment (publishes event)")
	logger.Info("")
	logger.Info("👂 Message Consumers:")
	logger.Info("  ✓ Listening to all topics...")

	// Start application
	if err := application.Start(); err != nil {
		log.Fatal(err)
	}
}

// Helper functions
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func T(key, value string) contracts.Tag {
	return contracts.T(key, value)
}
