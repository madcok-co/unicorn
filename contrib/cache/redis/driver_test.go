package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/madcok-co/unicorn/core/pkg/contracts"
	"github.com/redis/go-redis/v9"
)

func setupTestRedis(t *testing.T) (*miniredis.Miniredis, *Driver) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	return mr, NewDriver(client)
}

func TestNewDriver(t *testing.T) {
	mr, driver := setupTestRedis(t)
	defer mr.Close()

	if driver == nil {
		t.Fatal("driver should not be nil")
	}
	if driver.client == nil {
		t.Error("client should not be nil")
	}
}

func TestNewDriverWithPrefix(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	driver := NewDriver(client, WithPrefix("myapp"))

	if driver.prefix != "myapp" {
		t.Errorf("expected prefix 'myapp', got %s", driver.prefix)
	}

	// Test that prefix is applied
	ctx := context.Background()
	driver.Set(ctx, "key", "value", time.Minute)

	// Check key in miniredis
	if !mr.Exists("myapp:key") {
		t.Error("key should have prefix")
	}
}

func TestDriver_Client(t *testing.T) {
	mr, driver := setupTestRedis(t)
	defer mr.Close()

	if driver.Client() == nil {
		t.Error("Client() should return the underlying client")
	}
}

func TestDriver_SetAndGet(t *testing.T) {
	mr, driver := setupTestRedis(t)
	defer mr.Close()
	ctx := context.Background()

	t.Run("string value", func(t *testing.T) {
		err := driver.Set(ctx, "name", "John", time.Minute)
		if err != nil {
			t.Fatalf("Set error: %v", err)
		}

		var result string
		err = driver.Get(ctx, "name", &result)
		if err != nil {
			t.Fatalf("Get error: %v", err)
		}
		if result != "John" {
			t.Errorf("expected 'John', got %s", result)
		}
	})

	t.Run("struct value", func(t *testing.T) {
		type User struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		}

		user := User{Name: "Jane", Email: "jane@example.com"}
		err := driver.Set(ctx, "user", user, time.Minute)
		if err != nil {
			t.Fatalf("Set error: %v", err)
		}

		var result User
		err = driver.Get(ctx, "user", &result)
		if err != nil {
			t.Fatalf("Get error: %v", err)
		}
		if result.Name != "Jane" || result.Email != "jane@example.com" {
			t.Errorf("unexpected result: %+v", result)
		}
	})

	t.Run("key not found", func(t *testing.T) {
		var result string
		err := driver.Get(ctx, "nonexistent", &result)
		if err == nil {
			t.Error("should return error for nonexistent key")
		}
	})
}

func TestDriver_Delete(t *testing.T) {
	mr, driver := setupTestRedis(t)
	defer mr.Close()
	ctx := context.Background()

	driver.Set(ctx, "key", "value", time.Minute)

	err := driver.Delete(ctx, "key")
	if err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	exists, _ := driver.Exists(ctx, "key")
	if exists {
		t.Error("key should be deleted")
	}
}

func TestDriver_Exists(t *testing.T) {
	mr, driver := setupTestRedis(t)
	defer mr.Close()
	ctx := context.Background()

	exists, err := driver.Exists(ctx, "key")
	if err != nil {
		t.Fatalf("Exists error: %v", err)
	}
	if exists {
		t.Error("key should not exist")
	}

	driver.Set(ctx, "key", "value", time.Minute)

	exists, err = driver.Exists(ctx, "key")
	if err != nil {
		t.Fatalf("Exists error: %v", err)
	}
	if !exists {
		t.Error("key should exist")
	}
}

func TestDriver_GetMany(t *testing.T) {
	mr, driver := setupTestRedis(t)
	defer mr.Close()
	ctx := context.Background()

	driver.Set(ctx, "key1", "value1", time.Minute)
	driver.Set(ctx, "key2", "value2", time.Minute)

	result, err := driver.GetMany(ctx, []string{"key1", "key2", "key3"})
	if err != nil {
		t.Fatalf("GetMany error: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 results, got %d", len(result))
	}
}

func TestDriver_SetMany(t *testing.T) {
	mr, driver := setupTestRedis(t)
	defer mr.Close()
	ctx := context.Background()

	items := map[string]any{
		"a": "value_a",
		"b": "value_b",
		"c": "value_c",
	}

	err := driver.SetMany(ctx, items, time.Minute)
	if err != nil {
		t.Fatalf("SetMany error: %v", err)
	}

	for key := range items {
		exists, _ := driver.Exists(ctx, key)
		if !exists {
			t.Errorf("key %s should exist", key)
		}
	}
}

