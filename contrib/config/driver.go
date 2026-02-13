// Package config provides configuration management using Viper for Unicorn Framework.
//
// Supports:
//   - Multiple config sources (files, env vars, remote)
//   - Hot reload / watch for changes
//   - Multiple formats (JSON, YAML, TOML, HCL, ENV)
//   - Environment-specific configs (dev, staging, prod)
//   - Nested configuration
//   - Type-safe getters
//
// Usage:
//
//	import (
//	    "github.com/madcok-co/unicorn/contrib/config"
//	)
//
//	// Initialize config
//	cfg := config.NewDriver(&config.Config{
//	    ConfigName: "app",
//	    ConfigPath: "./configs",
//	    ConfigType: "yaml",
//	})
//
//	// Get values
//	port := cfg.GetInt("server.port")
//	dbHost := cfg.GetString("database.host")
package config

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// Driver implements configuration management using Viper
type Driver struct {
	viper  *viper.Viper
	config *Config
	mu     sync.RWMutex

	// Callbacks for config changes
	onChange []func(key string, value interface{})
}

// Config for configuration driver
type Config struct {
	// Config file settings
	ConfigName string // Config file name (without extension)
	ConfigPath string // Config file path
	ConfigType string // Config file type (yaml, json, toml, etc.)
	ConfigFile string // Full path to config file (alternative to name+path)

	// Additional config paths to search
	ConfigPaths []string

	// Environment variables
	EnvPrefix      string // Prefix for environment variables (e.g., "APP")
	AutomaticEnv   bool   // Automatically read env vars
	EnvKeyReplacer string // Replace keys (e.g., "." to "_")

	// Remote config (etcd, consul)
	RemoteProvider string // "etcd", "consul"
	RemoteEndpoint string // Remote endpoint URL
	RemotePath     string // Path in remote config

	// Watching for changes
	WatchConfig bool

	// Default values
	Defaults map[string]interface{}
}

// DefaultConfig returns sensible defaults
func DefaultConfig() *Config {
	return &Config{
		ConfigName:   "config",
		ConfigPath:   ".",
		ConfigType:   "yaml",
		AutomaticEnv: true,
		EnvPrefix:    "APP",
		WatchConfig:  false,
	}
}

// NewDriver creates a new configuration driver
func NewDriver(cfg *Config) (*Driver, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	v := viper.New()

	// Set config file
	if cfg.ConfigFile != "" {
		v.SetConfigFile(cfg.ConfigFile)
	} else {
		v.SetConfigName(cfg.ConfigName)
		v.SetConfigType(cfg.ConfigType)
		v.AddConfigPath(cfg.ConfigPath)

		// Add additional paths
		for _, path := range cfg.ConfigPaths {
			v.AddConfigPath(path)
		}
	}

	// Environment variables
	if cfg.AutomaticEnv {
		v.AutomaticEnv()
		if cfg.EnvPrefix != "" {
			v.SetEnvPrefix(cfg.EnvPrefix)
		}
		if cfg.EnvKeyReplacer != "" {
			v.SetEnvKeyReplacer(strings.NewReplacer(cfg.EnvKeyReplacer, "_"))
		} else {
			v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
		}
	}

	// Set defaults
	for key, value := range cfg.Defaults {
		v.SetDefault(key, value)
	}

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		// Config file not required if using env vars or defaults
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
	}

	driver := &Driver{
		viper:    v,
		config:   cfg,
		onChange: make([]func(string, interface{}), 0),
	}

	// Watch for changes
	if cfg.WatchConfig {
		v.WatchConfig()
		v.OnConfigChange(func(e fsnotify.Event) {
			driver.mu.RLock()
			callbacks := driver.onChange
			driver.mu.RUnlock()

			// Notify all callbacks
			for _, callback := range callbacks {
				callback("", nil) // Full config changed
			}
		})
	}

	return driver, nil
}

// Get returns a value by key
func (d *Driver) Get(key string) interface{} {
	return d.viper.Get(key)
}

// GetString returns string value
func (d *Driver) GetString(key string) string {
	return d.viper.GetString(key)
}

// GetInt returns int value
func (d *Driver) GetInt(key string) int {
	return d.viper.GetInt(key)
}

// GetInt64 returns int64 value
func (d *Driver) GetInt64(key string) int64 {
	return d.viper.GetInt64(key)
}

// GetFloat64 returns float64 value
func (d *Driver) GetFloat64(key string) float64 {
	return d.viper.GetFloat64(key)
}

// GetBool returns bool value
func (d *Driver) GetBool(key string) bool {
	return d.viper.GetBool(key)
}

// GetStringSlice returns []string value
func (d *Driver) GetStringSlice(key string) []string {
	return d.viper.GetStringSlice(key)
}

// GetStringMap returns map[string]interface{} value
func (d *Driver) GetStringMap(key string) map[string]interface{} {
	return d.viper.GetStringMap(key)
}

