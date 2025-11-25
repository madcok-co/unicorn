package main

import (
	"fmt"
	"log"
	"time"

	httpAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/http"
	"github.com/madcok-co/unicorn/core/pkg/app"
	"github.com/madcok-co/unicorn/core/pkg/context"

	// Import memory adapters for demo
	memoryBroker "github.com/madcok-co/unicorn/core/pkg/adapters/broker/memory"
	cacheAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/cache"
	loggerAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/logger"
)

// ============================================================
// EXAMPLE 1: Basic CRUD with Validation
// ============================================================

type CreateProductRequest struct {
	Name        string  `json:"name" validate:"required,min=3,max=100"`
	Description string  `json:"description" validate:"required,min=10"`
	Price       float64 `json:"price" validate:"required,gt=0"`
	Stock       int     `json:"stock" validate:"required,gte=0"`
}

type Product struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Price       float64   `json:"price"`
	Stock       int       `json:"stock"`
	CreatedAt   time.Time `json:"created_at"`
}

// CreateProduct - HTTP POST /products
// Demonstrates: Validation, Cache, Logger
func CreateProduct(ctx *context.Context, req CreateProductRequest) (*Product, error) {
	logger := ctx.Logger()
	cache := ctx.Cache()

	// Business logic
	product := &Product{
		ID:          fmt.Sprintf("prod-%d", time.Now().Unix()),
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		Stock:       req.Stock,
		CreatedAt:   time.Now(),
	}

	// Save to cache (in real app: save to DB)
	cacheKey := fmt.Sprintf("product:%s", product.ID)
	if err := cache.Set(ctx.Context(), cacheKey, product, 1*time.Hour); err != nil {
		logger.Error("failed to cache product", "error", err)
	}

	logger.Info("product created",
		"id", product.ID,
		"name", product.Name,
		"price", product.Price)

	return product, nil
}

// GetProduct - HTTP GET /products/:id
// Demonstrates: Path Parameters, Cache
func GetProduct(ctx *context.Context) (*Product, error) {
	productID := ctx.Request().Params["id"]
	logger := ctx.Logger()

	// Try to get from cache
	cacheKey := fmt.Sprintf("product:%s", productID)
	var product Product
	if ctx.Cache() != nil {
		if err := ctx.Cache().Get(ctx.Context(), cacheKey, &product); err == nil {
			logger.Info("product retrieved from cache", "id", productID)
			return &product, nil
		}
	}

	// If not in cache, return mock data (in real app: get from DB)
	product = Product{
		ID:          productID,
		Name:        "Sample Product",
		Description: "This is a sample product",
		Price:       99.99,
		Stock:       100,
		CreatedAt:   time.Now(),
	}

	// Cache it
	if ctx.Cache() != nil {
		ctx.Cache().Set(ctx.Context(), cacheKey, &product, 1*time.Hour)
	}

	return &product, nil
}

// ListProducts - HTTP GET /products
// Demonstrates: Query Parameters, Pagination
func ListProducts(ctx *context.Context) (map[string]interface{}, error) {
	// Get query parameters
	page := ctx.Request().Query["page"]
	if page == "" {
		page = "1"
	}
	limit := ctx.Request().Query["limit"]
	if limit == "" {
		limit = "10"
	}

	// Mock data
	products := []*Product{
		{
			ID:          "prod-1",
			Name:        "Product 1",
			Description: "Description for product 1",
			Price:       29.99,
			Stock:       50,
			CreatedAt:   time.Now().Add(-24 * time.Hour),
		},
		{
			ID:          "prod-2",
			Name:        "Product 2",
			Description: "Description for product 2",
			Price:       49.99,
			Stock:       30,
			CreatedAt:   time.Now().Add(-12 * time.Hour),
		},
	}

	ctx.Logger().Info("products listed", "page", page, "limit", limit)

	return map[string]interface{}{
		"data": products,
		"meta": map[string]interface{}{
			"page":  page,
			"limit": limit,
			"total": len(products),
		},
	}, nil
}

// UpdateProduct - HTTP PUT /products/:id
// Demonstrates: Path Params + Request Body
func UpdateProduct(ctx *context.Context, req CreateProductRequest) (*Product, error) {
	productID := ctx.Request().Params["id"]
	logger := ctx.Logger()

	product := &Product{
		ID:          productID,
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		Stock:       req.Stock,
		CreatedAt:   time.Now(),
	}

	logger.Info("product updated", "id", productID)

	return product, nil
}

