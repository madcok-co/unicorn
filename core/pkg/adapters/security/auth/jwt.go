package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

var (
	ErrInvalidToken     = errors.New("invalid token")
	ErrExpiredToken     = errors.New("token has expired")
	ErrInvalidSignature = errors.New("invalid signature")
	ErrTokenRevoked     = errors.New("token has been revoked")
	ErrInvalidClaims    = errors.New("invalid token claims")
)

// JWTConfig adalah konfigurasi untuk JWT authenticator
type JWTConfig struct {
	// Secret key for signing (required for HS256)
	SecretKey string

	// Issuer claim
	Issuer string

	// Audience claim
	Audience string

	// Access token expiration
	AccessTokenTTL time.Duration

	// Refresh token expiration
	RefreshTokenTTL time.Duration

	// Algorithm (currently only HS256 supported)
	Algorithm string

	// Custom claims extractor
	ClaimsExtractor func(claims map[string]any) (*contracts.Identity, error)

	// Revocation cleanup interval (default: 1 hour)
	RevocationCleanupInterval time.Duration
}

// DefaultJWTConfig returns default configuration
func DefaultJWTConfig() *JWTConfig {
	return &JWTConfig{
		Algorithm:                 "HS256",
		AccessTokenTTL:            15 * time.Minute,
		RefreshTokenTTL:           7 * 24 * time.Hour,
		Issuer:                    "unicorn",
		RevocationCleanupInterval: 1 * time.Hour,
	}
}

// revokedEntry stores revocation info with expiration
type revokedEntry struct {
	revokedAt time.Time
	expiresAt time.Time
}

// JWTAuthenticator implements Authenticator using JWT
type JWTAuthenticator struct {
	config       *JWTConfig
	revokedStore sync.Map // token hash -> revokedEntry
	stopCh       chan struct{}
	stopOnce     sync.Once
}

// NewJWTAuthenticator creates a new JWT authenticator
func NewJWTAuthenticator(config *JWTConfig) *JWTAuthenticator {
	if config == nil {
		config = DefaultJWTConfig()
	}
	if config.Algorithm == "" {
		config.Algorithm = "HS256"
	}
	if config.AccessTokenTTL == 0 {
		config.AccessTokenTTL = 15 * time.Minute
	}
	if config.RefreshTokenTTL == 0 {
		config.RefreshTokenTTL = 7 * 24 * time.Hour
	}
	if config.RevocationCleanupInterval == 0 {
		config.RevocationCleanupInterval = 1 * time.Hour
	}

	j := &JWTAuthenticator{
		config: config,
		stopCh: make(chan struct{}),
	}

	// Start cleanup goroutine for revoked tokens
	go j.cleanupLoop()

	return j
}

// Close stops the JWT authenticator cleanup goroutine
func (j *JWTAuthenticator) Close() error {
	j.stopOnce.Do(func() {
		close(j.stopCh)
	})
	return nil
}

// cleanupLoop periodically removes expired revoked tokens
func (j *JWTAuthenticator) cleanupLoop() {
	ticker := time.NewTicker(j.config.RevocationCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			j.cleanupExpiredRevocations()
		case <-j.stopCh:
			return
		}
	}
}

// cleanupExpiredRevocations removes revoked tokens that have expired
func (j *JWTAuthenticator) cleanupExpiredRevocations() {
	now := time.Now()
	j.revokedStore.Range(func(key, value any) bool {
		entry := value.(revokedEntry)
		// Remove if the token would have expired anyway
		if now.After(entry.expiresAt) {
			j.revokedStore.Delete(key)
		}
		return true
	})
}

// Authenticate validates credentials and returns identity with tokens
func (j *JWTAuthenticator) Authenticate(ctx context.Context, credentials contracts.Credentials) (*contracts.Identity, error) {
	// JWT authenticator typically validates existing tokens
	// For username/password, you'd use a different authenticator or combine
	if credentials.Token != "" {
		return j.Validate(ctx, credentials.Token)
	}

	// If you need to issue tokens based on credentials,
	// use IssueTokens after validating credentials elsewhere
	return nil, errors.New("JWT authenticator requires a token; use IssueTokens to generate tokens after credential validation")
}

