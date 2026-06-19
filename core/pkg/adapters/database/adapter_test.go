package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

// ============ Mock Types ============

type mockDriver struct {
	connectErr error
	closeErr   error
	pingErr    error
	db         *sql.DB
	connected  bool
	closed     bool
	mu         sync.Mutex
}

func (m *mockDriver) Connect(config *contracts.DatabaseConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.connectErr != nil {
		return m.connectErr
	}
	m.connected = true
	return nil
}

func (m *mockDriver) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return m.closeErr
}

func (m *mockDriver) Ping(ctx context.Context) error {
	return m.pingErr
}

func (m *mockDriver) DB() *sql.DB {
	return m.db
}

type mockCRUD struct {
	createCalls   []any
	findByIDCalls []struct{ id, dest any }
	findOneCalls  []struct {
		dest  any
		query string
		args  []any
	}
	findAllCalls []struct {
		dest  any
		query string
		args  []any
	}
	updateCalls []any
	deleteCalls []any

	createErr   error
	findByIDErr error
	findOneErr  error
	findAllErr  error
	updateErr   error
	deleteErr   error

	mu sync.Mutex
}

func (m *mockCRUD) Create(ctx context.Context, entity any) error {
	m.mu.Lock()
	m.createCalls = append(m.createCalls, entity)
	m.mu.Unlock()
	return m.createErr
}

func (m *mockCRUD) FindByID(ctx context.Context, id any, dest any) error {
	m.mu.Lock()
	m.findByIDCalls = append(m.findByIDCalls, struct{ id, dest any }{id, dest})
	m.mu.Unlock()
	return m.findByIDErr
}

func (m *mockCRUD) FindOne(ctx context.Context, dest any, query string, args ...any) error {
	m.mu.Lock()
	m.findOneCalls = append(m.findOneCalls, struct {
		dest  any
		query string
		args  []any
	}{dest, query, args})
	m.mu.Unlock()
	return m.findOneErr
}

func (m *mockCRUD) FindAll(ctx context.Context, dest any, query string, args ...any) error {
	m.mu.Lock()
	m.findAllCalls = append(m.findAllCalls, struct {
		dest  any
		query string
		args  []any
	}{dest, query, args})
	m.mu.Unlock()
	return m.findAllErr
}

func (m *mockCRUD) Update(ctx context.Context, entity any) error {
	m.mu.Lock()
	m.updateCalls = append(m.updateCalls, entity)
	m.mu.Unlock()
	return m.updateErr
}

func (m *mockCRUD) Delete(ctx context.Context, entity any) error {
	m.mu.Lock()
	m.deleteCalls = append(m.deleteCalls, entity)
	m.mu.Unlock()
	return m.deleteErr
}

type mockQueryExecutor struct {
	queryCalls []struct {
		query string
		args  []any
	}
	queryRowCalls []struct {
		query string
		args  []any
	}
	execCalls []struct {
		query string
		args  []any
	}

	queryErr   error
	queryRows  *sql.Rows
	queryRow   *sql.Row
	execResult sql.Result
	execErr    error
}

func (m *mockQueryExecutor) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	m.queryCalls = append(m.queryCalls, struct {
		query string
		args  []any
	}{query, args})
	return m.queryRows, m.queryErr
}

func (m *mockQueryExecutor) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	m.queryRowCalls = append(m.queryRowCalls, struct {
		query string
		args  []any
	}{query, args})
	return m.queryRow
}

func (m *mockQueryExecutor) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	m.execCalls = append(m.execCalls, struct {
		query string
		args  []any
	}{query, args})
	return m.execResult, m.execErr
}

type mockTransactionExecutor struct {
	beginTxErr    error
	transaction   *mockTransaction
	beginTxCalled bool
}

func (m *mockTransactionExecutor) BeginTx(ctx context.Context) (Transaction, error) {
	m.beginTxCalled = true
	if m.beginTxErr != nil {
		return nil, m.beginTxErr
	}
	return m.transaction, nil
}

type mockTransaction struct {
	createErr   error
	findByIDErr error
	findOneErr  error
	findAllErr  error
	updateErr   error
	deleteErr   error
	queryErr    error
	queryRows   *sql.Rows
	execResult  sql.Result
	execErr     error
	commitErr   error
	rollbackErr error

	committed    bool
	rolledBack   bool
	createCalled bool
	updateCalled bool
	deleteCalled bool

	mu sync.Mutex
}

func (m *mockTransaction) Create(ctx context.Context, entity any) error {
	m.mu.Lock()
	m.createCalled = true
	m.mu.Unlock()
	return m.createErr
}

func (m *mockTransaction) FindByID(ctx context.Context, id any, dest any) error {
	return m.findByIDErr
}

func (m *mockTransaction) FindOne(ctx context.Context, dest any, query string, args ...any) error {
	return m.findOneErr
}

