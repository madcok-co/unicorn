package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/madcok-co/unicorn/core/pkg/context"
)

// Use type aliases for cleaner code
type corsContext = context.Context
type corsMiddlewareFunc = context.MiddlewareFunc
type corsHandlerFunc = context.HandlerFunc

// CORSConfig defines CORS middleware configuration
type CORSConfig struct {
	// AllowOrigins defines a list of origins that may access the resource
	// "*" allows all origins (not recommended for production with credentials)
	AllowOrigins []string

	// AllowOriginFunc is a custom function to validate origins
	// If set, AllowOrigins is ignored
	AllowOriginFunc func(origin string) bool

	// AllowMethods defines allowed HTTP methods
	AllowMethods []string

	// AllowHeaders defines allowed request headers
	AllowHeaders []string

	// AllowCredentials indicates whether credentials can be exposed
	AllowCredentials bool

	// ExposeHeaders defines headers exposed to the browser
	ExposeHeaders []string

	// MaxAge indicates how long preflight results can be cached
	MaxAge int

	// Skipper defines a function to skip middleware
	Skipper func(ctx *corsContext) bool
}

// DefaultCORSConfig returns default CORS configuration
func DefaultCORSConfig() *CORSConfig {
	return &CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{
			http.MethodGet,
			http.MethodHead,
			http.MethodPut,
			http.MethodPatch,
			http.MethodPost,
			http.MethodDelete,
		},
		AllowHeaders: []string{
			"Accept",
			"Accept-Language",
			"Content-Language",
			"Content-Type",
			"Authorization",
			"X-Requested-With",
		},
		AllowCredentials: false,
		MaxAge:           86400, // 24 hours
	}
}

// CORS returns CORS middleware with default config
func CORS() corsMiddlewareFunc {
	return CORSWithConfig(DefaultCORSConfig())
}

// CORSWithConfig returns CORS middleware with custom config
func CORSWithConfig(config *CORSConfig) corsMiddlewareFunc {
	if config == nil {
		config = DefaultCORSConfig()
	}

	if len(config.AllowOrigins) == 0 && config.AllowOriginFunc == nil {
		config.AllowOrigins = []string{"*"}
	}
	if len(config.AllowMethods) == 0 {
		config.AllowMethods = DefaultCORSConfig().AllowMethods
	}

	allowMethods := strings.Join(config.AllowMethods, ", ")
	allowHeaders := strings.Join(config.AllowHeaders, ", ")
	exposeHeaders := strings.Join(config.ExposeHeaders, ", ")
	maxAge := strconv.Itoa(config.MaxAge)

	return func(next corsHandlerFunc) corsHandlerFunc {
		return func(ctx *corsContext) error {
			// Check skipper
			if config.Skipper != nil && config.Skipper(ctx) {
				return next(ctx)
			}

			req := ctx.Request()
			origin := req.Header("Origin")

			// No origin header = same-origin request
			if origin == "" {
				return next(ctx)
			}

			// Check if origin is allowed
			allowOrigin := ""
			if config.AllowOriginFunc != nil {
				if config.AllowOriginFunc(origin) {
					allowOrigin = origin
				}
			} else {
				for _, o := range config.AllowOrigins {
					if o == "*" {
						if config.AllowCredentials {
							allowOrigin = origin // With credentials, can't use wildcard
						} else {
							allowOrigin = "*"
						}
						break
					}
					if o == origin {
						allowOrigin = origin
						break
					}
				}
			}

			// Origin not allowed
			if allowOrigin == "" {
				return next(ctx)
			}

			resp := ctx.Response()

			// Set CORS headers
			resp.SetHeader("Access-Control-Allow-Origin", allowOrigin)
			if config.AllowCredentials {
				resp.SetHeader("Access-Control-Allow-Credentials", "true")
			}
			if exposeHeaders != "" {
				resp.SetHeader("Access-Control-Expose-Headers", exposeHeaders)
			}

			// Handle preflight request
			if req.Method == http.MethodOptions {
				resp.SetHeader("Access-Control-Allow-Methods", allowMethods)
				if allowHeaders != "" {
					resp.SetHeader("Access-Control-Allow-Headers", allowHeaders)
				}
				if config.MaxAge > 0 {
					resp.SetHeader("Access-Control-Max-Age", maxAge)
				}

				// Respond to preflight with 204 No Content
				resp.StatusCode = http.StatusNoContent
				return nil
			}

			return next(ctx)
		}
	}
}

// CORSAllowAll returns CORS middleware that allows everything (dev only!)
func CORSAllowAll() corsMiddlewareFunc {
	return CORSWithConfig(&CORSConfig{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"*"},
		AllowHeaders:     []string{"*"},
		AllowCredentials: false,
	})
}
