// Package database provides a generic database adapter
// that wraps any database library (GORM, sqlx, pgx, etc.)
package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

// Driver is the interface that any database driver must implement
// This allows plugging in GORM, sqlx, pgx, ent, etc.
type Driver interface {
	// Connect establishes database connection
	Connect(config *contracts.DatabaseConfig) error
	// Close closes the connection
	Close() error
	// Ping checks connection health
	Ping(ctx context.Context) error
	// DB returns underlying *sql.DB if available
	DB() *sql.DB
}

// CRUD is the interface for basic CRUD operations
type CRUD interface {
	Create(ctx context.Context, entity any) error
	FindByID(ctx context.Context, id any, dest any) error
	FindOne(ctx context.Context, dest any, query string, args ...any) error
	FindAll(ctx context.Context, dest any, query string, args ...any) error
	Update(ctx context.Context, entity any) error
	Delete(ctx context.Context, entity any) error
}

// QueryExecutor is the interface for raw query execution
type QueryExecutor interface {
	Query(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRow(ctx context.Context, query string, args ...any) *sql.Row
	Exec(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// TransactionExecutor is the interface for transaction support
type TransactionExecutor interface {
	BeginTx(ctx context.Context) (Transaction, error)
}

// Transaction represents a database transaction
type Transaction interface {
	CRUD
	QueryExecutor
	Commit() error
	Rollback() error
}

// Adapter implements contracts.Database using pluggable driver
type Adapter struct {
	driver       Driver
	crud         CRUD
	queryExec    QueryExecutor
	txExecutor   TransactionExecutor
	config       *contracts.DatabaseConfig
	queryBuilder func() contracts.QueryBuilder
}

// New creates a new database adapter
func New(driver Driver) *Adapter {
	return &Adapter{
		driver: driver,
	}
}

// WithCRUD sets the CRUD implementation
func (a *Adapter) WithCRUD(crud CRUD) *Adapter {
	a.crud = crud
	return a
}

// WithQueryExecutor sets the query executor
func (a *Adapter) WithQueryExecutor(exec QueryExecutor) *Adapter {
	a.queryExec = exec
	return a
}

// WithTransactionExecutor sets the transaction executor
func (a *Adapter) WithTransactionExecutor(exec TransactionExecutor) *Adapter {
	a.txExecutor = exec
	return a
}

// WithQueryBuilder sets the query builder factory
func (a *Adapter) WithQueryBuilder(factory func() contracts.QueryBuilder) *Adapter {
	a.queryBuilder = factory
	return a
}

// Connect connects to the database
func (a *Adapter) Connect(config *contracts.DatabaseConfig) error {
	a.config = config
	return a.driver.Connect(config)
}

// ============ contracts.Database Implementation ============

// Create creates a new entity
func (a *Adapter) Create(ctx context.Context, entity any) error {
	if a.crud == nil {
		return fmt.Errorf("CRUD not configured")
	}
	return a.crud.Create(ctx, entity)
}

// FindByID finds entity by ID
func (a *Adapter) FindByID(ctx context.Context, id any, dest any) error {
	if a.crud == nil {
		return fmt.Errorf("CRUD not configured")
	}
	return a.crud.FindByID(ctx, id, dest)
}

// FindOne finds single entity
func (a *Adapter) FindOne(ctx context.Context, dest any, query string, args ...any) error {
	if a.crud == nil {
		return fmt.Errorf("CRUD not configured")
	}
	return a.crud.FindOne(ctx, dest, query, args...)
}

// FindAll finds all matching entities
func (a *Adapter) FindAll(ctx context.Context, dest any, query string, args ...any) error {
	if a.crud == nil {
		return fmt.Errorf("CRUD not configured")
	}
	return a.crud.FindAll(ctx, dest, query, args...)
}

// Update updates entity
func (a *Adapter) Update(ctx context.Context, entity any) error {
	if a.crud == nil {
		return fmt.Errorf("CRUD not configured")
	}
	return a.crud.Update(ctx, entity)
}

// Delete deletes entity
func (a *Adapter) Delete(ctx context.Context, entity any) error {
	if a.crud == nil {
		return fmt.Errorf("CRUD not configured")
	}
	return a.crud.Delete(ctx, entity)
}

// Query returns a query builder
func (a *Adapter) Query() contracts.QueryBuilder {
	if a.queryBuilder != nil {
		return a.queryBuilder()
	}
	return NewSimpleQueryBuilder(a)
}

// Transaction executes function in a transaction
func (a *Adapter) Transaction(ctx context.Context, fn func(tx contracts.Database) error) error {
	if a.txExecutor == nil {
		return fmt.Errorf("transaction executor not configured")
	}

	tx, err := a.txExecutor.BeginTx(ctx)
	if err != nil {
		return err
	}

	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback()
			panic(r)
		}
	}()

	// Create a transaction adapter
	txAdapter := &txDatabaseAdapter{tx: tx}

	if err := fn(txAdapter); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}

// Raw executes raw query and returns rows
func (a *Adapter) Raw(ctx context.Context, query string, args ...any) (contracts.Result, error) {
	if a.queryExec == nil {
		return nil, fmt.Errorf("query executor not configured")
	}
	rows, err := a.queryExec.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return &rowsResult{rows: rows}, nil
}

// Exec executes a query without returning rows
func (a *Adapter) Exec(ctx context.Context, query string, args ...any) (contracts.ExecResult, error) {
	if a.queryExec == nil {
		return nil, fmt.Errorf("query executor not configured")
	}
	return a.queryExec.Exec(ctx, query, args...)
}

// Ping checks database connection
func (a *Adapter) Ping(ctx context.Context) error {
	return a.driver.Ping(ctx)
}

// Close closes database connection
func (a *Adapter) Close() error {
	return a.driver.Close()
}

// ============ Helper Types ============

// rowsResult wraps sql.Rows to implement contracts.Result
type rowsResult struct {
	rows *sql.Rows
}

func (r *rowsResult) Scan(dest ...any) error {
	return r.rows.Scan(dest...)
}

func (r *rowsResult) Next() bool {
	return r.rows.Next()
}

func (r *rowsResult) Close() error {
	return r.rows.Close()
}

// txDatabaseAdapter wraps Transaction to implement contracts.Database
type txDatabaseAdapter struct {
	tx Transaction
}

func (t *txDatabaseAdapter) Create(ctx context.Context, entity any) error {
	return t.tx.Create(ctx, entity)
}

func (t *txDatabaseAdapter) FindByID(ctx context.Context, id any, dest any) error {
	return t.tx.FindByID(ctx, id, dest)
}

func (t *txDatabaseAdapter) FindOne(ctx context.Context, dest any, query string, args ...any) error {
	return t.tx.FindOne(ctx, dest, query, args...)
}

func (t *txDatabaseAdapter) FindAll(ctx context.Context, dest any, query string, args ...any) error {
	return t.tx.FindAll(ctx, dest, query, args...)
}

func (t *txDatabaseAdapter) Update(ctx context.Context, entity any) error {
	return t.tx.Update(ctx, entity)
}

func (t *txDatabaseAdapter) Delete(ctx context.Context, entity any) error {
	return t.tx.Delete(ctx, entity)
}

func (t *txDatabaseAdapter) Query() contracts.QueryBuilder {
	return nil // Not supported in transaction
}

func (t *txDatabaseAdapter) Transaction(ctx context.Context, fn func(tx contracts.Database) error) error {
	return fmt.Errorf("nested transactions not supported")
}

func (t *txDatabaseAdapter) Raw(ctx context.Context, query string, args ...any) (contracts.Result, error) {
	rows, err := t.tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return &rowsResult{rows: rows}, nil
}

func (t *txDatabaseAdapter) Exec(ctx context.Context, query string, args ...any) (contracts.ExecResult, error) {
	return t.tx.Exec(ctx, query, args...)
}

func (t *txDatabaseAdapter) Ping(ctx context.Context) error {
	return nil
}

func (t *txDatabaseAdapter) Close() error {
	return nil
}

// ============ Simple Query Builder ============

// SimpleQueryBuilder provides basic query building
type SimpleQueryBuilder struct {
	db         *Adapter
	selectCols []string
	table      string
	wheres     []whereClause
	orders     []orderClause
	limit      int
	offset     int
	joins      []string
	groupBy    []string
	having     []whereClause
}

type whereClause struct {
	condition string
	args      []any
}

type orderClause struct {
	column    string
	direction string
}

// NewSimpleQueryBuilder creates a new query builder
func NewSimpleQueryBuilder(db *Adapter) *SimpleQueryBuilder {
	return &SimpleQueryBuilder{
		db:         db,
		selectCols: []string{"*"},
		wheres:     make([]whereClause, 0),
		orders:     make([]orderClause, 0),
		joins:      make([]string, 0),
		groupBy:    make([]string, 0),
		having:     make([]whereClause, 0),
	}
}

func (q *SimpleQueryBuilder) Select(columns ...string) contracts.QueryBuilder {
	q.selectCols = columns
	return q
}

func (q *SimpleQueryBuilder) From(table string) contracts.QueryBuilder {
	q.table = table
	return q
}

func (q *SimpleQueryBuilder) Where(condition string, args ...any) contracts.QueryBuilder {
	q.wheres = append(q.wheres, whereClause{condition: condition, args: args})
	return q
}

func (q *SimpleQueryBuilder) WhereIn(column string, values ...any) contracts.QueryBuilder {
	placeholders := ""
	for i := range values {
		if i > 0 {
			placeholders += ", "
		}
		placeholders += "?"
	}
	q.wheres = append(q.wheres, whereClause{
		condition: fmt.Sprintf("%s IN (%s)", column, placeholders),
		args:      values,
	})
	return q
}

func (q *SimpleQueryBuilder) OrderBy(column string, direction string) contracts.QueryBuilder {
	q.orders = append(q.orders, orderClause{column: column, direction: direction})
	return q
}

func (q *SimpleQueryBuilder) Limit(limit int) contracts.QueryBuilder {
	q.limit = limit
	return q
}

func (q *SimpleQueryBuilder) Offset(offset int) contracts.QueryBuilder {
	q.offset = offset
	return q
}

func (q *SimpleQueryBuilder) Join(table string, condition string) contracts.QueryBuilder {
	q.joins = append(q.joins, fmt.Sprintf("JOIN %s ON %s", table, condition))
	return q
}

func (q *SimpleQueryBuilder) LeftJoin(table string, condition string) contracts.QueryBuilder {
	q.joins = append(q.joins, fmt.Sprintf("LEFT JOIN %s ON %s", table, condition))
	return q
}

func (q *SimpleQueryBuilder) GroupBy(columns ...string) contracts.QueryBuilder {
	q.groupBy = append(q.groupBy, columns...)
	return q
}

func (q *SimpleQueryBuilder) Having(condition string, args ...any) contracts.QueryBuilder {
	q.having = append(q.having, whereClause{condition: condition, args: args})
	return q
}

func (q *SimpleQueryBuilder) buildQuery() (string, []any) {
	var args []any

	// SELECT
	query := "SELECT " + joinStrings(q.selectCols, ", ")

	// FROM
	query += " FROM " + q.table

	// JOINS
	for _, join := range q.joins {
		query += " " + join
	}

	// WHERE
	if len(q.wheres) > 0 {
		query += " WHERE "
		for i, w := range q.wheres {
			if i > 0 {
				query += " AND "
			}
			query += w.condition
			args = append(args, w.args...)
		}
	}

	// GROUP BY
	if len(q.groupBy) > 0 {
		query += " GROUP BY " + joinStrings(q.groupBy, ", ")
	}

	// HAVING
	if len(q.having) > 0 {
		query += " HAVING "
		for i, h := range q.having {
			if i > 0 {
				query += " AND "
			}
			query += h.condition
			args = append(args, h.args...)
		}
	}

	// ORDER BY
	if len(q.orders) > 0 {
		query += " ORDER BY "
		for i, o := range q.orders {
			if i > 0 {
				query += ", "
			}
			query += o.column + " " + o.direction
		}
	}

	// LIMIT
	if q.limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", q.limit)
	}

	// OFFSET
	if q.offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", q.offset)
	}

	return query, args
}

