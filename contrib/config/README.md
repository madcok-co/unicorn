# Configuration Management with Viper

Production-ready configuration management for Unicorn Framework using Viper. Supports multiple config sources, formats, and hot reload.

## Features

- **Multiple Config Sources**
  - Config files (YAML, JSON, TOML, HCL, ENV)
  - Environment variables
  - Default values
  - Remote config (etcd, consul)

- **Hot Reload** - Watch for config file changes
- **Type-Safe Getters** - Get values with type safety
- **Nested Configuration** - Deep nested config support
- **Environment Override** - Env vars override file config
- **Unmarshal to Structs** - Map config to Go structs
- **Sub-Configurations** - Work with config sections

## Installation

```bash
go get github.com/madcok-co/unicorn/contrib/config
go get github.com/spf13/viper
```

## Quick Start

### Basic Usage

```go
import (
    "github.com/madcok-co/unicorn/contrib/config"
)

func main() {
    // Initialize config
    cfg, err := config.NewDriver(&config.Config{
        ConfigName: "app",           // config file name (without extension)
        ConfigPath: "./configs",      // config file path
        ConfigType: "yaml",          // yaml, json, toml, etc.
    })
    if err != nil {
        panic(err)
    }

    // Get values
    port := cfg.GetInt("server.port")
    dbHost := cfg.GetString("database.host")
    enableCache := cfg.GetBool("cache.enabled")
    
    fmt.Printf("Server running on port %d\n", port)
}
```

### Config File Example

**config.yaml:**
```yaml
server:
  port: 8080
  host: 0.0.0.0
  timeout: 30s

database:
  host: localhost
  port: 5432
  name: myapp
  pool:
    min: 5
    max: 20

cache:
  enabled: true
  ttl: 5m

features:
  experimental: false
  analytics: true
```

## Configuration Sources

### 1. Config File

```go
cfg, err := config.NewDriver(&config.Config{
    ConfigFile: "/etc/myapp/config.yaml",
    ConfigType: "yaml",
})
```

Or specify name and search paths:

```go
cfg, err := config.NewDriver(&config.Config{
    ConfigName: "app",
    ConfigType: "yaml",
    ConfigPaths: []string{
        "/etc/myapp",
        "$HOME/.myapp",
        ".",
    },
})
```

### 2. Environment Variables

```go
cfg, err := config.NewDriver(&config.Config{
    ConfigName:   "app",
    ConfigPath:   ".",
    ConfigType:   "yaml",
    AutomaticEnv: true,        // Read env vars
    EnvPrefix:    "MYAPP",     // Prefix for env vars
})

// Config keys: server.port, database.host
// Env vars: MYAPP_SERVER_PORT, MYAPP_DATABASE_HOST
```

**Example:**
```bash
export MYAPP_SERVER_PORT=9000
export MYAPP_DATABASE_HOST=prod-db.example.com
```

Env vars override config file values.

### 3. Default Values

```go
cfg, err := config.NewDriver(&config.Config{
    ConfigName: "app",
    ConfigPath: ".",
    ConfigType: "yaml",
    Defaults: map[string]interface{}{
        "server.port":    8080,
        "server.host":    "0.0.0.0",
        "cache.enabled":  false,
        "cache.ttl":      "10m",
    },
})
```

## Type-Safe Getters

```go
// String
appName := cfg.GetString("app.name")

// Integer
port := cfg.GetInt("server.port")
maxConn := cfg.GetInt64("server.maxConnections")

// Float
price := cfg.GetFloat64("pricing.basePrice")

// Boolean
enabled := cfg.GetBool("features.analytics")

// Duration
timeout := cfg.GetDuration("server.timeout") // 30s, 5m, 2h

// Time
startTime := cfg.GetTime("deployment.startTime")

// String Slice
origins := cfg.GetStringSlice("cors.allowedOrigins")

// Map
metadata := cfg.GetStringMap("app.metadata")
headers := cfg.GetStringMapString("http.headers")

// Generic
value := cfg.Get("custom.field")
```

## Get with Defaults

```go
// Returns default if key not found
port := cfg.GetIntWithDefault("server.port", 3000)
host := cfg.GetStringWithDefault("server.host", "localhost")
debug := cfg.GetBoolWithDefault("debug", false)
```

## Must Get (Panic if Missing)

