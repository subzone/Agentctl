package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// NamespaceSeparator joins an MCP server's tool_prefix with the upstream
// tool name. Double-underscore is safe under Anthropic's and OpenAI's
// tool-name regex (`^[a-zA-Z0-9_-]{1,64}$`).
const NamespaceSeparator = "__"

// ToolAdapter wraps an MCP tool so it satisfies tools.Tool. The model
// sees a namespaced name (e.g. "github__create_issue"); the adapter
// strips the prefix when calling the upstream server.
type ToolAdapter struct {
	transport  Transport
	upstream   Tool
	namespaced string
}

// NewToolAdapter binds upstream to client and constructs the namespaced
// name. If prefix is empty, the upstream name is used as-is.
func NewToolAdapter(client *Client, upstream Tool, prefix string) *ToolAdapter {
	return NewToolAdapterTransport(client, upstream, prefix)
}

// NewToolAdapterTransport binds upstream to any Transport implementation.
func NewToolAdapterTransport(t Transport, upstream Tool, prefix string) *ToolAdapter {
	name := sanitize(upstream.Name)
	if prefix != "" {
		name = sanitize(prefix) + NamespaceSeparator + name
	}
	return &ToolAdapter{transport: t, upstream: upstream, namespaced: name}
}

// Name implements tools.Tool.
func (a *ToolAdapter) Name() string { return a.namespaced }

// Description implements tools.Tool.
func (a *ToolAdapter) Description() string { return a.upstream.Description }

// InputSchema implements tools.Tool. Defaults to an empty object schema
// so providers don't reject the tool when an MCP server omits one.
func (a *ToolAdapter) InputSchema() json.RawMessage {
	if len(a.upstream.InputSchema) == 0 {
		return json.RawMessage(`{"type":"object"}`)
	}
	return a.upstream.InputSchema
}

// Run implements tools.Tool. Upstream errors (transport/protocol) become
// Go errors; an isError=true result is rendered as a non-nil error so the
// engine flags it accordingly to the model.
func (a *ToolAdapter) Run(ctx context.Context, input json.RawMessage) (string, error) {
	out, isErr, err := a.transport.CallTool(ctx, a.upstream.Name, input)
	if err != nil {
		return out, err
	}
	if isErr {
		if out == "" {
			out = "(no detail)"
		}
		return out, fmt.Errorf("upstream tool reported error")
	}
	return out, nil
}

// sanitize replaces characters that providers' tool-name regexes reject
// with underscores. Idempotent for already-clean names.
func sanitize(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		switch {
		case (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '_' || r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}