func (q *SimpleQueryBuilder) Get(ctx context.Context, dest any) error {
	query, args := q.buildQuery()
	return q.db.FindAll(ctx, dest, query, args...)
}

func (q *SimpleQueryBuilder) First(ctx context.Context, dest any) error {
	q.limit = 1
	query, args := q.buildQuery()
	return q.db.FindOne(ctx, dest, query, args...)
}

func (q *SimpleQueryBuilder) Count(ctx context.Context) (int64, error) {
	q.selectCols = []string{"COUNT(*)"}
	query, args := q.buildQuery()

	result, err := q.db.Raw(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	defer func() { _ = result.Close() }() // Best-effort close

	var count int64
	if result.Next() {
		if err := result.Scan(&count); err != nil {
			return 0, err
		}
	}
	return count, nil
}

func (q *SimpleQueryBuilder) Exists(ctx context.Context) (bool, error) {
	count, err := q.Count(ctx)
	return count > 0, err
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

// ============ Standard SQL Driver ============

// StandardSQLDriver wraps standard database/sql
type StandardSQLDriver struct {
	db     *sql.DB
	config *contracts.DatabaseConfig
}

// NewStandardSQLDriver creates a driver using database/sql
// Usage:
//
//	import _ "github.com/lib/pq" // or mysql driver
//	driver := database.NewStandardSQLDriver()
//	adapter := database.New(driver)
//	adapter.Connect(&contracts.DatabaseConfig{...})
func NewStandardSQLDriver() *StandardSQLDriver {
	return &StandardSQLDriver{}
}

// Connect connects using database/sql
func (d *StandardSQLDriver) Connect(config *contracts.DatabaseConfig) error {
	dsn := buildDSN(config)

	db, err := sql.Open(config.Driver, dsn)
	if err != nil {
		return err
	}

	// Configure connection pool
	if config.MaxOpenConns > 0 {
		db.SetMaxOpenConns(config.MaxOpenConns)
	}
	if config.MaxIdleConns > 0 {
		db.SetMaxIdleConns(config.MaxIdleConns)
	}
	if config.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(time.Duration(config.ConnMaxLifetime) * time.Second)
	}

	d.db = db
	d.config = config
	return nil
}

// Close closes the connection
func (d *StandardSQLDriver) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

// Ping checks connection
func (d *StandardSQLDriver) Ping(ctx context.Context) error {
	return d.db.PingContext(ctx)
}

// DB returns the underlying *sql.DB
func (d *StandardSQLDriver) DB() *sql.DB {
	return d.db
}

// buildDSN builds connection string
func buildDSN(config *contracts.DatabaseConfig) string {
	switch config.Driver {
	case "postgres", "postgresql":
		sslMode := config.SSLMode
		if sslMode == "" {
			sslMode = "disable"
		}
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			config.Host, config.Port, config.Username, config.Password, config.Database, sslMode)
	case "mysql":
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
			config.Username, config.Password, config.Host, config.Port, config.Database)
	case "sqlite", "sqlite3":
		return config.Database
	default:
		return ""
	}
}

