package main

import (
	"fmt"
	"log"
	"os"
	"time"

	httpAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/http"
	"github.com/madcok-co/unicorn/core/pkg/app"
	"github.com/madcok-co/unicorn/core/pkg/context"
	"github.com/madcok-co/unicorn/core/pkg/contracts"

	// Adapters
	memoryBroker "github.com/madcok-co/unicorn/core/pkg/adapters/broker/memory"
	cacheAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/cache"
	loggerAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/logger"

	// Security
	"github.com/madcok-co/unicorn/core/pkg/adapters/security/auth"
	"github.com/madcok-co/unicorn/core/pkg/adapters/security/hasher"
)

// ============================================================
// MODELS & DTOs
// ============================================================

type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Password  string    `json:"-"` // Never expose password in JSON
	CreatedAt time.Time `json:"created_at"`
}

type RegisterRequest struct {
	Username string `json:"username" validate:"required,min=3,max=50"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type LoginResponse struct {
	Token string `json:"token"`
	User  *User  `json:"user"`
}

type Product struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Price       float64   `json:"price"`
	Stock       int       `json:"stock"`
	CreatedAt   time.Time `json:"created_at"`
}

type CreateProductRequest struct {
	Name        string  `json:"name" validate:"required,min=3"`
	Description string  `json:"description" validate:"required"`
	Price       float64 `json:"price" validate:"required,gt=0"`
	Stock       int     `json:"stock" validate:"required,gte=0"`
}

// ============================================================
// AUTHENTICATION HANDLERS
// ============================================================

// Register - Create new user with password hashing
func Register(ctx *context.Context, req RegisterRequest) (map[string]interface{}, error) {
	logger := ctx.Logger()
	cache := ctx.Cache()

	// Get password hasher from context
	passwordHasher := ctx.GetService("passwordHasher").(hasher.PasswordHasher)

	// Hash password
	hashedPassword, err := passwordHasher.Hash(req.Password)
	if err != nil {
		logger.Error("failed to hash password", "error", err)
		return nil, fmt.Errorf("failed to create user")
	}

	// Create user
	user := &User{
		ID:        fmt.Sprintf("user_%d", time.Now().Unix()),
		Username:  req.Username,
		Email:     req.Email,
		Password:  hashedPassword,
		CreatedAt: time.Now(),
	}

	// Save to cache (in production: save to database)
	cacheKey := fmt.Sprintf("user:email:%s", user.Email)
	if err := cache.Set(ctx.Context(), cacheKey, user, 24*time.Hour); err != nil {
		logger.Error("failed to cache user", "error", err)
	}

	logger.Info("user registered", "user_id", user.ID, "email", user.Email)

	return map[string]interface{}{
		"message": "User registered successfully",
		"user_id": user.ID,
	}, nil
}

// Login - Authenticate user and return JWT token
func Login(ctx *context.Context, req LoginRequest) (*LoginResponse, error) {
	logger := ctx.Logger()
	cache := ctx.Cache()

	// Get services
	passwordHasher := ctx.GetService("passwordHasher").(hasher.PasswordHasher)
	jwtAuth := ctx.GetService("jwtAuth").(*auth.JWTAuthenticator)

	// Get user from cache (in production: get from database)
	cacheKey := fmt.Sprintf("user:email:%s", req.Email)
	var user User
	if err := cache.Get(ctx.Context(), cacheKey, &user); err != nil {
		logger.Warn("user not found", "email", req.Email)
		return nil, fmt.Errorf("invalid credentials")
	}

	// Verify password
	if err := passwordHasher.Verify(req.Password, user.Password); err != nil {
		logger.Warn("invalid password", "email", req.Email)
		return nil, fmt.Errorf("invalid credentials")
	}

	// Generate JWT token
	identity := &contracts.Identity{
		ID:    user.ID,
		Name:  user.Username,
		Email: user.Email,
	}
	tokenPair, err := jwtAuth.IssueTokens(identity)
	if err != nil {
		logger.Error("failed to generate token", "error", err)
		return nil, fmt.Errorf("failed to generate token")
	}

	logger.Info("user logged in", "user_id", user.ID, "email", user.Email)

	return &LoginResponse{
		Token: tokenPair.AccessToken,
		User:  &user,
	}, nil
}

// VerifyToken - Verify JWT token
func VerifyToken(ctx *context.Context) (map[string]interface{}, error) {
	// Get token from Authorization header
	authHeader := ctx.Request().Headers["Authorization"]
	if authHeader == "" {
		return nil, fmt.Errorf("missing authorization header")
	}

	// Extract token (format: "Bearer <token>")
	token := authHeader
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		token = authHeader[7:]
	}

	// Verify token
	jwtAuth := ctx.GetService("jwtAuth").(*auth.JWTAuthenticator)
	identity, err := jwtAuth.Validate(ctx.Context(), token)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	return map[string]interface{}{
		"valid": true,
		"identity": map[string]interface{}{
			"id":    identity.ID,
			"name":  identity.Name,
			"email": identity.Email,
		},
	}, nil
}

// GetProfile - Get authenticated user profile
func GetProfile(ctx *context.Context) (map[string]interface{}, error) {
	// In production, you would extract user ID from JWT token
	// For demo, we'll use a header
	userID := ctx.Request().Headers["X-User-ID"]
	if userID == "" {
		userID = "demo-user"
	}

	// Get user from cache/database
	cache := ctx.Cache()
	var user User
	cacheKey := fmt.Sprintf("user:id:%s", userID)

	if err := cache.Get(ctx.Context(), cacheKey, &user); err != nil {
		// Return mock data if not found
		return map[string]interface{}{
			"id":       userID,
			"username": "demo_user",
			"email":    "demo@example.com",
		}, nil
	}

	return map[string]interface{}{
		"id":       user.ID,
		"username": user.Username,
		"email":    user.Email,
	}, nil
}

// ============================================================
// PRODUCT HANDLERS (CRUD with Auth)
// ============================================================

// CreateProduct - Create product (requires authentication)
func CreateProduct(ctx *context.Context, req CreateProductRequest) (*Product, error) {
	logger := ctx.Logger()
	cache := ctx.Cache()

	// In production: check authentication here

	product := &Product{
		ID:          fmt.Sprintf("prod_%d", time.Now().Unix()),
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		Stock:       req.Stock,
		CreatedAt:   time.Now(),
	}

	// Save to cache
	cacheKey := fmt.Sprintf("product:%s", product.ID)
	if cache != nil {
		if err := cache.Set(ctx.Context(), cacheKey, product, 1*time.Hour); err != nil {
			logger.Error("failed to cache product", "error", err)
		}
	}

	logger.Info("product created", "id", product.ID, "name", product.Name)

	return product, nil
}

// ListProducts - List all products with pagination
func ListProducts(ctx *context.Context) (map[string]interface{}, error) {
	page := ctx.Request().Query["page"]
	if page == "" {
		page = "1"
	}

	// Mock data
	products := []*Product{
		{ID: "prod_1", Name: "Laptop", Description: "Gaming Laptop", Price: 1299.99, Stock: 10, CreatedAt: time.Now()},
		{ID: "prod_2", Name: "Mouse", Description: "Wireless Mouse", Price: 29.99, Stock: 50, CreatedAt: time.Now()},
		{ID: "prod_3", Name: "Keyboard", Description: "Mechanical Keyboard", Price: 99.99, Stock: 30, CreatedAt: time.Now()},
	}

	return map[string]interface{}{
		"data": products,
		"meta": map[string]interface{}{
			"page":  page,
			"total": len(products),
		},
	}, nil
}

// GetProduct - Get product by ID
func GetProduct(ctx *context.Context) (*Product, error) {
	productID := ctx.Request().Params["id"]
	logger := ctx.Logger()

	// Try cache first
	if ctx.Cache() != nil {
		cacheKey := fmt.Sprintf("product:%s", productID)
		var product Product
		if err := ctx.Cache().Get(ctx.Context(), cacheKey, &product); err == nil {
			logger.Info("product retrieved from cache", "id", productID)
			return &product, nil
		}
	}

	// Return mock data
	return &Product{
		ID:          productID,
		Name:        "Sample Product",
		Description: "This is a sample product",
		Price:       99.99,
		Stock:       100,
		CreatedAt:   time.Now(),
	}, nil
}

// ============================================================
// HEALTH & METRICS
// ============================================================

func HealthCheck(ctx *context.Context) (map[string]interface{}, error) {
	cache := ctx.Cache()
	cacheStatus := "ok"

	if cache != nil {
		if err := cache.Set(ctx.Context(), "health:check", "ok", 10*time.Second); err != nil {
			cacheStatus = "error"
		}
	}

	return map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
		"services": map[string]string{
			"cache": cacheStatus,
		},
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
		Name:       getEnv("APP_NAME", "unicorn-complete-example"),
		Version:    getEnv("APP_VERSION", "1.0.0"),
		EnableHTTP: true,
		HTTP: &httpAdapter.Config{
			Host: getEnv("HTTP_HOST", "0.0.0.0"),
			Port: getEnvInt("HTTP_PORT", 8080),
		},
		EnableBroker: getEnvBool("ENABLE_BROKER", true),
	})

	// Setup infrastructure
	logger := loggerAdapter.NewConsoleLogger("info")
	application.SetLogger(logger)

	cache := cacheAdapter.New(cacheAdapter.NewMemoryDriver())
	application.SetCache(cache)

	broker := memoryBroker.New()
	application.SetBroker(broker)

	// Setup security services
	passwordHasher := hasher.NewBcryptHasher(nil) // Use default config
	application.RegisterService("passwordHasher", passwordHasher)

	jwtAuth := auth.NewJWTAuthenticator(&auth.JWTConfig{
		SecretKey:      jwtSecret,
		AccessTokenTTL: 24 * time.Hour,
	})
	application.RegisterService("jwtAuth", jwtAuth)

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

	application.RegisterHandler(VerifyToken).
		Named("verify-token").
		HTTP("POST", "/auth/verify").
		Done()

	application.RegisterHandler(GetProfile).
		Named("get-profile").
		HTTP("GET", "/auth/profile").
		Done()

	// === Products (CRUD) ===
	application.RegisterHandler(CreateProduct).
		Named("create-product").
		HTTP("POST", "/products").
		Done()

	application.RegisterHandler(ListProducts).
		Named("list-products").
		HTTP("GET", "/products").
		Done()

	application.RegisterHandler(GetProduct).
		Named("get-product").
		HTTP("GET", "/products/:id").
		Done()

	// === Health ===
	application.RegisterHandler(HealthCheck).
		Named("health").
		HTTP("GET", "/health").
		Done()

	// Startup hook
	application.OnStart(func() error {
		fmt.Println("🚀 Unicorn Enhanced Example Started!")
		fmt.Println("\n📚 Available Endpoints:")
		fmt.Println("  Authentication:")
		fmt.Println("    POST /auth/register    - Register new user")
		fmt.Println("    POST /auth/login       - Login and get JWT token")
		fmt.Println("    POST /auth/verify      - Verify JWT token")
		fmt.Println("    GET  /auth/profile     - Get user profile")
		fmt.Println("\n  Products:")
		fmt.Println("    POST /products         - Create product")
		fmt.Println("    GET  /products         - List products")
		fmt.Println("    GET  /products/:id     - Get product by ID")
		fmt.Println("\n  Health:")
		fmt.Println("    GET  /health           - Health check")
		fmt.Println("\n🔐 Security Features:")
		fmt.Println("  ✓ Password Hashing (bcrypt)")
		fmt.Println("  ✓ JWT Authentication")
		fmt.Println("  ✓ Token Verification")
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
