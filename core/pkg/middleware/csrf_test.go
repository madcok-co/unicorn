package middleware

import (
	"context"
	"testing"
	"time"

	unicornContext "github.com/madcok-co/unicorn/core/pkg/context"
)

func TestDefaultCSRFConfig(t *testing.T) {
	config := DefaultCSRFConfig()

	if config.TokenLength != 32 {
		t.Errorf("Expected TokenLength 32, got %d", config.TokenLength)
	}

	if config.CookieName != "_csrf" {
		t.Errorf("Expected CookieName _csrf, got %s", config.CookieName)
	}

	if config.CookieMaxAge != 86400 {
		t.Errorf("Expected CookieMaxAge 86400, got %d", config.CookieMaxAge)
	}

	if !config.CookieHTTPOnly {
		t.Error("Expected CookieHTTPOnly to be true")
	}
}

func TestCSRFSafeMethods(t *testing.T) {
	middleware := CSRF()

	safeMethods := []string{"GET", "HEAD", "OPTIONS"}
	for _, method := range safeMethods {
		t.Run(method, func(t *testing.T) {
			handler := middleware(func(ctx *unicornContext.Context) error {
				return ctx.JSON(200, map[string]string{"status": "ok"})
			})

			ctx := unicornContext.New(context.Background())
			ctx.SetRequest(&unicornContext.Request{
				Method: method,
				Path:   "/api/test",
			})

			err := handler(ctx)
			if err != nil {
				t.Errorf("Safe method %s should not require CSRF token, got error: %v", method, err)
			}
		})
	}
}

func TestCSRFUnsafeMethodWithoutToken(t *testing.T) {
	middleware := CSRF()

	handler := middleware(func(ctx *unicornContext.Context) error {
		return ctx.JSON(200, map[string]string{"status": "ok"})
	})

	ctx := unicornContext.New(context.Background())
	ctx.SetRequest(&unicornContext.Request{
		Method:  "POST",
		Path:    "/api/test",
		Headers: make(map[string]string),
	})

	err := handler(ctx)
	if err != nil {
		t.Errorf("Expected no error (should fail in validation), got %v", err)
	}

	// Check response status
	if ctx.Response().StatusCode != 403 {
		t.Errorf("Expected status 403, got %d", ctx.Response().StatusCode)
	}
}

func TestCSRFWithValidToken(t *testing.T) {
	config := DefaultCSRFConfig()
	middleware := CSRFWithConfig(config)

	token := generateToken(32)

	handler := middleware(func(ctx *unicornContext.Context) error {
		return ctx.JSON(200, map[string]string{"status": "ok"})
	})

	ctx := unicornContext.New(context.Background())
	ctx.SetRequest(&unicornContext.Request{
		Method: "POST",
		Path:   "/api/test",
		Headers: map[string]string{
			"X-CSRF-Token": token,
		},
		Cookies: map[string]string{
			"_csrf": token,
		},
	})

	err := handler(ctx)
	if err != nil {
		t.Errorf("Expected no error with valid token, got %v", err)
	}

	if ctx.Response().StatusCode == 403 {
		t.Error("Should not return 403 with valid token")
	}
}

func TestCSRFTokenGeneration(t *testing.T) {
	token := generateToken(32)

	if len(token) == 0 {
		t.Error("Generated token should not be empty")
	}

	// Generate multiple tokens and ensure they're different
	token2 := generateToken(32)
	if token == token2 {
		t.Error("Generated tokens should be unique")
	}
}

func TestCSRFTokenValidation(t *testing.T) {
	token := "test-token-123"

	tests := []struct {
		name        string
		serverToken string
		clientToken string
		valid       bool
	}{
		{"matching tokens", token, token, true},
		{"different tokens", token, "wrong-token", false},
		{"empty client token", token, "", false},
		{"empty server token", "", token, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateToken(tt.serverToken, tt.clientToken)
			if result != tt.valid {
				t.Errorf("validateToken(%q, %q) = %v, want %v",
					tt.serverToken, tt.clientToken, result, tt.valid)
			}
		})
	}
}

