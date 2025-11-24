// Package cache provides a generic cache adapter
// that wraps any cache library (go-redis, memcache, bigcache, etc.)
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

// Driver is the interface that any cache driver must implement
type Driver interface {
	// Get retrieves value by key, returns ErrNotFound if not exists
	Get(ctx context.Context, key string) ([]byte, error)
	// Set stores value with TTL
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	// Delete removes key
	Delete(ctx context.Context, key string) error
	// Exists checks if key exists
	Exists(ctx context.Context, key string) (bool, error)
	// Ping checks connection
	Ping(ctx context.Context) error
	// Close closes connection
	Close() error
}

// MultiKeyDriver extends Driver with multi-key operations
type MultiKeyDriver interface {
	Driver
	GetMany(ctx context.Context, keys []string) (map[string][]byte, error)
	SetMany(ctx context.Context, items map[string][]byte, ttl time.Duration) error
	DeleteMany(ctx context.Context, keys []string) error
}

// AtomicDriver extends Driver with atomic operations
type AtomicDriver interface {
	Driver
	Increment(ctx context.Context, key string, delta int64) (int64, error)
	Decrement(ctx context.Context, key string, delta int64) (int64, error)
}

// TTLDriver extends Driver with TTL operations
type TTLDriver interface {
	Driver
	Expire(ctx context.Context, key string, ttl time.Duration) error
	TTL(ctx context.Context, key string) (time.Duration, error)
}

// PatternDriver extends Driver with pattern operations
type PatternDriver interface {
	Driver
	Keys(ctx context.Context, pattern string) ([]string, error)
	Flush(ctx context.Context) error
}

// LockDriver extends Driver with distributed locking
type LockDriver interface {
	Driver
	Lock(ctx context.Context, key string, ttl time.Duration) (contracts.Lock, error)
}

// ErrNotFound is returned when key is not found
var ErrNotFound = fmt.Errorf("key not found")

// Adapter implements contracts.Cache
type Adapter struct {
	driver     Driver
	config     *contracts.CacheConfig
	serializer Serializer
}

// Serializer handles serialization/deserialization
type Serializer interface {
	Marshal(v any) ([]byte, error)
	Unmarshal(data []byte, v any) error
}

// JSONSerializer is the default serializer
type JSONSerializer struct{}

func (j *JSONSerializer) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func (j *JSONSerializer) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

// New creates a new cache adapter
func New(driver Driver) *Adapter {
	return &Adapter{
		driver:     driver,
		serializer: &JSONSerializer{},
	}
}

// WithSerializer sets custom serializer
func (a *Adapter) WithSerializer(s Serializer) *Adapter {
	a.serializer = s
	return a
}

// WithConfig sets configuration
func (a *Adapter) WithConfig(config *contracts.CacheConfig) *Adapter {
	a.config = config
	return a
}

// ============ contracts.Cache Implementation ============

// Get retrieves value by key
func (a *Adapter) Get(ctx context.Context, key string, dest any) error {
	data, err := a.driver.Get(ctx, a.prefixKey(key))
	if err != nil {
		return err
	}
	return a.serializer.Unmarshal(data, dest)
}

// Set stores value with TTL
func (a *Adapter) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	data, err := a.serializer.Marshal(value)
	if err != nil {
		return err
	}
	return a.driver.Set(ctx, a.prefixKey(key), data, ttl)
}

// Delete removes key
func (a *Adapter) Delete(ctx context.Context, key string) error {
	return a.driver.Delete(ctx, a.prefixKey(key))
}

// Exists checks if key exists
func (a *Adapter) Exists(ctx context.Context, key string) (bool, error) {
	return a.driver.Exists(ctx, a.prefixKey(key))
}

// GetMany retrieves multiple keys
func (a *Adapter) GetMany(ctx context.Context, keys []string) (map[string]any, error) {
	multiDriver, ok := a.driver.(MultiKeyDriver)
	if !ok {
		return a.getManyFallback(ctx, keys)
	}

	prefixedKeys := make([]string, len(keys))
	for i, k := range keys {
		prefixedKeys[i] = a.prefixKey(k)
	}

	data, err := multiDriver.GetMany(ctx, prefixedKeys)
	if err != nil {
		return nil, err
	}

	result := make(map[string]any)
	prefix := a.getPrefix()
	for k, v := range data {
		var val any
		if err := a.serializer.Unmarshal(v, &val); err == nil {
			// Remove prefix from key
			if len(k) > len(prefix) {
				k = k[len(prefix):]
			}
			result[k] = val
		}
	}
	return result, nil
}

