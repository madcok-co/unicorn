package http

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	ucontext "github.com/madcok-co/unicorn/core/pkg/context"
	"github.com/madcok-co/unicorn/core/pkg/contracts"
	"github.com/madcok-co/unicorn/core/pkg/handler"
)

// Adapter adalah HTTP server adapter
type Adapter struct {
	server   *http.Server
	registry *handler.Registry
	config   *Config

	// Middleware untuk semua routes
	middlewares []Middleware

	// Route parameters extractor
	paramExtractor ParamExtractor
}

// Config untuk HTTP adapter
type Config struct {
	Host         string
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration

	// TLS Configuration
	TLS *contracts.TLSConfig

	// Max request body size (0 = no limit)
	MaxBodySize int64

	// Trusted proxies for X-Forwarded-* headers
	TrustedProxies []string

	// Enable HTTP/2
	EnableHTTP2 bool
}

// Middleware type untuk HTTP
type Middleware func(http.Handler) http.Handler

// ParamExtractor extracts path parameters
type ParamExtractor func(pattern, path string) map[string]string

// New creates a new HTTP adapter
func New(registry *handler.Registry, config *Config) *Adapter {
	if config == nil {
		config = &Config{
			Host:         "0.0.0.0",
			Port:         8080,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
			MaxBodySize:  10 << 20, // 10MB default
		}
	}

	return &Adapter{
		registry:       registry,
		config:         config,
		middlewares:    make([]Middleware, 0),
		paramExtractor: defaultParamExtractor,
	}
}

// Use adds middleware
func (a *Adapter) Use(middleware ...Middleware) {
	a.middlewares = append(a.middlewares, middleware...)
}

// Start starts the HTTP server
func (a *Adapter) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// Register all HTTP handlers from registry
	for route, h := range a.registry.HTTPRoutes() {
		parts := strings.SplitN(route, ":", 2)
		if len(parts) != 2 {
			continue
		}
		method, path := parts[0], parts[1]

		// Create handler for this route
		httpHandler := a.createHandler(h, method, path)
		mux.HandleFunc(path, httpHandler)
	}

	// Apply global middlewares
	var finalHandler http.Handler = mux
	for i := len(a.middlewares) - 1; i >= 0; i-- {
		finalHandler = a.middlewares[i](finalHandler)
	}

	a.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", a.config.Host, a.config.Port),
		Handler:      finalHandler,
		ReadTimeout:  a.config.ReadTimeout,
		WriteTimeout: a.config.WriteTimeout,
		IdleTimeout:  a.config.IdleTimeout,
	}

	// Configure TLS if enabled
	if a.config.TLS != nil && a.config.TLS.Enabled {
		tlsConfig, err := a.configureTLS()
		if err != nil {
			return fmt.Errorf("failed to configure TLS: %w", err)
		}
		a.server.TLSConfig = tlsConfig
	}

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		var err error
		if a.config.TLS != nil && a.config.TLS.Enabled {
			err = a.server.ListenAndServeTLS(a.config.TLS.CertFile, a.config.TLS.KeyFile)
		} else {
			err = a.server.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Wait for context cancellation or error
	select {
	case <-ctx.Done():
		return a.Shutdown(context.Background())
	case err := <-errCh:
		return err
	}
}

// configureTLS configures TLS for the server
func (a *Adapter) configureTLS() (*tls.Config, error) {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
		PreferServerCipherSuites: true,
		CurvePreferences: []tls.CurveID{
			tls.CurveP256,
			tls.X25519,
		},
	}

	// Apply custom settings
	if a.config.TLS.MinVersion != 0 {
		tlsConfig.MinVersion = a.config.TLS.MinVersion
	}
	if a.config.TLS.MaxVersion != 0 {
		tlsConfig.MaxVersion = a.config.TLS.MaxVersion
	}
	if len(a.config.TLS.CipherSuites) > 0 {
		tlsConfig.CipherSuites = a.config.TLS.CipherSuites
	}
	if a.config.TLS.ClientAuth != 0 {
		tlsConfig.ClientAuth = a.config.TLS.ClientAuth
	}

	return tlsConfig, nil
}

// Shutdown gracefully shuts down the server
func (a *Adapter) Shutdown(ctx context.Context) error {
	if a.server == nil {
		return nil
	}
	return a.server.Shutdown(ctx)
}