func TestDriver_DeleteMany(t *testing.T) {
	mr, driver := setupTestRedis(t)
	defer mr.Close()
	ctx := context.Background()

	driver.Set(ctx, "a", "1", time.Minute)
	driver.Set(ctx, "b", "2", time.Minute)
	driver.Set(ctx, "c", "3", time.Minute)

	err := driver.DeleteMany(ctx, "a", "b")
	if err != nil {
		t.Fatalf("DeleteMany error: %v", err)
	}

	exists, _ := driver.Exists(ctx, "a")
	if exists {
		t.Error("key 'a' should be deleted")
	}
	exists, _ = driver.Exists(ctx, "b")
	if exists {
		t.Error("key 'b' should be deleted")
	}
	exists, _ = driver.Exists(ctx, "c")
	if !exists {
		t.Error("key 'c' should still exist")
	}
}

func TestDriver_Increment(t *testing.T) {
	mr, driver := setupTestRedis(t)
	defer mr.Close()
	ctx := context.Background()

	result, err := driver.Increment(ctx, "counter", 1)
	if err != nil {
		t.Fatalf("Increment error: %v", err)
	}
	if result != 1 {
		t.Errorf("expected 1, got %d", result)
	}

	result, err = driver.Increment(ctx, "counter", 5)
	if err != nil {
		t.Fatalf("Increment error: %v", err)
	}
	if result != 6 {
		t.Errorf("expected 6, got %d", result)
	}
}

func TestDriver_Decrement(t *testing.T) {
	mr, driver := setupTestRedis(t)
	defer mr.Close()
	ctx := context.Background()

	driver.Increment(ctx, "counter", 10)

	result, err := driver.Decrement(ctx, "counter", 3)
	if err != nil {
		t.Fatalf("Decrement error: %v", err)
	}
	if result != 7 {
		t.Errorf("expected 7, got %d", result)
	}
}

func TestDriver_Expire(t *testing.T) {
	mr, driver := setupTestRedis(t)
	defer mr.Close()
	ctx := context.Background()

	driver.Set(ctx, "key", "value", 0)

	err := driver.Expire(ctx, "key", time.Minute)
	if err != nil {
		t.Fatalf("Expire error: %v", err)
	}

	ttl, err := driver.TTL(ctx, "key")
	if err != nil {
		t.Fatalf("TTL error: %v", err)
	}
	if ttl <= 0 {
		t.Error("TTL should be positive")
	}
}

func TestDriver_TTL(t *testing.T) {
	mr, driver := setupTestRedis(t)
	defer mr.Close()
	ctx := context.Background()

	driver.Set(ctx, "key", "value", time.Minute)

	ttl, err := driver.TTL(ctx, "key")
	if err != nil {
		t.Fatalf("TTL error: %v", err)
	}
	if ttl <= 0 || ttl > time.Minute {
		t.Errorf("unexpected TTL: %v", ttl)
	}
}

func TestDriver_Keys(t *testing.T) {
	mr, driver := setupTestRedis(t)
	defer mr.Close()
	ctx := context.Background()

	driver.Set(ctx, "user:1", "a", time.Minute)
	driver.Set(ctx, "user:2", "b", time.Minute)
	driver.Set(ctx, "order:1", "c", time.Minute)

	keys, err := driver.Keys(ctx, "user:*")
	if err != nil {
		t.Fatalf("Keys error: %v", err)
	}
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}

func TestDriver_Flush(t *testing.T) {
	mr, driver := setupTestRedis(t)
	defer mr.Close()
	ctx := context.Background()

	driver.Set(ctx, "a", "1", time.Minute)
	driver.Set(ctx, "b", "2", time.Minute)

	err := driver.Flush(ctx)
	if err != nil {
		t.Fatalf("Flush error: %v", err)
	}

	keys, _ := driver.Keys(ctx, "*")
	if len(keys) != 0 {
		t.Errorf("expected 0 keys after flush, got %d", len(keys))
	}
}

