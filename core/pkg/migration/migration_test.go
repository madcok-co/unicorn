package migration

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	contracts "github.com/madcok-co/unicorn/core/pkg/contracts"
)

// ---------------------------------------------------------------------------
// Mock helpers
// ---------------------------------------------------------------------------

// trackedCall records a single method invocation.
type trackedCall struct {
	Method string
	Query  string
	Args   []any
}

// mockExecResult implements contracts.ExecResult.
type mockExecResult struct {
	lastInsertID int64
	rowsAffected int64
	insertErr    error
	rowsErr      error
}

func (r *mockExecResult) LastInsertId() (int64, error) { return r.lastInsertID, r.insertErr }
func (r *mockExecResult) RowsAffected() (int64, error) { return r.rowsAffected, r.rowsErr }

// mockResult implements contracts.Result.
type mockResult struct{}

func (r *mockResult) Scan(dest ...any) error { return nil }
func (r *mockResult) Next() bool             { return false }
func (r *mockResult) Close() error           { return nil }

// mockQueryBuilder implements contracts.QueryBuilder.
type mockQueryBuilder struct{}

func (b *mockQueryBuilder) Select(columns ...string) contracts.QueryBuilder                { return b }
func (b *mockQueryBuilder) From(table string) contracts.QueryBuilder                       { return b }
func (b *mockQueryBuilder) Where(condition string, args ...any) contracts.QueryBuilder     { return b }
func (b *mockQueryBuilder) WhereIn(column string, values ...any) contracts.QueryBuilder    { return b }
func (b *mockQueryBuilder) OrderBy(column string, direction string) contracts.QueryBuilder { return b }
func (b *mockQueryBuilder) Limit(limit int) contracts.QueryBuilder                         { return b }
func (b *mockQueryBuilder) Offset(offset int) contracts.QueryBuilder                       { return b }
func (b *mockQueryBuilder) Join(table string, condition string) contracts.QueryBuilder     { return b }
func (b *mockQueryBuilder) LeftJoin(table string, condition string) contracts.QueryBuilder { return b }
func (b *mockQueryBuilder) GroupBy(columns ...string) contracts.QueryBuilder               { return b }
func (b *mockQueryBuilder) Having(condition string, args ...any) contracts.QueryBuilder    { return b }
func (b *mockQueryBuilder) Get(ctx context.Context, dest any) error                        { return nil }
func (b *mockQueryBuilder) First(ctx context.Context, dest any) error                      { return nil }
func (b *mockQueryBuilder) Count(ctx context.Context) (int64, error)                       { return 0, nil }
func (b *mockQueryBuilder) Exists(ctx context.Context) (bool, error)                       { return false, nil }

// mockDatabase implements contracts.Database with call tracking and
// configurable behaviour.
type mockDatabase struct {
	mu sync.Mutex

	// Call tracking
	Calls []trackedCall

	// Configurable behaviour
	ExecResult    contracts.ExecResult
	ExecErr       error
	ExecFn        func(ctx context.Context, query string, args ...any) (contracts.ExecResult, error)
	FindOneFn     func(dest any, query string, args ...any) error
	FindOneResult any   // value to assign to *dest when FindOneFn is nil
	FindOneErr    error // error returned by FindOne when FindOneFn is nil
	RawResult     contracts.Result
	RawErr        error
	CreateErr     error
	FindByIDErr   error
	FindAllErr    error
	UpdateErr     error
	DeleteErr     error
	PingErr       error
	CloseErr      error
}

func newMockDatabase() *mockDatabase {
	return &mockDatabase{
		ExecResult: &mockExecResult{rowsAffected: 1},
	}
}

func (d *mockDatabase) record(method, query string, args ...any) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Calls = append(d.Calls, trackedCall{Method: method, Query: query, Args: args})
}

// FindCall returns the first call matching method (or "" for any), or nil.
func (d *mockDatabase) FindCall(method string) *trackedCall {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i := range d.Calls {
		if method == "" || d.Calls[i].Method == method {
			return &d.Calls[i]
		}
	}
	return nil
}

// FindCallsByMethod returns all calls with the given method.
func (d *mockDatabase) FindCallsByMethod(method string) []trackedCall {
	d.mu.Lock()
	defer d.mu.Unlock()
	var out []trackedCall
	for _, c := range d.Calls {
		if c.Method == method {
			out = append(out, c)
		}
	}
	return out
}

// CallCountByMethod returns the number of calls with the given method.
func (d *mockDatabase) CallCountByMethod(method string) int {
	return len(d.FindCallsByMethod(method))
}

// Database interface implementation -------------------------------------------------

func (d *mockDatabase) Create(ctx context.Context, entity any) error {
	d.record("Create", "")
	return d.CreateErr
}

