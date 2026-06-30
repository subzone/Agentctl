package desktop

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/subzone/Agentctl/internal/config"
	"github.com/subzone/Agentctl/internal/mcp"
)

// MCPDoc is a user-authored MCP server MD file surfaced to the MCP tab.
type MCPDoc struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Transport   string `json:"transport"`
	Path        string `json:"path"`
	Content     string `json:"content"`
	Error       string `json:"error,omitempty"`
}

// userMCPDir is where MCP server definitions live (shared with the CLI's
// `m mcp` registry). Each file (type: mcp_server) is opened at session start
// and its tools are namespaced and merged into the agent's toolset.
func userMCPDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "m", "mcp")
}

// ListMCP returns all MCP server MD files in ~/.config/m/mcp.
func (a *App) ListMCP() []MCPDoc {
	dir := userMCPDir()
	out := []MCPDoc{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return out
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		md := MCPDoc{
			Name:    strings.TrimSuffix(e.Name(), ".md"),
			Path:    p,
			Content: string(data),
		}
		if doc, err := config.Parse(data); err != nil {
			md.Error = err.Error()
		} else if spec, ok := doc.Spec.(*config.MCPServerSpec); ok {
			md.Name = spec.Name
			md.Description = spec.Description
			md.Transport = string(spec.Transport)
		} else {
			md.Error = "document type is not 'mcp_server'"
		}
		out = append(out, md)
	}
	return out
}

// SaveMCP validates and writes an MCP server MD file. oldName is the file's
// previous name (empty for new servers); a rename removes the old file.
func (a *App) SaveMCP(oldName, content string) error {
	doc, err := config.Parse([]byte(content))
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	spec, ok := doc.Spec.(*config.MCPServerSpec)
	if !ok {
		return errors.New("frontmatter 'type' must be 'mcp_server'")
	}
	if issues := config.Validate(doc); len(issues) > 0 {
		msgs := make([]string, 0, len(issues))
		for _, is := range issues {
			if is.Field == "name" {
				msgs = append(msgs, "name must be lowercase letters, numbers, '.', '-' or '_' (e.g. my_server) and start with a letter or number")
				continue
			}
			msgs = append(msgs, is.Error())
		}
		return errors.New(strings.Join(msgs, "; "))
	}
	dir := userMCPDir()
	if dir == "" {
		return errors.New("no user config directory available")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	dst := filepath.Join(dir, spec.Name+".md")
	if oldName != spec.Name {
		if _, err := os.Stat(dst); err == nil {
			return fmt.Errorf("an MCP server named %q already exists — choose a different name", spec.Name)
		}
	}
	if err := os.WriteFile(dst, []byte(content), 0o644); err != nil {
		return err
	}
	if oldName != "" && oldName != spec.Name {
		_ = os.Remove(filepath.Join(dir, oldName+".md"))
	}
	return nil
}

// DeleteMCP removes an MCP server MD file by name.
func (a *App) DeleteMCP(name string) error {
	dir := userMCPDir()
	if dir == "" {
		return errors.New("no user config directory available")
	}
	if strings.TrimSpace(name) == "" {
		return errors.New("name is required")
	}
	return os.Remove(filepath.Join(dir, name+".md"))
}

// NewMCPTemplate returns starter MD for a new MCP server.
func (a *App) NewMCPTemplate() string {
	return `---
name: my_server
type: mcp_server
description: One-line summary of what this MCP server provides.
transport: stdio
command:
  - npx
  - "-y"
  - "@modelcontextprotocol/server-filesystem"
  - /path/to/expose
tool_prefix: fsx
# allowed_agents: [m]   # optional: restrict this server to specific agents
---
This server's tools are namespaced with tool_prefix and merged into the agent's
toolset on your next New Chat. For stdio servers set a 'command:' (launched as a
subprocess); for sse/http servers drop 'command:' and set a 'url:' instead.
`
}

// loadUserMCPSpecs parses every MCP server MD file in the user MCP dir. Files
// that fail to parse or aren't MCP servers are skipped.
func loadUserMCPSpecs() []*config.MCPServerSpec {
	dir := userMCPDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var specs []*config.MCPServerSpec
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		doc, err := config.ParseFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		if spec, ok := doc.Spec.(*config.MCPServerSpec); ok {
			specs = append(specs, spec)
		}
	}
	return specs
}

// openUserMCP opens every configured MCP server for the given agent and returns
// a Manager whose tools should be merged into the session. Returns nil when no
// servers are configured. The Manager (when non-nil) must be closed via the
// session's cleanups. Startup failures are logged to status and skipped, so a
// single broken server never blocks the chat.
func (a *App) openUserMCP(agentName string, status io.Writer) *mcp.Manager {
	specs := loadUserMCPSpecs()
	if len(specs) == 0 {
		return nil
	}
	// a.ctx (app lifetime) backs the subprocesses; they're torn down when the
	// session is closed via mgr.Close, not by this context.
	mgr, err := mcp.Open(a.ctx, specs, agentName, status)
	if err != nil {
		fmt.Fprintf(status, "mcp: %v\n", err)
		return nil
	}
	return mgr
}
