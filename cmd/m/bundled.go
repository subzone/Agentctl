package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/subzone/Agentctl/examples"
)

// extractBundledAgents copies embedded agents to ~/.config/m/agents/ if the
// directory doesn't exist yet (first run). Existing files are never overwritten
// so user edits are preserved.
func extractBundledAgents() error {
	if err := extractEmbedded(examples.Agents, "agents", agentRegistryDir()); err != nil {
		return err
	}
	// MCP server definitions ship alongside agents so that agents which
	// reference them (e.g. the MoE experts referencing knowledge-master)
	// can resolve them as companion docs.
	return extractEmbedded(examples.MCP, "mcp", mcpRegistryDir())
}

// extractEmbedded copies every .md under root in src into dstDir, creating
// the directory if needed. Existing files are never overwritten so user
// edits are preserved.
func extractEmbedded(src fs.FS, root, dstDir string) error {
	if dstDir == "" {
		return nil
	}
	if err := os.MkdirAll(dstDir, 0o700); err != nil {
		return err
	}
	return fs.WalkDir(src, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		dst := filepath.Join(dstDir, filepath.Base(path))
		if _, err := os.Stat(dst); err == nil {
			return nil // never overwrite user-edited files
		}
		data, err := fs.ReadFile(src, path)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, data, 0o644)
	})
}

// bundledAgentPath looks up an agent by name in the embedded FS directly
// (fallback when the file isn't in the config dir). Returns "" if not found.
func bundledAgentPath(name string) string {
	path := "agents/" + name + ".md"
	if _, err := fs.Stat(examples.Agents, path); err == nil {
		// Extract on demand to a temp location so callers get a real path.
		data, err := examples.Agents.ReadFile(path)
		if err != nil {
			return ""
		}
		dir := agentRegistryDir()
		if dir == "" {
			return ""
		}
		os.MkdirAll(dir, 0o700)
		dst := filepath.Join(dir, name+".md")
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return ""
		}
		fmt.Fprintf(os.Stderr, "  extracted bundled agent → %s\n", dst)
		return dst
	}
	return ""
}