func (a *Adapter) getManyFallback(ctx context.Context, keys []string) (map[string]any, error) {
	result := make(map[string]any)
	for _, key := range keys {
		var val any
		if err := a.Get(ctx, key, &val); err == nil {
			result[key] = val
		}
	}
	return result, nil
}

// SetMany stores multiple key-value pairs
func (a *Adapter) SetMany(ctx context.Context, items map[string]any, ttl time.Duration) error {
	multiDriver, ok := a.driver.(MultiKeyDriver)
	if !ok {
		return a.setManyFallback(ctx, items, ttl)
	}

	data := make(map[string][]byte)
	for k, v := range items {
		bytes, err := a.serializer.Marshal(v)
		if err != nil {
			return err
		}
		data[a.prefixKey(k)] = bytes
	}

	return multiDriver.SetMany(ctx, data, ttl)
}

func (a *Adapter) setManyFallback(ctx context.Context, items map[string]any, ttl time.Duration) error {
	for k, v := range items {
		if err := a.Set(ctx, k, v, ttl); err != nil {
			return err
		}
	}
	return nil
}

// DeleteMany removes multiple keys
func (a *Adapter) DeleteMany(ctx context.Context, keys ...string) error {
	multiDriver, ok := a.driver.(MultiKeyDriver)
	if !ok {
		return a.deleteManyFallback(ctx, keys)
	}

	prefixedKeys := make([]string, len(keys))
	for i, k := range keys {
		prefixedKeys[i] = a.prefixKey(k)
	}

	return multiDriver.DeleteMany(ctx, prefixedKeys)
}

func (a *Adapter) deleteManyFallback(ctx context.Context, keys []string) error {
	for _, k := range keys {
		if err := a.Delete(ctx, k); err != nil {
			return err
		}
	}
	return nil
}

// Increment atomically increments value
func (a *Adapter) Increment(ctx context.Context, key string, delta int64) (int64, error) {
	atomicDriver, ok := a.driver.(AtomicDriver)
	if !ok {
		return 0, fmt.Errorf("atomic operations not supported")
	}
	return atomicDriver.Increment(ctx, a.prefixKey(key), delta)
}

// Decrement atomically decrements value
func (a *Adapter) Decrement(ctx context.Context, key string, delta int64) (int64, error) {
	atomicDriver, ok := a.driver.(AtomicDriver)
	if !ok {
		return 0, fmt.Errorf("atomic operations not supported")
	}
	return atomicDriver.Decrement(ctx, a.prefixKey(key), delta)
}

// Expire sets TTL on existing key
func (a *Adapter) Expire(ctx context.Context, key string, ttl time.Duration) error {
	ttlDriver, ok := a.driver.(TTLDriver)
	if !ok {
		return fmt.Errorf("TTL operations not supported")
	}
	return ttlDriver.Expire(ctx, a.prefixKey(key), ttl)
}

// TTL gets remaining TTL
func (a *Adapter) TTL(ctx context.Context, key string) (time.Duration, error) {
	ttlDriver, ok := a.driver.(TTLDriver)
	if !ok {
		return 0, fmt.Errorf("TTL operations not supported")
	}
	return ttlDriver.TTL(ctx, a.prefixKey(key))
}

// Keys returns keys matching pattern
func (a *Adapter) Keys(ctx context.Context, pattern string) ([]string, error) {
	patternDriver, ok := a.driver.(PatternDriver)
	if !ok {
		return nil, fmt.Errorf("pattern operations not supported")
	}
	return patternDriver.Keys(ctx, a.prefixKey(pattern))
}

// Flush clears all keys
func (a *Adapter) Flush(ctx context.Context) error {
	patternDriver, ok := a.driver.(PatternDriver)
	if !ok {
		return fmt.Errorf("flush not supported")
	}
	return patternDriver.Flush(ctx)
}

// Lock acquires a distributed lock
func (a *Adapter) Lock(ctx context.Context, key string, ttl time.Duration) (contracts.Lock, error) {
	lockDriver, ok := a.driver.(LockDriver)
	if !ok {
		return nil, fmt.Errorf("locking not supported")
	}
	return lockDriver.Lock(ctx, a.prefixKey(key), ttl)
}

// Remember gets from cache or computes and stores
func (a *Adapter) Remember(ctx context.Context, key string, ttl time.Duration, fn func() (any, error), dest any) error {
	// Try to get from cache
	err := a.Get(ctx, key, dest)
	if err == nil {
		return nil
	}

	// Compute value
	value, err := fn()
	if err != nil {
		return err
	}

	// Store in cache
	if err := a.Set(ctx, key, value, ttl); err != nil {
		return err
	}

	// Copy to dest
	data, _ := a.serializer.Marshal(value)
	return a.serializer.Unmarshal(data, dest)
}