// ============ GORM Wrapper ============

// GORMDriver interface for GORM compatibility
// Usage:
//
//	import "gorm.io/gorm"
//	gormDB, _ := gorm.Open(postgres.Open(dsn), &gorm.Config{})
//	driver := database.WrapGORM(gormDB)
//	adapter := database.New(driver)
type GORMDriver interface {
	Driver
	CRUD
	QueryExecutor
	TransactionExecutor
}

// GORMWrapper wraps GORM DB instance
type GORMWrapper struct {
	db GORMDatabase
}

// GORMDatabase is the interface GORM DB implements
type GORMDatabase interface {
	Create(value any) GORMDatabase
	First(dest any, conds ...any) GORMDatabase
	Find(dest any, conds ...any) GORMDatabase
	Save(value any) GORMDatabase
	Delete(value any, conds ...any) GORMDatabase
	Where(query any, args ...any) GORMDatabase
	Raw(sql string, values ...any) GORMDatabase
	Exec(sql string, values ...any) GORMDatabase
	Scan(dest any) GORMDatabase
	Error() error
	RowsAffected() int64
	Begin() GORMDatabase
	Commit() GORMDatabase
	Rollback() GORMDatabase
	WithContext(ctx context.Context) GORMDatabase
}

// WrapGORM wraps a GORM database instance
func WrapGORM(db GORMDatabase) *GORMWrapper {
	return &GORMWrapper{db: db}
}

