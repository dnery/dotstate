// Package logging provides structured logging for dotstate.
//
// The logging system writes to two destinations:
//  1. Structured JSON logs to a file (state/logs/dot.log)
//  2. Human-readable logs to stderr (when verbose mode is enabled)
//
// Usage:
//
//	logger := logging.New(logging.Config{
//	    Verbose:  true,
//	    LogDir:   "/path/to/logs",
//	    LogLevel: logging.LevelInfo,
//	})
//	defer logger.Close()
//
//	logger.Info("operation completed", "count", 42, "path", "/some/path")
package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Level represents a log level.
type Level = slog.Level

// Log levels.
const (
	LevelDebug = slog.LevelDebug
	LevelInfo  = slog.LevelInfo
	LevelWarn  = slog.LevelWarn
	LevelError = slog.LevelError
)

// ParseLevel parses a log level from a string.
func ParseLevel(s string) (Level, error) {
	switch strings.ToLower(s) {
	case "debug":
		return LevelDebug, nil
	case "info":
		return LevelInfo, nil
	case "warn", "warning":
		return LevelWarn, nil
	case "error":
		return LevelError, nil
	default:
		return LevelInfo, fmt.Errorf("unknown log level: %s", s)
	}
}

// Config configures the logger.
type Config struct {
	// Verbose enables human-readable output to stderr.
	Verbose bool

	// LogDir is the directory for log files.
	// If empty, file logging is disabled.
	LogDir string

	// LogLevel is the minimum level to log.
	// Defaults to LevelInfo.
	LogLevel Level

	// StderrLevel is the minimum level for stderr output.
	// Only applies when Verbose is true.
	// Defaults to LevelInfo.
	StderrLevel Level
}

// Logger is the dotstate logger.
type Logger struct {
	slog    *slog.Logger
	file    *os.File
	mu      sync.Mutex
	closed  bool
	verbose bool
}

// New creates a new logger with the given configuration.
func New(cfg Config) (*Logger, error) {
	var handlers []slog.Handler

	l := &Logger{
		verbose: cfg.Verbose,
	}

	// File handler (JSON)
	if cfg.LogDir != "" {
		if err := os.MkdirAll(cfg.LogDir, 0o755); err != nil {
			return nil, fmt.Errorf("create log directory: %w", err)
		}

		logPath := filepath.Join(cfg.LogDir, "dot.log")
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, fmt.Errorf("open log file: %w", err)
		}
		l.file = f

		jsonHandler := slog.NewJSONHandler(f, &slog.HandlerOptions{
			Level: cfg.LogLevel,
		})
		handlers = append(handlers, jsonHandler)
	}

	// Stderr handler (human-readable)
	if cfg.Verbose {
		stderrLevel := cfg.StderrLevel
		if stderrLevel == 0 {
			stderrLevel = LevelInfo
		}
		textHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: stderrLevel,
		})
		handlers = append(handlers, textHandler)
	}

	// Create a combined handler
	if len(handlers) == 0 {
		// No handlers configured, use a no-op handler
		l.slog = slog.New(noopHandler{})
	} else if len(handlers) == 1 {
		l.slog = slog.New(handlers[0])
	} else {
		l.slog = slog.New(&multiHandler{handlers: handlers})
	}

	return l, nil
}

// NewNoop creates a no-op logger that discards all output.
func NewNoop() *Logger {
	return &Logger{
		slog: slog.New(noopHandler{}),
	}
}

// Close closes the log file if one is open.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return nil
	}
	l.closed = true

	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// With returns a new logger with the given attributes.
func (l *Logger) With(args ...any) *Logger {
	return &Logger{
		slog:    l.slog.With(args...),
		file:    l.file,
		verbose: l.verbose,
	}
}

// WithGroup returns a new logger with the given group name.
func (l *Logger) WithGroup(name string) *Logger {
	return &Logger{
		slog:    l.slog.WithGroup(name),
		file:    l.file,
		verbose: l.verbose,
	}
}

