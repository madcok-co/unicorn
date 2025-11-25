package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/madcok-co/unicorn/core/pkg/app"
	ucontext "github.com/madcok-co/unicorn/core/pkg/context"
	"github.com/madcok-co/unicorn/core/pkg/contracts"

	loggerAdapter "github.com/madcok-co/unicorn/core/pkg/adapters/logger"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ============================================================
// DATABASE MODELS
// ============================================================

type User struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	Username  string    `gorm:"uniqueIndex;not null" json:"username"`
	Email     string    `gorm:"uniqueIndex;not null" json:"email"`
	Age       int       `json:"age"`
	Active    bool      `gorm:"default:true" json:"active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Posts     []Post    `gorm:"foreignKey:UserID" json:"posts,omitempty"`
}

type Post struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	Title     string    `gorm:"not null" json:"title"`
	Content   string    `gorm:"type:text" json:"content"`
	Published bool      `gorm:"default:false" json:"published"`
	ViewCount int       `gorm:"default:0" json:"view_count"`
	UserID    uint      `gorm:"not null" json:"user_id"`
	User      User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Tags      []Tag     `gorm:"many2many:post_tags;" json:"tags,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Tag struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	Name      string    `gorm:"uniqueIndex;not null" json:"name"`
	CreatedAt time.Time `json:"created_at"`
	Posts     []Post    `gorm:"many2many:post_tags;" json:"posts,omitempty"`
}

// ============================================================
// REQUEST DTOs
// ============================================================

type CreateUserRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Age      int    `json:"age"`
}

type UpdateUserRequest struct {
	Username string `json:"username,omitempty"`
	Email    string `json:"email,omitempty"`
	Age      int    `json:"age,omitempty"`
	Active   *bool  `json:"active,omitempty"`
}

type CreatePostRequest struct {
	Title   string   `json:"title"`
	Content string   `json:"content"`
	UserID  uint     `json:"user_id"`
	Tags    []string `json:"tags,omitempty"`
}

type UpdatePostRequest struct {
	Title     string `json:"title,omitempty"`
	Content   string `json:"content,omitempty"`
	Published *bool  `json:"published,omitempty"`
}

// ============================================================
// HANDLERS - BASIC CRUD
// ============================================================

