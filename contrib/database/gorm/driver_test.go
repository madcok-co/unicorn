package gorm

import (
	"context"
	"testing"

	"github.com/madcok-co/unicorn/core/pkg/contracts"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Test model
type User struct {
	ID    uint   `gorm:"primarykey"`
	Name  string `gorm:"size:100"`
	Email string `gorm:"size:100;uniqueIndex"`
	Age   int
}

func setupTestDB(t *testing.T) *Driver {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}

	// Auto migrate
	db.AutoMigrate(&User{})

	return NewDriver(db)
}

func TestNewDriver(t *testing.T) {
	driver := setupTestDB(t)

	if driver == nil {
		t.Fatal("driver should not be nil")
	}
	if driver.db == nil {
		t.Error("db should not be nil")
	}
}

func TestDriver_DB(t *testing.T) {
	driver := setupTestDB(t)

	if driver.DB() == nil {
		t.Error("DB() should return underlying gorm.DB")
	}
}

func TestDriver_Create(t *testing.T) {
	driver := setupTestDB(t)
	ctx := context.Background()

	user := &User{Name: "John", Email: "john@example.com", Age: 30}
	err := driver.Create(ctx, user)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	if user.ID == 0 {
		t.Error("ID should be set after create")
	}
}

func TestDriver_FindByID(t *testing.T) {
	driver := setupTestDB(t)
	ctx := context.Background()

	// Create a user first
	user := &User{Name: "John", Email: "john@example.com", Age: 30}
	driver.Create(ctx, user)

	t.Run("found", func(t *testing.T) {
		var found User
		err := driver.FindByID(ctx, user.ID, &found)
		if err != nil {
			t.Fatalf("FindByID error: %v", err)
		}
		if found.Name != "John" {
			t.Errorf("expected name 'John', got %s", found.Name)
		}
	})

	t.Run("not found", func(t *testing.T) {
		var found User
		err := driver.FindByID(ctx, 9999, &found)
		if err == nil {
			t.Error("should return error for non-existent ID")
		}
	})
}

func TestDriver_FindOne(t *testing.T) {
	driver := setupTestDB(t)
	ctx := context.Background()

	driver.Create(ctx, &User{Name: "John", Email: "john@example.com", Age: 30})
	driver.Create(ctx, &User{Name: "Jane", Email: "jane@example.com", Age: 25})

	t.Run("found", func(t *testing.T) {
		var found User
		err := driver.FindOne(ctx, &found, "email = ?", "jane@example.com")
		if err != nil {
			t.Fatalf("FindOne error: %v", err)
		}
		if found.Name != "Jane" {
			t.Errorf("expected name 'Jane', got %s", found.Name)
		}
	})

	t.Run("not found", func(t *testing.T) {
		var found User
		err := driver.FindOne(ctx, &found, "email = ?", "nonexistent@example.com")
		if err == nil {
			t.Error("should return error for non-existent record")
		}
	})
}

func TestDriver_FindAll(t *testing.T) {
	driver := setupTestDB(t)
	ctx := context.Background()

	driver.Create(ctx, &User{Name: "John", Email: "john@example.com", Age: 30})
	driver.Create(ctx, &User{Name: "Jane", Email: "jane@example.com", Age: 25})
	driver.Create(ctx, &User{Name: "Bob", Email: "bob@example.com", Age: 35})

	t.Run("find all", func(t *testing.T) {
		var users []User
		err := driver.FindAll(ctx, &users, "")
		if err != nil {
			t.Fatalf("FindAll error: %v", err)
		}
		if len(users) != 3 {
			t.Errorf("expected 3 users, got %d", len(users))
		}
	})

	t.Run("find with query", func(t *testing.T) {
		var users []User
		err := driver.FindAll(ctx, &users, "age >= ?", 30)
		if err != nil {
			t.Fatalf("FindAll error: %v", err)
		}
		if len(users) != 2 {
			t.Errorf("expected 2 users, got %d", len(users))
		}
	})
}

func TestDriver_Update(t *testing.T) {
	driver := setupTestDB(t)
	ctx := context.Background()

	user := &User{Name: "John", Email: "john@example.com", Age: 30}
	driver.Create(ctx, user)

	user.Name = "John Updated"
	err := driver.Update(ctx, user)
	if err != nil {
		t.Fatalf("Update error: %v", err)
	}

	var found User
	driver.FindByID(ctx, user.ID, &found)
	if found.Name != "John Updated" {
		t.Errorf("expected name 'John Updated', got %s", found.Name)
	}
}