func (d *mockDatabase) FindByID(ctx context.Context, id any, dest any) error {
	d.record("FindByID", "")
	return d.FindByIDErr
}

func (d *mockDatabase) FindOne(ctx context.Context, dest any, query string, args ...any) error {
	d.record("FindOne", query, args...)
	if d.FindOneFn != nil {
		return d.FindOneFn(dest, query, args...)
	}
	if d.FindOneResult != nil && dest != nil {
		// dest is a pointer to a struct; copy via fmt hack for simplicity.
		// The migration code uses a struct{Version int64 `db:"version"`}.
		_, _ = fmt.Sscanf(fmt.Sprint(d.FindOneResult), "%d", dest)
	}
	return d.FindOneErr
}

func (d *mockDatabase) FindAll(ctx context.Context, dest any, query string, args ...any) error {
	d.record("FindAll", query, args...)
	return d.FindAllErr
}

func (d *mockDatabase) Update(ctx context.Context, entity any) error {
	d.record("Update", "")
	return d.UpdateErr
}

func (d *mockDatabase) Delete(ctx context.Context, entity any) error {
	d.record("Delete", "")
	return d.DeleteErr
}

func (d *mockDatabase) Query() contracts.QueryBuilder {
	d.record("Query", "")
	return &mockQueryBuilder{}
}

func (d *mockDatabase) Transaction(ctx context.Context, fn func(tx contracts.Database) error) error {
	d.record("Transaction", "")
	// Execute the function in a real DB scenario; our mock simply runs it.
	return fn(d)
}

func (d *mockDatabase) Raw(ctx context.Context, query string, args ...any) (contracts.Result, error) {
	d.record("Raw", query, args...)
	if d.RawResult == nil {
		d.RawResult = &mockResult{}
	}
	return d.RawResult, d.RawErr
}

func (d *mockDatabase) Exec(ctx context.Context, query string, args ...any) (contracts.ExecResult, error) {
	d.record("Exec", query, args...)
	if d.ExecFn != nil {
		return d.ExecFn(ctx, query, args...)
	}
	if d.ExecResult == nil {
		d.ExecResult = &mockExecResult{rowsAffected: 1}
	}
	return d.ExecResult, d.ExecErr
}

func (d *mockDatabase) Ping(ctx context.Context) error {
	d.record("Ping", "")
	return d.PingErr
}

func (d *mockDatabase) Close() error {
	d.record("Close", "")
	return d.CloseErr
}

// mockLogger implements contracts.Logger and tracks Info messages.
type mockLogger struct {
	mu       sync.Mutex
	Messages []string
}

func newMockLogger() *mockLogger { return &mockLogger{} }

func (l *mockLogger) Debug(msg string, fields ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Messages = append(l.Messages, "DEBUG:"+msg)
}

func (l *mockLogger) Info(msg string, fields ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Messages = append(l.Messages, "INFO:"+msg)
}

func (l *mockLogger) Warn(msg string, fields ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Messages = append(l.Messages, "WARN:"+msg)
}

func (l *mockLogger) Error(msg string, fields ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Messages = append(l.Messages, "ERROR:"+msg)
}

func (l *mockLogger) Fatal(msg string, fields ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Messages = append(l.Messages, "FATAL:"+msg)
}

func (l *mockLogger) WithContext(ctx context.Context) contracts.Logger { return l }
func (l *mockLogger) WithFields(fields ...any) contracts.Logger        { return l }
func (l *mockLogger) WithError(err error) contracts.Logger             { return l }
func (l *mockLogger) Named(name string) contracts.Logger               { return l }
func (l *mockLogger) Sync() error                                      { return nil }