func (w *GORMWrapper) Connect(config *contracts.DatabaseConfig) error {
	return nil // Already connected
}

func (w *GORMWrapper) Close() error {
	return nil // GORM handles this
}

func (w *GORMWrapper) Ping(ctx context.Context) error {
	return nil // GORM handles this
}

func (w *GORMWrapper) DB() *sql.DB {
	return nil // GORM wraps this
}

func (w *GORMWrapper) Create(ctx context.Context, entity any) error {
	return w.db.WithContext(ctx).Create(entity).Error()
}

func (w *GORMWrapper) FindByID(ctx context.Context, id any, dest any) error {
	return w.db.WithContext(ctx).First(dest, id).Error()
}

func (w *GORMWrapper) FindOne(ctx context.Context, dest any, query string, args ...any) error {
	return w.db.WithContext(ctx).Where(query, args...).First(dest).Error()
}

func (w *GORMWrapper) FindAll(ctx context.Context, dest any, query string, args ...any) error {
	if query == "" {
		return w.db.WithContext(ctx).Find(dest).Error()
	}
	return w.db.WithContext(ctx).Where(query, args...).Find(dest).Error()
}

func (w *GORMWrapper) Update(ctx context.Context, entity any) error {
	return w.db.WithContext(ctx).Save(entity).Error()
}

