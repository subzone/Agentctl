package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/subzone/Agentctl/internal/a2a"
)

// A2ACallTool lets an agent delegate a task to a remote A2A agent over HTTP.
// This is the agent-to-agent counterpart of MCP: instead of calling a tool on
// a server, the agent sends a natural-language task to another agent and gets
// its reply back.
type A2ACallTool struct{}

// NewA2ACall constructs the a2a_call tool.
func NewA2ACall() *A2ACallTool { return &A2ACallTool{} }

func (t *A2ACallTool) Name() string { return "a2a_call" }

func (t *A2ACallTool) Description() string {
	return "Delegate a task to a remote agent over the A2A protocol. " +
		"Provide the agent's base URL (e.g. http://localhost:8080) and a natural-language message. " +
		"Returns the remote agent's reply."
}

func (t *A2ACallTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "url": {"type": "string", "description": "Base URL of the remote A2A agent, e.g. http://localhost:8080"},
    "message": {"type": "string", "description": "The task or question to send to the remote agent"}
  },
  "required": ["url", "message"]
}`)
}

func (t *A2ACallTool) Run(ctx context.Context, input json.RawMessage) (string, error) {
	var args struct {
		URL     string `json:"url"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if args.URL == "" {
		return "", fmt.Errorf("url is required")
	}
	if args.Message == "" {
		return "", fmt.Errorf("message is required")
	}
	reply, err := a2a.NewClient(args.URL).Send(ctx, args.Message)
	if err != nil {
		return "", fmt.Errorf("a2a call to %s: %w", args.URL, err)
	}
	return reply, nil
}