func TestDriver_Delete(t *testing.T) {
	driver := setupTestDB(t)
	ctx := context.Background()

	user := &User{Name: "John", Email: "john@example.com", Age: 30}
	driver.Create(ctx, user)

	err := driver.Delete(ctx, user)
	if err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	var found User
	err = driver.FindByID(ctx, user.ID, &found)
	if err == nil {
		t.Error("should return error for deleted record")
	}
}

func TestDriver_Transaction(t *testing.T) {
	driver := setupTestDB(t)
	ctx := context.Background()

	t.Run("successful transaction", func(t *testing.T) {
		err := driver.Transaction(ctx, func(tx contracts.Database) error {
			tx.Create(ctx, &User{Name: "TxUser1", Email: "tx1@example.com", Age: 20})
			tx.Create(ctx, &User{Name: "TxUser2", Email: "tx2@example.com", Age: 21})
			return nil
		})
		if err != nil {
			t.Fatalf("Transaction error: %v", err)
		}

		var users []User
		driver.FindAll(ctx, &users, "name LIKE ?", "TxUser%")
		if len(users) != 2 {
			t.Errorf("expected 2 users, got %d", len(users))
		}
	})

	t.Run("rollback transaction", func(t *testing.T) {
		initialCount := int64(0)
		driver.db.Model(&User{}).Count(&initialCount)

		err := driver.Transaction(ctx, func(tx contracts.Database) error {
			tx.Create(ctx, &User{Name: "RollbackUser", Email: "rollback@example.com", Age: 20})
			return gorm.ErrInvalidTransaction // Force rollback
		})
		if err == nil {
			t.Error("should return error")
		}

		var count int64
		driver.db.Model(&User{}).Count(&count)
		if count != initialCount {
			t.Error("transaction should be rolled back")
		}
	})
}

func TestDriver_Raw(t *testing.T) {
	driver := setupTestDB(t)
	ctx := context.Background()

	driver.Create(ctx, &User{Name: "John", Email: "john@example.com", Age: 30})

	result, err := driver.Raw(ctx, "SELECT name, email FROM users WHERE age = ?", 30)
	if err != nil {
		t.Fatalf("Raw error: %v", err)
	}
	defer result.Close()

	if result.Next() {
		var name, email string
		err := result.Scan(&name, &email)
		if err != nil {
			t.Fatalf("Scan error: %v", err)
		}
		if name != "John" {
			t.Errorf("expected name 'John', got %s", name)
		}
	}
}