// Tags returns a tagged cache
func (a *Adapter) Tags(tags ...string) contracts.TaggedCache {
	return &taggedCache{
		adapter: a,
		tags:    tags,
	}
}

// Ping checks connection
func (a *Adapter) Ping(ctx context.Context) error {
	return a.driver.Ping(ctx)
}

// Close closes connection
func (a *Adapter) Close() error {
	return a.driver.Close()
}

// Helper methods
func (a *Adapter) prefixKey(key string) string {
	return a.getPrefix() + key
}

func (a *Adapter) getPrefix() string {
	if a.config != nil && a.config.Prefix != "" {
		return a.config.Prefix + ":"
	}
	return ""
}

// taggedCache implements contracts.TaggedCache
type taggedCache struct {
	adapter *Adapter
	tags    []string
}

func (t *taggedCache) Get(ctx context.Context, key string, dest any) error {
	return t.adapter.Get(ctx, t.taggedKey(key), dest)
}

func (t *taggedCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	return t.adapter.Set(ctx, t.taggedKey(key), value, ttl)
}

func (t *taggedCache) Flush(ctx context.Context) error {
	// Flush all keys with these tags
	for _, tag := range t.tags {
		keys, err := t.adapter.Keys(ctx, "tag:"+tag+":*")
		if err != nil {
			continue
		}
		_ = t.adapter.DeleteMany(ctx, keys...)
	}
	return nil
}

func (t *taggedCache) taggedKey(key string) string {
	if len(t.tags) == 0 {
		return key
	}
	return "tag:" + t.tags[0] + ":" + key
}

// ============ In-Memory Cache Driver ============

// MemoryDriver is an in-memory cache implementation
type MemoryDriver struct {
	data    map[string]*cacheItem
	mu      sync.RWMutex
	stopCh  chan struct{}
	running bool
}

type cacheItem struct {
	value     []byte
	expiresAt time.Time
}

// NewMemoryDriver creates an in-memory cache
func NewMemoryDriver() *MemoryDriver {
	m := &MemoryDriver{
		data:   make(map[string]*cacheItem),
		stopCh: make(chan struct{}),
	}
	m.startCleanup()
	return m
}

func (m *MemoryDriver) startCleanup() {
	m.running = true
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-m.stopCh:
				return
			case <-ticker.C:
				m.cleanup()
			}
		}
	}()
}

func (m *MemoryDriver) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for k, v := range m.data {
		if !v.expiresAt.IsZero() && now.After(v.expiresAt) {
			delete(m.data, k)
		}
	}
}

func (m *MemoryDriver) Get(ctx context.Context, key string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	item, ok := m.data[key]
	if !ok {
		return nil, ErrNotFound
	}

	if !item.expiresAt.IsZero() && time.Now().After(item.expiresAt) {
		return nil, ErrNotFound
	}

	return item.value, nil
}

func (m *MemoryDriver) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	item := &cacheItem{value: value}
	if ttl > 0 {
		item.expiresAt = time.Now().Add(ttl)
	}
	m.data[key] = item
	return nil
}

func (m *MemoryDriver) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}

func (m *MemoryDriver) Exists(ctx context.Context, key string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	item, ok := m.data[key]
	if !ok {
		return false, nil
	}

	if !item.expiresAt.IsZero() && time.Now().After(item.expiresAt) {
		return false, nil
	}

	return true, nil
}

func (m *MemoryDriver) Ping(ctx context.Context) error {
	return nil
}

func (m *MemoryDriver) Close() error {
	if m.running {
		close(m.stopCh)
		m.running = false
	}
	return nil
}

// Implement MultiKeyDriver
func (m *MemoryDriver) GetMany(ctx context.Context, keys []string) (map[string][]byte, error) {
	result := make(map[string][]byte)
	for _, key := range keys {
		if data, err := m.Get(ctx, key); err == nil {
			result[key] = data
		}
	}
	return result, nil
}

func (m *MemoryDriver) SetMany(ctx context.Context, items map[string][]byte, ttl time.Duration) error {
	for k, v := range items {
		if err := m.Set(ctx, k, v, ttl); err != nil {
			return err
		}
	}
	return nil
}

func (m *MemoryDriver) DeleteMany(ctx context.Context, keys []string) error {
	for _, k := range keys {
		_ = m.Delete(ctx, k)
	}
	return nil
}

// Implement AtomicDriver
func (m *MemoryDriver) Increment(ctx context.Context, key string, delta int64) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	item, ok := m.data[key]
	var current int64
	if ok {
		_ = json.Unmarshal(item.value, &current)
	}

	current += delta
	data, _ := json.Marshal(current)
	m.data[key] = &cacheItem{value: data}
	return current, nil
}