// Debug logs at debug level.
func (l *Logger) Debug(msg string, args ...any) {
	l.slog.Debug(msg, args...)
}

// Info logs at info level.
func (l *Logger) Info(msg string, args ...any) {
	l.slog.Info(msg, args...)
}

// Warn logs at warning level.
func (l *Logger) Warn(msg string, args ...any) {
	l.slog.Warn(msg, args...)
}

// Error logs at error level.
func (l *Logger) Error(msg string, args ...any) {
	l.slog.Error(msg, args...)
}

// Log logs at the given level.
func (l *Logger) Log(ctx context.Context, level Level, msg string, args ...any) {
	l.slog.Log(ctx, level, msg, args...)
}

// Slog returns the underlying slog.Logger for advanced usage.
func (l *Logger) Slog() *slog.Logger {
	return l.slog
}

// multiHandler fans out log records to multiple handlers.
type multiHandler struct {
	handlers []slog.Handler
}

func (h *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h *multiHandler) Handle(ctx context.Context, record slog.Record) error {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, record.Level) {
			if err := handler.Handle(ctx, record); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		newHandlers[i] = handler.WithAttrs(attrs)
	}
	return &multiHandler{handlers: newHandlers}
}

func (h *multiHandler) WithGroup(name string) slog.Handler {
	newHandlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		newHandlers[i] = handler.WithGroup(name)
	}
	return &multiHandler{handlers: newHandlers}
}

// noopHandler discards all log records.
type noopHandler struct{}

func (noopHandler) Enabled(context.Context, slog.Level) bool  { return false }
func (noopHandler) Handle(context.Context, slog.Record) error { return nil }
func (h noopHandler) WithAttrs([]slog.Attr) slog.Handler      { return h }
func (h noopHandler) WithGroup(string) slog.Handler           { return h }

// PrettyHandler formats log output for human consumption.
// This is an alternative to TextHandler with colored output.
type PrettyHandler struct {
	w     io.Writer
	level slog.Level
	attrs []slog.Attr
	group string
	mu    *sync.Mutex
}

// NewPrettyHandler creates a handler that writes colored, human-readable output.
func NewPrettyHandler(w io.Writer, level slog.Level) *PrettyHandler {
	return &PrettyHandler{
		w:     w,
		level: level,
		mu:    &sync.Mutex{},
	}
}

func (h *PrettyHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *PrettyHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Format: time level message key=value...
	ts := r.Time.Format(time.TimeOnly)

	var levelStr string
	switch r.Level {
	case slog.LevelDebug:
		levelStr = "\033[36mDBG\033[0m" // Cyan
	case slog.LevelInfo:
		levelStr = "\033[32mINF\033[0m" // Green
	case slog.LevelWarn:
		levelStr = "\033[33mWRN\033[0m" // Yellow
	case slog.LevelError:
		levelStr = "\033[31mERR\033[0m" // Red
	default:
		levelStr = "???"
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%s %s %s", ts, levelStr, r.Message)

	// Add stored attrs
	for _, attr := range h.attrs {
		fmt.Fprintf(&sb, " \033[90m%s\033[0m=%v", attr.Key, attr.Value)
	}

	// Add record attrs
	r.Attrs(func(attr slog.Attr) bool {
		fmt.Fprintf(&sb, " \033[90m%s\033[0m=%v", attr.Key, attr.Value)
		return true
	})

	sb.WriteString("\n")

	_, err := io.WriteString(h.w, sb.String())
	return err
}

func (h *PrettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, 0, len(h.attrs)+len(attrs))
	newAttrs = append(newAttrs, h.attrs...)
	newAttrs = append(newAttrs, attrs...)
	return &PrettyHandler{
		w:     h.w,
		level: h.level,
		attrs: newAttrs,
		group: h.group,
		mu:    h.mu,
	}
}

func (h *PrettyHandler) WithGroup(name string) slog.Handler {
	return &PrettyHandler{
		w:     h.w,
		level: h.level,
		attrs: h.attrs,
		group: name,
		mu:    h.mu,
	}
}
