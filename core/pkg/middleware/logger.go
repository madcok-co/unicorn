package middleware

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/madcok-co/unicorn/core/pkg/context"
	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

// LoggerConfig defines configuration for request/response logging middleware
type LoggerConfig struct {
	// Logger instance to use for logging
	Logger contracts.Logger

	// LogLevel defines the log level (default: INFO)
	LogLevel string

	// SkipPaths defines paths to skip logging
	SkipPaths []string

	// SkipPathsRegex defines regex patterns for paths to skip
	SkipPathsRegex []*regexp.Regexp

	// LogRequestBody enables logging of request body (default: true)
	LogRequestBody bool

	// LogResponseBody enables logging of response body (default: true)
	LogResponseBody bool

	// LogHeaders enables logging of request/response headers (default: true)
	LogHeaders bool

	// MaxBodySize limits the body size to log (bytes). 0 = unlimited
	MaxBodySize int

	// SensitiveFields defines field names that should be masked
	// Common examples: password, token, secret, api_key, credit_card
	SensitiveFields []string

	// SensitiveHeaders defines header names that should be masked
	// Common examples: Authorization, X-API-Key, Cookie
	SensitiveHeaders []string

	// MaskValue is the value to replace sensitive data (default: "***MASKED***")
	MaskValue string

	// LogLatency enables logging of request latency (default: true)
	LogLatency bool

	// LogUserAgent enables logging of User-Agent header (default: true)
	LogUserAgent bool

	// LogIP enables logging of client IP (default: true)
	LogIP bool

	// CustomFields allows adding custom fields to log entries
	CustomFields func(ctx *context.Context) map[string]any

	// Skipper defines a function to skip middleware
	Skipper func(ctx *context.Context) bool
}

// DefaultLoggerConfig returns default logger configuration
func DefaultLoggerConfig() *LoggerConfig {
	return &LoggerConfig{
		LogLevel:        "INFO",
		LogRequestBody:  true,
		LogResponseBody: true,
		LogHeaders:      true,
		MaxBodySize:     10240, // 10KB
		SensitiveFields: []string{
			"password", "token", "secret", "api_key", "apikey",
			"access_token", "refresh_token", "private_key",
			"credit_card", "card_number", "cvv", "ssn",
		},
		SensitiveHeaders: []string{
			"Authorization", "X-API-Key", "X-Auth-Token",
			"Cookie", "Set-Cookie", "X-CSRF-Token",
		},
		MaskValue:    "***MASKED***",
		LogLatency:   true,
		LogUserAgent: true,
		LogIP:        true,
		SkipPaths: []string{
			"/health",
			"/health/live",
			"/health/ready",
			"/metrics",
		},
	}
}

// RequestResponseLogger returns logging middleware with default config
func RequestResponseLogger(logger contracts.Logger) context.MiddlewareFunc {
	config := DefaultLoggerConfig()
	config.Logger = logger
	return RequestResponseLoggerWithConfig(config)
}