func (m *MemoryDriver) Decrement(ctx context.Context, key string, delta int64) (int64, error) {
	return m.Increment(ctx, key, -delta)
}

// Implement PatternDriver
func (m *MemoryDriver) Keys(ctx context.Context, pattern string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keys := make([]string, 0)
	for k := range m.data {
		// Simple pattern matching (only supports * at end)
		if matchPattern(pattern, k) {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

func (m *MemoryDriver) Flush(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = make(map[string]*cacheItem)
	return nil
}

func matchPattern(pattern, key string) bool {
	if pattern == "*" {
		return true
	}
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(key) >= len(prefix) && key[:len(prefix)] == prefix
	}
	return pattern == key
}

// ============ Redis Driver Interface ============

// RedisClient is the interface that redis clients implement
// Compatible with go-redis/redis
type RedisClient interface {
	Get(ctx context.Context, key string) StringCmd
	Set(ctx context.Context, key string, value any, ttl time.Duration) StatusCmd
	Del(ctx context.Context, keys ...string) IntCmd
	Exists(ctx context.Context, keys ...string) IntCmd
	Ping(ctx context.Context) StatusCmd
	Close() error
	MGet(ctx context.Context, keys ...string) SliceCmd
	MSet(ctx context.Context, values ...any) StatusCmd
	IncrBy(ctx context.Context, key string, value int64) IntCmd
	DecrBy(ctx context.Context, key string, value int64) IntCmd
	Expire(ctx context.Context, key string, ttl time.Duration) BoolCmd
	TTL(ctx context.Context, key string) DurationCmd
	Keys(ctx context.Context, pattern string) StringSliceCmd
	FlushDB(ctx context.Context) StatusCmd
}

// Command result interfaces
type StringCmd interface {
	Result() (string, error)
	Bytes() ([]byte, error)
}

type StatusCmd interface {
	Err() error
}

type IntCmd interface {
	Result() (int64, error)
}

type BoolCmd interface {
	Result() (bool, error)
}

type DurationCmd interface {
	Result() (time.Duration, error)
}

type SliceCmd interface {
	Result() ([]any, error)
}

type StringSliceCmd interface {
	Result() ([]string, error)
}

// RedisDriver wraps a Redis client
type RedisDriver struct {
	client RedisClient
}

// WrapRedis wraps a Redis client to implement Driver
func WrapRedis(client RedisClient) *RedisDriver {
	return &RedisDriver{client: client}
}

func (r *RedisDriver) Get(ctx context.Context, key string) ([]byte, error) {
	data, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, ErrNotFound
	}
	return data, nil
}

func (r *RedisDriver) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

func (r *RedisDriver) Delete(ctx context.Context, key string) error {
	_, err := r.client.Del(ctx, key).Result()
	return err
}

func (r *RedisDriver) Exists(ctx context.Context, key string) (bool, error) {
	n, err := r.client.Exists(ctx, key).Result()
	return n > 0, err
}

func (r *RedisDriver) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

func (r *RedisDriver) Close() error {
	return r.client.Close()
}

func (r *RedisDriver) GetMany(ctx context.Context, keys []string) (map[string][]byte, error) {
	vals, err := r.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	result := make(map[string][]byte)
	for i, val := range vals {
		if val != nil {
			if str, ok := val.(string); ok {
				result[keys[i]] = []byte(str)
			}
		}
	}
	return result, nil
}

func (r *RedisDriver) SetMany(ctx context.Context, items map[string][]byte, ttl time.Duration) error {
	values := make([]any, 0, len(items)*2)
	for k, v := range items {
		values = append(values, k, v)
	}
	return r.client.MSet(ctx, values...).Err()
}

func (r *RedisDriver) DeleteMany(ctx context.Context, keys []string) error {
	_, err := r.client.Del(ctx, keys...).Result()
	return err
}

func (r *RedisDriver) Increment(ctx context.Context, key string, delta int64) (int64, error) {
	return r.client.IncrBy(ctx, key, delta).Result()
}

func (r *RedisDriver) Decrement(ctx context.Context, key string, delta int64) (int64, error) {
	return r.client.DecrBy(ctx, key, delta).Result()
}

func (r *RedisDriver) Expire(ctx context.Context, key string, ttl time.Duration) error {
	_, err := r.client.Expire(ctx, key, ttl).Result()
	return err
}

func (r *RedisDriver) TTL(ctx context.Context, key string) (time.Duration, error) {
	return r.client.TTL(ctx, key).Result()
}

func (r *RedisDriver) Keys(ctx context.Context, pattern string) ([]string, error) {
	return r.client.Keys(ctx, pattern).Result()
}

func (r *RedisDriver) Flush(ctx context.Context) error {
	return r.client.FlushDB(ctx).Err()
}
