// Package redis provides a Redis implementation of the unicorn Cache interface.
//
// Usage:
//
//	import (
//	    "github.com/madcok-co/unicorn/contrib/cache/redis"
//	    goredis "github.com/redis/go-redis/v9"
//	)
//
//	rdb := goredis.NewClient(&goredis.Options{Addr: "localhost:6379"})
//	driver := redis.NewDriver(rdb)
//	app.SetCache(driver)
package redis

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/madcok-co/unicorn/core/pkg/contracts"
	"github.com/redis/go-redis/v9"
)

// Driver implements contracts.Cache using Redis
type Driver struct {
	client *redis.Client
	prefix string
}

// Option configures the Driver
type Option func(*Driver)

// WithPrefix sets a key prefix for all cache operations
func WithPrefix(prefix string) Option {
	return func(d *Driver) {
		d.prefix = prefix
	}
}

// NewDriver creates a new Redis cache driver
func NewDriver(client *redis.Client, opts ...Option) *Driver {
	d := &Driver{client: client}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Client returns the underlying Redis client
func (d *Driver) Client() *redis.Client {
	return d.client
}

func (d *Driver) key(k string) string {
	if d.prefix == "" {
		return k
	}
	return d.prefix + ":" + k
}

// Get retrieves a value from cache
func (d *Driver) Get(ctx context.Context, key string, dest any) error {
	val, err := d.client.Get(ctx, d.key(key)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return errors.New("cache: key not found")
		}
		return err
	}
	return json.Unmarshal(val, dest)
}

// Set stores a value in cache with TTL
func (d *Driver) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return d.client.Set(ctx, d.key(key), data, ttl).Err()
}

// Delete removes a key from cache
func (d *Driver) Delete(ctx context.Context, key string) error {
	return d.client.Del(ctx, d.key(key)).Err()
}

// Exists checks if a key exists in cache
func (d *Driver) Exists(ctx context.Context, key string) (bool, error) {
	result, err := d.client.Exists(ctx, d.key(key)).Result()
	return result > 0, err
}

// GetMany retrieves multiple values from cache
func (d *Driver) GetMany(ctx context.Context, keys []string) (map[string]any, error) {
	prefixedKeys := make([]string, len(keys))
	for i, k := range keys {
		prefixedKeys[i] = d.key(k)
	}

	values, err := d.client.MGet(ctx, prefixedKeys...).Result()
	if err != nil {
		return nil, err
	}

	result := make(map[string]any)
	for i, val := range values {
		if val != nil {
			var decoded any
			if str, ok := val.(string); ok {
				if err := json.Unmarshal([]byte(str), &decoded); err == nil {
					result[keys[i]] = decoded
				}
			}
		}
	}
	return result, nil
}

// SetMany stores multiple values in cache
func (d *Driver) SetMany(ctx context.Context, items map[string]any, ttl time.Duration) error {
	pipe := d.client.Pipeline()
	for key, value := range items {
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		pipe.Set(ctx, d.key(key), data, ttl)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// DeleteMany removes multiple keys from cache
func (d *Driver) DeleteMany(ctx context.Context, keys ...string) error {
	prefixedKeys := make([]string, len(keys))
	for i, k := range keys {
		prefixedKeys[i] = d.key(k)
	}
	return d.client.Del(ctx, prefixedKeys...).Err()
}

// Increment atomically increments a value
func (d *Driver) Increment(ctx context.Context, key string, delta int64) (int64, error) {
	return d.client.IncrBy(ctx, d.key(key), delta).Result()
}

// Decrement atomically decrements a value
func (d *Driver) Decrement(ctx context.Context, key string, delta int64) (int64, error) {
	return d.client.DecrBy(ctx, d.key(key), delta).Result()
}

// Expire sets TTL on a key
func (d *Driver) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return d.client.Expire(ctx, d.key(key), ttl).Err()
}

// TTL returns remaining TTL for a key
func (d *Driver) TTL(ctx context.Context, key string) (time.Duration, error) {
	return d.client.TTL(ctx, d.key(key)).Result()
}

// Keys returns all keys matching pattern
func (d *Driver) Keys(ctx context.Context, pattern string) ([]string, error) {
	return d.client.Keys(ctx, d.key(pattern)).Result()
}

// Flush removes all keys (use with caution!)
func (d *Driver) Flush(ctx context.Context) error {
	return d.client.FlushDB(ctx).Err()
}

// Lock acquires a distributed lock
func (d *Driver) Lock(ctx context.Context, key string, ttl time.Duration) (contracts.Lock, error) {
	lockKey := d.key("lock:" + key)

	// Try to acquire lock using SET NX
	ok, err := d.client.SetNX(ctx, lockKey, "1", ttl).Result()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("cache: failed to acquire lock")
	}

	return &Lock{
		client: d.client,
		key:    lockKey,
	}, nil
}

// Remember gets from cache or computes and stores
func (d *Driver) Remember(ctx context.Context, key string, ttl time.Duration, fn func() (any, error), dest any) error {
	// Try to get from cache first
	err := d.Get(ctx, key, dest)
	if err == nil {
		return nil
	}

	// Compute value
	value, err := fn()
	if err != nil {
		return err
	}

	// Store in cache
	if err := d.Set(ctx, key, value, ttl); err != nil {
		return err
	}

	// Set dest to computed value
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

// Tags returns a tagged cache instance
func (d *Driver) Tags(tags ...string) contracts.TaggedCache {
	return &TaggedCache{
		driver: d,
		tags:   tags,
	}
}

// Ping checks Redis connectivity
func (d *Driver) Ping(ctx context.Context) error {
	return d.client.Ping(ctx).Err()
}

// Close closes the Redis connection
func (d *Driver) Close() error {
	return d.client.Close()
}

// Lock implements contracts.Lock
type Lock struct {
	client *redis.Client
	key    string
}

func (l *Lock) Unlock(ctx context.Context) error {
	return l.client.Del(ctx, l.key).Err()
}

func (l *Lock) Extend(ctx context.Context, ttl time.Duration) error {
	return l.client.Expire(ctx, l.key, ttl).Err()
}

// TaggedCache implements contracts.TaggedCache
type TaggedCache struct {
	driver *Driver
	tags   []string
}

func (t *TaggedCache) tagKey(key string) string {
	// Combine tags with key for namespacing
	tagPrefix := ""
	for _, tag := range t.tags {
		tagPrefix += tag + ":"
	}
	return tagPrefix + key
}

func (t *TaggedCache) Get(ctx context.Context, key string, dest any) error {
	return t.driver.Get(ctx, t.tagKey(key), dest)
}

func (t *TaggedCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	fullKey := t.tagKey(key)

	// Store the key in tag sets for later invalidation
	for _, tag := range t.tags {
		t.driver.client.SAdd(ctx, t.driver.key("tag:"+tag), fullKey)
	}

	return t.driver.Set(ctx, fullKey, value, ttl)
}

func (t *TaggedCache) Flush(ctx context.Context) error {
	// Delete all keys associated with these tags
	for _, tag := range t.tags {
		keys, err := t.driver.client.SMembers(ctx, t.driver.key("tag:"+tag)).Result()
		if err != nil {
			continue
		}
		if len(keys) > 0 {
			t.driver.client.Del(ctx, keys...)
		}
		t.driver.client.Del(ctx, t.driver.key("tag:"+tag))
	}
	return nil
}

// Ensure Driver implements contracts.Cache
var _ contracts.Cache = (*Driver)(nil)
