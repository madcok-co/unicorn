// ============================================
// UNICORN Framework - Service Context
// Provides access to all framework resources
// ============================================

package unicorn

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
)

var (
	// ErrConnectionNotInitialized indicates connection manager not initialized
	ErrConnectionNotInitialized = errors.New("connection manager not initialized")

	// ErrPluginNotFound indicates plugin not found or not enabled
	ErrPluginNotFound = errors.New("plugin not found or not enabled")
)

// Context provides access to all framework resources within a service.
// It wraps the standard context and adds framework-specific features.
type Context struct {
	ctx         context.Context
	connManager *ConnectionManager
	pluginMgr   *PluginManager
	logger      Logger
	metadata    map[string]interface{}
	metadataMu  sync.RWMutex
	requestID   string
	startTime   time.Time
}

// NewContext creates a new service context.
func NewContext(ctx context.Context) *Context {
	return &Context{
		ctx:       ctx,
		metadata:  make(map[string]interface{}),
		requestID: generateRequestID(),
		startTime: time.Now(),
	}
}

// NewContextWithConnections creates a context with connection manager.
func NewContextWithConnections(ctx context.Context, cm *ConnectionManager) *Context {
	sctx := NewContext(ctx)
	sctx.connManager = cm
	return sctx
}

// NewContextWithPlugins creates a context with plugin manager.
func NewContextWithPlugins(ctx context.Context, pm *PluginManager) *Context {
	sctx := NewContext(ctx)
	sctx.pluginMgr = pm
	return sctx
}

// NewFullContext creates a context with all managers.
func NewFullContext(ctx context.Context, cm *ConnectionManager, pm *PluginManager, logger Logger) *Context {
	sctx := NewContext(ctx)
	sctx.connManager = cm
	sctx.pluginMgr = pm
	sctx.logger = logger
	return sctx
}

// Context returns the underlying context.Context.
func (c *Context) Context() context.Context {
	return c.ctx
}

// WithContext returns a new Context with the given context.Context.
func (c *Context) WithContext(ctx context.Context) *Context {
	newCtx := &Context{
		ctx:         ctx,
		connManager: c.connManager,
		pluginMgr:   c.pluginMgr,
		logger:      c.logger,
		metadata:    make(map[string]interface{}),
		requestID:   c.requestID,
		startTime:   c.startTime,
	}

	// Copy metadata
	c.metadataMu.RLock()
	for k, v := range c.metadata {
		newCtx.metadata[k] = v
	}
	c.metadataMu.RUnlock()

	return newCtx
}

// ============================================
// DATABASE OPERATIONS
// ============================================

// GetDB returns a database connection by name.
// If name is empty, returns the default database.
func (c *Context) GetDB(name string) (*sql.DB, error) {
	if c.connManager == nil {
		return nil, ErrConnectionNotInitialized
	}

	db := c.connManager.GetDB(name)
	if db == nil {
		return nil, fmt.Errorf("database not found: %s", name)
	}

	return db, nil
}

// Query executes a query with the given database.
// It's a convenience method that handles context and error logging.
func (c *Context) Query(dbName string, query string, args ...interface{}) (*sql.Rows, error) {
	db, err := c.GetDB(dbName)
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(c.ctx, query, args...)
	if err != nil {
		if c.logger != nil {
			c.logger.Error("query failed", "db", dbName, "error", err)
		}
		return nil, err
	}

	return rows, nil
}

// QueryRow executes a query that returns a single row.
func (c *Context) QueryRow(dbName string, query string, args ...interface{}) *sql.Row {
	db, _ := c.GetDB(dbName)
	return db.QueryRowContext(c.ctx, query, args...)
}

// Exec executes a query without returning rows.
func (c *Context) Exec(dbName string, query string, args ...interface{}) (sql.Result, error) {
	db, err := c.GetDB(dbName)
	if err != nil {
		return nil, err
	}

	result, err := db.ExecContext(c.ctx, query, args...)
	if err != nil {
		if c.logger != nil {
			c.logger.Error("exec failed", "db", dbName, "error", err)
		}
		return nil, err
	}

	return result, nil
}

// BeginTx starts a database transaction.
func (c *Context) BeginTx(dbName string, opts *sql.TxOptions) (*sql.Tx, error) {
	db, err := c.GetDB(dbName)
	if err != nil {
		return nil, err
	}

	return db.BeginTx(c.ctx, opts)
}

// ============================================
// REDIS OPERATIONS
// ============================================

// GetRedis returns a Redis client by name.
// If name is empty, returns the default Redis client.
func (c *Context) GetRedis(name string) (*redis.Client, error) {
	if c.connManager == nil {
		return nil, ErrConnectionNotInitialized
	}

	client := c.connManager.GetRedis(name)
	if client == nil {
		return nil, fmt.Errorf("redis not found: %s", name)
	}

	return client, nil
}

// RedisGet gets a value from Redis.
func (c *Context) RedisGet(redisName, key string) (string, error) {
	client, err := c.GetRedis(redisName)
	if err != nil {
		return "", err
	}

	val, err := client.Get(c.ctx, key).Result()
	if err != nil {
		if c.logger != nil && err != redis.Nil {
			c.logger.Error("redis get failed", "redis", redisName, "key", key, "error", err)
		}
		return "", err
	}

	return val, nil
}

// RedisSet sets a value in Redis with optional TTL.
func (c *Context) RedisSet(redisName, key, value string, ttl time.Duration) error {
	client, err := c.GetRedis(redisName)
	if err != nil {
		return err
	}

	err = client.Set(c.ctx, key, value, ttl).Err()
	if err != nil && c.logger != nil {
		c.logger.Error("redis set failed", "redis", redisName, "key", key, "error", err)
	}

	return err
}

