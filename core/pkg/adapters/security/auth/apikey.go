package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

var (
	ErrAPIKeyNotFound    = errors.New("API key not found")
	ErrAPIKeyExpired     = errors.New("API key has expired")
	ErrAPIKeyRevoked     = errors.New("API key has been revoked")
	ErrInvalidAPIKey     = errors.New("invalid API key format")
	ErrUnauthorizedScope = errors.New("API key does not have required scope")
)

// APIKeyEntry represents a stored API key
type APIKeyEntry struct {
	ID         string
	KeyHash    string // Hashed key for security
	Name       string // Human-readable name
	OwnerID    string // User/Service that owns this key
	OwnerType  string // "user", "service"
	Roles      []string
	Scopes     []string
	Metadata   map[string]any
	CreatedAt  time.Time
	ExpiresAt  *time.Time // nil = never expires
	LastUsedAt *time.Time
	Revoked    bool
}

// APIKeyStore is the interface for storing API keys
type APIKeyStore interface {
	// Get retrieves an API key entry by key hash
	GetByHash(ctx context.Context, keyHash string) (*APIKeyEntry, error)

	// Save stores an API key entry
	Save(ctx context.Context, entry *APIKeyEntry) error

	// Delete removes an API key entry
	Delete(ctx context.Context, id string) error

	// UpdateLastUsed updates the last used timestamp
	UpdateLastUsed(ctx context.Context, id string, t time.Time) error
}

// InMemoryAPIKeyStore is a simple in-memory store for API keys
type InMemoryAPIKeyStore struct {
	mu      sync.RWMutex
	entries map[string]*APIKeyEntry // keyHash -> entry
	byID    map[string]*APIKeyEntry // id -> entry
}

// NewInMemoryAPIKeyStore creates a new in-memory API key store
func NewInMemoryAPIKeyStore() *InMemoryAPIKeyStore {
	return &InMemoryAPIKeyStore{
		entries: make(map[string]*APIKeyEntry),
		byID:    make(map[string]*APIKeyEntry),
	}
}

func (s *InMemoryAPIKeyStore) GetByHash(ctx context.Context, keyHash string) (*APIKeyEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.entries[keyHash]
	if !ok {
		return nil, ErrAPIKeyNotFound
	}
	return entry, nil
}

func (s *InMemoryAPIKeyStore) Save(ctx context.Context, entry *APIKeyEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries[entry.KeyHash] = entry
	s.byID[entry.ID] = entry
	return nil
}

func (s *InMemoryAPIKeyStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.byID[id]
	if !ok {
		return ErrAPIKeyNotFound
	}

	delete(s.entries, entry.KeyHash)
	delete(s.byID, id)
	return nil
}

func (s *InMemoryAPIKeyStore) UpdateLastUsed(ctx context.Context, id string, t time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.byID[id]
	if !ok {
		return ErrAPIKeyNotFound
	}

	entry.LastUsedAt = &t
	return nil
}

// APIKeyConfig adalah konfigurasi untuk API Key authenticator
type APIKeyConfig struct {
	// Store for API keys
	Store APIKeyStore

	// Prefix for generated keys (e.g., "uk_" for unicorn key)
	KeyPrefix string

	// Key length in bytes (before hex encoding)
	KeyLength int

	// Track last used timestamp
	TrackUsage bool

	// Header name for API key
	HeaderName string

	// Query parameter name for API key
	QueryParam string
}

// DefaultAPIKeyConfig returns default configuration
func DefaultAPIKeyConfig() *APIKeyConfig {
	return &APIKeyConfig{
		Store:      NewInMemoryAPIKeyStore(),
		KeyPrefix:  "uk_",
		KeyLength:  32,
		TrackUsage: true,
		HeaderName: "X-API-Key",
		QueryParam: "api_key",
	}
}

// APIKeyAuthenticator implements Authenticator using API keys
type APIKeyAuthenticator struct {
	config *APIKeyConfig
}

// NewAPIKeyAuthenticator creates a new API key authenticator
func NewAPIKeyAuthenticator(config *APIKeyConfig) *APIKeyAuthenticator {
	if config == nil {
		config = DefaultAPIKeyConfig()
	}
	if config.Store == nil {
		config.Store = NewInMemoryAPIKeyStore()
	}
	if config.KeyPrefix == "" {
		config.KeyPrefix = "uk_"
	}
	if config.KeyLength == 0 {
		config.KeyLength = 32
	}
	if config.HeaderName == "" {
		config.HeaderName = "X-API-Key"
	}

	return &APIKeyAuthenticator{
		config: config,
	}
}