func TestDriver_Lock(t *testing.T) {
	mr, driver := setupTestRedis(t)
	defer mr.Close()
	ctx := context.Background()

	t.Run("acquire and release lock", func(t *testing.T) {
		lock, err := driver.Lock(ctx, "resource", time.Minute)
		if err != nil {
			t.Fatalf("Lock error: %v", err)
		}

		// Second lock should fail
		_, err = driver.Lock(ctx, "resource", time.Minute)
		if err == nil {
			t.Error("second lock should fail")
		}

		// Release lock
		err = lock.Unlock(ctx)
		if err != nil {
			t.Fatalf("Unlock error: %v", err)
		}

		// Now lock should succeed
		lock2, err := driver.Lock(ctx, "resource", time.Minute)
		if err != nil {
			t.Fatalf("Lock after unlock error: %v", err)
		}
		lock2.Unlock(ctx)
	})

	t.Run("extend lock", func(t *testing.T) {
		lock, err := driver.Lock(ctx, "extendable", time.Second)
		if err != nil {
			t.Fatalf("Lock error: %v", err)
		}
		defer lock.Unlock(ctx)

		err = lock.Extend(ctx, time.Minute)
		if err != nil {
			t.Fatalf("Extend error: %v", err)
		}
	})
}

func TestDriver_Remember(t *testing.T) {
	mr, driver := setupTestRedis(t)
	defer mr.Close()
	ctx := context.Background()

	callCount := 0
	computeFn := func() (any, error) {
		callCount++
		return "computed_value", nil
	}

	t.Run("computes and caches", func(t *testing.T) {
		var result string
		err := driver.Remember(ctx, "remember_key", time.Minute, computeFn, &result)
		if err != nil {
			t.Fatalf("Remember error: %v", err)
		}
		if result != "computed_value" {
			t.Errorf("expected 'computed_value', got %s", result)
		}
		if callCount != 1 {
			t.Errorf("expected 1 call, got %d", callCount)
		}
	})

	t.Run("returns cached value", func(t *testing.T) {
		var result string
		err := driver.Remember(ctx, "remember_key", time.Minute, computeFn, &result)
		if err != nil {
			t.Fatalf("Remember error: %v", err)
		}
		if result != "computed_value" {
			t.Errorf("expected 'computed_value', got %s", result)
		}
		if callCount != 1 {
			t.Errorf("compute function should not be called again, callCount: %d", callCount)
		}
	})
}

func TestDriver_Tags(t *testing.T) {
	mr, driver := setupTestRedis(t)
	defer mr.Close()
	ctx := context.Background()

	tagged := driver.Tags("users", "active")

	t.Run("set and get tagged value", func(t *testing.T) {
		err := tagged.Set(ctx, "user:1", "John", time.Minute)
		if err != nil {
			t.Fatalf("Set error: %v", err)
		}

		var result string
		err = tagged.Get(ctx, "user:1", &result)
		if err != nil {
			t.Fatalf("Get error: %v", err)
		}
		if result != "John" {
			t.Errorf("expected 'John', got %s", result)
		}
	})

	t.Run("flush tagged cache", func(t *testing.T) {
		tagged.Set(ctx, "user:2", "Jane", time.Minute)

		err := tagged.Flush(ctx)
		if err != nil {
			t.Fatalf("Flush error: %v", err)
		}
	})
}

func TestDriver_Ping(t *testing.T) {
	mr, driver := setupTestRedis(t)
	defer mr.Close()

	err := driver.Ping(context.Background())
	if err != nil {
		t.Errorf("Ping should succeed: %v", err)
	}
}

func TestDriver_Close(t *testing.T) {
	mr, driver := setupTestRedis(t)
	defer mr.Close()

	err := driver.Close()
	if err != nil {
		t.Errorf("Close should succeed: %v", err)
	}
}

func TestDriver_ImplementsCache(t *testing.T) {
	var _ contracts.Cache = (*Driver)(nil)
}

func BenchmarkDriver_Set(b *testing.B) {
	mr, _ := miniredis.Run()
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	driver := NewDriver(client)
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		driver.Set(ctx, "key", "value", time.Minute)
	}
}

func BenchmarkDriver_Get(b *testing.B) {
	mr, _ := miniredis.Run()
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	driver := NewDriver(client)
	ctx := context.Background()

	driver.Set(ctx, "key", "value", time.Minute)

	b.ResetTimer()
	b.ReportAllocs()

	var result string
	for i := 0; i < b.N; i++ {
		driver.Get(ctx, "key", &result)
	}
}
