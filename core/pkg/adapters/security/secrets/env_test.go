package secrets

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	contracts "github.com/madcok-co/unicorn/core/pkg/contracts"
)

// ---------- helpers ----------

func newTestManager(t *testing.T, cfg *EnvSecretManagerConfig) *EnvSecretManager {
	t.Helper()
	m, err := NewEnvSecretManager(cfg)
	if err != nil {
		t.Fatalf("NewEnvSecretManager: %v", err)
	}
	return m
}

// ---------- error variables ----------

func TestErrorVariables(t *testing.T) {
	if ErrSecretNotFound == nil {
		t.Error("ErrSecretNotFound should be non-nil")
	}
	if ErrSecretNotFound.Error() != "secret not found" {
		t.Errorf("unexpected ErrSecretNotFound message: %s", ErrSecretNotFound)
	}

	if ErrSetNotSupported == nil {
		t.Error("ErrSetNotSupported should be non-nil")
	}
	if ErrSetNotSupported.Error() != "set operation not supported" {
		t.Errorf("unexpected ErrSetNotSupported message: %s", ErrSetNotSupported)
	}

	if ErrDeleteNotSupported == nil {
		t.Error("ErrDeleteNotSupported should be non-nil")
	}
	if ErrDeleteNotSupported.Error() != "delete operation not supported" {
		t.Errorf("unexpected ErrDeleteNotSupported message: %s", ErrDeleteNotSupported)
	}

	if ErrWatchNotSupported == nil {
		t.Error("ErrWatchNotSupported should be non-nil")
	}
	if ErrWatchNotSupported.Error() != "watch operation not supported" {
		t.Errorf("unexpected ErrWatchNotSupported message: %s", ErrWatchNotSupported)
	}
}

// ---------- DefaultEnvSecretManagerConfig ----------

func TestDefaultEnvSecretManagerConfig(t *testing.T) {
	cfg := DefaultEnvSecretManagerConfig()

	if cfg.Prefix != "" {
		t.Errorf("expected empty Prefix, got %q", cfg.Prefix)
	}
	if cfg.KeySeparator != "." {
		t.Errorf("expected KeySeparator '.', got %q", cfg.KeySeparator)
	}
	if !cfg.EnableCache {
		t.Error("expected EnableCache == true")
	}
	if cfg.CacheTTL != 5*time.Minute {
		t.Errorf("expected CacheTTL 5m, got %v", cfg.CacheTTL)
	}
	if cfg.Defaults == nil {
		t.Error("expected non-nil Defaults map")
	}
	if len(cfg.Defaults) != 0 {
		t.Errorf("expected empty Defaults, got %d entries", len(cfg.Defaults))
	}
}

// ---------- NewEnvSecretManager ----------

