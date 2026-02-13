package main

import (
	"fmt"
	"log"
	"time"

	"github.com/madcok-co/unicorn/contrib/auth/oauth2"
	"github.com/madcok-co/unicorn/contrib/authz/rbac"
	"github.com/madcok-co/unicorn/contrib/config"
	"github.com/madcok-co/unicorn/contrib/multitenancy"
	"github.com/madcok-co/unicorn/contrib/pagination"
	"github.com/madcok-co/unicorn/contrib/versioning"
	httpAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/http"
	"github.com/madcok-co/unicorn/core/pkg/app"
	appContext "github.com/madcok-co/unicorn/core/pkg/context"
	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

// ============================================================================
// Domain Models
// ============================================================================

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	TenantID  string    `json:"tenant_id"`
	Roles     []string  `json:"roles"`
	CreatedAt time.Time `json:"created_at"`
}

type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	TenantID    string    `json:"tenant_id"`
	OwnerID     string    `json:"owner_id"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ============================================================================
// Request/Response DTOs
// ============================================================================

type LoginRequest struct {
	Code  string `json:"code" validate:"required"`
	State string `json:"state" validate:"required"`
}

type LoginResponse struct {
	Token        string    `json:"token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	User         User      `json:"user"`
}

type CreateProjectRequest struct {
	Name        string `json:"name" validate:"required,min=3,max=100"`
	Description string `json:"description" validate:"max=500"`
}

type UpdateProjectRequest struct {
	Name        string `json:"name" validate:"omitempty,min=3,max=100"`
	Description string `json:"description" validate:"omitempty,max=500"`
	Status      string `json:"status" validate:"omitempty,oneof=active inactive archived"`
}

type ListProjectsRequest struct {
	Page   int    `query:"page"`
	Limit  int    `query:"limit"`
	Sort   string `query:"sort"`
	Order  string `query:"order"`
	Cursor string `query:"cursor"`
}

type ProjectResponse struct {
	Project Project `json:"project"`
}

// ============================================================================
// Global Infrastructure (In real app, use proper dependency injection)
// ============================================================================

var (
	tenantManager  *multitenancy.Driver
	authzManager   *rbac.Driver
	versionManager *versioning.Manager
	configManager  *config.Driver
)

// ============================================================================
// Main Application Setup
// ============================================================================

func main() {
	// Initialize configuration
	configManager = initializeConfig()

	// Initialize multi-tenancy
	tenantManager = initializeMultiTenancy()

	// Initialize OAuth2 authentication
	authDriver := initializeOAuth2()

	// Initialize RBAC authorization
	authzManager = initializeRBAC()

	// Initialize API versioning
	versionManager = initializeVersioning()

	// Create application
	application := app.New(&app.Config{
		Name:       configManager.GetString("app.name"),
		Version:    configManager.GetString("app.version"),
		EnableHTTP: true,
		HTTP: &httpAdapter.Config{
			Host: configManager.GetString("http.host"),
			Port: configManager.GetInt("http.port"),
		},
	})

	// Set authentication and authorization
	application.SetAuth(authDriver)
	application.SetAuthz(authzManager)

	// Register API v1 handlers
	registerV1Handlers(application)

	// Register API v2 handlers
	registerV2Handlers(application)

	// Start application
	log.Printf("Starting %s v%s on %s:%d",
		configManager.GetString("app.name"),
		configManager.GetString("app.version"),
		configManager.GetString("http.host"),
		configManager.GetInt("http.port"),
	)

	if err := application.Start(); err != nil {
		log.Fatal(err)
	}
}

// ============================================================================
// Infrastructure Initialization
// ============================================================================

