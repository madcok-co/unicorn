package contracts

import (
	"context"
	"crypto/tls"
	"time"
)

// ============ Authentication ============

// Authenticator adalah generic interface untuk authentication
// Implementasi bisa JWT, OAuth2, API Key, Basic Auth, dll
type Authenticator interface {
	// Authenticate validates credentials and returns identity
	Authenticate(ctx context.Context, credentials Credentials) (*Identity, error)

	// Validate validates a token/session
	Validate(ctx context.Context, token string) (*Identity, error)

	// Refresh refreshes an expired token
	Refresh(ctx context.Context, refreshToken string) (*TokenPair, error)

	// Revoke revokes a token
	Revoke(ctx context.Context, token string) error
}

// Credentials represents authentication credentials
type Credentials struct {
	Type string // "jwt", "api_key", "basic", "oauth2"

	// Basic auth
	Username string
	Password string

	// Token-based
	Token        string
	RefreshToken string

	// API Key
	APIKey    string
	APISecret string

	// OAuth2
	ClientID     string
	ClientSecret string
	Code         string
	RedirectURI  string

	// Additional data
	Metadata map[string]string
}

// Identity represents authenticated user/service identity
type Identity struct {
	ID       string   // User/Service ID
	Type     string   // "user", "service", "api_key"
	Name     string   // Display name
	Email    string   // Email if applicable
	Roles    []string // Assigned roles
	Scopes   []string // Granted scopes/permissions
	Metadata map[string]any

	// Token info
	TokenID   string
	ExpiresAt time.Time
	IssuedAt  time.Time
}