// DeleteProduct - HTTP DELETE /products/:id
// Demonstrates: Delete operation with custom response
func DeleteProduct(ctx *context.Context) (map[string]interface{}, error) {
	productID := ctx.Request().Params["id"]
	logger := ctx.Logger()

	// Delete from cache
	cacheKey := fmt.Sprintf("product:%s", productID)
	if ctx.Cache() != nil {
		ctx.Cache().Delete(ctx.Context(), cacheKey)
	}

	logger.Info("product deleted", "id", productID)

	return map[string]interface{}{
		"message": "Product deleted successfully",
		"id":      productID,
	}, nil
}

// ============================================================
// EXAMPLE 2: Message Queue / Event Processing
// ============================================================

type ProductCreatedEvent struct {
	ProductID string    `json:"product_id"`
	Name      string    `json:"name"`
	Price     float64   `json:"price"`
	Timestamp time.Time `json:"timestamp"`
}

// ProcessProductCreatedEvent - Message Broker Handler
// Demonstrates: Same handler for HTTP and Message Queue
func ProcessProductCreatedEvent(ctx *context.Context, event ProductCreatedEvent) (map[string]interface{}, error) {
	logger := ctx.Logger()

	// Check trigger type
	triggerType := ctx.Request().TriggerType
	logger.Info("processing product created event",
		"trigger", triggerType,
		"product_id", event.ProductID,
		"name", event.Name)

	// Business logic - send email, update analytics, etc.
	// In real app, you might call other services here

	return map[string]interface{}{
		"status":       "processed",
		"product_id":   event.ProductID,
		"processed_at": time.Now(),
	}, nil
}

// ============================================================
// EXAMPLE 3: Scheduled Tasks (Cron)
// ============================================================

// CleanupExpiredProducts - Cron Job Handler
// Demonstrates: Cron-triggered handler
func CleanupExpiredProducts(ctx *context.Context) (map[string]interface{}, error) {
	logger := ctx.Logger()

	logger.Info("running cleanup task")

	// Cleanup logic here
	// In real app: delete expired products, update inventory, etc.

	cleaned := 5 // Mock number

	logger.Info("cleanup completed", "items_cleaned", cleaned)

	return map[string]interface{}{
		"status":    "success",
		"cleaned":   cleaned,
		"timestamp": time.Now(),
	}, nil
}

// ============================================================
// EXAMPLE 4: Advanced Features
// ============================================================

// SearchProducts - HTTP GET /products/search
// Demonstrates: Complex query parameters, headers
func SearchProducts(ctx *context.Context) (map[string]interface{}, error) {
	// Get query parameters
	query := ctx.Request().Query["q"]
	category := ctx.Request().Query["category"]
	minPrice := ctx.Request().Query["min_price"]
	maxPrice := ctx.Request().Query["max_price"]

	// Get headers
	userAgent := ctx.Request().Headers["User-Agent"]

	ctx.Logger().Info("searching products",
		"query", query,
		"category", category,
		"min_price", minPrice,
		"max_price", maxPrice,
		"user_agent", userAgent)

	// Mock search results
	results := []*Product{
		{
			ID:          "prod-search-1",
			Name:        fmt.Sprintf("Result for '%s'", query),
			Description: "Search result product",
			Price:       39.99,
			Stock:       20,
			CreatedAt:   time.Now(),
		},
	}

	return map[string]interface{}{
		"query":   query,
		"results": results,
		"count":   len(results),
	}, nil
}

// HealthCheck - HTTP GET /health
// Demonstrates: Health check endpoint
func HealthCheck(ctx *context.Context) (map[string]interface{}, error) {
	// Check if cache is working
	cache := ctx.Cache()
	cacheStatus := "ok"
	if err := cache.Set(ctx.Context(), "health:test", "ok", 10*time.Second); err != nil {
		cacheStatus = "error"
	}

	return map[string]interface{}{
		"status":    "healthy",
		"version":   "1.0.0",
		"timestamp": time.Now(),
		"services": map[string]string{
			"cache": cacheStatus,
		},
	}, nil
}

// GetMetrics - HTTP GET /metrics
// Demonstrates: Metrics endpoint
func GetMetrics(ctx *context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{
		"requests_total":    1234,
		"requests_success":  1200,
		"requests_failed":   34,
		"avg_response_time": "45ms",
		"uptime":            "2h 30m",
	}, nil
}

// ============================================================
// EXAMPLE 5: Error Handling
// ============================================================

