# Pagination Helpers

Production-ready pagination utilities for Unicorn Framework supporting offset-based and cursor-based pagination.

## Features

- **Offset-Based Pagination** - Traditional page/limit pagination
- **Cursor-Based Pagination** - Efficient for large datasets
- **SQL Injection Protection** - Sanitized field names
- **HATEOAS Links** - Automatic pagination link generation
- **Type-Safe Parameters** - Validated pagination params
- **Flexible Limits** - Configurable max limits
- **Sort Support** - Multiple sort fields with ASC/DESC

## Installation

```bash
go get github.com/madcok-co/unicorn/contrib/pagination
```

## Offset-Based Pagination

Traditional page/limit pagination - best for small to medium datasets with known total count.

### Quick Start

```go
import (
    "github.com/madcok-co/unicorn/contrib/pagination"
    "github.com/madcok-co/unicorn/core/pkg/context"
)

type ListUsersRequest struct {
    Page  int    `query:"page"`
    Limit int    `query:"limit"`
    Sort  string `query:"sort"`
    Order string `query:"order"`
}

func ListUsers(ctx *context.Context, req ListUsersRequest) (*pagination.OffsetResult, error) {
    // Parse pagination params
    params := pagination.ParseOffsetParams(req.Page, req.Limit, req.Sort, req.Order)
    
    // Validate params
    if err := params.Validate(); err != nil {
        return nil, err
    }
    
    // Query with pagination
    var users []User
    query := ctx.DB().
        Offset(params.Offset()).
        Limit(params.Limit)
    
    // Add sorting
    if params.Sort != "" {
        orderBy := pagination.BuildOrderByClause(params.Sort, params.Order)
        query = query.Order(orderBy)
    }
    
    // Execute query
    if err := query.Find(ctx.Context(), &users); err != nil {
        return nil, err
    }
    
    // Get total count
    var total int64
    ctx.DB().Count(ctx.Context(), &User{}, &total)
    
    // Build result
    result := pagination.NewOffsetResult(users, total, params)
    
    return result, nil
}
```

### Response Format

```json
{
  "data": [
    {"id": "1", "name": "Alice"},
    {"id": "2", "name": "Bob"}
  ],
  "total": 100,
  "page": 2,
  "limit": 20,
  "total_pages": 5,
  "has_previous": true,
  "has_next": true
}
```

### With HATEOAS Links

```go
func ListUsersWithLinks(ctx *context.Context, req ListUsersRequest) (map[string]interface{}, error) {
    params := pagination.ParseOffsetParams(req.Page, req.Limit, req.Sort, req.Order)
    
    var users []User
    ctx.DB().
        Offset(params.Offset()).
        Limit(params.Limit).
        Find(ctx.Context(), &users)
    
    var total int64
    ctx.DB().Count(ctx.Context(), &User{}, &total)
    
    result := pagination.NewOffsetResult(users, total, params)
    
    // Build links
    baseURL := "https://api.example.com/users"
    links := pagination.BuildLinks(baseURL, params.Page, result.TotalPages)
    
    return map[string]interface{}{
        "data":  result.Data,
        "meta":  result,
        "links": links,
    }, nil
}
```

**Response with links:**
```json
{
  "data": [...],
  "meta": {
    "total": 100,
    "page": 2,
    "limit": 20,
    "total_pages": 5,
    "has_previous": true,
    "has_next": true
  },
  "links": {
    "self": "https://api.example.com/users?page=2",
    "first": "https://api.example.com/users?page=1",
    "prev": "https://api.example.com/users?page=1",
    "next": "https://api.example.com/users?page=3",
    "last": "https://api.example.com/users?page=5"
  }
}
```

## Cursor-Based Pagination

Cursor pagination - best for large datasets, real-time data, or infinite scroll.

### Quick Start