// RequestResponseLoggerWithConfig returns logging middleware with custom config
func RequestResponseLoggerWithConfig(config *LoggerConfig) context.MiddlewareFunc {
	if config == nil {
		config = DefaultLoggerConfig()
	}

	if config.Logger == nil {
		// Skip logging if no logger provided
		return func(next context.HandlerFunc) context.HandlerFunc {
			return next
		}
	}

	if config.MaskValue == "" {
		config.MaskValue = "***MASKED***"
	}

	if config.MaxBodySize == 0 {
		config.MaxBodySize = 10240 // Default 10KB
	}

	// Build sensitive fields map for fast lookup
	sensitiveFieldsMap := make(map[string]bool, len(config.SensitiveFields))
	for _, field := range config.SensitiveFields {
		sensitiveFieldsMap[strings.ToLower(field)] = true
	}

	// Build sensitive headers map for fast lookup
	sensitiveHeadersMap := make(map[string]bool, len(config.SensitiveHeaders))
	for _, header := range config.SensitiveHeaders {
		sensitiveHeadersMap[strings.ToLower(header)] = true
	}

	return func(next context.HandlerFunc) context.HandlerFunc {
		return func(ctx *context.Context) error {
			// Check skipper
			if config.Skipper != nil && config.Skipper(ctx) {
				return next(ctx)
			}

			req := ctx.Request()

			// Check skip paths
			for _, path := range config.SkipPaths {
				if req.Path == path {
					return next(ctx)
				}
			}

			// Check skip paths regex
			for _, pattern := range config.SkipPathsRegex {
				if pattern.MatchString(req.Path) {
					return next(ctx)
				}
			}

			start := time.Now()

			// Capture request data
			logEntry := make(map[string]any)
			logEntry["type"] = "request"
			logEntry["method"] = req.Method
			logEntry["path"] = req.Path

			// Log IP
			if config.LogIP {
				if ip := req.Header("X-Real-IP"); ip != "" {
					logEntry["client_ip"] = ip
				} else if ip := req.Header("X-Forwarded-For"); ip != "" {
					logEntry["client_ip"] = strings.Split(ip, ",")[0]
				}
			}

			// Log User-Agent
			if config.LogUserAgent {
				if ua := req.Header("User-Agent"); ua != "" {
					logEntry["user_agent"] = ua
				}
			}

			// Log request headers
			if config.LogHeaders && len(req.Headers) > 0 {
				maskedHeaders := maskSensitiveData(req.Headers, sensitiveHeadersMap, config.MaskValue)
				logEntry["request_headers"] = maskedHeaders
			}

			// Log request body
			if config.LogRequestBody && len(req.Body) > 0 {
				bodyStr := string(req.Body)
				if len(bodyStr) > config.MaxBodySize {
					bodyStr = bodyStr[:config.MaxBodySize] + "...[truncated]"
				}
				// Try to parse as JSON and mask sensitive fields
				maskedBody := maskSensitiveJSON(bodyStr, sensitiveFieldsMap, config.MaskValue)
				logEntry["request_body"] = maskedBody
			}

			// Log query params
			if len(req.Query) > 0 {
				maskedQuery := maskSensitiveData(req.Query, sensitiveFieldsMap, config.MaskValue)
				logEntry["query_params"] = maskedQuery
			}

			// Execute handler
			err := next(ctx)

			// Calculate latency
			var latency time.Duration
			if config.LogLatency {
				latency = time.Since(start)
				logEntry["latency_ms"] = latency.Milliseconds()
				logEntry["latency"] = latency.String()
			}

			// Capture response data
			resp := ctx.Response()
			logEntry["status_code"] = resp.StatusCode

			// Log response headers
			if config.LogHeaders && len(resp.Headers) > 0 {
				maskedRespHeaders := maskSensitiveData(resp.Headers, sensitiveHeadersMap, config.MaskValue)
				logEntry["response_headers"] = maskedRespHeaders
			}

			// Log response body
			if config.LogResponseBody && resp.Body != nil {
				bodyBytes, marshalErr := json.Marshal(resp.Body)
				if marshalErr == nil {
					bodyStr := string(bodyBytes)
					if len(bodyStr) > config.MaxBodySize {
						bodyStr = bodyStr[:config.MaxBodySize] + "...[truncated]"
					}
					maskedBody := maskSensitiveJSON(bodyStr, sensitiveFieldsMap, config.MaskValue)
					logEntry["response_body"] = maskedBody
				}
			}

			// Log error if present
			if err != nil {
				logEntry["error"] = err.Error()
			}

			// Add custom fields
			if config.CustomFields != nil {
				customFields := config.CustomFields(ctx)
				for k, v := range customFields {
					logEntry[k] = v
				}
			}

			// Convert logEntry to key-value pairs for structured logging
			kvPairs := make([]any, 0, len(logEntry)*2)
			for k, v := range logEntry {
				kvPairs = append(kvPairs, k, v)
			}

			// Log based on status code and error
			if err != nil || resp.StatusCode >= 500 {
				config.Logger.Error("HTTP Request", kvPairs...)
			} else if resp.StatusCode >= 400 {
				config.Logger.Warn("HTTP Request", kvPairs...)
			} else {
				config.Logger.Info("HTTP Request", kvPairs...)
			}

			return err
		}
	}
}

