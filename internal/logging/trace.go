package logging

import (
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"
)

var sessionCounter atomic.Int64

// SessionID generates a short unique session identifier.
func SessionID() string {
	n := sessionCounter.Add(1)
	return fmt.Sprintf("s%d-%d", time.Now().Unix()%100000, n)
}

// Span tracks the duration of an operation. Usage:
//
//	end := logging.Span("llm.stream", "model", model)
//	defer end()
func Span(op string, args ...any) func() {
	if level.Level() > slog.LevelDebug {
		return func() {}
	}
	start := time.Now()
	allArgs := append([]any{"op", op}, args...)
	logger.Debug("span.start", allArgs...)
	return func() {
		allArgs = append(allArgs, "duration_ms", time.Since(start).Milliseconds())
		logger.Debug("span.end", allArgs...)
	}
}