// RedisDel deletes a key from Redis.
func (c *Context) RedisDel(redisName string, keys ...string) error {
	client, err := c.GetRedis(redisName)
	if err != nil {
		return err
	}

	err = client.Del(c.ctx, keys...).Err()
	if err != nil && c.logger != nil {
		c.logger.Error("redis del failed", "redis", redisName, "error", err)
	}

	return err
}

// RedisExists checks if a key exists in Redis.
func (c *Context) RedisExists(redisName string, keys ...string) (bool, error) {
	client, err := c.GetRedis(redisName)
	if err != nil {
		return false, err
	}

	count, err := client.Exists(c.ctx, keys...).Result()
	return count > 0, err
}

// RedisGetJSON gets and unmarshals a JSON value from Redis.
func (c *Context) RedisGetJSON(redisName, key string, dest interface{}) error {
	val, err := c.RedisGet(redisName, key)
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(val), dest)
}

// RedisSetJSON marshals and sets a JSON value in Redis.
func (c *Context) RedisSetJSON(redisName, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return c.RedisSet(redisName, key, string(data), ttl)
}

// ============================================
// KAFKA OPERATIONS
// ============================================

// GetKafkaWriter returns the Kafka writer.
func (c *Context) GetKafkaWriter() *kafka.Writer {
	if c.connManager == nil {
		return nil
	}

	return c.connManager.GetKafkaWriter()
}

// KafkaPublish publishes a message to Kafka.
func (c *Context) KafkaPublish(topic string, key, value []byte) error {
	writer := c.GetKafkaWriter()
	if writer == nil {
		return errors.New("kafka writer not initialized")
	}

	err := writer.WriteMessages(c.ctx, kafka.Message{
		Topic: topic,
		Key:   key,
		Value: value,
		Time:  time.Now(),
	})

	if err != nil && c.logger != nil {
		c.logger.Error("kafka publish failed", "topic", topic, "error", err)
	}

	return err
}

// KafkaPublishJSON publishes a JSON message to Kafka.
func (c *Context) KafkaPublishJSON(topic string, key string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return c.KafkaPublish(topic, []byte(key), data)
}

// ============================================
// PLUGIN OPERATIONS
// ============================================

// UsePlugin gets a plugin by name.
func (c *Context) UsePlugin(name string) (Plugin, error) {
	if c.pluginMgr == nil {
		return nil, errors.New("plugin manager not initialized")
	}

	plugin := c.pluginMgr.Get(name)
	if plugin == nil {
		return nil, fmt.Errorf("%w: %s", ErrPluginNotFound, name)
	}

	return plugin, nil
}

// ============================================
// METADATA & REQUEST INFO
// ============================================

// SetMetadata sets a metadata value.
func (c *Context) SetMetadata(key string, value interface{}) {
	c.metadataMu.Lock()
	defer c.metadataMu.Unlock()

	c.metadata[key] = value
}

// GetMetadata gets a metadata value.
func (c *Context) GetMetadata(key string) interface{} {
	c.metadataMu.RLock()
	defer c.metadataMu.RUnlock()

	return c.metadata[key]
}

// GetMetadataString gets a metadata value as string.
func (c *Context) GetMetadataString(key string) string {
	val := c.GetMetadata(key)
	if val == nil {
		return ""
	}

	if s, ok := val.(string); ok {
		return s
	}

	return fmt.Sprintf("%v", val)
}

// GetAllMetadata returns all metadata.
func (c *Context) GetAllMetadata() map[string]interface{} {
	c.metadataMu.RLock()
	defer c.metadataMu.RUnlock()

	result := make(map[string]interface{})
	for k, v := range c.metadata {
		result[k] = v
	}

	return result
}

// RequestID returns the request ID.
func (c *Context) RequestID() string {
	return c.requestID
}

// SetRequestID sets the request ID.
func (c *Context) SetRequestID(id string) {
	c.requestID = id
}

// StartTime returns the request start time.
func (c *Context) StartTime() time.Time {
	return c.startTime
}

// Duration returns the elapsed time since the request started.
func (c *Context) Duration() time.Duration {
	return time.Since(c.startTime)
}

// ============================================
// LOGGER ACCESS
// ============================================

// Logger returns the logger instance.
func (c *Context) Logger() Logger {
	if c.logger == nil {
		// Return no-op logger if not set
		return &noopLogger{}
	}

	return c.logger
}

// SetLogger sets the logger instance.
func (c *Context) SetLogger(logger Logger) {
	c.logger = logger
}

// ============================================
// CONTEXT METHODS
// ============================================

// Deadline returns the context deadline.
func (c *Context) Deadline() (deadline time.Time, ok bool) {
	return c.ctx.Deadline()
}

// Done returns the context done channel.
func (c *Context) Done() <-chan struct{} {
	return c.ctx.Done()
}

// Err returns the context error.
func (c *Context) Err() error {
	return c.ctx.Err()
}

// Value returns the context value.
func (c *Context) Value(key interface{}) interface{} {
	return c.ctx.Value(key)
}

// ============================================
// HELPERS
// ============================================

// generateRequestID generates a unique request ID.
func generateRequestID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), time.Now().Unix())
}

// noopLogger is a logger that does nothing.
type noopLogger struct{}

func (l *noopLogger) Debug(msg string, keysAndValues ...interface{}) {}
func (l *noopLogger) Info(msg string, keysAndValues ...interface{})  {}
func (l *noopLogger) Warn(msg string, keysAndValues ...interface{})  {}
func (l *noopLogger) Error(msg string, keysAndValues ...interface{}) {}
func (l *noopLogger) With(keysAndValues ...interface{}) Logger       { return l }
