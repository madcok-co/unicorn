package contracts

import "context"

// Database adalah generic interface untuk semua database operations
// User tidak perlu tahu apakah ini PostgreSQL, MySQL, MongoDB, dll
type Database interface {
	// Basic CRUD
	Create(ctx context.Context, entity any) error
	FindByID(ctx context.Context, id any, dest any) error
	FindOne(ctx context.Context, dest any, query string, args ...any) error
	FindAll(ctx context.Context, dest any, query string, args ...any) error
	Update(ctx context.Context, entity any) error
	Delete(ctx context.Context, entity any) error

	// Query builder
	Query() QueryBuilder

	// Transaction
	Transaction(ctx context.Context, fn func(tx Database) error) error

	// Raw query (escape hatch)
	Raw(ctx context.Context, query string, args ...any) (Result, error)
	Exec(ctx context.Context, query string, args ...any) (ExecResult, error)

	// Connection management
	Ping(ctx context.Context) error
	Close() error
}

// QueryBuilder untuk building queries secara fluent
type QueryBuilder interface {
	Select(columns ...string) QueryBuilder
	From(table string) QueryBuilder
	Where(condition string, args ...any) QueryBuilder
	WhereIn(column string, values ...any) QueryBuilder
	OrderBy(column string, direction string) QueryBuilder
	Limit(limit int) QueryBuilder
	Offset(offset int) QueryBuilder
	Join(table string, condition string) QueryBuilder
	LeftJoin(table string, condition string) QueryBuilder
	GroupBy(columns ...string) QueryBuilder
	Having(condition string, args ...any) QueryBuilder

	// Execute
	Get(ctx context.Context, dest any) error
	First(ctx context.Context, dest any) error
	Count(ctx context.Context) (int64, error)
	Exists(ctx context.Context) (bool, error)
}

// Result untuk hasil query
type Result interface {
	Scan(dest ...any) error
	Next() bool
	Close() error
}

// ExecResult untuk hasil exec
type ExecResult interface {
	LastInsertId() (int64, error)
	RowsAffected() (int64, error)
}

// DatabaseConfig untuk konfigurasi database
type DatabaseConfig struct {
	Driver   string // postgres, mysql, mongodb, sqlite
	Host     string
	Port     int
	Username string
	Password string
	Database string
	SSLMode  string

	// Connection pool
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime int // in seconds

	// Additional options
	Options map[string]string
}
