package openapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/madcok-co/unicorn/core/pkg/context"
	"github.com/madcok-co/unicorn/core/pkg/handler"
)

// OpenAPI represents OpenAPI 3.0 specification
type OpenAPI struct {
	OpenAPI    string                `json:"openapi"`
	Info       Info                  `json:"info"`
	Servers    []Server              `json:"servers,omitempty"`
	Paths      map[string]PathItem   `json:"paths"`
	Components *Components           `json:"components,omitempty"`
	Security   []map[string][]string `json:"security,omitempty"`
	Tags       []Tag                 `json:"tags,omitempty"`
}

// Info provides metadata about the API
type Info struct {
	Title          string  `json:"title"`
	Description    string  `json:"description,omitempty"`
	TermsOfService string  `json:"termsOfService,omitempty"`
	Contact        Contact `json:"contact,omitempty"`
	License        License `json:"license,omitempty"`
	Version        string  `json:"version"`
}

// Contact information
type Contact struct {
	Name  string `json:"name,omitempty"`
	URL   string `json:"url,omitempty"`
	Email string `json:"email,omitempty"`
}

// License information
type License struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

// Server represents a server
type Server struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

// PathItem describes operations available on a single path
type PathItem struct {
	Summary     string     `json:"summary,omitempty"`
	Description string     `json:"description,omitempty"`
	Get         *Operation `json:"get,omitempty"`
	Post        *Operation `json:"post,omitempty"`
	Put         *Operation `json:"put,omitempty"`
	Delete      *Operation `json:"delete,omitempty"`
	Patch       *Operation `json:"patch,omitempty"`
	Options     *Operation `json:"options,omitempty"`
	Head        *Operation `json:"head,omitempty"`
}

// Operation describes a single API operation on a path
type Operation struct {
	Tags        []string              `json:"tags,omitempty"`
	Summary     string                `json:"summary,omitempty"`
	Description string                `json:"description,omitempty"`
	OperationID string                `json:"operationId,omitempty"`
	Parameters  []Parameter           `json:"parameters,omitempty"`
	RequestBody *RequestBody          `json:"requestBody,omitempty"`
	Responses   map[string]Response   `json:"responses"`
	Security    []map[string][]string `json:"security,omitempty"`
	Deprecated  bool                  `json:"deprecated,omitempty"`
}

// Parameter describes a single operation parameter
type Parameter struct {
	Name        string  `json:"name"`
	In          string  `json:"in"` // query, header, path, cookie
	Description string  `json:"description,omitempty"`
	Required    bool    `json:"required,omitempty"`
	Schema      *Schema `json:"schema,omitempty"`
	Example     any     `json:"example,omitempty"`
}

// RequestBody describes a single request body
type RequestBody struct {
	Description string               `json:"description,omitempty"`
	Content     map[string]MediaType `json:"content"`
	Required    bool                 `json:"required,omitempty"`
}

// MediaType provides schema and examples for the media type
type MediaType struct {
	Schema  *Schema `json:"schema,omitempty"`
	Example any     `json:"example,omitempty"`
}

// Response describes a single response
type Response struct {
	Description string               `json:"description"`
	Content     map[string]MediaType `json:"content,omitempty"`
}

// Schema defines input and output data types
type Schema struct {
	Type                 string             `json:"type,omitempty"`
	Format               string             `json:"format,omitempty"`
	Description          string             `json:"description,omitempty"`
	Properties           map[string]*Schema `json:"properties,omitempty"`
	Items                *Schema            `json:"items,omitempty"`
	Required             []string           `json:"required,omitempty"`
	Enum                 []any              `json:"enum,omitempty"`
	Example              any                `json:"example,omitempty"`
	Ref                  string             `json:"$ref,omitempty"`
	AdditionalProperties bool               `json:"additionalProperties,omitempty"`
}

// Components holds reusable objects
type Components struct {
	Schemas         map[string]*Schema        `json:"schemas,omitempty"`
	SecuritySchemes map[string]SecurityScheme `json:"securitySchemes,omitempty"`
}

// SecurityScheme defines a security scheme
type SecurityScheme struct {
	Type         string `json:"type"` // apiKey, http, oauth2, openIdConnect
	Description  string `json:"description,omitempty"`
	Name         string `json:"name,omitempty"`
	In           string `json:"in,omitempty"`     // query, header, cookie
	Scheme       string `json:"scheme,omitempty"` // bearer, basic
	BearerFormat string `json:"bearerFormat,omitempty"`
}

