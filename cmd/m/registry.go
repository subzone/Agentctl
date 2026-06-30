package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// agentRegistryDir returns ~/.config/m/agents/ (the user's installed agents).
func agentRegistryDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "m", "agents")
}

// mcpRegistryDir returns ~/.config/m/mcp/ (installed MCP server definitions).
// These are loaded as companion docs so agents can reference them by name.
func mcpRegistryDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "m", "mcp")
}

// toolsRegistryDir returns ~/.config/m/tools/ (user-authored tool MD files).
func toolsRegistryDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "m", "tools")
}

// resolveAgentPath resolves an agent reference to a file path.
// Checks in order:
//  1. Literal path (if file exists)
//  2. ~/.config/m/agents/<name>.md
//  3. ./examples/agents/<name>.md
//  4. <name>.md in current directory
//  5. Bundled (embedded) agents
func resolveAgentPath(ref string) string {
	// 1. Literal path.
	if _, err := os.Stat(ref); err == nil {
		return ref
	}
	// Strip .md if provided for name-based lookup.
	name := strings.TrimSuffix(ref, ".md")

	// 2. User registry.
	if dir := agentRegistryDir(); dir != "" {
		p := filepath.Join(dir, name+".md")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// 3. Examples directory (dev/clone scenario).
	p := filepath.Join("examples", "agents", name+".md")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	// 4. Current directory.
	p = name + ".md"
	if _, err := os.Stat(p); err == nil {
		return p
	}
	// 5. Bundled (embedded) — extract on demand.
	if bp := bundledAgentPath(name); bp != "" {
		return bp
	}
	// Not found — return original ref so the caller gets a clear error.
	return ref
}

func newInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install <agent.md>",
		Short: "Install an agent to the user registry",
		Long:  "Copies an agent .md file to ~/.config/m/agents/ so it can be run by name:\n  m install ./my-agent.md\n  m run my-agent \"task\"",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			src := args[0]
			if _, err := os.Stat(src); err != nil {
				return fmt.Errorf("file not found: %s", src)
			}
			dir := agentRegistryDir()
			if dir == "" {
				return fmt.Errorf("cannot determine config directory")
			}
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return err
			}
			dst := filepath.Join(dir, filepath.Base(src))
			in, err := os.Open(src)
			if err != nil {
				return err
			}
			defer in.Close()
			out, err := os.Create(dst)
			if err != nil {
				return err
			}
			defer out.Close()
			if _, err := io.Copy(out, in); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "installed %s → %s\n", filepath.Base(src), dst)
			fmt.Fprintf(cmd.OutOrStdout(), "run with: m chat %s\n", strings.TrimSuffix(filepath.Base(src), ".md"))
			return nil
		},
	}
}
