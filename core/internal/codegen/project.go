package codegen

import (
	"fmt"
	"os"
	"path/filepath"
)

// GenerateProject generates a new Unicorn project
func GenerateProject(name string) error {
	// Create project directory
	if err := os.MkdirAll(name, 0755); err != nil {
		return fmt.Errorf("failed to create project directory: %w", err)
	}

	// Create directory structure
	dirs := []string{
		"cmd/server",
		"internal/handlers",
		"internal/models",
		"internal/services",
		"config",
		"migrations",
	}

	for _, dir := range dirs {
		path := filepath.Join(name, dir)
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Generate files
	moduleName := fmt.Sprintf("github.com/yourname/%s", name)

	files := map[string]string{
		"go.mod":                      generateGoMod(moduleName),
		"cmd/server/main.go":          generateMainGo(moduleName),
		"config/config.go":            generateConfigGo(moduleName),
		"internal/handlers/health.go": generateHealthHandler(moduleName),
		".env.example":                generateEnvExample(),
		".gitignore":                  generateGitignore(),
	}

	for path, content := range files {
		fullPath := filepath.Join(name, path)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to create file %s: %w", path, err)
		}
	}

	return nil
}

func generateGoMod(moduleName string) string {
	return fmt.Sprintf(`module %s

go 1.21

require (
	github.com/madcok-co/unicorn v0.1.0
)
`, moduleName)
}

func generateMainGo(moduleName string) string {
	return fmt.Sprintf(`package main

import (
	"log"

	"github.com/madcok-co/unicorn/core/pkg/app"
	"%s/config"
	"%s/internal/handlers"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Create Unicorn app
	application := app.New(&app.Config{
		Name:       cfg.AppName,
		Version:    cfg.AppVersion,
		EnableHTTP: true,
		HTTP: &http.Config{
			Host: cfg.HTTPHost,
			Port: cfg.HTTPPort,
		},
	})

	// Setup infrastructure (uncomment as needed)
	// application.SetDB(yourDBAdapter)
	// application.SetCache(yourCacheAdapter)
	// application.SetLogger(yourLoggerAdapter)

	// Register handlers
	handlers.RegisterAll(application)

	// Start application
	log.Printf("Starting %%s v%%s on %%s:%%d", cfg.AppName, cfg.AppVersion, cfg.HTTPHost, cfg.HTTPPort)
	if err := application.Start(); err != nil {
		log.Fatalf("Application error: %%v", err)
	}
}
`, moduleName, moduleName)
}

func generateConfigGo(moduleName string) string {
	return `package config

import (
	"os"
	"strconv"
)

type Config struct {
	AppName    string
	AppVersion string
	Env        string

	// HTTP
	HTTPHost string
	HTTPPort int

	// Database
	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string

	// Redis
	RedisHost     string
	RedisPort     int
	RedisPassword string

	// Kafka
	KafkaBrokers string
	KafkaGroupID string
}

func Load() *Config {
	return &Config{
		AppName:    getEnv("APP_NAME", "unicorn-app"),
		AppVersion: getEnv("APP_VERSION", "1.0.0"),
		Env:        getEnv("APP_ENV", "development"),

		HTTPHost: getEnv("HTTP_HOST", "0.0.0.0"),
		HTTPPort: getEnvInt("HTTP_PORT", 8080),

		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnvInt("DB_PORT", 5432),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", ""),
		DBName:     getEnv("DB_NAME", "app"),

		RedisHost:     getEnv("REDIS_HOST", "localhost"),
		RedisPort:     getEnvInt("REDIS_PORT", 6379),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),

		KafkaBrokers: getEnv("KAFKA_BROKERS", "localhost:9092"),
		KafkaGroupID: getEnv("KAFKA_GROUP_ID", "unicorn-consumer"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}
`
}

func generateHealthHandler(moduleName string) string {
	return fmt.Sprintf(`package handlers

import (
	"github.com/madcok-co/unicorn/core/pkg/app"
	"github.com/madcok-co/unicorn/core/pkg/context"
)

// HealthResponse for health check endpoint
type HealthResponse struct {
	Status  string ` + "`json:\"status\"`" + `
	Version string ` + "`json:\"version\"`" + `
}

// HealthCheck handler
func HealthCheck(ctx *context.Context) (*HealthResponse, error) {
	return &HealthResponse{
		Status:  "healthy",
		Version: "1.0.0",
	}, nil
}

// RegisterAll registers all handlers
func RegisterAll(application *app.App) {
	// Health check endpoint
	application.RegisterHandler(HealthCheck).
		Named("health").
		HTTP("GET", "/health").
		Done()

	// Add more handlers here
	// application.RegisterHandler(YourHandler).
	//     Named("your-handler").
	//     HTTP("POST", "/your-endpoint").
	//     Kafka("your-topic").
	//     Done()
}
`)
}

func generateEnvExample() string {
	return `# Application
APP_NAME=unicorn-app
APP_VERSION=1.0.0
APP_ENV=development

# HTTP Server
HTTP_HOST=0.0.0.0
HTTP_PORT=8080

# Database (PostgreSQL)
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=secret
DB_NAME=app

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=

# Kafka
KAFKA_BROKERS=localhost:9092
KAFKA_GROUP_ID=unicorn-consumer
`
}

func generateGitignore() string {
	return `# Binaries
*.exe
*.exe~
*.dll
*.so
*.dylib
/bin/
/build/

# Test binary
*.test

# Output of the go coverage tool
*.out

# Dependency directories
/vendor/

# IDE
.idea/
.vscode/
*.swp
*.swo

# Environment
.env
.env.local

# OS
.DS_Store
Thumbs.db

# Logs
*.log
logs/
`
}