// Tag for API documentation
type Tag struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// Config for OpenAPI generation
type Config struct {
	Title           string
	Description     string
	Version         string
	Servers         []Server
	Contact         Contact
	License         License
	SecuritySchemes map[string]SecurityScheme
	Tags            []Tag
}

// DefaultConfig returns default OpenAPI config
func DefaultConfig() *Config {
	return &Config{
		Title:       "API Documentation",
		Description: "Auto-generated API documentation",
		Version:     "1.0.0",
	}
}

// Generator generates OpenAPI specification from handlers
type Generator struct {
	config   *Config
	registry *handler.Registry
	openapi  *OpenAPI
}

// NewGenerator creates a new OpenAPI generator
func NewGenerator(config *Config, registry *handler.Registry) *Generator {
	if config == nil {
		config = DefaultConfig()
	}

	return &Generator{
		config:   config,
		registry: registry,
		openapi: &OpenAPI{
			OpenAPI: "3.0.0",
			Info: Info{
				Title:       config.Title,
				Description: config.Description,
				Version:     config.Version,
				Contact:     config.Contact,
				License:     config.License,
			},
			Servers: config.Servers,
			Paths:   make(map[string]PathItem),
			Components: &Components{
				Schemas:         make(map[string]*Schema),
				SecuritySchemes: config.SecuritySchemes,
			},
			Tags: config.Tags,
		},
	}
}

// Generate generates the OpenAPI specification
func (g *Generator) Generate() (*OpenAPI, error) {
	handlers := g.registry.All()

	for _, h := range handlers {
		if err := g.processHandler(h); err != nil {
			return nil, fmt.Errorf("failed to process handler %s: %w", h.Name, err)
		}
	}

	return g.openapi, nil
}

// processHandler processes a single handler
func (g *Generator) processHandler(h *handler.Handler) error {
	// Process HTTP triggers
	for _, trigger := range h.Triggers() {
		if trigger.Type() == handler.TriggerHTTP {
			httpTrigger, ok := trigger.(*handler.HTTPTrigger)
			if !ok {
				continue
			}

			path := httpTrigger.Path
			method := strings.ToLower(httpTrigger.Method)

			// Get or create path item
			pathItem, exists := g.openapi.Paths[path]
			if !exists {
				pathItem = PathItem{}
			}

			// Create operation
			operation := g.createOperation(h, httpTrigger)

			// Set operation for method
			switch method {
			case "get":
				pathItem.Get = operation
			case "post":
				pathItem.Post = operation
			case "put":
				pathItem.Put = operation
			case "delete":
				pathItem.Delete = operation
			case "patch":
				pathItem.Patch = operation
			case "options":
				pathItem.Options = operation
			case "head":
				pathItem.Head = operation
			}

			g.openapi.Paths[path] = pathItem
		}
	}

	return nil
}

// createOperation creates an operation from handler
func (g *Generator) createOperation(h *handler.Handler, httpTrigger *handler.HTTPTrigger) *Operation {
	operation := &Operation{
		Summary:     h.Name,
		Description: h.Description,
		OperationID: g.generateOperationID(h.Name, httpTrigger.Method, httpTrigger.Path),
		Responses:   make(map[string]Response),
	}

	// Extract parameters from path
	params := extractPathParams(httpTrigger.Path)
	for _, param := range params {
		operation.Parameters = append(operation.Parameters, Parameter{
			Name:     param,
			In:       "path",
			Required: true,
			Schema: &Schema{
				Type: "string",
			},
		})
	}

	// Add request body for POST, PUT, PATCH
	if httpTrigger.Method == "POST" || httpTrigger.Method == "PUT" || httpTrigger.Method == "PATCH" {
		if h.RequestType() != nil {
			schema := g.generateSchema(h.RequestType())
			operation.RequestBody = &RequestBody{
				Required: true,
				Content: map[string]MediaType{
					"application/json": {
						Schema: schema,
					},
				},
			}
		}
	}

	// Add response
	if h.ResponseType() != nil {
		schema := g.generateSchema(h.ResponseType())
		operation.Responses["200"] = Response{
			Description: "Successful response",
			Content: map[string]MediaType{
				"application/json": {
					Schema: schema,
				},
			},
		}
	} else {
		operation.Responses["200"] = Response{
			Description: "Successful response",
		}
	}

	// Add error responses
	operation.Responses["400"] = Response{Description: "Bad Request"}
	operation.Responses["401"] = Response{Description: "Unauthorized"}
	operation.Responses["403"] = Response{Description: "Forbidden"}
	operation.Responses["404"] = Response{Description: "Not Found"}
	operation.Responses["500"] = Response{Description: "Internal Server Error"}

	return operation
}

