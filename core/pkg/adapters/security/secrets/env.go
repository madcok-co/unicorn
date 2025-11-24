package secrets

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

var (
	ErrSecretNotFound     = errors.New("secret not found")
	ErrSetNotSupported    = errors.New("set operation not supported")
	ErrDeleteNotSupported = errors.New("delete operation not supported")
	ErrWatchNotSupported  = errors.New("watch operation not supported")
)

// EnvSecretManagerConfig adalah konfigurasi untuk environment-based secret manager
type EnvSecretManagerConfig struct {
	// Prefix for environment variables (e.g., "APP_" -> APP_DATABASE_URL)
	Prefix string

	// Separator for nested keys (e.g., "." -> DATABASE.URL -> DATABASE_URL)
	KeySeparator string

	// Enable caching
	EnableCache bool

	// Cache TTL (how often to re-read env vars)
	CacheTTL time.Duration

	// Default values if env var not found
	Defaults map[string]string

	// Required keys (will error on startup if missing)
	Required []string
}

// DefaultEnvSecretManagerConfig returns default configuration
func DefaultEnvSecretManagerConfig() *EnvSecretManagerConfig {
	return &EnvSecretManagerConfig{
		Prefix:       "",
		KeySeparator: ".",
		EnableCache:  true,
		CacheTTL:     5 * time.Minute,
		Defaults:     make(map[string]string),
	}
}

// EnvSecretManager implements SecretManager using environment variables
type EnvSecretManager struct {
	config *EnvSecretManagerConfig

	// Cache
	cache sync.Map
}

type cacheEntry struct {
	value     string
	expiresAt time.Time
}

// NewEnvSecretManager creates a new environment-based secret manager
func NewEnvSecretManager(config *EnvSecretManagerConfig) (*EnvSecretManager, error) {
	if config == nil {
		config = DefaultEnvSecretManagerConfig()
	}

	sm := &EnvSecretManager{
		config: config,
	}

	// Validate required keys
	if len(config.Required) > 0 {
		missing := make([]string, 0)
		for _, key := range config.Required {
			if _, err := sm.Get(context.Background(), key); err != nil {
				missing = append(missing, key)
			}
		}
		if len(missing) > 0 {
			return nil, errors.New("missing required environment variables: " + strings.Join(missing, ", "))
		}
	}

	return sm, nil
}

// Get retrieves a secret from environment variables
func (m *EnvSecretManager) Get(ctx context.Context, key string) (string, error) {
	// Check cache first
	if m.config.EnableCache {
		if entry, ok := m.cache.Load(key); ok {
			cached := entry.(*cacheEntry)
			if time.Now().Before(cached.expiresAt) {
				return cached.value, nil
			}
		}
	}

	// Convert key to env var name
	envKey := m.keyToEnvVar(key)

	// Try to get from environment
	value := os.Getenv(envKey)
	if value == "" {
		// Try without prefix transformation
		value = os.Getenv(key)
	}

	// Check defaults
	if value == "" {
		if defaultVal, ok := m.config.Defaults[key]; ok {
			value = defaultVal
		}
	}

	if value == "" {
		return "", ErrSecretNotFound
	}

	// Cache the value
	if m.config.EnableCache {
		m.cache.Store(key, &cacheEntry{
			value:     value,
			expiresAt: time.Now().Add(m.config.CacheTTL),
		})
	}

	return value, nil
}

// GetJSON retrieves and unmarshals a JSON secret
func (m *EnvSecretManager) GetJSON(ctx context.Context, key string, dest any) error {
	value, err := m.Get(ctx, key)
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(value), dest)
}

// Set is not supported for environment variables
func (m *EnvSecretManager) Set(ctx context.Context, key, value string) error {
	// For testing/development, allow setting env vars
	envKey := m.keyToEnvVar(key)
	if err := os.Setenv(envKey, value); err != nil {
		return err
	}

	// Invalidate cache
	m.cache.Delete(key)

	return nil
}

