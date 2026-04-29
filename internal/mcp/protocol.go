// Package mcp implements a minimal Model Context Protocol client over the
// stdio transport. JSON-RPC 2.0 messages are exchanged as newline-delimited
// JSON on the server's stdin/stdout. We speak just enough of the protocol
// to discover and call tools (initialize, tools/list, tools/call); other
// MCP capabilities (resources, prompts, sampling, roots) are not used.
package mcp

import "encoding/json"

const (
	// JSONRPCVersion is the JSON-RPC dialect we and the server agree on.
	JSONRPCVersion = "2.0"
	// ProtocolVersion is the MCP version we advertise during initialize.
	ProtocolVersion = "2024-11-05"
)

// rpcRequest / rpcResponse / rpcNotification are the wire envelopes. Field
// types match the JSON-RPC spec: id may be string|number|null, params and
// result may be omitted entirely.
type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

type rpcNotification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// RPCError is a JSON-RPC error object surfaced to callers.
type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Tool is a single tool listing returned by tools/list.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// toolsListResult is the result envelope of tools/list.
type toolsListResult struct {
	Tools []Tool `json:"tools"`
}

// toolsCallParams is the params envelope of tools/call.
type toolsCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// toolsCallResult is the result envelope of tools/call.
type toolsCallResult struct {
	Content []contentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// contentItem is one element of a tools/call result. Only "text" is
// surfaced today; image/resource items are skipped.
type contentItem struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}