func (m *mockTransaction) FindAll(ctx context.Context, dest any, query string, args ...any) error {
	return m.findAllErr
}

func (m *mockTransaction) Update(ctx context.Context, entity any) error {
	m.mu.Lock()
	m.updateCalled = true
	m.mu.Unlock()
	return m.updateErr
}

func (m *mockTransaction) Delete(ctx context.Context, entity any) error {
	m.mu.Lock()
	m.deleteCalled = true
	m.mu.Unlock()
	return m.deleteErr
}

func (m *mockTransaction) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return m.queryRows, m.queryErr
}

func (m *mockTransaction) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return nil
}

func (m *mockTransaction) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return m.execResult, m.execErr
}

func (m *mockTransaction) Commit() error {
	m.mu.Lock()
	m.committed = true
	m.mu.Unlock()
	return m.commitErr
}

func (m *mockTransaction) Rollback() error {
	m.mu.Lock()
	m.rolledBack = true
	m.mu.Unlock()
	return m.rollbackErr
}

// ============ Adapter Tests ============

func TestNewAdapter(t *testing.T) {
	t.Run("creates adapter with driver", func(t *testing.T) {
		driver := &mockDriver{}
		a := New(driver)

		if a == nil {
			t.Fatal("adapter should not be nil")
		}
		if a.driver != driver {
			t.Error("driver not set correctly")
		}
		if a.crud != nil {
			t.Error("crud should be nil initially")
		}
		if a.queryExec != nil {
			t.Error("queryExec should be nil initially")
		}
		if a.txExecutor != nil {
			t.Error("txExecutor should be nil initially")
		}
	})

	t.Run("builder methods chain correctly", func(t *testing.T) {
		crud := &mockCRUD{}
		qe := &mockQueryExecutor{}
		te := &mockTransactionExecutor{}
		qb := func() contracts.QueryBuilder { return nil }

		a := New(&mockDriver{}).
			WithCRUD(crud).
			WithQueryExecutor(qe).
			WithTransactionExecutor(te).
			WithQueryBuilder(qb)

		if a.crud != crud {
			t.Error("crud not set")
		}
		if a.queryExec != qe {
			t.Error("queryExec not set")
		}
		if a.txExecutor != te {
			t.Error("txExecutor not set")
		}
		if a.queryBuilder == nil {
			t.Error("queryBuilder not set")
		}
	})
}

// ============ Adapter Connect / Ping / Close ============

func TestAdapter_Connect(t *testing.T) {
	t.Run("connects through driver", func(t *testing.T) {
		driver := &mockDriver{}
		a := New(driver)

		cfg := &contracts.DatabaseConfig{Driver: "postgres", Host: "localhost"}
		err := a.Connect(cfg)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !driver.connected {
			t.Error("driver should be connected")
		}
		if a.config != cfg {
			t.Error("config not stored")
		}
	})

	t.Run("returns driver connect error", func(t *testing.T) {
		driver := &mockDriver{connectErr: errors.New("connection refused")}
		a := New(driver)

		err := a.Connect(&contracts.DatabaseConfig{})
		if err == nil {
			t.Error("expected error from driver")
		}
	})
}

func TestAdapter_Ping(t *testing.T) {
	t.Run("delegates to driver", func(t *testing.T) {
		driver := &mockDriver{pingErr: errors.New("timeout")}
		a := New(driver)

		err := a.Ping(context.Background())
		if err == nil {
			t.Error("expected ping error")
		}
	})
}

func TestAdapter_Close(t *testing.T) {
	t.Run("delegates to driver", func(t *testing.T) {
		driver := &mockDriver{}
		a := New(driver)

		err := a.Close()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !driver.closed {
			t.Error("driver should be closed")
		}
	})
}

// ============ CRUD Methods ============