// Authenticate validates API key credentials
func (a *APIKeyAuthenticator) Authenticate(ctx context.Context, credentials contracts.Credentials) (*contracts.Identity, error) {
	apiKey := credentials.APIKey
	if apiKey == "" {
		apiKey = credentials.Token // Fall back to token field
	}
	if apiKey == "" {
		return nil, ErrInvalidAPIKey
	}

	return a.Validate(ctx, apiKey)
}

// Validate validates an API key and returns the identity
func (a *APIKeyAuthenticator) Validate(ctx context.Context, apiKey string) (*contracts.Identity, error) {
	// Hash the key for lookup
	keyHash := a.hashKey(apiKey)

	// Look up the key
	entry, err := a.config.Store.GetByHash(ctx, keyHash)
	if err != nil {
		return nil, err
	}

	// Check if revoked
	if entry.Revoked {
		return nil, ErrAPIKeyRevoked
	}

	// Check expiration
	if entry.ExpiresAt != nil && entry.ExpiresAt.Before(time.Now()) {
		return nil, ErrAPIKeyExpired
	}

	// Update last used timestamp
	if a.config.TrackUsage {
		now := time.Now()
		_ = a.config.Store.UpdateLastUsed(ctx, entry.ID, now)
	}

	// Build identity
	identity := &contracts.Identity{
		ID:       entry.OwnerID,
		Type:     entry.OwnerType,
		Name:     entry.Name,
		Roles:    entry.Roles,
		Scopes:   entry.Scopes,
		Metadata: entry.Metadata,
		TokenID:  entry.ID,
		IssuedAt: entry.CreatedAt,
	}

	if entry.ExpiresAt != nil {
		identity.ExpiresAt = *entry.ExpiresAt
	}

	return identity, nil
}

// Refresh is not supported for API keys
func (a *APIKeyAuthenticator) Refresh(ctx context.Context, refreshToken string) (*contracts.TokenPair, error) {
	return nil, errors.New("API keys do not support refresh; create a new key instead")
}

// Revoke revokes an API key
func (a *APIKeyAuthenticator) Revoke(ctx context.Context, apiKey string) error {
	keyHash := a.hashKey(apiKey)

	entry, err := a.config.Store.GetByHash(ctx, keyHash)
	if err != nil {
		return err
	}

	entry.Revoked = true
	return a.config.Store.Save(ctx, entry)
}

// CreateKey creates a new API key
func (a *APIKeyAuthenticator) CreateKey(ctx context.Context, opts CreateKeyOptions) (string, *APIKeyEntry, error) {
	// Generate random key
	keyBytes := make([]byte, a.config.KeyLength)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", nil, err
	}

	// Create the full key with prefix
	apiKey := a.config.KeyPrefix + hex.EncodeToString(keyBytes)

	// Generate ID
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return "", nil, err
	}

	entry := &APIKeyEntry{
		ID:        hex.EncodeToString(idBytes),
		KeyHash:   a.hashKey(apiKey),
		Name:      opts.Name,
		OwnerID:   opts.OwnerID,
		OwnerType: opts.OwnerType,
		Roles:     opts.Roles,
		Scopes:    opts.Scopes,
		Metadata:  opts.Metadata,
		CreatedAt: time.Now(),
		ExpiresAt: opts.ExpiresAt,
		Revoked:   false,
	}

	if err := a.config.Store.Save(ctx, entry); err != nil {
		return "", nil, err
	}

	// Return the plain key (only time it's visible)
	return apiKey, entry, nil
}

// CreateKeyOptions are options for creating a new API key
type CreateKeyOptions struct {
	Name      string
	OwnerID   string
	OwnerType string
	Roles     []string
	Scopes    []string
	Metadata  map[string]any
	ExpiresAt *time.Time
}

// ValidateKeyFormat checks if a key has the correct format
func (a *APIKeyAuthenticator) ValidateKeyFormat(apiKey string) bool {
	if len(apiKey) < len(a.config.KeyPrefix)+a.config.KeyLength*2 {
		return false
	}

	prefix := apiKey[:len(a.config.KeyPrefix)]
	return subtle.ConstantTimeCompare([]byte(prefix), []byte(a.config.KeyPrefix)) == 1
}

// hashKey creates a SHA-256 hash of the API key
func (a *APIKeyAuthenticator) hashKey(apiKey string) string {
	hash := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(hash[:])
}

// GetHeaderName returns the header name for API key
func (a *APIKeyAuthenticator) GetHeaderName() string {
	return a.config.HeaderName
}

// GetQueryParam returns the query parameter name for API key
func (a *APIKeyAuthenticator) GetQueryParam() string {
	return a.config.QueryParam
}

// Ensure APIKeyAuthenticator implements Authenticator
var _ contracts.Authenticator = (*APIKeyAuthenticator)(nil)