type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code"`
	Details string `json:"details,omitempty"`
}

// SimulateError - HTTP GET /error
// Demonstrates: Error handling
func SimulateError(ctx *context.Context) (*Product, error) {
	errorType := ctx.Request().Query["type"]

	switch errorType {
	case "validation":
		return nil, fmt.Errorf("validation error: invalid product data")
	case "not_found":
		return nil, fmt.Errorf("product not found")
	case "server":
		return nil, fmt.Errorf("internal server error")
	default:
		return nil, fmt.Errorf("unknown error type")
	}
}

// ============================================================
// MAIN APPLICATION
// ============================================================

func main() {
	// Create application
	application := app.New(&app.Config{
		Name:       "unicorn-complete-example",
		Version:    "1.0.0",
		EnableHTTP: true,
		HTTP: &httpAdapter.Config{
			Host: "0.0.0.0",
			Port: 8080,
		},
		EnableBroker: true,  // Enable message broker
		EnableCron:   false, // Disable cron for this example
	})

	// Setup infrastructure adapters
	logger := loggerAdapter.NewConsoleLogger("info")
	application.SetLogger(logger)

	cache := cacheAdapter.New(cacheAdapter.NewMemoryDriver())
	application.SetCache(cache)

	// Note: Validator setup is optional for this demo
	// In production, use: contrib/validator/playground

	broker := memoryBroker.New()
	application.SetBroker(broker)

	// Register handlers

	// === CRUD Operations ===
	application.RegisterHandler(CreateProduct).
		Named("create-product").
		HTTP("POST", "/products").
		Done()

	application.RegisterHandler(GetProduct).
		Named("get-product").
		HTTP("GET", "/products/:id").
		Done()

	application.RegisterHandler(ListProducts).
		Named("list-products").
		HTTP("GET", "/products").
		Done()

	application.RegisterHandler(UpdateProduct).
		Named("update-product").
		HTTP("PUT", "/products/:id").
		Done()

	application.RegisterHandler(DeleteProduct).
		Named("delete-product").
		HTTP("DELETE", "/products/:id").
		Done()

	// === Search ===
	application.RegisterHandler(SearchProducts).
		Named("search-products").
		HTTP("GET", "/products/search").
		Done()

	// === Message Queue ===
	application.RegisterHandler(ProcessProductCreatedEvent).
		Named("process-product-created").
		HTTP("POST", "/events/product-created"). // Can also trigger via HTTP for testing
		Message("product.created").              // Also listen to message broker
		Done()

	// === Scheduled Tasks ===
	// Uncomment to enable cron
	// application.RegisterHandler(CleanupExpiredProducts).
	// 	Named("cleanup-products").
	// 	Cron("0 0 * * *"). // Every day at midnight
	// 	Done()

	// === Health & Metrics ===
	application.RegisterHandler(HealthCheck).
		Named("health").
		HTTP("GET", "/health").
		Done()

	application.RegisterHandler(GetMetrics).
		Named("metrics").
		HTTP("GET", "/metrics").
		Done()

	// === Error Examples ===
	application.RegisterHandler(SimulateError).
		Named("simulate-error").
		HTTP("GET", "/error").
		Done()

	// Lifecycle hooks
	application.OnStart(func() error {
		fmt.Println("🚀 Application started successfully!")
		fmt.Println("\n📚 Available Endpoints:")
		fmt.Println("  CRUD Operations:")
		fmt.Println("    POST   /products           - Create product")
		fmt.Println("    GET    /products           - List products")
		fmt.Println("    GET    /products/:id       - Get product by ID")
		fmt.Println("    PUT    /products/:id       - Update product")
		fmt.Println("    DELETE /products/:id       - Delete product")
		fmt.Println("    GET    /products/search    - Search products")
		fmt.Println("\n  Events:")
		fmt.Println("    POST   /events/product-created - Trigger product created event")
		fmt.Println("\n  Monitoring:")
		fmt.Println("    GET    /health             - Health check")
		fmt.Println("    GET    /metrics            - Get metrics")
		fmt.Println("\n  Testing:")
		fmt.Println("    GET    /error?type=X       - Simulate error (validation|not_found|server)")
		fmt.Println()
		return nil
	})

	application.OnStop(func() error {
		fmt.Println("👋 Application stopped gracefully")
		return nil
	})

	// Start!
	log.Println("Starting Unicorn Complete Example")
	if err := application.Start(); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}