// GetStringMapString returns map[string]string value
func (d *Driver) GetStringMapString(key string) map[string]string {
	return d.viper.GetStringMapString(key)
}

// GetDuration returns time.Duration value
func (d *Driver) GetDuration(key string) time.Duration {
	return d.viper.GetDuration(key)
}

// GetTime returns time.Time value
func (d *Driver) GetTime(key string) time.Time {
	return d.viper.GetTime(key)
}

// IsSet checks if key is set
func (d *Driver) IsSet(key string) bool {
	return d.viper.IsSet(key)
}

// Set sets a value
func (d *Driver) Set(key string, value interface{}) {
	d.viper.Set(key, value)
}

// SetDefault sets default value
func (d *Driver) SetDefault(key string, value interface{}) {
	d.viper.SetDefault(key, value)
}

// AllKeys returns all keys
func (d *Driver) AllKeys() []string {
	return d.viper.AllKeys()
}

// AllSettings returns all settings
func (d *Driver) AllSettings() map[string]interface{} {
	return d.viper.AllSettings()
}

// Unmarshal unmarshals config into struct
func (d *Driver) Unmarshal(rawVal interface{}) error {
	return d.viper.Unmarshal(rawVal)
}

// UnmarshalKey unmarshals config key into struct
func (d *Driver) UnmarshalKey(key string, rawVal interface{}) error {
	return d.viper.UnmarshalKey(key, rawVal)
}

// Sub returns sub-config for key
func (d *Driver) Sub(key string) *Driver {
	subViper := d.viper.Sub(key)
	if subViper == nil {
		return nil
	}

	return &Driver{
		viper:    subViper,
		config:   d.config,
		onChange: d.onChange,
	}
}

// Reload reloads configuration from file
func (d *Driver) Reload() error {
	return d.viper.ReadInConfig()
}

// OnChange registers callback for config changes
func (d *Driver) OnChange(callback func(key string, value interface{})) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.onChange = append(d.onChange, callback)
}

// WriteConfig writes current config to file
func (d *Driver) WriteConfig() error {
	return d.viper.WriteConfig()
}

// SafeWriteConfig writes config to file (only if doesn't exist)
func (d *Driver) SafeWriteConfig() error {
	return d.viper.SafeWriteConfig()
}

// WriteConfigAs writes config to specified file
func (d *Driver) WriteConfigAs(filename string) error {
	return d.viper.WriteConfigAs(filename)
}

// GetViper returns underlying Viper instance
func (d *Driver) GetViper() *viper.Viper {
	return d.viper
}

// MergeConfig merges another config file
func (d *Driver) MergeConfig(in interface{}) error {
	return d.viper.MergeConfig(in.(interface{ Read([]byte) (int, error) }))
}

// ReadRemoteConfig reads config from remote source
func (d *Driver) ReadRemoteConfig() error {
	if d.config.RemoteProvider == "" {
		return fmt.Errorf("remote provider not configured")
	}

	// This requires viper remote config support
	// Implementation depends on remote provider
	return fmt.Errorf("remote config not yet implemented")
}

// GetWithDefault returns value or default if not found
func (d *Driver) GetWithDefault(key string, defaultValue interface{}) interface{} {
	if d.IsSet(key) {
		return d.Get(key)
	}
	return defaultValue
}

// GetStringWithDefault returns string value or default
func (d *Driver) GetStringWithDefault(key string, defaultValue string) string {
	if d.IsSet(key) {
		return d.GetString(key)
	}
	return defaultValue
}

// GetIntWithDefault returns int value or default
func (d *Driver) GetIntWithDefault(key string, defaultValue int) int {
	if d.IsSet(key) {
		return d.GetInt(key)
	}
	return defaultValue
}

// GetBoolWithDefault returns bool value or default
func (d *Driver) GetBoolWithDefault(key string, defaultValue bool) bool {
	if d.IsSet(key) {
		return d.GetBool(key)
	}
	return defaultValue
}

// MustGet returns value or panics if not found
func (d *Driver) MustGet(key string) interface{} {
	if !d.IsSet(key) {
		panic(fmt.Sprintf("required config key not found: %s", key))
	}
	return d.Get(key)
}

// MustGetString returns string value or panics if not found
func (d *Driver) MustGetString(key string) string {
	if !d.IsSet(key) {
		panic(fmt.Sprintf("required config key not found: %s", key))
	}
	return d.GetString(key)
}

// MustGetInt returns int value or panics if not found
func (d *Driver) MustGetInt(key string) int {
	if !d.IsSet(key) {
		panic(fmt.Sprintf("required config key not found: %s", key))
	}
	return d.GetInt(key)
}

// BindEnv binds specific env var to config key
func (d *Driver) BindEnv(input ...string) error {
	return d.viper.BindEnv(input...)
}

// Debug prints all config for debugging
func (d *Driver) Debug() map[string]interface{} {
	return d.AllSettings()
}
