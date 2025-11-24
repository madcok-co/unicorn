package contracts

import "context"

// Logger adalah generic interface untuk logging
// Implementasi bisa zap, zerolog, logrus, slog, dll
type Logger interface {
	// Log levels
	Debug(msg string, fields ...any)
	Info(msg string, fields ...any)
	Warn(msg string, fields ...any)
	Error(msg string, fields ...any)
	Fatal(msg string, fields ...any)

	// With context - untuk tracing/correlation ID
	WithContext(ctx context.Context) Logger

	// With fields - untuk menambahkan fields ke semua log berikutnya
	WithFields(fields ...any) Logger

	// With error - untuk attach error ke log
	WithError(err error) Logger

	// Named logger - untuk sub-logger dengan prefix
	Named(name string) Logger

	// Sync flushes any buffered log entries
	Sync() error
}

// LoggerConfig untuk konfigurasi logger
type LoggerConfig struct {
	// Level: debug, info, warn, error
	Level string

	// Format: json, console, text
	Format string

	// Output: stdout, stderr, file path
	Output string

	// File rotation (if output is file)
	MaxSize    int  // megabytes
	MaxBackups int  // number of backups
	MaxAge     int  // days
	Compress   bool // compress rotated files

	// Additional options
	AddCaller     bool // add caller info
	AddStacktrace bool // add stacktrace on error

	// Fields yang selalu ditambahkan
	DefaultFields map[string]any
}

// LogLevel constants
const (
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
)

// LogFormat constants
const (
	LogFormatJSON    = "json"
	LogFormatConsole = "console"
	LogFormatText    = "text"
)
