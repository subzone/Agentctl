package output

import (
	"encoding/json"
	"io"
	"strings"
	"sync"
	"time"
)

// JSONEmitter writes NDJSON events for CI consumption.
type JSONEmitter struct {
	enc *json.Encoder
	mu  sync.Mutex
}

func NewJSONEmitter(w io.Writer) *JSONEmitter {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return &JSONEmitter{enc: enc}
}

func (e *JSONEmitter) Emit(v map[string]any) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	_ = e.enc.Encode(v)
}

func (e *JSONEmitter) SessionStart(sessionID, agent string) {
	e.Emit(map[string]any{
		"type":       "session_start",
		"session_id": sessionID,
		"agent":      agent,
		"ts":         time.Now().UTC().Format(time.RFC3339),
	})
}

func (e *JSONEmitter) ToolCall(tool, args string) {
	e.Emit(map[string]any{
		"type": "tool_call",
		"tool": tool,
		"args": map[string]any{"raw": args},
	})
}

func (e *JSONEmitter) ToolResult(raw string, isError bool) {
	outcome := "success"
	if isError {
		outcome = "error"
	}
	e.Emit(map[string]any{
		"type":    "tool_result",
		"outcome": outcome,
		"raw":     raw,
	})
}

func (e *JSONEmitter) LLMResponse(input, output int, cost float64) {
	e.Emit(map[string]any{
		"type": "llm_response",
		"tokens": map[string]any{
			"input":  input,
			"output": output,
		},
		"cost_usd": cost,
	})
}

func (e *JSONEmitter) SessionEnd(sessionID, outcome string, turns int, totalCost float64) {
	e.Emit(map[string]any{
		"type":           "session_end",
		"session_id":     sessionID,
		"outcome":        outcome,
		"total_cost_usd": totalCost,
		"turns":          turns,
	})
}

// StatusEventWriter parses engine status lines into tool events.
type StatusEventWriter struct {
	emitter *JSONEmitter
	buf     string
}

func NewStatusEventWriter(emitter *JSONEmitter) *StatusEventWriter {
	return &StatusEventWriter{emitter: emitter}
}

func (w *StatusEventWriter) Write(p []byte) (int, error) {
	w.buf += string(p)
	for {
		idx := strings.IndexByte(w.buf, '\n')
		if idx < 0 {
			break
		}
		line := strings.TrimSpace(w.buf[:idx])
		w.buf = w.buf[idx+1:]
		w.handleLine(line)
	}
	return len(p), nil
}

func (w *StatusEventWriter) handleLine(line string) {
	if w.emitter == nil || line == "" {
		return
	}
	if strings.HasPrefix(line, "→ ") {
		rest := strings.TrimPrefix(line, "→ ")
		parts := strings.SplitN(rest, " ", 2)
		tool := parts[0]
		args := ""
		if len(parts) == 2 {
			args = parts[1]
		}
		w.emitter.ToolCall(tool, args)
		return
	}
	if strings.HasPrefix(line, "← ") {
		raw := strings.TrimPrefix(line, "← ")
		w.emitter.ToolResult(raw, strings.HasPrefix(raw, "error:"))
	}
}
