package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewDriver(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	content := `
server:
  port: 8080
  host: localhost
database:
  host: db.example.com
  port: 5432
`
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{
		ConfigFile: configFile,
		ConfigType: "yaml",
	}

	driver, err := NewDriver(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if driver == nil {
		t.Fatal("expected driver to be non-nil")
	}
}

func TestGetString(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	content := `
app:
  name: test-app
  version: 1.0.0
`
	os.WriteFile(configFile, []byte(content), 0644)

	driver, _ := NewDriver(&Config{
		ConfigFile: configFile,
		ConfigType: "yaml",
	})

	name := driver.GetString("app.name")
	if name != "test-app" {
		t.Errorf("expected test-app, got %s", name)
	}

	version := driver.GetString("app.version")
	if version != "1.0.0" {
		t.Errorf("expected 1.0.0, got %s", version)
	}
}

func TestGetInt(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	content := `
server:
  port: 8080
  maxConnections: 1000
`
	os.WriteFile(configFile, []byte(content), 0644)

	driver, _ := NewDriver(&Config{
		ConfigFile: configFile,
		ConfigType: "yaml",
	})

	port := driver.GetInt("server.port")
	if port != 8080 {
		t.Errorf("expected 8080, got %d", port)
	}

	maxConn := driver.GetInt("server.maxConnections")
	if maxConn != 1000 {
		t.Errorf("expected 1000, got %d", maxConn)
	}
}

func TestGetBool(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	content := `
features:
  enableCache: true
  enableMetrics: false
`
	os.WriteFile(configFile, []byte(content), 0644)

	driver, _ := NewDriver(&Config{
		ConfigFile: configFile,
		ConfigType: "yaml",
	})

	if !driver.GetBool("features.enableCache") {
		t.Error("expected enableCache to be true")
	}

	if driver.GetBool("features.enableMetrics") {
		t.Error("expected enableMetrics to be false")
	}
}

func TestGetFloat64(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	content := `
pricing:
  basePrice: 9.99
  taxRate: 0.15
`
	os.WriteFile(configFile, []byte(content), 0644)

	driver, _ := NewDriver(&Config{
		ConfigFile: configFile,
		ConfigType: "yaml",
	})

	price := driver.GetFloat64("pricing.basePrice")
	if price != 9.99 {
		t.Errorf("expected 9.99, got %f", price)
	}
}

func TestGetStringSlice(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	content := `
allowed:
  origins:
    - http://localhost:3000
    - http://localhost:8080
    - https://app.example.com
`
	os.WriteFile(configFile, []byte(content), 0644)

	driver, _ := NewDriver(&Config{
		ConfigFile: configFile,
		ConfigType: "yaml",
	})

	origins := driver.GetStringSlice("allowed.origins")
	if len(origins) != 3 {
		t.Errorf("expected 3 origins, got %d", len(origins))
	}

	if origins[0] != "http://localhost:3000" {
		t.Errorf("expected http://localhost:3000, got %s", origins[0])
	}
}

func TestGetDuration(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	content := `
timeouts:
  read: 30s
  write: 1m
  idle: 2h
`
	os.WriteFile(configFile, []byte(content), 0644)

	driver, _ := NewDriver(&Config{
		ConfigFile: configFile,
		ConfigType: "yaml",
	})

	readTimeout := driver.GetDuration("timeouts.read")
	if readTimeout != 30*time.Second {
		t.Errorf("expected 30s, got %v", readTimeout)
	}

	writeTimeout := driver.GetDuration("timeouts.write")
	if writeTimeout != time.Minute {
		t.Errorf("expected 1m, got %v", writeTimeout)
	}
}

func TestEnvironmentVariables(t *testing.T) {
	// Set env vars
	os.Setenv("APP_SERVER_PORT", "9000")
	os.Setenv("APP_DATABASE_HOST", "prod-db.example.com")
	defer func() {
		os.Unsetenv("APP_SERVER_PORT")
		os.Unsetenv("APP_DATABASE_HOST")
	}()

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	content := `
server:
  port: 8080
database:
  host: localhost
`
	os.WriteFile(configFile, []byte(content), 0644)

	driver, _ := NewDriver(&Config{
		ConfigFile:   configFile,
		ConfigType:   "yaml",
		AutomaticEnv: true,
		EnvPrefix:    "APP",
	})

	// Env var should override file config
	port := driver.GetInt("server.port")
	if port != 9000 {
		t.Errorf("expected env var 9000, got %d", port)
	}

	dbHost := driver.GetString("database.host")
	if dbHost != "prod-db.example.com" {
		t.Errorf("expected env var prod-db.example.com, got %s", dbHost)
	}
}

func TestDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	content := `
server:
  port: 8080
`
	os.WriteFile(configFile, []byte(content), 0644)

	driver, _ := NewDriver(&Config{
		ConfigFile: configFile,
		ConfigType: "yaml",
		Defaults: map[string]interface{}{
			"server.host":   "0.0.0.0",
			"database.host": "localhost",
			"database.port": 5432,
			"cache.enabled": true,
		},
	})

	// Should get default values
	host := driver.GetString("server.host")
	if host != "0.0.0.0" {
		t.Errorf("expected default 0.0.0.0, got %s", host)
	}

	dbPort := driver.GetInt("database.port")
	if dbPort != 5432 {
		t.Errorf("expected default 5432, got %d", dbPort)
	}

	cacheEnabled := driver.GetBool("cache.enabled")
	if !cacheEnabled {
		t.Error("expected default true")
	}
}

