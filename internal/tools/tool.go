// Package tools defines the Tool interface and a registry for builtins.
// MCP-backed tools will live alongside builtins in the same registry once
// M5 lands; the contract is identical.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// Tool is something the model can invoke. Run executes the tool with the
// model-provided JSON input and returns either a text result (passed back
// to the model verbatim) or an error.
//
// Convention: a tool that wraps a process whose own exit code communicates
// failure (e.g. shell) should return (output, nil) and let the model see
// the exit info inside the output. Reserve a non-nil error for failures of
// the tool itself: bad input, transport errors, timeouts.
type Tool interface {
	Name() string
	Description() string
	InputSchema() json.RawMessage
	Run(ctx context.Context, input json.RawMessage) (string, error)
}

// Registry holds a set of tools indexed by name.
type Registry struct {
	tools map[string]Tool
}

// NewRegistry constructs a registry from the given tools. Later entries
// override earlier ones if names collide.
func NewRegistry(ts ...Tool) *Registry {
	r := &Registry{tools: make(map[string]Tool, len(ts))}
	for _, t := range ts {
		r.tools[t.Name()] = t
	}
	return r
}

// Get returns a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	if r == nil {
		return nil, false
	}
	t, ok := r.tools[name]
	return t, ok
}

// All returns the registered tools. Order is unspecified.
func (r *Registry) All() []Tool {
	if r == nil {
		return nil
	}
	out := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t)
	}
	return out
}

// Filter returns a new registry containing only the named tools, plus the
// list of names that were not found. Callers typically use the missing
// list to warn the user that an agent's allowlist references unknown tools.
func (r *Registry) Filter(names []string) (*Registry, []string) {
	out := &Registry{tools: make(map[string]Tool, len(names))}
	var missing []string
	for _, n := range names {
		if t, ok := r.Get(n); ok {
			out.tools[n] = t
		} else {
			missing = append(missing, n)
		}
	}
	return out, missing
}

// Run executes a tool by name.
func (r *Registry) Run(ctx context.Context, name string, input json.RawMessage) (string, error) {
	t, ok := r.Get(name)
	if !ok {
		return "", fmt.Errorf("unknown tool %q", name)
	}
	return t.Run(ctx, input)
}

// Builtins returns a registry preloaded with the always-available tools.
// confirm gates fs_write; pass nil for auto-approve (non-interactive runs).
func Builtins(confirm ConfirmFunc, undo *UndoStack) *Registry {
	return NewRegistry(
		NewShell(), NewFSRead(), NewFSWrite(confirm, undo),
		NewFSWriteMulti(confirm, undo),
		NewFSList(), NewGit(), NewTestRun(), NewWebFetch(),
		NewCodeSearch(), NewKnowledge(), NewA2ACall(),
	)
}

// Merge combines registries into a single one. Later registries override
// earlier ones on name collisions.
func Merge(regs ...*Registry) *Registry {
	out := &Registry{tools: make(map[string]Tool)}
	for _, r := range regs {
		if r == nil {
			continue
		}
		for name, t := range r.tools {
			out.tools[name] = t
		}
	}
	return out
}
