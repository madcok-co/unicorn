package zap

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/madcok-co/unicorn/core/pkg/contracts"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestNewDriver(t *testing.T) {
	driver := NewDriver()

	if driver == nil {
		t.Fatal("driver should not be nil")
	}
	if driver.logger == nil {
		t.Error("logger should not be nil")
	}
	if driver.sugar == nil {
		t.Error("sugar should not be nil")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Level != "info" {
		t.Errorf("expected level 'info', got %s", cfg.Level)
	}
	if cfg.Format != "json" {
		t.Errorf("expected format 'json', got %s", cfg.Format)
	}
	if cfg.Output != "stdout" {
		t.Errorf("expected output 'stdout', got %s", cfg.Output)
	}
	if !cfg.AddCaller {
		t.Error("AddCaller should be true")
	}
	if !cfg.AddStacktrace {
		t.Error("AddStacktrace should be true")
	}
}

func TestNewDriverWithConfig(t *testing.T) {
	t.Run("debug level", func(t *testing.T) {
		cfg := &Config{Level: "debug", Format: "json", Output: "stdout"}
		driver := NewDriverWithConfig(cfg)
		if driver == nil {
			t.Fatal("driver should not be nil")
		}
	})

	t.Run("warn level", func(t *testing.T) {
		cfg := &Config{Level: "warn", Format: "json", Output: "stdout"}
		driver := NewDriverWithConfig(cfg)
		if driver == nil {
			t.Fatal("driver should not be nil")
		}
	})

	t.Run("error level", func(t *testing.T) {
		cfg := &Config{Level: "error", Format: "json", Output: "stdout"}
		driver := NewDriverWithConfig(cfg)
		if driver == nil {
			t.Fatal("driver should not be nil")
		}
	})

	t.Run("unknown level defaults to info", func(t *testing.T) {
		cfg := &Config{Level: "unknown", Format: "json", Output: "stdout"}
		driver := NewDriverWithConfig(cfg)
		if driver == nil {
			t.Fatal("driver should not be nil")
		}
	})

	t.Run("console format", func(t *testing.T) {
		cfg := &Config{Level: "info", Format: "console", Output: "stdout"}
		driver := NewDriverWithConfig(cfg)
		if driver == nil {
			t.Fatal("driver should not be nil")
		}
	})

	t.Run("stderr output", func(t *testing.T) {
		cfg := &Config{Level: "info", Format: "json", Output: "stderr"}
		driver := NewDriverWithConfig(cfg)
		if driver == nil {
			t.Fatal("driver should not be nil")
		}
	})

	t.Run("empty output defaults to stdout", func(t *testing.T) {
		cfg := &Config{Level: "info", Format: "json", Output: ""}
		driver := NewDriverWithConfig(cfg)
		if driver == nil {
			t.Fatal("driver should not be nil")
		}
	})

	t.Run("with default fields", func(t *testing.T) {
		cfg := &Config{
			Level:         "info",
			Format:        "json",
			Output:        "stdout",
			DefaultFields: map[string]any{"service": "test", "version": "1.0"},
		}
		driver := NewDriverWithConfig(cfg)
		if driver == nil {
			t.Fatal("driver should not be nil")
		}
	})

	t.Run("with caller and stacktrace disabled", func(t *testing.T) {
		cfg := &Config{
			Level:         "info",
			Format:        "json",
			Output:        "stdout",
			AddCaller:     false,
			AddStacktrace: false,
		}
		driver := NewDriverWithConfig(cfg)
		if driver == nil {
			t.Fatal("driver should not be nil")
		}
	})
}

func TestNewDriverWithLogger(t *testing.T) {
	zapLogger, _ := zap.NewDevelopment()
	driver := NewDriverWithLogger(zapLogger)

	if driver == nil {
		t.Fatal("driver should not be nil")
	}
	if driver.Logger() != zapLogger {
		t.Error("should return the provided logger")
	}
}

// createTestDriver creates a driver with observable logs for testing
func createTestDriver() (*Driver, *observer.ObservedLogs) {
	core, recorded := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)
	return &Driver{logger: logger, sugar: logger.Sugar()}, recorded
}

func TestDriver_Debug(t *testing.T) {
	driver, logs := createTestDriver()

	driver.Debug("test message", "key", "value")

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	if entries[0].Message != "test message" {
		t.Errorf("expected message 'test message', got %s", entries[0].Message)
	}
	if entries[0].Level != zapcore.DebugLevel {
		t.Errorf("expected debug level, got %s", entries[0].Level)
	}
}