func initializeConfig() *config.Driver {
	cfg := config.NewDriver(&config.Config{
		Defaults: map[string]interface{}{
			"app.name":                 "Enterprise SaaS Platform",
			"app.version":              "1.0.0",
			"http.host":                "0.0.0.0",
			"http.port":                8080,
			"oauth.provider":           "your-client-id",
			"oauth.client_secret":      "your-client-secret",
			"oauth.redirect_url":       "http://localhost:8080/api/v1/auth/callback",
			"multitenancy.strategy":    "subdomain",
			"multitenancy.domain":      "myapp.com",
			"pagination.default_limit": 20,
			"pagination.max_limit":     100,
		},
		EnvPrefix:  "APP",
		AutoReload: true,
	})

	// Watch for config changes
	cfg.OnChange(func(key string, value interface{}) {
		log.Printf("Config changed: %s = %v", key, value)
	})

	return cfg
}

func initializeMultiTenancy() *multitenancy.Driver {
	mt := multitenancy.NewDriver(&multitenancy.Config{
		Strategy:      multitenancy.StrategySubdomain,
		Domain:        configManager.GetString("multitenancy.domain"),
		DefaultTenant: "default",
	})

	// Create demo tenants
	mt.CreateTenant(&multitenancy.Tenant{
		ID:     "acme",
		Name:   "Acme Corporation",
		Active: true,
		Metadata: map[string]interface{}{
			"plan":           "enterprise",
			"max_us     ers": 100,
		},
	})

	mt.CreateTenant(&multitenancy.Tenant{
		ID:     "techcorp",
		Name:   "Tech Corp",
		Active: true,
		Metadata: map[string]interface{}{
			"plan":           "professional",
			"max_us     ers": 50,
		},
	})

	return mt
}

func initializeOAuth2() *oauth2.Driver {
	return oauth2.NewDriver(&oauth2.Config{
		Provider:     oauth2.ProviderGoogle,
		ClientID:     configManager.GetString("oauth.client_id"),
		ClientSecret: configManager.GetString("oauth.client_secret"),
		RedirectURL:  configManager.GetString("oauth.redirect_url"),
		Scopes:       []string{"openid", "email", "profile"},
	})
}

func initializeRBAC() *rbac.Driver {
	authz := rbac.NewDriver()

	// Define roles and permissions
	authz.CreateRole("super_admin", []string{"*"})
	authz.CreateRole("tenant_admin", []string{
		"projects:*",
		"users:*",
		"settings:*",
	})
	authz.CreateRole("project_manager", []string{
		"projects:read",
		"projects:create",
		"projects:update",
		"users:read",
	})
	authz.CreateRole("developer", []string{
		"projects:read",
		"projects:update",
	})
	authz.CreateRole("viewer", []string{
		"*:read",
	})

	// Set up role inheritance
	authz.SetRoleParent("tenant_admin", "project_manager")
	authz.SetRoleParent("project_manager", "developer")
	authz.SetRoleParent("developer", "viewer")

	// Demo: Assign roles to demo users
	authz.AssignRole("user-admin", "tenant_admin")
	authz.AssignRole("user-pm", "project_manager")
	authz.AssignRole("user-dev", "developer")

	return authz
}

func initializeVersioning() *versioning.Manager {
	vm := versioning.NewManager(&versioning.Config{
		Strategy:       versioning.StrategyURL,
		Prefix:         "/api",
		DefaultVersion: "1.0",
	})

	// Add version deprecation (v1 will be deprecated in 90 days)
	vm.AddDeprecation("1.0", time.Now().Add(90*24*time.Hour), "2.0")

	return vm
}

// ============================================================================
// Handler Registration
// ============================================================================

func registerV1Handlers(app *app.App) {
	// Authentication endpoints
	app.RegisterHandler(LoginV1).
		Named("auth.login.v1").
		HTTP("POST", "/api/v1/auth/login").
		Done()

	app.RegisterHandler(AuthCallbackV1).
		Named("auth.callback.v1").
		HTTP("GET", "/api/v1/auth/callback").
		Done()

	// Project endpoints
	app.RegisterHandler(ListProjectsV1).
		Named("projects.list.v1").
		HTTP("GET", "/api/v1/projects").
		Done()

	app.RegisterHandler(CreateProjectV1).
		Named("projects.create.v1").
		HTTP("POST", "/api/v1/projects").
		Done()

	app.RegisterHandler(GetProjectV1).
		Named("projects.get.v1").
		HTTP("GET", "/api/v1/projects/:id").
		Done()

	app.RegisterHandler(UpdateProjectV1).
		Named("projects.update.v1").
		HTTP("PUT", "/api/v1/projects/:id").
		Done()

	app.RegisterHandler(DeleteProjectV1).
		Named("projects.delete.v1").
		HTTP("DELETE", "/api/v1/projects/:id").
		Done()
}