func TestAdapter_Create(t *testing.T) {
	t.Run("delegates to CRUD", func(t *testing.T) {
		crud := &mockCRUD{}
		a := New(&mockDriver{}).WithCRUD(crud)

		err := a.Create(context.Background(), "entity")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(crud.createCalls) != 1 {
			t.Error("expected 1 create call")
		}
	})

	t.Run("errors when CRUD not configured", func(t *testing.T) {
		a := New(&mockDriver{})

		err := a.Create(context.Background(), "entity")
		if err == nil {
			t.Error("expected error")
		}
		if !strings.Contains(err.Error(), "CRUD not configured") {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestAdapter_FindByID(t *testing.T) {
	t.Run("delegates to CRUD", func(t *testing.T) {
		crud := &mockCRUD{}
		a := New(&mockDriver{}).WithCRUD(crud)

		var dest string
		err := a.FindByID(context.Background(), 42, &dest)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(crud.findByIDCalls) != 1 {
			t.Error("expected 1 call")
		}
	})

	t.Run("errors when CRUD not configured", func(t *testing.T) {
		a := New(&mockDriver{})
		err := a.FindByID(context.Background(), 1, nil)
		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestAdapter_FindOne(t *testing.T) {
	crud := &mockCRUD{findOneErr: errors.New("not found")}
	a := New(&mockDriver{}).WithCRUD(crud)

	var dest string
	err := a.FindOne(context.Background(), &dest, "name = ?", "john")
	if err == nil {
		t.Error("expected error")
	}
	if len(crud.findOneCalls) != 1 {
		t.Error("expected 1 call")
	}
}

func TestAdapter_FindAll(t *testing.T) {
	crud := &mockCRUD{}
	a := New(&mockDriver{}).WithCRUD(crud)

	var dest []string
	err := a.FindAll(context.Background(), &dest, "active = ?", true)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAdapter_Update(t *testing.T) {
	crud := &mockCRUD{}
	a := New(&mockDriver{}).WithCRUD(crud)

	err := a.Update(context.Background(), "updated")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(crud.updateCalls) != 1 {
		t.Error("expected 1 update call")
	}
}

func TestAdapter_Delete(t *testing.T) {
	crud := &mockCRUD{}
	a := New(&mockDriver{}).WithCRUD(crud)

	err := a.Delete(context.Background(), "entity")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(crud.deleteCalls) != 1 {
		t.Error("expected 1 delete call")
	}
}

// ============ Query / Raw / Exec ============

func TestAdapter_Query(t *testing.T) {
	t.Run("returns SimpleQueryBuilder when no custom builder", func(t *testing.T) {
		a := New(&mockDriver{})
		qb := a.Query()

		if qb == nil {
			t.Fatal("query builder should not be nil")
		}
		sqb, ok := qb.(*SimpleQueryBuilder)
		if !ok {
			t.Fatalf("expected *SimpleQueryBuilder, got %T", qb)
		}
		if sqb.db != a {
			t.Error("builder should reference adapter")
		}
	})

	t.Run("returns custom query builder when set", func(t *testing.T) {
		called := false
		a := New(&mockDriver{}).WithQueryBuilder(func() contracts.QueryBuilder {
			called = true
			return nil
		})

		_ = a.Query()
		if !called {
			t.Error("custom factory should be called")
		}
	})
}

func TestAdapter_Raw(t *testing.T) {
	t.Run("delegates to query executor", func(t *testing.T) {
		qe := &mockQueryExecutor{}
		a := New(&mockDriver{}).WithQueryExecutor(qe)

		_, err := a.Raw(context.Background(), "SELECT * FROM users")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("errors when no executor", func(t *testing.T) {
		a := New(&mockDriver{})
		_, err := a.Raw(context.Background(), "SELECT 1")
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("propagates query error", func(t *testing.T) {
		qe := &mockQueryExecutor{queryErr: errors.New("syntax error")}
		a := New(&mockDriver{}).WithQueryExecutor(qe)

		_, err := a.Raw(context.Background(), "BAD SQL")
		if err == nil {
			t.Error("expected query error")
		}
	})
}

func TestAdapter_Exec(t *testing.T) {
	t.Run("delegates to query executor", func(t *testing.T) {
		qe := &mockQueryExecutor{}
		a := New(&mockDriver{}).WithQueryExecutor(qe)

		_, err := a.Exec(context.Background(), "DELETE FROM users WHERE id = ?", 1)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(qe.execCalls) != 1 {
			t.Error("expected 1 exec call")
		}
	})

	t.Run("errors when no executor", func(t *testing.T) {
		a := New(&mockDriver{})
		_, err := a.Exec(context.Background(), "DELETE FROM users")
		if err == nil {
			t.Error("expected error")
		}
	})
}

// ============ Transaction ============

func TestAdapter_Transaction(t *testing.T) {
	t.Run("commits on success", func(t *testing.T) {
		tx := &mockTransaction{}
		te := &mockTransactionExecutor{transaction: tx}
		a := New(&mockDriver{}).WithTransactionExecutor(te)

		fnCalled := false
		err := a.Transaction(context.Background(), func(txDB contracts.Database) error {
			fnCalled = true
			return nil
		})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !fnCalled {
			t.Error("tx function should be called")
		}
		if !tx.committed {
			t.Error("transaction should be committed")
		}
		if tx.rolledBack {
			t.Error("transaction should NOT be rolled back")
		}
	})

	t.Run("rolls back on fn error", func(t *testing.T) {
		tx := &mockTransaction{}
		te := &mockTransactionExecutor{transaction: tx}
		a := New(&mockDriver{}).WithTransactionExecutor(te)

		wantErr := errors.New("business error")
		err := a.Transaction(context.Background(), func(txDB contracts.Database) error {
			return wantErr
		})

		if err != wantErr {
			t.Errorf("expected %v, got %v", wantErr, err)
		}
		if !tx.rolledBack {
			t.Error("transaction should be rolled back on error")
		}
		if tx.committed {
			t.Error("transaction should NOT be committed on error")
		}
	})

	t.Run("rolls back on panic", func(t *testing.T) {
		tx := &mockTransaction{}
		te := &mockTransactionExecutor{transaction: tx}
		a := New(&mockDriver{}).WithTransactionExecutor(te)

		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic to propagate")
			}
			if !tx.rolledBack {
				t.Error("transaction should be rolled back on panic")
			}
		}()

		_ = a.Transaction(context.Background(), func(txDB contracts.Database) error {
			panic("unexpected")
		})
	})

	t.Run("errors when no tx executor", func(t *testing.T) {
		a := New(&mockDriver{})
		err := a.Transaction(context.Background(), func(txDB contracts.Database) error {
			return nil
		})
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("txDatabaseAdapter works inside transaction", func(t *testing.T) {
		tx := &mockTransaction{}
		crud := &mockCRUD{}
		a := New(&mockDriver{}).
			WithCRUD(crud).
			WithTransactionExecutor(&mockTransactionExecutor{transaction: tx})

		err := a.Transaction(context.Background(), func(txDB contracts.Database) error {
			txDB.Create(context.Background(), "entity")
			txDB.Update(context.Background(), "entity")
			txDB.Delete(context.Background(), "entity")
			return nil
		})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !tx.createCalled {
			t.Error("tx Create should be called")
		}
		if !tx.updateCalled {
			t.Error("tx Update should be called")
		}
		if !tx.deleteCalled {
			t.Error("tx Delete should be called")
		}
	})
}

// ============ TxDatabaseAdapter ============

func TestTxDatabaseAdapter(t *testing.T) {
	t.Run("all CRUD delegates to transaction", func(t *testing.T) {
		tx := &mockTransaction{}
		txa := &txDatabaseAdapter{tx: tx}

		ctx := context.Background()
		txa.Create(ctx, "e")
		txa.FindByID(ctx, 1, nil)
		txa.FindOne(ctx, nil, "x=?", 1)
		txa.FindAll(ctx, nil, "x=?", 1)
		txa.Update(ctx, "e")
		txa.Delete(ctx, "e")

		if !tx.createCalled || !tx.updateCalled || !tx.deleteCalled {
			t.Error("all CRUD should delegate to tx")
		}
	})

	t.Run("Raw delegates to tx Query", func(t *testing.T) {
		tx := &mockTransaction{queryErr: errors.New("tx error")}
		txa := &txDatabaseAdapter{tx: tx}

		_, err := txa.Raw(context.Background(), "SELECT 1")
		if err == nil {
			t.Error("expected error from tx query")
		}
	})

	t.Run("Exec delegates to tx Exec", func(t *testing.T) {
		tx := &mockTransaction{execErr: errors.New("exec err")}
		txa := &txDatabaseAdapter{tx: tx}

		_, err := txa.Exec(context.Background(), "DELETE FROM x")
		if err == nil {
			t.Error("expected exec error")
		}
	})

	t.Run("Transaction returns error for nested", func(t *testing.T) {
		txa := &txDatabaseAdapter{tx: &mockTransaction{}}
		err := txa.Transaction(context.Background(), func(txDB contracts.Database) error {
			return nil
		})
		if err == nil {
			t.Error("expected nested transaction error")
		}
		if !strings.Contains(err.Error(), "nested transactions not supported") {
			t.Errorf("unexpected message: %v", err)
		}
	})

	t.Run("Query returns nil", func(t *testing.T) {
		txa := &txDatabaseAdapter{tx: &mockTransaction{}}
		if txa.Query() != nil {
			t.Error("Query should return nil in tx adapter")
		}
	})

	t.Run("Ping/Close return nil", func(t *testing.T) {
		txa := &txDatabaseAdapter{tx: &mockTransaction{}}
		if txa.Ping(context.Background()) != nil {
			t.Error("Ping should return nil")
		}
		if txa.Close() != nil {
			t.Error("Close should return nil")
		}
	})
}

// ============ SimpleQueryBuilder ============

func TestSimpleQueryBuilder_New(t *testing.T) {
	qb := NewSimpleQueryBuilder(&Adapter{})

	if qb == nil {
		t.Fatal("should not be nil")
	}
	if len(qb.selectCols) != 1 || qb.selectCols[0] != "*" {
		t.Error("default select should be *")
	}
	if qb.table != "" {
		t.Error("table should be empty")
	}
	if len(qb.wheres) != 0 {
		t.Error("wheres should be empty")
	}
}

func TestSimpleQueryBuilder_FluentAPI(t *testing.T) {
	qb := NewSimpleQueryBuilder(&Adapter{})

	qb.Select("id", "name")
	result := qb.From("users")
	qb.Where("active = ?", true)
	qb.OrderBy("name", "ASC")
	qb.Limit(10)
	qb.Offset(0)

	_, isQB := result.(contracts.QueryBuilder)
	if !isQB {
		t.Error("should return contracts.QueryBuilder")
	}
	if qb.table != "users" {
		t.Errorf("expected table 'users', got '%s'", qb.table)
	}
	if qb.limit != 10 {
		t.Errorf("expected limit 10, got %d", qb.limit)
	}
}

func TestSimpleQueryBuilder_buildQuery(t *testing.T) {
	t.Run("basic select query", func(t *testing.T) {
		qb := NewSimpleQueryBuilder(&Adapter{})
		qb.From("users")
		query, args := qb.buildQuery()

		expected := "SELECT * FROM users"
		if query != expected {
			t.Errorf("expected '%s', got '%s'", expected, query)
		}
		if len(args) != 0 {
			t.Errorf("expected 0 args, got %d", len(args))
		}
	})

	t.Run("select specific columns", func(t *testing.T) {
		qb := NewSimpleQueryBuilder(&Adapter{})
		qb.Select("id", "name", "email")
		qb.From("users")
		query, _ := qb.buildQuery()

		expected := "SELECT id, name, email FROM users"
		if query != expected {
			t.Errorf("expected '%s', got '%s'", expected, query)
		}
	})

	t.Run("single where clause", func(t *testing.T) {
		qb := NewSimpleQueryBuilder(&Adapter{})
		qb.From("users")
		qb.Where("active = ?", true)
		query, args := qb.buildQuery()

		expected := "SELECT * FROM users WHERE active = ?"
		if query != expected {
			t.Errorf("expected '%s', got '%s'", expected, query)
		}
		if len(args) != 1 || args[0] != true {
			t.Errorf("expected args [true], got %v", args)
		}
	})

	t.Run("multiple where clauses", func(t *testing.T) {
		qb := NewSimpleQueryBuilder(&Adapter{})
		qb.From("users")
		qb.Where("active = ?", true)
		qb.Where("age > ?", 18)
		query, args := qb.buildQuery()

		if !strings.Contains(query, "WHERE active = ? AND age > ?") {
			t.Errorf("expected AND in query, got '%s'", query)
		}
		if len(args) != 2 {
			t.Errorf("expected 2 args, got %d", len(args))
		}
	})

	t.Run("WhereIn", func(t *testing.T) {
		qb := NewSimpleQueryBuilder(&Adapter{})
		qb.From("users")
		qb.WhereIn("id", 1, 2, 3)
		query, args := qb.buildQuery()

		if !strings.Contains(query, "id IN (?, ?, ?)") {
			t.Errorf("expected IN clause, got '%s'", query)
		}
		if len(args) != 3 {
			t.Errorf("expected 3 args, got %d", len(args))
		}
	})

	t.Run("joins", func(t *testing.T) {
		qb := NewSimpleQueryBuilder(&Adapter{})
		qb.From("orders o")
		qb.Join("users u", "o.user_id = u.id")
		qb.LeftJoin("payments p", "o.id = p.order_id")
		query, _ := qb.buildQuery()

		if !strings.Contains(query, "JOIN users u ON o.user_id = u.id") {
			t.Errorf("expected JOIN, got '%s'", query)
		}
		if !strings.Contains(query, "LEFT JOIN payments p ON o.id = p.order_id") {
			t.Errorf("expected LEFT JOIN, got '%s'", query)
		}
	})

	t.Run("group by and having", func(t *testing.T) {
		qb := NewSimpleQueryBuilder(&Adapter{})
		qb.From("orders")
		qb.GroupBy("user_id")
		qb.Having("COUNT(*) > ?", 5)
		query, args := qb.buildQuery()

		if !strings.Contains(query, "GROUP BY user_id") {
			t.Errorf("expected GROUP BY, got '%s'", query)
		}
		if !strings.Contains(query, "HAVING COUNT(*) > ?") {
			t.Errorf("expected HAVING, got '%s'", query)
		}
		if len(args) != 1 || args[0] != 5 {
			t.Errorf("expected args [5], got %v", args)
		}
	})

	t.Run("order by multiple columns", func(t *testing.T) {
		qb := NewSimpleQueryBuilder(&Adapter{})
		qb.From("users")
		qb.OrderBy("name", "ASC")
		qb.OrderBy("age", "DESC")
		query, _ := qb.buildQuery()

		expected := " ORDER BY name ASC, age DESC"
		if !strings.Contains(query, expected) {
			t.Errorf("expected '%s' in query, got '%s'", expected, query)
		}
	})

	t.Run("limit and offset", func(t *testing.T) {
		qb := NewSimpleQueryBuilder(&Adapter{})
		qb.From("users")
		qb.Limit(10)
		qb.Offset(20)
		query, _ := qb.buildQuery()

		if !strings.Contains(query, "LIMIT 10") {
			t.Errorf("expected LIMIT 10, got '%s'", query)
		}
		if !strings.Contains(query, "OFFSET 20") {
			t.Errorf("expected OFFSET 20, got '%s'", query)
		}
	})

	t.Run("full complex query", func(t *testing.T) {
		qb := NewSimpleQueryBuilder(&Adapter{})
		qb.Select("u.id", "u.name", "COUNT(o.id) as order_count")
		qb.From("users u")
		qb.Join("orders o", "u.id = o.user_id")
		qb.Where("u.active = ?", true)
		qb.Where("o.created_at > ?", "2024-01-01")
		qb.GroupBy("u.id", "u.name")
		qb.Having("COUNT(o.id) > ?", 3)
		qb.OrderBy("order_count", "DESC")
		qb.Limit(5)
		qb.Offset(10)
		query, args := qb.buildQuery()

		if query == "" {
			t.Error("query should not be empty")
		}
		if !strings.HasPrefix(query, "SELECT") {
			t.Errorf("query should start with SELECT, got '%s'", query)
		}
		if !strings.Contains(query, "LIMIT 5") {
			t.Error("should have LIMIT 5")
		}
		if !strings.Contains(query, "OFFSET 10") {
			t.Error("should have OFFSET 10")
		}
		if len(args) != 3 {
			t.Errorf("expected 3 args, got %d: %v", len(args), args)
		}
	})
}

func TestSimpleQueryBuilder_First(t *testing.T) {
	t.Run("sets limit 1 and uses FindOne", func(t *testing.T) {
		crud := &mockCRUD{}
		a := New(&mockDriver{}).WithCRUD(crud)
		qb := NewSimpleQueryBuilder(a)
		qb.From("users")

		var dest string
		err := qb.First(context.Background(), &dest)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(crud.findOneCalls) != 1 {
			t.Error("expected 1 FindOne call")
		}
		if qb.limit != 1 {
			t.Errorf("expected limit 1, got %d", qb.limit)
		}
	})
}

func TestSimpleQueryBuilder_Get(t *testing.T) {
	t.Run("calls FindAll with built query", func(t *testing.T) {
		crud := &mockCRUD{}
		a := New(&mockDriver{}).WithCRUD(crud)
		qb := NewSimpleQueryBuilder(a)
		qb.From("users")
		qb.Where("active = ?", true)

		var dest []string
		err := qb.Get(context.Background(), &dest)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(crud.findAllCalls) != 1 {
			t.Error("expected 1 FindAll call")
		}
	})
}

func TestSimpleQueryBuilder_Count(t *testing.T) {
	t.Run("errors when Raw fails", func(t *testing.T) {
		qe := &mockQueryExecutor{queryErr: errors.New("table not found")}
		a := New(&mockDriver{}).WithQueryExecutor(qe)
		qb := NewSimpleQueryBuilder(a).From("users")

		_, err := qb.Count(context.Background())
		if err == nil {
			t.Error("expected error when Raw fails")
		}
	})
}

func TestSimpleQueryBuilder_Exists(t *testing.T) {
	t.Run("returns false when count is zero (raw error)", func(t *testing.T) {
		qe := &mockQueryExecutor{queryErr: errors.New("no table")}
		a := New(&mockDriver{}).WithQueryExecutor(qe)
		qb := NewSimpleQueryBuilder(a).From("users")

		exists, err := qb.Exists(context.Background())
		if err == nil {
			t.Error("expected error")
		}
		if exists {
			t.Error("should return false on error")
		}
	})
}

// ============ buildDSN ============

func TestBuildDSN(t *testing.T) {
	t.Run("postgres", func(t *testing.T) {
		cfg := &contracts.DatabaseConfig{
			Driver:   "postgres",
			Host:     "localhost",
			Port:     5432,
			Username: "user",
			Password: "pass",
			Database: "testdb",
		}
		dsn := buildDSN(cfg)

		if !strings.Contains(dsn, "host=localhost") {
			t.Errorf("expected host in DSN, got '%s'", dsn)
		}
		if !strings.Contains(dsn, "sslmode=disable") {
			t.Errorf("expected default sslmode, got '%s'", dsn)
		}
	})

	t.Run("postgres with custom sslmode", func(t *testing.T) {
		cfg := &contracts.DatabaseConfig{
			Driver:   "postgresql",
			Host:     "db.example.com",
			Port:     5432,
			Username: "admin",
			Password: "secret",
			Database: "prod",
			SSLMode:  "require",
		}
		dsn := buildDSN(cfg)

		if !strings.Contains(dsn, "sslmode=require") {
			t.Errorf("expected custom sslmode, got '%s'", dsn)
		}
	})

	t.Run("mysql", func(t *testing.T) {
		cfg := &contracts.DatabaseConfig{
			Driver:   "mysql",
			Host:     "localhost",
			Port:     3306,
			Username: "root",
			Password: "root",
			Database: "mydb",
		}
		dsn := buildDSN(cfg)

		if !strings.Contains(dsn, "root:root@tcp(localhost:3306)/mydb") {
			t.Errorf("expected MySQL DSN format, got '%s'", dsn)
		}
	})

	t.Run("sqlite", func(t *testing.T) {
		cfg := &contracts.DatabaseConfig{
			Driver:   "sqlite",
			Database: "/path/to/db.sqlite",
		}
		dsn := buildDSN(cfg)

		if dsn != "/path/to/db.sqlite" {
			t.Errorf("expected path, got '%s'", dsn)
		}
	})

	t.Run("unknown driver returns empty", func(t *testing.T) {
		cfg := &contracts.DatabaseConfig{Driver: "cassandra"}
		dsn := buildDSN(cfg)

		if dsn != "" {
			t.Errorf("expected empty DSN for unknown driver, got '%s'", dsn)
		}
	})
}

// ============ GORMWrapper ============

// mockGormDB implements GORMDatabase for testing GORMWrapper
type mockGormDB struct {
	createErr   error
	firstErr    error
	findErr     error
	saveErr     error
	deleteErr   error
	rawErr      error
	execErr     error
	scanErr     error
	beginErr    error
	commitErr   error
	rollbackErr error

	rowsAffected int64
	chainCalled  bool

	// Track the last self returned for fluent chaining
	lastSelf GORMDatabase
}

func (m *mockGormDB) Create(value any) GORMDatabase {
	m.lastSelf = m
	return m
}
func (m *mockGormDB) First(dest any, conds ...any) GORMDatabase {
	m.lastSelf = m
	return m
}
func (m *mockGormDB) Find(dest any, conds ...any) GORMDatabase {
	m.lastSelf = m
	return m
}
func (m *mockGormDB) Save(value any) GORMDatabase {
	m.lastSelf = m
	return m
}
func (m *mockGormDB) Delete(value any, conds ...any) GORMDatabase {
	m.lastSelf = m
	return m
}
func (m *mockGormDB) Where(query any, args ...any) GORMDatabase {
	m.lastSelf = m
	return m
}
func (m *mockGormDB) Raw(sql string, values ...any) GORMDatabase {
	m.lastSelf = m
	return m
}
func (m *mockGormDB) Exec(sql string, values ...any) GORMDatabase {
	m.lastSelf = m
	return m
}
func (m *mockGormDB) Scan(dest any) GORMDatabase {
	m.lastSelf = m
	return m
}
func (m *mockGormDB) Error() error {
	// Return the error for the last chain operation
	return nil
}
func (m *mockGormDB) RowsAffected() int64 {
	return m.rowsAffected
}
func (m *mockGormDB) Begin() GORMDatabase {
	m.lastSelf = m
	return m
}
func (m *mockGormDB) Commit() GORMDatabase {
	m.lastSelf = m
	return m
}
func (m *mockGormDB) Rollback() GORMDatabase {
	m.lastSelf = m
	return m
}
func (m *mockGormDB) WithContext(ctx context.Context) GORMDatabase {
	m.lastSelf = m
	return m
}

func TestWrapGORM(t *testing.T) {
	db := &mockGormDB{}
	w := WrapGORM(db)

	if w == nil {
		t.Fatal("wrapper should not be nil")
	}
	if w.db != db {
		t.Error("db not set")
	}
}

func TestGORMWrapper_Connect(t *testing.T) {
	w := WrapGORM(&mockGormDB{})
	err := w.Connect(&contracts.DatabaseConfig{})
	if err != nil {
		t.Error("Connect should return nil")
	}
}

func TestGORMWrapper_Close(t *testing.T) {
	w := WrapGORM(&mockGormDB{})
	if w.Close() != nil {
		t.Error("Close should return nil")
	}
}

func TestGORMWrapper_Ping(t *testing.T) {
	w := WrapGORM(&mockGormDB{})
	if w.Ping(context.Background()) != nil {
		t.Error("Ping should return nil")
	}
}

func TestGORMWrapper_DB(t *testing.T) {
	w := WrapGORM(&mockGormDB{})
	if w.DB() != nil {
		t.Error("DB should return nil for GORM")
	}
}

func TestGORMWrapper_CRUD(t *testing.T) {
	db := &mockGormDB{}
	w := WrapGORM(db)
	ctx := context.Background()

	t.Run("Create", func(t *testing.T) {
		err := w.Create(ctx, "entity")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("FindByID", func(t *testing.T) {
		err := w.FindByID(ctx, 1, nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("FindOne", func(t *testing.T) {
		err := w.FindOne(ctx, nil, "name = ?", "john")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("FindAll with query", func(t *testing.T) {
		err := w.FindAll(ctx, nil, "active = ?", true)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("FindAll with empty query", func(t *testing.T) {
		err := w.FindAll(ctx, nil, "")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Update", func(t *testing.T) {
		err := w.Update(ctx, "entity")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		err := w.Delete(ctx, "entity")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestGORMWrapper_Query(t *testing.T) {
	w := WrapGORM(&mockGormDB{})

	t.Run("Query returns unsupported error", func(t *testing.T) {
		_, err := w.Query(context.Background(), "SELECT 1")
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("QueryRow returns nil", func(t *testing.T) {
		if w.QueryRow(context.Background(), "SELECT 1") != nil {
			t.Error("QueryRow should return nil")
		}
	})
}

func TestGORMWrapper_Exec(t *testing.T) {
	w := WrapGORM(&mockGormDB{})
	result, err := w.Exec(context.Background(), "UPDATE users SET active = ?", false)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil {
		t.Error("result should not be nil")
	}
}

func TestGORMWrapper_BeginTx(t *testing.T) {
	t.Run("successful begin", func(t *testing.T) {
		w := WrapGORM(&mockGormDB{})
		tx, err := w.BeginTx(context.Background())
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if tx == nil {
			t.Error("transaction should not be nil")
		}
	})
}

// ============ GormTransaction ============

func TestGormTransaction_CRUD(t *testing.T) {
	db := &mockGormDB{}
	tx := &gormTransaction{db: db}
	ctx := context.Background()

	t.Run("Create", func(t *testing.T) {
		if err := tx.Create(ctx, "e"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("FindByID", func(t *testing.T) {
		if err := tx.FindByID(ctx, 1, nil); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("FindOne", func(t *testing.T) {
		if err := tx.FindOne(ctx, nil, "x=?", 1); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("FindAll", func(t *testing.T) {
		if err := tx.FindAll(ctx, nil, "x=?", 1); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("Update", func(t *testing.T) {
		if err := tx.Update(ctx, "e"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("Delete", func(t *testing.T) {
		if err := tx.Delete(ctx, "e"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestGormTransaction_Query(t *testing.T) {
	tx := &gormTransaction{db: &mockGormDB{}}

	t.Run("Query not supported", func(t *testing.T) {
		_, err := tx.Query(context.Background(), "SELECT 1")
		if err == nil {
			t.Error("expected not supported error")
		}
	})

	t.Run("QueryRow returns nil", func(t *testing.T) {
		if tx.QueryRow(context.Background(), "SELECT 1") != nil {
			t.Error("QueryRow should return nil")
		}
	})
}

func TestGormTransaction_Exec(t *testing.T) {
	tx := &gormTransaction{db: &mockGormDB{}}
	result, err := tx.Exec(context.Background(), "DELETE FROM x")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result == nil {
		t.Error("result should not be nil")
	}
}

func TestGormTransaction_CommitRollback(t *testing.T) {
	db := &mockGormDB{}
	tx := &gormTransaction{db: db}

	if tx.Commit() != nil {
		t.Error("Commit should succeed")
	}
	if tx.Rollback() != nil {
		t.Error("Rollback should succeed")
	}
}

// ============ GormExecResult ============

func TestGormExecResult(t *testing.T) {
	r := &gormExecResult{affected: 5}

	t.Run("LastInsertId returns 0", func(t *testing.T) {
		id, err := r.LastInsertId()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if id != 0 {
			t.Errorf("expected 0, got %d", id)
		}
	})

	t.Run("RowsAffected returns count", func(t *testing.T) {
		count, err := r.RowsAffected()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if count != 5 {
			t.Errorf("expected 5, got %d", count)
		}
	})
}

// ============ joinStrings ============

func TestJoinStrings(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		sep      string
		expected string
	}{
		{"empty", []string{}, ", ", ""},
		{"single", []string{"a"}, ", ", "a"},
		{"multiple", []string{"a", "b", "c"}, ", ", "a, b, c"},
		{"different sep", []string{"x", "y"}, " - ", "x - y"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := joinStrings(tt.input, tt.sep)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

// ============ Adapter implements contracts.Database ============

func TestAdapter_ImplementsContractsDatabase(t *testing.T) {
	// Compile-time check (if it compiles, it implements)
	var _ contracts.Database = (*Adapter)(nil)
}

// ============ Error propagation ============

func TestAdapter_ErrorPropagation(t *testing.T) {
	t.Run("CRUD errors propagate", func(t *testing.T) {
		crud := &mockCRUD{createErr: fmt.Errorf("unique violation")}
		a := New(&mockDriver{}).WithCRUD(crud)

		err := a.Create(context.Background(), "duplicate")
		if err == nil {
			t.Error("expected error to propagate")
		}
	})

	t.Run("Ping error propagates", func(t *testing.T) {
		driver := &mockDriver{pingErr: fmt.Errorf("connection lost")}
		a := New(driver)

		err := a.Ping(context.Background())
		if err == nil {
			t.Error("expected ping error")
		}
	})

	t.Run("Close error propagates", func(t *testing.T) {
		driver := &mockDriver{closeErr: fmt.Errorf("close failed")}
		a := New(driver)

		err := a.Close()
		if err == nil {
			t.Error("expected close error")
		}
	})
}
