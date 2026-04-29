package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// SpawnFunc runs a named subagent with a task and returns its final
// assistant text. Implementations are responsible for cycle detection,
// depth limiting, and resource cleanup; the tool just calls through.
type SpawnFunc func(ctx context.Context, name, task string) (string, error)

// DelegateTool exposes a "delegate" function to the model so it can hand
// off subtasks to subagents listed in its frontmatter.
type DelegateTool struct {
	spawn     SpawnFunc
	available []string
}

// NewDelegate constructs a DelegateTool. available is advertised to the
// model in the description and as an enum in the input schema; the tool
// also rejects calls naming a subagent not in the list, as a defense in
// depth against models hallucinating an out-of-policy delegation.
func NewDelegate(spawn SpawnFunc, available []string) *DelegateTool {
	cp := append([]string(nil), available...)
	return &DelegateTool{spawn: spawn, available: cp}
}

// Name implements Tool.
func (d *DelegateTool) Name() string { return "delegate" }

// Description implements Tool.
func (d *DelegateTool) Description() string {
	if len(d.available) == 0 {
		return "Delegate a subtask to a named subagent. (no subagents currently available)"
	}
	return fmt.Sprintf(
		"Delegate a subtask to a named subagent and wait for its final reply. "+
			"Available subagents: %s. Use this when a different agent is better "+
			"suited to handle a portion of the work; pass the full context they need.",
		strings.Join(d.available, ", "),
	)
}

// InputSchema implements Tool.
func (d *DelegateTool) InputSchema() json.RawMessage {
	name := map[string]any{
		"type":        "string",
		"description": "subagent name (must be one of the available subagents)",
	}
	if len(d.available) > 0 {
		// JSON-marshal of []string preserves order; provider schema validators
		// accept enums of plain strings.
		name["enum"] = d.available
	}
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": name,
			"task": map[string]any{
				"type":        "string",
				"description": "task description for the subagent (full context, not just a hint)",
			},
		},
		"required": []string{"name", "task"},
	}
	b, _ := json.Marshal(schema)
	return b
}

// Run implements Tool.
func (d *DelegateTool) Run(ctx context.Context, input json.RawMessage) (string, error) {
	var args struct {
		Name string `json:"name"`
		Task string `json:"task"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	if args.Name == "" {
		return "", errors.New("name is required")
	}
	if args.Task == "" {
		return "", errors.New("task is required")
	}
	if !d.allowed(args.Name) {
		return "", fmt.Errorf("subagent %q is not in this agent's subagents list", args.Name)
	}
	if d.spawn == nil {
		return "", errors.New("delegate: no spawn function wired (engine misconfigured)")
	}
	return d.spawn(ctx, args.Name, args.Task)
}

func (d *DelegateTool) allowed(name string) bool {
	for _, a := range d.available {
		if a == name {
			return true
		}
	}
	return false
}
