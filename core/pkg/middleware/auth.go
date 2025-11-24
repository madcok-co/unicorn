package middleware

import (
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"strings"

	ucontext "github.com/madcok-co/unicorn/core/pkg/context"
)

// ============ Common Auth Errors ============

var (
	ErrUnauthorized      = errors.New("unauthorized")
	ErrInvalidToken      = errors.New("invalid token")
	ErrTokenExpired      = errors.New("token expired")
	ErrMissingAuthHeader = errors.New("missing authorization header")
)

// ============ JWT Auth ============

// JWTConfig defines JWT authentication configuration
type JWTConfig struct {
	// SigningKey is the secret key for HS256 or public key for RS256
	SigningKey interface{}

	// SigningMethod is the signing algorithm (HS256, RS256, etc.)
	SigningMethod string

	// TokenLookup defines how to extract token
	// Format: "header:Authorization", "query:token", "cookie:jwt"
	TokenLookup string

	// TokenPrefix is the prefix before token (e.g., "Bearer ")
	TokenPrefix string

	// ContextKey is the key to store claims in context
	ContextKey string

	// Validator is a custom token validator function
	// Returns claims (map[string]interface{}) or error
	Validator func(token string) (map[string]interface{}, error)

	// ErrorHandler is called when auth fails
	ErrorHandler func(ctx *ucontext.Context, err error) error

	// Skipper defines a function to skip middleware
	Skipper func(ctx *ucontext.Context) bool

	// SuccessHandler is called after successful authentication
	SuccessHandler func(ctx *ucontext.Context, claims map[string]interface{})
}

// DefaultJWTConfig returns default JWT configuration
func DefaultJWTConfig() *JWTConfig {
	return &JWTConfig{
		SigningMethod: "HS256",
		TokenLookup:   "header:Authorization",
		TokenPrefix:   "Bearer ",
		ContextKey:    "user",
		ErrorHandler: func(ctx *ucontext.Context, err error) error {
			return ctx.Error(401, "Unauthorized")
		},
	}
}

// JWT returns JWT authentication middleware
func JWT(signingKey interface{}) ucontext.MiddlewareFunc {
	config := DefaultJWTConfig()
	config.SigningKey = signingKey
	return JWTWithConfig(config)
}

// JWTWithConfig returns JWT middleware with custom config
func JWTWithConfig(config *JWTConfig) ucontext.MiddlewareFunc {
	if config == nil {
		config = DefaultJWTConfig()
	}

	if config.TokenLookup == "" {
		config.TokenLookup = "header:Authorization"
	}
	if config.TokenPrefix == "" {
		config.TokenPrefix = "Bearer "
	}
	if config.ContextKey == "" {
		config.ContextKey = "user"
	}
	if config.ErrorHandler == nil {
		config.ErrorHandler = func(ctx *ucontext.Context, err error) error {
			return ctx.Error(401, "Unauthorized")
		}
	}

	// Parse token lookup
	parts := strings.SplitN(config.TokenLookup, ":", 2)
	lookupType := parts[0]
	lookupKey := ""
	if len(parts) > 1 {
		lookupKey = parts[1]
	}

	return func(next ucontext.HandlerFunc) ucontext.HandlerFunc {
		return func(ctx *ucontext.Context) error {
			// Check skipper
			if config.Skipper != nil && config.Skipper(ctx) {
				return next(ctx)
			}

			// Extract token
			var token string
			switch lookupType {
			case "header":
				auth := ctx.Request().Header(lookupKey)
				if auth != "" && strings.HasPrefix(auth, config.TokenPrefix) {
					token = strings.TrimPrefix(auth, config.TokenPrefix)
				}
			case "query":
				token = ctx.Request().QueryParam(lookupKey)
			case "cookie":
				token = ctx.Request().Cookie(lookupKey)
			}

			if token == "" {
				return config.ErrorHandler(ctx, ErrMissingAuthHeader)
			}

			// Validate token
			if config.Validator == nil {
				return config.ErrorHandler(ctx, ErrInvalidToken)
			}

			claims, err := config.Validator(token)
			if err != nil {
				return config.ErrorHandler(ctx, err)
			}

			// Store claims in context
			ctx.Set(config.ContextKey, claims)

			// Call success handler if set
			if config.SuccessHandler != nil {
				config.SuccessHandler(ctx, claims)
			}

			return next(ctx)
		}
	}
}

// ============ API Key Auth ============

// APIKeyConfig defines API key authentication configuration
type APIKeyConfig struct {
	// KeyLookup defines how to extract API key
	// Format: "header:X-API-Key", "query:api_key"
	KeyLookup string

	// Validator validates the API key and returns associated data
	Validator func(key string) (interface{}, error)

	// ContextKey is the key to store validated data in context
	ContextKey string

	// ErrorHandler is called when auth fails
	ErrorHandler func(ctx *ucontext.Context, err error) error

	// Skipper defines a function to skip middleware
	Skipper func(ctx *ucontext.Context) bool
}

// DefaultAPIKeyConfig returns default API key configuration
func DefaultAPIKeyConfig() *APIKeyConfig {
	return &APIKeyConfig{
		KeyLookup:  "header:X-API-Key",
		ContextKey: "api_key_data",
		ErrorHandler: func(ctx *ucontext.Context, err error) error {
			return ctx.Error(401, "Invalid API key")
		},
	}
}

// APIKey returns API key authentication middleware
func APIKey(validator func(key string) (interface{}, error)) ucontext.MiddlewareFunc {
	config := DefaultAPIKeyConfig()
	config.Validator = validator
	return APIKeyWithConfig(config)
}