func registerV2Handlers(app *app.App) {
	// V2 has enhanced pagination with cursor support
	app.RegisterHandler(ListProjectsV2).
		Named("projects.list.v2").
		HTTP("GET", "/api/v2/projects").
		Done()

	// V2 has batch operations
	app.RegisterHandler(BatchUpdateProjectsV2).
		Named("projects.batch_update.v2").
		HTTP("POST", "/api/v2/projects/batch").
		Done()
}

// ============================================================================
// V1 Handlers
// ============================================================================

func LoginV1(ctx *appContext.Context, req LoginRequest) (*LoginResponse, error) {
	// Authenticate with OAuth2
	identity, err := ctx.Auth().Authenticate(ctx.Context(), map[string]interface{}{
		"code":  req.Code,
		"state": req.State,
	})
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	// Create user object
	user := User{
		ID:        identity.ID,
		Email:     identity.Claims["email"].(string),
		Name:      identity.Claims["name"].(string),
		TenantID:  "acme", // In real app, derive from user data
		Roles:     []string{"developer"},
		CreatedAt: time.Now(),
	}

	// Assign default role
	authzManager.AssignRole(user.ID, "developer")

	return &LoginResponse{
		Token:        identity.Token,
		RefreshToken: identity.RefreshToken,
		ExpiresAt:    identity.ExpiresAt,
		User:         user,
	}, nil
}

func AuthCallbackV1(ctx *appContext.Context, req LoginRequest) (*LoginResponse, error) {
	return LoginV1(ctx, req)
}

func ListProjectsV1(ctx *appContext.Context, req ListProjectsRequest) (*pagination.OffsetResult, error) {
	// Resolve tenant from request
	tenant, err := tenantManager.GetTenantFromRequest(ctx.Context(), ctx.Request())
	if err != nil {
		return nil, fmt.Errorf("tenant resolution failed: %w", err)
	}

	// Check authorization
	identity := getCurrentIdentity(ctx)
	if allowed, err := ctx.Authz().Authorize(ctx.Context(), identity, "read", "projects"); err != nil || !allowed {
		return nil, fmt.Errorf("forbidden: insufficient permissions")
	}

	// Parse pagination parameters
	limit := req.Limit
	if limit == 0 {
		limit = configManager.GetInt("pagination.default_limit")
	}
	if limit > configManager.GetInt("pagination.max_limit") {
		limit = configManager.GetInt("pagination.max_limit")
	}

	params := pagination.ParseOffsetParams(req.Page, limit, req.Sort, req.Order)

	// In real app, query from database filtered by tenant
	projects := getProjectsByTenant(tenant.ID)
	total := int64(len(projects))

	// Return paginated result
	return pagination.NewOffsetResult(projects, total, params), nil
}

