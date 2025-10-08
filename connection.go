// ============================================
// UNICORN Framework - Connection Manager
// Thread-safe, no resource leaks, production-ready
// ============================================

package unicorn

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"

	// Database drivers
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

// Default connection pool settings
const (
	DefaultMaxOpenConns    = 25
	DefaultMaxIdleConns    = 5
	DefaultConnMaxLifetime = 5 * time.Minute
	DefaultConnectTimeout  = 5 * time.Second
)

// Common errors
var (
	ErrAlreadyInitialized = errors.New("connection manager already initialized")
	ErrNotInitialized     = errors.New("connection manager not initialized")
	ErrDatabaseNotFound   = errors.New("database not found")
	ErrRedisNotFound      = errors.New("redis instance not found")
)

// DatabaseConfig holds configuration for a single database connection.
type DatabaseConfig struct {
	Name     string `yaml:"name"`
	Driver   string `yaml:"driver"` // postgres, mysql, sqlite
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
	SSLMode  string `yaml:"ssl_mode"` // PostgreSQL: disable, require, verify-full
	MaxOpen  int    `yaml:"max_open_conns"`
	MaxIdle  int    `yaml:"max_idle_conns"`
	MaxLife  int    `yaml:"max_lifetime_seconds"`
}

// DSN builds a Data Source Name from the config.
func (c *DatabaseConfig) DSN() string {
	switch c.Driver {
	case "postgres":
		sslMode := c.SSLMode
		if sslMode == "" {
			sslMode = "disable"
		}
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			c.Host, c.Port, c.Username, c.Password, c.Database, sslMode)
	case "mysql":
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
			c.Username, c.Password, c.Host, c.Port, c.Database)
	default:
		return ""
	}
}

// RedisConfig holds configuration for a single Redis instance.
type RedisConfig struct {
	Name     string `yaml:"name"`
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

// KafkaConfig holds configuration for Kafka.
type KafkaConfig struct {
	Brokers []string `yaml:"brokers"`
	GroupID string   `yaml:"group_id"`
}

// ConnectionManager manages all external connections with thread-safety.
// It supports multiple databases and Redis instances with proper resource cleanup.
type ConnectionManager struct {
	databases    map[string]*sql.DB
	redisClients map[string]*redis.Client
	kafkaWriter  *kafka.Writer
	defaultDB    string
	defaultRedis string
	initialized  bool
	mu           sync.RWMutex
}

// NewConnectionManager creates a new connection manager.
func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		databases:    make(map[string]*sql.DB),
		redisClients: make(map[string]*redis.Client),
	}
}

// InitializeDatabases initializes database connections from config.
func (cm *ConnectionManager) InitializeDatabases(configs []DatabaseConfig) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.initialized {
		return ErrAlreadyInitialized
	}

	for i, cfg := range configs {
		if err := cm.initializeDatabase(cfg); err != nil {
			// Clean up any successful connections before returning
			cm.cleanup()
			return fmt.Errorf("failed to initialize database %d (%s): %w", i, cfg.Name, err)
		}

		// Set first database as default
		if cm.defaultDB == "" {
			cm.defaultDB = cfg.Name
		}
	}

	return nil
}

// initializeDatabase initializes a single database connection.
func (cm *ConnectionManager) initializeDatabase(cfg DatabaseConfig) error {
	if cfg.Name == "" {
		return errors.New("database name is required")
	}

	if cfg.Driver == "" {
		return errors.New("database driver is required")
	}

	dsn := cfg.DSN()
	if dsn == "" {
		return errors.New("invalid database configuration")
	}

	db, err := sql.Open(cfg.Driver, dsn)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Set connection pool settings with defaults
	maxOpen := cfg.MaxOpen
	if maxOpen <= 0 {
		maxOpen = DefaultMaxOpenConns
	}

	maxIdle := cfg.MaxIdle
	if maxIdle <= 0 {
		maxIdle = DefaultMaxIdleConns
	}

	maxLife := time.Duration(cfg.MaxLife) * time.Second
	if maxLife <= 0 {
		maxLife = DefaultConnMaxLifetime
	}

	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(maxLife)

	// Test connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), DefaultConnectTimeout)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		// Close the connection before returning error
		db.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}

	cm.databases[cfg.Name] = db

	return nil
}