func TestDriver_Exec(t *testing.T) {
	driver := setupTestDB(t)
	ctx := context.Background()

	driver.Create(ctx, &User{Name: "John", Email: "john@example.com", Age: 30})

	result, err := driver.Exec(ctx, "UPDATE users SET age = ? WHERE name = ?", 31, "John")
	if err != nil {
		t.Fatalf("Exec error: %v", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("RowsAffected error: %v", err)
	}
	if rows != 1 {
		t.Errorf("expected 1 row affected, got %d", rows)
	}
}

func TestDriver_Ping(t *testing.T) {
	driver := setupTestDB(t)
	ctx := context.Background()

	err := driver.Ping(ctx)
	if err != nil {
		t.Errorf("Ping should succeed: %v", err)
	}
}

func TestDriver_Close(t *testing.T) {
	driver := setupTestDB(t)

	err := driver.Close()
	if err != nil {
		t.Errorf("Close should succeed: %v", err)
	}
}

func TestQueryBuilder(t *testing.T) {
	driver := setupTestDB(t)
	ctx := context.Background()

	// Create test data
	driver.Create(ctx, &User{Name: "Alice", Email: "alice@example.com", Age: 25})
	driver.Create(ctx, &User{Name: "Bob", Email: "bob@example.com", Age: 30})
	driver.Create(ctx, &User{Name: "Charlie", Email: "charlie@example.com", Age: 35})

	t.Run("basic query", func(t *testing.T) {
		var users []User
		err := driver.Query().
			From("users").
			Where("age >= ?", 30).
			Get(ctx, &users)
		if err != nil {
			t.Fatalf("Query error: %v", err)
		}
		if len(users) != 2 {
			t.Errorf("expected 2 users, got %d", len(users))
		}
	})

	t.Run("select columns", func(t *testing.T) {
		var users []User
		err := driver.Query().
			Select("name", "email").
			From("users").
			Get(ctx, &users)
		if err != nil {
			t.Fatalf("Query error: %v", err)
		}
		if len(users) != 3 {
			t.Errorf("expected 3 users, got %d", len(users))
		}
	})

	t.Run("order by", func(t *testing.T) {
		var users []User
		err := driver.Query().
			From("users").
			OrderBy("age", "DESC").
			Get(ctx, &users)
		if err != nil {
			t.Fatalf("Query error: %v", err)
		}
		if users[0].Name != "Charlie" {
			t.Errorf("expected first user 'Charlie', got %s", users[0].Name)
		}
	})

	t.Run("limit and offset", func(t *testing.T) {
		var users []User
		err := driver.Query().
			From("users").
			OrderBy("age", "ASC").
			Limit(2).
			Offset(1).
			Get(ctx, &users)
		if err != nil {
			t.Fatalf("Query error: %v", err)
		}
		if len(users) != 2 {
			t.Errorf("expected 2 users, got %d", len(users))
		}
	})

	t.Run("first", func(t *testing.T) {
		var user User
		err := driver.Query().
			From("users").
			Where("age = ?", 30).
			First(ctx, &user)
		if err != nil {
			t.Fatalf("Query error: %v", err)
		}
		if user.Name != "Bob" {
			t.Errorf("expected 'Bob', got %s", user.Name)
		}
	})

	t.Run("count", func(t *testing.T) {
		count, err := driver.Query().
			From("users").
			Where("age >= ?", 30).
			Count(ctx)
		if err != nil {
			t.Fatalf("Query error: %v", err)
		}
		if count != 2 {
			t.Errorf("expected count 2, got %d", count)
		}
	})

	t.Run("exists", func(t *testing.T) {
		exists, err := driver.Query().
			From("users").
			Where("email = ?", "alice@example.com").
			Exists(ctx)
		if err != nil {
			t.Fatalf("Query error: %v", err)
		}
		if !exists {
			t.Error("should exist")
		}

		exists, err = driver.Query().
			From("users").
			Where("email = ?", "nonexistent@example.com").
			Exists(ctx)
		if err != nil {
			t.Fatalf("Query error: %v", err)
		}
		if exists {
			t.Error("should not exist")
		}
	})

	t.Run("where in", func(t *testing.T) {
		var users []User
		err := driver.Query().
			From("users").
			WhereIn("name", "Alice", "Bob").
			Get(ctx, &users)
		if err != nil {
			t.Fatalf("Query error: %v", err)
		}
		if len(users) != 2 {
			t.Errorf("expected 2 users, got %d", len(users))
		}
	})
}

func TestQueryBuilder_GroupBy(t *testing.T) {
	driver := setupTestDB(t)

	// This is a simple test just to ensure GroupBy doesn't panic
	qb := driver.Query().
		From("users").
		GroupBy("age")

	if qb == nil {
		t.Error("GroupBy should return query builder")
	}
}

func TestQueryBuilder_Having(t *testing.T) {
	driver := setupTestDB(t)

	qb := driver.Query().
		Select("age", "COUNT(*) as count").
		From("users").
		GroupBy("age").
		Having("COUNT(*) > ?", 0)

	if qb == nil {
		t.Error("Having should return query builder")
	}
}

func TestQueryBuilder_Join(t *testing.T) {
	driver := setupTestDB(t)

	qb := driver.Query().
		From("users").
		Join("orders", "orders.user_id = users.id")

	if qb == nil {
		t.Error("Join should return query builder")
	}
}

func TestQueryBuilder_LeftJoin(t *testing.T) {
	driver := setupTestDB(t)

	qb := driver.Query().
		From("users").
		LeftJoin("orders", "orders.user_id = users.id")

	if qb == nil {
		t.Error("LeftJoin should return query builder")
	}
}

func TestExecResult_LastInsertId(t *testing.T) {
	driver := setupTestDB(t)
	ctx := context.Background()

	result, _ := driver.Exec(ctx, "INSERT INTO users (name, email, age) VALUES (?, ?, ?)", "Test", "test@example.com", 20)

	_, err := result.LastInsertId()
	if err == nil {
		t.Error("LastInsertId should return error for GORM")
	}
}

func TestDriver_ImplementsDatabase(t *testing.T) {
	var _ contracts.Database = (*Driver)(nil)
}

func BenchmarkDriver_Create(b *testing.B) {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	db.AutoMigrate(&User{})
	driver := NewDriver(db)
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		driver.Create(ctx, &User{Name: "Test", Email: "test@example.com", Age: 20})
	}
}

func BenchmarkDriver_FindByID(b *testing.B) {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	db.AutoMigrate(&User{})
	driver := NewDriver(db)
	ctx := context.Background()

	user := &User{Name: "Test", Email: "test@example.com", Age: 20}
	driver.Create(ctx, user)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		var found User
		driver.FindByID(ctx, user.ID, &found)
	}
}