// createHandler creates an http.HandlerFunc from a Handler
func (a *Adapter) createHandler(h *handler.Handler, method, pattern string) http.HandlerFunc {
	executor := handler.NewExecutor(h)

	return func(w http.ResponseWriter, r *http.Request) {
		// Check method
		if r.Method != method && method != "*" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Limit body size
		if a.config.MaxBodySize > 0 {
			r.Body = http.MaxBytesReader(w, r.Body, a.config.MaxBodySize)
		}

		// Create unicorn context
		ctx := ucontext.New(r.Context())

		// Build request
		req := &ucontext.Request{
			Method:      r.Method,
			Path:        r.URL.Path,
			Headers:     make(map[string]string),
			Params:      a.paramExtractor(pattern, r.URL.Path),
			Query:       make(map[string]string),
			TriggerType: "http",
		}

		// Copy headers
		for k, v := range r.Header {
			if len(v) > 0 {
				req.Headers[k] = v[0]
			}
		}

		// Copy query params
		for k, v := range r.URL.Query() {
			if len(v) > 0 {
				req.Query[k] = v[0]
			}
		}

		// Read body
		if r.Body != nil {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "Failed to read body", http.StatusBadRequest)
				return
			}
			req.Body = body
		}

		ctx.SetRequest(req)

		// Execute handler
		if err := executor.Execute(ctx); err != nil {
			a.writeError(w, err)
			return
		}

		// Write response
		a.writeResponse(w, ctx.Response())
	}
}

// writeResponse writes the response to http.ResponseWriter
func (a *Adapter) writeResponse(w http.ResponseWriter, resp *ucontext.Response) {
	// Set headers
	for k, v := range resp.Headers {
		w.Header().Set(k, v)
	}

	// Set status code
	statusCode := resp.StatusCode
	if statusCode == 0 {
		statusCode = http.StatusOK
	}

	// Write body
	if resp.Body != nil {
		// Default to JSON
		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", "application/json")
		}

		w.WriteHeader(statusCode)

		if w.Header().Get("Content-Type") == "application/json" {
			_ = json.NewEncoder(w).Encode(resp.Body) // Best-effort write
		} else {
			_, _ = fmt.Fprint(w, resp.Body) // Best-effort write
		}
	} else {
		w.WriteHeader(statusCode)
	}
}

// HTTPError represents a structured HTTP error
type HTTPError struct {
	StatusCode int
	Message    string
	Internal   error // Internal error, not exposed to client
}

func (e *HTTPError) Error() string {
	return e.Message
}

// NewHTTPError creates a new HTTP error
func NewHTTPError(statusCode int, message string) *HTTPError {
	return &HTTPError{
		StatusCode: statusCode,
		Message:    message,
	}
}

// writeError writes an error response without exposing internal details
func (a *Adapter) writeError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")

	// Check if it's an HTTPError with a specific status code
	if httpErr, ok := err.(*HTTPError); ok {
		w.WriteHeader(httpErr.StatusCode)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": httpErr.Message,
		})
		return
	}

	// For all other errors, return generic message to prevent information leakage
	// The actual error should be logged internally, not exposed to client
	w.WriteHeader(http.StatusInternalServerError)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": "Internal server error",
	})
}

// defaultParamExtractor extracts path params like /users/:id
func defaultParamExtractor(pattern, path string) map[string]string {
	params := make(map[string]string)

	patternParts := strings.Split(pattern, "/")
	pathParts := strings.Split(path, "/")

	for i, part := range patternParts {
		if strings.HasPrefix(part, ":") && i < len(pathParts) {
			paramName := strings.TrimPrefix(part, ":")
			params[paramName] = pathParts[i]
		} else if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") && i < len(pathParts) {
			paramName := strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}")
			params[paramName] = pathParts[i]
		}
	}

	return params
}

// Address returns the server address
func (a *Adapter) Address() string {
	return fmt.Sprintf("%s:%d", a.config.Host, a.config.Port)
}

// IsTLS returns true if TLS is enabled
func (a *Adapter) IsTLS() bool {
	return a.config.TLS != nil && a.config.TLS.Enabled
}

// Scheme returns "https" or "http"
func (a *Adapter) Scheme() string {
	if a.IsTLS() {
		return "https"
	}
	return "http"
}

// URL returns the full URL of the server
func (a *Adapter) URL() string {
	return fmt.Sprintf("%s://%s", a.Scheme(), a.Address())
}
