package transaction

import (
	"context"
	"database/sql"
	"fmt"

	unicornContext "github.com/madcok-co/unicorn/core/pkg/context"
	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

// TxKey is the context key for storing transaction
type contextKey string

const (
	txKey contextKey = "unicorn_tx"
)

// Transaction represents a database transaction wrapper
type Transaction struct {
	db         contracts.Database
	tx         *sql.Tx
	committed  bool
	rolledBack bool
}

// Config defines transaction configuration
type Config struct {
	// IsolationLevel defines the transaction isolation level
	IsolationLevel sql.IsolationLevel

	// ReadOnly marks the transaction as read-only
	ReadOnly bool
}

// DefaultConfig returns default transaction configuration
func DefaultConfig() *Config {
	return &Config{
		IsolationLevel: sql.LevelDefault,
		ReadOnly:       false,
	}
}

// Begin starts a new transaction
func Begin(ctx context.Context, db contracts.Database) (*Transaction, error) {
	return BeginWithConfig(ctx, db, DefaultConfig())
}

// BeginWithConfig starts a new transaction with custom configuration
func BeginWithConfig(ctx context.Context, db contracts.Database, config *Config) (*Transaction, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Check if DB implements BeginTx (for *sql.DB)
	type txBeginner interface {
		BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
	}

	if txDB, ok := db.(txBeginner); ok {
		tx, err := txDB.BeginTx(ctx, &sql.TxOptions{
			Isolation: config.IsolationLevel,
			ReadOnly:  config.ReadOnly,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to begin transaction: %w", err)
		}

		return &Transaction{
			db: db,
			tx: tx,
		}, nil
	}

	return nil, fmt.Errorf("database does not support transactions")
}

// Commit commits the transaction
func (t *Transaction) Commit() error {
	if t.committed {
		return fmt.Errorf("transaction already committed")
	}
	if t.rolledBack {
		return fmt.Errorf("transaction already rolled back")
	}

	if err := t.tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	t.committed = true
	return nil
}

// Rollback rolls back the transaction
func (t *Transaction) Rollback() error {
	if t.committed {
		return fmt.Errorf("transaction already committed")
	}
	if t.rolledBack {
		return nil // Already rolled back, no error
	}

	if err := t.tx.Rollback(); err != nil {
		return fmt.Errorf("failed to rollback transaction: %w", err)
	}

	t.rolledBack = true
	return nil
}

// Tx returns the underlying *sql.Tx
func (t *Transaction) Tx() *sql.Tx {
	return t.tx
}

// Exec executes a query within the transaction
func (t *Transaction) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return t.tx.ExecContext(ctx, query, args...)
}

// Query executes a query that returns rows within the transaction
func (t *Transaction) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return t.tx.QueryContext(ctx, query, args...)
}

// QueryRow executes a query that returns a single row within the transaction
func (t *Transaction) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return t.tx.QueryRowContext(ctx, query, args...)
}

// WithTx executes a function within a transaction
// Automatically commits on success, rolls back on error or panic
func WithTx(ctx context.Context, db contracts.Database, fn func(*Transaction) error) error {
	return WithTxConfig(ctx, db, DefaultConfig(), fn)
}

// WithTxConfig executes a function within a transaction with custom config
// Automatically commits on success, rolls back on error or panic
func WithTxConfig(ctx context.Context, db contracts.Database, config *Config, fn func(*Transaction) error) error {
	tx, err := BeginWithConfig(ctx, db, config)
	if err != nil {
		return err
	}

	// Ensure rollback on panic
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r) // Re-panic after rollback
		}
	}()

	// Execute function
	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback failed: %v (original error: %w)", rbErr, err)
		}
		return err
	}

	// Commit transaction
	return tx.Commit()
}

// Transactional is a middleware that wraps handler in a transaction
// The transaction is stored in context and committed on success, rolled back on error
func Transactional(db contracts.Database) unicornContext.MiddlewareFunc {
	return TransactionalWithConfig(db, DefaultConfig())
}

// TransactionalWithConfig is a middleware with custom transaction config
func TransactionalWithConfig(db contracts.Database, config *Config) unicornContext.MiddlewareFunc {
	return func(next unicornContext.HandlerFunc) unicornContext.HandlerFunc {
		return func(ctx *unicornContext.Context) error {
			return WithTxConfig(ctx.Context(), db, config, func(tx *Transaction) error {
				// Store transaction in context
				newCtx := context.WithValue(ctx.Context(), txKey, tx)
				ctx.WithContext(newCtx)

				// Execute handler
				return next(ctx)
			})
		}
	}
}

