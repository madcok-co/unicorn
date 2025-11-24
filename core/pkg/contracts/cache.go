package contracts

import (
	"context"
	"time"
)

// Cache adalah generic interface untuk semua cache operations
// Implementasi bisa Redis, Memcached, in-memory, dll
type Cache interface {
	// Basic operations
	Get(ctx context.Context, key string, dest any) error
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)

	// Multiple keys
	GetMany(ctx context.Context, keys []string) (map[string]any, error)
	SetMany(ctx context.Context, items map[string]any, ttl time.Duration) error
	DeleteMany(ctx context.Context, keys ...string) error

	// Atomic operations
	Increment(ctx context.Context, key string, delta int64) (int64, error)
	Decrement(ctx context.Context, key string, delta int64) (int64, error)

	// TTL management
	Expire(ctx context.Context, key string, ttl time.Duration) error
	TTL(ctx context.Context, key string) (time.Duration, error)

	// Pattern operations
	Keys(ctx context.Context, pattern string) ([]string, error)
	Flush(ctx context.Context) error

	// Distributed lock
	Lock(ctx context.Context, key string, ttl time.Duration) (Lock, error)

	// Remember pattern - get from cache or compute
	Remember(ctx context.Context, key string, ttl time.Duration, fn func() (any, error), dest any) error

	// Tags for cache invalidation
	Tags(tags ...string) TaggedCache

	// Connection
	Ping(ctx context.Context) error
	Close() error
}

// Lock untuk distributed locking
type Lock interface {
	Unlock(ctx context.Context) error
	Extend(ctx context.Context, ttl time.Duration) error
}

// TaggedCache untuk cache dengan tags
type TaggedCache interface {
	Get(ctx context.Context, key string, dest any) error
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	Flush(ctx context.Context) error
}

// CacheConfig untuk konfigurasi cache
type CacheConfig struct {
	Driver   string // redis, memcached, memory
	Host     string
	Port     int
	Password string
	Database int
	Prefix   string

	// Connection pool
	PoolSize     int
	MinIdleConns int

	// Timeouts
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	// Additional options
	Options map[string]string
}
