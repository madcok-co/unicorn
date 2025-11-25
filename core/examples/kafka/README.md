# Kafka Example - Event-Driven Architecture

This example demonstrates how to use Apache Kafka with the Unicorn framework for building event-driven microservices.

## Features

- **Event Publishing** - Publish events to Kafka topics
- **Event Consumption** - Subscribe to topics with consumer groups
- **Event-Driven Flow** - Complete order processing workflow
- **Message Headers** - Metadata and tracing support
- **Error Handling** - Automatic retry and dead letter queues
- **Metrics** - Track publish/consume operations

## Architecture

```
HTTP Request (POST /orders)
    ↓
Publish: order.created
    ↓
┌─────────────────────────────────────┐
│         Kafka Topics                │
├─────────────────────────────────────┤
│ • order.created                     │
│ • payment.processed                 │
│ • inventory.updated                 │
│ • notification.send                 │
└─────────────────────────────────────┘
    ↓
Consumers Process Events
    ↓
Chain Reactions:
- Update inventory
- Send notifications
- Process payments
```

## Quick Start

### 1. Start Kafka with Docker Compose

```bash
docker-compose up -d
```

This starts:
- **Zookeeper** on port 2181
- **Kafka** on port 9092
- **Kafka UI** on http://localhost:8090

### 2. Run the Application

```bash
go run main.go
```

Or with custom Kafka brokers:

```bash
KAFKA_BROKERS="localhost:9092,localhost:9093" \
KAFKA_CONSUMER_GROUP="my-app" \
go run main.go
```

### 3. Test Event Publishing

```bash
# Create an order (publishes order.created event)
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user_123",
    "product_id": "prod_456",
    "amount": 99.99,
    "currency": "USD",
    "quantity": 2
  }'

# Process a payment (publishes payment.processed event)
curl -X POST http://localhost:8080/payments \
  -H "Content-Type: application/json" \
  -d '{
    "order_id": "order_123",
    "amount": 99.99,
    "status": "success"
  }'
```

## Event Flow Example

### 1. Order Created
When you POST to `/orders`:

```
1. HTTP Handler publishes → order.created
2. Consumer receives event
3. Consumer publishes → inventory.updated
4. Consumer publishes → notification.send
5. Both events are consumed by respective handlers
```

### 2. Payment Processed
When you POST to `/payments`:

```
1. HTTP Handler publishes → payment.processed
2. Consumer receives event
3. Update order status
4. Trigger fulfillment process
```

## Topics and Events

### order.created
```json
{
  "order_id": "order_1234567890",
  "user_id": "user_123",
  "product_id": "prod_456",
  "amount": 99.99,
  "currency": "USD",
  "quantity": 2,
  "status": "pending",
  "created_at": "2025-01-01T10:00:00Z"
}
```

### payment.processed
```json
{
  "payment_id": "pay_1234567890",
  "order_id": "order_123",
  "transaction_id": "txn_abc123",
  "amount": 99.99,
  "status": "success",
  "processed_at": "2025-01-01T10:00:05Z"
}
```

### inventory.updated
```json
{
  "product_id": "prod_456",
  "previous_stock": 100,
  "current_stock": 98,
  "reserved_amount": 2,
  "updated_at": "2025-01-01T10:00:02Z"
}
```

### notification.send
```json
{
  "user_id": "user_123",
  "type": "email",
  "template": "order_confirmation",
  "data": {
    "order_id": "order_123",
    "amount": 99.99,
    "currency": "USD"
  },
  "timestamp": "2025-01-01T10:00:03Z"
}
```

## Monitoring

### Kafka UI
Open http://localhost:8090 to:
- View topics and partitions
- Monitor consumer lag
- Inspect messages
- See consumer groups

### Application Logs
The application logs all events:
```
INFO  processing order.created event order_id=order_123 amount=99.99
INFO  📧 email sent user_id=user_123 template=order_confirmation
INFO  processing payment.processed event payment_id=pay_456 status=success
```

### Metrics
Metrics are tracked for:
- `kafka_published{topic="..."}` - Events published
- `kafka_consumed{topic="...",status="..."}` - Events consumed
- `kafka_publish_failed{topic="..."}` - Publish failures
- `kafka_processing_duration{topic="..."}` - Processing time

## Configuration

Environment variables:

```bash
# Kafka brokers (comma-separated)
KAFKA_BROKERS=localhost:9092

# Consumer group ID
KAFKA_CONSUMER_GROUP=unicorn-app

# HTTP server port
PORT=8080
```

## Production Considerations

### 1. Consumer Groups
Use consumer groups for load balancing:
```bash
# Run multiple instances with same consumer group
KAFKA_CONSUMER_GROUP=order-processor go run main.go  # Instance 1
KAFKA_CONSUMER_GROUP=order-processor go run main.go  # Instance 2
```

### 2. Partitions
- Use message keys for ordering guarantees
- Partition by entity ID (user_id, order_id)
- Scale consumers up to partition count

### 3. Error Handling
- Implement retry logic
- Configure dead letter queues
- Monitor consumer lag

### 4. Monitoring
- Track consumer lag metrics
- Set up alerts for processing delays
- Monitor partition rebalancing

## Clean Up

```bash
# Stop services
docker-compose down

# Remove volumes
docker-compose down -v
```

## Learn More

- [Kafka Documentation](https://kafka.apache.org/documentation/)
- [Event-Driven Architecture](https://martinfowler.com/articles/201701-event-driven.html)
- [Unicorn Framework Docs](../../README.md)
