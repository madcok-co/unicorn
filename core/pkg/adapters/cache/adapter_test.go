package cache

import (
	"context"
	"testing"
	"time"
)

func TestMemoryDriver(t *testing.T) {
	driver := NewMemoryDriver()
	ctx := context.Background()

	t.Run("Set and Get", func(t *testing.T) {
		err := driver.Set(ctx, "key1", []byte("value1"), time.Hour)
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}

		val, err := driver.Get(ctx, "key1")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if string(val) != "value1" {
			t.Errorf("expected 'value1', got '%s'", string(val))
		}
	})

	t.Run("Get non-existent key", func(t *testing.T) {
		_, err := driver.Get(ctx, "nonexistent")
		if err == nil {
			t.Error("expected error for non-existent key")
		}
	})

	t.Run("Exists", func(t *testing.T) {
		driver.Set(ctx, "exists_key", []byte("value"), time.Hour)

		exists, err := driver.Exists(ctx, "exists_key")
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if !exists {
			t.Error("expected key to exist")
		}

		exists, err = driver.Exists(ctx, "not_exists")
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if exists {
			t.Error("expected key to not exist")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		driver.Set(ctx, "delete_key", []byte("value"), time.Hour)

		err := driver.Delete(ctx, "delete_key")
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		exists, _ := driver.Exists(ctx, "delete_key")
		if exists {
			t.Error("key should be deleted")
		}
	})

	t.Run("TTL expiration", func(t *testing.T) {
		err := driver.Set(ctx, "ttl_key", []byte("value"), 50*time.Millisecond)
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}

		// Should exist initially
		val, err := driver.Get(ctx, "ttl_key")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if string(val) != "value" {
			t.Error("expected value to exist before expiration")
		}

		// Wait for expiration
		time.Sleep(100 * time.Millisecond)

		// Should be expired
		_, err = driver.Get(ctx, "ttl_key")
		if err == nil {
			t.Error("expected error for expired key")
		}
	})

	t.Run("Ping", func(t *testing.T) {
		err := driver.Ping(ctx)
		if err != nil {
			t.Errorf("Ping failed: %v", err)
		}
	})

	t.Run("Close", func(t *testing.T) {
		d := NewMemoryDriver()
		err := d.Close()
		if err != nil {
			t.Errorf("Close failed: %v", err)
		}
	})
}

func TestMemoryDriverBatch(t *testing.T) {
	driver := NewMemoryDriver()
	ctx := context.Background()

	t.Run("GetMany", func(t *testing.T) {
		driver.Set(ctx, "mk1", []byte("v1"), time.Hour)
		driver.Set(ctx, "mk2", []byte("v2"), time.Hour)

		vals, err := driver.GetMany(ctx, []string{"mk1", "mk2", "mk3"})
		if err != nil {
			t.Fatalf("GetMany failed: %v", err)
		}

		if string(vals["mk1"]) != "v1" {
			t.Errorf("expected 'v1', got '%s'", string(vals["mk1"]))
		}
		if string(vals["mk2"]) != "v2" {
			t.Errorf("expected 'v2', got '%s'", string(vals["mk2"]))
		}
	})

	t.Run("SetMany", func(t *testing.T) {
		items := map[string][]byte{
			"mset1": []byte("val1"),
			"mset2": []byte("val2"),
		}
		err := driver.SetMany(ctx, items, time.Hour)
		if err != nil {
			t.Fatalf("SetMany failed: %v", err)
		}

		val, _ := driver.Get(ctx, "mset1")
		if string(val) != "val1" {
			t.Errorf("expected 'val1', got '%s'", string(val))
		}
	})

	t.Run("DeleteMany", func(t *testing.T) {
		driver.Set(ctx, "mdel1", []byte("v1"), time.Hour)
		driver.Set(ctx, "mdel2", []byte("v2"), time.Hour)

		err := driver.DeleteMany(ctx, []string{"mdel1", "mdel2"})
		if err != nil {
			t.Fatalf("DeleteMany failed: %v", err)
		}

		exists1, _ := driver.Exists(ctx, "mdel1")
		exists2, _ := driver.Exists(ctx, "mdel2")
		if exists1 || exists2 {
			t.Error("keys should be deleted")
		}
	})
}

func TestMemoryDriverAtomic(t *testing.T) {
	driver := NewMemoryDriver()
	ctx := context.Background()

	t.Run("Increment", func(t *testing.T) {
		driver.Set(ctx, "counter", []byte("10"), time.Hour)

		val, err := driver.Increment(ctx, "counter", 1)
		if err != nil {
			t.Fatalf("Increment failed: %v", err)
		}
		if val != 11 {
			t.Errorf("expected 11, got %d", val)
		}

		val, err = driver.Increment(ctx, "counter", 5)
		if err != nil {
			t.Fatalf("Increment failed: %v", err)
		}
		if val != 16 {
			t.Errorf("expected 16, got %d", val)
		}
	})

	t.Run("Decrement", func(t *testing.T) {
		driver.Set(ctx, "counter2", []byte("100"), time.Hour)

		val, err := driver.Decrement(ctx, "counter2", 10)
		if err != nil {
			t.Fatalf("Decrement failed: %v", err)
		}
		if val != 90 {
			t.Errorf("expected 90, got %d", val)
		}
	})

	t.Run("Keys", func(t *testing.T) {
		d := NewMemoryDriver()
		d.Set(ctx, "prefix_a", []byte("1"), time.Hour)
		d.Set(ctx, "prefix_b", []byte("2"), time.Hour)
		d.Set(ctx, "other", []byte("3"), time.Hour)

		keys, err := d.Keys(ctx, "prefix_*")
		if err != nil {
			t.Fatalf("Keys failed: %v", err)
		}

		if len(keys) != 2 {
			t.Errorf("expected 2 keys, got %d", len(keys))
		}
	})

	t.Run("Flush", func(t *testing.T) {
		d := NewMemoryDriver()
		d.Set(ctx, "key1", []byte("1"), time.Hour)
		d.Set(ctx, "key2", []byte("2"), time.Hour)

		err := d.Flush(ctx)
		if err != nil {
			t.Fatalf("Flush failed: %v", err)
		}

		exists1, _ := d.Exists(ctx, "key1")
		exists2, _ := d.Exists(ctx, "key2")
		if exists1 || exists2 {
			t.Error("all keys should be flushed")
		}
	})
}