// logContains returns true if any tracked message contains substr.
func (l *mockLogger) logContains(substr string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, m := range l.Messages {
		if strings.Contains(m, substr) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.TableName != "schema_migrations" {
		t.Errorf("expected TableName 'schema_migrations', got %q", cfg.TableName)
	}
}

func TestNew_NilConfig(t *testing.T) {
	m := New(nil)
	if m == nil {
		t.Fatal("expected non-nil Migrator")
	}
	if m.config.TableName != "schema_migrations" {
		t.Errorf("expected default table name, got %q", m.config.TableName)
	}
	if len(m.migrations) != 0 {
		t.Errorf("expected empty migrations, got %d", len(m.migrations))
	}
}

func TestNew_EmptyTableName(t *testing.T) {
	m := New(&Config{TableName: ""})
	if m.config.TableName != "schema_migrations" {
		t.Errorf("expected fallback table name, got %q", m.config.TableName)
	}
}

func TestNew_CustomConfig(t *testing.T) {
	db := newMockDatabase()
	log := newMockLogger()
	cfg := &Config{
		TableName:         "my_migrations",
		Database:          db,
		Logger:            log,
		SkipTableCreation: true,
	}
	m := New(cfg)
	if m.config.TableName != "my_migrations" {
		t.Errorf("expected TableName 'my_migrations', got %q", m.config.TableName)
	}
	if m.config.Database != db {
		t.Error("expected Database to be set")
	}
	if m.config.Logger != log {
		t.Error("expected Logger to be set")
	}
	if !m.config.SkipTableCreation {
		t.Error("expected SkipTableCreation to be true")
	}
}

func TestRegister(t *testing.T) {
	m := New(nil)
	mig := Migration{Version: 1, Description: "init"}
	result := m.Register(mig)
	if result != m {
		t.Error("Register should return self for chaining")
	}
	if len(m.migrations) != 1 {
		t.Errorf("expected 1 migration, got %d", len(m.migrations))
	}
	if m.migrations[0].Version != 1 {
		t.Errorf("expected version 1, got %d", m.migrations[0].Version)
	}
}

func TestRegisterMany(t *testing.T) {
	m := New(nil)
	m.Register(Migration{Version: 1}) // seed one
	migrations := []Migration{
		{Version: 2},
		{Version: 3},
	}
	result := m.RegisterMany(migrations)
	if result != m {
		t.Error("RegisterMany should return self for chaining")
	}
	if len(m.migrations) != 3 {
		t.Errorf("expected 3 migrations, got %d", len(m.migrations))
	}
	if m.migrations[2].Version != 3 {
		t.Errorf("expected last version 3, got %d", m.migrations[2].Version)
	}
}

func TestStatus_NoDatabase(t *testing.T) {
	m := New(nil)
	_, err := m.Status(context.Background())
	if err == nil {
		t.Fatal("expected error when no database configured")
	}
	if !strings.Contains(err.Error(), "database not configured") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestStatus_AppliedFlag(t *testing.T) {
	db := newMockDatabase()
	// Simulate current version = 2
	db.FindOneFn = func(d any, query string, args ...any) error {
		p := d.(*struct {
			Version int64 `db:"version"`
		})
		p.Version = 2
		return nil
	}

	cfg := &Config{Database: db}
	m := New(cfg)
	m.Register(Migration{Version: 1, Description: "first"})
	m.Register(Migration{Version: 2, Description: "second"})
	m.Register(Migration{Version: 3, Description: "third"})

	statuses, err := m.Status(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(statuses) != 3 {
		t.Fatalf("expected 3 statuses, got %d", len(statuses))
	}

	expected := []struct {
		version int64
		applied bool
	}{
		{1, true},
		{2, true},
		{3, false},
	}
	for i, exp := range expected {
		if statuses[i].Version != exp.version {
			t.Errorf("status[%d]: expected version %d, got %d", i, exp.version, statuses[i].Version)
		}
		if statuses[i].Applied != exp.applied {
			t.Errorf("status[%d]: expected applied=%v, got %v", i, exp.applied, statuses[i].Applied)
		}
	}
}

func TestStatus_FindOneError(t *testing.T) {
	db := newMockDatabase()
	db.FindOneErr = errors.New("connection refused")
	cfg := &Config{Database: db}
	m := New(cfg)
	_, err := m.Status(context.Background())
	if err == nil {
		t.Fatal("expected error from FindOne")
	}
}

func TestFileMigrationsToMigrations(t *testing.T) {
	fm := []FileMigration{
		{Version: 1, Description: "create users", UpSQL: "CREATE TABLE users (id INT)", DownSQL: "DROP TABLE users"},
		{Version: 2, Description: "add email", UpSQL: "ALTER TABLE users ADD email TEXT", DownSQL: "ALTER TABLE users DROP email"},
	}
	migrations := FileMigrationsToMigrations(fm)
	if len(migrations) != 2 {
		t.Fatalf("expected 2 migrations, got %d", len(migrations))
	}
	if migrations[0].Version != 1 {
		t.Errorf("expected version 1, got %d", migrations[0].Version)
	}
	if migrations[0].Description != "create users" {
		t.Errorf("expected description 'create users', got %q", migrations[0].Description)
	}
	if migrations[1].Version != 2 {
		t.Errorf("expected version 2, got %d", migrations[1].Version)
	}

	// Verify Up function executes correct SQL
	db := newMockDatabase()
	err := migrations[0].Up(db)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	call := db.FindCall("Exec")
	if call == nil {
		t.Fatal("expected Exec call")
	}
	if !strings.Contains(call.Query, "CREATE TABLE users") {
		t.Errorf("unexpected query: %s", call.Query)
	}

	// Verify Down function
	db2 := newMockDatabase()
	err = migrations[0].Down(db2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	call = db2.FindCall("Exec")
	if call == nil {
		t.Fatal("expected Exec call")
	}
	if !strings.Contains(call.Query, "DROP TABLE users") {
		t.Errorf("unexpected query: %s", call.Query)
	}

	// Empty input
	empty := FileMigrationsToMigrations(nil)
	if len(empty) != 0 {
		t.Errorf("expected empty migrations from nil, got %d", len(empty))
	}

	empty = FileMigrationsToMigrations([]FileMigration{})
	if len(empty) != 0 {
		t.Errorf("expected empty migrations from empty slice, got %d", len(empty))
	}
}

func TestFileMigrationsToMigrations_ExecError(t *testing.T) {
	fm := []FileMigration{
		{Version: 1, Description: "bad sql", UpSQL: "INVALID SQL", DownSQL: "ALSO INVALID"},
	}
	migrations := FileMigrationsToMigrations(fm)
	db := newMockDatabase()
	db.ExecErr = errors.New("syntax error")
	err := migrations[0].Up(db)
	if err == nil {
		t.Fatal("expected error from Exec")
	}
	if err.Error() != "syntax error" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFromEmbedFS(t *testing.T) {
	_, err := FromEmbedFS(embed.FS{}, "migrations")
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "not implemented yet" {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Up
// ---------------------------------------------------------------------------

func TestUp_NoDatabase(t *testing.T) {
	m := New(nil)
	m.Register(Migration{Version: 1, Description: "init"})
	err := m.Up(context.Background())
	if err == nil {
		t.Fatal("expected error when no database")
	}
	if !strings.Contains(err.Error(), "database not configured") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUp_CreatesTableAndRunsPending(t *testing.T) {
	db := newMockDatabase()
	log := newMockLogger()

	// FindOne returns current version 0 (no migrations applied).
	db.FindOneFn = func(d any, query string, args ...any) error {
		return fmt.Errorf("sql: no rows in result set")
	}

	var upCalled bool
	m := New(&Config{Database: db, Logger: log})
	m.Register(Migration{
		Version:     1,
		Description: "create users",
		Up: func(d contracts.Database) error {
			upCalled = true
			return nil
		},
	})

	err := m.Up(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !upCalled {
		t.Error("expected Up function to be called")
	}

	// Verify CREATE TABLE was called
	createCalls := db.FindCallsByMethod("Exec")
	foundCreate := false
	for _, c := range createCalls {
		if strings.Contains(c.Query, "CREATE TABLE IF NOT EXISTS") {
			foundCreate = true
			break
		}
	}
	if !foundCreate {
		t.Error("expected CREATE TABLE IF NOT EXISTS call")
	}

	// The Exec for CREATE is called first, then the findOne,
	// then the migration Up (which doesn't call Exec on db in this test),
	// then recordMigration calls Exec for INSERT.
	insertCall := db.FindCall("Exec") // after all the calls, find last Exec
	// Actually let's check overall: we should have Exec for CREATE TABLE and Exec for INSERT
	_ = insertCall
	if db.CallCountByMethod("Exec") < 2 {
		t.Errorf("expected at least 2 Exec calls (CREATE + INSERT), got %d", db.CallCountByMethod("Exec"))
	}

	// Verify logger recorded messages
	if !log.logContains("Current version: 0") {
		t.Error("expected log message about current version")
	}
	if !log.logContains("Running migration 1: create users") {
		t.Error("expected log message about running migration")
	}
	if !log.logContains("Migration 1 completed") {
		t.Error("expected log message about completed migration")
	}
	if !log.logContains("Applied 1 migration") {
		t.Error("expected log message about applied count")
	}
}

func TestUp_SkipsAlreadyApplied(t *testing.T) {
	db := newMockDatabase()
	log := newMockLogger()

	// Current version is 2, so version 1 and 2 are skipped.
	db.FindOneFn = func(d any, query string, args ...any) error {
		p := d.(*struct {
			Version int64 `db:"version"`
		})
		p.Version = 2
		return nil
	}

	var upCalled bool
	m := New(&Config{Database: db, Logger: log})
	m.Register(Migration{Version: 1, Description: "v1"})
	m.Register(Migration{Version: 2, Description: "v2"})
	m.Register(Migration{
		Version:     3,
		Description: "v3",
		Up: func(d contracts.Database) error {
			upCalled = true
			return nil
		},
	})

	err := m.Up(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !upCalled {
		t.Error("expected Up to be called for version 3")
	}
	if !log.logContains("Applied 1 migration") {
		t.Error("expected only 1 migration applied")
	}
}

func TestUp_NoPendingMigrations(t *testing.T) {
	db := newMockDatabase()
	log := newMockLogger()

	db.FindOneFn = func(d any, query string, args ...any) error {
		p := d.(*struct {
			Version int64 `db:"version"`
		})
		p.Version = 3
		return nil
	}

	m := New(&Config{Database: db, Logger: log})
	m.Register(Migration{Version: 1})
	m.Register(Migration{Version: 2})

	err := m.Up(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !log.logContains("No pending migrations") {
		t.Error("expected 'No pending migrations' log message")
	}
}

func TestUp_SkipTableCreation(t *testing.T) {
	db := newMockDatabase()
	log := newMockLogger()

	db.FindOneFn = func(d any, query string, args ...any) error {
		return fmt.Errorf("sql: no rows in result set")
	}

	var upCalled bool
	m := New(&Config{
		Database:          db,
		Logger:            log,
		SkipTableCreation: true,
	})
	m.Register(Migration{
		Version:     1,
		Description: "v1",
		Up: func(d contracts.Database) error {
			upCalled = true
			return nil
		},
	})

	err := m.Up(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !upCalled {
		t.Error("expected Up function to be called even with SkipTableCreation")
	}
	// Should NOT have a CREATE TABLE call.
	for _, c := range db.Calls {
		if strings.Contains(c.Query, "CREATE TABLE") {
			t.Error("expected no CREATE TABLE call when SkipTableCreation is true")
		}
	}
}

func TestUp_CreateTableError(t *testing.T) {
	db := newMockDatabase()
	db.ExecErr = errors.New("permission denied")

	m := New(&Config{Database: db})
	m.Register(Migration{Version: 1})

	err := m.Up(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to create migrations table") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUp_GetCurrentVersionError(t *testing.T) {
	db := newMockDatabase()
	db.FindOneErr = errors.New("connection lost")
	// Override exec to succeed so createTable passes (FindOne fails after).
	// Actually createMigrationsTable uses Exec, then getCurrentVersion uses FindOne.
	// Set ExecErr to nil, FindOneErr to error.
	db.ExecErr = nil
	db.FindOneErr = errors.New("connection lost")

	m := New(&Config{Database: db})
	m.Register(Migration{Version: 1})

	err := m.Up(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to get current version") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUp_MigrationUpError(t *testing.T) {
	db := newMockDatabase()
	db.FindOneFn = func(d any, query string, args ...any) error {
		return fmt.Errorf("sql: no rows in result set")
	}

	m := New(&Config{Database: db})
	m.Register(Migration{
		Version:     1,
		Description: "bad migration",
		Up: func(d contracts.Database) error {
			return errors.New("up failed")
		},
	})

	err := m.Up(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "migration 1 failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUp_RecordMigrationError(t *testing.T) {
	db := newMockDatabase()

	// FindOne returns "no rows" so the migration is considered pending.
	db.FindOneFn = func(d any, query string, args ...any) error {
		return fmt.Errorf("sql: no rows in result set")
	}

	// Exec succeeds for CREATE TABLE but fails for the INSERT (recordMigration).
	// We track the call count and fail on the second Exec.
	var execCount int
	db.ExecFn = func(ctx context.Context, query string, args ...any) (contracts.ExecResult, error) {
		execCount++
		if execCount == 2 {
			// The second Exec call is the INSERT in recordMigration.
			return nil, errors.New("insert failed")
		}
		return &mockExecResult{rowsAffected: 1}, nil
	}

	m := New(&Config{Database: db})
	m.Register(Migration{
		Version:     1,
		Description: "v1",
		Up:          func(d contracts.Database) error { return nil },
	})

	err := m.Up(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to record migration") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Down
// ---------------------------------------------------------------------------

func TestDown_NoDatabase(t *testing.T) {
	m := New(nil)
	err := m.Down(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "database not configured") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDown_NoMigrationsToRollback(t *testing.T) {
	db := newMockDatabase()
	log := newMockLogger()
	db.FindOneFn = func(d any, query string, args ...any) error {
		return fmt.Errorf("sql: no rows in result set")
	}

	m := New(&Config{Database: db, Logger: log})
	err := m.Down(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !log.logContains("No migrations to rollback") {
		t.Error("expected 'No migrations to rollback' log message")
	}
}

func TestDown_RollsBackLastMigration(t *testing.T) {
	db := newMockDatabase()
	log := newMockLogger()

	// Current version is 2.
	db.FindOneFn = func(d any, query string, args ...any) error {
		p := d.(*struct {
			Version int64 `db:"version"`
		})
		p.Version = 2
		return nil
	}

	var downCalled bool
	m := New(&Config{Database: db, Logger: log})
	m.Register(Migration{Version: 1, Description: "v1"})
	m.Register(Migration{
		Version:     2,
		Description: "v2",
		Down: func(d contracts.Database) error {
			downCalled = true
			return nil
		},
	})

	err := m.Down(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !downCalled {
		t.Error("expected Down to be called for version 2")
	}
	if !log.logContains("Rolling back migration 2: v2") {
		t.Error("expected rollback log message")
	}
	if !log.logContains("Rollback 2 completed") {
		t.Error("expected rollback completed log message")
	}

	// Verify DELETE was called for version 2.
	deleteCalled := false
	for _, c := range db.Calls {
		if c.Method == "Exec" && strings.Contains(c.Query, "DELETE") {
			deleteCalled = true
			if len(c.Args) > 0 {
				// args[0] should be version 2
			}
		}
	}
	if !deleteCalled {
		t.Error("expected DELETE call for removing migration record")
	}
}

func TestDown_MigrationNotFound(t *testing.T) {
	db := newMockDatabase()
	// Current version is 5, but only versions 1-3 registered.
	db.FindOneFn = func(d any, query string, args ...any) error {
		p := d.(*struct {
			Version int64 `db:"version"`
		})
		p.Version = 5
		return nil
	}

	m := New(&Config{Database: db})
	m.Register(Migration{Version: 1})
	m.Register(Migration{Version: 2})
	m.Register(Migration{Version: 3})

	err := m.Down(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "migration 5 not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDown_GetCurrentVersionError(t *testing.T) {
	db := newMockDatabase()
	db.FindOneErr = errors.New("query timeout")

	m := New(&Config{Database: db})
	err := m.Down(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to get current version") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDown_DownFuncError(t *testing.T) {
	db := newMockDatabase()
	db.FindOneFn = func(d any, query string, args ...any) error {
		p := d.(*struct {
			Version int64 `db:"version"`
		})
		p.Version = 1
		return nil
	}

	m := New(&Config{Database: db})
	m.Register(Migration{
		Version:     1,
		Description: "v1",
		Down: func(d contracts.Database) error {
			return errors.New("down failed")
		},
	})

	err := m.Down(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "rollback 1 failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDown_RemoveMigrationError(t *testing.T) {
	db := newMockDatabase()
	db.FindOneFn = func(d any, query string, args ...any) error {
		p := d.(*struct {
			Version int64 `db:"version"`
		})
		p.Version = 1
		return nil
	}
	// Set ExecFn so only the DELETE (removeMigration) fails, not the Down func.
	db.ExecFn = func(ctx context.Context, query string, args ...any) (contracts.ExecResult, error) {
		if strings.Contains(query, "DELETE") {
			return nil, errors.New("delete failed")
		}
		return &mockExecResult{rowsAffected: 1}, nil
	}

	m := New(&Config{Database: db})
	m.Register(Migration{
		Version:     1,
		Description: "v1",
		Down:        func(d contracts.Database) error { return nil },
	})

	err := m.Down(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to remove migration") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// DownTo
// ---------------------------------------------------------------------------

func TestDownTo_AlreadyAtTarget(t *testing.T) {
	db := newMockDatabase()
	log := newMockLogger()
	db.FindOneFn = func(d any, query string, args ...any) error {
		p := d.(*struct {
			Version int64 `db:"version"`
		})
		p.Version = 3
		return nil
	}

	m := New(&Config{Database: db, Logger: log})
	err := m.DownTo(context.Background(), 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !log.logContains("Already at or below target version") {
		t.Error("expected 'Already at or below target version' log message")
	}
}

func TestDownTo_AlreadyBelowTarget(t *testing.T) {
	db := newMockDatabase()
	log := newMockLogger()
	db.FindOneFn = func(d any, query string, args ...any) error {
		p := d.(*struct {
			Version int64 `db:"version"`
		})
		p.Version = 2
		return nil
	}

	m := New(&Config{Database: db, Logger: log})
	err := m.DownTo(context.Background(), 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !log.logContains("Already at or below target version") {
		t.Error("expected 'Already at or below target version' log message")
	}
}

func TestDownTo_RollsBackToTarget(t *testing.T) {
	db := newMockDatabase()
	log := newMockLogger()

	// Sequence of getCurrentVersion calls:
	//   DownTo initial check -> 3
	//   Down's getCurrentVersion -> 3
	//   DownTo loop check -> 2
	//   Down's getCurrentVersion -> 2
	//   DownTo loop check -> 1 (loop exits)
	versionSeq := []int64{3, 3, 2, 2, 1}
	seqIdx := 0
	db.FindOneFn = func(d any, query string, args ...any) error {
		if seqIdx >= len(versionSeq) {
			p := d.(*struct {
				Version int64 `db:"version"`
			})
			p.Version = 1
			return nil
		}
		p := d.(*struct {
			Version int64 `db:"version"`
		})
		p.Version = versionSeq[seqIdx]
		seqIdx++
		return nil
	}

	var downVersions []int64
	m := New(&Config{Database: db, Logger: log})
	m.Register(Migration{
		Version: 3, Description: "v3",
		Down: func(d contracts.Database) error {
			downVersions = append(downVersions, 3)
			return nil
		},
	})
	m.Register(Migration{
		Version: 2, Description: "v2",
		Down: func(d contracts.Database) error {
			downVersions = append(downVersions, 2)
			return nil
		},
	})
	m.Register(Migration{Version: 1})

	err := m.DownTo(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(downVersions) != 2 {
		t.Errorf("expected 2 down calls, got %d: %v", len(downVersions), downVersions)
	}
	if len(downVersions) >= 1 && downVersions[0] != 3 {
		t.Errorf("expected first down for version 3, got %d", downVersions[0])
	}
	if len(downVersions) >= 2 && downVersions[1] != 2 {
		t.Errorf("expected second down for version 2, got %d", downVersions[1])
	}
}

func TestDownTo_GetCurrentVersionError(t *testing.T) {
	db := newMockDatabase()
	db.FindOneErr = errors.New("connection error")

	m := New(&Config{Database: db})
	err := m.DownTo(context.Background(), 0)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to get current version") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// createMigrationsTable (internal, tested through Up but also directly via
// table-driven test on public methods that exercise it)
// ---------------------------------------------------------------------------

func TestCreateMigrationsTable_Success(t *testing.T) {
	db := newMockDatabase()
	m := New(&Config{Database: db})
	err := m.createMigrationsTable(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	call := db.FindCall("Exec")
	if call == nil {
		t.Fatal("expected Exec call")
	}
	if !strings.Contains(call.Query, "CREATE TABLE IF NOT EXISTS") {
		t.Errorf("unexpected query: %s", call.Query)
	}
	if !strings.Contains(call.Query, "schema_migrations") {
		t.Error("expected table name in CREATE TABLE query")
	}
}

func TestCreateMigrationsTable_CustomTableName(t *testing.T) {
	db := newMockDatabase()
	m := New(&Config{Database: db, TableName: "custom_migrations"})
	err := m.createMigrationsTable(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	call := db.FindCall("Exec")
	if call == nil {
		t.Fatal("expected Exec call")
	}
	if !strings.Contains(call.Query, "custom_migrations") {
		t.Errorf("expected custom table name in query, got: %s", call.Query)
	}
}

func TestCreateMigrationsTable_Error(t *testing.T) {
	db := newMockDatabase()
	db.ExecErr = errors.New("disk full")
	m := New(&Config{Database: db})
	err := m.createMigrationsTable(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "disk full" {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// getCurrentVersion
// ---------------------------------------------------------------------------

func TestGetCurrentVersion_Zero(t *testing.T) {
	db := newMockDatabase()
	// "sql: no rows in result set" means no migrations yet, version = 0
	db.FindOneErr = fmt.Errorf("sql: no rows in result set")
	m := New(&Config{Database: db})
	v, err := m.getCurrentVersion(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 0 {
		t.Errorf("expected version 0, got %d", v)
	}
}

func TestGetCurrentVersion_HasVersion(t *testing.T) {
	db := newMockDatabase()
	db.FindOneFn = func(d any, query string, args ...any) error {
		p := d.(*struct {
			Version int64 `db:"version"`
		})
		p.Version = 7
		return nil
	}
	m := New(&Config{Database: db})
	v, err := m.getCurrentVersion(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 7 {
		t.Errorf("expected version 7, got %d", v)
	}
}

func TestGetCurrentVersion_NonRowError(t *testing.T) {
	db := newMockDatabase()
	db.FindOneErr = errors.New("table does not exist")
	m := New(&Config{Database: db})
	_, err := m.getCurrentVersion(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "table does not exist" {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// recordMigration
// ---------------------------------------------------------------------------

func TestRecordMigration_Success(t *testing.T) {
	db := newMockDatabase()
	m := New(&Config{Database: db})
	err := m.recordMigration(context.Background(), 42, "test migration")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	calls := db.FindCallsByMethod("Exec")
	found := false
	for _, c := range calls {
		if strings.Contains(c.Query, "INSERT INTO") {
			found = true
			if len(c.Args) < 3 {
				t.Error("expected at least 3 args (version, description, time)")
			}
			break
		}
	}
	if !found {
		t.Error("expected INSERT INTO Exec call")
	}
}

func TestRecordMigration_Error(t *testing.T) {
	db := newMockDatabase()
	db.ExecErr = errors.New("unique constraint violation")
	m := New(&Config{Database: db})
	err := m.recordMigration(context.Background(), 1, "dup")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// removeMigration
// ---------------------------------------------------------------------------

func TestRemoveMigration_Success(t *testing.T) {
	db := newMockDatabase()
	m := New(&Config{Database: db})
	err := m.removeMigration(context.Background(), 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	calls := db.FindCallsByMethod("Exec")
	found := false
	for _, c := range calls {
		if strings.Contains(c.Query, "DELETE FROM") {
			found = true
			if len(c.Args) != 1 {
				t.Errorf("expected 1 arg (version), got %d", len(c.Args))
			}
			break
		}
	}
	if !found {
		t.Error("expected DELETE FROM Exec call")
	}
}

func TestRemoveMigration_Error(t *testing.T) {
	db := newMockDatabase()
	db.ExecErr = errors.New("permission denied")
	m := New(&Config{Database: db})
	err := m.removeMigration(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// log (internal)
// ---------------------------------------------------------------------------

func TestLog_NoLogger(t *testing.T) {
	m := New(nil)
	// This should not panic.
	m.log("test message %d", 42)
}

func TestLog_WithLogger(t *testing.T) {
	log := newMockLogger()
	m := New(&Config{Logger: log})
	m.log("hello %s", "world")
	if !log.logContains("hello world") {
		t.Errorf("expected log message, got: %v", log.Messages)
	}
}

// ---------------------------------------------------------------------------
// Full integration-style test: Up + Down + Status cycle
// ---------------------------------------------------------------------------

func TestFullMigrationCycle(t *testing.T) {
	db := newMockDatabase()
	log := newMockLogger()

	// Track current version externally to simulate a real DB state.
	var currentDBVersion int64
	db.FindOneFn = func(d any, query string, args ...any) error {
		p := d.(*struct {
			Version int64 `db:"version"`
		})
		p.Version = currentDBVersion
		return nil
	}

	m := New(&Config{Database: db, Logger: log})
	m.Register(Migration{
		Version:     1,
		Description: "create users",
		Up:          func(d contracts.Database) error { return nil },
		Down:        func(d contracts.Database) error { return nil },
	})
	m.Register(Migration{
		Version:     2,
		Description: "add email column",
		Up:          func(d contracts.Database) error { return nil },
		Down:        func(d contracts.Database) error { return nil },
	})

	// --- Up: apply all ---
	err := m.Up(context.Background())
	if err != nil {
		t.Fatalf("Up failed: %v", err)
	}
	// Simulate that the DB now reflects version 2.
	currentDBVersion = 2

	// Verify status shows all applied.
	statuses, err := m.Status(context.Background())
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}
	if !statuses[0].Applied || !statuses[1].Applied {
		t.Error("expected all migrations applied")
	}

	// --- Up again: no pending ---
	log.Messages = nil // reset
	err = m.Up(context.Background())
	if err != nil {
		t.Fatalf("second Up failed: %v", err)
	}
	if !log.logContains("No pending migrations") {
		t.Error("expected no pending migrations on second Up")
	}

	// --- Down: rollback v2 ---
	log.Messages = nil
	err = m.Down(context.Background())
	if err != nil {
		t.Fatalf("Down failed: %v", err)
	}
	currentDBVersion = 1

	// Verify status.
	statuses, err = m.Status(context.Background())
	if err != nil {
		t.Fatalf("Status after Down failed: %v", err)
	}
	if statuses[0].Applied != true {
		t.Error("expected v1 still applied")
	}
	if statuses[1].Applied != false {
		t.Error("expected v2 not applied after rollback")
	}

	// --- DownTo: already at 1, should be no-op ---
	log.Messages = nil
	err = m.DownTo(context.Background(), 1)
	if err != nil {
		t.Fatalf("DownTo 1 failed: %v", err)
	}
	if !log.logContains("Already at or below target version") {
		t.Error("expected no-op for DownTo when at target")
	}

	// --- Down: rollback v1 ---
	err = m.Down(context.Background())
	if err != nil {
		t.Fatalf("Down for v1 failed: %v", err)
	}
	currentDBVersion = 0

	statuses, err = m.Status(context.Background())
	if err != nil {
		t.Fatalf("Status after full rollback failed: %v", err)
	}
	for i, s := range statuses {
		if s.Applied {
			t.Errorf("expected migration %d not applied, but it was", i+1)
		}
	}

	// --- Down with no applied migrations ---
	log.Messages = nil
	err = m.Down(context.Background())
	if err != nil {
		t.Fatalf("Down at zero failed: %v", err)
	}
	if !log.logContains("No migrations to rollback") {
		t.Error("expected no migrations to rollback")
	}
}