// maskSensitiveData masks sensitive fields in a map
func maskSensitiveData(data map[string]string, sensitiveFields map[string]bool, maskValue string) map[string]string {
	if len(data) == 0 {
		return data
	}

	masked := make(map[string]string, len(data))
	for k, v := range data {
		if sensitiveFields[strings.ToLower(k)] {
			masked[k] = maskValue
		} else {
			masked[k] = v
		}
	}
	return masked
}

// maskSensitiveJSON masks sensitive fields in JSON string
func maskSensitiveJSON(jsonStr string, sensitiveFields map[string]bool, maskValue string) string {
	// Try to parse as JSON
	var data map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		// Not valid JSON or not an object, return as-is
		return jsonStr
	}

	// Recursively mask sensitive fields
	maskSensitiveInMap(data, sensitiveFields, maskValue)

	// Marshal back to JSON
	masked, err := json.Marshal(data)
	if err != nil {
		return jsonStr
	}

	return string(masked)
}

// maskSensitiveInMap recursively masks sensitive fields in a map
func maskSensitiveInMap(data map[string]any, sensitiveFields map[string]bool, maskValue string) {
	for k, v := range data {
		keyLower := strings.ToLower(k)
		if sensitiveFields[keyLower] {
			data[k] = maskValue
			continue
		}

		// Recursively process nested objects
		switch val := v.(type) {
		case map[string]any:
			maskSensitiveInMap(val, sensitiveFields, maskValue)
		case []any:
			for _, item := range val {
				if itemMap, ok := item.(map[string]any); ok {
					maskSensitiveInMap(itemMap, sensitiveFields, maskValue)
				}
			}
		}
	}
}

// CompactLogger returns a minimal logger for high-throughput scenarios
func CompactLogger(logger contracts.Logger) context.MiddlewareFunc {
	return RequestResponseLoggerWithConfig(&LoggerConfig{
		Logger:          logger,
		LogRequestBody:  false,
		LogResponseBody: false,
		LogHeaders:      false,
		LogLatency:      true,
		LogUserAgent:    false,
		LogIP:           true,
		SkipPaths: []string{
			"/health",
			"/health/live",
			"/health/ready",
			"/metrics",
		},
	})
}

// DetailedLogger returns a comprehensive logger for debugging
func DetailedLogger(logger contracts.Logger) context.MiddlewareFunc {
	config := DefaultLoggerConfig()
	config.Logger = logger
	config.MaxBodySize = 102400 // 100KB for detailed logging
	return RequestResponseLoggerWithConfig(config)
}

// AuditLogger returns a logger specifically for audit trails
// Logs everything including bodies and headers
func AuditLogger(logger contracts.Logger) context.MiddlewareFunc {
	return RequestResponseLoggerWithConfig(&LoggerConfig{
		Logger:           logger,
		LogLevel:         "INFO",
		LogRequestBody:   true,
		LogResponseBody:  true,
		LogHeaders:       true,
		MaxBodySize:      0, // Unlimited
		SensitiveFields:  DefaultLoggerConfig().SensitiveFields,
		SensitiveHeaders: DefaultLoggerConfig().SensitiveHeaders,
		MaskValue:        "***MASKED***",
		LogLatency:       true,
		LogUserAgent:     true,
		LogIP:            true,
		SkipPaths:        []string{}, // Don't skip any paths for audit
	})
}

// CustomFieldsLogger creates a logger with custom fields extractor
func CustomFieldsLogger(logger contracts.Logger, customFields func(ctx *context.Context) map[string]any) context.MiddlewareFunc {
	config := DefaultLoggerConfig()
	config.Logger = logger
	config.CustomFields = customFields
	return RequestResponseLoggerWithConfig(config)
}

// LoggerWithSkipper creates a logger with custom skip logic
func LoggerWithSkipper(logger contracts.Logger, skipper func(ctx *context.Context) bool) context.MiddlewareFunc {
	config := DefaultLoggerConfig()
	config.Logger = logger
	config.Skipper = skipper
	return RequestResponseLoggerWithConfig(config)
}

// Helper to create regex patterns for skip paths
func MustCompilePatterns(patterns []string) []*regexp.Regexp {
	compiled := make([]*regexp.Regexp, len(patterns))
	for i, pattern := range patterns {
		compiled[i] = regexp.MustCompile(pattern)
	}
	return compiled
}
