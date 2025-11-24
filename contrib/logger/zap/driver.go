// Package zap provides a Zap implementation of the unicorn Logger interface.
//
// Usage:
//
//	import (
//	    "github.com/madcok-co/unicorn/contrib/logger/zap"
//	)
//
//	// Using default production logger
//	driver := zap.NewDriver()
//
//	// Using custom zap logger
//	zapLogger, _ := zap.NewProduction()
//	driver := zap.NewDriverWithLogger(zapLogger)
//
//	app.SetLogger(driver)
package zap

import (
	"context"
	"os"

	"github.com/madcok-co/unicorn/core/pkg/contracts"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Driver implements contracts.Logger using Zap
type Driver struct {
	logger *zap.Logger
	sugar  *zap.SugaredLogger
}

// Config for creating a new Zap driver
type Config struct {
	Level         string         // debug, info, warn, error
	Format        string         // json, console
	Output        string         // stdout, stderr, or file path
	AddCaller     bool           // add caller information
	AddStacktrace bool           // add stacktrace on error level
	DefaultFields map[string]any // fields added to all logs
}

// DefaultConfig returns a sensible default configuration
func DefaultConfig() *Config {
	return &Config{
		Level:         "info",
		Format:        "json",
		Output:        "stdout",
		AddCaller:     true,
		AddStacktrace: true,
	}
}

// NewDriver creates a new Zap logger driver with default production settings
func NewDriver() *Driver {
	return NewDriverWithConfig(DefaultConfig())
}

// NewDriverWithConfig creates a new Zap logger driver with custom config
func NewDriverWithConfig(cfg *Config) *Driver {
	// Parse log level
	var level zapcore.Level
	switch cfg.Level {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	default:
		level = zapcore.InfoLevel
	}

	// Configure encoder
	var encoder zapcore.Encoder
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	if cfg.Format == "console" {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	// Configure output
	var output zapcore.WriteSyncer
	switch cfg.Output {
	case "stdout", "":
		output = zapcore.AddSync(os.Stdout)
	case "stderr":
		output = zapcore.AddSync(os.Stderr)
	default:
		file, err := os.OpenFile(cfg.Output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			output = zapcore.AddSync(os.Stdout)
		} else {
			output = zapcore.AddSync(file)
		}
	}

	// Build core
	core := zapcore.NewCore(encoder, output, level)

	// Build options
	opts := []zap.Option{}
	if cfg.AddCaller {
		opts = append(opts, zap.AddCaller(), zap.AddCallerSkip(1))
	}
	if cfg.AddStacktrace {
		opts = append(opts, zap.AddStacktrace(zapcore.ErrorLevel))
	}

	// Add default fields
	if len(cfg.DefaultFields) > 0 {
		fields := make([]zap.Field, 0, len(cfg.DefaultFields))
		for k, v := range cfg.DefaultFields {
			fields = append(fields, zap.Any(k, v))
		}
		opts = append(opts, zap.Fields(fields...))
	}

	logger := zap.New(core, opts...)

	return &Driver{
		logger: logger,
		sugar:  logger.Sugar(),
	}
}

// NewDriverWithLogger creates a driver from an existing Zap logger
func NewDriverWithLogger(logger *zap.Logger) *Driver {
	return &Driver{
		logger: logger,
		sugar:  logger.Sugar(),
	}
}

// Logger returns the underlying Zap logger
func (d *Driver) Logger() *zap.Logger {
	return d.logger
}

// Debug logs a debug message
func (d *Driver) Debug(msg string, fields ...any) {
	d.sugar.Debugw(msg, fields...)
}

// Info logs an info message
func (d *Driver) Info(msg string, fields ...any) {
	d.sugar.Infow(msg, fields...)
}

// Warn logs a warning message
func (d *Driver) Warn(msg string, fields ...any) {
	d.sugar.Warnw(msg, fields...)
}

// Error logs an error message
func (d *Driver) Error(msg string, fields ...any) {
	d.sugar.Errorw(msg, fields...)
}

// Fatal logs a fatal message and exits
func (d *Driver) Fatal(msg string, fields ...any) {
	d.sugar.Fatalw(msg, fields...)
}

// WithContext returns a logger with context (for tracing)
func (d *Driver) WithContext(ctx context.Context) contracts.Logger {
	// Extract trace ID from context if available
	if traceID := ctx.Value("trace_id"); traceID != nil {
		return d.WithFields("trace_id", traceID)
	}
	if requestID := ctx.Value("request_id"); requestID != nil {
		return d.WithFields("request_id", requestID)
	}
	return d
}

// WithFields returns a logger with additional fields
func (d *Driver) WithFields(fields ...any) contracts.Logger {
	return &Driver{
		logger: d.logger,
		sugar:  d.sugar.With(fields...),
	}
}

// WithError returns a logger with error field
func (d *Driver) WithError(err error) contracts.Logger {
	return &Driver{
		logger: d.logger,
		sugar:  d.sugar.With("error", err.Error()),
	}
}

// Named returns a named sub-logger
func (d *Driver) Named(name string) contracts.Logger {
	return &Driver{
		logger: d.logger.Named(name),
		sugar:  d.logger.Named(name).Sugar(),
	}
}

// Sync flushes any buffered log entries
func (d *Driver) Sync() error {
	return d.logger.Sync()
}

// Ensure Driver implements contracts.Logger
var _ contracts.Logger = (*Driver)(nil)
