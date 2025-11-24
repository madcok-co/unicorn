package auth

import (
	"context"
	"testing"
	"time"

	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

func TestNewJWTAuthenticator(t *testing.T) {
	t.Run("with nil config uses defaults", func(t *testing.T) {
		auth := NewJWTAuthenticator(nil)
		defer auth.Close()

		if auth.config.Algorithm != "HS256" {
			t.Errorf("expected algorithm HS256, got %s", auth.config.Algorithm)
		}
		if auth.config.AccessTokenTTL != 15*time.Minute {
			t.Errorf("expected access token TTL 15m, got %v", auth.config.AccessTokenTTL)
		}
	})

	t.Run("with custom config", func(t *testing.T) {
		config := &JWTConfig{
			SecretKey:      "test-secret",
			Issuer:         "test-issuer",
			AccessTokenTTL: 30 * time.Minute,
		}
		auth := NewJWTAuthenticator(config)
		defer auth.Close()

		if auth.config.Issuer != "test-issuer" {
			t.Errorf("expected issuer test-issuer, got %s", auth.config.Issuer)
		}
	})
}

func TestJWTAuthenticator_IssueTokens(t *testing.T) {
	auth := NewJWTAuthenticator(&JWTConfig{
		SecretKey:       "super-secret-key-for-testing",
		Issuer:          "test",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 24 * time.Hour,
	})
	defer auth.Close()

	identity := &contracts.Identity{
		ID:     "user-123",
		Name:   "John Doe",
		Email:  "john@example.com",
		Roles:  []string{"admin", "user"},
		Scopes: []string{"read", "write"},
	}

	t.Run("issues valid token pair", func(t *testing.T) {
		tokens, err := auth.IssueTokens(identity)
		if err != nil {
			t.Fatalf("failed to issue tokens: %v", err)
		}

		if tokens.AccessToken == "" {
			t.Error("access token is empty")
		}
		if tokens.RefreshToken == "" {
			t.Error("refresh token is empty")
		}
		if tokens.TokenType != "Bearer" {
			t.Errorf("expected token type Bearer, got %s", tokens.TokenType)
		}
		if tokens.ExpiresIn != 900 { // 15 minutes in seconds
			t.Errorf("expected expires_in 900, got %d", tokens.ExpiresIn)
		}
	})
}

func TestJWTAuthenticator_Validate(t *testing.T) {
	auth := NewJWTAuthenticator(&JWTConfig{
		SecretKey:      "super-secret-key-for-testing",
		Issuer:         "test",
		AccessTokenTTL: 15 * time.Minute,
	})
	defer auth.Close()

	identity := &contracts.Identity{
		ID:     "user-123",
		Name:   "John Doe",
		Email:  "john@example.com",
		Roles:  []string{"admin"},
		Scopes: []string{"read"},
	}

	t.Run("validates valid token", func(t *testing.T) {
		tokens, _ := auth.IssueTokens(identity)

		validated, err := auth.Validate(context.Background(), tokens.AccessToken)
		if err != nil {
			t.Fatalf("failed to validate token: %v", err)
		}

		if validated.ID != identity.ID {
			t.Errorf("expected ID %s, got %s", identity.ID, validated.ID)
		}
		if validated.Name != identity.Name {
			t.Errorf("expected name %s, got %s", identity.Name, validated.Name)
		}
		if len(validated.Roles) != 1 || validated.Roles[0] != "admin" {
			t.Errorf("expected roles [admin], got %v", validated.Roles)
		}
	})

	t.Run("rejects invalid token", func(t *testing.T) {
		_, err := auth.Validate(context.Background(), "invalid.token.here")
		if err == nil {
			t.Error("expected error for invalid token")
		}
	})

	t.Run("rejects tampered token", func(t *testing.T) {
		tokens, _ := auth.IssueTokens(identity)
		// Tamper with the signature
		tamperedToken := tokens.AccessToken[:len(tokens.AccessToken)-5] + "xxxxx"

		_, err := auth.Validate(context.Background(), tamperedToken)
		if err != ErrInvalidSignature {
			t.Errorf("expected ErrInvalidSignature, got %v", err)
		}
	})

	t.Run("rejects wrong issuer", func(t *testing.T) {
		// Create token with different issuer
		otherAuth := NewJWTAuthenticator(&JWTConfig{
			SecretKey: "super-secret-key-for-testing",
			Issuer:    "other-issuer",
		})
		defer otherAuth.Close()

		tokens, _ := otherAuth.IssueTokens(identity)

		_, err := auth.Validate(context.Background(), tokens.AccessToken)
		if err != ErrInvalidClaims {
			t.Errorf("expected ErrInvalidClaims, got %v", err)
		}
	})
}

func TestJWTAuthenticator_Revoke(t *testing.T) {
	auth := NewJWTAuthenticator(&JWTConfig{
		SecretKey:      "super-secret-key-for-testing",
		AccessTokenTTL: 15 * time.Minute,
	})
	defer auth.Close()

	identity := &contracts.Identity{ID: "user-123"}
	tokens, _ := auth.IssueTokens(identity)

	t.Run("revokes token successfully", func(t *testing.T) {
		// Token should be valid before revocation
		_, err := auth.Validate(context.Background(), tokens.AccessToken)
		if err != nil {
			t.Fatalf("token should be valid: %v", err)
		}

		// Revoke the token
		err = auth.Revoke(context.Background(), tokens.AccessToken)
		if err != nil {
			t.Fatalf("failed to revoke: %v", err)
		}

		// Token should be invalid after revocation
		_, err = auth.Validate(context.Background(), tokens.AccessToken)
		if err != ErrTokenRevoked {
			t.Errorf("expected ErrTokenRevoked, got %v", err)
		}
	})
}

func TestJWTAuthenticator_Refresh(t *testing.T) {
	identity := &contracts.Identity{
		ID:   "user-123",
		Name: "John Doe",
	}

	t.Run("refreshes token successfully", func(t *testing.T) {
		auth := NewJWTAuthenticator(&JWTConfig{
			SecretKey:       "super-secret-key-for-testing",
			AccessTokenTTL:  15 * time.Minute,
			RefreshTokenTTL: 24 * time.Hour,
		})
		defer auth.Close()

		tokens, _ := auth.IssueTokens(identity)

		// Small delay to ensure different timestamp
		time.Sleep(10 * time.Millisecond)

		newTokens, err := auth.Refresh(context.Background(), tokens.RefreshToken)
		if err != nil {
			t.Fatalf("failed to refresh: %v", err)
		}

		if newTokens.AccessToken == tokens.AccessToken {
			t.Error("new access token should be different")
		}
		// Note: refresh tokens may be same if issued in same second due to same claims
	})

	t.Run("old refresh token is revoked after use", func(t *testing.T) {
		auth := NewJWTAuthenticator(&JWTConfig{
			SecretKey:       "super-secret-key-for-testing-2",
			AccessTokenTTL:  15 * time.Minute,
			RefreshTokenTTL: 24 * time.Hour,
		})
		defer auth.Close()

		tokens, _ := auth.IssueTokens(identity)

		_, err := auth.Refresh(context.Background(), tokens.RefreshToken)
		if err != nil {
			t.Fatalf("first refresh failed: %v", err)
		}

		// Try to use old refresh token again
		_, err = auth.Refresh(context.Background(), tokens.RefreshToken)
		if err != ErrTokenRevoked {
			t.Errorf("expected ErrTokenRevoked, got %v", err)
		}
	})

	t.Run("rejects access token for refresh", func(t *testing.T) {
		auth := NewJWTAuthenticator(&JWTConfig{
			SecretKey:       "super-secret-key-for-testing-3",
			AccessTokenTTL:  15 * time.Minute,
			RefreshTokenTTL: 24 * time.Hour,
		})
		defer auth.Close()

		tokens, _ := auth.IssueTokens(identity)

		_, err := auth.Refresh(context.Background(), tokens.AccessToken)
		if err == nil {
			t.Error("expected error when using access token for refresh")
		}
	})
}

func TestJWTAuthenticator_ExpiredToken(t *testing.T) {
	auth := NewJWTAuthenticator(&JWTConfig{
		SecretKey:      "super-secret-key-for-testing",
		AccessTokenTTL: 1 * time.Millisecond, // Very short TTL
	})
	defer auth.Close()

	identity := &contracts.Identity{ID: "user-123"}
	tokens, _ := auth.IssueTokens(identity)

	// Wait for token to expire
	time.Sleep(10 * time.Millisecond)

	_, err := auth.Validate(context.Background(), tokens.AccessToken)
	if err != ErrExpiredToken {
		t.Errorf("expected ErrExpiredToken, got %v", err)
	}
}

func TestJWTAuthenticator_Concurrent(t *testing.T) {
	auth := NewJWTAuthenticator(&JWTConfig{
		SecretKey:      "super-secret-key-for-testing",
		AccessTokenTTL: 15 * time.Minute,
	})
	defer auth.Close()

	identity := &contracts.Identity{ID: "user-123"}

	// Test concurrent token operations
	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func() {
			tokens, err := auth.IssueTokens(identity)
			if err != nil {
				t.Errorf("failed to issue: %v", err)
			}

			_, err = auth.Validate(context.Background(), tokens.AccessToken)
			if err != nil {
				t.Errorf("failed to validate: %v", err)
			}

			done <- true
		}()
	}

	for i := 0; i < 100; i++ {
		<-done
	}
}