```go
// Panics if key not found - use for required config
apiKey := cfg.MustGetString("api.key")
dbHost := cfg.MustGetString("database.host")
```

## Unmarshal to Structs

### Full Config

```go
type ServerConfig struct {
    Port    int    `mapstructure:"port"`
    Host    string `mapstructure:"host"`
    Timeout string `mapstructure:"timeout"`
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
if err := cfg.Unmarshal(&config); err != nil {
    log.Fatal(err)
}

fmt.Printf("Server: %s:%d\n", config.Server.Host, config.Server.Port)
```

### Specific Key

```go
type DatabaseConfig struct {
    Host     string `mapstructure:"host"`
    Port     int    `mapstructure:"port"`
    Username string `mapstructure:"username"`
    Password string `mapstructure:"password"`
}

var dbConfig DatabaseConfig
if err := cfg.UnmarshalKey("database", &dbConfig); err != nil {
    log.Fatal(err)
}
```

## Sub-Configurations

Work with specific config sections:

```go
// Get database sub-config
dbConfig := cfg.Sub("database")

host := dbConfig.GetString("host")      // database.host
port := dbConfig.GetInt("port")          // database.port

// Get pool sub-config
poolConfig := dbConfig.Sub("pool")
minPool := poolConfig.GetInt("min")      // database.pool.min
```

## Hot Reload / Watch Config

```go
cfg, err := config.NewDriver(&config.Config{
    ConfigFile:  "config.yaml",
    ConfigType:  "yaml",
    WatchConfig: true,  // Enable watching
})

// Register callback for changes
cfg.OnChange(func(key string, value interface{}) {
    log.Println("Config changed, reloading...")
    
    // Reload your services with new config
    newPort := cfg.GetInt("server.port")
    // ... restart server with new port
})
```

## Set Values Programmatically

```go
// Set value at runtime
cfg.Set("server.port", 9000)

// Set default (won't override existing)
cfg.SetDefault("cache.ttl", "5m")

// Write to file
cfg.WriteConfig()

// Write to specific file
cfg.WriteConfigAs("config.production.yaml")
```

## Environment-Specific Configs

### Multiple Config Files

```go
env := os.Getenv("APP_ENV") // dev, staging, prod
if env == "" {
    env = "dev"
}

cfg, err := config.NewDriver(&config.Config{
    ConfigName: fmt.Sprintf("config.%s", env),
    ConfigPath: "./configs",
    ConfigType: "yaml",
})
```

**File structure:**
```
configs/
  ├── config.dev.yaml
  ├── config.staging.yaml
  └── config.prod.yaml
```

### Merge Configs

```go
// Base config
cfg, _ := config.NewDriver(&config.Config{
    ConfigFile: "config.yaml",
})

// Merge environment-specific config
envConfig, _ := os.Open("config.prod.yaml")
cfg.MergeConfig(envConfig)
```

## Complete Example with Unicorn

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/madcok-co/unicorn/contrib/config"
    "github.com/madcok-co/unicorn/core/pkg/app"
    "github.com/madcok-co/unicorn/core/pkg/context"
    httpAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/http"
)

type Config struct {
    Server struct {
        Port int    `mapstructure:"port"`
        Host string `mapstructure:"host"`
    } `mapstructure:"server"`
    
    Database struct {
        Host     string `mapstructure:"host"`
        Port     int    `mapstructure:"port"`
        Name     string `mapstructure:"name"`
        Username string `mapstructure:"username"`
        Password string `mapstructure:"password"`
    } `mapstructure:"database"`
    
    Cache struct {
        Enabled bool   `mapstructure:"enabled"`
        TTL     string `mapstructure:"ttl"`
    } `mapstructure:"cache"`
}

func main() {
    // Load config
    cfg, err := config.NewDriver(&config.Config{
        ConfigName:   "app",
        ConfigPath:   "./configs",
        ConfigType:   "yaml",
        AutomaticEnv: true,
        EnvPrefix:    "APP",
        Defaults: map[string]interface{}{
            "server.port": 8080,
            "server.host": "0.0.0.0",
        },
    })
    if err != nil {
        log.Fatal(err)
    }

    // Unmarshal to struct
    var appConfig Config
    if err := cfg.Unmarshal(&appConfig); err != nil {
        log.Fatal(err)
    }

    // Create Unicorn app
    application := app.New(&app.Config{
        Name:       "my-app",
        EnableHTTP: true,
        HTTP: &httpAdapter.Config{
            Port: appConfig.Server.Port,
        },
    })

    // Register config as service
    application.RegisterService(cfg, "config")

    // Register handlers
    application.RegisterHandler(GetConfig).HTTP("GET", "/config").Done()
    application.RegisterHandler(GetHealth).HTTP("GET", "/health").Done()

    log.Printf("Server starting on %s:%d", appConfig.Server.Host, appConfig.Server.Port)
    application.Start()
}

