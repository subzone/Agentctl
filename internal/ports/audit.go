package ports

import (
	"context"
	"encoding/json"
	"time"
)

// AuditSink receives structured audit events.
type AuditSink interface {
	Emit(ctx context.Context, event AuditEvent) error
	Flush(ctx context.Context) error
	Close() error
}

// AuditEvent is the provider-agnostic schema used by the engine and CLI.
type AuditEvent struct {
	EventID      string          `json:"event_id,omitempty"`
	Type         string          `json:"type"`
	SessionID    string          `json:"session_id,omitempty"`
	TS           time.Time       `json:"ts,omitempty"`
	Agent        string          `json:"agent,omitempty"`
	Model        string          `json:"model,omitempty"`
	Tool         string          `json:"tool,omitempty"`
	Args         json.RawMessage `json:"args,omitempty"`
	Outcome      string          `json:"outcome,omitempty"`
	Error        string          `json:"error,omitempty"`
	DurationMS   int64           `json:"duration_ms,omitempty"`
	InputTokens  int             `json:"input_tokens,omitempty"`
	OutputTokens int             `json:"output_tokens,omitempty"`
	CostUSD      float64         `json:"cost_usd,omitempty"`
	Metadata     map[string]any  `json:"metadata,omitempty"`
	HMAC         string          `json:"hmac,omitempty"`
}
