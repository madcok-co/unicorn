package oauth2

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

func TestNewDriver(t *testing.T) {
	cfg := &Config{
		Provider:     ProviderGoogle,
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURL:  "http://localhost:8080/callback",
		Scopes:       []string{"email", "profile"},
	}

	driver := NewDriver(cfg)

	if driver == nil {
		t.Fatal("expected driver to be non-nil")
	}

	if driver.config.Provider != ProviderGoogle {
		t.Errorf("expected provider to be google, got %s", driver.config.Provider)
	}

	if driver.oauth2Config.ClientID != "test-client-id" {
		t.Errorf("expected client ID to be test-client-id, got %s", driver.oauth2Config.ClientID)
	}
}

func TestGetAuthURL(t *testing.T) {
	cfg := &Config{
		Provider:     ProviderGoogle,
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURL:  "http://localhost:8080/callback",
		Scopes:       []string{"email", "profile"},
	}

	driver := NewDriver(cfg)
	authURL := driver.GetAuthURL("random-state")

	if authURL == "" {
		t.Fatal("expected auth URL to be non-empty")
	}

	if !contains(authURL, "client_id=test-client-id") {
		t.Error("expected auth URL to contain client_id")
	}

	if !contains(authURL, "state=random-state") {
		t.Error("expected auth URL to contain state")
	}
}

func TestValidate_Google(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth2/v2/userinfo" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			t.Errorf("unexpected authorization header: %s", auth)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":      "123",
			"email":   "test@example.com",
			"name":    "Test User",
			"picture": "https://example.com/avatar.jpg",
		})
	}))
	defer server.Close()

	cfg := &Config{
		Provider:     ProviderGoogle,
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		UserInfoURL:  server.URL + "/oauth2/v2/userinfo",
	}

	driver := NewDriver(cfg)

	identity, err := driver.Validate(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if identity.ID != "123" {
		t.Errorf("expected ID to be 123, got %s", identity.ID)
	}

	if identity.Email != "test@example.com" {
		t.Errorf("expected email to be test@example.com, got %s", identity.Email)
	}

	if identity.Name != "Test User" {
		t.Errorf("expected name to be Test User, got %s", identity.Name)
	}

	if identity.Type != "user" {
		t.Errorf("expected type to be user, got %s", identity.Type)
	}
}

func TestValidate_GitHub(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		auth := r.Header.Get("Authorization")
		if auth != "Bearer github-token" {
			t.Errorf("unexpected authorization header: %s", auth)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":         456,
			"login":      "testuser",
			"email":      "github@example.com",
			"name":       "GitHub User",
			"avatar_url": "https://github.com/avatar.jpg",
		})
	}))
	defer server.Close()

	cfg := &Config{
		Provider:     ProviderGitHub,
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		UserInfoURL:  server.URL + "/user",
	}

	driver := NewDriver(cfg)

	identity, err := driver.Validate(context.Background(), "github-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if identity.ID != "456" {
		t.Errorf("expected ID to be 456, got %s", identity.ID)
	}

	if identity.Email != "github@example.com" {
		t.Errorf("expected email to be github@example.com, got %s", identity.Email)
	}

	if identity.Name != "GitHub User" {
		t.Errorf("expected name to be GitHub User, got %s", identity.Name)
	}
}

func TestValidate_Microsoft(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1.0/me" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":                "ms-123",
			"userPrincipalName": "user@company.com",
			"displayName":       "Microsoft User",
			"mail":              "ms@example.com",
		})
	}))
	defer server.Close()

	cfg := &Config{
		Provider:     ProviderMicrosoft,
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		UserInfoURL:  server.URL + "/v1.0/me",
	}

	driver := NewDriver(cfg)

	identity, err := driver.Validate(context.Background(), "ms-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if identity.ID != "ms-123" {
		t.Errorf("expected ID to be ms-123, got %s", identity.ID)
	}

	if identity.Email != "ms@example.com" {
		t.Errorf("expected email to be ms@example.com, got %s", identity.Email)
	}
}