// Validate validates a JWT token and returns the identity
func (j *JWTAuthenticator) Validate(ctx context.Context, token string) (*contracts.Identity, error) {
	// Hash token for revocation check (don't store raw tokens)
	tokenHash := j.hashToken(token)

	// Check if token is revoked
	if _, revoked := j.revokedStore.Load(tokenHash); revoked {
		return nil, ErrTokenRevoked
	}

	// Parse and validate token
	claims, err := j.parseToken(token)
	if err != nil {
		return nil, err
	}

	// Check expiration
	if exp, ok := claims["exp"].(float64); ok {
		if time.Unix(int64(exp), 0).Before(time.Now()) {
			return nil, ErrExpiredToken
		}
	}

	// Check issuer
	if j.config.Issuer != "" {
		if iss, ok := claims["iss"].(string); !ok || iss != j.config.Issuer {
			return nil, ErrInvalidClaims
		}
	}

	// Check audience
	if j.config.Audience != "" {
		if aud, ok := claims["aud"].(string); !ok || aud != j.config.Audience {
			return nil, ErrInvalidClaims
		}
	}

	// Extract identity from claims
	identity, err := j.extractIdentity(claims)
	if err != nil {
		return nil, err
	}

	return identity, nil
}

// Refresh refreshes an expired token pair
func (j *JWTAuthenticator) Refresh(ctx context.Context, refreshToken string) (*contracts.TokenPair, error) {
	tokenHash := j.hashToken(refreshToken)

	// Check if refresh token is revoked
	if _, revoked := j.revokedStore.Load(tokenHash); revoked {
		return nil, ErrTokenRevoked
	}

	// Parse refresh token
	claims, err := j.parseToken(refreshToken)
	if err != nil {
		return nil, err
	}

	// Check if it's a refresh token
	if tokenType, ok := claims["type"].(string); !ok || tokenType != "refresh" {
		return nil, errors.New("not a refresh token")
	}

	// Check expiration
	var expiresAt time.Time
	if exp, ok := claims["exp"].(float64); ok {
		expiresAt = time.Unix(int64(exp), 0)
		if expiresAt.Before(time.Now()) {
			return nil, ErrExpiredToken
		}
	}

	// Revoke old refresh token with expiration info
	j.revokedStore.Store(tokenHash, revokedEntry{
		revokedAt: time.Now(),
		expiresAt: expiresAt,
	})

	// Extract identity and issue new tokens
	identity, err := j.extractIdentity(claims)
	if err != nil {
		return nil, err
	}

	return j.IssueTokens(identity)
}

// Revoke revokes a token
func (j *JWTAuthenticator) Revoke(ctx context.Context, token string) error {
	tokenHash := j.hashToken(token)

	// Try to parse token to get expiration
	expiresAt := time.Now().Add(j.config.RefreshTokenTTL) // Default fallback
	if claims, err := j.parseToken(token); err == nil {
		if exp, ok := claims["exp"].(float64); ok {
			expiresAt = time.Unix(int64(exp), 0)
		}
	}

	j.revokedStore.Store(tokenHash, revokedEntry{
		revokedAt: time.Now(),
		expiresAt: expiresAt,
	})
	return nil
}

// hashToken creates a hash of the token for storage
func (j *JWTAuthenticator) hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// IssueTokens creates a new token pair for the given identity
func (j *JWTAuthenticator) IssueTokens(identity *contracts.Identity) (*contracts.TokenPair, error) {
	now := time.Now()

	// Create access token claims
	accessClaims := map[string]any{
		"sub":    identity.ID,
		"type":   "access",
		"name":   identity.Name,
		"email":  identity.Email,
		"roles":  identity.Roles,
		"scopes": identity.Scopes,
		"iat":    now.Unix(),
		"exp":    now.Add(j.config.AccessTokenTTL).Unix(),
	}

	if j.config.Issuer != "" {
		accessClaims["iss"] = j.config.Issuer
	}
	if j.config.Audience != "" {
		accessClaims["aud"] = j.config.Audience
	}
	if identity.Metadata != nil {
		accessClaims["metadata"] = identity.Metadata
	}

	accessToken, err := j.createToken(accessClaims)
	if err != nil {
		return nil, fmt.Errorf("failed to create access token: %w", err)
	}

	// Create refresh token claims
	refreshClaims := map[string]any{
		"sub":  identity.ID,
		"type": "refresh",
		"iat":  now.Unix(),
		"exp":  now.Add(j.config.RefreshTokenTTL).Unix(),
	}

	if j.config.Issuer != "" {
		refreshClaims["iss"] = j.config.Issuer
	}

	refreshToken, err := j.createToken(refreshClaims)
	if err != nil {
		return nil, fmt.Errorf("failed to create refresh token: %w", err)
	}

	return &contracts.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(j.config.AccessTokenTTL.Seconds()),
		Scope:        strings.Join(identity.Scopes, " "),
	}, nil
}