func (w *GORMWrapper) Delete(ctx context.Context, entity any) error {
	return w.db.WithContext(ctx).Delete(entity).Error()
}

func (w *GORMWrapper) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return nil, fmt.Errorf("use Raw for GORM")
}

func (w *GORMWrapper) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return nil
}

func (w *GORMWrapper) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	result := w.db.WithContext(ctx).Exec(query, args...)
	return &gormExecResult{affected: result.RowsAffected()}, result.Error()
}

func (w *GORMWrapper) BeginTx(ctx context.Context) (Transaction, error) {
	tx := w.db.WithContext(ctx).Begin()
	return &gormTransaction{db: tx}, tx.Error()
}

type gormExecResult struct {
	affected int64
}

func (r *gormExecResult) LastInsertId() (int64, error) {
	return 0, nil
}

func (r *gormExecResult) RowsAffected() (int64, error) {
	return r.affected, nil
}

type gormTransaction struct {
	db GORMDatabase
}

func (t *gormTransaction) Create(ctx context.Context, entity any) error {
	return t.db.Create(entity).Error()
}

func (t *gormTransaction) FindByID(ctx context.Context, id any, dest any) error {
	return t.db.First(dest, id).Error()
}

func (t *gormTransaction) FindOne(ctx context.Context, dest any, query string, args ...any) error {
	return t.db.Where(query, args...).First(dest).Error()
}

func (t *gormTransaction) FindAll(ctx context.Context, dest any, query string, args ...any) error {
	return t.db.Where(query, args...).Find(dest).Error()
}

func (t *gormTransaction) Update(ctx context.Context, entity any) error {
	return t.db.Save(entity).Error()
}

func (t *gormTransaction) Delete(ctx context.Context, entity any) error {
	return t.db.Delete(entity).Error()
}

func (t *gormTransaction) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return nil, fmt.Errorf("not supported")
}

func (t *gormTransaction) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return nil
}

func (t *gormTransaction) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	result := t.db.Exec(query, args...)
	return &gormExecResult{affected: result.RowsAffected()}, result.Error()
}

func (t *gormTransaction) Commit() error {
	return t.db.Commit().Error()
}

func (t *gormTransaction) Rollback() error {
	return t.db.Rollback().Error()
}