// CreateUser demonstrates INSERT operation
func CreateUser(ctx *ucontext.Context, req CreateUserRequest) (*User, error) {
	db := ctx.DB()
	logger := ctx.Logger()

	user := &User{
		Username: req.Username,
		Email:    req.Email,
		Age:      req.Age,
		Active:   true,
	}

	if err := db.Create(ctx.Context(), user); err != nil {
		logger.Error("failed to create user", "error", err)
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	logger.Info("user created", "id", user.ID, "username", user.Username)
	return user, nil
}

// GetUser demonstrates SELECT by ID
func GetUser(ctx *ucontext.Context) (*User, error) {
	userID := ctx.Request().Params["id"]

	adapter := ctx.DB().(*GORMAdapter)
	var user User

	if err := adapter.db.First(&user, userID).Error; err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	return &user, nil
}

// GetUserWithPosts demonstrates JOIN / Preload (1-to-Many)
func GetUserWithPosts(ctx *ucontext.Context) (*User, error) {
	userID := ctx.Request().Params["id"]

	adapter := ctx.DB().(*GORMAdapter)
	var user User

	// Preload related data
	if err := adapter.db.Preload("Posts").First(&user, userID).Error; err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	return &user, nil
}

// ListUsers demonstrates SELECT with WHERE, ORDER, LIMIT
func ListUsers(ctx *ucontext.Context) ([]User, error) {
	page := 1
	perPage := 10

	if p := ctx.Request().Query["page"]; p != "" {
		fmt.Sscanf(p, "%d", &page)
	}
	if pp := ctx.Request().Query["per_page"]; pp != "" {
		fmt.Sscanf(pp, "%d", &perPage)
	}

	adapter := ctx.DB().(*GORMAdapter)
	var users []User

	offset := (page - 1) * perPage

	// Complex query with conditions
	query := adapter.db.Where("active = ?", true)

	// Optional filter by age
	if minAge := ctx.Request().Query["min_age"]; minAge != "" {
		var age int
		fmt.Sscanf(minAge, "%d", &age)
		query = query.Where("age >= ?", age)
	}

	if err := query.Order("created_at DESC").
		Limit(perPage).
		Offset(offset).
		Find(&users).Error; err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	return users, nil
}

// UpdateUser demonstrates UPDATE operation
func UpdateUser(ctx *ucontext.Context, req UpdateUserRequest) (*User, error) {
	logger := ctx.Logger()
	userID := ctx.Request().Params["id"]

	adapter := ctx.DB().(*GORMAdapter)
	var user User

	if err := adapter.db.First(&user, userID).Error; err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	// Update only provided fields
	updates := make(map[string]interface{})
	if req.Username != "" {
		updates["username"] = req.Username
	}
	if req.Email != "" {
		updates["email"] = req.Email
	}
	if req.Age > 0 {
		updates["age"] = req.Age
	}
	if req.Active != nil {
		updates["active"] = *req.Active
	}

	if err := adapter.db.Model(&user).Updates(updates).Error; err != nil {
		logger.Error("failed to update user", "error", err)
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	// Reload to get updated data
	adapter.db.First(&user, userID)

	logger.Info("user updated", "id", user.ID)
	return &user, nil
}

// DeleteUser demonstrates DELETE operation (soft delete)
func DeleteUser(ctx *ucontext.Context) (map[string]interface{}, error) {
	logger := ctx.Logger()
	userID := ctx.Request().Params["id"]

	adapter := ctx.DB().(*GORMAdapter)

	// Delete will be soft delete if model has DeletedAt field
	if err := adapter.db.Delete(&User{}, userID).Error; err != nil {
		logger.Error("failed to delete user", "error", err)
		return nil, fmt.Errorf("failed to delete user: %w", err)
	}

	logger.Info("user deleted", "id", userID)
	return map[string]interface{}{
		"message": "User deleted successfully",
		"id":      userID,
	}, nil
}

// ============================================================
// HANDLERS - ADVANCED FEATURES
// ============================================================

// CreatePostWithTransaction demonstrates TRANSACTION with multiple operations
func CreatePostWithTransaction(ctx *ucontext.Context, req CreatePostRequest) (*Post, error) {
	db := ctx.DB()
	logger := ctx.Logger()

	var post *Post

	// Use transaction for atomic operations
	err := db.Transaction(ctx.Context(), func(tx contracts.Database) error {
		txAdapter := tx.(*GORMAdapter)

		// 1. Check if user exists
		var user User
		if err := txAdapter.db.First(&user, req.UserID).Error; err != nil {
			return fmt.Errorf("user not found: %w", err)
		}

		// 2. Create post
		post = &Post{
			Title:     req.Title,
			Content:   req.Content,
			UserID:    req.UserID,
			Published: false,
		}

		if err := txAdapter.db.Create(post).Error; err != nil {
			return fmt.Errorf("failed to create post: %w", err)
		}

		// 3. Handle tags (Many-to-Many relationship)
		if len(req.Tags) > 0 {
			for _, tagName := range req.Tags {
				var tag Tag
				// Find or create tag
				if err := txAdapter.db.Where("name = ?", tagName).FirstOrCreate(&tag, Tag{Name: tagName}).Error; err != nil {
					return fmt.Errorf("failed to handle tag: %w", err)
				}

				// Associate tag with post
				if err := txAdapter.db.Model(post).Association("Tags").Append(&tag); err != nil {
					return fmt.Errorf("failed to associate tag: %w", err)
				}
			}
		}

		logger.Info("post created in transaction", "id", post.ID, "user_id", req.UserID)
		return nil
	})

	if err != nil {
		logger.Error("transaction failed", "error", err)
		return nil, err
	}

	// Reload with associations
	adapter := db.(*GORMAdapter)
	adapter.db.Preload("Tags").First(post, post.ID)

	return post, nil
}

// SearchPosts demonstrates complex WHERE queries with LIKE
func SearchPosts(ctx *ucontext.Context) ([]Post, error) {
	keyword := ctx.Request().Query["q"]
	publishedOnly := ctx.Request().Query["published"] == "true"

	adapter := ctx.DB().(*GORMAdapter)
	var posts []Post

	query := adapter.db.Model(&Post{})

	// Search in title or content
	if keyword != "" {
		query = query.Where("title LIKE ? OR content LIKE ?",
			"%"+keyword+"%", "%"+keyword+"%")
	}

	// Filter by published status
	if publishedOnly {
		query = query.Where("published = ?", true)
	}

	// Order and limit
	if err := query.Order("created_at DESC").
		Limit(20).
		Preload("User").
		Preload("Tags").
		Find(&posts).Error; err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	return posts, nil
}

// IncrementViewCount demonstrates UPDATE with expression
func IncrementViewCount(ctx *ucontext.Context) (map[string]interface{}, error) {
	postID := ctx.Request().Params["id"]

	adapter := ctx.DB().(*GORMAdapter)

	// Increment view count atomically
	if err := adapter.db.Model(&Post{}).
		Where("id = ?", postID).
		Update("view_count", gorm.Expr("view_count + ?", 1)).Error; err != nil {
		return nil, fmt.Errorf("failed to increment view count: %w", err)
	}

	return map[string]interface{}{
		"message": "View count incremented",
	}, nil
}

// GetStats demonstrates AGGREGATION queries
func GetStats(ctx *ucontext.Context) (map[string]interface{}, error) {
	adapter := ctx.DB().(*GORMAdapter)

	var stats struct {
		TotalUsers       int64
		ActiveUsers      int64
		TotalPosts       int64
		PublishedPosts   int64
		TotalViews       int64
		AvgPostsPerUser  float64
		MostActiveUserID uint
		MostActiveCount  int64
	}

	// Count queries
	adapter.db.Model(&User{}).Count(&stats.TotalUsers)
	adapter.db.Model(&User{}).Where("active = ?", true).Count(&stats.ActiveUsers)
	adapter.db.Model(&Post{}).Count(&stats.TotalPosts)
	adapter.db.Model(&Post{}).Where("published = ?", true).Count(&stats.PublishedPosts)

	// Sum aggregation
	adapter.db.Model(&Post{}).Select("COALESCE(SUM(view_count), 0)").Scan(&stats.TotalViews)

	// Average calculation
	if stats.TotalUsers > 0 {
		stats.AvgPostsPerUser = float64(stats.TotalPosts) / float64(stats.TotalUsers)
	}

	// Complex aggregation: most active user
	type UserPostCount struct {
		UserID uint
		Count  int64
	}
	var mostActive UserPostCount
	adapter.db.Model(&Post{}).
		Select("user_id, count(*) as count").
		Group("user_id").
		Order("count DESC").
		Limit(1).
		Scan(&mostActive)

	stats.MostActiveUserID = mostActive.UserID
	stats.MostActiveCount = mostActive.Count

	return map[string]interface{}{
		"total_users":         stats.TotalUsers,
		"active_users":        stats.ActiveUsers,
		"total_posts":         stats.TotalPosts,
		"published_posts":     stats.PublishedPosts,
		"total_views":         stats.TotalViews,
		"avg_posts_per_user":  stats.AvgPostsPerUser,
		"most_active_user_id": stats.MostActiveUserID,
		"most_active_posts":   stats.MostActiveCount,
	}, nil
}

// GetPostsByTag demonstrates Many-to-Many query
func GetPostsByTag(ctx *ucontext.Context) ([]Post, error) {
	tagName := ctx.Request().Params["tag"]

	adapter := ctx.DB().(*GORMAdapter)
	var posts []Post

	// Query through many-to-many relationship
	if err := adapter.db.
		Joins("JOIN post_tags ON post_tags.post_id = posts.id").
		Joins("JOIN tags ON tags.id = post_tags.tag_id").
		Where("tags.name = ?", tagName).
		Preload("User").
		Preload("Tags").
		Find(&posts).Error; err != nil {
		return nil, fmt.Errorf("failed to get posts by tag: %w", err)
	}

	return posts, nil
}

// BulkCreateUsers demonstrates BATCH INSERT
func BulkCreateUsers(ctx *ucontext.Context, users []CreateUserRequest) (map[string]interface{}, error) {
	logger := ctx.Logger()
	adapter := ctx.DB().(*GORMAdapter)

	var userModels []User
	for _, req := range users {
		userModels = append(userModels, User{
			Username: req.Username,
			Email:    req.Email,
			Age:      req.Age,
			Active:   true,
		})
	}

	// Batch insert
	if err := adapter.db.Create(&userModels).Error; err != nil {
		logger.Error("bulk create failed", "error", err)
		return nil, fmt.Errorf("bulk create failed: %w", err)
	}

	logger.Info("bulk create successful", "count", len(userModels))

	return map[string]interface{}{
		"message": "Users created successfully",
		"count":   len(userModels),
		"users":   userModels,
	}, nil
}

// ============================================================
// MAIN
// ============================================================

func main() {
	// Create application
	application := app.New(&app.Config{
		Name:    "database-example",
		Version: "1.0.0",
	})

	// Setup logger
	logger := loggerAdapter.NewConsoleLogger("info")
	application.SetLogger(logger)

	// Setup database with GORM (SQLite)
	gormDB, err := gorm.Open(sqlite.Open("example.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect database:", err)
	}

	// Auto-migrate schema
	if err := gormDB.AutoMigrate(&User{}, &Post{}, &Tag{}); err != nil {
		log.Fatal("Failed to migrate database:", err)
	}

	// Wrap GORM in contracts.Database
	dbAdapter := &GORMAdapter{db: gormDB}
	application.SetDB(dbAdapter)

	logger.Info("✅ Database connected and migrated successfully")

	// Register handlers - Basic CRUD
	application.RegisterHandler(CreateUser).Named("create-user").HTTP("POST", "/users").Done()
	application.RegisterHandler(GetUser).Named("get-user").HTTP("GET", "/users/:id").Done()
	application.RegisterHandler(GetUserWithPosts).Named("get-user-posts").HTTP("GET", "/users/:id/posts").Done()
	application.RegisterHandler(ListUsers).Named("list-users").HTTP("GET", "/users").Done()
	application.RegisterHandler(UpdateUser).Named("update-user").HTTP("PUT", "/users/:id").Done()
	application.RegisterHandler(DeleteUser).Named("delete-user").HTTP("DELETE", "/users/:id").Done()

	// Register handlers - Advanced features
	application.RegisterHandler(CreatePostWithTransaction).Named("create-post").HTTP("POST", "/posts").Done()
	application.RegisterHandler(SearchPosts).Named("search-posts").HTTP("GET", "/posts/search").Done()
	application.RegisterHandler(IncrementViewCount).Named("increment-views").HTTP("POST", "/posts/:id/views").Done()
	application.RegisterHandler(GetStats).Named("get-stats").HTTP("GET", "/stats").Done()
	application.RegisterHandler(GetPostsByTag).Named("posts-by-tag").HTTP("GET", "/tags/:tag/posts").Done()
	application.RegisterHandler(BulkCreateUsers).Named("bulk-users").HTTP("POST", "/users/bulk").Done()

	logger.Info("🗄️  Database Example with GORM Started!")
	logger.Info("📊 Database: SQLite (example.db)")
	logger.Info("")
	logger.Info("📝 Features:")
	logger.Info("  ✓ Basic CRUD operations")
	logger.Info("  ✓ Transactions")
	logger.Info("  ✓ Relationships (1-to-Many, Many-to-Many)")
	logger.Info("  ✓ Complex queries (WHERE, LIKE, JOIN)")
	logger.Info("  ✓ Aggregations (COUNT, SUM, AVG, GROUP BY)")
	logger.Info("  ✓ Batch operations")
	logger.Info("")
	logger.Info("📖 API Endpoints:")
	logger.Info("  POST   /users              - Create user")
	logger.Info("  GET    /users              - List users (pagination)")
	logger.Info("  GET    /users/:id          - Get user by ID")
	logger.Info("  GET    /users/:id/posts    - Get user with posts")
	logger.Info("  PUT    /users/:id          - Update user")
	logger.Info("  DELETE /users/:id          - Delete user")
	logger.Info("  POST   /users/bulk         - Bulk create users")
	logger.Info("  POST   /posts              - Create post (transaction)")
	logger.Info("  GET    /posts/search?q=... - Search posts")
	logger.Info("  POST   /posts/:id/views    - Increment view count")
	logger.Info("  GET    /tags/:tag/posts    - Get posts by tag")
	logger.Info("  GET    /stats              - Get statistics")

	// Start application
	if err := application.Start(); err != nil {
		log.Fatal(err)
	}
}

// ============================================================
// GORM ADAPTER
// ============================================================

type GORMAdapter struct {
	db *gorm.DB
}

func (g *GORMAdapter) Create(ctx context.Context, entity any) error {
	return g.db.WithContext(ctx).Create(entity).Error
}

func (g *GORMAdapter) FindByID(ctx context.Context, id any, dest any) error {
	return g.db.WithContext(ctx).First(dest, id).Error
}

func (g *GORMAdapter) FindOne(ctx context.Context, dest any, query string, args ...any) error {
	return g.db.WithContext(ctx).Where(query, args...).First(dest).Error
}

func (g *GORMAdapter) FindAll(ctx context.Context, dest any, query string, args ...any) error {
	return g.db.WithContext(ctx).Where(query, args...).Find(dest).Error
}

func (g *GORMAdapter) Update(ctx context.Context, entity any) error {
	return g.db.WithContext(ctx).Save(entity).Error
}

func (g *GORMAdapter) Delete(ctx context.Context, entity any) error {
	return g.db.WithContext(ctx).Delete(entity).Error
}

func (g *GORMAdapter) Query() contracts.QueryBuilder {
	return nil
}

func (g *GORMAdapter) Transaction(ctx context.Context, fn func(tx contracts.Database) error) error {
	return g.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&GORMAdapter{db: tx})
	})
}

func (g *GORMAdapter) Raw(ctx context.Context, query string, args ...any) (contracts.Result, error) {
	return nil, fmt.Errorf("not implemented")
}

func (g *GORMAdapter) Exec(ctx context.Context, query string, args ...any) (contracts.ExecResult, error) {
	result := g.db.WithContext(ctx).Exec(query, args...)
	return &gormExecResult{result: result}, result.Error
}

func (g *GORMAdapter) Ping(ctx context.Context) error {
	sqlDB, err := g.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

func (g *GORMAdapter) Close() error {
	sqlDB, err := g.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

type gormExecResult struct {
	result *gorm.DB
}

func (g *gormExecResult) RowsAffected() (int64, error) {
	return g.result.RowsAffected, nil
}

func (g *gormExecResult) LastInsertId() (int64, error) {
	return 0, nil
}
