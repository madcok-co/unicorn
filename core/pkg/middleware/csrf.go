package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/madcok-co/unicorn/core/pkg/context"
)

// CSRFConfig defines CSRF protection configuration
type CSRFConfig struct {
	// TokenLength is the length of the CSRF token (default: 32)
	TokenLength int

	// TokenLookup defines where to look for the token
	// Format: "<source>:<name>"
	// Sources: header, form, query
	// Examples: "header:X-CSRF-Token", "form:csrf_token", "query:csrf"
	TokenLookup string

	// CookieName is the name of the CSRF cookie (default: _csrf)
	CookieName string

	// CookieDomain sets the cookie domain
	CookieDomain string

	// CookiePath sets the cookie path (default: /)
	CookiePath string

	// CookieMaxAge sets the cookie max age in seconds (default: 86400 = 24h)
	CookieMaxAge int

	// CookieSecure sets the Secure flag on cookie (default: false)
	CookieSecure bool

	// CookieHTTPOnly sets the HttpOnly flag on cookie (default: true)
	CookieHTTPOnly bool

	// CookieSameSite sets the SameSite attribute (default: Strict)
	CookieSameSite string

	// ContextKey is the key to store token in context (default: csrf)
	ContextKey string

	// SafeMethods are HTTP methods that don't require CSRF protection
	// Default: GET, HEAD, OPTIONS, TRACE
	SafeMethods []string

	// ErrorHandler is called when CSRF validation fails
	ErrorHandler func(ctx *context.Context, err error) error

	// Skipper defines a function to skip middleware
	Skipper func(ctx *context.Context) bool
}

// DefaultCSRFConfig returns default CSRF configuration
func DefaultCSRFConfig() *CSRFConfig {
	return &CSRFConfig{
		TokenLength:    32,
		TokenLookup:    "header:X-CSRF-Token",
		CookieName:     "_csrf",
		CookiePath:     "/",
		CookieMaxAge:   86400, // 24 hours
		CookieSecure:   false,
		CookieHTTPOnly: true,
		CookieSameSite: "Strict",
		ContextKey:     "csrf",
		SafeMethods:    []string{"GET", "HEAD", "OPTIONS", "TRACE"},
		ErrorHandler: func(ctx *context.Context, err error) error {
			return ctx.Error(http.StatusForbidden, "Invalid CSRF token")
		},
	}
}

// csrfTokenExtractor extracts token from request
type csrfTokenExtractor func(ctx *context.Context) (string, error)

// CSRF returns CSRF protection middleware with default config
func CSRF() context.MiddlewareFunc {
	return CSRFWithConfig(DefaultCSRFConfig())
}

// CSRFWithConfig returns CSRF protection middleware with custom config
func CSRFWithConfig(config *CSRFConfig) context.MiddlewareFunc {
	if config == nil {
		config = DefaultCSRFConfig()
	}

	if config.TokenLength == 0 {
		config.TokenLength = 32
	}

	if config.CookieName == "" {
		config.CookieName = "_csrf"
	}

	if config.CookiePath == "" {
		config.CookiePath = "/"
	}

	if config.CookieMaxAge == 0 {
		config.CookieMaxAge = 86400
	}

	if config.ContextKey == "" {
		config.ContextKey = "csrf"
	}

	if len(config.SafeMethods) == 0 {
		config.SafeMethods = []string{"GET", "HEAD", "OPTIONS", "TRACE"}
	}

	// Build safe methods map
	safeMethods := make(map[string]bool)
	for _, method := range config.SafeMethods {
		safeMethods[method] = true
	}

	// Parse token lookup
	extractor := createTokenExtractor(config.TokenLookup)

	return func(next context.HandlerFunc) context.HandlerFunc {
		return func(ctx *context.Context) error {
			// Check skipper
			if config.Skipper != nil && config.Skipper(ctx) {
				return next(ctx)
			}

			req := ctx.Request()

			// Get or create CSRF token
			token := req.Cookie(config.CookieName)
			if token == "" {
				// Generate new token
				token = generateToken(config.TokenLength)
			}

			// Set CSRF cookie
			setCookie(ctx, config, token)

			// Store token in context
			ctx.Set(config.ContextKey, token)

			// Check if method requires CSRF validation
			if safeMethods[req.Method] {
				return next(ctx)
			}

			// Extract token from request
			clientToken, err := extractor(ctx)
			if err != nil {
				if config.ErrorHandler != nil {
					return config.ErrorHandler(ctx, err)
				}
				return ctx.Error(http.StatusForbidden, "CSRF token not found")
			}

			// Validate token
			if !validateToken(token, clientToken) {
				if config.ErrorHandler != nil {
					return config.ErrorHandler(ctx, fmt.Errorf("invalid CSRF token"))
				}
				return ctx.Error(http.StatusForbidden, "Invalid CSRF token")
			}

			return next(ctx)
		}
	}
}

