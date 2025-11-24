// Package logger provides a generic logger adapter
// that wraps any logging library (zap, zerolog, logrus, slog, etc.)
package logger

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

// Driver is the interface that any logger must implement
type Driver interface {
	Log(level Level, msg string, fields ...any)
	Sync() error
}

// Level represents log level
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	case LevelFatal:
		return "fatal"
	default:
		return "unknown"
	}
}

// ParseLevel parses level string
func ParseLevel(s string) Level {
	switch s {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	case "fatal":
		return LevelFatal
	default:
		return LevelInfo
	}
}

// Adapter implements contracts.Logger
type Adapter struct {
	driver        Driver
	level         Level
	fields        []any
	contextFields []any
	name          string
	errField      error
}

// New creates a new logger adapter
func New(driver Driver) *Adapter {
	return &Adapter{
		driver: driver,
		level:  LevelInfo,
		fields: make([]any, 0),
	}
}

// WithLevel sets minimum log level
func (a *Adapter) WithLevel(level Level) *Adapter {
	a.level = level
	return a
}

// ============ contracts.Logger Implementation ============

func (a *Adapter) Debug(msg string, fields ...any) {
	if a.level <= LevelDebug {
		a.log(LevelDebug, msg, fields...)
	}
}

func (a *Adapter) Info(msg string, fields ...any) {
	if a.level <= LevelInfo {
		a.log(LevelInfo, msg, fields...)
	}
}

func (a *Adapter) Warn(msg string, fields ...any) {
	if a.level <= LevelWarn {
		a.log(LevelWarn, msg, fields...)
	}
}

func (a *Adapter) Error(msg string, fields ...any) {
	if a.level <= LevelError {
		a.log(LevelError, msg, fields...)
	}
}

func (a *Adapter) Fatal(msg string, fields ...any) {
	a.log(LevelFatal, msg, fields...)
	os.Exit(1)
}

func (a *Adapter) WithContext(ctx context.Context) contracts.Logger {
	newAdapter := a.clone()

	// Extract trace ID if available
	if traceID := ctx.Value("trace_id"); traceID != nil {
		newAdapter.contextFields = append(newAdapter.contextFields, "trace_id", traceID)
	}

	// Extract request ID if available
	if requestID := ctx.Value("request_id"); requestID != nil {
		newAdapter.contextFields = append(newAdapter.contextFields, "request_id", requestID)
	}

	return newAdapter
}

func (a *Adapter) WithFields(fields ...any) contracts.Logger {
	newAdapter := a.clone()
	newAdapter.fields = append(newAdapter.fields, fields...)
	return newAdapter
}

func (a *Adapter) WithError(err error) contracts.Logger {
	newAdapter := a.clone()
	newAdapter.errField = err
	return newAdapter
}

func (a *Adapter) Named(name string) contracts.Logger {
	newAdapter := a.clone()
	if newAdapter.name != "" {
		newAdapter.name = newAdapter.name + "." + name
	} else {
		newAdapter.name = name
	}
	return newAdapter
}

func (a *Adapter) Sync() error {
	return a.driver.Sync()
}

func (a *Adapter) log(level Level, msg string, fields ...any) {
	allFields := make([]any, 0, len(a.fields)+len(a.contextFields)+len(fields)+4)

	// Add name if present
	if a.name != "" {
		allFields = append(allFields, "logger", a.name)
	}

	// Add error if present
	if a.errField != nil {
		allFields = append(allFields, "error", a.errField.Error())
	}

	// Add context fields
	allFields = append(allFields, a.contextFields...)

	// Add default fields
	allFields = append(allFields, a.fields...)

	// Add call-site fields
	allFields = append(allFields, fields...)

	a.driver.Log(level, msg, allFields...)
}

func (a *Adapter) clone() *Adapter {
	return &Adapter{
		driver:        a.driver,
		level:         a.level,
		fields:        append([]any{}, a.fields...),
		contextFields: append([]any{}, a.contextFields...),
		name:          a.name,
	}
}

// ============ Standard Logger Driver ============

// StdDriver wraps standard log package
type StdDriver struct {
	logger *log.Logger
	format Format
	mu     sync.Mutex
}

// Format represents output format
type Format int