// HasRole checks if identity has a role
func (i *Identity) HasRole(role string) bool {
	for _, r := range i.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// HasScope checks if identity has a scope
func (i *Identity) HasScope(scope string) bool {
	for _, s := range i.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// HasAnyRole checks if identity has any of the roles
func (i *Identity) HasAnyRole(roles ...string) bool {
	for _, role := range roles {
		if i.HasRole(role) {
			return true
		}
	}
	return false
}

// HasAllScopes checks if identity has all scopes
func (i *Identity) HasAllScopes(scopes ...string) bool {
	for _, scope := range scopes {
		if !i.HasScope(scope) {
			return false
		}
	}
	return true
}

// TokenPair represents access and refresh tokens
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	TokenType    string // "Bearer"
	ExpiresIn    int64  // seconds
	Scope        string
}

// ============ Authorization ============

// Authorizer adalah interface untuk authorization
type Authorizer interface {
	// Authorize checks if identity can perform action on resource
	Authorize(ctx context.Context, identity *Identity, action, resource string) (bool, error)

	// AuthorizeAll checks multiple permissions
	AuthorizeAll(ctx context.Context, identity *Identity, permissions []Permission) (bool, error)
}

// Permission represents a single permission check
type Permission struct {
	Action   string // "read", "write", "delete", "admin"
	Resource string // "users", "orders", "users:123"
}

// ============ Secret Management ============

// SecretManager adalah interface untuk manage secrets
// Implementasi bisa Vault, AWS Secrets Manager, GCP Secret Manager, env vars
type SecretManager interface {
	// Get retrieves a secret
	Get(ctx context.Context, key string) (string, error)

	// GetJSON retrieves and unmarshals a JSON secret
	GetJSON(ctx context.Context, key string, dest any) error

	// Set stores a secret (if supported)
	Set(ctx context.Context, key, value string) error

	// Delete removes a secret (if supported)
	Delete(ctx context.Context, key string) error

	// List lists available secrets (if supported)
	List(ctx context.Context, prefix string) ([]string, error)

	// Watch watches for secret changes (if supported)
	Watch(ctx context.Context, key string, callback func(newValue string)) error
}

// SecretManagerConfig untuk konfigurasi
type SecretManagerConfig struct {
	Provider string // "vault", "aws", "gcp", "azure", "env"

	// Vault specific
	VaultAddress string
	VaultToken   string
	VaultPath    string

	// AWS specific
	AWSRegion   string
	AWSSecretID string

	// Environment variable prefix
	EnvPrefix string

	// Caching
	CacheTTL time.Duration
}

// ============ Encryption ============

// Encryptor adalah interface untuk encryption/decryption
type Encryptor interface {
	// Encrypt encrypts plaintext
	Encrypt(plaintext []byte) ([]byte, error)

	// Decrypt decrypts ciphertext
	Decrypt(ciphertext []byte) ([]byte, error)

	// EncryptString encrypts string
	EncryptString(plaintext string) (string, error)

	// DecryptString decrypts string
	DecryptString(ciphertext string) (string, error)

	// Hash hashes data (one-way)
	Hash(data []byte) string

	// CompareHash compares data with hash
	CompareHash(data []byte, hash string) bool
}

// ============ TLS Configuration ============

// TLSConfig untuk konfigurasi TLS
type TLSConfig struct {
	// Enable TLS
	Enabled bool

	// Certificate files
	CertFile string
	KeyFile  string
	CAFile   string // For client cert verification

	// Or provide directly
	Certificate *tls.Certificate
	RootCAs     [][]byte

	// Settings
	MinVersion         uint16 // tls.VersionTLS12
	MaxVersion         uint16 // tls.VersionTLS13
	InsecureSkipVerify bool   // For development only!
	ClientAuth         tls.ClientAuthType

	// Cipher suites (empty = use defaults)
	CipherSuites []uint16
}

// ToTLSConfig converts to standard tls.Config
func (c *TLSConfig) ToTLSConfig() (*tls.Config, error) {
	if !c.Enabled {
		return nil, nil
	}

	tlsConfig := &tls.Config{
		MinVersion:         c.MinVersion,
		MaxVersion:         c.MaxVersion,
		InsecureSkipVerify: c.InsecureSkipVerify,
		ClientAuth:         c.ClientAuth,
	}

	if len(c.CipherSuites) > 0 {
		tlsConfig.CipherSuites = c.CipherSuites
	}

	// Set default versions if not specified
	if tlsConfig.MinVersion == 0 {
		tlsConfig.MinVersion = tls.VersionTLS12
	}

	return tlsConfig, nil
}

// ============ Rate Limiting ============

// RateLimiter adalah interface untuk rate limiting
type RateLimiter interface {
	// Allow checks if request is allowed
	Allow(ctx context.Context, key string) (bool, error)

	// AllowN checks if N requests are allowed
	AllowN(ctx context.Context, key string, n int) (bool, error)

	// Remaining returns remaining requests in window
	Remaining(ctx context.Context, key string) (int, error)

	// Reset resets the limit for a key
	Reset(ctx context.Context, key string) error
}

// RateLimitConfig untuk konfigurasi rate limiter
type RateLimitConfig struct {
	// Requests per window
	Limit int

	// Window duration
	Window time.Duration

	// Key extractor strategy
	KeyBy string // "ip", "user", "api_key", "custom"

	// Burst allowance
	Burst int

	// Skip paths
	SkipPaths []string

	// Custom response on limit
	LimitExceededMessage string
}

// ============ CORS Configuration ============

// CORSConfig untuk konfigurasi CORS
type CORSConfig struct {
	// Allowed origins (use "*" for all)
	AllowedOrigins []string

	// Allowed methods
	AllowedMethods []string

	// Allowed headers
	AllowedHeaders []string

	// Exposed headers
	ExposedHeaders []string

	// Allow credentials
	AllowCredentials bool

	// Max age for preflight cache
	MaxAge int
}

// DefaultCORSConfig returns sensible defaults
func DefaultCORSConfig() *CORSConfig {
	return &CORSConfig{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
		ExposedHeaders: []string{"X-Request-ID"},
		MaxAge:         86400, // 24 hours
	}
}

// ============ Audit Logging ============

// AuditLogger untuk audit trail
type AuditLogger interface {
	// Log logs an audit event
	Log(ctx context.Context, event *AuditEvent) error

	// Query queries audit logs
	Query(ctx context.Context, filter *AuditFilter) ([]*AuditEvent, error)
}

// AuditEvent represents an audit log entry
type AuditEvent struct {
	ID         string    `json:"id"`
	Timestamp  time.Time `json:"timestamp"`
	Action     string    `json:"action"`   // "create", "read", "update", "delete", "login", "logout"
	Resource   string    `json:"resource"` // "users", "orders"
	ResourceID string    `json:"resource_id"`

	// Actor info
	ActorID   string `json:"actor_id"`
	ActorType string `json:"actor_type"` // "user", "service", "system"
	ActorName string `json:"actor_name"`
	ActorIP   string `json:"actor_ip"`

	// Request info
	Method    string `json:"method,omitempty"`
	Path      string `json:"path,omitempty"`
	UserAgent string `json:"user_agent,omitempty"`

	// Result
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`

	// Additional data
	OldValue any            `json:"old_value,omitempty"`
	NewValue any            `json:"new_value,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// AuditFilter untuk query audit logs
type AuditFilter struct {
	ActorID    string
	Action     string
	Resource   string
	ResourceID string
	Success    *bool
	StartTime  time.Time
	EndTime    time.Time
	Limit      int
	Offset     int
}

// ============ Security Context Key ============

type contextKey string

const (
	IdentityContextKey contextKey = "unicorn_identity"
)

// GetIdentityFromContext extracts identity from context
func GetIdentityFromContext(ctx context.Context) (*Identity, bool) {
	identity, ok := ctx.Value(IdentityContextKey).(*Identity)
	return identity, ok
}

// SetIdentityInContext stores identity in context
func SetIdentityInContext(ctx context.Context, identity *Identity) context.Context {
	return context.WithValue(ctx, IdentityContextKey, identity)
}