func TestDriver_Info(t *testing.T) {
	driver, logs := createTestDriver()

	driver.Info("info message", "count", 42)

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	if entries[0].Message != "info message" {
		t.Errorf("expected message 'info message', got %s", entries[0].Message)
	}
	if entries[0].Level != zapcore.InfoLevel {
		t.Errorf("expected info level, got %s", entries[0].Level)
	}
}

func TestDriver_Warn(t *testing.T) {
	driver, logs := createTestDriver()

	driver.Warn("warning message")

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	if entries[0].Level != zapcore.WarnLevel {
		t.Errorf("expected warn level, got %s", entries[0].Level)
	}
}

func TestDriver_Error(t *testing.T) {
	driver, logs := createTestDriver()

	driver.Error("error message")

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}
	if entries[0].Level != zapcore.ErrorLevel {
		t.Errorf("expected error level, got %s", entries[0].Level)
	}
}

func TestDriver_WithContext(t *testing.T) {
	driver, _ := createTestDriver()

	t.Run("with trace_id", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "trace_id", "abc123")
		newDriver := driver.WithContext(ctx)

		if newDriver == nil {
			t.Error("should return a logger")
		}
	})

	t.Run("with request_id", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), "request_id", "req-456")
		newDriver := driver.WithContext(ctx)

		if newDriver == nil {
			t.Error("should return a logger")
		}
	})

	t.Run("without context values", func(t *testing.T) {
		newDriver := driver.WithContext(context.Background())

		if newDriver != driver {
			t.Error("should return same driver when no context values")
		}
	})
}

func TestDriver_WithFields(t *testing.T) {
	driver, logs := createTestDriver()

	newDriver := driver.WithFields("user_id", "123", "action", "login")

	if newDriver == nil {
		t.Fatal("should return a new logger")
	}

	// Log something with the new driver
	newDriver.(*Driver).Info("user action")

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}

	// Check that fields are present
	contextMap := entries[0].ContextMap()
	if contextMap["user_id"] != "123" {
		t.Errorf("expected user_id '123', got %v", contextMap["user_id"])
	}
	if contextMap["action"] != "login" {
		t.Errorf("expected action 'login', got %v", contextMap["action"])
	}
}

func TestDriver_WithError(t *testing.T) {
	driver, logs := createTestDriver()

	testErr := errors.New("test error")
	newDriver := driver.WithError(testErr)

	if newDriver == nil {
		t.Fatal("should return a new logger")
	}

	// Log something with the new driver
	newDriver.(*Driver).Error("operation failed")

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}

	contextMap := entries[0].ContextMap()
	if contextMap["error"] != "test error" {
		t.Errorf("expected error 'test error', got %v", contextMap["error"])
	}
}

func TestDriver_Named(t *testing.T) {
	driver, logs := createTestDriver()

	namedDriver := driver.Named("myservice")

	if namedDriver == nil {
		t.Fatal("should return a new logger")
	}

	namedDriver.(*Driver).Info("named log")

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}

	if entries[0].LoggerName != "myservice" {
		t.Errorf("expected logger name 'myservice', got %s", entries[0].LoggerName)
	}
}

func TestDriver_Sync(t *testing.T) {
	driver, _ := createTestDriver()

	err := driver.Sync()

	if err != nil {
		t.Errorf("sync should not error: %v", err)
	}
}

func TestDriver_ImplementsLogger(t *testing.T) {
	var _ contracts.Logger = (*Driver)(nil)
}

func TestDriver_FileOutput(t *testing.T) {
	// Test invalid file path falls back to stdout
	cfg := &Config{
		Level:  "info",
		Format: "json",
		Output: "/nonexistent/path/that/should/not/exist/test.log",
	}

	driver := NewDriverWithConfig(cfg)
	if driver == nil {
		t.Fatal("driver should not be nil even with invalid file path")
	}
}

func TestDriver_LogOutput(t *testing.T) {
	// Create a buffer to capture output
	var buf bytes.Buffer

	encoder := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
	core := zapcore.NewCore(encoder, zapcore.AddSync(&buf), zapcore.InfoLevel)
	logger := zap.New(core)

	driver := &Driver{logger: logger, sugar: logger.Sugar()}

	driver.Info("test output", "key", "value")
	driver.Sync()

	output := buf.String()
	if !strings.Contains(output, "test output") {
		t.Errorf("output should contain 'test output', got: %s", output)
	}
	if !strings.Contains(output, "key") {
		t.Errorf("output should contain 'key', got: %s", output)
	}
}

func BenchmarkDriver_Info(b *testing.B) {
	driver, _ := createTestDriver()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		driver.Info("benchmark message", "iteration", i)
	}
}

func BenchmarkDriver_WithFields(b *testing.B) {
	driver, _ := createTestDriver()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		driver.WithFields("key", "value").(*Driver).Info("message")
	}
}
