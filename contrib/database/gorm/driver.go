// Package gorm provides a GORM implementation of the unicorn Database interface.
//
// Usage:
//
//	import (
//	    "github.com/madcok-co/unicorn/contrib/database/gorm"
//	    "gorm.io/driver/postgres"
//	    gormpkg "gorm.io/gorm"
//	)
//
//	db, _ := gormpkg.Open(postgres.Open(dsn), &gormpkg.Config{})
//	driver := gorm.NewDriver(db)
//	app.SetDB(driver)
package gorm

import (
	"context"
	"errors"

	"github.com/madcok-co/unicorn/core/pkg/contracts"
	"gorm.io/gorm"
)

// Driver implements contracts.Database using GORM
type Driver struct {
	db *gorm.DB
}

// NewDriver creates a new GORM database driver
func NewDriver(db *gorm.DB) *Driver {
	return &Driver{db: db}
}

// DB returns the underlying GORM database instance
func (d *Driver) DB() *gorm.DB {
	return d.db
}

// Create creates a new entity in the database
func (d *Driver) Create(ctx context.Context, entity any) error {
	return d.db.WithContext(ctx).Create(entity).Error
}

// FindByID finds an entity by its ID
func (d *Driver) FindByID(ctx context.Context, id any, dest any) error {
	return d.db.WithContext(ctx).First(dest, id).Error
}

// FindOne finds a single entity matching the query
func (d *Driver) FindOne(ctx context.Context, dest any, query string, args ...any) error {
	return d.db.WithContext(ctx).Where(query, args...).First(dest).Error
}

// FindAll finds all entities matching the query
func (d *Driver) FindAll(ctx context.Context, dest any, query string, args ...any) error {
	if query == "" {
		return d.db.WithContext(ctx).Find(dest).Error
	}
	return d.db.WithContext(ctx).Where(query, args...).Find(dest).Error
}

// Update updates an entity in the database
func (d *Driver) Update(ctx context.Context, entity any) error {
	return d.db.WithContext(ctx).Save(entity).Error
}

// Delete deletes an entity from the database
func (d *Driver) Delete(ctx context.Context, entity any) error {
	return d.db.WithContext(ctx).Delete(entity).Error
}

// Query returns a new query builder
func (d *Driver) Query() contracts.QueryBuilder {
	return &QueryBuilder{db: d.db}
}

// Transaction executes a function within a database transaction
func (d *Driver) Transaction(ctx context.Context, fn func(tx contracts.Database) error) error {
	return d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txDriver := &Driver{db: tx}
		return fn(txDriver)
	})
}

// Raw executes a raw SQL query and returns results
func (d *Driver) Raw(ctx context.Context, query string, args ...any) (contracts.Result, error) {
	rows, err := d.db.WithContext(ctx).Raw(query, args...).Rows()
	if err != nil {
		return nil, err
	}
	return &Result{rows: rows}, nil
}

// Exec executes a raw SQL statement
func (d *Driver) Exec(ctx context.Context, query string, args ...any) (contracts.ExecResult, error) {
	result := d.db.WithContext(ctx).Exec(query, args...)
	if result.Error != nil {
		return nil, result.Error
	}
	return &ExecResult{result: result}, nil
}

// Ping checks database connectivity
func (d *Driver) Ping(ctx context.Context) error {
	sqlDB, err := d.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

// Close closes the database connection
func (d *Driver) Close() error {
	sqlDB, err := d.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// QueryBuilder implements contracts.QueryBuilder using GORM
type QueryBuilder struct {
	db *gorm.DB
}

func (q *QueryBuilder) Select(columns ...string) contracts.QueryBuilder {
	q.db = q.db.Select(columns)
	return q
}

func (q *QueryBuilder) From(table string) contracts.QueryBuilder {
	q.db = q.db.Table(table)
	return q
}

func (q *QueryBuilder) Where(condition string, args ...any) contracts.QueryBuilder {
	q.db = q.db.Where(condition, args...)
	return q
}

func (q *QueryBuilder) WhereIn(column string, values ...any) contracts.QueryBuilder {
	q.db = q.db.Where(column+" IN ?", values)
	return q
}

func (q *QueryBuilder) OrderBy(column string, direction string) contracts.QueryBuilder {
	q.db = q.db.Order(column + " " + direction)
	return q
}

func (q *QueryBuilder) Limit(limit int) contracts.QueryBuilder {
	q.db = q.db.Limit(limit)
	return q
}

func (q *QueryBuilder) Offset(offset int) contracts.QueryBuilder {
	q.db = q.db.Offset(offset)
	return q
}

func (q *QueryBuilder) Join(table string, condition string) contracts.QueryBuilder {
	q.db = q.db.Joins("JOIN " + table + " ON " + condition)
	return q
}

func (q *QueryBuilder) LeftJoin(table string, condition string) contracts.QueryBuilder {
	q.db = q.db.Joins("LEFT JOIN " + table + " ON " + condition)
	return q
}

func (q *QueryBuilder) GroupBy(columns ...string) contracts.QueryBuilder {
	for _, col := range columns {
		q.db = q.db.Group(col)
	}
	return q
}

func (q *QueryBuilder) Having(condition string, args ...any) contracts.QueryBuilder {
	q.db = q.db.Having(condition, args...)
	return q
}

func (q *QueryBuilder) Get(ctx context.Context, dest any) error {
	return q.db.WithContext(ctx).Find(dest).Error
}

func (q *QueryBuilder) First(ctx context.Context, dest any) error {
	return q.db.WithContext(ctx).First(dest).Error
}

func (q *QueryBuilder) Count(ctx context.Context) (int64, error) {
	var count int64
	err := q.db.WithContext(ctx).Count(&count).Error
	return count, err
}

func (q *QueryBuilder) Exists(ctx context.Context) (bool, error) {
	count, err := q.Count(ctx)
	return count > 0, err
}

// Result implements contracts.Result
type Result struct {
	rows interface {
		Scan(dest ...any) error
		Next() bool
		Close() error
	}
}

func (r *Result) Scan(dest ...any) error {
	return r.rows.Scan(dest...)
}

func (r *Result) Next() bool {
	return r.rows.Next()
}

func (r *Result) Close() error {
	return r.rows.Close()
}

// ExecResult implements contracts.ExecResult
type ExecResult struct {
	result *gorm.DB
}

func (r *ExecResult) LastInsertId() (int64, error) {
	// GORM doesn't directly expose LastInsertId
	// This is typically handled by the model's ID field
	return 0, errors.New("LastInsertId not supported in GORM, use model's ID field instead")
}

func (r *ExecResult) RowsAffected() (int64, error) {
	return r.result.RowsAffected, nil
}

// Ensure Driver implements contracts.Database
var _ contracts.Database = (*Driver)(nil)
