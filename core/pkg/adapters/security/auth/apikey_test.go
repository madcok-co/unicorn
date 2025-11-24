package auth

import (
	"context"
	"testing"
	"time"
)

func TestNewAPIKeyAuthenticator(t *testing.T) {
	t.Run("with nil config uses defaults", func(t *testing.T) {
		auth := NewAPIKeyAuthenticator(nil)

		if auth.config.KeyPrefix != "uk_" {
			t.Errorf("expected prefix uk_, got %s", auth.config.KeyPrefix)
		}
		if auth.config.KeyLength != 32 {
			t.Errorf("expected key length 32, got %d", auth.config.KeyLength)
		}
	})

	t.Run("with custom config", func(t *testing.T) {
		config := &APIKeyConfig{
			KeyPrefix:  "myapp_",
			KeyLength:  64,
			HeaderName: "X-Custom-Key",
		}
		auth := NewAPIKeyAuthenticator(config)

		if auth.config.KeyPrefix != "myapp_" {
			t.Errorf("expected prefix myapp_, got %s", auth.config.KeyPrefix)
		}
	})
}

func TestAPIKeyAuthenticator_CreateKey(t *testing.T) {
	auth := NewAPIKeyAuthenticator(&APIKeyConfig{
		KeyPrefix: "test_",
		KeyLength: 32,
	})

	t.Run("creates valid key", func(t *testing.T) {
		opts := CreateKeyOptions{
			Name:      "Test Key",
			OwnerID:   "user-123",
			OwnerType: "user",
			Roles:     []string{"admin"},
			Scopes:    []string{"read", "write"},
		}

		apiKey, entry, err := auth.CreateKey(context.Background(), opts)
		if err != nil {
			t.Fatalf("failed to create key: %v", err)
		}

		// Check key format
		if len(apiKey) < 10 {
			t.Error("API key is too short")
		}
		if apiKey[:5] != "test_" {
			t.Errorf("expected prefix test_, got %s", apiKey[:5])
		}

		// Check entry
		if entry.Name != "Test Key" {
			t.Errorf("expected name Test Key, got %s", entry.Name)
		}
		if entry.OwnerID != "user-123" {
			t.Errorf("expected owner user-123, got %s", entry.OwnerID)
		}
		if len(entry.Roles) != 1 || entry.Roles[0] != "admin" {
			t.Errorf("expected roles [admin], got %v", entry.Roles)
		}
	})

	t.Run("creates key with expiration", func(t *testing.T) {
		expiresAt := time.Now().Add(24 * time.Hour)
		opts := CreateKeyOptions{
			Name:      "Expiring Key",
			OwnerID:   "user-456",
			OwnerType: "user",
			ExpiresAt: &expiresAt,
		}

		_, entry, err := auth.CreateKey(context.Background(), opts)
		if err != nil {
			t.Fatalf("failed to create key: %v", err)
		}

		if entry.ExpiresAt == nil {
			t.Error("expected expiration to be set")
		}
	})
}

func TestAPIKeyAuthenticator_Validate(t *testing.T) {
	auth := NewAPIKeyAuthenticator(&APIKeyConfig{
		KeyPrefix:  "test_",
		TrackUsage: true,
	})

	opts := CreateKeyOptions{
		Name:      "Test Key",
		OwnerID:   "user-123",
		OwnerType: "user",
		Roles:     []string{"admin"},
		Scopes:    []string{"read"},
	}

	apiKey, _, _ := auth.CreateKey(context.Background(), opts)

	t.Run("validates valid key", func(t *testing.T) {
		identity, err := auth.Validate(context.Background(), apiKey)
		if err != nil {
			t.Fatalf("failed to validate: %v", err)
		}

		if identity.ID != "user-123" {
			t.Errorf("expected ID user-123, got %s", identity.ID)
		}
		if identity.Type != "user" {
			t.Errorf("expected type user, got %s", identity.Type)
		}
		if len(identity.Roles) != 1 || identity.Roles[0] != "admin" {
			t.Errorf("expected roles [admin], got %v", identity.Roles)
		}
	})

	t.Run("rejects invalid key", func(t *testing.T) {
		_, err := auth.Validate(context.Background(), "invalid_key_12345")
		if err == nil {
			t.Error("expected error for invalid key")
		}
	})

	t.Run("rejects non-existent key", func(t *testing.T) {
		_, err := auth.Validate(context.Background(), "test_aaaabbbbccccddddeeeeffffgggghhhh")
		if err != ErrAPIKeyNotFound {
			t.Errorf("expected ErrAPIKeyNotFound, got %v", err)
		}
	})
}

func TestAPIKeyAuthenticator_Revoke(t *testing.T) {
	auth := NewAPIKeyAuthenticator(nil)

	opts := CreateKeyOptions{
		Name:      "Test Key",
		OwnerID:   "user-123",
		OwnerType: "user",
	}

	apiKey, _, _ := auth.CreateKey(context.Background(), opts)

	t.Run("revokes key successfully", func(t *testing.T) {
		// Key should be valid before revocation
		_, err := auth.Validate(context.Background(), apiKey)
		if err != nil {
			t.Fatalf("key should be valid: %v", err)
		}

		// Revoke the key
		err = auth.Revoke(context.Background(), apiKey)
		if err != nil {
			t.Fatalf("failed to revoke: %v", err)
		}

		// Key should be invalid after revocation
		_, err = auth.Validate(context.Background(), apiKey)
		if err != ErrAPIKeyRevoked {
			t.Errorf("expected ErrAPIKeyRevoked, got %v", err)
		}
	})
}

func TestAPIKeyAuthenticator_ExpiredKey(t *testing.T) {
	auth := NewAPIKeyAuthenticator(nil)

	// Create key that expires immediately
	expiresAt := time.Now().Add(-1 * time.Hour) // Already expired
	opts := CreateKeyOptions{
		Name:      "Expired Key",
		OwnerID:   "user-123",
		OwnerType: "user",
		ExpiresAt: &expiresAt,
	}

	apiKey, _, _ := auth.CreateKey(context.Background(), opts)

	_, err := auth.Validate(context.Background(), apiKey)
	if err != ErrAPIKeyExpired {
		t.Errorf("expected ErrAPIKeyExpired, got %v", err)
	}
}

func TestAPIKeyAuthenticator_ValidateKeyFormat(t *testing.T) {
	auth := NewAPIKeyAuthenticator(&APIKeyConfig{
		KeyPrefix: "myapp_",
		KeyLength: 32,
	})

	tests := []struct {
		name     string
		key      string
		expected bool
	}{
		{"valid format", "myapp_" + string(make([]byte, 64)), true},
		{"wrong prefix", "other_" + string(make([]byte, 64)), false},
		{"too short", "myapp_short", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := auth.ValidateKeyFormat(tt.key)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestAPIKeyAuthenticator_Concurrent(t *testing.T) {
	auth := NewAPIKeyAuthenticator(nil)

	// Create multiple keys concurrently
	done := make(chan bool)
	for i := 0; i < 50; i++ {
		go func(idx int) {
			opts := CreateKeyOptions{
				Name:      "Concurrent Key",
				OwnerID:   "user-concurrent",
				OwnerType: "user",
			}

			apiKey, _, err := auth.CreateKey(context.Background(), opts)
			if err != nil {
				t.Errorf("failed to create key: %v", err)
			}

			_, err = auth.Validate(context.Background(), apiKey)
			if err != nil {
				t.Errorf("failed to validate key: %v", err)
			}

			done <- true
		}(i)
	}

	for i := 0; i < 50; i++ {
		<-done
	}
}
