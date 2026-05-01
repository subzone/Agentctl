// Package llm defines the provider-agnostic LLM interface used by the agent
// engine. Concrete providers (Anthropic, OpenAI, Ollama) live in subpackages
// and self-register via init(). The engine never imports a provider directly.
package llm

import (
	"context"
	"encoding/json"
)

// Role is the speaker of a Message.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// BlockType discriminates ContentBlock payloads. Mirrors Anthropic's
// content-block model since it is the most expressive of the providers; other
// adapters translate as needed.
type BlockType string

const (
	BlockText       BlockType = "text"
	BlockToolUse    BlockType = "tool_use"
	BlockToolResult BlockType = "tool_result"
)

// ContentBlock is a single typed piece of a Message. A text-only message has
// one block of type BlockText. An assistant turn that calls a tool has one or
// more BlockToolUse blocks (possibly interleaved with BlockText). The result
// returned to the model is a user message containing BlockToolResult blocks.
type ContentBlock struct {
	Type BlockType

	// BlockText
	Text string

	// BlockToolUse (assistant → tool call request)
	ToolID    string
	ToolName  string
	ToolInput json.RawMessage

	// BlockToolResult (user → tool execution response)
	ToolUseID string
	Output    string
	IsError   bool
}

// Message is one turn in a conversation.
type Message struct {
	Role    Role
	Content []ContentBlock
}

// TextMessage is a convenience constructor for plain-text turns.
func TextMessage(role Role, text string) Message {
	return Message{Role: role, Content: []ContentBlock{{Type: BlockText, Text: text}}}
}

// ToolSchema describes a tool the model may call.
type ToolSchema struct {
	Name        string
	Description string
	InputSchema json.RawMessage
}

// Request is a single completion call. Model is the bare model id (provider
// prefix already stripped by Resolve).
type Request struct {
	Model       string
	System      string
	Messages    []Message
	Tools       []ToolSchema
	Temperature *float64
	MaxTokens   int
}

// Usage carries token counts for a single provider round-trip.
type Usage struct {
	InputTokens  int
	OutputTokens int
}

// EventKind discriminates streamed Event payloads.
type EventKind int

const (
	// EventText is a text token chunk to append to the assistant reply.
	EventText EventKind = iota
	// EventToolCall is emitted once per tool invocation, after the model has
	// produced the full tool_use block (id, name, input JSON).
	EventToolCall
	// EventDone is emitted once when the model has finished its turn.
	// StopReason carries the reason ("end_turn", "tool_use", "max_tokens"...).
	EventDone
	// EventUsage is emitted once per response with token counts. Providers
	// that don't report usage simply omit this event.
	EventUsage
	// EventError is a terminal failure event; the channel closes after.
	EventError
)

// Event is a single item in a streamed response.
type Event struct {
	Kind EventKind

	// EventText
	Text string

	// EventToolCall
	ToolID    string
	ToolName  string
	ToolInput json.RawMessage

	// EventDone
	StopReason string

	// EventUsage
	Usage Usage

	// EventError
	Err error
}

// Provider is the contract every backend implements.
type Provider interface {
	// Stream returns a channel that yields Events until the response is
	// complete or ctx is cancelled. The channel is always closed by the
	// implementation. An error returned synchronously means no events were
	// emitted (e.g. transport failure before the stream opened).
	Stream(ctx context.Context, req Request) (<-chan Event, error)
}
