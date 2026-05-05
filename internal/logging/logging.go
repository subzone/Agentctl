// Package logging provides structured logging for AgentCTL using stdlib slog.
// Default level is Warn (silent in normal use). Enable debug with --debug flag
// or /debug command.
package logging

import (
	"io"
	"log/slog"
	"os"
	"sync"
)

var (
	mu     sync.Mutex
	logger *slog.Logger
	level  = new(slog.LevelVar)
)

func init() {
	level.Set(slog.LevelWarn)
	logger = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}

// Init configures the logger output and level.
func Init(w io.Writer, debug bool) {
	mu.Lock()
	defer mu.Unlock()
	if debug {
		level.Set(slog.LevelDebug)
	} else {
		level.Set(slog.LevelWarn)
	}
	logger = slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}))
}

// SetDebug toggles debug level at runtime (for /debug command).
func SetDebug(on bool) {
	if on {
		level.Set(slog.LevelDebug)
	} else {
		level.Set(slog.LevelWarn)
	}
}

// IsDebug reports whether debug logging is active.
func IsDebug() bool { return level.Level() == slog.LevelDebug }

func Debug(msg string, args ...any) { logger.Debug(msg, args...) }
func Info(msg string, args ...any)  { logger.Info(msg, args...) }
func Warn(msg string, args ...any)  { logger.Warn(msg, args...) }
func Error(msg string, args ...any) { logger.Error(msg, args...) }