// FromContext retrieves the transaction from context
func FromContext(ctx context.Context) (*Transaction, bool) {
	tx, ok := ctx.Value(txKey).(*Transaction)
	return tx, ok
}

// MustFromContext retrieves the transaction from context or panics
func MustFromContext(ctx context.Context) *Transaction {
	tx, ok := FromContext(ctx)
	if !ok {
		panic("transaction not found in context")
	}
	return tx
}

// GetTx retrieves transaction from Unicorn context
func GetTx(ctx *unicornContext.Context) (*Transaction, bool) {
	return FromContext(ctx.Context())
}

// MustGetTx retrieves transaction from Unicorn context or panics
func MustGetTx(ctx *unicornContext.Context) *Transaction {
	return MustFromContext(ctx.Context())
}

// ReadOnly starts a read-only transaction
func ReadOnly(ctx context.Context, db contracts.Database, fn func(*Transaction) error) error {
	config := DefaultConfig()
	config.ReadOnly = true
	return WithTxConfig(ctx, db, config, fn)
}

// Serializable starts a transaction with serializable isolation level
func Serializable(ctx context.Context, db contracts.Database, fn func(*Transaction) error) error {
	config := DefaultConfig()
	config.IsolationLevel = sql.LevelSerializable
	return WithTxConfig(ctx, db, config, fn)
}

// RepeatableRead starts a transaction with repeatable read isolation level
func RepeatableRead(ctx context.Context, db contracts.Database, fn func(*Transaction) error) error {
	config := DefaultConfig()
	config.IsolationLevel = sql.LevelRepeatableRead
	return WithTxConfig(ctx, db, config, fn)
}

// ReadCommitted starts a transaction with read committed isolation level
func ReadCommitted(ctx context.Context, db contracts.Database, fn func(*Transaction) error) error {
	config := DefaultConfig()
	config.IsolationLevel = sql.LevelReadCommitted
	return WithTxConfig(ctx, db, config, fn)
}

// ReadUncommitted starts a transaction with read uncommitted isolation level
func ReadUncommitted(ctx context.Context, db contracts.Database, fn func(*Transaction) error) error {
	config := DefaultConfig()
	config.IsolationLevel = sql.LevelReadUncommitted
	return WithTxConfig(ctx, db, config, fn)
}

// Nested executes a function within a nested transaction (savepoint)
// Note: Not all databases support savepoints
func Nested(ctx context.Context, tx *Transaction, name string, fn func(*Transaction) error) error {
	// Create savepoint
	if _, err := tx.Exec(ctx, fmt.Sprintf("SAVEPOINT %s", name)); err != nil {
		return fmt.Errorf("failed to create savepoint: %w", err)
	}

	// Execute function
	err := fn(tx)
	if err != nil {
		// Rollback to savepoint
		if _, rbErr := tx.Exec(ctx, fmt.Sprintf("ROLLBACK TO SAVEPOINT %s", name)); rbErr != nil {
			return fmt.Errorf("rollback to savepoint failed: %v (original error: %w)", rbErr, err)
		}
		return err
	}

	// Release savepoint
	if _, err := tx.Exec(ctx, fmt.Sprintf("RELEASE SAVEPOINT %s", name)); err != nil {
		return fmt.Errorf("failed to release savepoint: %w", err)
	}

	return nil
}

// RetryableTransaction executes a transaction with automatic retry on deadlock
type RetryConfig struct {
	MaxRetries  int
	ShouldRetry func(error) bool
}

// DefaultRetryConfig returns default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries: 3,
		ShouldRetry: func(err error) bool {
			// Check for common deadlock/serialization errors
			errMsg := err.Error()
			return contains(errMsg, "deadlock") ||
				contains(errMsg, "serialization") ||
				contains(errMsg, "could not serialize")
		},
	}
}

// WithRetry executes a transaction with automatic retry
func WithRetry(ctx context.Context, db contracts.Database, config *RetryConfig, fn func(*Transaction) error) error {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var lastErr error
	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		err := WithTx(ctx, db, fn)
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Check if should retry
		if !config.ShouldRetry(err) {
			return err // Don't retry
		}

		// Don't sleep on last attempt
		if attempt < config.MaxRetries {
			// Could add exponential backoff here
			continue
		}
	}

	return fmt.Errorf("transaction failed after %d retries: %w", config.MaxRetries, lastErr)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr || len(substr) == 0 || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
