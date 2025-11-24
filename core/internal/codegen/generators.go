package codegen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// GenerateHandler generates a new handler file
func GenerateHandler(name string) error {
	// Ensure handlers directory exists
	dir := "internal/handlers"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	filename := filepath.Join(dir, strings.ToLower(name)+".go")

	// Check if file exists
	if _, err := os.Stat(filename); err == nil {
		return fmt.Errorf("handler file already exists: %s", filename)
	}

	content := generateHandlerContent(name)
	if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("Created: %s\n", filename)
	return nil
}

// GenerateModel generates a new model file
func GenerateModel(name string) error {
	dir := "internal/models"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	filename := filepath.Join(dir, strings.ToLower(name)+".go")

	if _, err := os.Stat(filename); err == nil {
		return fmt.Errorf("model file already exists: %s", filename)
	}

	content := generateModelContent(name)
	if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("Created: %s\n", filename)
	return nil
}

// GenerateService generates a new service file
func GenerateService(name string) error {
	dir := "internal/services"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	filename := filepath.Join(dir, strings.ToLower(name)+".go")

	if _, err := os.Stat(filename); err == nil {
		return fmt.Errorf("service file already exists: %s", filename)
	}

	content := generateServiceContent(name)
	if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("Created: %s\n", filename)
	return nil
}

func generateHandlerContent(name string) string {
	titleName := toPascalCase(name)
	lowerName := strings.ToLower(name)

	return fmt.Sprintf(`package handlers

import (
	"github.com/madcok-co/unicorn/core/pkg/context"
)

// ============ Request/Response DTOs ============

// Create%sRequest for creating %s
type Create%sRequest struct {
	Name string `+"`json:\"name\" validate:\"required\"`"+`
	// Add more fields as needed
}

// Update%sRequest for updating %s
type Update%sRequest struct {
	Name string `+"`json:\"name\"`"+`
	// Add more fields as needed
}

// %sResponse for %s responses
type %sResponse struct {
	ID   string `+"`json:\"id\"`"+`
	Name string `+"`json:\"name\"`"+`
	// Add more fields as needed
}

// %sListResponse for list responses
type %sListResponse struct {
	Data  []*%sResponse `+"`json:\"data\"`"+`
	Total int64              `+"`json:\"total\"`"+`
}

// ============ Handlers ============

// Create%s creates a new %s
func Create%s(ctx *context.Context, req Create%sRequest) (*%sResponse, error) {
	// Get infrastructure from context
	db := ctx.DB()
	log := ctx.Logger()

	log.Info("creating %s", "name", req.Name)

	// TODO: Implement business logic
	// Example:
	// %s := &models.%s{Name: req.Name}
	// if err := db.Create(%s); err != nil {
	//     return nil, err
	// }

	_ = db // Remove this line when implementing

	return &%sResponse{
		ID:   "generated-id",
		Name: req.Name,
	}, nil
}

// Get%s gets a %s by ID
func Get%s(ctx *context.Context) (*%sResponse, error) {
	id := ctx.Request().Params["id"]

	db := ctx.DB()
	log := ctx.Logger()

	log.Info("getting %s", "id", id)

	// TODO: Implement business logic
	// Example:
	// var %s models.%s
	// if err := db.FindByID(ctx.Context(), id, &%s); err != nil {
	//     return nil, err
	// }

	_ = db // Remove this line when implementing

	return &%sResponse{
		ID:   id,
		Name: "example",
	}, nil
}

// List%s lists all %s
func List%s(ctx *context.Context) (*%sListResponse, error) {
	db := ctx.DB()
	log := ctx.Logger()

	log.Info("listing %s")

	// TODO: Implement business logic
	_ = db // Remove this line when implementing

	return &%sListResponse{
		Data:  []*%sResponse{},
		Total: 0,
	}, nil
}

// Update%s updates a %s
func Update%s(ctx *context.Context, req Update%sRequest) (*%sResponse, error) {
	id := ctx.Request().Params["id"]

	db := ctx.DB()
	log := ctx.Logger()

	log.Info("updating %s", "id", id)

	// TODO: Implement business logic
	_ = db // Remove this line when implementing

	return &%sResponse{
		ID:   id,
		Name: req.Name,
	}, nil
}

// Delete%s deletes a %s
func Delete%s(ctx *context.Context) error {
	id := ctx.Request().Params["id"]

	db := ctx.DB()
	log := ctx.Logger()

	log.Info("deleting %s", "id", id)

	// TODO: Implement business logic
	_ = db // Remove this line when implementing

	return ctx.NoContent()
}

// ============ Registration Helper ============

// Register%sHandlers registers all %s handlers to the app
// Call this from your main handlers.RegisterAll function
/*
func Register%sHandlers(application *app.App) {
	application.RegisterHandler(Create%s).
		Named("create-%s").
		HTTP("POST", "/%ss").
		Done()

	application.RegisterHandler(Get%s).
		Named("get-%s").
		HTTP("GET", "/%ss/:id").
		Done()

	application.RegisterHandler(List%s).
		Named("list-%s").
		HTTP("GET", "/%ss").
		Done()

	application.RegisterHandler(Update%s).
		Named("update-%s").
		HTTP("PUT", "/%ss/:id").
		Done()

	application.RegisterHandler(Delete%s).
		Named("delete-%s").
		HTTP("DELETE", "/%ss/:id").
		Done()
}
*/
`,
		// Create request
		titleName, lowerName, titleName,
		// Update request
		titleName, lowerName, titleName,
		// Response
		titleName, lowerName, titleName,
		// List response
		titleName, titleName, titleName,
		// Create handler
		titleName, lowerName, titleName, titleName, titleName,
		lowerName, lowerName, titleName, lowerName,
		titleName,
		// Get handler
		titleName, lowerName, titleName, titleName,
		lowerName, lowerName, titleName, lowerName,
		titleName,
		// List handler
		titleName, lowerName, titleName, titleName,
		lowerName,
		titleName, titleName,
		// Update handler
		titleName, lowerName, titleName, titleName, titleName,
		lowerName,
		titleName,
		// Delete handler
		titleName, lowerName, titleName,
		lowerName,
		// Registration
		titleName, lowerName,
		titleName, titleName, lowerName, lowerName,
		titleName, lowerName, lowerName,
		titleName, lowerName, lowerName,
		titleName, lowerName, lowerName,
		titleName, lowerName, lowerName,
	)
}

