package mcp

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/milenkom81/m/internal/config"
	"github.com/milenkom81/m/internal/tools"
)

// Manager owns a set of running MCP clients and exposes their tools as
// adapters in a tools.Registry.
type Manager struct {
	clients []namedClient
	tools   []tools.Tool
}

type namedClient struct {
	name   string
	client *Client
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
		if spec.Transport != config.TransportStdio {
			fmt.Fprintf(status, "skipping mcp server %q: transport %q not yet supported (stdio only)\n", spec.Name, spec.Transport)
			continue
		}

		client, err := NewProcessClient(ctx, spec.Command, spec.Env)
		if err != nil {
			m.closeAll()
			return nil, fmt.Errorf("mcp %s: %w", spec.Name, err)
		}
		if err := client.Initialize(ctx, "agent", "0.1"); err != nil {
			_ = client.Close()
			m.closeAll()
			return nil, fmt.Errorf("mcp %s: initialize: %w", spec.Name, err)
		}
		listing, err := client.ListTools(ctx)
		if err != nil {
			_ = client.Close()
			m.closeAll()
			return nil, fmt.Errorf("mcp %s: tools/list: %w", spec.Name, err)
		}

		prefix := spec.ToolPrefix
		if prefix == "" {
			prefix = spec.Name
		}
		for _, ut := range listing {
			m.tools = append(m.tools, NewToolAdapter(client, ut, prefix))
		}
		m.clients = append(m.clients, namedClient{name: spec.Name, client: client})
		fmt.Fprintf(status, "mcp %s: %d tool(s) available\n", spec.Name, len(listing))
	}
	return m, nil
}

// Registry returns a fresh registry containing every tool exposed by every
// open client. Builtins are not included; callers compose them externally.
func (m *Manager) Registry() *tools.Registry {
	return tools.NewRegistry(m.tools...)
}

// Close shuts down every client. The first error is returned; later errors
// are dropped so we always tear all servers down.
func (m *Manager) Close() error {
	return m.closeAll()
}

func (m *Manager) closeAll() error {
	var first error
	for _, nc := range m.clients {
		if err := nc.client.Close(); err != nil && first == nil {
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