// createTokenExtractor creates a token extractor based on lookup string
func createTokenExtractor(lookup string) csrfTokenExtractor {
	if lookup == "" {
		lookup = "header:X-CSRF-Token"
	}

	parts := strings.Split(lookup, ":")
	if len(parts) != 2 {
		// Default to header
		return func(ctx *context.Context) (string, error) {
			token := ctx.Request().Header("X-CSRF-Token")
			if token == "" {
				return "", fmt.Errorf("CSRF token not found in header")
			}
			return token, nil
		}
	}

	source := parts[0]
	name := parts[1]

	switch source {
	case "header":
		return func(ctx *context.Context) (string, error) {
			token := ctx.Request().Header(name)
			if token == "" {
				return "", fmt.Errorf("CSRF token not found in header %s", name)
			}
			return token, nil
		}
	case "form":
		return func(ctx *context.Context) (string, error) {
			token := ctx.Request().Param(name)
			if token == "" {
				return "", fmt.Errorf("CSRF token not found in form field %s", name)
			}
			return token, nil
		}
	case "query":
		return func(ctx *context.Context) (string, error) {
			token := ctx.Request().QueryParam(name)
			if token == "" {
				return "", fmt.Errorf("CSRF token not found in query param %s", name)
			}
			return token, nil
		}
	default:
		// Default to header
		return func(ctx *context.Context) (string, error) {
			token := ctx.Request().Header(name)
			if token == "" {
				return "", fmt.Errorf("CSRF token not found")
			}
			return token, nil
		}
	}
}

// generateToken generates a random CSRF token
func generateToken(length int) string {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based token
		return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	}
	return base64.URLEncoding.EncodeToString(b)
}

// validateToken validates CSRF token using constant-time comparison
func validateToken(token, clientToken string) bool {
	// Constant-time comparison to prevent timing attacks
	return subtle.ConstantTimeCompare([]byte(token), []byte(clientToken)) == 1
}

// setCookie sets the CSRF cookie
func setCookie(ctx *context.Context, config *CSRFConfig, token string) {
	cookie := fmt.Sprintf("%s=%s; Path=%s; Max-Age=%d",
		config.CookieName, token, config.CookiePath, config.CookieMaxAge)

	if config.CookieDomain != "" {
		cookie += fmt.Sprintf("; Domain=%s", config.CookieDomain)
	}

	if config.CookieSecure {
		cookie += "; Secure"
	}

	if config.CookieHTTPOnly {
		cookie += "; HttpOnly"
	}

	if config.CookieSameSite != "" {
		cookie += fmt.Sprintf("; SameSite=%s", config.CookieSameSite)
	}

	ctx.Response().SetHeader("Set-Cookie", cookie)
}

// GetCSRFToken retrieves the CSRF token from context
func GetCSRFToken(ctx *context.Context) string {
	token, ok := ctx.Get("csrf")
	if !ok {
		return ""
	}
	if str, ok := token.(string); ok {
		return str
	}
	return ""
}

// CSRFWithStore returns CSRF middleware with token storage
// Useful for distributed systems where you can't rely on cookies
type CSRFStore interface {
	Get(key string) (string, error)
	Set(key string, value string, ttl time.Duration) error
	Delete(key string) error
}

// MemoryCSRFStore is an in-memory CSRF token store
type MemoryCSRFStore struct {
	tokens map[string]tokenEntry
	mu     sync.RWMutex
}

type tokenEntry struct {
	value     string
	expiresAt time.Time
}

// NewMemoryCSRFStore creates a new in-memory CSRF store
func NewMemoryCSRFStore() *MemoryCSRFStore {
	store := &MemoryCSRFStore{
		tokens: make(map[string]tokenEntry),
	}
	// Start cleanup goroutine
	go store.cleanup()
	return store
}

// Get retrieves a token from the store
func (s *MemoryCSRFStore) Get(key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.tokens[key]
	if !ok {
		return "", fmt.Errorf("token not found")
	}

	if time.Now().After(entry.expiresAt) {
		return "", fmt.Errorf("token expired")
	}

	return entry.value, nil
}

// Set stores a token in the store
func (s *MemoryCSRFStore) Set(key string, value string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tokens[key] = tokenEntry{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}

	return nil
}

// Delete removes a token from the store
func (s *MemoryCSRFStore) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.tokens, key)
	return nil
}

// cleanup removes expired tokens
func (s *MemoryCSRFStore) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for key, entry := range s.tokens {
			if now.After(entry.expiresAt) {
				delete(s.tokens, key)
			}
		}
		s.mu.Unlock()
	}
}

// CSRFFromReferer validates CSRF based on Referer header
// Less secure but useful for API endpoints
func CSRFFromReferer(allowedOrigins []string) context.MiddlewareFunc {
	originsMap := make(map[string]bool)
	for _, origin := range allowedOrigins {
		originsMap[strings.ToLower(origin)] = true
	}

	return func(next context.HandlerFunc) context.HandlerFunc {
		return func(ctx *context.Context) error {
			req := ctx.Request()

			// Skip safe methods
			if req.Method == "GET" || req.Method == "HEAD" || req.Method == "OPTIONS" {
				return next(ctx)
			}

			referer := req.Header("Referer")
			if referer == "" {
				return ctx.Error(http.StatusForbidden, "Missing Referer header")
			}

			// Extract origin from referer
			referer = strings.ToLower(referer)
			for origin := range originsMap {
				if strings.HasPrefix(referer, origin) {
					return next(ctx)
				}
			}

			return ctx.Error(http.StatusForbidden, "Invalid Referer")
		}
	}
}