```go
type ListPostsRequest struct {
    Cursor string `query:"cursor"`
    Limit  int    `query:"limit"`
    Sort   string `query:"sort"`
    Order  string `query:"order"`
}

func ListPosts(ctx *context.Context, req ListPostsRequest) (*pagination.CursorResult, error) {
    // Parse cursor params
    params := pagination.ParseCursorParams(req.Cursor, req.Limit, req.Sort, req.Order)
    
    if err := params.Validate(); err != nil {
        return nil, err
    }
    
    var posts []Post
    query := ctx.DB().Limit(params.Limit + 1) // Fetch one extra to check hasNext
    
    // Decode cursor if present
    if params.Cursor != "" {
        lastID, err := pagination.DecodeCursor(params.Cursor)
        if err != nil {
            return nil, err
        }
        
        // Filter by cursor
        if params.Order == "asc" {
            query = query.Where("id > ?", lastID)
        } else {
            query = query.Where("id < ?", lastID)
        }
    }
    
    // Add sorting
    if params.Sort != "" {
        orderBy := pagination.BuildOrderByClause(params.Sort, params.Order)
        query = query.Order(orderBy)
    }
    
    // Execute query
    if err := query.Find(ctx.Context(), &posts); err != nil {
        return nil, err
    }
    
    // Check if there are more results
    hasNext := len(posts) > params.Limit
    if hasNext {
        posts = posts[:params.Limit] // Remove extra item
    }
    
    // Build next cursor
    var nextCursor string
    if hasNext && len(posts) > 0 {
        lastPost := posts[len(posts)-1]
        nextCursor = pagination.EncodeCursor(lastPost.ID)
    }
    
    // Build result
    result := pagination.NewCursorResult(posts, nextCursor, "", hasNext, false)
    
    return result, nil
}
```

### Response Format

```json
{
  "data": [
    {"id": "10", "title": "Post 10"},
    {"id": "11", "title": "Post 11"}
  ],
  "next_cursor": "MTI=",
  "has_next": true,
  "has_prev": false
}
```

## Configuration

### Default Parameters

```go
// Offset defaults
params := pagination.DefaultOffsetParams()
// Page: 1
// Limit: 20
// MaxLimit: 100
// Order: "asc"

// Cursor defaults
params := pagination.DefaultCursorParams()
// Limit: 20
// MaxLimit: 100
// Order: "asc"
```

### Custom Parameters

```go
params := &pagination.OffsetParams{
    Page:     1,
    Limit:    50,
    Sort:     "created_at",
    Order:    "desc",
    MaxLimit: 200,
}
```

## Query Parameter Parsing

### From HTTP Request

```go
type PaginationRequest struct {
    Page  int    `query:"page"`
    Limit int    `query:"limit"`
    Sort  string `query:"sort"`
    Order string `query:"order"`
}

func ListItems(ctx *context.Context, req PaginationRequest) (*pagination.OffsetResult, error) {
    params := pagination.ParseOffsetParams(req.Page, req.Limit, req.Sort, req.Order)
    // ... use params
}
```

### From Query Strings

```go
// Using utility functions
page := pagination.ParsePageFromQuery(req.Query("page"))
limit := pagination.ParseLimitFromQuery(req.Query("limit"), 20, 100)
```

## Sorting

### Single Field

```go
params := &pagination.OffsetParams{
    Sort:  "created_at",
    Order: "desc",
}

orderBy := pagination.BuildOrderByClause(params.Sort, params.Order)
// Result: "created_at DESC"

query.Order(orderBy)
```

### Multiple Fields

```go
// Manual approach for multiple fields
query.Order("status ASC, created_at DESC")
```

### SQL Injection Protection

Field names are automatically sanitized:

```go
// Safe
pagination.BuildOrderByClause("user_id", "asc")
// Result: "user_id ASC"

// Sanitized (SQL injection attempt)
pagination.BuildOrderByClause("user_id; DROP TABLE", "asc")
// Result: "user_idDROPTABLE ASC" (invalid but safe)
```

## Complete Examples

### REST API with Offset Pagination

