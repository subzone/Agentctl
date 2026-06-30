package a2a

import "encoding/json"

// JSON-RPC 2.0 envelopes used by the A2A message endpoint.

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// JSON-RPC standard error codes.
const (
	codeParseError     = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternalError  = -32603
)

// Part is a single piece of a message. Only text parts are supported.
type Part struct {
	Kind string `json:"kind"`
	Text string `json:"text,omitempty"`
}

// Message is an A2A message (a turn from user or agent).
type Message struct {
	Role      string `json:"role"` // "user" or "agent"
	Parts     []Part `json:"parts"`
	MessageID string `json:"messageId,omitempty"`
	Kind      string `json:"kind"` // always "message"
}

// sendParams is the params object for the message/send method.
type sendParams struct {
	Message Message `json:"message"`
}

// Text concatenates all text parts of a message.
func (m Message) Text() string {
	var s string
	for _, p := range m.Parts {
		if p.Kind == "text" || p.Kind == "" {
			s += p.Text
		}
	}
	return s
}

// TextMessage builds a single-text-part message.
func TextMessage(role, id, text string) Message {
	return Message{
		Role:      role,
		Parts:     []Part{{Kind: "text", Text: text}},
		MessageID: id,
		Kind:      "message",
	}
}
