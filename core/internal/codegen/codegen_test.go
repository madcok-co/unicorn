package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// toPascalCase
// ---------------------------------------------------------------------------

func TestToPascalCase(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "empty", input: "", expected: ""},
		{name: "lowercase", input: "hello", expected: "Hello"},
		{name: "snake_case", input: "hello_world", expected: "HelloWorld"},
		{name: "kebab-case", input: "hello-world", expected: "HelloWorld"},
		{name: "already_pascal", input: "HelloWorld", expected: "HelloWorld"},
		{name: "single_char", input: "a", expected: "A"},
		{name: "multi_snake", input: "hello_world_test", expected: "HelloWorldTest"},
		{name: "mixed_kebab_snake", input: "hello-world_test", expected: "HelloWorldTest"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toPascalCase(tt.input)
			if got != tt.expected {
				t.Errorf("toPascalCase(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Content generators (unexported, tested in same package)
// ---------------------------------------------------------------------------

func TestGenerateHandlerContent(t *testing.T) {
	content := generateHandlerContent("user")

	if len(content) == 0 {
		t.Fatal("generateHandlerContent returned empty string")
	}

	// Should contain package declaration
	if !strings.Contains(content, "package handlers") {
		t.Error("expected 'package handlers' in content")
	}

	// Name should be interpolated as PascalCase in struct/function names
	if !strings.Contains(content, "CreateUserRequest") {
		t.Error("expected 'CreateUserRequest' in content")
	}
	if !strings.Contains(content, "UpdateUserRequest") {
		t.Error("expected 'UpdateUserRequest' in content")
	}
	if !strings.Contains(content, "UserResponse") {
		t.Error("expected 'UserResponse' in content")
	}
	if !strings.Contains(content, "UserListResponse") {
		t.Error("expected 'UserListResponse' in content")
	}
	if !strings.Contains(content, "CreateUser") {
		t.Error("expected 'CreateUser' function in content")
	}
	if !strings.Contains(content, "GetUser") {
		t.Error("expected 'GetUser' function in content")
	}
	if !strings.Contains(content, "ListUser") {
		t.Error("expected 'ListUser' function in content")
	}
	if !strings.Contains(content, "UpdateUser") {
		t.Error("expected 'UpdateUser' function in content")
	}
	if !strings.Contains(content, "DeleteUser") {
		t.Error("expected 'DeleteUser' function in content")
	}
	if !strings.Contains(content, "RegisterUserHandlers") {
		t.Error("expected 'RegisterUserHandlers' in content")
	}
}

func TestGenerateModelContent(t *testing.T) {
	content := generateModelContent("user")

	if len(content) == 0 {
		t.Fatal("generateModelContent returned empty string")
	}

	if !strings.Contains(content, "package models") {
		t.Error("expected 'package models' in content")
	}

	// Struct name should be PascalCase
	if !strings.Contains(content, "type User struct") {
		t.Error("expected 'type User struct' in content")
	}

	// TableName should use lowercased plural
	if !strings.Contains(content, "return \"users\"") {
		t.Error("expected TableName returning \"users\" in content")
	}

	// Hooks should be present
	if !strings.Contains(content, "func (m *User) TableName()") {
		t.Error("expected TableName method on User")
	}
	if !strings.Contains(content, "func (m *User) BeforeCreate()") {
		t.Error("expected BeforeCreate hook")
	}
	if !strings.Contains(content, "func (m *User) BeforeUpdate()") {
		t.Error("expected BeforeUpdate hook")
	}
}

func TestGenerateServiceContent(t *testing.T) {
	content := generateServiceContent("user")

	if len(content) == 0 {
		t.Fatal("generateServiceContent returned empty string")
	}

	if !strings.Contains(content, "package services") {
		t.Error("expected 'package services' in content")
	}

	// Service struct and constructor
	if !strings.Contains(content, "type UserService struct") {
		t.Error("expected 'type UserService struct' in content")
	}
	if !strings.Contains(content, "func NewUserService") {
		t.Error("expected 'func NewUserService' in content")
	}

	// Methods
	if !strings.Contains(content, "func (s *UserService) Create(") {
		t.Error("expected Create method")
	}
	if !strings.Contains(content, "func (s *UserService) GetByID(") {
		t.Error("expected GetByID method")
	}
	if !strings.Contains(content, "func (s *UserService) Update(") {
		t.Error("expected Update method")
	}
	if !strings.Contains(content, "func (s *UserService) Delete(") {
		t.Error("expected Delete method")
	}
	if !strings.Contains(content, "func (s *UserService) List(") {
		t.Error("expected List method")
	}
}

// ---------------------------------------------------------------------------
// GenerateHandler
// ---------------------------------------------------------------------------

func TestGenerateHandler(t *testing.T) {
	dir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current dir: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Fatalf("failed to restore dir: %v", err)
		}
	}()

	// -- First call: should succeed --
	err = GenerateHandler("product")
	if err != nil {
		t.Fatalf("unexpected error on first GenerateHandler: %v", err)
	}

	// Verify directory exists
	if _, err := os.Stat("internal/handlers"); os.IsNotExist(err) {
		t.Error("expected internal/handlers directory to exist")
	}

	// Verify file exists
	fpath := filepath.Join("internal", "handlers", "product.go")
	if _, err := os.Stat(fpath); os.IsNotExist(err) {
		t.Errorf("expected file %s to exist", fpath)
	}

	// Verify file content has PascalCase names
	data, err := os.ReadFile(fpath)
	if err != nil {
		t.Fatalf("failed to read generated file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "package handlers") {
		t.Error("generated handler missing package declaration")
	}
	if !strings.Contains(content, "CreateProductRequest") {
		t.Error("generated handler missing CreateProductRequest")
	}

	// -- Second call with same name: should error --
	err = GenerateHandler("product")
	if err == nil {
		t.Error("expected error on duplicate GenerateHandler, got nil")
	}
}

// ---------------------------------------------------------------------------
// GenerateModel
// ---------------------------------------------------------------------------

func TestGenerateModel(t *testing.T) {
	dir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current dir: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Fatalf("failed to restore dir: %v", err)
		}
	}()

	// -- First call: should succeed --
	err = GenerateModel("order")
	if err != nil {
		t.Fatalf("unexpected error on first GenerateModel: %v", err)
	}

	// Verify directory and file
	fpath := filepath.Join("internal", "models", "order.go")
	if _, err := os.Stat(fpath); os.IsNotExist(err) {
		t.Errorf("expected file %s to exist", fpath)
	}

	data, err := os.ReadFile(fpath)
	if err != nil {
		t.Fatalf("failed to read generated file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "type Order struct") {
		t.Error("generated model missing type Order struct")
	}
	if !strings.Contains(content, "return \"orders\"") {
		t.Error("generated model missing expected TableName")
	}

	// -- Second call: should error --
	err = GenerateModel("order")
	if err == nil {
		t.Error("expected error on duplicate GenerateModel, got nil")
	}
}

// ---------------------------------------------------------------------------
// GenerateService
// ---------------------------------------------------------------------------

func TestGenerateService(t *testing.T) {
	dir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current dir: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Fatalf("failed to restore dir: %v", err)
		}
	}()

	// -- First call: should succeed --
	err = GenerateService("payment")
	if err != nil {
		t.Fatalf("unexpected error on first GenerateService: %v", err)
	}

	fpath := filepath.Join("internal", "services", "payment.go")
	if _, err := os.Stat(fpath); os.IsNotExist(err) {
		t.Errorf("expected file %s to exist", fpath)
	}

	data, err := os.ReadFile(fpath)
	if err != nil {
		t.Fatalf("failed to read generated file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "type PaymentService struct") {
		t.Error("generated service missing PaymentService struct")
	}
	if !strings.Contains(content, "func NewPaymentService") {
		t.Error("generated service missing constructor")
	}

	// -- Second call: should error --
	err = GenerateService("payment")
	if err == nil {
		t.Error("expected error on duplicate GenerateService, got nil")
	}
}

