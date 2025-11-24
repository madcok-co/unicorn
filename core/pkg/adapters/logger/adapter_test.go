package logger

import (
	"bytes"
	"strings"
	"testing"
)

func TestStdDriver(t *testing.T) {
	t.Run("Log text format", func(t *testing.T) {
		var buf bytes.Buffer
		driver := NewStdDriver(&buf, FormatText)

		driver.Log(LevelDebug, "debug message")
		driver.Log(LevelInfo, "info message")
		driver.Log(LevelWarn, "warn message")
		driver.Log(LevelError, "error message")

		output := buf.String()
		// Level strings are lowercase in the implementation
		if !strings.Contains(output, "[debug]") {
			t.Error("expected debug log")
		}
		if !strings.Contains(output, "[info]") {
			t.Error("expected info log")
		}
		if !strings.Contains(output, "[warn]") {
			t.Error("expected warn log")
		}
		if !strings.Contains(output, "[error]") {
			t.Error("expected error log")
		}
	})

	t.Run("Log JSON format", func(t *testing.T) {
		var buf bytes.Buffer
		driver := NewStdDriver(&buf, FormatJSON)

		driver.Log(LevelInfo, "test message")

		output := buf.String()
		// Level is lowercase in JSON output
		if !strings.Contains(output, `"level":"info"`) {
			t.Errorf("expected JSON level field: %s", output)
		}
		if !strings.Contains(output, `"msg":"test message"`) {
			t.Errorf("expected JSON msg field: %s", output)
		}
	})

	t.Run("Log with fields text format", func(t *testing.T) {
		var buf bytes.Buffer
		driver := NewStdDriver(&buf, FormatText)

		driver.Log(LevelInfo, "user login", "user_id", 123, "ip", "192.168.1.1")

		output := buf.String()
		if !strings.Contains(output, "user_id=123") {
			t.Errorf("expected user_id field in output: %s", output)
		}
		if !strings.Contains(output, "ip=192.168.1.1") {
			t.Errorf("expected ip field in output: %s", output)
		}
	})

	t.Run("Log with fields JSON format", func(t *testing.T) {
		var buf bytes.Buffer
		driver := NewStdDriver(&buf, FormatJSON)

		driver.Log(LevelInfo, "user login", "user_id", 123, "ip", "192.168.1.1")

		output := buf.String()
		if !strings.Contains(output, `"user_id":123`) {
			t.Errorf("expected user_id field in JSON: %s", output)
		}
		if !strings.Contains(output, `"ip":"192.168.1.1"`) {
			t.Errorf("expected ip field in JSON: %s", output)
		}
	})

	t.Run("Sync", func(t *testing.T) {
		var buf bytes.Buffer
		driver := NewStdDriver(&buf, FormatText)
		err := driver.Sync()
		if err != nil {
			t.Errorf("Sync failed: %v", err)
		}
	})

	t.Run("Nil writer defaults to stdout", func(t *testing.T) {
		driver := NewStdDriver(nil, FormatText)
		// Should not panic
		driver.Log(LevelInfo, "test")
	})
}

func TestMultiDriver(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	driver1 := NewStdDriver(&buf1, FormatText)
	driver2 := NewStdDriver(&buf2, FormatText)

	multi := NewMultiDriver(driver1, driver2)

	t.Run("Log to multiple drivers", func(t *testing.T) {
		multi.Log(LevelInfo, "test message")

		if !strings.Contains(buf1.String(), "test message") {
			t.Error("expected message in driver1")
		}
		if !strings.Contains(buf2.String(), "test message") {
			t.Error("expected message in driver2")
		}
	})

	t.Run("Sync all drivers", func(t *testing.T) {
		err := multi.Sync()
		if err != nil {
			t.Errorf("Sync failed: %v", err)
		}
	})
}

func TestLevelString(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{LevelDebug, "debug"},
		{LevelInfo, "info"},
		{LevelWarn, "warn"},
		{LevelError, "error"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.level.String() != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, tt.level.String())
			}
		})
	}
}

func TestNoopDriver(t *testing.T) {
	driver := &NoopDriver{}

	// Should not panic
	driver.Log(LevelInfo, "test message", "key", "value")

	err := driver.Sync()
	if err != nil {
		t.Errorf("Sync should return nil: %v", err)
	}
}
