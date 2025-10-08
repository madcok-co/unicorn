// ============================================
// 4. LOGGING MIDDLEWARE
// ============================================
package middleware

import (
	"time"

	"github.com/madcok-co/unicorn"
)

type LoggingMiddleware struct {
	logger unicorn.Logger
}

func NewLoggingMiddleware(logger unicorn.Logger) *LoggingMiddleware {
	return &LoggingMiddleware{
		logger: logger,
	}
}

func (m *LoggingMiddleware) Handle(ctx *unicorn.Context, next func() (interface{}, error)) (interface{}, error) {
	start := time.Now()

	// Log request
	m.logger.Info("Request started",
		"request_id", ctx.RequestID(),
		"service", ctx.GetMetadataString("service_name"),
	)

	// Execute next
	result, err := next()

	// Log response
	duration := time.Since(start)
	if err != nil {
		m.logger.Error("Request failed",
			"request_id", ctx.RequestID(),
			"duration", duration,
			"error", err,
		)
	} else {
		m.logger.Info("Request completed",
			"request_id", ctx.RequestID(),
			"duration", duration,
		)
	}

	return result, err
}
