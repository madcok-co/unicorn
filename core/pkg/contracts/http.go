package contracts

import (
	"context"
	"net/http"
	"time"
)

// HTTPClient adalah generic interface untuk HTTP client
// Implementasi bisa standard http, resty, req, dll
type HTTPClient interface {
	// Basic HTTP methods
	Get(ctx context.Context, url string, opts ...RequestOption) (*HTTPResponse, error)
	Post(ctx context.Context, url string, body any, opts ...RequestOption) (*HTTPResponse, error)
	Put(ctx context.Context, url string, body any, opts ...RequestOption) (*HTTPResponse, error)
	Patch(ctx context.Context, url string, body any, opts ...RequestOption) (*HTTPResponse, error)
	Delete(ctx context.Context, url string, opts ...RequestOption) (*HTTPResponse, error)

	// Generic request
	Do(ctx context.Context, req *HTTPRequest) (*HTTPResponse, error)

	// Configuration
	SetBaseURL(url string) HTTPClient
	SetHeader(key, value string) HTTPClient
	SetHeaders(headers map[string]string) HTTPClient
	SetTimeout(timeout time.Duration) HTTPClient
	SetRetry(count int, waitTime time.Duration) HTTPClient

	// Auth
	SetBasicAuth(username, password string) HTTPClient
	SetBearerToken(token string) HTTPClient
}

// HTTPRequest represents an HTTP request
type HTTPRequest struct {
	Method  string
	URL     string
	Headers map[string]string
	Query   map[string]string
	Body    any

	// Options
	Timeout time.Duration
}

// HTTPResponse represents an HTTP response
type HTTPResponse struct {
	StatusCode int
	Status     string
	Headers    http.Header
	Body       []byte

	// Timing
	Duration time.Duration
}

// JSON unmarshals response body to dest
func (r *HTTPResponse) JSON(dest any) error {
	// Implementation will be in the adapter
	return nil
}

// IsSuccess returns true if status code is 2xx
func (r *HTTPResponse) IsSuccess() bool {
	return r.StatusCode >= 200 && r.StatusCode < 300
}

// IsError returns true if status code is 4xx or 5xx
func (r *HTTPResponse) IsError() bool {
	return r.StatusCode >= 400
}

// RequestOption untuk konfigurasi per-request
type RequestOption func(*requestOptions)

type requestOptions struct {
	Headers map[string]string
	Query   map[string]string
	Timeout time.Duration
}

// WithHeaders adds headers to request
func WithHeaders(headers map[string]string) RequestOption {
	return func(o *requestOptions) {
		o.Headers = headers
	}
}

// WithQuery adds query parameters
func WithQuery(query map[string]string) RequestOption {
	return func(o *requestOptions) {
		o.Query = query
	}
}

// WithTimeout sets request timeout
func WithTimeout(timeout time.Duration) RequestOption {
	return func(o *requestOptions) {
		o.Timeout = timeout
	}
}

// HTTPClientConfig untuk konfigurasi HTTP client
type HTTPClientConfig struct {
	BaseURL string
	Timeout time.Duration
	Headers map[string]string

	// Retry
	RetryCount    int
	RetryWaitTime time.Duration

	// TLS
	InsecureSkipVerify bool
	CertFile           string
	KeyFile            string
	CAFile             string

	// Connection pool
	MaxIdleConns        int
	MaxIdleConnsPerHost int
	MaxConnsPerHost     int
	IdleConnTimeout     time.Duration
}