// Handler using config
func GetConfig(ctx *context.Context, req struct{}) (map[string]interface{}, error) {
    cfg := ctx.Service("config").(*config.Driver)
    
    return map[string]interface{}{
        "server": cfg.GetStringMap("server"),
        "cache":  cfg.GetStringMap("cache"),
    }, nil
}

func GetHealth(ctx *context.Context, req struct{}) (map[string]string, error) {
    cfg := ctx.Service("config").(*config.Driver)
    
    return map[string]string{
        "status":  "healthy",
        "version": cfg.GetString("app.version"),
    }, nil
}
```

## Multiple Formats

### YAML
```yaml
server:
  port: 8080
  host: localhost
```

### JSON
```json
{
  "server": {
    "port": 8080,
    "host": "localhost"
  }
}
```

### TOML
```toml
[server]
port = 8080
host = "localhost"
```

### ENV
```env
SERVER_PORT=8080
SERVER_HOST=localhost
```

## Utility Methods

```go
// Check if key exists
if cfg.IsSet("database.host") {
    // ...
}

// Get all keys
keys := cfg.AllKeys()

// Get all settings
settings := cfg.AllSettings()

// Debug config
debug := cfg.Debug()
fmt.Printf("%+v\n", debug)

// Bind specific env var
cfg.BindEnv("api.key", "API_KEY")

// Reload config from file
cfg.Reload()
```

## Best Practices

1. **Use Environment Variables for Secrets**
   ```go
   // DON'T store secrets in config files
   // DO use environment variables
   dbPassword := cfg.GetString("database.password") // from env var
   ```

2. **Validate Required Config**
   ```go
   requiredKeys := []string{
       "database.host",
       "database.name",
       "api.key",
   }
   
   for _, key := range requiredKeys {
       if !cfg.IsSet(key) {
           log.Fatalf("Required config missing: %s", key)
       }
   }
   ```

3. **Use Struct Unmarshal for Type Safety**
   ```go
   // GOOD - Type safe
   var config AppConfig
   cfg.Unmarshal(&config)
   port := config.Server.Port
   
   // OK but less safe
   port := cfg.GetInt("server.port")
   ```

4. **Separate Config by Environment**
   ```
   configs/
     ├── config.yaml          # Base config
     ├── config.dev.yaml      # Development overrides
     ├── config.staging.yaml  # Staging overrides
     └── config.prod.yaml     # Production overrides
   ```

5. **Use Defaults for Optional Config**
   ```go
   timeout := cfg.GetDurationWithDefault("server.timeout", 30*time.Second)
   ```

## Testing

```bash
# Run tests
cd contrib/config
go test -v

# Run with coverage
go test -v -cover
```

## Common Patterns

### Database Configuration
```yaml
database:
  host: ${DB_HOST:localhost}
  port: ${DB_PORT:5432}
  name: myapp
  username: ${DB_USER}
  password: ${DB_PASSWORD}
  pool:
    min: 5
    max: 20
  ssl:
    enabled: true
    mode: verify-full
```

### Feature Flags
```yaml
features:
  newUI: false
  betaFeatures: true
  experimentalAPI: false
```

```go
if cfg.GetBool("features.newUI") {
    // Use new UI
}
```

### Multi-Tenant Config
```yaml
tenants:
  tenant1:
    database: tenant1_db
    features:
      analytics: true
  tenant2:
    database: tenant2_db
    features:
      analytics: false
```

## Error Handling

```go
cfg, err := config.NewDriver(&config.Config{
    ConfigFile: "config.yaml",
})
if err != nil {
    // Config file not found or parse error
    log.Printf("Config error: %v", err)
    
    // Fall back to defaults
    cfg, _ = config.NewDriver(&config.Config{
        Defaults: map[string]interface{}{
            "server.port": 8080,
        },
    })
}
```

## License

MIT License - see LICENSE file for details