// APIKeyWithConfig returns API key middleware with custom config
func APIKeyWithConfig(config *APIKeyConfig) ucontext.MiddlewareFunc {
	if config == nil {
		config = DefaultAPIKeyConfig()
	}

	if config.KeyLookup == "" {
		config.KeyLookup = "header:X-API-Key"
	}
	if config.ContextKey == "" {
		config.ContextKey = "api_key_data"
	}
	if config.ErrorHandler == nil {
		config.ErrorHandler = func(ctx *ucontext.Context, err error) error {
			return ctx.Error(401, "Invalid API key")
		}
	}

	// Parse key lookup
	parts := strings.SplitN(config.KeyLookup, ":", 2)
	lookupType := parts[0]
	lookupKey := ""
	if len(parts) > 1 {
		lookupKey = parts[1]
	}

	return func(next ucontext.HandlerFunc) ucontext.HandlerFunc {
		return func(ctx *ucontext.Context) error {
			// Check skipper
			if config.Skipper != nil && config.Skipper(ctx) {
				return next(ctx)
			}

			// Extract API key
			var apiKey string
			switch lookupType {
			case "header":
				apiKey = ctx.Request().Header(lookupKey)
			case "query":
				apiKey = ctx.Request().QueryParam(lookupKey)
			}

			if apiKey == "" {
				return config.ErrorHandler(ctx, ErrMissingAuthHeader)
			}

			// Validate API key
			if config.Validator == nil {
				return config.ErrorHandler(ctx, ErrInvalidToken)
			}

			data, err := config.Validator(apiKey)
			if err != nil {
				return config.ErrorHandler(ctx, err)
			}

			// Store data in context
			ctx.Set(config.ContextKey, data)

			return next(ctx)
		}
	}
}

// ============ Basic Auth ============

// BasicAuthConfig defines basic authentication configuration
type BasicAuthConfig struct {
	// Validator validates username and password
	// Returns associated user data and error
	Validator func(username, password string) (interface{}, error)

	// Realm is the authentication realm
	Realm string

	// ContextKey is the key to store user data in context
	ContextKey string

	// ErrorHandler is called when auth fails
	ErrorHandler func(ctx *ucontext.Context, err error) error

	// Skipper defines a function to skip middleware
	Skipper func(ctx *ucontext.Context) bool
}

// DefaultBasicAuthConfig returns default basic auth configuration
func DefaultBasicAuthConfig() *BasicAuthConfig {
	return &BasicAuthConfig{
		Realm:      "Restricted",
		ContextKey: "user",
	}
}

// BasicAuth returns basic authentication middleware
func BasicAuth(validator func(username, password string) (interface{}, error)) ucontext.MiddlewareFunc {
	config := DefaultBasicAuthConfig()
	config.Validator = validator
	return BasicAuthWithConfig(config)
}

// BasicAuthWithConfig returns basic auth middleware with custom config
func BasicAuthWithConfig(config *BasicAuthConfig) ucontext.MiddlewareFunc {
	if config == nil {
		config = DefaultBasicAuthConfig()
	}

	if config.Realm == "" {
		config.Realm = "Restricted"
	}
	if config.ContextKey == "" {
		config.ContextKey = "user"
	}
	if config.ErrorHandler == nil {
		config.ErrorHandler = func(ctx *ucontext.Context, err error) error {
			ctx.Response().SetHeader("WWW-Authenticate", `Basic realm="`+config.Realm+`"`)
			return ctx.Error(401, "Unauthorized")
		}
	}

	return func(next ucontext.HandlerFunc) ucontext.HandlerFunc {
		return func(ctx *ucontext.Context) error {
			// Check skipper
			if config.Skipper != nil && config.Skipper(ctx) {
				return next(ctx)
			}

			// Extract credentials from Authorization header
			auth := ctx.Request().Header("Authorization")
			if auth == "" || !strings.HasPrefix(auth, "Basic ") {
				return config.ErrorHandler(ctx, ErrMissingAuthHeader)
			}

			// Decode base64
			payload, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(auth, "Basic "))
			if err != nil {
				return config.ErrorHandler(ctx, ErrInvalidToken)
			}

			// Split username:password
			pair := strings.SplitN(string(payload), ":", 2)
			if len(pair) != 2 {
				return config.ErrorHandler(ctx, ErrInvalidToken)
			}

			username := pair[0]
			password := pair[1]

			// Validate credentials
			if config.Validator == nil {
				return config.ErrorHandler(ctx, ErrInvalidToken)
			}

			user, err := config.Validator(username, password)
			if err != nil {
				return config.ErrorHandler(ctx, err)
			}

			// Store user data in context
			ctx.Set(config.ContextKey, user)

			return next(ctx)
		}
	}
}

// ============ Helper Functions ============

// SecureCompare compares two strings in constant time
func SecureCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// SkipPaths returns a skipper that skips specified paths
func SkipPaths(paths ...string) func(*ucontext.Context) bool {
	pathMap := make(map[string]bool)
	for _, p := range paths {
		pathMap[p] = true
	}
	return func(ctx *ucontext.Context) bool {
		return pathMap[ctx.Request().Path]
	}
}

// SkipPathPrefixes returns a skipper that skips paths with specified prefixes
func SkipPathPrefixes(prefixes ...string) func(*ucontext.Context) bool {
	return func(ctx *ucontext.Context) bool {
		path := ctx.Request().Path
		for _, prefix := range prefixes {
			if strings.HasPrefix(path, prefix) {
				return true
			}
		}
		return false
	}
}