const (
	FormatText Format = iota
	FormatJSON
)

// NewStdDriver creates a standard logger driver
func NewStdDriver(w io.Writer, format Format) *StdDriver {
	if w == nil {
		w = os.Stdout
	}
	return &StdDriver{
		logger: log.New(w, "", 0),
		format: format,
	}
}

func (d *StdDriver) Log(level Level, msg string, fields ...any) {
	d.mu.Lock()
	defer d.mu.Unlock()

	timestamp := time.Now().Format(time.RFC3339)

	if d.format == FormatJSON {
		d.logJSON(timestamp, level, msg, fields...)
	} else {
		d.logText(timestamp, level, msg, fields...)
	}
}

func (d *StdDriver) logJSON(timestamp string, level Level, msg string, fields ...any) {
	// Build JSON manually for zero dependencies
	json := fmt.Sprintf(`{"time":"%s","level":"%s","msg":"%s"`, timestamp, level, escapeJSON(msg))

	for i := 0; i < len(fields)-1; i += 2 {
		key := fmt.Sprintf("%v", fields[i])
		value := fields[i+1]

		switch v := value.(type) {
		case string:
			json += fmt.Sprintf(`,"%s":"%s"`, key, escapeJSON(v))
		case int, int64, float64, bool:
			json += fmt.Sprintf(`,"%s":%v`, key, v)
		default:
			json += fmt.Sprintf(`,"%s":"%v"`, key, v)
		}
	}

	json += "}"
	d.logger.Println(json)
}

func (d *StdDriver) logText(timestamp string, level Level, msg string, fields ...any) {
	line := fmt.Sprintf("%s [%s] %s", timestamp, level, msg)

	for i := 0; i < len(fields)-1; i += 2 {
		line += fmt.Sprintf(" %v=%v", fields[i], fields[i+1])
	}

	d.logger.Println(line)
}

func (d *StdDriver) Sync() error {
	return nil
}

func escapeJSON(s string) string {
	result := ""
	for _, r := range s {
		switch r {
		case '"':
			result += `\"`
		case '\\':
			result += `\\`
		case '\n':
			result += `\n`
		case '\r':
			result += `\r`
		case '\t':
			result += `\t`
		default:
			result += string(r)
		}
	}
	return result
}

// ============ Noop Driver ============

// NoopDriver discards all logs
type NoopDriver struct{}

// NewNoopDriver creates a no-op driver
func NewNoopDriver() *NoopDriver {
	return &NoopDriver{}
}

func (d *NoopDriver) Log(level Level, msg string, fields ...any) {}
func (d *NoopDriver) Sync() error                                { return nil }

// ============ Zap Wrapper ============

// ZapLogger is the interface that zap.Logger implements
type ZapLogger interface {
	Debug(msg string, fields ...any)
	Info(msg string, fields ...any)
	Warn(msg string, fields ...any)
	Error(msg string, fields ...any)
	Fatal(msg string, fields ...any)
	Sync() error
	With(fields ...any) ZapLogger
	Named(name string) ZapLogger
}

// ZapDriver wraps zap logger
type ZapDriver struct {
	logger       ZapLogger
	fieldAdapter func(fields []any) []any
}

// WrapZap wraps a zap logger
// fieldAdapter converts key-value pairs to zap.Field if needed
func WrapZap(logger ZapLogger, fieldAdapter func([]any) []any) *ZapDriver {
	return &ZapDriver{
		logger:       logger,
		fieldAdapter: fieldAdapter,
	}
}

func (d *ZapDriver) Log(level Level, msg string, fields ...any) {
	adaptedFields := fields
	if d.fieldAdapter != nil {
		adaptedFields = d.fieldAdapter(fields)
	}

	switch level {
	case LevelDebug:
		d.logger.Debug(msg, adaptedFields...)
	case LevelInfo:
		d.logger.Info(msg, adaptedFields...)
	case LevelWarn:
		d.logger.Warn(msg, adaptedFields...)
	case LevelError:
		d.logger.Error(msg, adaptedFields...)
	case LevelFatal:
		d.logger.Fatal(msg, adaptedFields...)
	}
}

func (d *ZapDriver) Sync() error {
	return d.logger.Sync()
}