// parseToken parses and validates a JWT token
func (j *JWTAuthenticator) parseToken(token string) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	// Decode header
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrInvalidToken
	}

	var header map[string]any
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, ErrInvalidToken
	}

	// Verify algorithm
	if alg, ok := header["alg"].(string); !ok || alg != j.config.Algorithm {
		return nil, fmt.Errorf("unsupported algorithm: %v", header["alg"])
	}

	// Verify signature using constant-time comparison to prevent timing attacks
	signatureInput := parts[0] + "." + parts[1]
	expectedSignature := j.sign(signatureInput)

	// Decode both signatures for constant-time comparison
	providedSig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrInvalidSignature
	}
	expectedSig, err := base64.RawURLEncoding.DecodeString(expectedSignature)
	if err != nil {
		return nil, ErrInvalidSignature
	}

	// Constant-time comparison to prevent timing attacks
	if subtle.ConstantTimeCompare(providedSig, expectedSig) != 1 {
		return nil, ErrInvalidSignature
	}

	// Decode payload
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}

	var claims map[string]any
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// createToken creates a JWT token from claims
func (j *JWTAuthenticator) createToken(claims map[string]any) (string, error) {
	// Create header
	header := map[string]string{
		"alg": j.config.Algorithm,
		"typ": "JWT",
	}

	headerBytes, err := json.Marshal(header)
	if err != nil {
		return "", err
	}

	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	// Encode
	headerEncoded := base64.RawURLEncoding.EncodeToString(headerBytes)
	payloadEncoded := base64.RawURLEncoding.EncodeToString(payloadBytes)

	// Sign
	signatureInput := headerEncoded + "." + payloadEncoded
	signature := j.sign(signatureInput)

	return signatureInput + "." + signature, nil
}

// sign creates HMAC-SHA256 signature
func (j *JWTAuthenticator) sign(input string) string {
	h := hmac.New(sha256.New, []byte(j.config.SecretKey))
	h.Write([]byte(input))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

// extractIdentity extracts identity from JWT claims
func (j *JWTAuthenticator) extractIdentity(claims map[string]any) (*contracts.Identity, error) {
	// Use custom extractor if provided
	if j.config.ClaimsExtractor != nil {
		return j.config.ClaimsExtractor(claims)
	}

	identity := &contracts.Identity{
		Metadata: make(map[string]any),
	}

	// Extract standard claims
	if sub, ok := claims["sub"].(string); ok {
		identity.ID = sub
	}
	if name, ok := claims["name"].(string); ok {
		identity.Name = name
	}
	if email, ok := claims["email"].(string); ok {
		identity.Email = email
	}

	// Extract roles
	if roles, ok := claims["roles"].([]any); ok {
		for _, r := range roles {
			if role, ok := r.(string); ok {
				identity.Roles = append(identity.Roles, role)
			}
		}
	}

	// Extract scopes
	if scopes, ok := claims["scopes"].([]any); ok {
		for _, s := range scopes {
			if scope, ok := s.(string); ok {
				identity.Scopes = append(identity.Scopes, scope)
			}
		}
	}

	// Extract metadata
	if metadata, ok := claims["metadata"].(map[string]any); ok {
		identity.Metadata = metadata
	}

	// Set token times
	if iat, ok := claims["iat"].(float64); ok {
		identity.IssuedAt = time.Unix(int64(iat), 0)
	}
	if exp, ok := claims["exp"].(float64); ok {
		identity.ExpiresAt = time.Unix(int64(exp), 0)
	}

	return identity, nil
}

// GetRevokedCount returns the number of revoked tokens (for monitoring)
func (j *JWTAuthenticator) GetRevokedCount() int {
	count := 0
	j.revokedStore.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}

// Ensure JWTAuthenticator implements Authenticator
var _ contracts.Authenticator = (*JWTAuthenticator)(nil)