// ---------------------------------------------------------------------------
// GenerateProject
// ---------------------------------------------------------------------------

func TestGenerateProject(t *testing.T) {
	dir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current dir: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Fatalf("failed to restore dir: %v", err)
		}
	}()

	projectName := "testproject"
	err = GenerateProject(projectName)
	if err != nil {
		t.Fatalf("unexpected error from GenerateProject: %v", err)
	}

	// Expected directories (relative to project root)
	expectedDirs := []string{
		"cmd/server",
		"internal/handlers",
		"internal/models",
		"internal/services",
		"config",
		"migrations",
	}
	for _, d := range expectedDirs {
		fullPath := filepath.Join(projectName, d)
		info, err := os.Stat(fullPath)
		if os.IsNotExist(err) {
			t.Errorf("expected directory %s to exist", fullPath)
		} else if err != nil {
			t.Errorf("error stating %s: %v", fullPath, err)
		} else if !info.IsDir() {
			t.Errorf("expected %s to be a directory", fullPath)
		}
	}

	// Expected files
	expectedFiles := map[string][]string{
		"go.mod":                      {"module github.com/yourname/testproject", "go 1.21"},
		"cmd/server/main.go":          {"package main", "github.com/madcok-co/unicorn/core/pkg/app"},
		"config/config.go":            {"package config", "func Load()"},
		"internal/handlers/health.go": {"package handlers", "func HealthCheck", "func RegisterAll"},
		".env.example":                {"APP_NAME", "HTTP_PORT", "DB_HOST"},
		".gitignore":                  {"*.exe", ".env", "logs/"},
	}

	for relPath, patterns := range expectedFiles {
		fullPath := filepath.Join(projectName, relPath)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			t.Errorf("expected file %s to exist: %v", fullPath, err)
			continue
		}
		content := string(data)
		if len(content) == 0 {
			t.Errorf("file %s is empty", fullPath)
		}
		for _, pat := range patterns {
			if !strings.Contains(content, pat) {
				t.Errorf("file %s: expected to contain %q", fullPath, pat)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Individual file generators (unexported)
// ---------------------------------------------------------------------------

func TestGenerateGoMod(t *testing.T) {
	content := generateGoMod("github.com/testuser/myapp")

	if !strings.Contains(content, "module github.com/testuser/myapp") {
		t.Error("expected module declaration with correct name")
	}
	if !strings.Contains(content, "go 1.21") {
		t.Error("expected go version 1.21")
	}
	if !strings.Contains(content, "github.com/madcok-co/unicorn") {
		t.Error("expected unicorn dependency")
	}
}

func TestGenerateMainGo(t *testing.T) {
	content := generateMainGo("github.com/testuser/myapp")

	if !strings.Contains(content, "package main") {
		t.Error("expected package main")
	}
	if !strings.Contains(content, "\"github.com/testuser/myapp/config\"") {
		t.Error("expected config import with correct module path")
	}
	if !strings.Contains(content, "\"github.com/testuser/myapp/internal/handlers\"") {
		t.Error("expected handlers import with correct module path")
	}
	if !strings.Contains(content, "func main()") {
		t.Error("expected main function")
	}
	if !strings.Contains(content, "application := app.New") {
		t.Error("expected app.New call")
	}
	if !strings.Contains(content, "handlers.RegisterAll(application)") {
		t.Error("expected RegisterAll call")
	}
}

func TestGenerateConfigGo(t *testing.T) {
	content := generateConfigGo("irrelevant") // moduleName is unused in this function

	if len(content) == 0 {
		t.Fatal("generateConfigGo returned empty string")
	}
	if !strings.Contains(content, "package config") {
		t.Error("expected package config")
	}
	if !strings.Contains(content, "type Config struct") {
		t.Error("expected Config struct")
	}
	if !strings.Contains(content, "func Load()") {
		t.Error("expected Load function")
	}
	if !strings.Contains(content, "func getEnv(") {
		t.Error("expected getEnv helper")
	}
	if !strings.Contains(content, "func getEnvInt(") {
		t.Error("expected getEnvInt helper")
	}
	// Spot-check some fields
	if !strings.Contains(content, "HTTPHost string") {
		t.Error("expected HTTPHost field")
	}
	if !strings.Contains(content, "DBHost") {
		t.Error("expected DBHost field")
	}
}

func TestGenerateHealthHandler(t *testing.T) {
	content := generateHealthHandler("github.com/testuser/myapp")

	if len(content) == 0 {
		t.Fatal("generateHealthHandler returned empty string")
	}
	if !strings.Contains(content, "package handlers") {
		t.Error("expected package handlers")
	}
	if !strings.Contains(content, "func HealthCheck") {
		t.Error("expected HealthCheck function")
	}
	if !strings.Contains(content, "func RegisterAll") {
		t.Error("expected RegisterAll function")
	}
	if !strings.Contains(content, "HealthResponse") {
		t.Error("expected HealthResponse struct")
	}
	if !strings.Contains(content, "\"healthy\"") {
		t.Error("expected healthy status")
	}
}

func TestGenerateEnvExample(t *testing.T) {
	content := generateEnvExample()

	if len(content) == 0 {
		t.Fatal("generateEnvExample returned empty string")
	}
	if !strings.Contains(content, "APP_NAME=") {
		t.Error("expected APP_NAME")
	}
	if !strings.Contains(content, "HTTP_HOST=") {
		t.Error("expected HTTP_HOST")
	}
	if !strings.Contains(content, "DB_HOST=") {
		t.Error("expected DB_HOST")
	}
	if !strings.Contains(content, "REDIS_HOST=") {
		t.Error("expected REDIS_HOST")
	}
	if !strings.Contains(content, "KAFKA_BROKERS=") {
		t.Error("expected KAFKA_BROKERS")
	}
}

func TestGenerateGitignore(t *testing.T) {
	content := generateGitignore()

	if len(content) == 0 {
		t.Fatal("generateGitignore returned empty string")
	}
	if !strings.Contains(content, "*.exe") {
		t.Error("expected *.exe pattern")
	}
	if !strings.Contains(content, ".env") {
		t.Error("expected .env pattern")
	}
	if !strings.Contains(content, ".idea/") {
		t.Error("expected .idea/ pattern")
	}
	if !strings.Contains(content, "logs/") {
		t.Error("expected logs/ pattern")
	}
	if !strings.Contains(content, "*.log") {
		t.Error("expected *.log pattern")
	}
}
