package logster

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
)

type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

type Format string

const (
	FormatJSON Format = "json"
)

type Sage struct {
	Env    string
	Group  string
	System string
}

type Config struct {
	MinLevel Level
	Format   Format
	Sage     Sage
}

type Logger struct {
	mu     sync.Mutex
	level  Level
	logger *slog.Logger
	attrs  []any
}

func LevelFromString(v string) (Level, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "debug":
		return LevelDebug, nil
	case "info":
		return LevelInfo, nil
	case "warn", "warning":
		return LevelWarn, nil
	case "error":
		return LevelError, nil
	case "fatal":
		return LevelFatal, nil
	default:
		return LevelInfo, fmt.Errorf("unknown log level %q", v)
	}
}

func NewLogger(cfg Config) (*Logger, error) {
	lvl := slog.LevelInfo
	switch cfg.MinLevel {
	case LevelDebug:
		lvl = slog.LevelDebug
	case LevelWarn:
		lvl = slog.LevelWarn
	case LevelError, LevelFatal:
		lvl = slog.LevelError
	}

	opts := &slog.HandlerOptions{
		Level:     lvl,
		AddSource: false,
	}

	var handler slog.Handler = slog.NewJSONHandler(os.Stdout, opts)
	if cfg.Format != FormatJSON {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	l := &Logger{
		level:  cfg.MinLevel,
		logger: slog.New(handler),
		attrs: []any{
			"env", cfg.Sage.Env,
			"group", cfg.Sage.Group,
			"system", cfg.Sage.System,
		},
	}
	return l, nil
}

func (l *Logger) Close() error { return nil }

func (l *Logger) Debugf(format string, args ...any) { l.logf(LevelDebug, format, args...) }
func (l *Logger) Infof(format string, args ...any)  { l.logf(LevelInfo, format, args...) }
func (l *Logger) Warnf(format string, args ...any)  { l.logf(LevelWarn, format, args...) }
func (l *Logger) Errorf(format string, args ...any) { l.logf(LevelError, format, args...) }

func (l *Logger) Fatalf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	l.log(LevelFatal, msg)
	panic(msg)
}

func (l *Logger) Debug(msg string, kv ...any) { l.log(LevelDebug, msg, kv...) }
func (l *Logger) Info(msg string, kv ...any)  { l.log(LevelInfo, msg, kv...) }
func (l *Logger) Warn(msg string, kv ...any)  { l.log(LevelWarn, msg, kv...) }
func (l *Logger) Error(msg string, kv ...any) { l.log(LevelError, msg, kv...) }

func (l *Logger) Fatal(msg string, kv ...any) {
	l.log(LevelFatal, msg, kv...)
	panic(msg)
}

func (l *Logger) logf(level Level, format string, args ...any) {
	l.log(level, fmt.Sprintf(format, args...))
}

func (l *Logger) log(level Level, msg string, kv ...any) {
	if l == nil || l.logger == nil {
		return
	}
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	args := make([]any, 0, len(l.attrs)+len(kv))
	args = append(args, l.attrs...)
	args = append(args, kv...)

	switch level {
	case LevelDebug:
		l.logger.Debug(msg, args...)
	case LevelInfo:
		l.logger.Info(msg, args...)
	case LevelWarn:
		l.logger.Warn(msg, args...)
	default:
		l.logger.Error(msg, args...)
	}
}