func TestCSRFTokenExtractorHeader(t *testing.T) {
	extractor := createTokenExtractor("header:X-CSRF-Token")

	ctx := unicornContext.New(context.Background())
	ctx.SetRequest(&unicornContext.Request{
		Headers: map[string]string{
			"X-CSRF-Token": "test-token",
		},
	})

	token, err := extractor(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if token != "test-token" {
		t.Errorf("Expected token 'test-token', got %s", token)
	}
}

func TestCSRFTokenExtractorForm(t *testing.T) {
	extractor := createTokenExtractor("form:csrf_token")

	ctx := unicornContext.New(context.Background())
	ctx.SetRequest(&unicornContext.Request{
		Params: map[string]string{
			"csrf_token": "form-token",
		},
	})

	token, err := extractor(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if token != "form-token" {
		t.Errorf("Expected token 'form-token', got %s", token)
	}
}

func TestCSRFTokenExtractorQuery(t *testing.T) {
	extractor := createTokenExtractor("query:csrf")

	ctx := unicornContext.New(context.Background())
	ctx.SetRequest(&unicornContext.Request{
		Query: map[string]string{
			"csrf": "query-token",
		},
	})

	token, err := extractor(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if token != "query-token" {
		t.Errorf("Expected token 'query-token', got %s", token)
	}
}

func TestGetCSRFToken(t *testing.T) {
	ctx := unicornContext.New(context.Background())
	ctx.Set("csrf", "test-csrf-token")

	token := GetCSRFToken(ctx)
	if token != "test-csrf-token" {
		t.Errorf("Expected token 'test-csrf-token', got %s", token)
	}
}

func TestGetCSRFTokenNotFound(t *testing.T) {
	ctx := unicornContext.New(context.Background())

	token := GetCSRFToken(ctx)
	if token != "" {
		t.Errorf("Expected empty token, got %s", token)
	}
}

func TestCSRFWithSkipper(t *testing.T) {
	config := DefaultCSRFConfig()
	config.Skipper = func(ctx *unicornContext.Context) bool {
		return ctx.Request().Path == "/skip"
	}

	middleware := CSRFWithConfig(config)

	handler := middleware(func(ctx *unicornContext.Context) error {
		return ctx.JSON(200, map[string]string{"status": "ok"})
	})

	// Test skipped path
	ctx := unicornContext.New(context.Background())
	ctx.SetRequest(&unicornContext.Request{
		Method: "POST",
		Path:   "/skip",
	})

	err := handler(ctx)
	if err != nil {
		t.Errorf("Expected no error for skipped path, got %v", err)
	}

	if ctx.Response().StatusCode == 403 {
		t.Error("Skipped path should not return 403")
	}
}

func TestMemoryCSRFStore(t *testing.T) {
	store := NewMemoryCSRFStore()

	// Test Set and Get
	err := store.Set("user123", "token123", time.Hour)
	if err != nil {
		t.Errorf("Expected no error on Set, got %v", err)
	}

	token, err := store.Get("user123")
	if err != nil {
		t.Errorf("Expected no error on Get, got %v", err)
	}

	if token != "token123" {
		t.Errorf("Expected token 'token123', got %s", token)
	}

	// Test Delete
	err = store.Delete("user123")
	if err != nil {
		t.Errorf("Expected no error on Delete, got %v", err)
	}

	_, err = store.Get("user123")
	if err == nil {
		t.Error("Expected error when getting deleted token")
	}
}

func TestMemoryCSRFStoreExpiration(t *testing.T) {
	store := NewMemoryCSRFStore()

	// Set token with very short TTL
	err := store.Set("user123", "token123", time.Millisecond*10)
	if err != nil {
		t.Errorf("Expected no error on Set, got %v", err)
	}

	// Wait for expiration
	time.Sleep(time.Millisecond * 20)

	_, err = store.Get("user123")
	if err == nil {
		t.Error("Expected error when getting expired token")
	}
}

func TestCSRFFromReferer(t *testing.T) {
	middleware := CSRFFromReferer([]string{"https://example.com", "https://api.example.com"})

	tests := []struct {
		name       string
		method     string
		referer    string
		shouldPass bool
	}{
		{"GET request", "GET", "", true},
		{"Valid referer", "POST", "https://example.com/page", true},
		{"Valid referer subdomain", "POST", "https://api.example.com/endpoint", true},
		{"Invalid referer", "POST", "https://evil.com", false},
		{"Missing referer", "POST", "", false},
		{"Case insensitive", "POST", "HTTPS://EXAMPLE.COM/page", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := middleware(func(ctx *unicornContext.Context) error {
				return ctx.JSON(200, map[string]string{"status": "ok"})
			})

			ctx := unicornContext.New(context.Background())
			ctx.SetRequest(&unicornContext.Request{
				Method: tt.method,
				Path:   "/api/test",
				Headers: map[string]string{
					"Referer": tt.referer,
				},
			})

			err := handler(ctx)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			passed := ctx.Response().StatusCode != 403
			if passed != tt.shouldPass {
				t.Errorf("Expected pass=%v, got pass=%v (status=%d)",
					tt.shouldPass, passed, ctx.Response().StatusCode)
			}
		})
	}
}

func TestCSRFCustomErrorHandler(t *testing.T) {
	customErrorCalled := false

	config := DefaultCSRFConfig()
	config.ErrorHandler = func(ctx *unicornContext.Context, err error) error {
		customErrorCalled = true
		return ctx.Error(400, "Custom CSRF error")
	}

	middleware := CSRFWithConfig(config)

	handler := middleware(func(ctx *unicornContext.Context) error {
		return ctx.JSON(200, map[string]string{"status": "ok"})
	})

	ctx := unicornContext.New(context.Background())
	ctx.SetRequest(&unicornContext.Request{
		Method:  "POST",
		Path:    "/api/test",
		Headers: make(map[string]string),
	})

	handler(ctx)

	if !customErrorCalled {
		t.Error("Custom error handler should have been called")
	}

	if ctx.Response().StatusCode != 400 {
		t.Errorf("Expected status 400, got %d", ctx.Response().StatusCode)
	}
}

func TestCSRFCookieSettings(t *testing.T) {
	config := DefaultCSRFConfig()
	config.CookieDomain = "example.com"
	config.CookieSecure = true
	config.CookieSameSite = "Strict"

	middleware := CSRFWithConfig(config)

	handler := middleware(func(ctx *unicornContext.Context) error {
		return ctx.JSON(200, map[string]string{"status": "ok"})
	})

	ctx := unicornContext.New(context.Background())
	ctx.SetRequest(&unicornContext.Request{
		Method: "GET",
		Path:   "/api/test",
	})

	handler(ctx)

	// Check Set-Cookie header
	setCookie := ctx.Response().Header("Set-Cookie")
	if setCookie == "" {
		t.Error("Expected Set-Cookie header to be set")
	}

	if !contains(setCookie, "example.com") {
		t.Error("Cookie should contain domain")
	}

	if !contains(setCookie, "Secure") {
		t.Error("Cookie should be Secure")
	}

	if !contains(setCookie, "HttpOnly") {
		t.Error("Cookie should be HttpOnly")
	}

	if !contains(setCookie, "SameSite=Strict") {
		t.Error("Cookie should have SameSite=Strict")
	}
}
