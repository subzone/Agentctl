package mcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/subzone/Agentctl/internal/config"
	"github.com/subzone/Agentctl/internal/tools"
)

// Manager owns a set of running MCP clients and exposes their tools as
// adapters in a tools.Registry.
type Manager struct {
	clients []namedTransport
	tools   []tools.Tool
}

type namedTransport struct {
	name      string
	transport Transport
}

// Open spawns the MCP servers in specs (filtered by allowed_agents if
// agentName is set), runs initialize + tools/list against each, and
// returns a Manager whose Registry() exposes namespaced tools. On any
// error the partially-opened clients are closed before returning.
//
// Servers whose transport is not stdio are skipped with a warning on
// status; M5 supports stdio only.
func Open(ctx context.Context, specs []*config.MCPServerSpec, agentName string, status io.Writer) (*Manager, error) {
	if status == nil {
		status = io.Discard
	}
	m := &Manager{}

	for _, spec := range specs {
		if !isAllowedFor(spec, agentName) {
			fmt.Fprintf(status, "skipping mcp server %q: agent %q not in allowed_agents\n", spec.Name, agentName)
			continue
		}

		var transport Transport
		var err error

		switch spec.Transport {
		case config.TransportStdio:
			transport, err = NewProcessClient(ctx, spec.Command, spec.Env)
		case config.TransportHTTP:
			if spec.URL == "" {
				fmt.Fprintf(status, "skipping mcp server %q: http transport requires url\n", spec.Name)
				continue
			}
			transport = NewHTTPClient(spec.URL, false)
		case config.TransportSSE:
			if spec.URL == "" {
				fmt.Fprintf(status, "skipping mcp server %q: sse transport requires url\n", spec.Name)
				continue
			}
			transport = NewHTTPClient(spec.URL, true)
		default:
			fmt.Fprintf(status, "skipping mcp server %q: unknown transport %q\n", spec.Name, spec.Transport)
			continue
		}
		if err != nil {
			// A server that can't start (e.g. the binary isn't installed)
			// should not take the whole agent down. Warn and carry on with
			// whatever else connected — consistent with the skip-and-log
			// handling for unsupported transports and disallowed agents.
			fmt.Fprintf(status, "skipping mcp server %q: %v\n", spec.Name, err)
			continue
		}

		if err := transport.Initialize(ctx, "agent", "0.1"); err != nil {
			_ = transport.Close()
			fmt.Fprintf(status, "skipping mcp server %q: initialize: %v\n", spec.Name, err)
			continue
		}
		listing, err := transport.ListTools(ctx)
		if err != nil {
			_ = transport.Close()
			fmt.Fprintf(status, "skipping mcp server %q: tools/list: %v\n", spec.Name, err)
			continue
		}

		prefix := spec.ToolPrefix
		if prefix == "" {
			prefix = spec.Name
		}
		for _, ut := range listing {
			m.tools = append(m.tools, NewToolAdapterTransport(transport, ut, prefix))
		}
		m.clients = append(m.clients, namedTransport{name: spec.Name, transport: transport})
		fmt.Fprintf(status, "mcp %s (%s): %d tool(s) available\n", spec.Name, spec.Transport, len(listing))
	}
	return m, nil
}

// Registry returns a fresh registry containing every tool exposed by every
// open client. Builtins are not included; callers compose them externally.
func (m *Manager) Registry() *tools.Registry {
	return tools.NewRegistry(m.tools...)
}

// HealthCheck checks the health of each MCP server by calling ListTools.
// Returns a map of server name to error (nil if healthy).
func (m *Manager) HealthCheck(ctx context.Context) map[string]error {
	results := make(map[string]error)
	for _, nc := range m.clients {
		// Use a short timeout for health check
		hctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_, err := nc.transport.ListTools(hctx)
		results[nc.name] = err
	}
	return results
}

// Close shuts down every client. The first error is returned; later errors
// are dropped so we always tear all servers down.
func (m *Manager) Close() error {
	return m.closeAll()
}

func (m *Manager) closeAll() error {
	var first error
	for _, nc := range m.clients {
		if err := nc.transport.Close(); err != nil && first == nil {
			first = fmt.Errorf("mcp %s: close: %w", nc.name, err)
		}
	}
	m.clients = nil
	m.tools = nil
	return first
}

func isAllowedFor(spec *config.MCPServerSpec, agent string) bool {
	if len(spec.AllowedAgents) == 0 {
		return true
	}
	if agent == "" {
		return true
	}
	for _, a := range spec.AllowedAgents {
		if a == agent {
			return true
		}
	}
	return false
}

// Resolve picks out MCPServerSpec documents from a doc collection by name.
// Unknown refs are returned in missing.
func Resolve(docs []*config.Document, refs []string) (specs []*config.MCPServerSpec, missing []string) {
	byName := make(map[string]*config.MCPServerSpec)
	for _, d := range docs {
		if s, ok := d.Spec.(*config.MCPServerSpec); ok {
			byName[s.Name] = s
		}
	}
	for _, ref := range refs {
		if s, ok := byName[ref]; ok {
			specs = append(specs, s)
			continue
		}
		missing = append(missing, ref)
	}
	return specs, missing
}

// ErrNoTransport is returned when no spec is usable (every entry skipped).
var ErrNoTransport = errors.New("mcp: no usable servers")
