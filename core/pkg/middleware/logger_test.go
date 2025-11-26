package middleware

import (
	"context"
	"regexp"
	"testing"

	unicornContext "github.com/madcok-co/unicorn/core/pkg/context"
	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

type mockLoggerForRequestResponse struct {
	infoLogs  []logEntry
	warnLogs  []logEntry
	errorLogs []logEntry
}

type logEntry struct {
	message string
	fields  map[string]any
}

func (m *mockLoggerForRequestResponse) Debug(msg string, keysAndValues ...any) {
	// Not tested
}

func (m *mockLoggerForRequestResponse) Info(msg string, keysAndValues ...any) {
	entry := logEntry{
		message: msg,
		fields:  kvToMap(keysAndValues),
	}
	m.infoLogs = append(m.infoLogs, entry)
}

func (m *mockLoggerForRequestResponse) Warn(msg string, keysAndValues ...any) {
	entry := logEntry{
		message: msg,
		fields:  kvToMap(keysAndValues),
	}
	m.warnLogs = append(m.warnLogs, entry)
}

func (m *mockLoggerForRequestResponse) Error(msg string, keysAndValues ...any) {
	entry := logEntry{
		message: msg,
		fields:  kvToMap(keysAndValues),
	}
	m.errorLogs = append(m.errorLogs, entry)
}

func (m *mockLoggerForRequestResponse) Fatal(msg string, keysAndValues ...any) {
	// Not tested
}

func (m *mockLoggerForRequestResponse) WithContext(ctx context.Context) contracts.Logger {
	return m
}

func (m *mockLoggerForRequestResponse) WithFields(fields ...any) contracts.Logger {
	return m
}

func (m *mockLoggerForRequestResponse) WithError(err error) contracts.Logger {
	return m
}

func (m *mockLoggerForRequestResponse) Named(name string) contracts.Logger {
	return m
}

func (m *mockLoggerForRequestResponse) Sync() error {
	return nil
}

func kvToMap(kv []any) map[string]any {
	m := make(map[string]any)
	for i := 0; i < len(kv); i += 2 {
		if i+1 < len(kv) {
			key := kv[i].(string)
			m[key] = kv[i+1]
		}
	}
	return m
}

func TestRequestResponseLogger(t *testing.T) {
	logger := &mockLoggerForRequestResponse{}
	middleware := RequestResponseLogger(logger)

	handler := middleware(func(ctx *unicornContext.Context) error {
		return ctx.JSON(200, map[string]string{"message": "success"})
	})

	ctx := createTestContext()
	ctx.Request().Method = "GET"
	ctx.Request().Path = "/api/test"

	err := handler(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(logger.infoLogs) != 1 {
		t.Errorf("Expected 1 info log, got %d", len(logger.infoLogs))
	}

	log := logger.infoLogs[0]
	if log.fields["method"] != "GET" {
		t.Errorf("Expected method GET, got %v", log.fields["method"])
	}
	if log.fields["path"] != "/api/test" {
		t.Errorf("Expected path /api/test, got %v", log.fields["path"])
	}
	if log.fields["status_code"] != 200 {
		t.Errorf("Expected status 200, got %v", log.fields["status_code"])
	}
}

func TestLoggerWithSensitiveDataMasking(t *testing.T) {
	logger := &mockLoggerForRequestResponse{}
	config := DefaultLoggerConfig()
	config.Logger = logger

	middleware := RequestResponseLoggerWithConfig(config)

	handler := middleware(func(ctx *unicornContext.Context) error {
		return ctx.JSON(200, map[string]string{"message": "success"})
	})

	ctx := createTestContext()
	ctx.Request().Method = "POST"
	ctx.Request().Path = "/api/login"
	ctx.Request().Body = []byte(`{"username":"john","password":"secret123"}`)

	err := handler(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(logger.infoLogs) != 1 {
		t.Errorf("Expected 1 info log, got %d", len(logger.infoLogs))
	}

	log := logger.infoLogs[0]
	requestBody, ok := log.fields["request_body"].(string)
	if !ok {
		t.Fatal("Expected request_body to be string")
	}

	// Password should be masked
	if !contains(requestBody, "***MASKED***") {
		t.Errorf("Expected password to be masked, got: %s", requestBody)
	}

	// Username should not be masked
	if !contains(requestBody, "john") {
		t.Errorf("Expected username to be present, got: %s", requestBody)
	}
}

func TestLoggerWithSensitiveHeaders(t *testing.T) {
	logger := &mockLoggerForRequestResponse{}
	config := DefaultLoggerConfig()
	config.Logger = logger

	middleware := RequestResponseLoggerWithConfig(config)

	handler := middleware(func(ctx *unicornContext.Context) error {
		return ctx.JSON(200, map[string]string{"message": "success"})
	})

	ctx := createTestContext()
	ctx.Request().Method = "GET"
	ctx.Request().Path = "/api/test"
	ctx.Request().Headers["Authorization"] = "Bearer secret-token"
	ctx.Request().Headers["Content-Type"] = "application/json"

	err := handler(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(logger.infoLogs) != 1 {
		t.Errorf("Expected 1 info log, got %d", len(logger.infoLogs))
	}

	log := logger.infoLogs[0]
	headers, ok := log.fields["request_headers"].(map[string]string)
	if !ok {
		t.Fatal("Expected request_headers to be map[string]string")
	}

	// Authorization should be masked
	if headers["Authorization"] != "***MASKED***" {
		t.Errorf("Expected Authorization to be masked, got: %s", headers["Authorization"])
	}

	// Content-Type should not be masked
	if headers["Content-Type"] != "application/json" {
		t.Errorf("Expected Content-Type to be preserved, got: %s", headers["Content-Type"])
	}
}

func TestLoggerSkipPaths(t *testing.T) {
	logger := &mockLoggerForRequestResponse{}
	config := DefaultLoggerConfig()
	config.Logger = logger

	middleware := RequestResponseLoggerWithConfig(config)

	handler := middleware(func(ctx *unicornContext.Context) error {
		return ctx.JSON(200, map[string]string{"status": "ok"})
	})

	ctx := createTestContext()
	ctx.Request().Method = "GET"
	ctx.Request().Path = "/health"

	err := handler(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Health check should be skipped
	if len(logger.infoLogs) != 0 {
		t.Errorf("Expected 0 info logs for /health, got %d", len(logger.infoLogs))
	}
}

func TestLoggerSkipPathsRegex(t *testing.T) {
	logger := &mockLoggerForRequestResponse{}
	config := DefaultLoggerConfig()
	config.Logger = logger
	config.SkipPathsRegex = []*regexp.Regexp{
		regexp.MustCompile(`^/api/v\d+/health`),
	}

	middleware := RequestResponseLoggerWithConfig(config)

	handler := middleware(func(ctx *unicornContext.Context) error {
		return ctx.JSON(200, map[string]string{"status": "ok"})
	})

	ctx := createTestContext()
	ctx.Request().Method = "GET"
	ctx.Request().Path = "/api/v1/health"

	err := handler(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Should be skipped by regex
	if len(logger.infoLogs) != 0 {
		t.Errorf("Expected 0 info logs for regex match, got %d", len(logger.infoLogs))
	}
}

func TestLoggerErrorLogging(t *testing.T) {
	logger := &mockLoggerForRequestResponse{}
	config := DefaultLoggerConfig()
	config.Logger = logger

	middleware := RequestResponseLoggerWithConfig(config)

	handler := middleware(func(ctx *unicornContext.Context) error {
		return ctx.Error(500, "Internal server error")
	})

	ctx := createTestContext()
	ctx.Request().Method = "GET"
	ctx.Request().Path = "/api/error"

	err := handler(ctx)
	if err != nil {
		t.Errorf("Expected no error from handler, got %v", err)
	}

	// Should log as error due to 500 status
	if len(logger.errorLogs) != 1 {
		t.Errorf("Expected 1 error log, got %d", len(logger.errorLogs))
	}

	log := logger.errorLogs[0]
	if log.fields["status_code"] != 500 {
		t.Errorf("Expected status 500, got %v", log.fields["status_code"])
	}
}

func TestLoggerWarnLogging(t *testing.T) {
	logger := &mockLoggerForRequestResponse{}
	config := DefaultLoggerConfig()
	config.Logger = logger

	middleware := RequestResponseLoggerWithConfig(config)

	handler := middleware(func(ctx *unicornContext.Context) error {
		return ctx.Error(404, "Not found")
	})

	ctx := createTestContext()
	ctx.Request().Method = "GET"
	ctx.Request().Path = "/api/notfound"

	err := handler(ctx)
	if err != nil {
		t.Errorf("Expected no error from handler, got %v", err)
	}

	// Should log as warning due to 404 status
	if len(logger.warnLogs) != 1 {
		t.Errorf("Expected 1 warn log, got %d", len(logger.warnLogs))
	}

	log := logger.warnLogs[0]
	if log.fields["status_code"] != 404 {
		t.Errorf("Expected status 404, got %v", log.fields["status_code"])
	}
}

func TestCompactLogger(t *testing.T) {
	logger := &mockLoggerForRequestResponse{}
	middleware := CompactLogger(logger)

	handler := middleware(func(ctx *unicornContext.Context) error {
		return ctx.JSON(200, map[string]string{"message": "success"})
	})

	ctx := createTestContext()
	ctx.Request().Method = "GET"
	ctx.Request().Path = "/api/test"
	ctx.Request().Body = []byte(`{"data":"test"}`)

	err := handler(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(logger.infoLogs) != 1 {
		t.Errorf("Expected 1 info log, got %d", len(logger.infoLogs))
	}

	log := logger.infoLogs[0]

	// Compact logger should not log request/response bodies
	if _, exists := log.fields["request_body"]; exists {
		t.Errorf("Compact logger should not log request body")
	}
	if _, exists := log.fields["response_body"]; exists {
		t.Errorf("Compact logger should not log response body")
	}

	// But should log latency
	if _, exists := log.fields["latency_ms"]; !exists {
		t.Errorf("Compact logger should log latency")
	}
}

func TestLoggerWithCustomFields(t *testing.T) {
	logger := &mockLoggerForRequestResponse{}
	middleware := CustomFieldsLogger(logger, func(ctx *unicornContext.Context) map[string]any {
		return map[string]any{
			"user_id":    "123",
			"request_id": "abc-def-ghi",
		}
	})

	handler := middleware(func(ctx *unicornContext.Context) error {
		return ctx.JSON(200, map[string]string{"message": "success"})
	})

	ctx := createTestContext()
	ctx.Request().Method = "GET"
	ctx.Request().Path = "/api/test"

	err := handler(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(logger.infoLogs) != 1 {
		t.Errorf("Expected 1 info log, got %d", len(logger.infoLogs))
	}

	log := logger.infoLogs[0]

	// Custom fields should be present
	if log.fields["user_id"] != "123" {
		t.Errorf("Expected user_id to be 123, got %v", log.fields["user_id"])
	}
	if log.fields["request_id"] != "abc-def-ghi" {
		t.Errorf("Expected request_id to be abc-def-ghi, got %v", log.fields["request_id"])
	}
}

func TestLoggerWithSkipper(t *testing.T) {
	logger := &mockLoggerForRequestResponse{}
	middleware := LoggerWithSkipper(logger, func(ctx *unicornContext.Context) bool {
		// Skip if path contains "skip"
		return contains(ctx.Request().Path, "skip")
	})

	handler := middleware(func(ctx *unicornContext.Context) error {
		return ctx.JSON(200, map[string]string{"message": "success"})
	})

	// Test skipped path
	ctx := createTestContext()
	ctx.Request().Method = "GET"
	ctx.Request().Path = "/api/skip/test"

	err := handler(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(logger.infoLogs) != 0 {
		t.Errorf("Expected 0 info logs for skipped path, got %d", len(logger.infoLogs))
	}

	// Test non-skipped path
	ctx2 := createTestContext()
	ctx2.Request().Method = "GET"
	ctx2.Request().Path = "/api/test"

	err = handler(ctx2)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(logger.infoLogs) != 1 {
		t.Errorf("Expected 1 info log for non-skipped path, got %d", len(logger.infoLogs))
	}
}

func TestLoggerMaxBodySize(t *testing.T) {
	logger := &mockLoggerForRequestResponse{}
	config := DefaultLoggerConfig()
	config.Logger = logger
	config.MaxBodySize = 10 // Very small limit

	middleware := RequestResponseLoggerWithConfig(config)

	handler := middleware(func(ctx *unicornContext.Context) error {
		return ctx.JSON(200, map[string]string{"message": "success"})
	})

	ctx := createTestContext()
	ctx.Request().Method = "POST"
	ctx.Request().Path = "/api/test"
	ctx.Request().Body = []byte(`{"data":"this is a very long body that should be truncated"}`)

	err := handler(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(logger.infoLogs) != 1 {
		t.Errorf("Expected 1 info log, got %d", len(logger.infoLogs))
	}

	log := logger.infoLogs[0]
	requestBody, ok := log.fields["request_body"].(string)
	if !ok {
		t.Fatal("Expected request_body to be string")
	}

	// Should be truncated
	if !contains(requestBody, "[truncated]") {
		t.Errorf("Expected body to be truncated, got: %s", requestBody)
	}
}

func TestMaskSensitiveJSON(t *testing.T) {
	sensitiveFields := map[string]bool{
		"password":    true,
		"token":       true,
		"api_key":     true,
		"credit_card": true,
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple password masking",
			input:    `{"username":"john","password":"secret"}`,
			expected: `***MASKED***`,
		},
		{
			name:     "Nested object",
			input:    `{"user":{"name":"john","password":"secret"}}`,
			expected: `***MASKED***`,
		},
		{
			name:     "Array of objects",
			input:    `{"users":[{"name":"john","password":"secret1"},{"name":"jane","password":"secret2"}]}`,
			expected: `***MASKED***`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskSensitiveJSON(tt.input, sensitiveFields, "***MASKED***")
			if !contains(result, tt.expected) {
				t.Errorf("Expected result to contain %s, got: %s", tt.expected, result)
			}
		})
	}
}

func TestMustCompilePatterns(t *testing.T) {
	patterns := []string{
		`^/api/v\d+/health`,
		`^/admin/.*`,
	}

	compiled := MustCompilePatterns(patterns)

	if len(compiled) != 2 {
		t.Errorf("Expected 2 compiled patterns, got %d", len(compiled))
	}

	// Test matching
	if !compiled[0].MatchString("/api/v1/health") {
		t.Errorf("Expected pattern to match /api/v1/health")
	}

	if !compiled[1].MatchString("/admin/users") {
		t.Errorf("Expected pattern to match /admin/users")
	}

	if compiled[0].MatchString("/api/users") {
		t.Errorf("Expected pattern NOT to match /api/users")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func createTestContext() *unicornContext.Context {
	ctx := unicornContext.New(context.Background())
	return ctx
}