// InitializeRedis initializes Redis connections from config.
func (cm *ConnectionManager) InitializeRedis(configs []RedisConfig) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for i, cfg := range configs {
		if err := cm.initializeRedisInstance(cfg); err != nil {
			// Clean up any successful connections before returning
			cm.cleanup()
			return fmt.Errorf("failed to initialize redis %d (%s): %w", i, cfg.Name, err)
		}

		// Set first Redis as default
		if cm.defaultRedis == "" {
			cm.defaultRedis = cfg.Name
		}
	}

	return nil
}

// initializeRedisInstance initializes a single Redis connection.
func (cm *ConnectionManager) initializeRedisInstance(cfg RedisConfig) error {
	if cfg.Name == "" {
		return errors.New("redis name is required")
	}

	if cfg.Addr == "" {
		return errors.New("redis address is required")
	}

	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
		MinIdleConns: 5,
	})

	// Test connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), DefaultConnectTimeout)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		// Close the client before returning error
		client.Close()
		return fmt.Errorf("failed to ping redis: %w", err)
	}

	cm.redisClients[cfg.Name] = client

	return nil
}

// InitializeKafka initializes Kafka writer from config.
func (cm *ConnectionManager) InitializeKafka(cfg KafkaConfig) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if len(cfg.Brokers) == 0 {
		return nil
	}

	cm.kafkaWriter = &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	return nil
}

// MarkInitialized marks the connection manager as initialized.
func (cm *ConnectionManager) MarkInitialized() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.initialized = true
}

// GetDB returns a database connection by name.
// If name is empty, it returns the default database.
// It returns nil if the database doesn't exist.
func (cm *ConnectionManager) GetDB(name string) *sql.DB {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if !cm.initialized {
		return nil
	}

	if name == "" {
		name = cm.defaultDB
	}

	return cm.databases[name]
}

// GetRedis returns a Redis client by name.
// If name is empty, it returns the default Redis instance.
// It returns nil if the Redis instance doesn't exist.
func (cm *ConnectionManager) GetRedis(name string) *redis.Client {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if !cm.initialized {
		return nil
	}

	if name == "" {
		name = cm.defaultRedis
	}

	return cm.redisClients[name]
}

// GetAllDBNames returns all configured database names.
// The returned slice is a copy and safe to modify.
func (cm *ConnectionManager) GetAllDBNames() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	names := make([]string, 0, len(cm.databases))
	for name := range cm.databases {
		names = append(names, name)
	}

	return names
}

// GetAllRedisNames returns all configured Redis instance names.
// The returned slice is a copy and safe to modify.
func (cm *ConnectionManager) GetAllRedisNames() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	names := make([]string, 0, len(cm.redisClients))
	for name := range cm.redisClients {
		names = append(names, name)
	}

	return names
}

// GetKafkaWriter returns the Kafka writer.
// It may be nil if Kafka is not configured.
func (cm *ConnectionManager) GetKafkaWriter() *kafka.Writer {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return cm.kafkaWriter
}

// Close closes all connections gracefully.
// It waits for all connections to close and returns any errors encountered.
func (cm *ConnectionManager) Close() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	return cm.cleanup()
}

// cleanup closes all connections. Must be called with lock held.
func (cm *ConnectionManager) cleanup() error {
	var errs []error

	// Close all databases
	for name, db := range cm.databases {
		if err := db.Close(); err != nil {
			errs = append(errs, fmt.Errorf("error closing database %s: %w", name, err))
		}
	}
	cm.databases = make(map[string]*sql.DB)

	// Close all Redis clients
	for name, client := range cm.redisClients {
		if err := client.Close(); err != nil {
			errs = append(errs, fmt.Errorf("error closing redis %s: %w", name, err))
		}
	}
	cm.redisClients = make(map[string]*redis.Client)

	// Close Kafka writer
	if cm.kafkaWriter != nil {
		if err := cm.kafkaWriter.Close(); err != nil {
			errs = append(errs, fmt.Errorf("error closing kafka: %w", err))
		}
		cm.kafkaWriter = nil
	}

	cm.initialized = false

	if len(errs) > 0 {
		return fmt.Errorf("connection cleanup errors: %v", errs)
	}

	return nil
}

// Stats returns connection statistics for monitoring.
type Stats struct {
	DatabaseCount int
	RedisCount    int
	Initialized   bool
}

// GetStats returns current connection statistics.
func (cm *ConnectionManager) GetStats() Stats {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return Stats{
		DatabaseCount: len(cm.databases),
		RedisCount:    len(cm.redisClients),
		Initialized:   cm.initialized,
	}
}
