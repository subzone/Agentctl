package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/subzone/m/internal/config"
	"github.com/subzone/m/internal/engine"
	"github.com/subzone/m/internal/llm"
	"github.com/subzone/m/internal/mcp"
	"github.com/subzone/m/internal/tools"
)

// MaxSubagentDepth caps recursive delegation. The hub runs at depth 0; a
// subagent it spawns runs at depth 1; a sub-subagent at depth 2. Depth 2
// is the deepest agent allowed. Matches PLAN.md M6 ("two levels deep").
const MaxSubagentDepth = 2

// spawner is the back end of the delegate builtin. It owns the project's
// parsed companion docs, the destinations for output and status, and the
// depth at which the next subagent will run.
type spawner struct {
	docs       []*config.Document
	out        io.Writer
	status     io.Writer
	spawnDepth int // depth of the agent that this spawner.spawn() will create
}

// spawn runs the named subagent with task and returns its final assistant
// text. The subagent's tokens stream to s.out prefixed with "[<name>] " so
// the user can see the delegated reasoning live; the same text is captured
// and returned to the parent.
func (s *spawner) spawn(ctx context.Context, name, task string) (string, error) {
	if s.spawnDepth > MaxSubagentDepth {
		return "", fmt.Errorf("delegate: max subagent depth (%d) reached; cannot spawn %q", MaxSubagentDepth, name)
	}

	doc, err := findAgentDoc(s.docs, name)
	if err != nil {
		return "", err
	}
	spec := doc.Spec.(*config.AgentSpec)

	provider, model, err := llm.Resolve(spec.Model)
	if err != nil {
		return "", err
	}

	childPrefix := fmt.Sprintf("[%s] ", name)
	childOut := &prefixWriter{w: s.out, prefix: childPrefix}
	childStatus := &prefixWriter{w: s.status, prefix: childPrefix}

	fmt.Fprintf(s.status, "→ delegate %s: %s\n", name, summarizeTask(task))

	childSpawner := &spawner{
		docs:       s.docs,
		out:        s.out,
		status:     s.status,
		spawnDepth: s.spawnDepth + 1,
	}

	rt, err := buildAgentRuntime(ctx, spec, s.docs, childSpawner, childStatus)
	if err != nil {
		return "", fmt.Errorf("delegate %s: %w", name, err)
	}
	defer rt.close()

	maxTokens := defaultMaxTokens
	if spec.MaxTokens != nil {
		maxTokens = *spec.MaxTokens
	}

	var captured bytes.Buffer
	cfg := engine.Config{
		Provider:    provider,
		Model:       model,
		System:      doc.Body,
		Tools:       rt.registry,
		Temperature: spec.Temperature,
		MaxTokens:   maxTokens,
		Out:         io.MultiWriter(&captured, childOut),
		Status:      childStatus,
	}
	if err := engine.Run(ctx, cfg, task); err != nil {
		fmt.Fprintf(s.status, "← delegate %s: error: %v\n", name, err)
		return "", err
	}

	out := strings.TrimRight(captured.String(), "\n")
	fmt.Fprintf(s.status, "← delegate %s (%d bytes)\n", name, len(out))
	return out, nil
}

// agentRuntime bundles the tool registry an agent will use plus any
// resources (MCP managers, etc) that need to be torn down when its run
// completes.
type agentRuntime struct {
	registry *tools.Registry
	cleanups []func() error
}

func (r *agentRuntime) close() {
	for _, c := range r.cleanups {
		_ = c()
	}
	r.cleanups = nil
}

// buildAgentRuntime composes the tool set for an agent: builtins, plus
// MCP-backed tools the agent references, plus the delegate tool when
// subagents are listed and depth allows. The agent's `tools:` allowlist
// is then applied; an empty allowlist exposes only builtins (+ delegate).
//
// sp may be nil, in which case the delegate tool is omitted regardless of
// the agent's `subagents:` field. status is where warnings are written.
func buildAgentRuntime(
	ctx context.Context,
	spec *config.AgentSpec,
	docs []*config.Document,
	sp *spawner,
	status io.Writer,
) (*agentRuntime, error) {
	if status == nil {
		status = io.Discard
	}
	rt := &agentRuntime{}
	combined := tools.Builtins()

	if len(spec.MCP) > 0 {
		specs, missing := mcp.Resolve(docs, spec.MCP)
		for _, m := range missing {
			fmt.Fprintf(status, "warning: agent references unknown mcp_server %q\n", m)
		}
		mgr, err := mcp.Open(ctx, specs, spec.Name, status)
		if err != nil {
			return nil, err
		}
		rt.cleanups = append(rt.cleanups, mgr.Close)
		combined = tools.Merge(combined, mgr.Registry())
	}

	if len(spec.Subagents) > 0 && sp != nil && sp.spawnDepth <= MaxSubagentDepth {
		combined = tools.Merge(combined, tools.NewRegistry(
			tools.NewDelegate(sp.spawn, spec.Subagents),
		))
	}

	if len(spec.Tools) == 0 {
		// No allowlist: builtins only, plus delegate if available, no MCP. The
		// rationale is the same as M5 — third-party servers shouldn't be
		// auto-exposed; users opt in via `tools:`.
		out := tools.Builtins()
		if d, ok := combined.Get("delegate"); ok {
			out = tools.Merge(out, tools.NewRegistry(d))
		}
		rt.registry = out
		return rt, nil
	}

	list := append([]string(nil), spec.Tools...)
	if _, ok := combined.Get("delegate"); ok && !contains(list, "delegate") {
		list = append(list, "delegate")
	}
	registry, missing := combined.Filter(list)
	for _, m := range missing {
		fmt.Fprintf(status, "warning: tool %q is not registered; skipping\n", m)
	}
	rt.registry = registry
	return rt, nil
}

func findAgentDoc(docs []*config.Document, name string) (*config.Document, error) {
	for _, d := range docs {
		if a, ok := d.Spec.(*config.AgentSpec); ok && a.Name == name {
			return d, nil
		}
	}
	return nil, fmt.Errorf("subagent %q not found in companion docs", name)
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func summarizeTask(s string) string {
	const limit = 80
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > limit {
		return s[:limit] + "..."
	}
	return s
}

// prefixWriter wraps an underlying writer and writes p.prefix at the start
// of each line in the stream. State persists across Write calls so that
// streamed deltas (which often arrive sub-line) get exactly one prefix
// per line of output.
type prefixWriter struct {
	w       io.Writer
	prefix  string
	midLine bool
}

// Write implements io.Writer.
func (p *prefixWriter) Write(b []byte) (int, error) {
	n := len(b)
	for len(b) > 0 {
		if !p.midLine {
			if _, err := io.WriteString(p.w, p.prefix); err != nil {
				return 0, err
			}
			p.midLine = true
		}
		i := bytes.IndexByte(b, '\n')
		if i < 0 {
			if _, err := p.w.Write(b); err != nil {
				return 0, err
			}
			return n, nil
		}
		if _, err := p.w.Write(b[:i+1]); err != nil {
			return 0, err
		}
		p.midLine = false
		b = b[i+1:]
	}
	return n, nil
}
