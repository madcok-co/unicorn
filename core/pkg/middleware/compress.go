package middleware

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"strings"

	"github.com/andybalholm/brotli"
	"github.com/madcok-co/unicorn/core/pkg/context"
)

// CompressionLevel defines compression level
type CompressionLevel int

const (
	// BestSpeed provides fastest compression with lower ratio
	BestSpeed CompressionLevel = iota
	// BestCompression provides best compression ratio but slower
	BestCompression
	// DefaultCompression balances speed and compression ratio
	DefaultCompression
)

// CompressConfig defines configuration for compression middleware
type CompressConfig struct {
	// Level defines compression level (default: DefaultCompression)
	Level CompressionLevel

	// MinLength defines minimum body size to compress (bytes, default: 1024)
	// Bodies smaller than this won't be compressed
	MinLength int

	// CompressionTypes defines Content-Types that should be compressed
	// Default: text/*, application/json, application/javascript, application/xml
	CompressionTypes []string

	// ExcludedPaths defines paths that should not be compressed
	ExcludedPaths []string

	// ExcludedExtensions defines file extensions that should not be compressed
	// Default: .jpg, .jpeg, .png, .gif, .webp, .mp4, .zip, .gz, .br
	ExcludedExtensions []string

	// EnableBrotli enables brotli compression (higher compression ratio)
	// Brotli is preferred over gzip if client supports it
	EnableBrotli bool

	// Skipper defines a function to skip middleware
	Skipper func(ctx *context.Context) bool
}

// DefaultCompressConfig returns default compression configuration
func DefaultCompressConfig() *CompressConfig {
	return &CompressConfig{
		Level:     DefaultCompression,
		MinLength: 1024, // 1KB
		CompressionTypes: []string{
			"text/html",
			"text/css",
			"text/plain",
			"text/javascript",
			"application/json",
			"application/javascript",
			"application/xml",
			"application/x-javascript",
			"application/xhtml+xml",
			"application/rss+xml",
			"application/atom+xml",
		},
		ExcludedExtensions: []string{
			".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".ico",
			".mp4", ".avi", ".mov", ".mp3", ".wav",
			".zip", ".gz", ".br", ".tar", ".rar", ".7z",
			".pdf", ".woff", ".woff2", ".ttf", ".eot",
		},
		EnableBrotli: true,
	}
}

// Compress returns compression middleware with default config
func Compress() context.MiddlewareFunc {
	return CompressWithConfig(DefaultCompressConfig())
}

// CompressWithConfig returns compression middleware with custom config
func CompressWithConfig(config *CompressConfig) context.MiddlewareFunc {
	if config == nil {
		config = DefaultCompressConfig()
	}

	if config.MinLength <= 0 {
		config.MinLength = 1024
	}

	// Build compression types map for fast lookup
	compressTypes := make(map[string]bool, len(config.CompressionTypes))
	for _, ct := range config.CompressionTypes {
		compressTypes[strings.ToLower(ct)] = true
	}

	// Build excluded extensions map
	excludedExts := make(map[string]bool, len(config.ExcludedExtensions))
	for _, ext := range config.ExcludedExtensions {
		excludedExts[strings.ToLower(ext)] = true
	}

	// Build excluded paths map
	excludedPaths := make(map[string]bool, len(config.ExcludedPaths))
	for _, path := range config.ExcludedPaths {
		excludedPaths[path] = true
	}

	return func(next context.HandlerFunc) context.HandlerFunc {
		return func(ctx *context.Context) error {
			// Check skipper
			if config.Skipper != nil && config.Skipper(ctx) {
				return next(ctx)
			}

			req := ctx.Request()

			// Check excluded paths
			if excludedPaths[req.Path] {
				return next(ctx)
			}

			// Check excluded extensions
			for ext := range excludedExts {
				if strings.HasSuffix(strings.ToLower(req.Path), ext) {
					return next(ctx)
				}
			}

			// Check if client accepts compression
			acceptEncoding := req.Header("Accept-Encoding")
			if acceptEncoding == "" {
				return next(ctx)
			}

			// Determine compression algorithm
			var useCompression string
			if config.EnableBrotli && strings.Contains(acceptEncoding, "br") {
				useCompression = "br"
			} else if strings.Contains(acceptEncoding, "gzip") {
				useCompression = "gzip"
			} else {
				// Client doesn't support compression
				return next(ctx)
			}

			// Execute handler
			err := next(ctx)
			if err != nil {
				return err
			}

			resp := ctx.Response()

			// Set default Content-Type if not set
			if resp.Header("Content-Type") == "" {
				resp.SetHeader("Content-Type", "application/json")
			}

			// Check if response should be compressed
			if !shouldCompress(resp, compressTypes, config.MinLength) {
				return nil
			}

			// Get response body as bytes
			bodyBytes, err := marshalBody(resp.Body)
			if err != nil {
				return nil // Skip compression on marshal error
			}

			// Check body size
			if len(bodyBytes) < config.MinLength {
				return nil
			}

			// Compress the body
			var compressed []byte
			if useCompression == "br" {
				compressed, err = compressBrotli(bodyBytes, config.Level)
			} else {
				compressed, err = compressGzip(bodyBytes, config.Level)
			}

			if err != nil {
				return nil // Skip compression on error
			}

			// Only use compressed if it's actually smaller
			if len(compressed) < len(bodyBytes) {
				resp.Body = compressed
				resp.SetHeader("Content-Encoding", useCompression)
				resp.SetHeader("Vary", "Accept-Encoding")
				// Remove Content-Length as it's now compressed
				delete(resp.Headers, "Content-Length")
			}

			return nil
		}
	}
}

