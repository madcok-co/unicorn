package transaction

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

// mockDB implements a minimal Database interface for testing
type mockDB struct {
	beginTxFunc func(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

func (m *mockDB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	if m.beginTxFunc != nil {
		return m.beginTxFunc(ctx, opts)
	}
	return nil, errors.New("not implemented")
}

func (m *mockDB) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return nil, errors.New("not implemented")
}

func (m *mockDB) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return nil, errors.New("not implemented")
}

func (m *mockDB) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return nil
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.IsolationLevel != sql.LevelDefault {
		t.Errorf("Expected IsolationLevel to be LevelDefault, got %v", config.IsolationLevel)
	}

	if config.ReadOnly {
		t.Error("Expected ReadOnly to be false")
	}
}

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	if config.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries to be 3, got %d", config.MaxRetries)
	}

	if config.ShouldRetry == nil {
		t.Error("Expected ShouldRetry to be set")
	}

	// Test retry logic
	if !config.ShouldRetry(errors.New("deadlock detected")) {
		t.Error("Should retry on deadlock")
	}

	if !config.ShouldRetry(errors.New("serialization failure")) {
		t.Error("Should retry on serialization failure")
	}

	if config.ShouldRetry(errors.New("syntax error")) {
		t.Error("Should not retry on syntax error")
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		substr string
		want   bool
	}{
		{"exact match", "deadlock", "deadlock", true},
		{"contains", "database deadlock detected", "deadlock", true},
		{"not contains", "some error", "deadlock", false},
		{"empty substr", "test", "", true},
		{"empty string", "", "test", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := contains(tt.s, tt.substr)
			if got != tt.want {
				t.Errorf("contains(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
			}
		})
	}
}

func TestFromContext(t *testing.T) {
	ctx := context.Background()

	// Test without transaction
	_, ok := FromContext(ctx)
	if ok {
		t.Error("Expected no transaction in empty context")
	}

	// Test with transaction
	tx := &Transaction{}
	ctx = context.WithValue(ctx, txKey, tx)

	gotTx, ok := FromContext(ctx)
	if !ok {
		t.Error("Expected transaction to be found")
	}
	if gotTx != tx {
		t.Error("Expected same transaction instance")
	}
}

func TestMustFromContext(t *testing.T) {
	// Test panic on missing transaction
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when transaction not in context")
		}
	}()

	ctx := context.Background()
	MustFromContext(ctx)
}

func TestMustFromContextSuccess(t *testing.T) {
	tx := &Transaction{}
	ctx := context.WithValue(context.Background(), txKey, tx)

	gotTx := MustFromContext(ctx)
	if gotTx != tx {
		t.Error("Expected same transaction instance")
	}
}

func TestTransactionStates(t *testing.T) {
	tx := &Transaction{
		committed:  false,
		rolledBack: false,
	}

	// Test initial state
	if tx.committed {
		t.Error("Transaction should not be committed initially")
	}
	if tx.rolledBack {
		t.Error("Transaction should not be rolled back initially")
	}

	// Test committed state
	tx.committed = true
	if !tx.committed {
		t.Error("Transaction should be committed")
	}

	// Test rolled back state
	tx2 := &Transaction{
		committed:  false,
		rolledBack: true,
	}
	if !tx2.rolledBack {
		t.Error("Transaction should be rolled back")
	}
}

func TestReadOnlyConfig(t *testing.T) {
	config := DefaultConfig()
	config.ReadOnly = true

	if !config.ReadOnly {
		t.Error("Expected ReadOnly to be true")
	}
}

func TestIsolationLevels(t *testing.T) {
	tests := []struct {
		name  string
		level sql.IsolationLevel
	}{
		{"Default", sql.LevelDefault},
		{"ReadUncommitted", sql.LevelReadUncommitted},
		{"ReadCommitted", sql.LevelReadCommitted},
		{"RepeatableRead", sql.LevelRepeatableRead},
		{"Serializable", sql.LevelSerializable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()
			config.IsolationLevel = tt.level

			if config.IsolationLevel != tt.level {
				t.Errorf("Expected isolation level %v, got %v", tt.level, config.IsolationLevel)
			}
		})
	}
}

func TestFindSubstring(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		substr string
		want   bool
	}{
		{"found at start", "hello world", "hello", true},
		{"found at end", "hello world", "world", true},
		{"found in middle", "hello world", "lo wo", true},
		{"not found", "hello world", "xyz", false},
		{"exact match", "test", "test", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findSubstring(tt.s, tt.substr)
			if got != tt.want {
				t.Errorf("findSubstring(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
			}
		})
	}
}

func TestConfigWithDifferentOptions(t *testing.T) {
	t.Run("serializable read-only", func(t *testing.T) {
		config := &Config{
			IsolationLevel: sql.LevelSerializable,
			ReadOnly:       true,
		}

		if config.IsolationLevel != sql.LevelSerializable {
			t.Error("Expected Serializable isolation level")
		}
		if !config.ReadOnly {
			t.Error("Expected ReadOnly to be true")
		}
	})

	t.Run("read committed", func(t *testing.T) {
		config := &Config{
			IsolationLevel: sql.LevelReadCommitted,
			ReadOnly:       false,
		}

		if config.IsolationLevel != sql.LevelReadCommitted {
			t.Error("Expected ReadCommitted isolation level")
		}
		if config.ReadOnly {
			t.Error("Expected ReadOnly to be false")
		}
	})
}