```go
package main

import (
    "github.com/madcok-co/unicorn/contrib/pagination"
    "github.com/madcok-co/unicorn/core/pkg/app"
    "github.com/madcok-co/unicorn/core/pkg/context"
)

type User struct {
    ID        string `json:"id"`
    Name      string `json:"name"`
    Email     string `json:"email"`
    CreatedAt string `json:"created_at"`
}

type ListUsersRequest struct {
    Page  int    `query:"page"`
    Limit int    `query:"limit"`
    Sort  string `query:"sort"`
    Order string `query:"order"`
}

func ListUsers(ctx *context.Context, req ListUsersRequest) (*pagination.OffsetResult, error) {
    // Parse and validate params
    params := pagination.ParseOffsetParams(req.Page, req.Limit, req.Sort, req.Order)
    if err := params.Validate(); err != nil {
        return nil, err
    }
    
    // Query database
    var users []User
    query := ctx.DB().
        Offset(params.Offset()).
        Limit(params.Limit)
    
    if params.Sort != "" {
        query = query.Order(pagination.BuildOrderByClause(params.Sort, params.Order))
    }
    
    if err := query.Find(ctx.Context(), &users); err != nil {
        return nil, err
    }
    
    // Get total count
    var total int64
    ctx.DB().Count(ctx.Context(), &User{}, &total)
    
    // Return paginated result
    return pagination.NewOffsetResult(users, total, params), nil
}

func main() {
    application := app.New(&app.Config{
        Name:       "pagination-example",
        EnableHTTP: true,
    })
    
    application.RegisterHandler(ListUsers).
        HTTP("GET", "/users").
        Done()
    
    application.Start()
}
```

**Request:**
```
GET /users?page=2&limit=50&sort=created_at&order=desc
```

**Response:**
```json
{
  "data": [...],
  "total": 1000,
  "page": 2,
  "limit": 50,
  "total_pages": 20,
  "has_previous": true,
  "has_next": true
}
```

### Cursor Pagination with Timestamps

```go
type Post struct {
    ID        string    `json:"id"`
    Title     string    `json:"title"`
    CreatedAt time.Time `json:"created_at"`
}

type ListPostsRequest struct {
    Cursor string `query:"cursor"`
    Limit  int    `query:"limit"`
}

func ListPosts(ctx *context.Context, req ListPostsRequest) (*pagination.CursorResult, error) {
    params := pagination.ParseCursorParams(req.Cursor, req.Limit, "created_at", "desc")
    
    var posts []Post
    query := ctx.DB().
        Order("created_at DESC").
        Limit(params.Limit + 1)
    
    // Apply cursor filter
    if params.Cursor != "" {
        lastTimestamp, err := pagination.DecodeCursor(params.Cursor)
        if err != nil {
            return nil, err
        }
        query = query.Where("created_at < ?", lastTimestamp)
    }
    
    if err := query.Find(ctx.Context(), &posts); err != nil {
        return nil, err
    }
    
    hasNext := len(posts) > params.Limit
    if hasNext {
        posts = posts[:params.Limit]
    }
    
    var nextCursor string
    if hasNext {
        lastPost := posts[len(posts)-1]
        nextCursor = pagination.EncodeCursor(lastPost.CreatedAt.Format(time.RFC3339))
    }
    
    return pagination.NewCursorResult(posts, nextCursor, "", hasNext, false), nil
}
```

### Bidirectional Cursor Pagination

```go
func ListItemsBidirectional(ctx *context.Context, req ListItemsRequest) (*pagination.CursorResult, error) {
    params := pagination.ParseCursorParams(req.Cursor, req.Limit, "id", "asc")
    
    var items []Item
    query := ctx.DB().Limit(params.Limit + 2) // +2 for hasNext and hasPrev
    
    var afterID, beforeID string
    if params.Cursor != "" {
        cursorID, _ := pagination.DecodeCursor(params.Cursor)
        afterID = cursorID
        query = query.Where("id > ?", afterID)
    }
    
    query = query.Order("id ASC")
    query.Find(ctx.Context(), &items)
    
    // Determine hasNext and hasPrev
    hasNext := len(items) > params.Limit
    hasPrev := afterID != ""
    
    if hasNext {
        items = items[:params.Limit]
    }
    
    var nextCursor, prevCursor string
    if hasNext && len(items) > 0 {
        nextCursor = pagination.EncodeCursor(items[len(items)-1].ID)
    }
    if hasPrev && len(items) > 0 {
        prevCursor = pagination.EncodeCursor(items[0].ID)
    }
    
    return pagination.NewCursorResult(items, nextCursor, prevCursor, hasNext, hasPrev), nil
}
```

## Best Practices

### 1. Always Validate Parameters