func TestValidate_InvalidToken(t *testing.T) {
	// Create mock server that returns 401
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid_token"}`))
	}))
	defer server.Close()

	cfg := &Config{
		Provider:     ProviderGoogle,
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		UserInfoURL:  server.URL + "/userinfo",
	}

	driver := NewDriver(cfg)

	_, err := driver.Validate(context.Background(), "invalid-token")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestAuthenticate_UnsupportedType(t *testing.T) {
	cfg := &Config{
		Provider:     ProviderGoogle,
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
	}

	driver := NewDriver(cfg)

	creds := contracts.Credentials{
		Type:     "jwt",
		Username: "test",
		Password: "test",
	}

	_, err := driver.Authenticate(context.Background(), creds)
	if err == nil {
		t.Fatal("expected error for unsupported credential type")
	}

	if !contains(err.Error(), "unsupported credential type") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGetString(t *testing.T) {
	tests := []struct {
		name     string
		m        map[string]any
		keys     []string
		expected string
	}{
		{
			name:     "first key exists",
			m:        map[string]any{"name": "John", "display_name": "Johnny"},
			keys:     []string{"name", "display_name"},
			expected: "John",
		},
		{
			name:     "second key exists",
			m:        map[string]any{"display_name": "Johnny"},
			keys:     []string{"name", "display_name"},
			expected: "Johnny",
		},
		{
			name:     "no keys exist",
			m:        map[string]any{"other": "value"},
			keys:     []string{"name", "display_name"},
			expected: "",
		},
		{
			name:     "value is not string",
			m:        map[string]any{"name": 123},
			keys:     []string{"name"},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getString(tt.m, tt.keys...)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestProviders(t *testing.T) {
	tests := []struct {
		provider     Provider
		expectedAuth string
		expectedUser string
	}{
		{
			provider:     ProviderGoogle,
			expectedAuth: "https://accounts.google.com/o/oauth2/auth",
			expectedUser: "https://www.googleapis.com/oauth2/v2/userinfo",
		},
		{
			provider:     ProviderGitHub,
			expectedAuth: "https://github.com/login/oauth/authorize",
			expectedUser: "https://api.github.com/user",
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			cfg := &Config{
				Provider:     tt.provider,
				ClientID:     "test-id",
				ClientSecret: "test-secret",
			}

			driver := NewDriver(cfg)

			if driver.oauth2Config.Endpoint.AuthURL != tt.expectedAuth {
				t.Errorf("expected auth URL %s, got %s", tt.expectedAuth, driver.oauth2Config.Endpoint.AuthURL)
			}

			if driver.config.UserInfoURL != tt.expectedUser {
				t.Errorf("expected user info URL %s, got %s", tt.expectedUser, driver.config.UserInfoURL)
			}
		})
	}
}

func TestRefresh(t *testing.T) {
	// Create mock token server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/token" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "new-access-token",
			"refresh_token": "new-refresh-token",
			"token_type":    "Bearer",
			"expires_in":    3600,
		})
	}))
	defer server.Close()

	cfg := &Config{
		Provider:     ProviderGeneric,
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		TokenURL:     server.URL + "/token",
	}

	driver := NewDriver(cfg)

	tokenPair, err := driver.Refresh(context.Background(), "old-refresh-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tokenPair.AccessToken != "new-access-token" {
		t.Errorf("expected access token to be new-access-token, got %s", tokenPair.AccessToken)
	}

	if tokenPair.RefreshToken != "new-refresh-token" {
		t.Errorf("expected refresh token to be new-refresh-token, got %s", tokenPair.RefreshToken)
	}

	if tokenPair.TokenType != "Bearer" {
		t.Errorf("expected token type to be Bearer, got %s", tokenPair.TokenType)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Provider != ProviderGoogle {
		t.Errorf("expected default provider to be google, got %s", cfg.Provider)
	}

	if len(cfg.Scopes) != 2 {
		t.Errorf("expected 2 default scopes, got %d", len(cfg.Scopes))
	}

	if !cfg.ValidateToken {
		t.Error("expected ValidateToken to be true")
	}

	if cfg.CacheTTL != 5*time.Minute {
		t.Errorf("expected CacheTTL to be 5 minutes, got %v", cfg.CacheTTL)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsInside(s, substr)))
}

func containsInside(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