// Delete is not supported for environment variables
func (m *EnvSecretManager) Delete(ctx context.Context, key string) error {
	envKey := m.keyToEnvVar(key)
	if err := os.Unsetenv(envKey); err != nil {
		return err
	}

	// Invalidate cache
	m.cache.Delete(key)

	return nil
}

// List lists available secrets with a prefix
func (m *EnvSecretManager) List(ctx context.Context, prefix string) ([]string, error) {
	envPrefix := m.keyToEnvVar(prefix)
	if envPrefix != "" && !strings.HasSuffix(envPrefix, "_") {
		envPrefix += "_"
	}

	var keys []string
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}

		envKey := parts[0]
		if envPrefix == "" || strings.HasPrefix(envKey, envPrefix) {
			// Convert back to key format
			key := m.envVarToKey(envKey)
			keys = append(keys, key)
		}
	}

	return keys, nil
}

// Watch is not supported for environment variables
func (m *EnvSecretManager) Watch(ctx context.Context, key string, callback func(newValue string)) error {
	// Polling-based watch for env vars
	go func() {
		ticker := time.NewTicker(m.config.CacheTTL)
		defer ticker.Stop()

		var lastValue string
		if v, err := m.Get(ctx, key); err == nil {
			lastValue = v
		}

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Invalidate cache to get fresh value
				m.cache.Delete(key)

				newValue, err := m.Get(ctx, key)
				if err != nil {
					continue
				}

				if newValue != lastValue {
					lastValue = newValue
					callback(newValue)
				}
			}
		}
	}()

	return nil
}

// keyToEnvVar converts a key to environment variable name
func (m *EnvSecretManager) keyToEnvVar(key string) string {
	// Replace separator with underscore
	envKey := strings.ReplaceAll(key, m.config.KeySeparator, "_")

	// Convert to uppercase
	envKey = strings.ToUpper(envKey)

	// Add prefix
	if m.config.Prefix != "" {
		envKey = m.config.Prefix + envKey
	}

	return envKey
}

// envVarToKey converts environment variable name to key
func (m *EnvSecretManager) envVarToKey(envVar string) string {
	key := envVar

	// Remove prefix
	if m.config.Prefix != "" {
		key = strings.TrimPrefix(key, m.config.Prefix)
	}

	// Convert to lowercase
	key = strings.ToLower(key)

	// Replace underscore with separator
	key = strings.ReplaceAll(key, "_", m.config.KeySeparator)

	return key
}

// ClearCache clears the cache
func (m *EnvSecretManager) ClearCache() {
	m.cache = sync.Map{}
}

// GetConfig returns the configuration
func (m *EnvSecretManager) GetConfig() *EnvSecretManagerConfig {
	return m.config
}

// MustGet retrieves a secret or panics
func (m *EnvSecretManager) MustGet(ctx context.Context, key string) string {
	value, err := m.Get(ctx, key)
	if err != nil {
		panic("failed to get secret " + key + ": " + err.Error())
	}
	return value
}

// GetWithDefault retrieves a secret or returns default value
func (m *EnvSecretManager) GetWithDefault(ctx context.Context, key, defaultValue string) string {
	value, err := m.Get(ctx, key)
	if err != nil {
		return defaultValue
	}
	return value
}

// GetInt retrieves a secret as integer
func (m *EnvSecretManager) GetInt(ctx context.Context, key string) (int, error) {
	value, err := m.Get(ctx, key)
	if err != nil {
		return 0, err
	}

	var result int
	err = json.Unmarshal([]byte(value), &result)
	return result, err
}

// GetBool retrieves a secret as boolean
func (m *EnvSecretManager) GetBool(ctx context.Context, key string) (bool, error) {
	value, err := m.Get(ctx, key)
	if err != nil {
		return false, err
	}

	switch strings.ToLower(value) {
	case "true", "1", "yes", "on":
		return true, nil
	case "false", "0", "no", "off", "":
		return false, nil
	default:
		return false, errors.New("invalid boolean value: " + value)
	}
}

// Ensure EnvSecretManager implements SecretManager
var _ contracts.SecretManager = (*EnvSecretManager)(nil)