// ============ Zerolog Wrapper ============

// ZerologLogger is the interface for zerolog
type ZerologLogger interface {
	Debug() ZerologEvent
	Info() ZerologEvent
	Warn() ZerologEvent
	Error() ZerologEvent
	Fatal() ZerologEvent
}

// ZerologEvent is the interface for zerolog event
type ZerologEvent interface {
	Str(key, val string) ZerologEvent
	Int(key string, val int) ZerologEvent
	Int64(key string, val int64) ZerologEvent
	Float64(key string, val float64) ZerologEvent
	Bool(key string, val bool) ZerologEvent
	Interface(key string, val any) ZerologEvent
	Msg(msg string)
}

// ZerologDriver wraps zerolog
type ZerologDriver struct {
	logger ZerologLogger
}

// WrapZerolog wraps a zerolog logger
func WrapZerolog(logger ZerologLogger) *ZerologDriver {
	return &ZerologDriver{logger: logger}
}

func (d *ZerologDriver) Log(level Level, msg string, fields ...any) {
	var event ZerologEvent

	switch level {
	case LevelDebug:
		event = d.logger.Debug()
	case LevelInfo:
		event = d.logger.Info()
	case LevelWarn:
		event = d.logger.Warn()
	case LevelError:
		event = d.logger.Error()
	case LevelFatal:
		event = d.logger.Fatal()
	default:
		event = d.logger.Info()
	}

	// Add fields
	for i := 0; i < len(fields)-1; i += 2 {
		key := fmt.Sprintf("%v", fields[i])
		value := fields[i+1]

		switch v := value.(type) {
		case string:
			event = event.Str(key, v)
		case int:
			event = event.Int(key, v)
		case int64:
			event = event.Int64(key, v)
		case float64:
			event = event.Float64(key, v)
		case bool:
			event = event.Bool(key, v)
		default:
			event = event.Interface(key, v)
		}
	}

	event.Msg(msg)
}

func (d *ZerologDriver) Sync() error {
	return nil // zerolog doesn't need sync
}

// ============ Slog Wrapper (Go 1.21+) ============

// SlogLogger is the interface for slog.Logger
type SlogLogger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// SlogDriver wraps slog
type SlogDriver struct {
	logger SlogLogger
}

// WrapSlog wraps a slog logger
func WrapSlog(logger SlogLogger) *SlogDriver {
	return &SlogDriver{logger: logger}
}

func (d *SlogDriver) Log(level Level, msg string, fields ...any) {
	switch level {
	case LevelDebug:
		d.logger.Debug(msg, fields...)
	case LevelInfo:
		d.logger.Info(msg, fields...)
	case LevelWarn:
		d.logger.Warn(msg, fields...)
	case LevelError, LevelFatal:
		d.logger.Error(msg, fields...)
	}
}

func (d *SlogDriver) Sync() error {
	return nil
}

// ============ Multi Logger ============

// MultiDriver logs to multiple drivers
type MultiDriver struct {
	drivers []Driver
}

// NewMultiDriver creates a driver that logs to multiple destinations
func NewMultiDriver(drivers ...Driver) *MultiDriver {
	return &MultiDriver{drivers: drivers}
}

func (d *MultiDriver) Log(level Level, msg string, fields ...any) {
	for _, driver := range d.drivers {
		driver.Log(level, msg, fields...)
	}
}

func (d *MultiDriver) Sync() error {
	var lastErr error
	for _, driver := range d.drivers {
		if err := driver.Sync(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// ============ Helper Functions ============

// NewConsoleLogger creates a logger that outputs to console
func NewConsoleLogger(level string) contracts.Logger {
	return New(NewStdDriver(os.Stdout, FormatText)).WithLevel(ParseLevel(level))
}

// NewJSONLogger creates a logger that outputs JSON
func NewJSONLogger(level string) contracts.Logger {
	return New(NewStdDriver(os.Stdout, FormatJSON)).WithLevel(ParseLevel(level))
}

// NewFileLogger creates a logger that outputs to file
func NewFileLogger(path string, level string, format Format) (contracts.Logger, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}
	return New(NewStdDriver(f, format)).WithLevel(ParseLevel(level)), nil
}