// shouldCompress checks if response should be compressed
func shouldCompress(resp *context.Response, compressTypes map[string]bool, minLength int) bool {
	// Don't compress if already compressed
	if resp.Header("Content-Encoding") != "" {
		return false
	}

	// Check content type
	contentType := resp.Header("Content-Type")
	if contentType == "" {
		contentType = "application/json" // Default for JSON responses
	}

	// Extract base content type (remove charset, etc.)
	ct := strings.ToLower(strings.Split(contentType, ";")[0])
	ct = strings.TrimSpace(ct)

	// Check if content type should be compressed
	if !compressTypes[ct] {
		// Check wildcard patterns (e.g., text/*)
		for compressType := range compressTypes {
			if strings.HasSuffix(compressType, "/*") {
				prefix := strings.TrimSuffix(compressType, "/*")
				if strings.HasPrefix(ct, prefix+"/") {
					return true
				}
			}
		}
		return false
	}

	return true
}

// marshalBody converts response body to bytes
func marshalBody(body any) ([]byte, error) {
	if body == nil {
		return []byte{}, nil
	}

	switch v := body.(type) {
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	default:
		return json.Marshal(body)
	}
}

// compressGzip compresses data using gzip
func compressGzip(data []byte, level CompressionLevel) ([]byte, error) {
	var buf bytes.Buffer

	var gzipLevel int
	switch level {
	case BestSpeed:
		gzipLevel = gzip.BestSpeed
	case BestCompression:
		gzipLevel = gzip.BestCompression
	default:
		gzipLevel = gzip.DefaultCompression
	}

	writer, err := gzip.NewWriterLevel(&buf, gzipLevel)
	if err != nil {
		return nil, err
	}

	_, err = writer.Write(data)
	if err != nil {
		writer.Close()
		return nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// compressBrotli compresses data using brotli
func compressBrotli(data []byte, level CompressionLevel) ([]byte, error) {
	var buf bytes.Buffer

	var brotliLevel int
	switch level {
	case BestSpeed:
		brotliLevel = brotli.BestSpeed
	case BestCompression:
		brotliLevel = brotli.BestCompression
	default:
		brotliLevel = brotli.DefaultCompression
	}

	writer := brotli.NewWriterLevel(&buf, brotliLevel)

	_, err := writer.Write(data)
	if err != nil {
		writer.Close()
		return nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// GzipCompress returns middleware that only uses gzip compression
func GzipCompress() context.MiddlewareFunc {
	config := DefaultCompressConfig()
	config.EnableBrotli = false
	return CompressWithConfig(config)
}

// BrotliCompress returns middleware that only uses brotli compression
func BrotliCompress() context.MiddlewareFunc {
	config := DefaultCompressConfig()
	config.EnableBrotli = true
	return CompressWithConfig(config)
}

// FastCompress returns middleware optimized for speed
func FastCompress() context.MiddlewareFunc {
	config := DefaultCompressConfig()
	config.Level = BestSpeed
	return CompressWithConfig(config)
}

// HighCompress returns middleware optimized for compression ratio
func HighCompress() context.MiddlewareFunc {
	config := DefaultCompressConfig()
	config.Level = BestCompression
	return CompressWithConfig(config)
}

// CompressWithTypes returns middleware that compresses only specified content types
func CompressWithTypes(contentTypes ...string) context.MiddlewareFunc {
	config := DefaultCompressConfig()
	config.CompressionTypes = contentTypes
	return CompressWithConfig(config)
}

// CompressWithMinLength returns middleware that compresses only bodies larger than minLength
func CompressWithMinLength(minLength int) context.MiddlewareFunc {
	config := DefaultCompressConfig()
	config.MinLength = minLength
	return CompressWithConfig(config)
}