func TestNewEnvSecretManager_NilConfig(t *testing.T) {
	m, err := NewEnvSecretManager(nil)
	if err != nil {
		t.Fatalf("nil config should succeed: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
	cfg := m.GetConfig()
	if cfg.Prefix != "" || cfg.KeySeparator != "." {
		t.Errorf("expected default config, got %+v", cfg)
	}
}

func TestNewEnvSecretManager_RequiredKeysPresent(t *testing.T) {
	t.Setenv("MY_REQUIRED_VAR", "hello")

	cfg := &EnvSecretManagerConfig{
		KeySeparator: ".",
		Required:     []string{"my.required.var"},
	}
	m, err := NewEnvSecretManager(cfg)
	if err != nil {
		t.Fatalf("expected success when required keys present: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
}

func TestNewEnvSecretManager_RequiredKeysMissing(t *testing.T) {
	cfg := &EnvSecretManagerConfig{
		KeySeparator: ".",
		Required:     []string{"nonexistent.key"},
	}
	_, err := NewEnvSecretManager(cfg)
	if err == nil {
		t.Fatal("expected error when required keys missing")
	}
	// NewEnvSecretManager returns a plain errors.New(...), not wrapping ErrSecretNotFound
	expectedMsg := "missing required environment variables: nonexistent.key"
	if err.Error() != expectedMsg {
		t.Errorf("expected error %q, got %q", expectedMsg, err.Error())
	}
}

// ---------- Get ----------

func TestGet_Success(t *testing.T) {
	t.Setenv("MY_VAL", "secret_value")
	m := newTestManager(t, &EnvSecretManagerConfig{
		KeySeparator: ".",
		EnableCache:  false,
	})

	val, err := m.Get(context.Background(), "my.val")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "secret_value" {
		t.Errorf("expected 'secret_value', got %q", val)
	}
}

func TestGet_NoPrefix(t *testing.T) {
	t.Setenv("PLAIN", "plain_value")
	m := newTestManager(t, &EnvSecretManagerConfig{
		Prefix:       "",
		KeySeparator: ".",
		EnableCache:  false,
	})

	val, err := m.Get(context.Background(), "PLAIN")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "plain_value" {
		t.Errorf("expected 'plain_value', got %q", val)
	}
}

func TestGet_NotFound(t *testing.T) {
	m := newTestManager(t, &EnvSecretManagerConfig{
		KeySeparator: ".",
		EnableCache:  false,
	})

	_, err := m.Get(context.Background(), "nonexistent.var")
	if !errors.Is(err, ErrSecretNotFound) {
		t.Errorf("expected ErrSecretNotFound, got %v", err)
	}
}

func TestGet_FromDefaults(t *testing.T) {
	m := newTestManager(t, &EnvSecretManagerConfig{
		KeySeparator: ".",
		EnableCache:  false,
		Defaults: map[string]string{
			"my.defaulted.key": "default_value",
		},
	})

	val, err := m.Get(context.Background(), "my.defaulted.key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "default_value" {
		t.Errorf("expected 'default_value', got %q", val)
	}
}

func TestGet_EnvOverridesDefaults(t *testing.T) {
	t.Setenv("MY_OVERRIDE_KEY", "env_value")
	m := newTestManager(t, &EnvSecretManagerConfig{
		KeySeparator: ".",
		EnableCache:  false,
		Defaults: map[string]string{
			"my.override.key": "default_value",
		},
	})

	val, err := m.Get(context.Background(), "my.override.key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "env_value" {
		t.Errorf("expected 'env_value', got %q", val)
	}
}

func TestGet_WithCacheHit(t *testing.T) {
	t.Setenv("CACHED_VAR", "cached_value")
	m := newTestManager(t, &EnvSecretManagerConfig{
		KeySeparator: ".",
		EnableCache:  true,
		CacheTTL:     1 * time.Hour,
	})

	// First call populates cache
	val1, err := m.Get(context.Background(), "cached.var")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val1 != "cached_value" {
		t.Errorf("expected 'cached_value', got %q", val1)
	}

	// Change the env var
	os.Setenv("CACHED_VAR", "changed_value")
	defer os.Unsetenv("CACHED_VAR")

	// Second call should return cached value
	val2, err := m.Get(context.Background(), "cached.var")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val2 != "cached_value" {
		t.Errorf("expected cached 'cached_value', got %q", val2)
	}
}

func TestGet_CacheExpired(t *testing.T) {
	t.Setenv("EXPIRING_VAR", "original")
	m := newTestManager(t, &EnvSecretManagerConfig{
		KeySeparator: ".",
		EnableCache:  true,
		CacheTTL:     1 * time.Nanosecond,
	})

	// Populate cache
	m.Get(context.Background(), "expiring.var")

	// Give enough time for cache to expire
	time.Sleep(5 * time.Millisecond)

	os.Setenv("EXPIRING_VAR", "updated")
	defer os.Unsetenv("EXPIRING_VAR")

	val, err := m.Get(context.Background(), "expiring.var")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "updated" {
		t.Errorf("expected 'updated' after expiry, got %q", val)
	}
}

func TestGet_WithPrefix(t *testing.T) {
	t.Setenv("APP_MY_KEY", "prefixed_value")
	m := newTestManager(t, &EnvSecretManagerConfig{
		Prefix:       "APP_",
		KeySeparator: ".",
		EnableCache:  false,
	})

	val, err := m.Get(context.Background(), "my.key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "prefixed_value" {
		t.Errorf("expected 'prefixed_value', got %q", val)
	}
}

func TestGet_FallbackToRawKey(t *testing.T) {
	// When the transformed env var doesn't exist, it falls back to the raw key
	t.Setenv("raw-key", "raw_value")
	m := newTestManager(t, &EnvSecretManagerConfig{
		KeySeparator: ".",
		EnableCache:  false,
	})

	val, err := m.Get(context.Background(), "raw-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "raw_value" {
		t.Errorf("expected 'raw_value', got %q", val)
	}
}

// ---------- GetJSON ----------

func TestGetJSON_Success(t *testing.T) {
	t.Setenv("JSON_VAR", `{"name":"test","age":30}`)
	m := newTestManager(t, &EnvSecretManagerConfig{
		KeySeparator: ".",
		EnableCache:  false,
	})

	var dest struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	err := m.GetJSON(context.Background(), "json.var", &dest)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dest.Name != "test" || dest.Age != 30 {
		t.Errorf("unexpected dest: %+v", dest)
	}
}

func TestGetJSON_NotFound(t *testing.T) {
	m := newTestManager(t, &EnvSecretManagerConfig{
		KeySeparator: ".",
		EnableCache:  false,
	})

	var dest map[string]any
	err := m.GetJSON(context.Background(), "nonexistent.json", &dest)
	if !errors.Is(err, ErrSecretNotFound) {
		t.Errorf("expected ErrSecretNotFound, got %v", err)
	}
}

func TestGetJSON_InvalidJSON(t *testing.T) {
	t.Setenv("BAD_JSON", "not-valid-json")
	m := newTestManager(t, &EnvSecretManagerConfig{
		KeySeparator: ".",
		EnableCache:  false,
	})

	var dest map[string]any
	err := m.GetJSON(context.Background(), "bad.json", &dest)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !errors.Is(err, ErrSecretNotFound) {
		var syntaxErr *json.SyntaxError
		if !errors.As(err, &syntaxErr) {
			t.Errorf("expected json.SyntaxError, got %T: %v", err, err)
		}
	}
}

// ---------- Set ----------

func TestSet_Success(t *testing.T) {
	m := newTestManager(t, &EnvSecretManagerConfig{
		KeySeparator: ".",
		EnableCache:  false,
	})

	envKey := "SET_TEST_VAR"
	defer os.Unsetenv(envKey)

	err := m.Set(context.Background(), "set.test.var", "new_value")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := os.Getenv(envKey)
	if got != "new_value" {
		t.Errorf("expected 'new_value', got %q", got)
	}
}

func TestSet_InvalidatesCache(t *testing.T) {
	t.Setenv("CACHE_SET_VAR", "initial")
	m := newTestManager(t, &EnvSecretManagerConfig{
		KeySeparator: ".",
		EnableCache:  true,
		CacheTTL:     1 * time.Hour,
	})

	// Populate cache
	val1, _ := m.Get(context.Background(), "cache.set.var")
	if val1 != "initial" {
		t.Fatalf("expected 'initial', got %q", val1)
	}

	// Set triggers cache invalidation
	m.Set(context.Background(), "cache.set.var", "updated")
	defer os.Unsetenv("CACHE_SET_VAR")

	// Now Get should return the new value
	val2, err := m.Get(context.Background(), "cache.set.var")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val2 != "updated" {
		t.Errorf("expected 'updated' after cache invalidation, got %q", val2)
	}
}

// ---------- Delete ----------

func TestDelete_Success(t *testing.T) {
	envKey := "DELETE_TEST_VAR"
	os.Setenv(envKey, "to_delete")
	defer os.Unsetenv(envKey)

	m := newTestManager(t, &EnvSecretManagerConfig{
		KeySeparator: ".",
		EnableCache:  false,
	})

	err := m.Delete(context.Background(), "delete.test.var")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, exists := os.LookupEnv(envKey)
	if exists {
		t.Error("expected env var to be unset")
	}
}

func TestDelete_InvalidatesCache(t *testing.T) {
	t.Setenv("CACHE_DEL_VAR", "cached_val")
	m := newTestManager(t, &EnvSecretManagerConfig{
		KeySeparator: ".",
		EnableCache:  true,
		CacheTTL:     1 * time.Hour,
	})

	// Populate cache
	val1, _ := m.Get(context.Background(), "cache.del.var")
	if val1 != "cached_val" {
		t.Fatalf("expected 'cached_val', got %q", val1)
	}

	// Delete invalidates cache
	m.Delete(context.Background(), "cache.del.var")

	_, err := m.Get(context.Background(), "cache.del.var")
	if !errors.Is(err, ErrSecretNotFound) {
		t.Errorf("expected ErrSecretNotFound after delete, got %v", err)
	}
}

// ---------- List ----------

func TestList_NoPrefix(t *testing.T) {
	t.Setenv("LIST_A", "a")
	t.Setenv("LIST_B", "b")

	m := newTestManager(t, &EnvSecretManagerConfig{
		Prefix:       "",
		KeySeparator: ".",
		EnableCache:  false,
	})

	keys, err := m.List(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundA, foundB := false, false
	for _, k := range keys {
		if k == "list.a" || k == "LIST_A" {
			foundA = true
		}
		if k == "list.b" || k == "LIST_B" {
			foundB = true
		}
	}
	if !foundA || !foundB {
		t.Errorf("expected to find LIST_A and LIST_B in %v", keys)
	}
}

func TestList_WithPrefix(t *testing.T) {
	t.Setenv("APP_SVC_A", "a")
	t.Setenv("APP_SVC_B", "b")
	t.Setenv("OTHER_VAR", "other")

	m := newTestManager(t, &EnvSecretManagerConfig{
		Prefix:       "APP_",
		KeySeparator: ".",
		EnableCache:  false,
	})

	keys, err := m.List(context.Background(), "svc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The prefix "svc" becomes "APP_SVC_"  (converted to env prefix)
	// So we should get APP_SVC_A and APP_SVC_B but not OTHER_VAR
	foundA, foundB, foundOther := false, false, false
	for _, k := range keys {
		switch k {
		case "svc.a":
			foundA = true
		case "svc.b":
			foundB = true
		case "other.var":
			foundOther = true
		}
	}
	if !foundA {
		t.Error("expected to find 'svc.a'")
	}
	if !foundB {
		t.Error("expected to find 'svc.b'")
	}
	if foundOther {
		t.Error("should not have found 'other.var'")
	}

	if len(keys) < 2 {
		t.Errorf("expected at least 2 keys, got %v", keys)
	}
}

// ---------- Watch ----------

func TestWatch_CallbackOnChange(t *testing.T) {
	envKey := "WATCH_TEST_VAR"
	os.Setenv(envKey, "value1")
	defer os.Unsetenv(envKey)

	m := newTestManager(t, &EnvSecretManagerConfig{
		KeySeparator: ".",
		EnableCache:  true,
		CacheTTL:     10 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	var receivedValues []string

	err := m.Watch(ctx, "watch.test.var", func(newValue string) {
		mu.Lock()
		defer mu.Unlock()
		receivedValues = append(receivedValues, newValue)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Change the value
	time.Sleep(5 * time.Millisecond)
	os.Setenv(envKey, "value2")

	// Wait for polling
	time.Sleep(30 * time.Millisecond)

	mu.Lock()
	hasValue2 := false
	for _, v := range receivedValues {
		if v == "value2" {
			hasValue2 = true
			break
		}
	}
	mu.Unlock()

	if !hasValue2 {
		t.Errorf("expected callback to receive 'value2', got %v", receivedValues)
	}

	cancel()
}

// ---------- ClearCache ----------

func TestClearCache(t *testing.T) {
	t.Setenv("CLEAR_CACHE_VAR", "val")
	m := newTestManager(t, &EnvSecretManagerConfig{
		KeySeparator: ".",
		EnableCache:  true,
		CacheTTL:     1 * time.Hour,
	})

	// Populate cache
	m.Get(context.Background(), "clear.cache.var")

	// Verify it's cached
	_, loaded := m.cache.Load("clear.cache.var")
	if !loaded {
		t.Fatal("expected cache entry to exist before clear")
	}

	m.ClearCache()

	// Verify cache is empty
	m.cache.Range(func(_, _ interface{}) bool {
		t.Error("expected empty cache after ClearCache")
		return false
	})
}

// ---------- GetConfig ----------

func TestGetConfig(t *testing.T) {
	customCfg := &EnvSecretManagerConfig{
		Prefix:       "CUSTOM_",
		KeySeparator: "/",
		EnableCache:  false,
		CacheTTL:     10 * time.Minute,
	}
	m := newTestManager(t, customCfg)

	cfg := m.GetConfig()
	if cfg.Prefix != "CUSTOM_" {
		t.Errorf("expected Prefix 'CUSTOM_', got %q", cfg.Prefix)
	}
	if cfg.KeySeparator != "/" {
		t.Errorf("expected KeySeparator '/', got %q", cfg.KeySeparator)
	}
	if cfg.EnableCache {
		t.Error("expected EnableCache false")
	}
	if cfg.CacheTTL != 10*time.Minute {
		t.Errorf("expected CacheTTL 10m, got %v", cfg.CacheTTL)
	}
}

// ---------- MustGet ----------

func TestMustGet_Success(t *testing.T) {
	t.Setenv("MUST_GET_VAR", "must_value")
	m := newTestManager(t, &EnvSecretManagerConfig{
		KeySeparator: ".",
		EnableCache:  false,
	})

	val := m.MustGet(context.Background(), "must.get.var")
	if val != "must_value" {
		t.Errorf("expected 'must_value', got %q", val)
	}
}

func TestMustGet_Panics(t *testing.T) {
	m := newTestManager(t, &EnvSecretManagerConfig{
		KeySeparator: ".",
		EnableCache:  false,
	})

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic")
		}
	}()
	_ = m.MustGet(context.Background(), "nonexistent.must.var")
}

// ---------- GetWithDefault ----------

func TestGetWithDefault_Found(t *testing.T) {
	t.Setenv("WITH_DEFAULT_VAR", "real_val")
	m := newTestManager(t, &EnvSecretManagerConfig{
		KeySeparator: ".",
		EnableCache:  false,
	})

	val := m.GetWithDefault(context.Background(), "with.default.var", "fallback")
	if val != "real_val" {
		t.Errorf("expected 'real_val', got %q", val)
	}
}

func TestGetWithDefault_NotFount(t *testing.T) {
	m := newTestManager(t, &EnvSecretManagerConfig{
		KeySeparator: ".",
		EnableCache:  false,
	})

	val := m.GetWithDefault(context.Background(), "missing.var", "fallback")
	if val != "fallback" {
		t.Errorf("expected 'fallback', got %q", val)
	}
}

// ---------- GetInt ----------

func TestGetInt_Success(t *testing.T) {
	t.Setenv("INT_VAR", "42")
	m := newTestManager(t, &EnvSecretManagerConfig{
		KeySeparator: ".",
		EnableCache:  false,
	})

	val, err := m.GetInt(context.Background(), "int.var")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 42 {
		t.Errorf("expected 42, got %d", val)
	}
}

func TestGetInt_Negative(t *testing.T) {
	t.Setenv("NEG_INT_VAR", "-10")
	m := newTestManager(t, &EnvSecretManagerConfig{
		KeySeparator: ".",
		EnableCache:  false,
	})

	val, err := m.GetInt(context.Background(), "neg.int.var")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != -10 {
		t.Errorf("expected -10, got %d", val)
	}
}

func TestGetInt_NotFound(t *testing.T) {
	m := newTestManager(t, &EnvSecretManagerConfig{
		KeySeparator: ".",
		EnableCache:  false,
	})

	_, err := m.GetInt(context.Background(), "missing.int")
	if !errors.Is(err, ErrSecretNotFound) {
		t.Errorf("expected ErrSecretNotFound, got %v", err)
	}
}

func TestGetInt_InvalidValue(t *testing.T) {
	t.Setenv("BAD_INT_VAR", "not-a-number")
	m := newTestManager(t, &EnvSecretManagerConfig{
		KeySeparator: ".",
		EnableCache:  false,
	})

	_, err := m.GetInt(context.Background(), "bad.int.var")
	if err == nil {
		t.Fatal("expected error for invalid int")
	}
}

// ---------- GetBool ----------

func TestGetBool_TrueVariants(t *testing.T) {
	m := newTestManager(t, &EnvSecretManagerConfig{
		KeySeparator: ".",
		EnableCache:  false,
	})

	tests := []string{"true", "1", "yes", "on", "TRUE", "Yes", "ON"}
	for _, val := range tests {
		t.Run(val, func(t *testing.T) {
			t.Setenv("BOOL_VAR", val)
			result, err := m.GetBool(context.Background(), "bool.var")
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", val, err)
			}
			if !result {
				t.Errorf("expected true for %q", val)
			}
		})
	}
}

func TestGetBool_FalseVariants(t *testing.T) {
	m := newTestManager(t, &EnvSecretManagerConfig{
		KeySeparator: ".",
		EnableCache:  false,
	})

	tests := []string{"false", "0", "no", "off", "FALSE", "No", "OFF"}
	for _, val := range tests {
		t.Run(val, func(t *testing.T) {
			t.Setenv("BOOL_VAR", val)
			result, err := m.GetBool(context.Background(), "bool.var")
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", val, err)
			}
			if result {
				t.Errorf("expected false for %q", val)
			}
		})
	}
}

func TestGetBool_EmptyString(t *testing.T) {
	// os.Setenv with "" is indistinguishable from unset via os.Getenv,
	// so Get returns ErrSecretNotFound before reaching GetBool's switch.
	// The "" → false case in GetBool is unreachable from env-based secrets.
	t.Setenv("BOOL_VAR", "")
	m := newTestManager(t, &EnvSecretManagerConfig{
		KeySeparator: ".",
		EnableCache:  false,
	})

	_, err := m.GetBool(context.Background(), "bool.var")
	if !errors.Is(err, ErrSecretNotFound) {
		t.Errorf("expected ErrSecretNotFound for empty-string env var, got %v", err)
	}
}

func TestGetBool_NotFound(t *testing.T) {
	m := newTestManager(t, &EnvSecretManagerConfig{
		KeySeparator: ".",
		EnableCache:  false,
	})

	_, err := m.GetBool(context.Background(), "missing.bool")
	if !errors.Is(err, ErrSecretNotFound) {
		t.Errorf("expected ErrSecretNotFound, got %v", err)
	}
}

func TestGetBool_InvalidValue(t *testing.T) {
	t.Setenv("BAD_BOOL_VAR", "maybe")
	m := newTestManager(t, &EnvSecretManagerConfig{
		KeySeparator: ".",
		EnableCache:  false,
	})

	_, err := m.GetBool(context.Background(), "bad.bool.var")
	if err == nil {
		t.Fatal("expected error for invalid boolean value")
	}
	if err.Error() != "invalid boolean value: maybe" {
		t.Errorf("unexpected error message: %s", err)
	}
}

// ---------- keyToEnvVar ----------

func TestKeyToEnvVar_NoPrefix(t *testing.T) {
	m := newTestManager(t, &EnvSecretManagerConfig{
		Prefix:       "",
		KeySeparator: ".",
	})

	result := m.keyToEnvVar("database.host")
	if result != "DATABASE_HOST" {
		t.Errorf("expected 'DATABASE_HOST', got %q", result)
	}
}

func TestKeyToEnvVar_WithPrefix(t *testing.T) {
	m := newTestManager(t, &EnvSecretManagerConfig{
		Prefix:       "APP_",
		KeySeparator: ".",
	})

	result := m.keyToEnvVar("database.host")
	if result != "APP_DATABASE_HOST" {
		t.Errorf("expected 'APP_DATABASE_HOST', got %q", result)
	}
}

func TestKeyToEnvVar_CustomSeparator(t *testing.T) {
	m := newTestManager(t, &EnvSecretManagerConfig{
		Prefix:       "CFG_",
		KeySeparator: "/",
	})

	result := m.keyToEnvVar("db/host")
	if result != "CFG_DB_HOST" {
		t.Errorf("expected 'CFG_DB_HOST', got %q", result)
	}
}

func TestKeyToEnvVar_NoSeparator(t *testing.T) {
	m := newTestManager(t, &EnvSecretManagerConfig{
		Prefix:       "",
		KeySeparator: ".",
	})

	result := m.keyToEnvVar("simple")
	if result != "SIMPLE" {
		t.Errorf("expected 'SIMPLE', got %q", result)
	}
}

// ---------- envVarToKey ----------

func TestEnvVarToKey_NoPrefix(t *testing.T) {
	m := newTestManager(t, &EnvSecretManagerConfig{
		Prefix:       "",
		KeySeparator: ".",
	})

	result := m.envVarToKey("DATABASE_HOST")
	if result != "database.host" {
		t.Errorf("expected 'database.host', got %q", result)
	}
}

func TestEnvVarToKey_WithPrefix(t *testing.T) {
	m := newTestManager(t, &EnvSecretManagerConfig{
		Prefix:       "APP_",
		KeySeparator: ".",
	})

	result := m.envVarToKey("APP_DATABASE_HOST")
	if result != "database.host" {
		t.Errorf("expected 'database.host', got %q", result)
	}
}

func TestEnvVarToKey_CustomSeparator(t *testing.T) {
	m := newTestManager(t, &EnvSecretManagerConfig{
		Prefix:       "CFG_",
		KeySeparator: "/",
	})

	result := m.envVarToKey("CFG_DB_HOST")
	if result != "db/host" {
		t.Errorf("expected 'db/host', got %q", result)
	}
}

func TestEnvVarToKey_NoPrefixMatch(t *testing.T) {
	// If prefix doesn't match, it's left as-is (then lowercased, underscores replaced)
	m := newTestManager(t, &EnvSecretManagerConfig{
		Prefix:       "APP_",
		KeySeparator: ".",
	})

	result := m.envVarToKey("OTHER_DATABASE_HOST")
	// prefix "APP_" not found => remains "OTHER_DATABASE_HOST"
	// lowercase => "other_database_host"
	// _ → . => "other.database.host"
	if result != "other.database.host" {
		t.Errorf("expected 'other.database.host', got %q", result)
	}
}

// ---------- roundtrip ----------

func TestKeyEnvVarRoundtrip(t *testing.T) {
	m := newTestManager(t, &EnvSecretManagerConfig{
		Prefix:       "APP_",
		KeySeparator: ".",
	})

	original := "database.connection.pool.size"
	envVar := m.keyToEnvVar(original)
	back := m.envVarToKey(envVar)

	if back != original {
		t.Errorf("roundtrip failed: %q → %q → %q", original, envVar, back)
	}
}

// ---------- interface compliance ----------

func TestImplementsSecretManager(t *testing.T) {
	var _ contracts.SecretManager = (*EnvSecretManager)(nil)
	// Compile-time check — if this test compiles, the interface is satisfied
}

// ---------- concurrent access ----------

func TestConcurrentAccess(t *testing.T) {
	t.Setenv("CONCURRENT_VAR", "val")
	m := newTestManager(t, &EnvSecretManagerConfig{
		KeySeparator: ".",
		EnableCache:  true,
		CacheTTL:     10 * time.Millisecond,
	})

	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				m.Get(context.Background(), "concurrent.var")
				m.Set(context.Background(), "concurrent.var", "val")
				m.Delete(context.Background(), "concurrent.var")
				m.Set(context.Background(), "concurrent.var", "val")
				m.GetWithDefault(context.Background(), "concurrent.var", "default")
				_ = m.GetConfig()
				m.MustGet(context.Background(), "concurrent.var")
			}
		}()
	}

	wg.Wait()
}