```go
params := pagination.ParseOffsetParams(req.Page, req.Limit, req.Sort, req.Order)
if err := params.Validate(); err != nil {
    return nil, fmt.Errorf("invalid pagination params: %w", err)
}
```

### 2. Set Reasonable Max Limits

```go
params := &pagination.OffsetParams{
    MaxLimit: 100, // Prevent excessive data retrieval
}
```

### 3. Use Indexes for Sorted Fields

```sql
-- If sorting by created_at
CREATE INDEX idx_users_created_at ON users(created_at DESC);

-- If using cursor on id
CREATE INDEX idx_posts_id ON posts(id);
```

### 4. Choose Right Pagination Type

**Use Offset Pagination when:**
- Dataset is small to medium (< 10,000 records)
- Need total count
- Need to jump to specific pages
- Building traditional web UI

**Use Cursor Pagination when:**
- Dataset is large (> 10,000 records)
- Building infinite scroll
- Real-time data (new items added frequently)
- Performance is critical

### 5. Cache Total Count (Offset Pagination)

```go
// Cache total count for better performance
cacheKey := "users:total_count"
total, err := ctx.Cache().Get(ctx.Context(), cacheKey)
if err != nil {
    ctx.DB().Count(ctx.Context(), &User{}, &total)
    ctx.Cache().Set(ctx.Context(), cacheKey, total, 5*time.Minute)
}
```

## Cursor Encoding

### Custom Cursor Encoder

```go
type CustomCursorEncoder struct{}

func (e *CustomCursorEncoder) Encode(value interface{}) string {
    // Custom encoding logic
    return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%v", value)))
}

func (e *CustomCursorEncoder) Decode(cursor string) (interface{}, error) {
    decoded, err := base64.StdEncoding.DecodeString(cursor)
    return string(decoded), err
}

// Use custom encoder
encoder := &CustomCursorEncoder{}
cursor := encoder.Encode(lastID)
```

## Testing

```bash
# Run tests
cd contrib/pagination
go test -v

# Run with coverage
go test -v -cover
```

## Common Patterns

### Search with Pagination

```go
func SearchUsers(ctx *context.Context, req SearchUsersRequest) (*pagination.OffsetResult, error) {
    params := pagination.ParseOffsetParams(req.Page, req.Limit, req.Sort, req.Order)
    
    var users []User
    query := ctx.DB()
    
    // Apply search filter
    if req.Query != "" {
        query = query.Where("name LIKE ? OR email LIKE ?", 
            "%"+req.Query+"%", "%"+req.Query+"%")
    }
    
    // Apply pagination
    query.Offset(params.Offset()).Limit(params.Limit)
    
    if params.Sort != "" {
        query = query.Order(pagination.BuildOrderByClause(params.Sort, params.Order))
    }
    
    query.Find(ctx.Context(), &users)
    
    // Count with same filter
    var total int64
    ctx.DB().
        Where("name LIKE ? OR email LIKE ?", "%"+req.Query+"%", "%"+req.Query+"%").
        Count(ctx.Context(), &User{}, &total)
    
    return pagination.NewOffsetResult(users, total, params), nil
}
```

### Filter + Pagination

```go
func ListUsersByStatus(ctx *context.Context, req ListByStatusRequest) (*pagination.OffsetResult, error) {
    params := pagination.ParseOffsetParams(req.Page, req.Limit, req.Sort, req.Order)
    
    var users []User
    ctx.DB().
        Where("status = ?", req.Status).
        Offset(params.Offset()).
        Limit(params.Limit).
        Order(pagination.BuildOrderByClause(params.Sort, params.Order)).
        Find(ctx.Context(), &users)
    
    var total int64
    ctx.DB().Where("status = ?", req.Status).Count(ctx.Context(), &User{}, &total)
    
    return pagination.NewOffsetResult(users, total, params), nil
}
```

## Performance Tips

1. **Index Pagination Columns** - Always index sort columns
2. **Avoid COUNT(*) on Large Tables** - Use estimates or cache
3. **Use Cursor for Large Datasets** - More efficient than offset
4. **Limit Max Page Size** - Prevent resource exhaustion
5. **Consider Read Replicas** - For read-heavy workloads

## License

MIT License - see LICENSE file for details