func generateModelContent(name string) string {
	titleName := toPascalCase(name)

	return fmt.Sprintf(`package models

import (
	"time"
)

// %s represents a %s entity
type %s struct {
	ID        string    `+"`json:\"id\" db:\"id\"`"+`
	Name      string    `+"`json:\"name\" db:\"name\"`"+`
	CreatedAt time.Time `+"`json:\"created_at\" db:\"created_at\"`"+`
	UpdatedAt time.Time `+"`json:\"updated_at\" db:\"updated_at\"`"+`

	// Add more fields as needed
}

// TableName returns the database table name
func (m *%s) TableName() string {
	return "%ss"
}

// BeforeCreate hook - called before inserting
func (m *%s) BeforeCreate() error {
	now := time.Now()
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	if m.UpdatedAt.IsZero() {
		m.UpdatedAt = now
	}
	return nil
}

// BeforeUpdate hook - called before updating
func (m *%s) BeforeUpdate() error {
	m.UpdatedAt = time.Now()
	return nil
}
`, titleName, strings.ToLower(name), titleName, titleName, strings.ToLower(name), titleName, titleName)
}

func generateServiceContent(name string) string {
	titleName := toPascalCase(name)
	lowerName := strings.ToLower(name)

	return fmt.Sprintf(`package services

import (
	"context"

	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

// %sService handles %s business logic
type %sService struct {
	db    contracts.Database
	cache contracts.Cache
	log   contracts.Logger
}

// New%sService creates a new %s service
func New%sService(db contracts.Database, cache contracts.Cache, log contracts.Logger) *%sService {
	return &%sService{
		db:    db,
		cache: cache,
		log:   log,
	}
}

// Create creates a new %s
func (s *%sService) Create(ctx context.Context, data any) error {
	s.log.WithContext(ctx).Info("creating %s")

	// TODO: Implement business logic
	// Example:
	// return s.db.Create(ctx, data)

	return nil
}

// GetByID gets a %s by ID
func (s *%sService) GetByID(ctx context.Context, id string) (any, error) {
	s.log.WithContext(ctx).Info("getting %s", "id", id)

	// TODO: Implement with caching
	// Example:
	// cacheKey := fmt.Sprintf("%s:%%s", id)
	// var result YourModel
	//
	// err := s.cache.Remember(ctx, cacheKey, time.Hour, func() (any, error) {
	//     var model YourModel
	//     if err := s.db.FindByID(ctx, id, &model); err != nil {
	//         return nil, err
	//     }
	//     return &model, nil
	// }, &result)

	return nil, nil
}

// Update updates a %s
func (s *%sService) Update(ctx context.Context, id string, data any) error {
	s.log.WithContext(ctx).Info("updating %s", "id", id)

	// TODO: Implement business logic
	// Don't forget to invalidate cache
	// s.cache.Delete(ctx, fmt.Sprintf("%s:%%s", id))

	return nil
}

// Delete deletes a %s
func (s *%sService) Delete(ctx context.Context, id string) error {
	s.log.WithContext(ctx).Info("deleting %s", "id", id)

	// TODO: Implement business logic
	// Don't forget to invalidate cache
	// s.cache.Delete(ctx, fmt.Sprintf("%s:%%s", id))

	return nil
}

// List lists all %s with pagination
func (s *%sService) List(ctx context.Context, page, limit int) ([]any, int64, error) {
	s.log.WithContext(ctx).Info("listing %s", "page", page, "limit", limit)

	// TODO: Implement business logic
	// Example:
	// var results []YourModel
	// err := s.db.Query().
	//     From("your_table").
	//     Limit(limit).
	//     Offset((page-1) * limit).
	//     Get(ctx, &results)

	return nil, 0, nil
}
`,
		titleName, lowerName, titleName,
		titleName, lowerName, titleName, titleName, titleName,
		lowerName, titleName, lowerName,
		lowerName, titleName, lowerName, lowerName,
		lowerName, titleName, lowerName, lowerName,
		lowerName, titleName, lowerName, lowerName,
		lowerName, titleName, lowerName,
	)
}

// toPascalCase converts string to PascalCase
func toPascalCase(s string) string {
	if len(s) == 0 {
		return s
	}

	// Handle snake_case and kebab-case
	s = strings.ReplaceAll(s, "-", "_")
	parts := strings.Split(s, "_")

	var result strings.Builder
	for _, part := range parts {
		if len(part) > 0 {
			runes := []rune(part)
			runes[0] = unicode.ToUpper(runes[0])
			result.WriteString(string(runes))
		}
	}

	return result.String()
}
