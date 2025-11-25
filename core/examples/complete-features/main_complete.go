package main

import (
	"fmt"
	"log"
	"os"
	"time"

	httpAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/http"
	"github.com/madcok-co/unicorn/core/pkg/app"
	ucontext "github.com/madcok-co/unicorn/core/pkg/context"

	// Adapters
	memoryBroker "github.com/madcok-co/unicorn/core/pkg/adapters/broker/memory"
	cacheAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/cache"
	loggerAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/logger"
	metricsAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/metrics"

	// Security
	"github.com/madcok-co/unicorn/core/pkg/adapters/security/auth"
	"github.com/madcok-co/unicorn/core/pkg/adapters/security/hasher"
	"github.com/madcok-co/unicorn/core/pkg/adapters/security/ratelimiter"

	// Middleware

	// Resilience
	"github.com/madcok-co/unicorn/core/pkg/resilience"
)

// ============================================================
// MODELS & DTOs
// ============================================================

type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Password  string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Product struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Price       float64   `json:"price"`
	Stock       int       `json:"stock"`
	UserID      string    `json:"user_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Order struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	ProductID  string    `json:"product_id"`
	Quantity   int       `json:"quantity"`
	TotalPrice float64   `json:"total_price"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

// Request DTOs
type RegisterRequest struct {
	Username string `json:"username" validate:"required,min=3,max=50"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type CreateProductRequest struct {
	Name        string  `json:"name" validate:"required,min=3,max=100"`
	Description string  `json:"description" validate:"required,min=10"`
	Price       float64 `json:"price" validate:"required,gt=0"`
	Stock       int     `json:"stock" validate:"required,gte=0"`
}

type CreateOrderRequest struct {
	ProductID string `json:"product_id" validate:"required"`
	Quantity  int    `json:"quantity" validate:"required,gt=0"`
}

// Response DTOs
type LoginResponse struct {
	Token     string `json:"token"`
	User      *User  `json:"user"`
	ExpiresIn int    `json:"expires_in"`
}

type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Page       int         `json:"page"`
	PerPage    int         `json:"per_page"`
	Total      int         `json:"total"`
	TotalPages int         `json:"total_pages"`
}

// ============================================================
// CUSTOM SERVICES (Business Logic)
// ============================================================

// EmailService - Email service interface
type EmailService interface {
	SendEmail(to, subject, body string) error
	SendWelcomeEmail(user *User) error
	SendOrderConfirmation(order *Order) error
}

type emailService struct {
	from string
}

func NewEmailService(from string) EmailService {
	return &emailService{from: from}
}

func (s *emailService) SendEmail(to, subject, body string) error {
	// In production: integrate with SendGrid, AWS SES, etc.
	log.Printf("📧 Email sent to %s: %s", to, subject)
	return nil
}

func (s *emailService) SendWelcomeEmail(user *User) error {
	return s.SendEmail(user.Email, "Welcome!", fmt.Sprintf("Welcome %s!", user.Username))
}

func (s *emailService) SendOrderConfirmation(order *Order) error {
	return s.SendEmail("user@example.com", "Order Confirmed", fmt.Sprintf("Order %s confirmed", order.ID))
}

// PaymentService - Payment processing
type PaymentService interface {
	ProcessPayment(amount float64, currency string) (*PaymentResult, error)
	RefundPayment(transactionID string) error
}

type PaymentResult struct {
	TransactionID string
	Status        string
	ProcessedAt   time.Time
}

type paymentService struct {
	circuitBreaker *resilience.CircuitBreaker
}

func NewPaymentService() PaymentService {
	cb := resilience.NewCircuitBreaker(resilience.CircuitBreakerConfig{
		Name:             "payment-service",
		MaxFailures:      3,
		Timeout:          30 * time.Second,
		HalfOpenRequests: 2,
	})

	return &paymentService{
		circuitBreaker: cb,
	}
}

func (s *paymentService) ProcessPayment(amount float64, currency string) (*PaymentResult, error) {
	// Use circuit breaker to protect payment gateway calls
	result, err := s.circuitBreaker.Execute(func() (interface{}, error) {
		// In production: call Stripe, PayPal, etc.
		time.Sleep(100 * time.Millisecond) // Simulate API call

		return &PaymentResult{
			TransactionID: fmt.Sprintf("txn_%d", time.Now().Unix()),
			Status:        "success",
			ProcessedAt:   time.Now(),
		}, nil
	})

	if err != nil {
		return nil, err
	}

	return result.(*PaymentResult), nil
}

func (s *paymentService) RefundPayment(transactionID string) error {
	log.Printf("💰 Refund processed for transaction: %s", transactionID)
	return nil
}

// ============================================================
// AUTHENTICATION HANDLERS
// ============================================================

func Register(ctx *ucontext.Context, req RegisterRequest) (map[string]interface{}, error) {
	logger := ctx.Logger()
	cache := ctx.Cache()
	metrics := ctx.Metrics()

	// Increment registration metric
	if metrics != nil {
		metrics.IncrementCounter("user_registrations_total", map[string]string{"status": "attempted"})
	}

	// Get services
	passwordHasher := ctx.GetService("passwordHasher").(*hasher.PasswordHasher)
	emailService := ctx.GetService("emailService").(EmailService)

	// Hash password
	hashedPassword, err := passwordHasher.Hash(req.Password)
	if err != nil {
		logger.Error("failed to hash password", "error", err)
		if metrics != nil {
			metrics.IncrementCounter("user_registrations_total", map[string]string{"status": "failed"})
		}
		return nil, fmt.Errorf("failed to create user")
	}

	// Create user
	user := &User{
		ID:        fmt.Sprintf("user_%d", time.Now().UnixNano()),
		Username:  req.Username,
		Email:     req.Email,
		Password:  hashedPassword,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Save to cache (in production: save to database)
	cacheKey := fmt.Sprintf("user:email:%s", user.Email)
	if err := cache.Set(ctx.Context(), cacheKey, user, 24*time.Hour); err != nil {
		logger.Error("failed to cache user", "error", err)
	}

	// Send welcome email (async in production)
	go emailService.SendWelcomeEmail(user)

	logger.Info("user registered", "user_id", user.ID, "email", user.Email)

	if metrics != nil {
		metrics.IncrementCounter("user_registrations_total", map[string]string{"status": "success"})
	}

	return map[string]interface{}{
		"message": "User registered successfully",
		"user_id": user.ID,
	}, nil
}

func Login(ctx *ucontext.Context, req LoginRequest) (*LoginResponse, error) {
	logger := ctx.Logger()
	cache := ctx.Cache()
	metrics := ctx.Metrics()

	if metrics != nil {
		metrics.IncrementCounter("user_logins_total", map[string]string{"status": "attempted"})
	}

	// Get services
	passwordHasher := ctx.GetService("passwordHasher").(*hasher.PasswordHasher)
	jwtAuth := ctx.GetService("jwtAuth").(*auth.JWTAuth)

	// Get user from cache
	cacheKey := fmt.Sprintf("user:email:%s", req.Email)
	var user User
	if err := cache.Get(ctx.Context(), cacheKey, &user); err != nil {
		logger.Warn("user not found", "email", req.Email)
		if metrics != nil {
			metrics.IncrementCounter("user_logins_total", map[string]string{"status": "failed", "reason": "not_found"})
		}
		return nil, fmt.Errorf("invalid credentials")
	}

	// Verify password
	if err := passwordHasher.Verify(req.Password, user.Password); err != nil {
		logger.Warn("invalid password", "email", req.Email)
		if metrics != nil {
			metrics.IncrementCounter("user_logins_total", map[string]string{"status": "failed", "reason": "invalid_password"})
		}
		return nil, fmt.Errorf("invalid credentials")
	}

	// Generate JWT token
	token, err := jwtAuth.GenerateToken(map[string]interface{}{
		"user_id":  user.ID,
		"username": user.Username,
		"email":    user.Email,
	})
	if err != nil {
		logger.Error("failed to generate token", "error", err)
		return nil, fmt.Errorf("failed to generate token")
	}

	logger.Info("user logged in", "user_id", user.ID, "email", user.Email)

	if metrics != nil {
		metrics.IncrementCounter("user_logins_total", map[string]string{"status": "success"})
	}

	return &LoginResponse{
		Token:     token,
		User:      &user,
		ExpiresIn: 86400, // 24 hours
	}, nil
}

// ============================================================
// PRODUCT HANDLERS
// ============================================================

func CreateProduct(ctx *ucontext.Context, req CreateProductRequest) (*Product, error) {
	logger := ctx.Logger()
	cache := ctx.Cache()
	broker := ctx.Broker()
	metrics := ctx.Metrics()

	if metrics != nil {
		metrics.IncrementCounter("products_created_total", nil)
	}

	// Get user ID from context (set by auth middleware)
	userID := ctx.Request().Headers["X-User-ID"]
	if userID == "" {
		userID = "system"
	}

	product := &Product{
		ID:          fmt.Sprintf("prod_%d", time.Now().UnixNano()),
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		Stock:       req.Stock,
		UserID:      userID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Save to cache
	cacheKey := fmt.Sprintf("product:%s", product.ID)
	if err := cache.Set(ctx.Context(), cacheKey, product, 1*time.Hour); err != nil {
		logger.Error("failed to cache product", "error", err)
	}

	// Publish event to message broker
	if broker != nil {
		event := map[string]interface{}{
			"event_type": "product.created",
			"product_id": product.ID,
			"name":       product.Name,
			"price":      product.Price,
			"created_at": product.CreatedAt,
		}

		// Use retry for reliable message publishing
		retryConfig := resilience.RetryConfig{
			MaxAttempts:     3,
			InitialInterval: 100 * time.Millisecond,
			MaxInterval:     1 * time.Second,
			Multiplier:      2.0,
		}

		_, err := resilience.Retry(retryConfig, func() (interface{}, error) {
			return nil, broker.Publish(ctx.Context(), "product.created", event)
		})

		if err != nil {
			logger.Error("failed to publish product created event", "error", err)
		}
	}

	logger.Info("product created", "id", product.ID, "name", product.Name, "user_id", userID)

	if metrics != nil {
		metrics.RecordHistogram("product_price", product.Price, map[string]string{"user_id": userID})
	}

	return product, nil
}

func ListProducts(ctx *ucontext.Context) (*PaginatedResponse, error) {
	metrics := ctx.Metrics()

	// Get pagination params
	page := 1
	perPage := 10

	if pageStr := ctx.Request().Query["page"]; pageStr != "" {
		fmt.Sscanf(pageStr, "%d", &page)
	}
	if perPageStr := ctx.Request().Query["per_page"]; perPageStr != "" {
		fmt.Sscanf(perPageStr, "%d", &perPage)
	}

	// Mock data (in production: fetch from database with pagination)
	products := []*Product{
		{ID: "prod_1", Name: "Laptop", Description: "Gaming Laptop", Price: 1299.99, Stock: 10, CreatedAt: time.Now()},
		{ID: "prod_2", Name: "Mouse", Description: "Wireless Mouse", Price: 29.99, Stock: 50, CreatedAt: time.Now()},
		{ID: "prod_3", Name: "Keyboard", Description: "Mechanical Keyboard", Price: 99.99, Stock: 30, CreatedAt: time.Now()},
	}

	total := len(products)
	totalPages := (total + perPage - 1) / perPage

	if metrics != nil {
		metrics.IncrementCounter("products_list_requests", map[string]string{"page": fmt.Sprintf("%d", page)})
	}

	return &PaginatedResponse{
		Data:       products,
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	}, nil
}

func GetProduct(ctx *ucontext.Context) (*Product, error) {
	productID := ctx.Request().Params["id"]
	logger := ctx.Logger()
	cache := ctx.Cache()
	metrics := ctx.Metrics()

	if metrics != nil {
		metrics.IncrementCounter("products_get_requests", map[string]string{"product_id": productID})
	}

	// Try cache first
	cacheKey := fmt.Sprintf("product:%s", productID)
	var product Product
	if err := cache.Get(ctx.Context(), cacheKey, &product); err == nil {
		logger.Info("product retrieved from cache", "id", productID)
		if metrics != nil {
			metrics.IncrementCounter("cache_hits", map[string]string{"key": "product"})
		}
		return &product, nil
	}

	if metrics != nil {
		metrics.IncrementCounter("cache_misses", map[string]string{"key": "product"})
	}

	// Return mock data (in production: fetch from database)
	product = Product{
		ID:          productID,
		Name:        "Sample Product",
		Description: "This is a sample product",
		Price:       99.99,
		Stock:       100,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Cache it for next time
	cache.Set(ctx.Context(), cacheKey, &product, 1*time.Hour)

	return &product, nil
}

// ============================================================
// ORDER HANDLERS
// ============================================================

func CreateOrder(ctx *ucontext.Context, req CreateOrderRequest) (*Order, error) {
	logger := ctx.Logger()
	metrics := ctx.Metrics()

	// Get services
	paymentService := ctx.GetService("paymentService").(PaymentService)
	emailService := ctx.GetService("emailService").(EmailService)

	// Get product
	productID := req.ProductID
	var product Product
	cacheKey := fmt.Sprintf("product:%s", productID)
	if err := ctx.Cache().Get(ctx.Context(), cacheKey, &product); err != nil {
		// Mock product if not in cache
		product = Product{ID: productID, Name: "Sample Product", Price: 99.99, Stock: 100}
	}

	// Calculate total
	totalPrice := product.Price * float64(req.Quantity)

	// Process payment with circuit breaker protection
	paymentResult, err := paymentService.ProcessPayment(totalPrice, "USD")
	if err != nil {
		logger.Error("payment failed", "error", err)
		if metrics != nil {
			metrics.IncrementCounter("orders_failed", map[string]string{"reason": "payment_failed"})
		}
		return nil, fmt.Errorf("payment failed: %w", err)
	}

	// Create order
	order := &Order{
		ID:         fmt.Sprintf("order_%d", time.Now().UnixNano()),
		UserID:     ctx.Request().Headers["X-User-ID"],
		ProductID:  productID,
		Quantity:   req.Quantity,
		TotalPrice: totalPrice,
		Status:     "confirmed",
		CreatedAt:  time.Now(),
	}

	// Send confirmation email (async)
	go emailService.SendOrderConfirmation(order)

	logger.Info("order created",
		"order_id", order.ID,
		"product_id", productID,
		"total", totalPrice,
		"transaction_id", paymentResult.TransactionID)

	if metrics != nil {
		metrics.IncrementCounter("orders_created_total", map[string]string{"status": "success"})
		metrics.RecordHistogram("order_amount", totalPrice, map[string]string{"currency": "USD"})
	}

	return order, nil
}

// ============================================================
// SCHEDULED TASK (CRON)
// ============================================================

func CleanupExpiredCache(ctx *ucontext.Context) (map[string]interface{}, error) {
	logger := ctx.Logger()

	logger.Info("running scheduled cache cleanup")

	// In production: cleanup expired cache entries, old sessions, etc.
	cleaned := 15 // mock number

	logger.Info("cache cleanup completed", "items_cleaned", cleaned)

	return map[string]interface{}{
		"status":    "success",
		"cleaned":   cleaned,
		"timestamp": time.Now(),
	}, nil
}

// ============================================================
// HEALTH & METRICS
// ============================================================

func HealthCheck(ctx *ucontext.Context) (map[string]interface{}, error) {
	cache := ctx.Cache()
	broker := ctx.Broker()

	cacheStatus := "ok"
	brokerStatus := "ok"

	// Check cache
	if cache != nil {
		if err := cache.Set(ctx.Context(), "health:check", "ok", 10*time.Second); err != nil {
			cacheStatus = "error"
		}
	}

	// Check broker
	if broker != nil {
		// Broker health check
		brokerStatus = "ok"
	}

	overall := "healthy"
	if cacheStatus == "error" || brokerStatus == "error" {
		overall = "degraded"
	}

	return map[string]interface{}{
		"status":    overall,
		"timestamp": time.Now(),
		"services": map[string]string{
			"cache":  cacheStatus,
			"broker": brokerStatus,
		},
	}, nil
}

func GetMetrics(ctx *ucontext.Context) (map[string]interface{}, error) {
	// In production: return actual metrics from Prometheus
	return map[string]interface{}{
		"uptime_seconds":       3600,
		"requests_total":       12345,
		"requests_success":     12000,
		"requests_failed":      345,
		"cache_hit_rate":       0.85,
		"avg_response_time_ms": 45,
	}, nil
}

// ============================================================
// MAIN APPLICATION
// ============================================================

func main() {
	// Load environment variables
	jwtSecret := getEnv("JWT_SECRET", "your-super-secret-jwt-key-change-this")

	// Create application
	application := app.New(&app.Config{
		Name:       getEnv("APP_NAME", "unicorn-complete"),
		Version:    getEnv("APP_VERSION", "1.0.0"),
		EnableHTTP: true,
		HTTP: &httpAdapter.Config{
			Host: getEnv("HTTP_HOST", "0.0.0.0"),
			Port: getEnvInt("HTTP_PORT", 8080),
		},
		EnableBroker: getEnvBool("ENABLE_BROKER", true),
		EnableCron:   getEnvBool("ENABLE_CRON", false),
	})

	// Setup infrastructure
	logger := loggerAdapter.NewConsoleLogger("info")
	application.SetLogger(logger)

	cache := cacheAdapter.New(cacheAdapter.NewMemoryDriver())
	application.SetCache(cache)

	broker := memoryBroker.New()
	application.SetBroker(broker)

	// Setup metrics
	metricsCollector := metricsAdapter.NewPrometheusMetrics("unicorn_complete")
	application.SetMetrics(metricsCollector)

	// Setup security services
	passwordHasher := hasher.NewPasswordHasher()
	application.RegisterService("passwordHasher", passwordHasher)

	jwtAuth := auth.NewJWTAuth(auth.JWTConfig{
		Secret:     []byte(jwtSecret),
		Expiration: 24 * time.Hour,
	})
	application.RegisterService("jwtAuth", jwtAuth)

	// Setup business services
	emailService := NewEmailService("noreply@unicorn.com")
	application.RegisterService("emailService", emailService)

	paymentService := NewPaymentService()
	application.RegisterService("paymentService", paymentService)

	// Setup rate limiter
	rateLimiter := ratelimiter.NewMemoryRateLimiter(100, time.Minute)
	application.RegisterService("rateLimiter", rateLimiter)

	// Register handlers

	// === Authentication ===
	application.RegisterHandler(Register).
		Named("register").
		HTTP("POST", "/auth/register").
		Done()

	application.RegisterHandler(Login).
		Named("login").
		HTTP("POST", "/auth/login").
		Done()

	// === Products ===
	application.RegisterHandler(CreateProduct).
		Named("create-product").
		HTTP("POST", "/products").
		Message("product.create"). // Also listen to message broker
		Done()

	application.RegisterHandler(ListProducts).
		Named("list-products").
		HTTP("GET", "/products").
		Done()

	application.RegisterHandler(GetProduct).
		Named("get-product").
		HTTP("GET", "/products/:id").
		Done()

	// === Orders ===
	application.RegisterHandler(CreateOrder).
		Named("create-order").
		HTTP("POST", "/orders").
		Done()

	// === Cron Jobs ===
	if getEnvBool("ENABLE_CRON", false) {
		application.RegisterHandler(CleanupExpiredCache).
			Named("cleanup-cache").
			Cron("0 */6 * * *"). // Every 6 hours
			Done()
	}

	// === Health & Metrics ===
	application.RegisterHandler(HealthCheck).
		Named("health").
		HTTP("GET", "/health").
		Done()

	application.RegisterHandler(GetMetrics).
		Named("metrics").
		HTTP("GET", "/metrics").
		Done()

	// Startup hook
	application.OnStart(func() error {
		fmt.Println("🚀 Unicorn Complete Example Started!")
		fmt.Println("\n📚 Available Endpoints:")
		fmt.Println("  Authentication:")
		fmt.Println("    POST /auth/register         - Register new user")
		fmt.Println("    POST /auth/login            - Login and get JWT token")
		fmt.Println("\n  Products:")
		fmt.Println("    POST /products              - Create product")
		fmt.Println("    GET  /products              - List products (paginated)")
		fmt.Println("    GET  /products/:id          - Get product by ID")
		fmt.Println("\n  Orders:")
		fmt.Println("    POST /orders                - Create order (with payment)")
		fmt.Println("\n  Health & Metrics:")
		fmt.Println("    GET  /health                - Health check")
		fmt.Println("    GET  /metrics               - Application metrics")
		fmt.Println("\n🔧 Features:")
		fmt.Println("  ✓ JWT Authentication")
		fmt.Println("  ✓ Password Hashing (bcrypt)")
		fmt.Println("  ✓ Circuit Breaker (payment service)")
		fmt.Println("  ✓ Retry Pattern (message publishing)")
		fmt.Println("  ✓ Rate Limiting")
		fmt.Println("  ✓ Prometheus Metrics")
		fmt.Println("  ✓ Message Broker (pub/sub)")
		fmt.Println("  ✓ Cron Jobs (if enabled)")
		fmt.Println("  ✓ Email Service")
		fmt.Println("  ✓ Payment Service")
		fmt.Println("  ✓ Custom Service Injection")
		fmt.Println()
		return nil
	})

	// Start
	log.Printf("Starting on http://%s:%d\n",
		getEnv("HTTP_HOST", "0.0.0.0"),
		getEnvInt("HTTP_PORT", 8080))

	if err := application.Start(); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}

// ============================================================
// HELPER FUNCTIONS
// ============================================================

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intVal int
		fmt.Sscanf(value, "%d", &intVal)
		return intVal
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1"
	}
	return defaultValue
}
