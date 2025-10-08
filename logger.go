// ============================================
// UNICORN Framework - Logger Interface
// Unified logging interface for the framework
// ============================================

package unicorn

// Logger defines the logging interface used throughout UNICORN.
// Implementations can use any logging library (zap, logrus, etc.).
type Logger interface {
	// Debug logs a debug message
	Debug(msg string, keysAndValues ...interface{})

	// Info logs an info message
	Info(msg string, keysAndValues ...interface{})

	// Warn logs a warning message
	Warn(msg string, keysAndValues ...interface{})

	// Error logs an error message
	Error(msg string, keysAndValues ...interface{})

	// With creates a child logger with additional fields
	With(keysAndValues ...interface{}) Logger
}

// SetGlobalLogger sets the global logger instance.
var globalLogger Logger

// SetGlobalLogger sets the global logger.
func SetGlobalLogger(logger Logger) {
	globalLogger = logger
}

// GetGlobalLogger returns the global logger.
func GetGlobalLogger() Logger {
	if globalLogger == nil {
		return &noopLogger{}
	}
	return globalLogger
}
