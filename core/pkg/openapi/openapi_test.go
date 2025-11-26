package openapi

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/madcok-co/unicorn/core/pkg/context"
	"github.com/madcok-co/unicorn/core/pkg/handler"
)

type TestRequest struct {
	Name  string `json:"name"`
	Email string `json:"email" description:"User email address"`
	Age   int    `json:"age,omitempty"`
}

type TestResponse struct {
	ID      int64  `json:"id"`
	Message string `json:"message"`
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Title != "API Documentation" {
		t.Errorf("Expected title 'API Documentation', got %s", config.Title)
	}

	if config.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got %s", config.Version)
	}
}

func TestNewGenerator(t *testing.T) {
	config := DefaultConfig()
	registry := handler.NewRegistry()

	gen := NewGenerator(config, registry)

	if gen == nil {
		t.Fatal("Generator should not be nil")
	}

	if gen.openapi.OpenAPI != "3.0.0" {
		t.Errorf("Expected OpenAPI version 3.0.0, got %s", gen.openapi.OpenAPI)
	}

	if gen.openapi.Info.Title != config.Title {
		t.Errorf("Expected title %s, got %s", config.Title, gen.openapi.Info.Title)
	}
}

func TestGenerateSchema(t *testing.T) {
	gen := NewGenerator(DefaultConfig(), handler.NewRegistry())

	tests := []struct {
		name     string
		typeVal  reflect.Type
		expected string // Expected type
	}{
		{"string", reflect.TypeOf(""), "string"},
		{"int", reflect.TypeOf(0), "integer"},
		{"int64", reflect.TypeOf(int64(0)), "integer"},
		{"float64", reflect.TypeOf(float64(0)), "number"},
		{"bool", reflect.TypeOf(true), "boolean"},
		{"slice", reflect.TypeOf([]string{}), "array"},
		{"map", reflect.TypeOf(map[string]string{}), "object"},
		{"struct", reflect.TypeOf(TestRequest{}), "object"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := gen.generateSchema(tt.typeVal)
			if schema.Type != tt.expected {
				t.Errorf("Expected type %s, got %s", tt.expected, schema.Type)
			}
		})
	}
}

func TestGenerateSchemaStruct(t *testing.T) {
	gen := NewGenerator(DefaultConfig(), handler.NewRegistry())

	schema := gen.generateSchema(reflect.TypeOf(TestRequest{}))

	if schema.Type != "object" {
		t.Errorf("Expected type object, got %s", schema.Type)
	}

	if schema.Properties == nil {
		t.Fatal("Properties should not be nil")
	}

	// Check name field
	if schema.Properties["name"] == nil {
		t.Error("Expected 'name' property")
	}

	if schema.Properties["name"].Type != "string" {
		t.Errorf("Expected name type string, got %s", schema.Properties["name"].Type)
	}

	// Check email field with description
	if schema.Properties["email"] == nil {
		t.Error("Expected 'email' property")
	}

	if schema.Properties["email"].Description != "User email address" {
		t.Errorf("Expected description 'User email address', got %s",
			schema.Properties["email"].Description)
	}

	// Check required fields (name and email, not age because of omitempty)
	if len(schema.Required) < 2 {
		t.Errorf("Expected at least 2 required fields, got %d", len(schema.Required))
	}

	hasName := false
	hasEmail := false
	for _, field := range schema.Required {
		if field == "name" {
			hasName = true
		}
		if field == "email" {
			hasEmail = true
		}
	}

	if !hasName {
		t.Error("Expected 'name' to be required")
	}

	if !hasEmail {
		t.Error("Expected 'email' to be required")
	}
}