// generateSchema generates schema from Go type
func (g *Generator) generateSchema(t reflect.Type) *Schema {
	if t == nil {
		return &Schema{Type: "object"}
	}

	// Handle pointer types
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	schema := &Schema{}

	switch t.Kind() {
	case reflect.String:
		schema.Type = "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		schema.Type = "integer"
		if t.Kind() == reflect.Int64 {
			schema.Format = "int64"
		} else {
			schema.Format = "int32"
		}
	case reflect.Float32, reflect.Float64:
		schema.Type = "number"
		if t.Kind() == reflect.Float64 {
			schema.Format = "double"
		} else {
			schema.Format = "float"
		}
	case reflect.Bool:
		schema.Type = "boolean"
	case reflect.Slice, reflect.Array:
		schema.Type = "array"
		schema.Items = g.generateSchema(t.Elem())
	case reflect.Map:
		schema.Type = "object"
		schema.AdditionalProperties = true
	case reflect.Struct:
		schema.Type = "object"
		schema.Properties = make(map[string]*Schema)
		required := []string{}

		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)

			// Skip unexported fields
			if !field.IsExported() {
				continue
			}

			jsonTag := field.Tag.Get("json")
			if jsonTag == "-" {
				continue
			}

			fieldName := field.Name
			if jsonTag != "" {
				parts := strings.Split(jsonTag, ",")
				if parts[0] != "" {
					fieldName = parts[0]
				}
				// Check if field is required (no omitempty)
				if !contains(parts, "omitempty") {
					required = append(required, fieldName)
				}
			}

			fieldSchema := g.generateSchema(field.Type)

			// Add description from tag
			if desc := field.Tag.Get("description"); desc != "" {
				fieldSchema.Description = desc
			}

			schema.Properties[fieldName] = fieldSchema
		}

		if len(required) > 0 {
			schema.Required = required
		}
	default:
		schema.Type = "object"
	}

	return schema
}

// generateOperationID generates a unique operation ID
func (g *Generator) generateOperationID(name, method, path string) string {
	// Clean path
	cleanPath := strings.ReplaceAll(path, "/", "_")
	cleanPath = strings.ReplaceAll(cleanPath, "{", "")
	cleanPath = strings.ReplaceAll(cleanPath, "}", "")
	cleanPath = strings.Trim(cleanPath, "_")

	return fmt.Sprintf("%s_%s_%s", method, cleanPath, name)
}

// extractPathParams extracts path parameters from path
func extractPathParams(path string) []string {
	var params []string
	parts := strings.Split(path, "/")
	for _, part := range parts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			param := strings.Trim(part, "{}")
			params = append(params, param)
		} else if strings.HasPrefix(part, ":") {
			param := strings.TrimPrefix(part, ":")
			params = append(params, param)
		}
	}
	return params
}

// ToJSON converts OpenAPI spec to JSON
func (g *Generator) ToJSON() ([]byte, error) {
	return json.MarshalIndent(g.openapi, "", "  ")
}

// SwaggerUIHandler returns a handler that serves Swagger UI
func SwaggerUIHandler(spec *OpenAPI) context.HandlerFunc {
	return func(ctx *context.Context) error {
		specJSON, err := json.Marshal(spec)
		if err != nil {
			return ctx.Error(http.StatusInternalServerError, "Failed to marshal spec")
		}

		html := fmt.Sprintf(`
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>API Documentation</title>
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
    <script>
        window.onload = function() {
            const spec = %s;
            SwaggerUIBundle({
                spec: spec,
                dom_id: '#swagger-ui',
                deepLinking: true,
                presets: [
                    SwaggerUIBundle.presets.apis,
                    SwaggerUIBundle.SwaggerUIStandalonePreset
                ],
            });
        };
    </script>
</body>
</html>`, string(specJSON))

		ctx.Response().StatusCode = http.StatusOK
		ctx.Response().Body = []byte(html)
		ctx.Response().SetHeader("Content-Type", "text/html; charset=utf-8")
		return nil
	}
}

// SpecJSONHandler returns a handler that serves OpenAPI spec as JSON
func SpecJSONHandler(spec *OpenAPI) context.HandlerFunc {
	return func(ctx *context.Context) error {
		return ctx.JSON(http.StatusOK, spec)
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