func TestIsSet(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	content := `
server:
  port: 8080
`
	os.WriteFile(configFile, []byte(content), 0644)

	driver, _ := NewDriver(&Config{
		ConfigFile: configFile,
		ConfigType: "yaml",
	})

	if !driver.IsSet("server.port") {
		t.Error("expected server.port to be set")
	}

	if driver.IsSet("database.host") {
		t.Error("expected database.host to not be set")
	}
}

func TestSet(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	content := `
server:
  port: 8080
`
	os.WriteFile(configFile, []byte(content), 0644)

	driver, _ := NewDriver(&Config{
		ConfigFile: configFile,
		ConfigType: "yaml",
	})

	// Set new value
	driver.Set("server.host", "localhost")

	host := driver.GetString("server.host")
	if host != "localhost" {
		t.Errorf("expected localhost, got %s", host)
	}

	// Override existing value
	driver.Set("server.port", 9000)

	port := driver.GetInt("server.port")
	if port != 9000 {
		t.Errorf("expected 9000, got %d", port)
	}
}

func TestUnmarshal(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	content := `
server:
  port: 8080
  host: localhost
database:
  host: db.example.com
  port: 5432
  name: mydb
`
	os.WriteFile(configFile, []byte(content), 0644)

	driver, _ := NewDriver(&Config{
		ConfigFile: configFile,
		ConfigType: "yaml",
	})

	type ServerConfig struct {
		Port int    `mapstructure:"port"`
		Host string `mapstructure:"host"`
	}

	type DatabaseConfig struct {
		Host string `mapstructure:"host"`
		Port int    `mapstructure:"port"`
		Name string `mapstructure:"name"`
	}

	type AppConfig struct {
		Server   ServerConfig   `mapstructure:"server"`
		Database DatabaseConfig `mapstructure:"database"`
	}

	var config AppConfig
	err := driver.Unmarshal(&config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if config.Server.Port != 8080 {
		t.Errorf("expected port 8080, got %d", config.Server.Port)
	}

	if config.Database.Name != "mydb" {
		t.Errorf("expected database name mydb, got %s", config.Database.Name)
	}
}

func TestUnmarshalKey(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	content := `
database:
  host: db.example.com
  port: 5432
  username: admin
  password: secret
`
	os.WriteFile(configFile, []byte(content), 0644)

	driver, _ := NewDriver(&Config{
		ConfigFile: configFile,
		ConfigType: "yaml",
	})

	type DatabaseConfig struct {
		Host     string `mapstructure:"host"`
		Port     int    `mapstructure:"port"`
		Username string `mapstructure:"username"`
		Password string `mapstructure:"password"`
	}

	var dbConfig DatabaseConfig
	err := driver.UnmarshalKey("database", &dbConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if dbConfig.Host != "db.example.com" {
		t.Errorf("expected db.example.com, got %s", dbConfig.Host)
	}

	if dbConfig.Port != 5432 {
		t.Errorf("expected 5432, got %d", dbConfig.Port)
	}
}

func TestSub(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	content := `
database:
  host: db.example.com
  port: 5432
  pool:
    min: 5
    max: 20
`
	os.WriteFile(configFile, []byte(content), 0644)

	driver, _ := NewDriver(&Config{
		ConfigFile: configFile,
		ConfigType: "yaml",
	})

	// Get sub-config
	dbConfig := driver.Sub("database")
	if dbConfig == nil {
		t.Fatal("expected sub-config to be non-nil")
	}

	host := dbConfig.GetString("host")
	if host != "db.example.com" {
		t.Errorf("expected db.example.com, got %s", host)
	}

	poolConfig := dbConfig.Sub("pool")
	if poolConfig == nil {
		t.Fatal("expected pool sub-config to be non-nil")
	}

	minPool := poolConfig.GetInt("min")
	if minPool != 5 {
		t.Errorf("expected 5, got %d", minPool)
	}
}

func TestAllKeys(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	content := `
server:
  port: 8080
database:
  host: localhost
cache:
  enabled: true
`
	os.WriteFile(configFile, []byte(content), 0644)

	driver, _ := NewDriver(&Config{
		ConfigFile: configFile,
		ConfigType: "yaml",
	})

	keys := driver.AllKeys()
	if len(keys) < 3 {
		t.Errorf("expected at least 3 keys, got %d", len(keys))
	}
}

func TestGetWithDefault(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	content := `
server:
  port: 8080
`
	os.WriteFile(configFile, []byte(content), 0644)

	driver, _ := NewDriver(&Config{
		ConfigFile: configFile,
		ConfigType: "yaml",
	})

	// Existing key
	port := driver.GetIntWithDefault("server.port", 3000)
	if port != 8080 {
		t.Errorf("expected 8080, got %d", port)
	}

	// Non-existing key with default
	host := driver.GetStringWithDefault("server.host", "0.0.0.0")
	if host != "0.0.0.0" {
		t.Errorf("expected default 0.0.0.0, got %s", host)
	}
}

func TestMustGet(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	content := `
server:
  port: 8080
`
	os.WriteFile(configFile, []byte(content), 0644)

	driver, _ := NewDriver(&Config{
		ConfigFile: configFile,
		ConfigType: "yaml",
	})

	// Should not panic for existing key
	port := driver.MustGetInt("server.port")
	if port != 8080 {
		t.Errorf("expected 8080, got %d", port)
	}

	// Should panic for non-existing key
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for missing key")
		}
	}()

	driver.MustGetString("database.host")
}

func TestJSONConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.json")

	content := `{
  "server": {
    "port": 8080,
    "host": "localhost"
  },
  "database": {
    "host": "db.example.com"
  }
}`
	os.WriteFile(configFile, []byte(content), 0644)

	driver, err := NewDriver(&Config{
		ConfigFile: configFile,
		ConfigType: "json",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	port := driver.GetInt("server.port")
	if port != 8080 {
		t.Errorf("expected 8080, got %d", port)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.ConfigName != "config" {
		t.Errorf("expected default config name to be config, got %s", cfg.ConfigName)
	}

	if cfg.ConfigType != "yaml" {
		t.Errorf("expected default config type to be yaml, got %s", cfg.ConfigType)
	}

	if !cfg.AutomaticEnv {
		t.Error("expected AutomaticEnv to be true by default")
	}
}