func TestGenerateOperationID(t *testing.T) {
	gen := NewGenerator(DefaultConfig(), handler.NewRegistry())

	tests := []struct {
		name     string
		method   string
		path     string
		expected string
	}{
		{"simple", "GET", "/users", "GET_users_simple"},
		{"with param", "POST", "/users/{id}", "POST_users_id_with param"},
		{"nested", "DELETE", "/api/v1/users/{id}", "DELETE_api_v1_users_id_nested"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := gen.generateOperationID(tt.name, tt.method, tt.path)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestExtractPathParams(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected []string
	}{
		{"no params", "/users", []string{}},
		{"single param curly", "/users/{id}", []string{"id"}},
		{"single param colon", "/users/:id", []string{"id"}},
		{"multiple params", "/users/{userId}/posts/{postId}", []string{"userId", "postId"}},
		{"mixed", "/api/:version/users/{id}", []string{"version", "id"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractPathParams(tt.path)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d params, got %d", len(tt.expected), len(result))
				return
			}

			for i, param := range result {
				if param != tt.expected[i] {
					t.Errorf("Expected param %s, got %s", tt.expected[i], param)
				}
			}
		})
	}
}

func TestGenerate(t *testing.T) {
	config := DefaultConfig()
	registry := handler.NewRegistry()

	// Register a test handler
	// Create a handler function with the proper signature
	handlerFunc := func(ctx *context.Context, req TestRequest) (TestResponse, error) {
		return TestResponse{}, nil
	}

	h := handler.New(handlerFunc).
		Named("GetUser").
		Describe("Get user by ID").
		HTTP("GET", "/users/{id}")

	registry.Register(h)

	gen := NewGenerator(config, registry)
	spec, err := gen.Generate()

	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if spec == nil {
		t.Fatal("Spec should not be nil")
	}

	// Check paths
	if len(spec.Paths) == 0 {
		t.Error("Expected at least one path")
	}

	// Check specific path
	pathItem, exists := spec.Paths["/users/{id}"]
	if !exists {
		t.Error("Expected path /users/{id}")
	}

	if pathItem.Get == nil {
		t.Error("Expected GET operation")
	}

	if pathItem.Get.Summary != "GetUser" {
		t.Errorf("Expected summary 'GetUser', got %s", pathItem.Get.Summary)
	}

	if pathItem.Get.Description != "Get user by ID" {
		t.Errorf("Expected description 'Get user by ID', got %s", pathItem.Get.Description)
	}

	// Check parameters
	if len(pathItem.Get.Parameters) == 0 {
		t.Error("Expected at least one parameter")
	}

	if pathItem.Get.Parameters[0].Name != "id" {
		t.Errorf("Expected parameter name 'id', got %s", pathItem.Get.Parameters[0].Name)
	}

	if pathItem.Get.Parameters[0].In != "path" {
		t.Errorf("Expected parameter in 'path', got %s", pathItem.Get.Parameters[0].In)
	}

	// Check responses
	if len(pathItem.Get.Responses) == 0 {
		t.Error("Expected at least one response")
	}

	if _, exists := pathItem.Get.Responses["200"]; !exists {
		t.Error("Expected 200 response")
	}
}

func TestGenerateWithPostMethod(t *testing.T) {
	config := DefaultConfig()
	registry := handler.NewRegistry()

	handlerFunc := func(ctx *context.Context, req TestRequest) (TestResponse, error) {
		return TestResponse{}, nil
	}

	h := handler.New(handlerFunc).
		Named("CreateUser").
		Describe("Create a new user").
		HTTP("POST", "/users")

	registry.Register(h)

	gen := NewGenerator(config, registry)
	spec, err := gen.Generate()

	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	pathItem := spec.Paths["/users"]
	if pathItem.Post == nil {
		t.Fatal("Expected POST operation")
	}

	// Check request body
	if pathItem.Post.RequestBody == nil {
		t.Error("Expected request body for POST")
	}

	if !pathItem.Post.RequestBody.Required {
		t.Error("Expected request body to be required")
	}

	if pathItem.Post.RequestBody.Content == nil {
		t.Fatal("Expected request body content")
	}

	if _, exists := pathItem.Post.RequestBody.Content["application/json"]; !exists {
		t.Error("Expected application/json content type")
	}
}

func TestToJSON(t *testing.T) {
	config := DefaultConfig()
	registry := handler.NewRegistry()

	gen := NewGenerator(config, registry)
	gen.Generate()

	jsonData, err := gen.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	if len(jsonData) == 0 {
		t.Error("JSON data should not be empty")
	}

	// Verify it's valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal(jsonData, &result); err != nil {
		t.Errorf("Generated JSON is invalid: %v", err)
	}

	// Check OpenAPI version
	if result["openapi"] != "3.0.0" {
		t.Errorf("Expected openapi version 3.0.0, got %v", result["openapi"])
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		item     string
		expected bool
	}{
		{"found", []string{"a", "b", "c"}, "b", true},
		{"not found", []string{"a", "b", "c"}, "d", false},
		{"empty slice", []string{}, "a", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.slice, tt.item)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestSecurityScheme(t *testing.T) {
	scheme := SecurityScheme{
		Type:         "http",
		Scheme:       "bearer",
		BearerFormat: "JWT",
		Description:  "JWT Bearer token",
	}

	if scheme.Type != "http" {
		t.Errorf("Expected type http, got %s", scheme.Type)
	}

	if scheme.Scheme != "bearer" {
		t.Errorf("Expected scheme bearer, got %s", scheme.Scheme)
	}

	if scheme.BearerFormat != "JWT" {
		t.Errorf("Expected bearer format JWT, got %s", scheme.BearerFormat)
	}
}

func TestConfigWithServers(t *testing.T) {
	config := &Config{
		Title:       "My API",
		Description: "Test API",
		Version:     "2.0.0",
		Servers: []Server{
			{URL: "https://api.example.com", Description: "Production"},
			{URL: "https://staging.example.com", Description: "Staging"},
		},
	}

	registry := handler.NewRegistry()
	gen := NewGenerator(config, registry)

	if len(gen.openapi.Servers) != 2 {
		t.Errorf("Expected 2 servers, got %d", len(gen.openapi.Servers))
	}

	if gen.openapi.Servers[0].URL != "https://api.example.com" {
		t.Errorf("Expected first server URL https://api.example.com, got %s",
			gen.openapi.Servers[0].URL)
	}
}

func TestGenerateSchemaPointer(t *testing.T) {
	gen := NewGenerator(DefaultConfig(), handler.NewRegistry())

	// Test pointer type
	schema := gen.generateSchema(reflect.TypeOf(&TestRequest{}))

	if schema.Type != "object" {
		t.Errorf("Expected type object for pointer, got %s", schema.Type)
	}

	if schema.Properties == nil {
		t.Error("Expected properties for pointer struct")
	}
}

func TestGenerateSchemaArray(t *testing.T) {
	gen := NewGenerator(DefaultConfig(), handler.NewRegistry())

	schema := gen.generateSchema(reflect.TypeOf([]TestRequest{}))

	if schema.Type != "array" {
		t.Errorf("Expected type array, got %s", schema.Type)
	}

	if schema.Items == nil {
		t.Fatal("Expected items for array")
	}

	if schema.Items.Type != "object" {
		t.Errorf("Expected items type object, got %s", schema.Items.Type)
	}
}