func CreateProjectV1(ctx *appContext.Context, req CreateProjectRequest) (*ProjectResponse, error) {
	// Resolve tenant
	tenant, err := tenantManager.GetTenantFromRequest(ctx.Context(), ctx.Request())
	if err != nil {
		return nil, fmt.Errorf("tenant resolution failed: %w", err)
	}

	// Check authorization
	identity := getCurrentIdentity(ctx)
	if allowed, err := ctx.Authz().Authorize(ctx.Context(), identity, "create", "projects"); err != nil || !allowed {
		return nil, fmt.Errorf("forbidden: insufficient permissions")
	}

	// Check tenant-specific feature flags
	if !tenant.HasFeature("create_projects") {
		return nil, fmt.Errorf("feature not available in your plan")
	}

	// Create project
	project := Project{
		ID:          fmt.Sprintf("proj-%d", time.Now().Unix()),
		Name:        req.Name,
		Description: req.Description,
		TenantID:    tenant.ID,
		OwnerID:     identity.ID,
		Status:      "active",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// In real app, save to database
	log.Printf("Created project: %+v", project)

	return &ProjectResponse{Project: project}, nil
}

func GetProjectV1(ctx *appContext.Context, req struct{}) (*ProjectResponse, error) {
	projectID := ctx.Param("id")

	// Resolve tenant
	tenant, err := tenantManager.GetTenantFromRequest(ctx.Context(), ctx.Request())
	if err != nil {
		return nil, fmt.Errorf("tenant resolution failed: %w", err)
	}

	// Check authorization
	identity := getCurrentIdentity(ctx)
	if allowed, err := ctx.Authz().Authorize(ctx.Context(), identity, "read", "projects"); err != nil || !allowed {
		return nil, fmt.Errorf("forbidden: insufficient permissions")
	}

	// Get project (in real app, from database)
	project := getProjectByID(projectID, tenant.ID)
	if project == nil {
		return nil, fmt.Errorf("project not found")
	}

	return &ProjectResponse{Project: *project}, nil
}

func UpdateProjectV1(ctx *appContext.Context, req UpdateProjectRequest) (*ProjectResponse, error) {
	projectID := ctx.Param("id")

	// Resolve tenant
	tenant, err := tenantManager.GetTenantFromRequest(ctx.Context(), ctx.Request())
	if err != nil {
		return nil, fmt.Errorf("tenant resolution failed: %w", err)
	}

	// Check authorization
	identity := getCurrentIdentity(ctx)
	if allowed, err := ctx.Authz().Authorize(ctx.Context(), identity, "update", "projects"); err != nil || !allowed {
		return nil, fmt.Errorf("forbidden: insufficient permissions")
	}

	// Get and update project
	project := getProjectByID(projectID, tenant.ID)
	if project == nil {
		return nil, fmt.Errorf("project not found")
	}

	if req.Name != "" {
		project.Name = req.Name
	}
	if req.Description != "" {
		project.Description = req.Description
	}
	if req.Status != "" {
		project.Status = req.Status
	}
	project.UpdatedAt = time.Now()

	// In real app, save to database
	log.Printf("Updated project: %+v", project)

	return &ProjectResponse{Project: *project}, nil
}

func DeleteProjectV1(ctx *appContext.Context, req struct{}) error {
	projectID := ctx.Param("id")

	// Resolve tenant
	tenant, err := tenantManager.GetTenantFromRequest(ctx.Context(), ctx.Request())
	if err != nil {
		return fmt.Errorf("tenant resolution failed: %w", err)
	}

	// Check authorization
	identity := getCurrentIdentity(ctx)
	if allowed, err := ctx.Authz().Authorize(ctx.Context(), identity, "delete", "projects"); err != nil || !allowed {
		return fmt.Errorf("forbidden: insufficient permissions")
	}

	// Verify project ownership or admin role
	project := getProjectByID(projectID, tenant.ID)
	if project == nil {
		return fmt.Errorf("project not found")
	}

	if project.OwnerID != identity.ID {
		if allowed, _ := ctx.Authz().Authorize(ctx.Context(), identity, "delete", "*"); !allowed {
			return fmt.Errorf("forbidden: can only delete own projects")
		}
	}

	// In real app, delete from database
	log.Printf("Deleted project: %s", projectID)

	return nil
}

// ============================================================================
// V2 Handlers (Enhanced Features)
// ============================================================================

func ListProjectsV2(ctx *appContext.Context, req ListProjectsRequest) (*pagination.CursorResult, error) {
	// V2 uses cursor-based pagination for better performance
	tenant, err := tenantManager.GetTenantFromRequest(ctx.Context(), ctx.Request())
	if err != nil {
		return nil, fmt.Errorf("tenant resolution failed: %w", err)
	}

	identity := getCurrentIdentity(ctx)
	if allowed, err := ctx.Authz().Authorize(ctx.Context(), identity, "read", "projects"); err != nil || !allowed {
		return nil, fmt.Errorf("forbidden: insufficient permissions")
	}

	// Parse cursor pagination parameters
	limit := req.Limit
	if limit == 0 {
		limit = configManager.GetInt("pagination.default_limit")
	}
	if limit > configManager.GetInt("pagination.max_limit") {
		limit = configManager.GetInt("pagination.max_limit")
	}

	params := pagination.ParseCursorParams(req.Cursor, limit, req.Sort, req.Order)

	// In real app, query from database with cursor
	projects := getProjectsByTenant(tenant.ID)

	return pagination.NewCursorResult(projects, params), nil
}

func BatchUpdateProjectsV2(ctx *appContext.Context, req struct {
	ProjectIDs []string               `json:"project_ids" validate:"required,min=1,max=100"`
	Updates    map[string]interface{} `json:"updates" validate:"required"`
}) (map[string]interface{}, error) {
	tenant, err := tenantManager.GetTenantFromRequest(ctx.Context(), ctx.Request())
	if err != nil {
		return nil, fmt.Errorf("tenant resolution failed: %w", err)
	}

	identity := getCurrentIdentity(ctx)
	if allowed, err := ctx.Authz().Authorize(ctx.Context(), identity, "update", "projects"); err != nil || !allowed {
		return nil, fmt.Errorf("forbidden: insufficient permissions")
	}

	// Batch update projects
	updated := 0
	for _, projectID := range req.ProjectIDs {
		project := getProjectByID(projectID, tenant.ID)
		if project != nil {
			// Apply updates
			if name, ok := req.Updates["name"].(string); ok {
				project.Name = name
			}
			if desc, ok := req.Updates["description"].(string); ok {
				project.Description = desc
			}
			if status, ok := req.Updates["status"].(string); ok {
				project.Status = status
			}
			project.UpdatedAt = time.Now()
			updated++
		}
	}

	return map[string]interface{}{
		"updated": updated,
		"total":   len(req.ProjectIDs),
	}, nil
}

// ============================================================================
// Helper Functions (Mock Data)
// ============================================================================

func getCurrentIdentity(ctx *appContext.Context) *contracts.Identity {
	// In real app, extract from JWT token in Authorization header
	return &contracts.Identity{
		ID: "user-dev",
		Claims: map[string]interface{}{
			"email": "dev@acme.com",
			"name":  "Developer User",
		},
	}
}

func getProjectsByTenant(tenantID string) []Project {
	// Mock data
	return []Project{
		{
			ID:          "proj-1",
			Name:        "Project Alpha",
			Description: "First project",
			TenantID:    tenantID,
			OwnerID:     "user-dev",
			Status:      "active",
			CreatedAt:   time.Now().Add(-48 * time.Hour),
			UpdatedAt:   time.Now().Add(-24 * time.Hour),
		},
		{
			ID:          "proj-2",
			Name:        "Project Beta",
			Description: "Second project",
			TenantID:    tenantID,
			OwnerID:     "user-pm",
			Status:      "active",
			CreatedAt:   time.Now().Add(-72 * time.Hour),
			UpdatedAt:   time.Now().Add(-48 * time.Hour),
		},
		{
			ID:          "proj-3",
			Name:        "Project Gamma",
			Description: "Third project",
			TenantID:    tenantID,
			OwnerID:     "user-dev",
			Status:      "inactive",
			CreatedAt:   time.Now().Add(-96 * time.Hour),
			UpdatedAt:   time.Now().Add(-72 * time.Hour),
		},
	}
}

func getProjectByID(id string, tenantID string) *Project {
	projects := getProjectsByTenant(tenantID)
	for _, p := range projects {
		if p.ID == id {
			return &p
		}
	}
	return nil
}
