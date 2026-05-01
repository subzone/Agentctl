// Package ports defines the interfaces that decouple the engine and CLI
// from concrete infrastructure. Adapters (filesystem, SQLite, Vault, etc.)
// implement these interfaces; the engine and CLI depend only on the ports.
//
// This is the "ports" half of ports-and-adapters (hexagonal architecture).
// Adding a new backend (e.g. SQLite state store, AWS Secrets Manager) means
// writing a new adapter — no engine changes required.
package ports

import "context"

// ConfigSource loads and saves the user's CLI configuration (provider,
// model, base URL, default agent). The filesystem adapter is the default;
// a k8s operator might use a ConfigMap adapter instead.
type ConfigSource interface {
	Load() (*Config, error)
	Save(cfg *Config) error
	Exists() bool
}

// Config mirrors userconfig.Config but lives in the ports package so
// adapters don't import the CLI layer.
type Config struct {
	Provider     string
	Model        string
	BaseURL      string
	DefaultAgent string
}

// Secrets reads and writes API keys. The default adapters use the OS
// keychain (macOS security CLI, Linux secret-tool). Enterprise deployments
// might use Vault, AWS Secrets Manager, or environment variables.
type Secrets interface {
	Get(ctx context.Context, provider string) (string, error)
	Set(ctx context.Context, provider, key string) error
	Delete(ctx context.Context, provider string) error
}

// StateStore persists session state across invocations. The default
// adapter is in-memory (no persistence). A filesystem or SQLite adapter
// enables conversation resumption.
type StateStore interface {
	// SaveSession persists a named session's messages and metadata.
	SaveSession(ctx context.Context, id string, data *SessionData) error
	// LoadSession retrieves a previously saved session. Returns
	// ErrNotFound if the session doesn't exist.
	LoadSession(ctx context.Context, id string) (*SessionData, error)
	// ListSessions returns the ids of all saved sessions.
	ListSessions(ctx context.Context) ([]string, error)
	// DeleteSession removes a saved session.
	DeleteSession(ctx context.Context, id string) error
}

// SessionData is the serializable state of a conversation.
type SessionData struct {
	Provider     string
	Model        string
	Messages     []Message
	InputTokens  int
	OutputTokens int
}

// Message is a provider-agnostic serializable message. It mirrors
// llm.Message but uses plain strings so the ports package doesn't
// import the llm package.
type Message struct {
	Role    string
	Content []ContentBlock
}

// ContentBlock is a serializable content block.
type ContentBlock struct {
	Type      string
	Text      string
	ToolID    string
	ToolName  string
	ToolInput string
	ToolUseID string
	Output    string
	IsError   bool
}

// ErrNotFound is returned by StateStore when a session doesn't exist.
type ErrNotFound struct{ ID string }

func (e *ErrNotFound) Error() string { return "session not found: " + e.ID }
